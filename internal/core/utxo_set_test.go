package core

import (
	"path/filepath"
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
)

func TestUTXOSet(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	bc, err := CreateBlockchain(addr, db)
	assert.NoError(t, err)

	utxoSet := UTXOSet{Blockchain: bc}
	err = utxoSet.Reindex()
	assert.NoError(t, err)

	count, err := utxoSet.CountTransactions()
	assert.NoError(t, err)
	assert.Equal(t, 1, count) // Only the coinbase transaction

	// Test FindSpendableOutputs
	pubKeyHash := HashPubKey(wallet.PublicKey)
	accumulated, unspentOutputs, err := utxoSet.FindSpendableOutputs(pubKeyHash, 5)
	assert.NoError(t, err)
	assert.True(t, accumulated >= 5)
	assert.NotEmpty(t, unspentOutputs)
}
