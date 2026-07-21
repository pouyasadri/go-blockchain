package core

import (
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
)

func TestEscrowMechanics(t *testing.T) {
	// Create wallets
	buyerWallet, _ := NewWallet()
	sellerWallet, _ := NewWallet()
	otherWallet, _ := NewWallet()

	buyerAddr := string(buyerWallet.GetAddress())
	sellerAddr := string(sellerWallet.GetAddress())

	buyerPubKeyHash := HashPubKey(buyerWallet.PublicKey)
	sellerPubKeyHash := HashPubKey(sellerWallet.PublicKey)
	otherPubKeyHash := HashPubKey(otherWallet.PublicKey)

	data := []byte("secret deliverable data")
	dataHash := sha256.Sum256(data)
	wrongData := []byte("wrong data")

	// 1. Test Escrow Output Creation
	out := NewEscrowOutput(100, sellerAddr, buyerAddr, dataHash[:], 10)
	assert.Equal(t, int64(100), out.Value)
	assert.Equal(t, ScriptTypeEscrow, out.ScriptType)
	assert.Equal(t, dataHash[:], out.DataHashLock)
	assert.Equal(t, int64(10), out.TimeoutBlock)
	assert.Equal(t, sellerPubKeyHash, out.PubKeyHash)
	assert.Equal(t, buyerPubKeyHash, out.BuyerPubKeyHash)

	// 2. Test CanClaimEscrow (Seller Releases)
	assert.True(t, out.CanClaimEscrow(data, sellerPubKeyHash))
	assert.False(t, out.CanClaimEscrow(wrongData, sellerPubKeyHash))
	assert.False(t, out.CanClaimEscrow(data, otherPubKeyHash))
	assert.False(t, out.CanClaimEscrow(data, buyerPubKeyHash))

	// 3. Test CanRefundEscrow (Buyer Refunds)
	// Before timeout
	assert.False(t, out.CanRefundEscrow(5, buyerPubKeyHash))
	assert.False(t, out.CanRefundEscrow(10, buyerPubKeyHash))
	// After timeout
	assert.True(t, out.CanRefundEscrow(11, buyerPubKeyHash))
	assert.False(t, out.CanRefundEscrow(11, sellerPubKeyHash))
	assert.False(t, out.CanRefundEscrow(11, otherPubKeyHash))
}

func TestEscrowTransactions(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	buyerWallet, _ := NewWallet()
	sellerWallet, _ := NewWallet()
	buyerAddr := string(buyerWallet.GetAddress())
	sellerAddr := string(sellerWallet.GetAddress())

	bc, err := CreateBlockchain(buyerAddr, db)
	assert.NoError(t, err)

	utxoSet := UTXOSet{Blockchain: bc}
	err = utxoSet.Reindex()
	assert.NoError(t, err)

	data := []byte("secret contract deliverable")
	dataHash := sha256.Sum256(data)
	var timeoutBlock int64 = 10

	// 1. Create Escrow Transaction (Buyer locks 5 coins)
	escrowTx, err := NewEscrowTransaction(buyerWallet, sellerAddr, 5, dataHash[:], timeoutBlock, &utxoSet)
	assert.NoError(t, err)
	assert.NotNil(t, escrowTx)

	// Save transaction by mining a block
	cbTx, err := NewCoinbaseTX(buyerAddr, "")
	assert.NoError(t, err)
	block, err := bc.MineBlock(t.Context(), []*Transaction{cbTx, escrowTx})
	assert.NoError(t, err)
	err = utxoSet.Update(block)
	assert.NoError(t, err)

	// 2. Test Claim Transaction (Seller satisfies hash lock)
	claimTx, err := NewEscrowClaimTransaction(sellerWallet, escrowTx.ID, 0, data, bc)
	assert.NoError(t, err)
	assert.NotNil(t, claimTx)

	// Verify claim transaction against blockchain (height is 1)
	valid, err := bc.VerifyTransaction(claimTx)
	assert.NoError(t, err)
	assert.True(t, valid)

	// Test invalid claim data
	invalidClaimTx, err := NewEscrowClaimTransaction(sellerWallet, escrowTx.ID, 0, []byte("invalid data"), bc)
	assert.NoError(t, err)
	valid, err = bc.VerifyTransaction(invalidClaimTx)
	assert.NoError(t, err)
	assert.False(t, valid)

	// 3. Test Refund Transaction (Buyer reclaims after timeout)
	refundTx, err := NewEscrowRefundTransaction(buyerWallet, escrowTx.ID, 0, bc)
	assert.NoError(t, err)
	assert.NotNil(t, refundTx)

	// Verify refund fails before timeout (current height is 1, timeout is 10)
	valid, err = bc.VerifyTransaction(refundTx)
	assert.NoError(t, err)
	assert.False(t, valid)

	// Mine 10 more blocks to pass timeout
	for i := 0; i < 10; i++ {
		cb, err := NewCoinbaseTX(buyerAddr, "")
		assert.NoError(t, err)
		blk, err := bc.MineBlock(t.Context(), []*Transaction{cb})
		assert.NoError(t, err)
		err = utxoSet.Update(blk)
		assert.NoError(t, err)
	}

	height, err := bc.GetBestHeight()
	assert.NoError(t, err)
	assert.True(t, int64(height) > timeoutBlock)

	// Verify refund succeeds after timeout
	valid, err = bc.VerifyTransaction(refundTx)
	assert.NoError(t, err)
	assert.True(t, valid)
}
