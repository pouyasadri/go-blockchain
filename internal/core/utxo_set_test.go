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

	// Test Update with remaining outputs
	// 1. Manually insert a UTXO with 2 outputs
	testTxID := []byte("test_tx_id_12345")
	out1 := TXOutput{Value: 10, PubKeyHash: pubKeyHash}
	out2 := TXOutput{Value: 5, PubKeyHash: pubKeyHash}
	txouts := TXOutputs{Outputs: []TXOutput{out1, out2}}
	err = db.SaveUTXO(testTxID, txouts.Serialize())
	assert.NoError(t, err)

	// 2. Create a transaction that spends only the first output (index 0)
	txin := TXInput{
		Txid:      testTxID,
		Vout:      0,
		Signature: nil,
		PubKey:    wallet.PublicKey,
	}
	txout := NewTXOutput(10, addr)
	txn := &Transaction{
		ID:   []byte("test_txn_spend_one"),
		Vin:  []TXInput{txin},
		Vout: []TXOutput{*txout},
	}

	// 3. Create a block containing this transaction manually
	newBlock := &Block{
		Transactions: []*Transaction{txn},
	}

	// 4. Update the UTXO set: this should update testTxID UTXO to keep only out2,
	// exercising the else branch in Update() where len(updatedOuts.Outputs) > 0.
	err = utxoSet.Update(newBlock)
	assert.NoError(t, err)

	// 5. Verify that testTxID still exists in UTXO database with out2
	updatedBytes, err := db.GetUTXO(testTxID)
	assert.NoError(t, err)
	updatedOuts := DeserializeOutputs(updatedBytes)
	assert.Len(t, updatedOuts.Outputs, 1)
	assert.Equal(t, 5, updatedOuts.Outputs[0].Value)
}
