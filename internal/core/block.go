package core

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"time"
)

// Block represents a block in the blockchain
type Block struct {
	Timestamp     int64
	Transactions  []*Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
	Height        int
}

// NewBlock creates and returns Block
func NewBlock(transactions []*Transaction, prevBlockHash []byte, height int) *Block {
	block := &Block{
		Timestamp:     time.Now().Unix(),
		Transactions:  transactions,
		PrevBlockHash: prevBlockHash,
		Hash:          []byte{},
		Nonce:         0,
		Height:        height,
	}
	pow := NewProofOfWork(block)
	nonce, hash, err := pow.Run()
	if err != nil {
		// Proof-of-work failure is unrecoverable during block construction.
		// This should only happen if the transaction set is empty.
		panic(fmt.Sprintf("proof-of-work failed: %v", err))
	}

	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

// NewGenesisBlock creates and returns genesis Block
func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{}, 0)
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() []byte {
	if len(b.Transactions) == 0 {
		return nil
	}
	var transactions [][]byte
	for _, tx := range b.Transactions {
		serialized := tx.Serialize()
		if serialized == nil {
			return nil
		}
		transactions = append(transactions, serialized)
	}
	mTree := NewMerkleTree(transactions)
	if mTree == nil || mTree.RootNode == nil {
		return nil
	}

	return mTree.RootNode.Data
}

// Serialize serializes the block
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		// encoding a struct should not fail under normal circumstances
		return nil
	}

	return result.Bytes()
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) (*Block, error) {
	if len(d) == 0 {
		return nil, errors.New("cannot deserialize empty block data")
	}
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %w", err)
	}

	return &block, nil
}
