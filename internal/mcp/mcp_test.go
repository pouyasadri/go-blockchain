package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/firewall"
	"github.com/stretchr/testify/assert"
)

func TestMCPInitialize(t *testing.T) {
	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	fw := firewall.NewFirewall(&firewall.Policy{NodeGRPCAddress: "localhost:1234"})
	daemon, _ := NewMCPDaemon(fw, in, out, errOut)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}`),
	}
	reqData, _ := json.Marshal(req)
	in.Write(reqData)
	in.WriteString("\n")

	// Need a context to cancel the run loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool)
	go func() {
		_ = daemon.Run(ctx)
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)
	cancel() // Stop the daemon
	<-done   // Wait for it to exit

	var resp Response
	err := json.Unmarshal(out.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestToolsList(t *testing.T) {
	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	fw := firewall.NewFirewall(&firewall.Policy{NodeGRPCAddress: "localhost:1234"})
	daemon, _ := NewMCPDaemon(fw, in, out, errOut)
	daemon.initialized = true // bypass init

	req := Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
	reqData, _ := json.Marshal(req)
	in.Write(reqData)
	in.WriteString("\n")

	daemon.handleMessage(reqData)

	var resp Response
	err := json.Unmarshal(out.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Nil(t, resp.Error)

	resultMap := resp.Result.(map[string]interface{})
	tools := resultMap["tools"].([]interface{})
	assert.GreaterOrEqual(t, len(tools), 3)
}

func TestToolCall_TxSend_FirewallReject(t *testing.T) {
	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	fw := firewall.NewFirewall(&firewall.Policy{
		NodeGRPCAddress:   "localhost:1234",
		MaxPerTransaction: 100, // Very low cap
	})
	daemon, _ := NewMCPDaemon(fw, in, out, errOut)
	daemon.initialized = true

	req := Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "tx_send", "arguments": {"to": "addr1", "amount": 1000}}`), // Exceeds cap
	}
	reqData, _ := json.Marshal(req)
	
	daemon.handleMessage(reqData)

	var resp Response
	err := json.Unmarshal(out.Bytes(), &resp)
	assert.NoError(t, err)

	// In MCP, errors from tools are returned as normal responses with isError=true in the result,
	// NOT as JSON-RPC errors (unless the tool itself couldn't be invoked).
	resultMap := resp.Result.(map[string]interface{})
	assert.True(t, resultMap["isError"].(bool))
	
	content := resultMap["content"].([]interface{})[0].(map[string]interface{})
	text := content["text"].(string)
	assert.Contains(t, text, "firewall rejected")
}
