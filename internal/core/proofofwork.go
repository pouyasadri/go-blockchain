package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/big"
)

const (
	maxNonce   = math.MaxInt64
	targetBits = 16
)

// ProofOfWork represents a proof-of-work
type ProofOfWork struct {
	block  *Block
	target *big.Int
}

// NewProofOfWork builds and returns a ProofOfWork
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{
		block:  b,
		target: target,
	}

	return pow
}

func (pow *ProofOfWork) prepareData(nonce int, txHash []byte) []byte {
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			txHash,
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)

	return data
}

// Run performs a proof-of-work
func (pow *ProofOfWork) Run() (int, []byte, error) {
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
