package core

import (
	"context"
	"testing"
	"time"

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

	block, err := NewBlock(context.Background(), []*Transaction{tx}, []byte{}, 0, 0)
	assert.NoError(t, err)

	pow := NewProofOfWork(block)
	assert.NotNil(t, pow)

	isValid := pow.Validate()
	assert.True(t, isValid)
}

func TestProofOfWorkCancellation(t *testing.T) {
	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	txin := TXInput{[]byte{}, -1, nil, wallet.PublicKey}
	txout := NewTXOutput(10, addr)
	tx := &Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()

	// High difficulty to ensure it won't finish mining immediately
	block := &Block{
		Timestamp:     time.Now().Unix(),
		Transactions:  []*Transaction{tx},
		PrevBlockHash: []byte{},
		Hash:          []byte{},
		Nonce:         0,
		Height:        0,
		Bits:          24, // Very high difficulty for a single CPU thread test
	}

	pow := NewProofOfWork(block)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 20 milliseconds
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, _, err := pow.Run(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCalculateNewDifficulty(t *testing.T) {
	// 1. Difficulty should remain the same if height not on interval
	block1 := &Block{Height: 5, Bits: 12, Timestamp: 1000}
	blockAnchor := &Block{Height: 0, Bits: 12, Timestamp: 0}
	bits := CalculateNewDifficulty(block1, blockAnchor)
	assert.Equal(t, 12, bits)

	// 2. Difficulty should increase (+1) if blocks mined too fast
	// targetBlockTimeSeconds = 600, difficultyAdjustmentInterval = 10 -> targetTime = 6000
	// If actual time is 1000 (< targetTime / 2 which is 3000)
	block2 := &Block{Height: 10, Bits: 12, Timestamp: 1000}
	bitsFast := CalculateNewDifficulty(block2, blockAnchor)
	assert.Equal(t, 13, bitsFast)

	// 3. Difficulty should decrease (-1) if blocks mined too slow
	// If actual time is 15000 (> targetTime * 2 which is 12000)
	block3 := &Block{Height: 10, Bits: 12, Timestamp: 15000}
	bitsSlow := CalculateNewDifficulty(block3, blockAnchor)
	assert.Equal(t, 11, bitsSlow)

	// 4. Clamping bounds (min 8, max 24)
	blockMin := &Block{Height: 10, Bits: 8, Timestamp: 15000}
	bitsMin := CalculateNewDifficulty(blockMin, blockAnchor)
	assert.Equal(t, 8, bitsMin)

	blockMax := &Block{Height: 10, Bits: 24, Timestamp: 1000}
	bitsMax := CalculateNewDifficulty(blockMax, blockAnchor)
	assert.Equal(t, 24, bitsMax)
}
