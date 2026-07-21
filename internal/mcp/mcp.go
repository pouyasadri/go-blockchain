package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/pouyasadri/go-blockchain/api/proto"
	"github.com/pouyasadri/go-blockchain/internal/firewall"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MCPDaemon holds the state of the MCP server
type MCPDaemon struct {
	fw       *firewall.Firewall
	keyring  *Keyring
	grpcConn *grpc.ClientConn
	api      proto.NodeServiceClient
	
	// I/O streams for JSON-RPC
	in       io.Reader
	out      io.Writer
	errLog   *log.Logger
	
	initialized bool
}

// NewMCPDaemon creates a new MCP daemon
func NewMCPDaemon(fw *firewall.Firewall, in io.Reader, out io.Writer, errOut io.Writer) (*MCPDaemon, error) {
	kr, err := NewKeyring()
	if err != nil {
		return nil, err
	}
	
	// Note: in a real environment, we'd use TLS for gRPC.
	conn, err := grpc.NewClient(fw.GetPolicy().NodeGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to node at %s: %w", fw.GetPolicy().NodeGRPCAddress, err)
	}
	
	api := proto.NewNodeServiceClient(conn)
	
	return &MCPDaemon{
		fw:          fw,
		keyring:     kr,
		grpcConn:    conn,
		api:         api,
		in:          in,
		out:         out,
		errLog:      log.New(errOut, "[mcp-daemon] ", log.LstdFlags),
		initialized: false,
	}, nil
}

// Close gracefully closes connections
func (d *MCPDaemon) Close() {
	if d.grpcConn != nil {
		_ = d.grpcConn.Close()
	}
}

// Run starts the JSON-RPC stdio loop
func (d *MCPDaemon) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(d.in)
	
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("read error: %w", err)
				}
				// EOF
				return nil
			}
			
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			
			d.handleMessage([]byte(line))
		}
	}
}

func (d *MCPDaemon) handleMessage(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		d.sendError(nil, ParseError, "Parse error", err.Error())
		return
	}
	
	if req.JSONRPC != "2.0" || req.Method == "" {
		d.sendError(req.ID, InvalidRequest, "Invalid Request", nil)
		return
	}
	
	d.errLog.Printf("Received method: %s", req.Method)
	
	if !d.initialized && req.Method != "initialize" && req.Method != "notifications/initialized" {
		d.sendError(req.ID, InternalError, "Server not initialized", nil)
		return
	}
	
	switch req.Method {
	case "initialize":
		d.handleInitialize(req)
	case "notifications/initialized":
		d.initialized = true
	case "tools/list":
		d.handleToolsList(req)
	case "tools/call":
		d.handleToolsCall(req)
	default:
		d.sendError(req.ID, MethodNotFound, "Method not found", nil)
	}
}

func (d *MCPDaemon) handleInitialize(req Request) {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil && len(req.Params) > 0 {
		d.sendError(req.ID, InvalidParams, "Invalid params", err.Error())
		return
	}
	
	result := InitializeResult{
		ProtocolVersion: "2024-11-05", // From MCP specification
		Capabilities: map[string]any{
			"tools": map[string]any{},
		},
	}
	result.ServerInfo.Name = "ai-to-ai-ledger-mcp"
	result.ServerInfo.Version = "0.1.0"
	
	d.sendResponse(req.ID, result)
}

func (d *MCPDaemon) handleToolsList(req Request) {
	tools := []Tool{
		{
			Name:        "wallet_create",
			Description: "Generate a new ephemeral wallet for this session",
			InputSchema: struct {
				Type       string         `json:"type"`
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required,omitempty"`
			}{
				Type:       "object",
				Properties: map[string]any{},
			},
		},
		{
			Name:        "wallet_balance",
			Description: "Query balance for an address",
			InputSchema: struct {
				Type       string         `json:"type"`
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]any{
					"address": map[string]any{
						"type": "string",
						"description": "Base58 encoded address",
					},
				},
				Required: []string{"address"},
			},
		},
		{
			Name:        "tx_send",
			Description: "Send a standard P2PKH transaction",
			InputSchema: struct {
				Type       string         `json:"type"`
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]any{
					"to": map[string]any{
						"type": "string",
						"description": "Recipient address",
					},
					"amount": map[string]any{
						"type": "integer",
						"description": "Amount in micro-cents",
					},
				},
				Required: []string{"to", "amount"},
			},
		},
		// ... escrow tools ...
	}
	
	d.sendResponse(req.ID, map[string]any{
		"tools": tools,
	})
}

func (d *MCPDaemon) sendResponse(id any, result any) {
	resp := NewResponse(id, result)
	d.write(resp)
}

func (d *MCPDaemon) sendError(id any, code int, message string, data any) {
	resp := NewErrorResponse(id, code, message, data)
	d.write(resp)
}

func (d *MCPDaemon) write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		d.errLog.Printf("Failed to marshal response: %v", err)
		return
	}
	
	// newline-delimited JSON
	data = append(data, '\n')
	_, err = d.out.Write(data)
	if err != nil {
		d.errLog.Printf("Failed to write to stdout: %v", err)
	}
}
