package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"sync"
	"time"
)

const (
	// Compile-time safety limits for firewall/node fallback
	MaxTransactionSizeLimitBytes = 1024 * 1024            // 1 MB maximum transaction payload size
	MaxTransactionValueLimit     = int64(100_000_000_000) // 1,000,000.00000 currency units (10^11 micro-cents)
)

var (
	authorizedKeysMu sync.RWMutex
	authorizedKeys   [][]byte
)

// AddAuthorizedKey adds a new validator public key
func AddAuthorizedKey(pubKey []byte) {
	authorizedKeysMu.Lock()
	defer authorizedKeysMu.Unlock()
	
	// Check for duplicates
	for _, k := range authorizedKeys {
		if bytes.Equal(k, pubKey) {
			return
		}
	}
	authorizedKeys = append(authorizedKeys, pubKey)
}

// SetAuthorizedKeys replaces the authorized keys list
func SetAuthorizedKeys(keys [][]byte) {
	authorizedKeysMu.Lock()
	defer authorizedKeysMu.Unlock()
	authorizedKeys = keys
}

// IsAuthorizedKey checks if a public key is authorized
func IsAuthorizedKey(pubKey []byte) bool {
	authorizedKeysMu.RLock()
	defer authorizedKeysMu.RUnlock()
	for _, key := range authorizedKeys {
		if bytes.Equal(key, pubKey) {
			return true
		}
	}
	return false
}

// SignHash computes the hash of the block fields that are signed
func (b *Block) SignHash() []byte {
	txHash := b.HashTransactions()
	data := bytes.Join(
		[][]byte{
			b.PrevBlockHash,
			txHash,
			IntToHex(b.Timestamp),
			IntToHex(int64(b.Height)),
			IntToHex(int64(b.Bits)),
		},
		[]byte{},
	)
	h := sha256.Sum256(data)
	return h[:]
}

// NewBlockPoA creates a block signed by a validator key instead of running PoW
func NewBlockPoA(transactions []*Transaction, prevBlockHash []byte, height int, bits int, authorityKey *ecdsa.PrivateKey) (*Block, error) {
	if bits == 0 {
		bits = targetBits
	}
	block := &Block{
		Timestamp:          time.Now().Unix(),
		Transactions:       transactions,
		PrevBlockHash:      prevBlockHash,
		Hash:               []byte{},
		Nonce:              0,
		Height:             height,
		Bits:               bits,
		AuthoritySignature: nil,
		AuthorityPubKey:    nil,
	}

	// Prepare public key encoding
	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	authorityKey.X.FillBytes(xBytes)
	authorityKey.Y.FillBytes(yBytes)
	pubKeyBytes := make([]byte, 64)
	copy(pubKeyBytes, xBytes)
	copy(pubKeyBytes[32:], yBytes)
	block.AuthorityPubKey = pubKeyBytes

	// Hash representation
	signHash := block.SignHash()
	block.Hash = signHash

	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, authorityKey, signHash)
	if err != nil {
		return nil, err
	}

	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)
	r.FillBytes(rBytes)
	s.FillBytes(sBytes)
	block.AuthoritySignature = append(rBytes, sBytes...)

	return block, nil
}

// VerifyPoABlock checks the signature of a block against authorized keys
func VerifyPoABlock(block *Block) bool {
	if len(block.AuthoritySignature) != 64 || len(block.AuthorityPubKey) != 64 {
		return false
	}

	// Check if the signer is in the authorized keys list
	if !IsAuthorizedKey(block.AuthorityPubKey) {
		return false
	}

	curve := elliptic.P256()
	x := new(big.Int).SetBytes(block.AuthorityPubKey[:32])
	y := new(big.Int).SetBytes(block.AuthorityPubKey[32:])
	pubKey := ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	r := new(big.Int).SetBytes(block.AuthoritySignature[:32])
	s := new(big.Int).SetBytes(block.AuthoritySignature[32:])

	signHash := block.SignHash()
	return ecdsa.Verify(&pubKey, signHash, r, s)
}
