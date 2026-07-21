package core

import "bytes"

// TXInput represents a transaction input
type TXInput struct {
	Txid          []byte
	Vout          int
	Signature     []byte
	PubKey        []byte
	EscrowWitness []byte // The delivered data proving SHA256 match
	IsRefund      bool   // True if this is a timeout refund claim
}

// UsesKey checks whether the address initiated the transaction
func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := HashPubKey(in.PubKey)

	return bytes.Equal(lockingHash, pubKeyHash)
}
