package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// TXOutput represents a transaction output
type TXOutput struct {
	Value      int
	PubKeyHash []byte
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

// NewTXOutput create a new TXOutput
func NewTXOutput(value int, address string) *TXOutput {
	txo := &TXOutput{
		Value:      value,
		PubKeyHash: nil,
	}
	if err := txo.Lock([]byte(address)); err != nil {
		// Lock should only fail for malformed addresses, which are validated upstream.
		panic(fmt.Sprintf("NewTXOutput: invalid address %q: %v", address, err))
	}

	return txo
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
