package mcp

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/pouyasadri/go-blockchain/internal/core"
)

// Keyring manages ephemeral wallets for the MCP daemon session.
type Keyring struct {
	mu      sync.RWMutex
	keys    map[string]*core.Wallet // address -> wallet
	primary string                  // primary session wallet address
}

// NewKeyring creates a new keyring and generates an initial ephemeral wallet.
func NewKeyring() (*Keyring, error) {
	kr := &Keyring{
		keys: make(map[string]*core.Wallet),
	}

	wallet, err := core.NewWallet()
	if err != nil {
		return nil, fmt.Errorf("failed to generate initial ephemeral wallet: %w", err)
	}

	addr := string(wallet.GetAddress())
	kr.keys[addr] = wallet
	kr.primary = addr

	return kr, nil
}

// GenerateNewWallet creates another ephemeral wallet and returns its address.
func (kr *Keyring) GenerateNewWallet() (string, error) {
	wallet, err := core.NewWallet()
	if err != nil {
		return "", fmt.Errorf("failed to generate ephemeral wallet: %w", err)
	}

	addr := string(wallet.GetAddress())
	
	kr.mu.Lock()
	kr.keys[addr] = wallet
	kr.mu.Unlock()

	return addr, nil
}

// GetPrimaryAddress returns the primary wallet address.
func (kr *Keyring) GetPrimaryAddress() string {
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	return kr.primary
}

// GetWallet returns a wallet by address, if it exists in the keyring.
func (kr *Keyring) GetWallet(address string) (*core.Wallet, error) {
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	
	wallet, exists := kr.keys[address]
	if !exists {
		return nil, fmt.Errorf("wallet %s not found in keyring", address)
	}
	return wallet, nil
}

// SignTransaction signs all inputs of a transaction using the appropriate keys from the keyring.
func (kr *Keyring) SignTransaction(tx *core.Transaction, prevTXs map[string]core.Transaction) error {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	// In a complete implementation, this would look up the correct key for each input
	// based on the previous output's PubKeyHash.
	// For simplicity in the MCP Daemon Phase 2, we assume all inputs are spent by a key we hold,
	// and we will attempt to find the right wallet.
	
	for i, vin := range tx.Vin {
		prevTx, ok := prevTXs[hex.EncodeToString(vin.Txid)]
		if !ok {
			return fmt.Errorf("previous transaction %x not found", vin.Txid)
		}
		
		prevOut := prevTx.Vout[vin.Vout]
		pubKeyHash := prevOut.PubKeyHash
		if vin.IsRefund && prevOut.ScriptType == core.ScriptTypeEscrow {
			pubKeyHash = prevOut.BuyerPubKeyHash
		}
		
		// Find matching wallet
		var matchedWallet *core.Wallet
		for _, w := range kr.keys {
			if string(core.HashPubKey(w.PublicKey)) == string(pubKeyHash) {
				matchedWallet = w
				break
			}
		}
		
		if matchedWallet == nil {
			return fmt.Errorf("no key found in keyring to spend input %d", i)
		}
		
		err := tx.Sign(matchedWallet.PrivateKey, prevTXs)
		if err != nil {
			return fmt.Errorf("failed to sign input %d: %w", i, err)
		}
		// Since tx.Sign signs ALL inputs using the SAME private key (a limitation of the current core.Transaction.Sign),
		// we break early. The core node assumes a single signer for the whole transaction.
		break
	}

	return nil
}
