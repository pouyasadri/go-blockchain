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
