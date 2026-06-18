package storage

import "errors"

var (
	ErrNotFound = errors.New("key not found in storage")
)

// Storage defines the interface for the blockchain database.
type Storage interface {
	// Block operations
	SaveBlock(hash []byte, blockData []byte) error
	// SaveBlockAndTip atomically saves a block and updates the chain tip in one transaction.
	SaveBlockAndTip(hash []byte, blockData []byte) error
	GetBlock(hash []byte) ([]byte, error)
	GetTip() ([]byte, error)
	UpdateTip(hash []byte) error

	// UTXO operations
	GetUTXO(txID []byte) ([]byte, error)
	SaveUTXO(txID []byte, outsData []byte) error
	DeleteUTXO(txID []byte) error
	ClearUTXOSet() error

	// Iterators
	// IterateUTXO returns a sequence of UTXO items (key, value)
	// It uses Go 1.23+ range-over-func style iterators via a callback.
	IterateUTXO(yield func(k, v []byte) bool) error

	Close() error
}
