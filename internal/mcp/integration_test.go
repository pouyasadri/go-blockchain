package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/firewall"
	"github.com/stretchr/testify/assert"
)

func TestIntegration_StdioLoop(t *testing.T) {
	// 1. Setup mock stdin/stdout
	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	// 2. Write initialization sequence to stdin
	initReq := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}`),
	}
	reqData, _ := json.Marshal(initReq)
	in.Write(reqData)
	in.WriteString("\n")

	initNotif := Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	reqData2, _ := json.Marshal(initNotif)
	in.Write(reqData2)
	in.WriteString("\n")

	toolReq := Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "session_budget"}`),
	}
	reqData3, _ := json.Marshal(toolReq)
	in.Write(reqData3)
	in.WriteString("\n")

	// 3. Run daemon synchronously (it will exit when stdin hits EOF)
	fw := firewall.NewFirewall(&firewall.Policy{
		NodeGRPCAddress: "localhost:1234",
		SessionBudget:   10000,
	})
	daemon, _ := NewMCPDaemon(fw, in, out, errOut)
	
	err := daemon.Run(context.Background())
	assert.NoError(t, err)

	// 4. Verify stdout contains expected responses
	outStr := out.String()
	assert.Contains(t, outStr, `"id":1`) // init response
	assert.Contains(t, outStr, `"id":2`) // tool response
	assert.Contains(t, outStr, "Session budget: 10000")
}
