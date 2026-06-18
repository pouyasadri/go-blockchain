package core

import (
	"path/filepath"
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
)

func TestBlockchainAndUTXOIntegration(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	// 1. Create a wallet and blockchain
	wallet, _ := NewWallet()
	address := string(wallet.GetAddress())

	bc, err := CreateBlockchain(address, db)
	assert.NoError(t, err)
	assert.NotNil(t, bc)

	// Best height should be 0 (genesis block)
	bestHeight, err := bc.GetBestHeight()
	assert.NoError(t, err)
	assert.Equal(t, 0, bestHeight)

	// 2. Initialize UTXO set
	utxoSet := UTXOSet{Blockchain: bc}
	err = utxoSet.Reindex()
	assert.NoError(t, err)

	// The genesis block should give the wallet some balance
	pubKeyHash := HashPubKey(wallet.PublicKey)
	utxos, err := utxoSet.FindUTXO(pubKeyHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, utxos)

	// 3. Send some coins to a new wallet
	wallet2, _ := NewWallet()
	addr2 := string(wallet2.GetAddress())

	tx, err := NewUTXOTransaction(wallet, addr2, 2, &utxoSet)
	assert.NoError(t, err)

	cbTx, _ := NewCoinbaseTX(address, "")
	newBlock, err := bc.MineBlock([]*Transaction{cbTx, tx})
	assert.NoError(t, err)
	assert.NotNil(t, newBlock)

	err = utxoSet.Update(newBlock)
	assert.NoError(t, err)

	err = utxoSet.Reindex()
	assert.NoError(t, err)

	// Verify height
	bestHeight, err = bc.GetBestHeight()
	assert.NoError(t, err)
	assert.Equal(t, 1, bestHeight)

	// 4. Verify blocks via iter.Seq
	count := 0
	for block := range bc.Blocks() {
		count++
		assert.NotNil(t, block)
	}
	assert.Equal(t, 2, count) // Genesis + newly mined block

	// 5. Test FindTransaction
	foundTx, err := bc.FindTransaction(tx.ID)
	assert.NoError(t, err)
	assert.Equal(t, tx.ID, foundTx.ID)
}

func TestGetBlockAndHashes(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test2.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	bc, _ := CreateBlockchain(addr, db)

	hashes := bc.GetBlockHashes()
	assert.NotEmpty(t, hashes)

	block, err := bc.GetBlock(hashes[0])
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.Equal(t, hashes[0], block.Hash)
}

func TestNewBlockchainAndAddBlock(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test3.db")

	// 1. NewBlockchain on non-existent DB should fail
	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)

	bc, err := NewBlockchain(db)
	assert.Error(t, err)
	assert.Nil(t, bc)

	// 2. Create Blockchain
	wallet, err := NewWallet()
	assert.NoError(t, err)
	addr := string(wallet.GetAddress())

	bc, err = CreateBlockchain(addr, db)
	assert.NoError(t, err)
	assert.NotNil(t, bc)

	// 3. NewBlockchain on existing DB should succeed
	bc2, err := NewBlockchain(db)
	assert.NoError(t, err)
	assert.NotNil(t, bc2)
	assert.Equal(t, bc.tip, bc2.tip)

	// 4. Test AddBlock
	// Let's create a new block
	prevTip := bc.tip
	bestHeight, err := bc.GetBestHeight()
	assert.NoError(t, err)

	tx, err := NewCoinbaseTX(addr, "")
	assert.NoError(t, err)
	txs := []*Transaction{tx}
	newBlock := NewBlock(txs, prevTip, bestHeight+1)

	// Add block to database
	err = bc.AddBlock(newBlock)
	assert.NoError(t, err)

	// Verify tip is updated
	newTip, err := db.GetTip()
	assert.NoError(t, err)
	assert.Equal(t, newBlock.Hash, newTip)
	assert.Equal(t, newBlock.Hash, bc.tip)

	// Try adding the same block again (should do nothing/return nil)
	err = bc.AddBlock(newBlock)
	assert.NoError(t, err)

	// Try adding a block with a lower height
	lowerBlock := NewBlock(txs, prevTip, bestHeight)
	err = bc.AddBlock(lowerBlock)
	assert.NoError(t, err)

	// Verify tip remains at newBlock.Hash
	assert.Equal(t, newBlock.Hash, bc.tip)

	// Close database
	assert.NoError(t, db.Close())
}

func TestBlockchainErrors(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test_err.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	wallet, _ := NewWallet()
	addr := string(wallet.GetAddress())

	bc, err := CreateBlockchain(addr, db)
	assert.NoError(t, err)

	// 1. CreateBlockchain failing when blockchain already exists
	_, err = CreateBlockchain(addr, db)
	assert.Error(t, err)

	// 2. FindTransaction failing when not found
	_, err = bc.FindTransaction([]byte("nonexistent_tx"))
	assert.Error(t, err)

	// 3. GetBlock failing when not found
	_, err = bc.GetBlock([]byte("nonexistent_block"))
	assert.Error(t, err)

	// 4. MineBlock failing with invalid transaction
	invalidTx := &Transaction{
		ID:  []byte("invalid_tx"),
		Vin: []TXInput{{Txid: []byte("parent"), Vout: 0}},
	}
	_, err = bc.MineBlock([]*Transaction{invalidTx})
	assert.Error(t, err)
}
