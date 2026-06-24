package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pouyasadri/go-blockchain/internal/core"
)

// ServerInterface specifies the P2P server methods needed by the API handlers
type ServerInterface interface {
	MempoolSnapshot() []core.Transaction
	GetKnownNodes() []string
}

// TransactionJSON represents a JSON-friendly transaction
type TransactionJSON struct {
	ID   string         `json:"id"`
	Vin  []TXInputJSON  `json:"vin"`
	Vout []TXOutputJSON `json:"vout"`
}

// TXInputJSON represents a JSON-friendly transaction input
type TXInputJSON struct {
	Txid      string `json:"txid"`
	Vout      int    `json:"vout"`
	Signature string `json:"signature,omitempty"`
	PubKey    string `json:"pubkey,omitempty"`
}

// TXOutputJSON represents a JSON-friendly transaction output
type TXOutputJSON struct {
	Value      int    `json:"value"`
	PubKeyHash string `json:"pubkey_hash"`
}

// BlockJSON represents a JSON-friendly block
type BlockJSON struct {
	Timestamp     int64             `json:"timestamp"`
	Transactions  []TransactionJSON `json:"transactions"`
	PrevBlockHash string            `json:"prev_block_hash"`
	Hash          string            `json:"hash"`
	Nonce         int               `json:"nonce"`
	Height        int               `json:"height"`
	Bits          int               `json:"bits"`
}

// HeightJSON represents a response containing block height information
type HeightJSON struct {
	Height  int    `json:"height"`
	TipHash string `json:"tip_hash"`
}

// BalanceJSON represents a balance response
type BalanceJSON struct {
	Address string `json:"address"`
	Balance int    `json:"balance"`
}

func mapTransaction(tx *core.Transaction) TransactionJSON {
	vins := make([]TXInputJSON, len(tx.Vin))
	for i, in := range tx.Vin {
		vins[i] = TXInputJSON{
			Txid:      hex.EncodeToString(in.Txid),
			Vout:      in.Vout,
			Signature: hex.EncodeToString(in.Signature),
			PubKey:    hex.EncodeToString(in.PubKey),
		}
	}

	vouts := make([]TXOutputJSON, len(tx.Vout))
	for i, out := range tx.Vout {
		vouts[i] = TXOutputJSON{
			Value:      out.Value,
			PubKeyHash: hex.EncodeToString(out.PubKeyHash),
		}
	}

	return TransactionJSON{
		ID:   hex.EncodeToString(tx.ID),
		Vin:  vins,
		Vout: vouts,
	}
}

func mapBlock(b *core.Block) BlockJSON {
	txs := make([]TransactionJSON, len(b.Transactions))
	for i, tx := range b.Transactions {
		txs[i] = mapTransaction(tx)
	}

	return BlockJSON{
		Timestamp:     b.Timestamp,
		Transactions:  txs,
		PrevBlockHash: hex.EncodeToString(b.PrevBlockHash),
		Hash:          hex.EncodeToString(b.Hash),
		Nonce:         b.Nonce,
		Height:        b.Height,
		Bits:          b.Bits,
	}
}

// enableCORS adds CORS headers to allow browser dashboard calls
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

// HandleBlocks returns list of blocks, paginated
func HandleBlocks(bc *core.Blockchain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 10
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
		offset := 0
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}

		var blocks []BlockJSON
		count := 0
		skipped := 0
		for b := range bc.Blocks() {
			if skipped < offset {
				skipped++
				continue
			}
			blocks = append(blocks, mapBlock(b))
			count++
			if count >= limit {
				break
			}
		}

		_ = json.NewEncoder(w).Encode(blocks)
	}
}

// HandleBlockByHash returns a single block by its hex hash
func HandleBlockByHash(bc *core.Blockchain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		hashHex := r.PathValue("hash")
		hash, err := hex.DecodeString(hashHex)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid hash format"})
			return
		}

		b, err := bc.GetBlock(hash)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "block not found"})
			return
		}

		_ = json.NewEncoder(w).Encode(mapBlock(b))
	}
}

// HandleHeight returns the best block height and its hash
func HandleHeight(bc *core.Blockchain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		height, err := bc.GetBestHeight()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		var tipHash string
		for b := range bc.Blocks() {
			tipHash = hex.EncodeToString(b.Hash)
			break
		}

		res := HeightJSON{
			Height:  height,
			TipHash: tipHash,
		}
		_ = json.NewEncoder(w).Encode(res)
	}
}

// HandleBalance returns the UTXO balance for a wallet address
func HandleBalance(bc *core.Blockchain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		address := r.PathValue("address")
		if !core.ValidateAddress(address) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid address"})
			return
		}

		utxoSet := core.UTXOSet{Blockchain: bc}
		pubKeyHash := core.Base58Decode([]byte(address))
		if len(pubKeyHash) < 5 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid address format"})
			return
		}
		pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

		utxos, err := utxoSet.FindUTXO(pubKeyHash)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		balance := 0
		for _, out := range utxos {
			balance += out.Value
		}

		res := BalanceJSON{
			Address: address,
			Balance: balance,
		}
		_ = json.NewEncoder(w).Encode(res)
	}
}

// HandleMempool returns the list of pending transactions in the mempool
func HandleMempool(srv ServerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		mempoolTxs := srv.MempoolSnapshot()
		res := make([]TransactionJSON, len(mempoolTxs))
		for i, tx := range mempoolTxs {
			res[i] = mapTransaction(&tx)
		}

		_ = json.NewEncoder(w).Encode(res)
	}
}
