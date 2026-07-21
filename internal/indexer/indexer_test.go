package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestIndexerEscrowLifecycle(t *testing.T) {
	store := NewIndexStore()
	idx := NewIndexer(store)

	sellerWallet, err := core.NewWallet()
	assert.NoError(t, err)
	buyerWallet, err := core.NewWallet()
	assert.NoError(t, err)

	preimage := []byte("secret_payment_preimage_123")
	hashLock := sha256.Sum256(preimage)

	// 1. Create Escrow Output
	escrowOut := core.NewEscrowOutput(
		1000,
		string(sellerWallet.GetAddress()),
		string(buyerWallet.GetAddress()),
		hashLock[:],
		100,
	)

	txin := core.TXInput{Txid: []byte("coinbase"), Vout: 0}
	escrowTx := core.Transaction{
		ID:   []byte("tx_escrow_1"),
		Vin:  []core.TXInput{txin},
		Vout: []core.TXOutput{*escrowOut},
	}

	block1 := &core.Block{
		Height:       10,
		Transactions: []*core.Transaction{&escrowTx},
	}

	err = idx.ProcessBlock(block1)
	assert.NoError(t, err)

	// Verify escrow stored as PENDING
	escrow, found := store.GetEscrow(hex.EncodeToString(escrowTx.ID), 0)
	assert.True(t, found)
	assert.Equal(t, EscrowStatusPending, escrow.Status)
	assert.Equal(t, int64(1000), escrow.Amount)

	// 2. Claim Escrow
	claimTxInput := core.TXInput{
		Txid:          escrowTx.ID,
		Vout:          0,
		Signature:     nil,
		PubKey:        sellerWallet.PublicKey,
		EscrowWitness: preimage,
		IsRefund:      false,
	}
	claimTxOut := core.NewTXOutput(1000, string(sellerWallet.GetAddress()))
	claimTx := &core.Transaction{
		ID:   []byte("tx_claim_1"),
		Vin:  []core.TXInput{claimTxInput},
		Vout: []core.TXOutput{*claimTxOut},
	}

	block2 := &core.Block{
		Height:       11,
		Transactions: []*core.Transaction{claimTx},
	}

	err = idx.ProcessBlock(block2)
	assert.NoError(t, err)

	// Verify escrow status changed to CLAIMED
	escrow, found = store.GetEscrow(hex.EncodeToString(escrowTx.ID), 0)
	assert.True(t, found)
	assert.Equal(t, EscrowStatusClaimed, escrow.Status)
	assert.Equal(t, hex.EncodeToString(preimage), escrow.PreimageHex)
}

func TestIndexerServiceCatalog(t *testing.T) {
	idx := NewIndexer(nil)
	w, err := core.NewWallet()
	assert.NoError(t, err)
	addr := string(w.GetAddress())

	srv, err := idx.RegisterServiceOffer(
		addr,
		"GPT4-MicroTask",
		"Fast text summarize",
		"https://agent-a.ai/api",
		50,
	)
	assert.NoError(t, err)
	assert.NotEmpty(t, srv.ID)

	services := idx.Store.ListServices()
	assert.Len(t, services, 1)
	assert.Equal(t, "GPT4-MicroTask", services[0].Name)
}
