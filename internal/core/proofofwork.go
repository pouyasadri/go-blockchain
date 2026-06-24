package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/big"
)

const (
	maxNonce                     = math.MaxInt64
	targetBits                   = 16
	difficultyAdjustmentInterval = 10  // Adjust difficulty every 10 blocks
	targetBlockTimeSeconds       = 600 // Target 10 minutes per block (600 seconds)
)

// ProofOfWork represents a proof-of-work
type ProofOfWork struct {
	block  *Block
	target *big.Int
}

// NewProofOfWork builds and returns a ProofOfWork
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	bits := b.Bits
	if bits == 0 {
		bits = targetBits
	}
	target.Lsh(target, uint(256-bits))

	pow := &ProofOfWork{
		block:  b,
		target: target,
	}

	return pow
}

func (pow *ProofOfWork) prepareData(nonce int, txHash []byte) []byte {
	bits := pow.block.Bits
	if bits == 0 {
		bits = targetBits
	}
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			txHash,
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(bits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)

	return data
}

// Run performs a proof-of-work
func (pow *ProofOfWork) Run(ctx context.Context) (int, []byte, error) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0

	// Cache the transaction hash once to avoid rebuilding the Merkle tree on every iteration
	txHash := pow.block.HashTransactions()
	if txHash == nil {
		return 0, nil, errors.New("failed to compute transaction hash")
	}

	fmt.Printf("Mining a new block")
	for nonce < maxNonce {
		// Check for context cancellation every 1000 iterations to avoid select statement overhead
		if nonce%1000 == 0 {
			select {
			case <-ctx.Done():
				fmt.Print("\nMining cancelled\n")
				return 0, nil, ctx.Err()
			default:
			}
		}

		data := pow.prepareData(nonce, txHash)

		hash = sha256.Sum256(data)
		if nonce%100000 == 0 {
			fmt.Printf("\r%x", hash)
		}
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {
			break
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")

	return nonce, hash[:], nil
}

// Validate validates block's PoW
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	txHash := pow.block.HashTransactions()
	data := pow.prepareData(pow.block.Nonce, txHash)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1

	return isValid
}

// CalculateNewDifficulty calculates the new difficulty target bits.
// Since we store difficulty in bits (leading zeroes), each increment
// doubles the difficulty. We adjust by +/-1 bit to prevent wild shifts.
func CalculateNewDifficulty(lastBlock, anchorBlock *Block) int {
	if lastBlock.Height%difficultyAdjustmentInterval != 0 {
		return lastBlock.Bits
	}

	actualTime := lastBlock.Timestamp - anchorBlock.Timestamp
	targetTime := int64(difficultyAdjustmentInterval * targetBlockTimeSeconds)

	newBits := lastBlock.Bits
	if actualTime < targetTime/2 {
		// Blocks mined too fast -> increase difficulty (more leading zeroes)
		newBits++
	} else if actualTime > targetTime*2 {
		// Blocks mined too slow -> decrease difficulty (fewer leading zeroes)
		newBits--
	}

	// Prevent difficulty from dropping too low or rising too high for safety/demo
	if newBits < 8 {
		newBits = 8
	}
	if newBits > 24 {
		newBits = 24
	}

	return newBits
}
