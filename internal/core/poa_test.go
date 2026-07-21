package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"path/filepath"
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
)

func TestPoAConsensus(t *testing.T) {
	// 1. Generate authority key pair
	curve := elliptic.P256()
	authorityKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	assert.NoError(t, err)

	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	authorityKey.X.FillBytes(xBytes)
	authorityKey.Y.FillBytes(yBytes)
	pubKeyBytes := append(xBytes, yBytes...)

	// Create an unauthorized key
	unauthorizedKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	assert.NoError(t, err)

	// Set authorized validator keys
	SetAuthorizedKeys([][]byte{pubKeyBytes})

	// Create mock transactions
	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())
	cbtx, err := NewCoinbaseTX(addr, "POA block")
	assert.NoError(t, err)

	// 2. Create PoA Block
	prevHash := []byte("previous_block_hash_placeholder_123")
	poaBlock, err := NewBlockPoA([]*Transaction{cbtx}, prevHash, 1, 0, authorityKey)
	assert.NoError(t, err)
	assert.NotNil(t, poaBlock)
	assert.Equal(t, pubKeyBytes, poaBlock.AuthorityPubKey)
	assert.NotEmpty(t, poaBlock.AuthoritySignature)

	// 3. Verify PoA Block
	assert.True(t, VerifyPoABlock(poaBlock))

	// Rejects if using empty signature
	emptySigBlock := *poaBlock
	emptySigBlock.AuthoritySignature = nil
	assert.False(t, VerifyPoABlock(&emptySigBlock))

	// Rejects if authority key not authorized
	SetAuthorizedKeys([][]byte{})
	assert.False(t, VerifyPoABlock(poaBlock))
	SetAuthorizedKeys([][]byte{pubKeyBytes})

	// Rejects block produced by unauthorized key
	unauthBlock, err := NewBlockPoA([]*Transaction{cbtx}, prevHash, 1, 0, unauthorizedKey)
	assert.NoError(t, err)
	assert.False(t, VerifyPoABlock(unauthBlock))
}

func TestAddBlockPoA(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	bc, err := CreateBlockchain(addr, db)
	assert.NoError(t, err)

	// Set up PoA authority key
	curve := elliptic.P256()
	authKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	assert.NoError(t, err)

	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	authKey.X.FillBytes(xBytes)
	authKey.Y.FillBytes(yBytes)
	pubKeyBytes := append(xBytes, yBytes...)

	SetAuthorizedKeys([][]byte{pubKeyBytes})

	// Create transaction
	cbtx, err := NewCoinbaseTX(addr, "PoA Sync")
	assert.NoError(t, err)

	tip, err := bc.GetBlock(bc.tip)
	assert.NoError(t, err)

	// Produce PoA block
	poaBlock, err := NewBlockPoA([]*Transaction{cbtx}, tip.Hash, tip.Height+1, tip.Bits, authKey)
	assert.NoError(t, err)

	// Adding valid PoA block to blockchain should succeed
	err = bc.AddBlock(poaBlock)
	assert.NoError(t, err)

	// Verify tip updated
	newTip, err := bc.GetBlock(bc.tip)
	assert.NoError(t, err)
	assert.Equal(t, poaBlock.Hash, newTip.Hash)
	assert.Equal(t, tip.Height+1, newTip.Height)
}
