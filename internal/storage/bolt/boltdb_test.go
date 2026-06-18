package bolt

import (
	"path/filepath"
	"testing"

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

	err = db.IterateUTXO(func(k, v []byte) bool {
		assert.Equal(t, []byte("txid"), k)
		assert.Equal(t, []byte("txouts"), v)
		return true
	})
	assert.NoError(t, err)

	// Test DeleteUTXO
	err = db.DeleteUTXO([]byte("txid"))
	assert.NoError(t, err)

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
}
