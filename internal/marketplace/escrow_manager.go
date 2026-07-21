package marketplace

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/pouyasadri/go-blockchain/internal/indexer"
)

// EscrowManager manages HTLC negotiation and resolution workflows
type EscrowManager struct {
	Store *indexer.IndexStore
}

// NewEscrowManager creates a new EscrowManager instance
func NewEscrowManager(store *indexer.IndexStore) *EscrowManager {
	return &EscrowManager{
		Store: store,
	}
}

// HTLCDeal represents a negotiated micro-task escrow deal
type HTLCDeal struct {
	Preimage  []byte `json:"preimage"`
	HashLock  []byte `json:"hash_lock"`
	Amount    int64  `json:"amount"`
	Timeout   int64  `json:"timeout"`
	BuyerAddr string `json:"buyer_addr"`
	SellerAddr string `json:"seller_addr"`
}

// CreateHTLCSecret generates a random 32-byte secret preimage and its SHA-256 hashlock
func (em *EscrowManager) CreateHTLCSecret() ([]byte, []byte, error) {
	preimage := make([]byte, 32)
	_, err := rand.Read(preimage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate random secret: %w", err)
	}

	hashLock := sha256.Sum256(preimage)
	return preimage, hashLock[:], nil
}

// BuildEscrowTransaction constructs a new HTLC escrow transaction
func (em *EscrowManager) BuildEscrowTransaction(
	buyerWallet *core.Wallet,
	sellerAddress string,
	amount int64,
	hashLock []byte,
	timeoutBlock int64,
	utxoSet *core.UTXOSet,
) (*core.Transaction, error) {
	if !core.ValidateAddress(sellerAddress) {
		return nil, fmt.Errorf("invalid seller address")
	}

	return core.NewEscrowTransaction(
		buyerWallet,
		sellerAddress,
		amount,
		hashLock,
		timeoutBlock,
		utxoSet,
	)
}

// BuildClaimTransaction constructs a transaction claiming funds from an escrow using secret preimage
func (em *EscrowManager) BuildClaimTransaction(
	sellerWallet *core.Wallet,
	escrowTxID []byte,
	escrowVout int,
	preimage []byte,
	blockchain *core.Blockchain,
) (*core.Transaction, error) {
	return core.NewEscrowClaimTransaction(
		sellerWallet,
		escrowTxID,
		escrowVout,
		preimage,
		blockchain,
	)
}

// BuildRefundTransaction constructs a transaction refunding funds back to buyer post timeout
func (em *EscrowManager) BuildRefundTransaction(
	buyerWallet *core.Wallet,
	escrowTxID []byte,
	escrowVout int,
	blockchain *core.Blockchain,
) (*core.Transaction, error) {
	return core.NewEscrowRefundTransaction(
		buyerWallet,
		escrowTxID,
		escrowVout,
		blockchain,
	)
}

// BuildZKCPTransaction constructs a new ZKCP escrow transaction
func (em *EscrowManager) BuildZKCPTransaction(
	buyerWallet *core.Wallet,
	sellerAddress string,
	amount int64,
	hashLock []byte,
	timeoutBlock int64,
	utxoSet *core.UTXOSet,
) (*core.Transaction, error) {
	if !core.ValidateAddress(sellerAddress) {
		return nil, fmt.Errorf("invalid seller address")
	}

	buyerPubKeyHash := core.HashPubKey(buyerWallet.PublicKey)
	buyerAddress := string(buyerWallet.GetAddress())

	accumulated, validOutputs, err := utxoSet.FindSpendableOutputs(buyerPubKeyHash, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to find spendable outputs: %w", err)
	}

	if accumulated < amount {
		return nil, fmt.Errorf("insufficient buyer balance: have %d, need %d", accumulated, amount)
	}

	var inputs []core.TXInput
	var outputs []core.TXOutput

	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tx string: %w", err)
		}
		for _, out := range outs {
			inputs = append(inputs, core.TXInput{
				Txid:          txID,
				Vout:          out,
				Signature:     nil,
				PubKey:        buyerWallet.PublicKey,
				EscrowWitness: nil,
				IsRefund:      false,
			})
		}
	}

	zkcpOutput := core.NewZKCPOutput(amount, sellerAddress, buyerAddress, hashLock, timeoutBlock)
	outputs = append(outputs, *zkcpOutput)

	if accumulated > amount {
		outputs = append(outputs, *core.NewTXOutput(accumulated-amount, buyerAddress))
	}

	tx := core.Transaction{
		ID:   nil,
		Vin:  inputs,
		Vout: outputs,
	}
	tx.ID = tx.Hash()

	err = utxoSet.Blockchain.SignTransaction(&tx, buyerWallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ZKCP transaction: %w", err)
	}

	return &tx, nil
}
