package firewall

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

// SignPolicy signs the canonical hash of the policy using the provided private key
// and populates the Signature and SignaturePublicKey fields.
func SignPolicy(policy *Policy, privKey *ecdsa.PrivateKey) error {
	hash, err := policy.CanonicalHash()
	if err != nil {
		return err
	}

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash)
	if err != nil {
		return fmt.Errorf("failed to sign policy: %w", err)
	}

	// Format signature as r || s (64 bytes total for P-256)
	sigBytes := make([]byte, 64)
	r.FillBytes(sigBytes[:32])
	s.FillBytes(sigBytes[32:])

	// Format public key as x || y (64 bytes total for P-256)
	pubBytes := make([]byte, 64)
	privKey.X.FillBytes(pubBytes[:32])
	privKey.Y.FillBytes(pubBytes[32:])

	policy.Signature = hex.EncodeToString(sigBytes)
	policy.SignaturePublicKey = hex.EncodeToString(pubBytes)

	return nil
}

// VerifyPolicySignature verifies the signature on the policy.
// It returns nil if the signature is valid, or an error if invalid.
func VerifyPolicySignature(policy *Policy) error {
	if policy.Signature == "" || policy.SignaturePublicKey == "" {
		return fmt.Errorf("missing signature or public key")
	}

	sigBytes, err := hex.DecodeString(policy.Signature)
	if err != nil || len(sigBytes) != 64 {
		return fmt.Errorf("invalid signature format")
	}

	pubBytes, err := hex.DecodeString(policy.SignaturePublicKey)
	if err != nil || len(pubBytes) != 64 {
		return fmt.Errorf("invalid public key format")
	}

	hash, err := policy.CanonicalHash()
	if err != nil {
		return fmt.Errorf("failed to generate canonical hash: %w", err)
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	x := new(big.Int).SetBytes(pubBytes[:32])
	y := new(big.Int).SetBytes(pubBytes[32:])

	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	if !ecdsa.Verify(pubKey, hash, r, s) {
		return fmt.Errorf("cryptographic signature verification failed")
	}

	return nil
}
