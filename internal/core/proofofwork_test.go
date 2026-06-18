package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProofOfWork(t *testing.T) {
	// Create a mock transaction and block
	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	txin := TXInput{[]byte{}, -1, nil, wallet.PublicKey}
	txout := NewTXOutput(10, addr)
	tx := &Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()

	block := NewBlock([]*Transaction{tx}, []byte{}, 0)

	pow := NewProofOfWork(block)
	assert.NotNil(t, pow)

	isValid := pow.Validate()
	assert.True(t, isValid)
}
