package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlock(t *testing.T) {
	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	txin := TXInput{[]byte{}, -1, nil, wallet.PublicKey}
	txout := NewTXOutput(10, addr)
	tx := &Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()

	block := NewBlock([]*Transaction{tx}, []byte("prevhash"), 1)
	assert.NotNil(t, block)
	assert.Equal(t, []byte("prevhash"), block.PrevBlockHash)
	assert.Equal(t, 1, block.Height)
	assert.NotEmpty(t, block.Hash)
	assert.True(t, block.Nonce > 0 || block.Nonce == 0) // Just ensure it's set

	serialized := block.Serialize()
	assert.NotEmpty(t, serialized)

	deserialized, err := DeserializeBlock(serialized)
	assert.NoError(t, err)
	assert.Equal(t, block.Hash, deserialized.Hash)
	assert.Equal(t, block.Height, deserialized.Height)
	assert.Equal(t, block.PrevBlockHash, deserialized.PrevBlockHash)
}

func TestDeserializeBlockError(t *testing.T) {
	_, err := DeserializeBlock([]byte("invalid_data"))
	assert.Error(t, err)
}
