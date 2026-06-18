package core

import (
	"encoding/hex"
	"fmt"

	"github.com/pouyasadri/go-blockchain/internal/storage"
)

// UTXOSet represents UTXO set
type UTXOSet struct {
	Blockchain *Blockchain
}

// FindSpendableOutputs finds and returns unspent outputs to reference in inputs
func (u UTXOSet) FindSpendableOutputs(pubkeyHash []byte, amount int) (int, map[string][]int, error) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.DB()

	err := db.IterateUTXO(func(k, v []byte) bool {
		txID := hex.EncodeToString(k)
		outs := DeserializeOutputs(v)

		for outIdx, out := range outs.Outputs {
			if out.IsLockedWithKey(pubkeyHash) && accumulated < amount {
				accumulated += out.Value
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
			}
		}
		// If we accumulated enough, stop iterating
		if accumulated >= amount {
			return false
		}
		return true
	})

	if err != nil {
		return 0, nil, fmt.Errorf("failed to iterate utxos: %w", err)
	}

	return accumulated, unspentOutputs, nil
}

// FindUTXO finds UTXO for a public key hash
func (u UTXOSet) FindUTXO(pubKeyHash []byte) ([]TXOutput, error) {
	var UTXOs []TXOutput
	db := u.Blockchain.DB()

	err := db.IterateUTXO(func(k, v []byte) bool {
		outs := DeserializeOutputs(v)
		for _, out := range outs.Outputs {
			if out.IsLockedWithKey(pubKeyHash) {
				UTXOs = append(UTXOs, out)
			}
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate utxos: %w", err)
	}

	return UTXOs, nil
}

// CountTransactions returns the number of transactions in the UTXO set
func (u UTXOSet) CountTransactions() (int, error) {
	db := u.Blockchain.DB()
	counter := 0

	err := db.IterateUTXO(func(k, v []byte) bool {
		counter++
		return true
	})

	if err != nil {
		return 0, fmt.Errorf("failed to count utxos: %w", err)
	}

	return counter, nil
}

// Reindex rebuilds the UTXO set
func (u UTXOSet) Reindex() error {
	db := u.Blockchain.DB()

	err := db.ClearUTXOSet()
	if err != nil {
		return fmt.Errorf("failed to clear utxo set: %w", err)
	}

	UTXO := u.Blockchain.FindUTXO()

	for txID, outs := range UTXO {
		key, err := hex.DecodeString(txID)
		if err != nil {
			return fmt.Errorf("failed to decode tx id: %w", err)
		}

		err = db.SaveUTXO(key, outs.Serialize())
		if err != nil {
			return fmt.Errorf("failed to save utxo: %w", err)
		}
	}

	return nil
}

// Update updates the UTXO set with transactions from the Block
// The Block is considered to be the tip of a blockchain
func (u UTXOSet) Update(block *Block) error {
	db := u.Blockchain.DB()

	for _, tx := range block.Transactions {
		if !tx.IsCoinbase() {
			for _, vin := range tx.Vin {
				updatedOuts := TXOutputs{}
				outsBytes, err := db.GetUTXO(vin.Txid)
				if err != nil {
					// UTXO might already be missing, skip or error?
					if err == storage.ErrNotFound {
						continue
					}
					return fmt.Errorf("failed to get utxo: %w", err)
				}

				outs := DeserializeOutputs(outsBytes)
				for outIdx, out := range outs.Outputs {
					if outIdx != vin.Vout {
						updatedOuts.Outputs = append(updatedOuts.Outputs, out)
					}
				}

				if len(updatedOuts.Outputs) == 0 {
					err := db.DeleteUTXO(vin.Txid)
					if err != nil {
						return fmt.Errorf("failed to delete utxo: %w", err)
					}
				} else {
					err := db.SaveUTXO(vin.Txid, updatedOuts.Serialize())
					if err != nil {
						return fmt.Errorf("failed to update utxo: %w", err)
					}
				}
			}
		}

		newOutputs := TXOutputs{}
		newOutputs.Outputs = append(newOutputs.Outputs, tx.Vout...)

		err := db.SaveUTXO(tx.ID, newOutputs.Serialize())
		if err != nil {
			return fmt.Errorf("failed to save new utxos: %w", err)
		}
	}

	return nil
}
