package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"strings"

	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
)

const subsidy = int64(10)

// Transaction represents a transaction
type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

// IsCoinbase checks whether the transaction is coinbase
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

// Serialize returns a serialized Transaction
func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		// encoding a struct should not fail under normal circumstances
		return nil
	}

	return encoded.Bytes()
}

// Hash returns the hash of the Transaction
func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}

	hash = sha256.Sum256(txCopy.Serialize())

	return hash[:]
}

// Sign signs each input of a Transaction
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) error {
	if tx.IsCoinbase() {
		return nil
	}

	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil {
			return errors.New("previous transaction is not correct")
		}
	}

	txCopy := tx.TrimmedCopy()

	for inID, vin := range txCopy.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		prevOut := prevTx.Vout[vin.Vout]
		txCopy.Vin[inID].Signature = nil

		expectedPubKeyHash := prevOut.PubKeyHash
		if tx.Vin[inID].IsRefund && prevOut.ScriptType == ScriptTypeEscrow {
			expectedPubKeyHash = prevOut.BuyerPubKeyHash
		}
		txCopy.Vin[inID].PubKey = expectedPubKeyHash

		// Use the canonical hash of the trimmed copy as the message to sign
		dataToSign := txCopy.Hash()

		r, s, err := ecdsa.Sign(rand.Reader, &privKey, dataToSign)
		if err != nil {
			return fmt.Errorf("failed to sign transaction: %w", err)
		}
		// Zero-pad r and s to exactly 32 bytes each (P-256 curve).
		rBytes := make([]byte, 32)
		sBytes := make([]byte, 32)
		r.FillBytes(rBytes)
		s.FillBytes(sBytes)
		tx.Vin[inID].Signature = append(rBytes, sBytes...)
		txCopy.Vin[inID].PubKey = nil
	}

	return nil
}

// String returns a human-readable representation of a transaction
func (tx Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))

	for i, input := range tx.Vin {
		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
		if len(input.EscrowWitness) > 0 {
			lines = append(lines, fmt.Sprintf("       Witness:   %x", input.EscrowWitness))
		}
		if input.IsRefund {
			lines = append(lines, "       IsRefund:  true")
		}
	}

	for i, output := range tx.Vout {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
		lines = append(lines, fmt.Sprintf("       Type:   %d", output.ScriptType))
	}

	return strings.Join(lines, "\n")
}

// TrimmedCopy creates a trimmed copy of Transaction to be used in signing
func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	for _, vin := range tx.Vin {
		inputs = append(inputs, TXInput{vin.Txid, vin.Vout, nil, nil, nil, vin.IsRefund})
	}

	for _, vout := range tx.Vout {
		outputs = append(outputs, TXOutput{
			Value:           vout.Value,
			PubKeyHash:      vout.PubKeyHash,
			ScriptType:      vout.ScriptType,
			DataHashLock:    vout.DataHashLock,
			TimeoutBlock:    vout.TimeoutBlock,
			BuyerPubKeyHash: vout.BuyerPubKeyHash,
		})
	}

	txCopy := Transaction{tx.ID, inputs, outputs}

	return txCopy
}

// Verify verifies signatures and escrow conditions of Transaction inputs
func (tx *Transaction) Verify(prevTXs map[string]Transaction, currentBlockHeight int64) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil {
			return false
		}
	}

	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()

	var inputSum int64
	var outputSum int64

	for _, out := range tx.Vout {
		if out.Value <= 0 {
			return false // Outputs must be strictly positive
		}
		outputSum += out.Value
	}

	for inID, vin := range tx.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		prevOut := prevTx.Vout[vin.Vout]
		txCopy.Vin[inID].Signature = nil

		expectedPubKeyHash := prevOut.PubKeyHash
		if vin.IsRefund && prevOut.ScriptType == ScriptTypeEscrow {
			expectedPubKeyHash = prevOut.BuyerPubKeyHash
		}
		txCopy.Vin[inID].PubKey = expectedPubKeyHash

		// Signatures are stored as 32-byte zero-padded r || s (P-256 curve).
		if len(vin.Signature) != 64 {
			return false
		}
		r := new(big.Int).SetBytes(vin.Signature[:32])
		s := new(big.Int).SetBytes(vin.Signature[32:])

		// Public keys are stored as 32-byte zero-padded x || y (P-256 curve).
		if len(vin.PubKey) != 64 {
			return false
		}
		x := new(big.Int).SetBytes(vin.PubKey[:32])
		y := new(big.Int).SetBytes(vin.PubKey[32:])

		// Verify against the same canonical hash used during signing.
		dataToVerify := txCopy.Hash()

		rawPubKey := ecdsa.PublicKey{Curve: curve, X: x, Y: y}
		if !ecdsa.Verify(&rawPubKey, dataToVerify, r, s) {
			return false
		}

		// Verify additional script constraints for escrow and ZKCP outputs
		if prevOut.ScriptType == ScriptTypeEscrow || prevOut.ScriptType == ScriptTypeZKCP {
			claimerPubKeyHash := HashPubKey(vin.PubKey)
			if vin.IsRefund {
				if !prevOut.CanRefundEscrow(currentBlockHeight, claimerPubKeyHash) {
					return false
				}
			} else if prevOut.ScriptType == ScriptTypeEscrow {
				if !prevOut.CanClaimEscrow(vin.EscrowWitness, claimerPubKeyHash) {
					return false
				}
			} else if prevOut.ScriptType == ScriptTypeZKCP {
				if !prevOut.CanClaimZKCP(vin.EscrowWitness, claimerPubKeyHash) {
					return false
				}
			}
		}

		txCopy.Vin[inID].PubKey = nil
		inputSum += prevOut.Value
	}

	if inputSum < outputSum {
		return false // Inflation detected
	}

	return true
}

// NewCoinbaseTX creates a new coinbase transaction
func NewCoinbaseTX(to, data string) (*Transaction, error) {
	if data == "" {
		randData := make([]byte, 20)
		_, err := rand.Read(randData)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random data: %w", err)
		}

		data = fmt.Sprintf("%x", randData)
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data), nil, false}
	txout := NewTXOutput(subsidy, to)
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()

	return &tx, nil
}

// NewUTXOTransaction creates a new transaction
func NewUTXOTransaction(wallet *Wallet, to string, amount int64, UTXOSet *UTXOSet) (*Transaction, error) {
	var inputs []TXInput
	var outputs []TXOutput

	pubKeyHash := HashPubKey(wallet.PublicKey)
	acc, validOutputs, err := UTXOSet.FindSpendableOutputs(pubKeyHash, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to find spendable outputs: %w", err)
	}

	if acc < amount {
		return nil, errors.New("not enough funds")
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tx string: %w", err)
		}

		for _, out := range outs {
			input := TXInput{
				Txid:          txID,
				Vout:          out,
				Signature:     nil,
				PubKey:        wallet.PublicKey,
				EscrowWitness: nil,
				IsRefund:      false,
			}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	from := string(wallet.GetAddress())
	outputs = append(outputs, *NewTXOutput(amount, to))
	if acc > amount {
		outputs = append(outputs, *NewTXOutput(acc-amount, from)) // a change
	}

	tx := Transaction{
		ID:   nil,
		Vin:  inputs,
		Vout: outputs,
	}
	tx.ID = tx.Hash()
	err = UTXOSet.Blockchain.SignTransaction(&tx, wallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return &tx, nil
}

// NewEscrowTransaction creates a new escrow transaction
func NewEscrowTransaction(buyerWallet *Wallet, sellerAddr string, amount int64, dataHash []byte, timeoutBlock int64, utxoSet *UTXOSet) (*Transaction, error) {
	var inputs []TXInput
	var outputs []TXOutput

	pubKeyHash := HashPubKey(buyerWallet.PublicKey)
	acc, validOutputs, err := utxoSet.FindSpendableOutputs(pubKeyHash, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to find spendable outputs: %w", err)
	}

	if acc < amount {
		return nil, errors.New("not enough funds")
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tx string: %w", err)
		}

		for _, out := range outs {
			input := TXInput{
				Txid:          txID,
				Vout:          out,
				Signature:     nil,
				PubKey:        buyerWallet.PublicKey,
				EscrowWitness: nil,
				IsRefund:      false,
			}
			inputs = append(inputs, input)
		}
	}

	// Build the escrow output
	buyerAddr := string(buyerWallet.GetAddress())
	escrowOut := NewEscrowOutput(amount, sellerAddr, buyerAddr, dataHash, timeoutBlock)
	outputs = append(outputs, *escrowOut)

	// Add a change output if we accumulated more than amount
	if acc > amount {
		outputs = append(outputs, *NewTXOutput(acc-amount, buyerAddr))
	}

	tx := Transaction{
		ID:   nil,
		Vin:  inputs,
		Vout: outputs,
	}
	tx.ID = tx.Hash()

	err = utxoSet.Blockchain.SignTransaction(&tx, buyerWallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return &tx, nil
}

// NewEscrowClaimTransaction creates a transaction that spends an escrow output by providing the matching data
func NewEscrowClaimTransaction(sellerWallet *Wallet, escrowTxID []byte, escrowOutIdx int, deliveredData []byte, blockchain *Blockchain) (*Transaction, error) {
	// Retrieve the escrow transaction
	escrowTx, err := blockchain.FindTransaction(escrowTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to find escrow transaction: %w", err)
	}

	if escrowOutIdx < 0 || escrowOutIdx >= len(escrowTx.Vout) {
		return nil, errors.New("invalid escrow output index")
	}
	escrowOut := escrowTx.Vout[escrowOutIdx]

	// Build the input spending the escrow output
	input := TXInput{
		Txid:          escrowTxID,
		Vout:          escrowOutIdx,
		Signature:     nil,
		PubKey:        sellerWallet.PublicKey,
		EscrowWitness: deliveredData,
		IsRefund:      false,
	}

	// Build the output sending the funds to the seller's address
	sellerAddr := string(sellerWallet.GetAddress())
	output := NewTXOutput(escrowOut.Value, sellerAddr)

	tx := Transaction{
		ID:   nil,
		Vin:  []TXInput{input},
		Vout: []TXOutput{*output},
	}
	tx.ID = tx.Hash()

	// Sign the transaction
	err = blockchain.SignTransaction(&tx, sellerWallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign claim transaction: %w", err)
	}

	return &tx, nil
}

// NewEscrowRefundTransaction creates a timeout refund transaction
func NewEscrowRefundTransaction(buyerWallet *Wallet, escrowTxID []byte, escrowOutIdx int, blockchain *Blockchain) (*Transaction, error) {
	// Retrieve the escrow transaction
	escrowTx, err := blockchain.FindTransaction(escrowTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to find escrow transaction: %w", err)
	}

	if escrowOutIdx < 0 || escrowOutIdx >= len(escrowTx.Vout) {
		return nil, errors.New("invalid escrow output index")
	}
	escrowOut := escrowTx.Vout[escrowOutIdx]

	// Build the input spending the escrow output with refund path
	input := TXInput{
		Txid:          escrowTxID,
		Vout:          escrowOutIdx,
		Signature:     nil,
		PubKey:        buyerWallet.PublicKey,
		EscrowWitness: nil,
		IsRefund:      true,
	}

	// Build the output sending the refund to the buyer
	buyerAddr := string(buyerWallet.GetAddress())
	output := NewTXOutput(escrowOut.Value, buyerAddr)

	tx := Transaction{
		ID:   nil,
		Vin:  []TXInput{input},
		Vout: []TXOutput{*output},
	}
	tx.ID = tx.Hash()

	// Sign the transaction
	err = blockchain.SignTransaction(&tx, buyerWallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refund transaction: %w", err)
	}

	return &tx, nil
}

// DeserializeTransaction deserializes a transaction
func DeserializeTransaction(data []byte) (Transaction, error) {
	var transaction Transaction

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	if err != nil {
		return transaction, fmt.Errorf("failed to deserialize transaction: %w", err)
	}

	return transaction, nil
}
