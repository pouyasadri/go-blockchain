package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
)

type ScriptType uint8

const (
	ScriptTypeP2PKH  ScriptType = 0 // Standard pay-to-pubkey-hash
	ScriptTypeEscrow ScriptType = 1 // Data-hash-lock escrow
	ScriptTypeZKCP   ScriptType = 2 // Zero-Knowledge Contingent Payment
)

// TXOutput represents a transaction output
type TXOutput struct {
	Value           int64
	PubKeyHash      []byte
	ScriptType      ScriptType
	DataHashLock    []byte // SHA-256 hash that seller must satisfy
	TimeoutBlock    int64  // Block height after which buyer can reclaim
	BuyerPubKeyHash []byte // Buyer's pubkey hash for timeout refund
}

// Lock signs the output
func (out *TXOutput) Lock(address []byte) error {
	pubKeyHash := Base58Decode(address)
	if len(pubKeyHash) < addressChecksumLen+1 {
		return fmt.Errorf("invalid address: too short after base58 decode")
	}
	out.PubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]
	return nil
}

// IsLockedWithKey checks if the output can be used by the owner of the pubkey
func (out *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

// IsBuyer checks if the output buyer matches the pubkey
func (out *TXOutput) IsBuyer(pubKeyHash []byte) bool {
	return bytes.Equal(out.BuyerPubKeyHash, pubKeyHash)
}

// NewTXOutput create a new TXOutput
func NewTXOutput(value int64, address string) *TXOutput {
	if value <= 0 {
		panic("NewTXOutput: value must be strictly positive")
	}

	txo := &TXOutput{
		Value:      value,
		PubKeyHash: nil,
		ScriptType: ScriptTypeP2PKH,
	}
	if err := txo.Lock([]byte(address)); err != nil {
		// Lock should only fail for malformed addresses, which are validated upstream.
		panic(fmt.Sprintf("NewTXOutput: invalid address %q: %v", address, err))
	}

	return txo
}

// NewEscrowOutput creates a new escrow-locked output
func NewEscrowOutput(value int64, sellerAddress string, buyerAddress string, dataHash []byte, timeoutBlock int64) *TXOutput {
	if value <= 0 {
		panic("NewEscrowOutput: value must be strictly positive")
	}

	sellerPubKeyHash := Base58Decode([]byte(sellerAddress))
	if len(sellerPubKeyHash) < addressChecksumLen+1 {
		panic(fmt.Sprintf("NewEscrowOutput: invalid seller address %q", sellerAddress))
	}
	sellerPKH := sellerPubKeyHash[1 : len(sellerPubKeyHash)-addressChecksumLen]

	buyerPubKeyHash := Base58Decode([]byte(buyerAddress))
	if len(buyerPubKeyHash) < addressChecksumLen+1 {
		panic(fmt.Sprintf("NewEscrowOutput: invalid buyer address %q", buyerAddress))
	}
	buyerPKH := buyerPubKeyHash[1 : len(buyerPubKeyHash)-addressChecksumLen]

	return &TXOutput{
		Value:           value,
		PubKeyHash:      sellerPKH,
		ScriptType:      ScriptTypeEscrow,
		DataHashLock:    dataHash,
		TimeoutBlock:    timeoutBlock,
		BuyerPubKeyHash: buyerPKH,
	}
}

// CanClaimEscrow checks if a claimant can claim the escrowed funds
func (out *TXOutput) CanClaimEscrow(deliveredData []byte, claimerPubKeyHash []byte) bool {
	if out.ScriptType != ScriptTypeEscrow {
		return false
	}
	h := sha256.Sum256(deliveredData)
	return bytes.Equal(h[:], out.DataHashLock) && bytes.Equal(claimerPubKeyHash, out.PubKeyHash)
}

// CanRefundEscrow checks if the buyer can claim a refund after the timeout
func (out *TXOutput) CanRefundEscrow(currentHeight int64, claimerPubKeyHash []byte) bool {
	if out.ScriptType != ScriptTypeEscrow && out.ScriptType != ScriptTypeZKCP {
		return false
	}
	return currentHeight > out.TimeoutBlock && bytes.Equal(claimerPubKeyHash, out.BuyerPubKeyHash)
}

// NewZKCPOutput creates a new ZKCP locked output
func NewZKCPOutput(value int64, sellerAddress string, buyerAddress string, hashLock []byte, timeoutBlock int64) *TXOutput {
	txo := NewEscrowOutput(value, sellerAddress, buyerAddress, hashLock, timeoutBlock)
	txo.ScriptType = ScriptTypeZKCP
	return txo
}

// CanClaimZKCP checks if a claimant can claim ZKCP escrowed funds
func (out *TXOutput) CanClaimZKCP(deliveredData []byte, claimerPubKeyHash []byte) bool {
	if out.ScriptType != ScriptTypeZKCP {
		return false
	}
	h := sha256.Sum256(deliveredData)
	return bytes.Equal(h[:], out.DataHashLock) && bytes.Equal(claimerPubKeyHash, out.PubKeyHash)
}

// TXOutputs collects TXOutput
type TXOutputs struct {
	Outputs []TXOutput
}

// Serialize serializes TXOutputs
func (outs TXOutputs) Serialize() []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	if err := enc.Encode(outs); err != nil {
		// Encoding a plain struct should never fail; if it does something
		// is fundamentally wrong with the runtime.
		panic(fmt.Sprintf("TXOutputs.Serialize: gob encode failed: %v", err))
	}

	return buff.Bytes()
}

// DeserializeOutputs deserializes TXOutputs
func DeserializeOutputs(data []byte) TXOutputs {
	var outputs TXOutputs

	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&outputs); err != nil {
		panic(fmt.Sprintf("DeserializeOutputs: gob decode failed: %v", err))
	}

	return outputs
}
