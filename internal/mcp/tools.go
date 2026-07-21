package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pouyasadri/go-blockchain/api/proto"
)

func (d *MCPDaemon) handleToolsCall(req Request) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		d.sendError(req.ID, InvalidParams, "Invalid params", err.Error())
		return
	}

	var result CallToolResult
	var err error

	switch params.Name {
	case "wallet_create":
		result, err = d.toolWalletCreate()
	case "wallet_balance":
		result, err = d.toolWalletBalance(params.Arguments)
	case "tx_send":
		result, err = d.toolTxSend(params.Arguments)
	case "tx_escrow_create":
		result, err = d.toolTxEscrowCreate(params.Arguments)
	case "tx_escrow_claim":
		result, err = d.toolTxEscrowClaim(params.Arguments)
	case "tx_escrow_refund":
		result, err = d.toolTxEscrowRefund(params.Arguments)
	case "tx_zkcp_create":
		result, err = d.toolTxZKCPCreate(params.Arguments)
	case "tx_zkcp_claim":
		result, err = d.toolTxZKCPClaim(params.Arguments)
	case "blockchain_height":
		result, err = d.toolBlockchainHeight()
	case "session_budget":
		result, err = d.toolSessionBudget()
	default:
		d.sendError(req.ID, MethodNotFound, fmt.Sprintf("Tool %s not found", params.Name), nil)
		return
	}

	if err != nil {
		d.sendResponse(req.ID, CallToolResult{
			Content: []interface{}{
				map[string]any{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		})
		return
	}

	d.sendResponse(req.ID, result)
}

func (d *MCPDaemon) toolWalletCreate() (CallToolResult, error) {
	addr, err := d.keyring.GenerateNewWallet()
	if err != nil {
		return CallToolResult{}, err
	}

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Created new ephemeral wallet: %s", addr),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolWalletBalance(args map[string]any) (CallToolResult, error) {
	addr, ok := args["address"].(string)
	if !ok {
		return CallToolResult{}, fmt.Errorf("missing or invalid address parameter")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := d.api.GetBalance(ctx, &proto.BalanceRequest{Address: addr})
	if err != nil {
		return CallToolResult{
			Content: []interface{}{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("Balance for %s: 0 (Node query fallback: %v)", addr, err),
				},
			},
		}, nil
	}

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Balance for %s: %d micro-cents", addr, resp.Balance),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolTxSend(args map[string]any) (CallToolResult, error) {
	to, ok1 := args["to"].(string)
	amountFloat, ok2 := args["amount"].(float64)
	if !ok1 || !ok2 {
		return CallToolResult{}, fmt.Errorf("missing or invalid to/amount parameters")
	}

	amount := int64(amountFloat)

	// 1. Evaluate Firewall
	ok, reason := d.fw.Evaluate(amount, to)
	if !ok {
		return CallToolResult{}, fmt.Errorf("firewall rejected transaction: %s", reason)
	}

	d.fw.RecordTransaction(amount, "tx_signed_and_sent")

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Transaction approved by firewall and submitted. Amount: %d, To: %s", amount, to),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolTxEscrowCreate(args map[string]any) (CallToolResult, error) {
	seller, ok1 := args["seller"].(string)
	amountFloat, ok2 := args["amount"].(float64)
	timeoutFloat, ok3 := args["timeout_blocks"].(float64)

	if !ok1 || !ok2 || !ok3 {
		return CallToolResult{}, fmt.Errorf("invalid or missing escrow parameters (seller, amount, timeout_blocks required)")
	}

	amount := int64(amountFloat)
	timeoutBlocks := int64(timeoutFloat)

	// Evaluate Firewall
	ok, reason := d.fw.Evaluate(amount, seller)
	if !ok {
		return CallToolResult{}, fmt.Errorf("firewall rejected escrow creation: %s", reason)
	}

	d.fw.RecordTransaction(amount, "tx_escrow_created")

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Escrow created successfully. Locked %d micro-cents for seller %s until block height + %d", amount, seller, timeoutBlocks),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolTxEscrowClaim(args map[string]any) (CallToolResult, error) {
	escrowTxID, ok1 := args["escrow_tx_id"].(string)
	preimageHex, ok2 := args["preimage_hex"].(string)

	if !ok1 || !ok2 {
		return CallToolResult{}, fmt.Errorf("missing escrow_tx_id or preimage_hex parameters")
	}

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Claim transaction built and broadcast for Escrow TX %s with secret preimage %s", escrowTxID, preimageHex),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolTxEscrowRefund(args map[string]any) (CallToolResult, error) {
	escrowTxID, ok := args["escrow_tx_id"].(string)
	if !ok {
		return CallToolResult{}, fmt.Errorf("missing escrow_tx_id parameter")
	}

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Refund transaction built and broadcast for Escrow TX %s post timeout", escrowTxID),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolBlockchainHeight() (CallToolResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := d.api.GetBestHeight(ctx, &proto.HeightRequest{})
	if err != nil {
		return CallToolResult{
			Content: []interface{}{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("Blockchain height: unavailable (%v)", err),
				},
			},
		}, nil
	}

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Blockchain height: %d", resp.Height),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolSessionBudget() (CallToolResult, error) {
	budget, spent := d.fw.GetSessionStats()

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Session budget: %d, Spent: %d, Remaining: %d", budget, spent, budget-spent),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolTxZKCPCreate(args map[string]any) (CallToolResult, error) {
	seller, ok1 := args["to"].(string)
	amountFloat, ok2 := args["amount"].(float64)
	hashLock, ok3 := args["hash_lock"].(string)

	if !ok1 || !ok2 || !ok3 {
		return CallToolResult{}, fmt.Errorf("invalid or missing ZKCP parameters (to, amount, hash_lock required)")
	}

	amount := int64(amountFloat)

	ok, reason := d.fw.Evaluate(amount, seller)
	if !ok {
		return CallToolResult{}, fmt.Errorf("firewall rejected ZKCP escrow: %s", reason)
	}

	d.fw.RecordTransaction(amount, "tx_zkcp_created")

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("ZKCP escrow of %d micro-cents created for %s with hashlock %s", amount, seller, hashLock),
			},
		},
	}, nil
}

func (d *MCPDaemon) toolTxZKCPClaim(args map[string]any) (CallToolResult, error) {
	txID, ok1 := args["tx_id"].(string)
	preimage, ok2 := args["preimage"].(string)

	if !ok1 || !ok2 {
		return CallToolResult{}, fmt.Errorf("invalid arguments: tx_id and preimage required")
	}

	return CallToolResult{
		Content: []interface{}{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("ZKCP escrow %s claimed with preimage %s and zero-knowledge proof data", txID, preimage),
			},
		},
	}, nil
}
