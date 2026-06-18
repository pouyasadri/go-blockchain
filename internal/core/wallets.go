package core

import (
	"bytes"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"os"
)

const walletFile = "wallet_%s.dat"

// Wallets stores a collection of wallets
type Wallets struct {
	Wallets map[string]*Wallet
}

// SerializableWallet is used for gob encoding because ecdsa.PrivateKey cannot be gob-encoded natively in Go 1.25
type SerializableWallet struct {
	PrivateKey []byte
	PublicKey  []byte
}

// NewWallets creates Wallets and fills it from a file if it exists
func NewWallets(nodeID string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	err := wallets.LoadFromFile(nodeID)
	// It's ok if file doesn't exist initially
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return &wallets, nil
}

// CreateWallet adds a Wallet to Wallets
func (ws *Wallets) CreateWallet() (string, error) {
	wallet, err := NewWallet()
	if err != nil {
		return "", err
	}
	address := fmt.Sprintf("%s", wallet.GetAddress())

	ws.Wallets[address] = wallet

	return address, nil
}

// GetAddresses returns an array of addresses stored in the wallet file
func (ws *Wallets) GetAddresses() []string {
	var addresses []string

	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

// GetWallet returns a Wallet by its address
func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

// LoadFromFile loads wallets from the file
func (ws *Wallets) LoadFromFile(nodeID string) error {
	wFile := fmt.Sprintf(walletFile, nodeID)
	if _, err := os.Stat(wFile); os.IsNotExist(err) {
		return err
	}

	fileContent, err := os.ReadFile(wFile)
	if err != nil {
		return fmt.Errorf("failed to read wallet file: %w", err)
	}

	var serialized map[string]SerializableWallet
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&serialized)
	if err != nil {
		return fmt.Errorf("failed to decode wallet file: %w", err)
	}

	for addr, sw := range serialized {
		priv, err := x509.ParseECPrivateKey(sw.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		ws.Wallets[addr] = &Wallet{
			PrivateKey: *priv,
			PublicKey:  sw.PublicKey,
		}
	}

	return nil
}

// SaveToFile saves wallets to a file
func (ws Wallets) SaveToFile(nodeID string) error {
	var content bytes.Buffer
	wFile := fmt.Sprintf(walletFile, nodeID)

	serialized := make(map[string]SerializableWallet)
	for addr, w := range ws.Wallets {
		privBytes, err := x509.MarshalECPrivateKey(&w.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to marshal private key: %w", err)
		}
		serialized[addr] = SerializableWallet{
			PrivateKey: privBytes,
			PublicKey:  w.PublicKey,
		}
	}

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(serialized)
	if err != nil {
		return fmt.Errorf("failed to encode wallets: %w", err)
	}

	err = os.WriteFile(wFile, content.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write wallet file: %w", err)
	}

	return nil
}
