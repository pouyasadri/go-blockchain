package firewall

import (
	"testing"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestFirewallCompileTimeCap(t *testing.T) {
	p := &Policy{
		MaxPerTransaction: core.MaxTransactionValueLimit + 1000,
	}
	fw := NewFirewall(p)

	ok, reason := fw.Evaluate(core.MaxTransactionValueLimit+1, "addr1")
	assert.False(t, ok)
	assert.Contains(t, reason, "hardcoded safety limit")
}

func TestFirewallPerTxCap(t *testing.T) {
	p := &Policy{
		MaxPerTransaction: 1000,
		SessionBudget:     5000,
		Rolling24hBudget:  10000,
		MaxTxPerHour:      10,
	}
	fw := NewFirewall(p)

	ok, _ := fw.Evaluate(500, "addr1")
	assert.True(t, ok)

	ok, reason := fw.Evaluate(1500, "addr1")
	assert.False(t, ok)
	assert.Contains(t, reason, "per-transaction limit")
}

func TestFirewallSessionBudget(t *testing.T) {
	p := &Policy{
		MaxPerTransaction: 1000,
		SessionBudget:     2500,
		Rolling24hBudget:  10000,
		MaxTxPerHour:      10,
	}
	fw := NewFirewall(p)

	// tx 1
	ok, _ := fw.Evaluate(1000, "addr1")
	assert.True(t, ok)
	fw.RecordTransaction(1000, "tx1")

	// tx 2
	ok, _ = fw.Evaluate(1000, "addr1")
	assert.True(t, ok)
	fw.RecordTransaction(1000, "tx2")

	// tx 3 (exceeds 2500 budget)
	ok, reason := fw.Evaluate(1000, "addr1")
	assert.False(t, ok)
	assert.Contains(t, reason, "session budget")
}

func TestFirewallRolling24h(t *testing.T) {
	p := &Policy{
		MaxPerTransaction: 1000,
		SessionBudget:     10000,
		Rolling24hBudget:  1500,
		MaxTxPerHour:      10,
	}
	fw := NewFirewall(p)

	now := time.Now()

	// Inject old transactions
	fw.rollingWindow = []LedgerEntry{
		{Timestamp: now.Add(-25 * time.Hour), Amount: 1000, TxID: "old1"}, // Should be purged
		{Timestamp: now.Add(-10 * time.Hour), Amount: 1000, TxID: "old2"}, // Still counts
	}

	ok, _ := fw.Evaluate(400, "addr1")
	assert.True(t, ok)

	ok, reason := fw.Evaluate(600, "addr1")
	assert.False(t, ok)
	assert.Contains(t, reason, "rolling 24h budget")
}

func TestFirewallRateLimit(t *testing.T) {
	p := &Policy{
		MaxPerTransaction: 1000,
		SessionBudget:     10000,
		Rolling24hBudget:  10000,
		MaxTxPerHour:      2,
	}
	fw := NewFirewall(p)

	now := time.Now()

	fw.rollingWindow = []LedgerEntry{
		{Timestamp: now.Add(-30 * time.Minute), Amount: 100, TxID: "recent1"},
		{Timestamp: now.Add(-10 * time.Minute), Amount: 100, TxID: "recent2"},
		{Timestamp: now.Add(-2 * time.Hour), Amount: 100, TxID: "old1"}, // Doesn't count for rate limit
	}

	ok, reason := fw.Evaluate(100, "addr1")
	assert.False(t, ok)
	assert.Contains(t, reason, "rate limit exceeded")
}

func TestFirewallAllowedRecipients(t *testing.T) {
	p := &Policy{
		MaxPerTransaction: 1000,
		SessionBudget:     10000,
		Rolling24hBudget:  10000,
		MaxTxPerHour:      10,
		AllowedRecipients: []string{"trusted_addr_1", "trusted_addr_2"},
	}
	fw := NewFirewall(p)

	ok, _ := fw.Evaluate(100, "trusted_addr_2")
	assert.True(t, ok)

	ok, reason := fw.Evaluate(100, "untrusted_addr")
	assert.False(t, ok)
	assert.Contains(t, reason, "allowlist")
}
