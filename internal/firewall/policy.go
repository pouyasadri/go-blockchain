package firewall

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
)

// Policy defines the financial firewall configuration for an MCP daemon session.
type Policy struct {
	Version            int      `json:"version"`
	SessionBudget      int64    `json:"session_budget"`       // Max spend per daemon session (micro-cents)
	MaxPerTransaction  int64    `json:"max_per_transaction"`  // Per-tx cap (micro-cents)
	Rolling24hBudget   int64    `json:"rolling_24h_budget"`   // Rolling 24-hour cap (micro-cents)
	MaxTxPerHour       int      `json:"max_tx_per_hour"`      // Rate limit
	AllowedRecipients  []string `json:"allowed_recipients,omitempty"` // Optional allowlist of addresses
	NodeGRPCAddress    string   `json:"node_grpc_address"`    // Core node endpoint
	PersistentKeyPath  string   `json:"persistent_key_path,omitempty"` // Optional key file path
	Signature          string   `json:"signature"`            // Hex-encoded WebAuthn/Passkey signature
	SignaturePublicKey string   `json:"signature_public_key"` // Hex-encoded public key
}

// CanonicalHash returns the SHA-256 hash of the policy without signature fields.
// This is the data that must be signed.
func (p *Policy) CanonicalHash() ([]byte, error) {
	// Create a copy without signatures to hash
	clone := *p
	clone.Signature = ""
	clone.SignaturePublicKey = ""

	// Marshal to canonical JSON (Go's json.Marshal sorts map keys, though we don't use maps here.
	// It's deterministic enough for struct fields in defined order).
	data, err := json.Marshal(clone)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal policy for hashing: %w", err)
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// LoadAndVerify reads policy.json from disk, verifies its signature,
// and returns the parsed policy. Panics if signature is missing or invalid.
func LoadAndVerify(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	var p Policy
	// We use json.NewDecoder to reject unknown fields to ensure strictly valid policies
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return nil, fmt.Errorf("failed to parse policy json: %w", err)
	}

	if p.Signature == "" || p.SignaturePublicKey == "" {
		panic(fmt.Sprintf("FATAL: Policy file %s is missing signature or public key. Firewall locked.", path))
	}

	if err := VerifyPolicySignature(&p); err != nil {
		panic(fmt.Sprintf("FATAL: Policy signature verification failed for %s: %v. Firewall locked.", path, err))
	}

	return &p, nil
}
