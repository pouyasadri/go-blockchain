package bolt

import (
	"fmt"

	"github.com/pouyasadri/go-blockchain/internal/storage"
	bolt "go.etcd.io/bbolt"
)

const (
	blocksBucket = "blocks"
	utxoBucket   = "chainstate"
	tipKey       = "l"
)

// DB represents a BoltDB storage implementation
type DB struct {
	db *bolt.DB
}

// Open creates a new BoltDB connection
func Open(dbFile string) (*DB, error) {
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	// Ensure buckets exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(blocksBucket))
		if err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte(utxoBucket))
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the bolt database
func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) SaveBlock(hash []byte, blockData []byte) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return storage.ErrNotFound
		}
		return b.Put(hash, blockData)
	})
}

// SaveBlockAndTip atomically saves a block and updates the chain tip
// in a single BoltDB transaction, preventing inconsistent state on crash.
func (d *DB) SaveBlockAndTip(hash []byte, blockData []byte) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return storage.ErrNotFound
		}
		if err := b.Put(hash, blockData); err != nil {
			return err
		}
		return b.Put([]byte(tipKey), hash)
	})
}

func (d *DB) GetBlock(hash []byte) ([]byte, error) {
	var blockData []byte
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return storage.ErrNotFound
		}
		data := b.Get(hash)
		if data == nil {
			return storage.ErrNotFound
		}
		// bolt DB returns a byte slice that is only valid for the life of the transaction.
		// we must copy it.
		blockData = make([]byte, len(data))
		copy(blockData, data)
		return nil
	})
	return blockData, err
}

func (d *DB) GetTip() ([]byte, error) {
	var tip []byte
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return storage.ErrNotFound
		}
		data := b.Get([]byte(tipKey))
		if data == nil {
			return storage.ErrNotFound
		}
		tip = make([]byte, len(data))
		copy(tip, data)
		return nil
	})
	return tip, err
}

func (d *DB) UpdateTip(hash []byte) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		return b.Put([]byte(tipKey), hash)
	})
}

func (d *DB) GetUTXO(txID []byte) ([]byte, error) {
	var outsData []byte
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		if b == nil {
			return storage.ErrNotFound
		}
		data := b.Get(txID)
		if data == nil {
			return storage.ErrNotFound
		}
		outsData = make([]byte, len(data))
		copy(outsData, data)
		return nil
	})
	return outsData, err
}

func (d *DB) SaveUTXO(txID []byte, outsData []byte) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		return b.Put(txID, outsData)
	})
}

func (d *DB) DeleteUTXO(txID []byte) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		return b.Delete(txID)
	})
}

func (d *DB) ClearUTXOSet() error {
	return d.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(utxoBucket))
		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err = tx.CreateBucket([]byte(utxoBucket))
		return err
	})
}

func (d *DB) IterateUTXO(yield func(k, v []byte) bool) error {
	return d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// Bolt DB slices are only valid for the transaction, but we yield them synchronously
			// We can yield them directly if the caller doesn't keep them across iterations,
			// or we can copy. Copying is safer.
			kc := make([]byte, len(k))
			copy(kc, k)
			vc := make([]byte, len(v))
			copy(vc, v)
			if !yield(kc, vc) {
				break
			}
		}
		return nil
	})
}
