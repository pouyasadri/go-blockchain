package firewall

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignAndVerifyPolicy(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	p := &Policy{
		Version:           1,
		SessionBudget:     100,
		MaxPerTransaction: 10,
	}

	err = SignPolicy(p, privKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, p.Signature)
	assert.NotEmpty(t, p.SignaturePublicKey)

	err = VerifyPolicySignature(p)
	assert.NoError(t, err)

	// Tamper with data
	p.SessionBudget = 200
	err = VerifyPolicySignature(p)
	assert.Error(t, err)
}

func TestLoadAndVerify(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	p := &Policy{
		Version:           1,
		SessionBudget:     100,
		MaxPerTransaction: 10,
	}

	err = SignPolicy(p, privKey)
	assert.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")

	data, err := json.Marshal(p)
	assert.NoError(t, err)

	err = os.WriteFile(path, data, 0644)
	assert.NoError(t, err)

	loaded, err := LoadAndVerify(path)
	assert.NoError(t, err)
	assert.Equal(t, p.SessionBudget, loaded.SessionBudget)
	assert.Equal(t, p.Signature, loaded.Signature)

	// Tamper file on disk
	p.SessionBudget = 200
	data, _ = json.Marshal(p) // New hash, old signature
	os.WriteFile(path, data, 0644)

	assert.Panics(t, func() {
		LoadAndVerify(path)
	})
}
