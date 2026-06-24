package core

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"

	"github.com/pouyasadri/go-blockchain/internal/storage"
)

const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

// Blockchain implements interactions with a DB
type Blockchain struct {
	tip []byte
	db  storage.Storage
}

// CreateBlockchain creates a new blockchain DB
func CreateBlockchain(address string, db storage.Storage) (*Blockchain, error) {
	tip, err := db.GetTip()
	if err == nil && len(tip) > 0 {
		return nil, errors.New("blockchain already exists")
	}

	cbtx, err := NewCoinbaseTX(address, genesisCoinbaseData)
	if err != nil {
		return nil, fmt.Errorf("failed to create coinbase tx: %w", err)
	}
	genesis := NewGenesisBlock(cbtx)

	err = db.SaveBlockAndTip(genesis.Hash, genesis.Serialize())
	if err != nil {
		return nil, fmt.Errorf("failed to save genesis block: %w", err)
	}

	bc := &Blockchain{tip: genesis.Hash, db: db}
	return bc, nil
}

// NewBlockchain creates a new Blockchain with genesis Block
func NewBlockchain(db storage.Storage) (*Blockchain, error) {
	tip, err := db.GetTip()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, errors.New("no existing blockchain found, create one first")
		}
		return nil, fmt.Errorf("failed to get tip: %w", err)
	}

	bc := &Blockchain{tip: tip, db: db}
	return bc, nil
}

// DB returns the underlying storage
func (bc *Blockchain) DB() storage.Storage {
	return bc.db
}

// AddBlock saves the block into the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
	_, err := bc.db.GetBlock(block.Hash)
	if err == nil {
		// Block already exists
		return nil
	}

	pow := NewProofOfWork(block)
	if !pow.Validate() {
		return errors.New("invalid block: proof-of-work failed")
	}

	blockData := block.Serialize()
	if blockData == nil {
		return errors.New("failed to serialize block")
	}

	lastHash, err := bc.db.GetTip()
	if err != nil {
		return fmt.Errorf("failed to get tip: %w", err)
	}

	lastBlockData, err := bc.db.GetBlock(lastHash)
	if err != nil {
		return fmt.Errorf("failed to get last block: %w", err)
	}

	lastBlock, err := DeserializeBlock(lastBlockData)
	if err != nil {
		return fmt.Errorf("failed to deserialize last block: %w", err)
	}

	if block.Height > lastBlock.Height {
		err = bc.db.SaveBlockAndTip(block.Hash, blockData)
		if err != nil {
			return fmt.Errorf("failed to save block and tip: %w", err)
		}
		bc.tip = block.Hash
	} else {
		err = bc.db.SaveBlock(block.Hash, blockData)
		if err != nil {
			return fmt.Errorf("failed to save block: %w", err)
		}
	}

	return nil
}

// Blocks returns an iterator over the blockchain blocks starting from the tip
func (bc *Blockchain) Blocks() iter.Seq[*Block] {
	return func(yield func(*Block) bool) {
		currentHash := bc.tip
		for len(currentHash) > 0 {
			blockData, err := bc.db.GetBlock(currentHash)
			if err != nil {
				// We stop iteration on error
				break
			}
			block, err := DeserializeBlock(blockData)
			if err != nil {
				break
			}

			if !yield(block) {
				return
			}
			currentHash = block.PrevBlockHash
		}
	}
}

// FindTransaction finds a transaction by its ID
func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	for block := range bc.Blocks() {
		for _, tx := range block.Transactions {
			if bytes.Equal(tx.ID, ID) {
				return *tx, nil
			}
		}
	}
	return Transaction{}, errors.New("transaction not found")
}

// FindUTXO finds all unspent transaction outputs and returns transactions with spent outputs removed
func (bc *Blockchain) FindUTXO() map[string]TXOutputs {
	UTXO := make(map[string]TXOutputs)
	spentTXOs := make(map[string][]int)

	for block := range bc.Blocks() {
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Vout {
				// Was the output spent?
				if spentTXOs[txID] != nil {
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx {
							continue Outputs
						}
					}
				}

				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			if !tx.IsCoinbase() {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}
	}

	return UTXO
}

// GetBestHeight returns the height of the latest block
func (bc *Blockchain) GetBestHeight() (int, error) {
	tip, err := bc.db.GetTip()
	if err != nil {
		return 0, err
	}
	blockData, err := bc.db.GetBlock(tip)
	if err != nil {
		return 0, err
	}
	lastBlock, err := DeserializeBlock(blockData)
	if err != nil {
		return 0, err
	}
	return lastBlock.Height, nil
}

// GetBlock finds a block by its hash and returns it
func (bc *Blockchain) GetBlock(blockHash []byte) (*Block, error) {
	blockData, err := bc.db.GetBlock(blockHash)
	if err != nil {
		return nil, err
	}
	return DeserializeBlock(blockData)
}

// GetBlockHashes returns a list of hashes of all the blocks in the chain
func (bc *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	for block := range bc.Blocks() {
		blocks = append(blocks, block.Hash)
	}
	return blocks
}

// MineBlock mines a new block with the provided transactions
func (bc *Blockchain) MineBlock(ctx context.Context, transactions []*Transaction) (*Block, error) {
	for _, tx := range transactions {
		valid, err := bc.VerifyTransaction(tx)
		if err != nil {
			return nil, fmt.Errorf("error verifying transaction: %w", err)
		}
		if !valid {
			return nil, errors.New("invalid transaction")
		}
	}

	lastHash, err := bc.db.GetTip()
	if err != nil {
		return nil, fmt.Errorf("failed to get tip: %w", err)
	}

	blockData, err := bc.db.GetBlock(lastHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get last block: %w", err)
	}
	lastBlock, err := DeserializeBlock(blockData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize last block: %w", err)
	}

	newBits := lastBlock.Bits
	if newBits == 0 {
		newBits = targetBits
	}
	newHeight := lastBlock.Height + 1
	if newHeight%difficultyAdjustmentInterval == 0 {
		// Find anchor block by traversing back
		anchorBlock := lastBlock
		for anchorBlock.Height > newHeight-difficultyAdjustmentInterval {
			prevBlockData, err := bc.db.GetBlock(anchorBlock.PrevBlockHash)
			if err != nil {
				return nil, fmt.Errorf("failed to get ancestor block: %w", err)
			}
			prevBlock, err := DeserializeBlock(prevBlockData)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize ancestor block: %w", err)
			}
			anchorBlock = prevBlock
		}
		newBits = CalculateNewDifficulty(lastBlock, anchorBlock)
	}

	newBlock, err := NewBlock(ctx, transactions, lastHash, newHeight, newBits)
	if err != nil {
		return nil, err
	}

	err = bc.db.SaveBlockAndTip(newBlock.Hash, newBlock.Serialize())
	if err != nil {
		return nil, fmt.Errorf("failed to save mined block: %w", err)
	}

	bc.tip = newBlock.Hash
	return newBlock, nil
}

// SignTransaction signs inputs of a Transaction
func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) error {
	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return fmt.Errorf("failed to find previous transaction: %w", err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Sign(privKey, prevTXs)
}

// VerifyTransaction verifies transaction input signatures
func (bc *Blockchain) VerifyTransaction(tx *Transaction) (bool, error) {
	if tx.IsCoinbase() {
		return true, nil
	}

	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return false, fmt.Errorf("failed to find previous transaction: %w", err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs), nil
}
