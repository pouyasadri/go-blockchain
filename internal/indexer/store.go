package indexer

import (
	"encoding/hex"
	"sync"
	"time"
)

// EscrowStatus represents the current state of an escrow transaction
type EscrowStatus string

const (
	EscrowStatusPending  EscrowStatus = "PENDING"
	EscrowStatusClaimed  EscrowStatus = "CLAIMED"
	EscrowStatusRefunded EscrowStatus = "REFUNDED"
	EscrowStatusExpired  EscrowStatus = "EXPIRED"
)

// IndexedEscrow tracks detailed HTLC escrow state
type IndexedEscrow struct {
	TxID            string       `json:"tx_id"`
	Vout            int          `json:"vout"`
	Amount          int64        `json:"amount"`
	SellerPubKeyHash string      `json:"seller_pub_key_hash"`
	BuyerPubKeyHash  string      `json:"buyer_pub_key_hash"`
	DataHashLock    string       `json:"data_hash_lock"`
	TimeoutBlock    int64        `json:"timeout_block"`
	Status          EscrowStatus `json:"status"`
	CreatedAtHeight int64        `json:"created_at_height"`
	ClaimedTxID     string       `json:"claimed_tx_id,omitempty"`
	PreimageHex     string       `json:"preimage_hex,omitempty"`
	RefundedTxID    string       `json:"refunded_tx_id,omitempty"`
}

// AgentService represents an AI Agent service offered in the marketplace catalog
type AgentService struct {
	ID           string    `json:"id"`
	AgentAddress string    `json:"agent_address"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	EndpointURL  string    `json:"endpoint_url"`
	PricePerCall int64     `json:"price_per_call"` // micro-cents
	RegisteredAt time.Time `json:"registered_at"`
	Active       bool      `json:"active"`
}

// Metrics tracks network activity summaries
type Metrics struct {
	TotalBlocksIndexed int64 `json:"total_blocks_indexed"`
	TotalTxCount       int64 `json:"total_tx_count"`
	TotalEscrowsCount  int64 `json:"total_escrows_count"`
	TotalVolume        int64 `json:"total_volume"`
}

// RecentBlock represents a summary of a recently indexed block
type RecentBlock struct {
	Height    int64     `json:"height"`
	Hash      string    `json:"hash"`
	TxCount   int       `json:"tx_count"`
	Timestamp time.Time `json:"timestamp"`
}

// IndexStore provides a thread-safe repository for indexed blockchain data
type IndexStore struct {
	mu            sync.RWMutex
	escrows       map[string]*IndexedEscrow // key: txid:vout
	services      map[string]*AgentService  // key: service ID
	agentServices map[string][]string     // key: agent address -> list of service IDs
	recentBlocks  []*RecentBlock
	metrics       Metrics
	latestHeight  int64
}

// NewIndexStore initializes a new index store
func NewIndexStore() *IndexStore {
	return &IndexStore{
		escrows:       make(map[string]*IndexedEscrow),
		services:      make(map[string]*AgentService),
		agentServices: make(map[string][]string),
		recentBlocks:  make([]*RecentBlock, 0),
	}
}

func escrowKey(txID string, vout int) string {
	return txID + ":" + hex.EncodeToString([]byte{byte(vout)})
}

// SaveEscrow adds or updates an escrow entry
func (s *IndexStore) SaveEscrow(escrow *IndexedEscrow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := escrowKey(escrow.TxID, escrow.Vout)
	if _, exists := s.escrows[key]; !exists {
		s.metrics.TotalEscrowsCount++
	}
	s.escrows[key] = escrow
}

// GetEscrow retrieves an escrow by txid and output index
func (s *IndexStore) GetEscrow(txID string, vout int) (*IndexedEscrow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := escrowKey(txID, vout)
	e, ok := s.escrows[key]
	return e, ok
}

// GetActiveEscrows returns all escrows with PENDING status
func (s *IndexStore) GetActiveEscrows() []*IndexedEscrow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*IndexedEscrow
	for _, e := range s.escrows {
		if e.Status == EscrowStatusPending {
			list = append(list, e)
		}
	}
	return list
}

// SaveService registers or updates a service listing
func (s *IndexStore) SaveService(srv *AgentService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[srv.ID] = srv
	s.agentServices[srv.AgentAddress] = append(s.agentServices[srv.AgentAddress], srv.ID)
}

// GetService returns a service by ID
func (s *IndexStore) GetService(id string) (*AgentService, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.services[id]
	return srv, ok
}

// ListServices returns all active service listings
func (s *IndexStore) ListServices() []*AgentService {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*AgentService
	for _, srv := range s.services {
		if srv.Active {
			list = append(list, srv)
		}
	}
	return list
}

// UpdateMetrics updates overall indexing metrics
func (s *IndexStore) UpdateMetrics(blockHeight int64, txCount int, volume int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latestHeight = blockHeight
	s.metrics.TotalBlocksIndexed++
	s.metrics.TotalTxCount += int64(txCount)
	s.metrics.TotalVolume += volume
}

// GetMetrics returns snapshot of metrics
func (s *IndexStore) GetMetrics() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

// SaveBlock stores a summary of a newly indexed block (keeping up to 20 recent blocks)
func (s *IndexStore) SaveBlock(blk *RecentBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recentBlocks = append([]*RecentBlock{blk}, s.recentBlocks...)
	if len(s.recentBlocks) > 20 {
		s.recentBlocks = s.recentBlocks[:20]
	}
}

// GetRecentBlocks returns a slice of recently indexed blocks
func (s *IndexStore) GetRecentBlocks() []*RecentBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*RecentBlock, len(s.recentBlocks))
	copy(result, s.recentBlocks)
	return result
}
