package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWallet(t *testing.T) {
	wallet, err := NewWallet()
	assert.NoError(t, err)
	assert.NotNil(t, wallet)
	assert.NotEmpty(t, wallet.PublicKey)
	assert.NotNil(t, wallet.PrivateKey)

	address := wallet.GetAddress()
	assert.NotEmpty(t, address)
	assert.True(t, ValidateAddress(string(address)))
}

func TestValidateAddress(t *testing.T) {
	wallet, _ := NewWallet()
	address := string(wallet.GetAddress())
	assert.True(t, ValidateAddress(address))

	assert.False(t, ValidateAddress("invalid_address"))
}

func TestWallets(t *testing.T) {
	nodeID := "testnode"

	t.Cleanup(func() {
		_ = os.Remove("wallet_testnode.dat")
	})

	ws, err := NewWallets(nodeID)
	assert.NoError(t, err)
	assert.NotNil(t, ws)

	addr, err := ws.CreateWallet()
	assert.NoError(t, err)
	assert.NotEmpty(t, addr)

	err = ws.SaveToFile(nodeID)
	assert.NoError(t, err)

	ws2, err := NewWallets(nodeID)
	assert.NoError(t, err)
	assert.NotNil(t, ws2)

	w := ws2.GetWallet(addr)
	assert.NotNil(t, w)
	assert.Equal(t, addr, string(w.GetAddress()))

	addresses := ws2.GetAddresses()
	assert.Contains(t, addresses, addr)

	// Test LoadFromFile on non-existent file
	wsMissing, err := NewWallets("missingnode")
	assert.NoError(t, err)
	err = wsMissing.LoadFromFile("missingnode")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Test SaveToFile error
	err = ws.SaveToFile("invalid_dir_1234/testnode")
	assert.Error(t, err)

	// Test LoadFromFile decode error
	err = os.WriteFile("wallet_badnode.dat", []byte("corrupted_data"), 0644)
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove("wallet_badnode.dat")
	}()

	_, err = NewWallets("badnode")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode wallet file")
}
