package indexer

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/core"
)

// Indexer parses raw blocks and builds searchable index states
type Indexer struct {
	Store *IndexStore
}

// NewIndexer creates a new indexer instance
func NewIndexer(store *IndexStore) *Indexer {
	if store == nil {
		store = NewIndexStore()
	}
	return &Indexer{
		Store: store,
	}
}

// ProcessBlock indexes transactions within a single block
func (idx *Indexer) ProcessBlock(block *core.Block) error {
	var blockVolume int64 = 0

	for _, tx := range block.Transactions {
		txIDHex := hex.EncodeToString(tx.ID)

		// 1. Process Outputs (Escrow creation & Service registration)
		for outIdx, out := range tx.Vout {
			blockVolume += out.Value

			if out.ScriptType == core.ScriptTypeEscrow || out.ScriptType == core.ScriptTypeZKCP {
				escrow := &IndexedEscrow{
					TxID:            txIDHex,
					Vout:            outIdx,
					Amount:          out.Value,
					SellerPubKeyHash: hex.EncodeToString(out.PubKeyHash),
					BuyerPubKeyHash:  hex.EncodeToString(out.BuyerPubKeyHash),
					DataHashLock:    hex.EncodeToString(out.DataHashLock),
					TimeoutBlock:    out.TimeoutBlock,
					Status:          EscrowStatusPending,
					CreatedAtHeight: int64(block.Height),
				}
				idx.Store.SaveEscrow(escrow)
			}
		}

		// 2. Process Inputs (Escrow claims & refunds)
		for _, in := range tx.Vin {
			prevTxIDHex := hex.EncodeToString(in.Txid)
			if escrow, ok := idx.Store.GetEscrow(prevTxIDHex, int(in.Vout)); ok {
				if len(in.EscrowWitness) > 0 {
					// Claimed with secret preimage
					escrow.Status = EscrowStatusClaimed
					escrow.ClaimedTxID = txIDHex
					escrow.PreimageHex = hex.EncodeToString(in.EscrowWitness)
					idx.Store.SaveEscrow(escrow)
				} else if in.IsRefund {
					// Refunded after timeout
					escrow.Status = EscrowStatusRefunded
					escrow.RefundedTxID = txIDHex
					idx.Store.SaveEscrow(escrow)
				}
			}
		}
	}

	// 3. Mark expired escrows based on current block height
	for _, escrow := range idx.Store.GetActiveEscrows() {
		if int64(block.Height) >= escrow.TimeoutBlock {
			escrow.Status = EscrowStatusExpired
			idx.Store.SaveEscrow(escrow)
		}
	}

	idx.Store.SaveBlock(&RecentBlock{
		Height:    int64(block.Height),
		Hash:      hex.EncodeToString(block.Hash),
		TxCount:   len(block.Transactions),
		Timestamp: time.Unix(block.Timestamp, 0),
	})

	idx.Store.UpdateMetrics(int64(block.Height), len(block.Transactions), blockVolume)
	return nil
}

// RegisterServiceOffer allows direct indexing of off-chain or on-chain service offer declarations
func (idx *Indexer) RegisterServiceOffer(agentAddr, name, description, endpointURL string, pricePerCall int64) (*AgentService, error) {
	if !core.ValidateAddress(agentAddr) {
		return nil, fmt.Errorf("invalid agent address")
	}
	if name == "" || endpointURL == "" || pricePerCall <= 0 {
		return nil, fmt.Errorf("invalid service parameters")
	}

	serviceID := fmt.Sprintf("srv_%s_%d", strings.ToLower(name), time.Now().UnixNano())
	srv := &AgentService{
		ID:           serviceID,
		AgentAddress: agentAddr,
		Name:         name,
		Description:  description,
		EndpointURL:  endpointURL,
		PricePerCall: pricePerCall,
		RegisteredAt: time.Now(),
		Active:       true,
	}

	idx.Store.SaveService(srv)
	return srv, nil
}
