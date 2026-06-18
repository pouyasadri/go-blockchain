package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransaction(t *testing.T) {
	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	txin := TXInput{[]byte{}, -1, nil, wallet.PublicKey}
	txout := NewTXOutput(10, addr)
	tx := &Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()

	assert.NotNil(t, tx)
	assert.NotEmpty(t, tx.ID)

	serialized := tx.Serialize()
	assert.NotEmpty(t, serialized)

	deserialized, err := DeserializeTransaction(serialized)
	assert.NoError(t, err)
	assert.Equal(t, tx.ID, deserialized.ID)
	assert.True(t, tx.IsCoinbase())
}

func TestTXOutput(t *testing.T) {
	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	out := NewTXOutput(100, addr)
	assert.Equal(t, 100, out.Value)
	assert.NotEmpty(t, out.PubKeyHash)
}

func TestTXInput(t *testing.T) {
	wallet, _ := NewWallet()
	pubKeyHash := HashPubKey(wallet.PublicKey)

	in := TXInput{[]byte("id"), 0, nil, wallet.PublicKey}
	assert.True(t, in.UsesKey(pubKeyHash))
}

func TestTransactionErrors(t *testing.T) {
	wallet, _ := NewWallet()
	txin := TXInput{Txid: []byte("parent"), Vout: 0}
	txn := &Transaction{Vin: []TXInput{txin}}

	// Test Sign returning error on invalid prevTXs
	err := txn.Sign(wallet.PrivateKey, map[string]Transaction{})
	assert.Error(t, err)

	// Test Verify returning false on invalid prevTXs
	valid := txn.Verify(map[string]Transaction{})
	assert.False(t, valid)

	// Test DeserializeTransaction error on bad input
	_, err = DeserializeTransaction([]byte("invalid_data"))
	assert.Error(t, err)

	// Test DeserializeOutputs panic on bad input
	assert.Panics(t, func() {
		DeserializeOutputs([]byte("invalid_data"))
	})
}
