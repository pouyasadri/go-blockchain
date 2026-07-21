package zkcp

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZKCPProofGenerationAndVerification(t *testing.T) {
	prover := NewProver()
	verifier := NewVerifier()

	preimage := []byte("secret_ai_computation_result_999")
	hashLock := sha256.Sum256(preimage)

	// 1. Generate Proof
	proof, err := prover.GeneratePreimageProof(preimage)
	assert.NoError(t, err)
	assert.NotNil(t, proof)
	assert.Equal(t, hashLock[:], proof.HashLock)

	// 2. Verify Valid Proof
	valid := verifier.VerifyProof(hashLock[:], preimage, proof)
	assert.True(t, valid)

	// 3. Reject Invalid Preimage
	wrongPreimage := []byte("wrong_secret_preimage")
	valid = verifier.VerifyProof(hashLock[:], wrongPreimage, proof)
	assert.False(t, valid)

	// 4. Reject Tampered Proof Data
	proof.ProofData[0] ^= 0xFF
	valid = verifier.VerifyProof(hashLock[:], preimage, proof)
	assert.False(t, valid)
}
