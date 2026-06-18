package bolt

import (
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/pouyasadri/go-blockchain/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestBoltDBStorage(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := Open(dbFile)
	assert.NoError(t, err)
	assert.NotNil(t, db)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	// Test GetTip initially
	tip, err := db.GetTip()
	assert.Error(t, err) // bucket shouldn't exist or no 'l' key
	assert.Empty(t, tip)

	// Test UpdateTip
	hash := []byte("lasthash")
	err = db.UpdateTip(hash)
	assert.NoError(t, err)

	// Test GetTip when tip exists
	tip, err = db.GetTip()
	assert.NoError(t, err)
	assert.Equal(t, hash, tip)

	// Test SaveBlock
	err = db.SaveBlock([]byte("blockhash"), []byte("blockdata"))
	assert.NoError(t, err)

	// Test GetBlock
	data, err := db.GetBlock([]byte("blockhash"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("blockdata"), data)

	// Test SaveUTXO and IterateUTXO
	err = db.SaveUTXO([]byte("txid"), []byte("txouts"))
	assert.NoError(t, err)

	// GetUTXO check
	utxoData, err := db.GetUTXO([]byte("txid"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("txouts"), utxoData)

	err = db.IterateUTXO(func(k, v []byte) bool {
		assert.Equal(t, []byte("txid"), k)
		assert.Equal(t, []byte("txouts"), v)
		return true
	})
	assert.NoError(t, err)

	// Test DeleteUTXO
	err = db.DeleteUTXO([]byte("txid"))
	assert.NoError(t, err)

	// GetUTXO check on deleted item
	_, err = db.GetUTXO([]byte("txid"))
	assert.ErrorIs(t, err, storage.ErrNotFound)

	// Test Iterate again to ensure deletion
	count := 0
	err = db.IterateUTXO(func(k, v []byte) bool {
		count++
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Test ClearUTXOSet
	err = db.SaveUTXO([]byte("txid2"), []byte("txouts2"))
	assert.NoError(t, err)
	err = db.ClearUTXOSet()
	assert.NoError(t, err)
	count = 0
	err = db.IterateUTXO(func(k, v []byte) bool {
		count++
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Test GetBlock for nonexistent key
	_, err = db.GetBlock([]byte("nonexistent_hash"))
	assert.ErrorIs(t, err, storage.ErrNotFound)

	// Test IterateUTXO breaking early
	err = db.ClearUTXOSet()
	assert.NoError(t, err)
	err = db.SaveUTXO([]byte("k1"), []byte("v1"))
	assert.NoError(t, err)
	err = db.SaveUTXO([]byte("k2"), []byte("v2"))
	assert.NoError(t, err)

	iterCount := 0
	err = db.IterateUTXO(func(k, v []byte) bool {
		iterCount++
		return false // break early
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, iterCount)

	// Test GetTip error when blocks bucket doesn't exist
	err = db.db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte("blocks"))
	})
	assert.NoError(t, err)

	_, err = db.GetTip()
	assert.ErrorIs(t, err, storage.ErrNotFound)

	// Test GetBlock when blocks bucket doesn't exist
	_, err = db.GetBlock([]byte("somehash"))
	assert.ErrorIs(t, err, storage.ErrNotFound)

	err = db.db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte(utxoBucket))
	})
	assert.NoError(t, err)

	_, err = db.GetUTXO([]byte("somehash"))
	assert.ErrorIs(t, err, storage.ErrNotFound)

	// Test Open error
	_, err = Open("/nonexistent_dir_1234/test.db")
	assert.Error(t, err)
}
