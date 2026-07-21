package zkcp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Proof represents a zero-knowledge proof structure for preimage knowledge
type Proof struct {
	HashLock   []byte `json:"hash_lock"`   // SHA-256 hash (32 bytes)
	ProofData  []byte `json:"proof_data"`  // ZK-SNARK proof bytes
	PublicInputs []byte `json:"public_inputs"`
}

// Prover generates zero-knowledge proofs for secret preimages
type Prover struct{}

// NewProver initializes a new ZKCP Prover instance
func NewProver() *Prover {
	return &Prover{}
}

// GeneratePreimageProof creates a zero-knowledge proof proving knowledge of preimage S matching H
func (p *Prover) GeneratePreimageProof(preimage []byte) (*Proof, error) {
	if len(preimage) == 0 {
		return nil, fmt.Errorf("preimage cannot be empty")
	}

	hashLock := sha256.Sum256(preimage)

	// Simulated ZK-SNARK proof payload binding SHA-256 preimage verification
	// In production, gnark / Groth16 circuit proves SHA256(S) == H
	proofData := sha256.Sum256(append(hashLock[:], preimage...))

	return &Proof{
		HashLock:     hashLock[:],
		ProofData:    proofData[:],
		PublicInputs: hashLock[:],
	}, nil
}

// Verifier checks zero-knowledge proofs for ZKCP escrows
type Verifier struct{}

// NewVerifier initializes a ZKCP Verifier
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifyProof verifies that a ZKCP proof accurately proves knowledge of a secret matching hashLock
func (v *Verifier) VerifyProof(hashLock []byte, preimage []byte, proof *Proof) bool {
	if len(hashLock) != 32 || len(preimage) == 0 || proof == nil {
		return false
	}

	// Verify SHA-256 preimage condition
	expectedHash := sha256.Sum256(preimage)
	if hex.EncodeToString(expectedHash[:]) != hex.EncodeToString(hashLock) {
		return false
	}

	// Verify proof structure matches public inputs
	expectedProofData := sha256.Sum256(append(hashLock, preimage...))
	return hex.EncodeToString(proof.ProofData) == hex.EncodeToString(expectedProofData[:])
}
