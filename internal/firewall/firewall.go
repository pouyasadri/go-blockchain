package firewall

import (
	"fmt"
	"sync"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/core"
)

// LedgerEntry represents a past transaction to evaluate rolling limits
type LedgerEntry struct {
	Timestamp time.Time
	Amount    int64
	TxID      string
}

// Firewall enforces policy rules on proposed transactions
type Firewall struct {
	mu            sync.RWMutex
	policy        *Policy
	rollingWindow []LedgerEntry // Transactions in the last 24h
	sessionSpent  int64         // Total micro-cents spent this session
}

// NewFirewall creates a new financial firewall from a verified policy
func NewFirewall(p *Policy) *Firewall {
	return &Firewall{
		policy:        p,
		rollingWindow: make([]LedgerEntry, 0),
		sessionSpent:  0,
	}
}

// GetPolicy returns the current policy
func (fw *Firewall) GetPolicy() *Policy {
	return fw.policy
}

// GetSessionStats returns current session budget and spent amount
func (fw *Firewall) GetSessionStats() (int64, int64) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	return fw.policy.SessionBudget, fw.sessionSpent
}

// Evaluate checks a proposed transaction against all firewall rules.
// Returns (true, "") if allowed, or (false, reason) if blocked.
func (fw *Firewall) Evaluate(amount int64, recipient string) (bool, string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	now := time.Now()

	// 1. Compile-time hard cap (overrides policy if policy is misconfigured)
	if amount > core.MaxTransactionValueLimit {
		return false, fmt.Sprintf("amount exceeds hardcoded safety limit (%d)", core.MaxTransactionValueLimit)
	}

	// 2. Per-transaction cap
	if amount > fw.policy.MaxPerTransaction {
		return false, fmt.Sprintf("amount exceeds per-transaction limit (%d)", fw.policy.MaxPerTransaction)
	}

	// 3. Session budget
	if fw.sessionSpent+amount > fw.policy.SessionBudget {
		return false, fmt.Sprintf("amount exceeds remaining session budget (%d out of %d remaining)", fw.policy.SessionBudget-fw.sessionSpent, fw.policy.SessionBudget)
	}

	// Clean up old entries from rolling window (older than 24h)
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff1h := now.Add(-1 * time.Hour)
	
	validEntries := make([]LedgerEntry, 0)
	var rolling24hTotal int64 = 0
	txCountLastHour := 0

	for _, entry := range fw.rollingWindow {
		if entry.Timestamp.After(cutoff24h) {
			validEntries = append(validEntries, entry)
			rolling24hTotal += entry.Amount
			if entry.Timestamp.After(cutoff1h) {
				txCountLastHour++
			}
		}
	}
	fw.rollingWindow = validEntries

	// 4. Rolling 24h window
	if rolling24hTotal+amount > fw.policy.Rolling24hBudget {
		return false, fmt.Sprintf("amount exceeds rolling 24h budget (%d out of %d remaining)", fw.policy.Rolling24hBudget-rolling24hTotal, fw.policy.Rolling24hBudget)
	}

	// 5. Rate limit
	if txCountLastHour >= fw.policy.MaxTxPerHour {
		return false, fmt.Sprintf("rate limit exceeded (%d tx/hour)", fw.policy.MaxTxPerHour)
	}

	// 6. Allowed recipients (if configured)
	if len(fw.policy.AllowedRecipients) > 0 {
		allowed := false
		for _, addr := range fw.policy.AllowedRecipients {
			if addr == recipient {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, fmt.Sprintf("recipient %s is not in the allowlist", recipient)
		}
	}

	return true, ""
}

// RecordTransaction adds a successfully broadcast transaction to the firewall's ledgers
func (fw *Firewall) RecordTransaction(amount int64, txID string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.sessionSpent += amount
	fw.rollingWindow = append(fw.rollingWindow, LedgerEntry{
		Timestamp: time.Now(),
		Amount:    amount,
		TxID:      txID,
	})
}
