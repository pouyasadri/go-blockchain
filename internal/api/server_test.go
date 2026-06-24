package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/core"
	_ "github.com/pouyasadri/go-blockchain/internal/metrics"
	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
)

type mockServer struct {
	mempool []core.Transaction
	nodes   []string
}

func (m *mockServer) MempoolSnapshot() []core.Transaction {
	return m.mempool
}

func (m *mockServer) GetKnownNodes() []string {
	return m.nodes
}

func TestAPIHandlers(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "api_test.db")
	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	wallet, err := core.NewWallet()
	assert.NoError(t, err)
	addr := string(wallet.GetAddress())

	bc, err := core.CreateBlockchain(addr, db)
	assert.NoError(t, err)

	utxoSet := core.UTXOSet{Blockchain: bc}
	err = utxoSet.Reindex()
	assert.NoError(t, err)

	mockSrv := &mockServer{
		mempool: []core.Transaction{},
		nodes:   []string{"localhost:3000"},
	}

	t.Run("GET /api/v1/height", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/height", nil)
		w := httptest.NewRecorder()

		handler := HandleHeight(bc)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res HeightJSON
		err := json.NewDecoder(w.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Equal(t, 0, res.Height)
		assert.NotEmpty(t, res.TipHash)
	})

	t.Run("GET /api/v1/blocks", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/blocks?limit=5", nil)
		w := httptest.NewRecorder()

		handler := HandleBlocks(bc)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res []BlockJSON
		err := json.NewDecoder(w.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Len(t, res, 1) // Only genesis block
		assert.Equal(t, 0, res[0].Height)
	})

	t.Run("GET /api/v1/mempool", func(t *testing.T) {
		txin := core.TXInput{Txid: []byte{}, Vout: -1, Signature: nil, PubKey: wallet.PublicKey}
		txout := core.NewTXOutput(10, addr)
		tx := core.Transaction{ID: []byte("dummytxid"), Vin: []core.TXInput{txin}, Vout: []core.TXOutput{*txout}}
		mockSrv.mempool = []core.Transaction{tx}

		req := httptest.NewRequest("GET", "/api/v1/mempool", nil)
		w := httptest.NewRecorder()

		handler := HandleMempool(mockSrv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res []TransactionJSON
		err := json.NewDecoder(w.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, hex.EncodeToString(tx.ID), res[0].ID)
	})

	t.Run("GET /api/v1/balance", func(t *testing.T) {
		// Valid address
		req := httptest.NewRequest("GET", "/api/v1/balance/"+addr, nil)
		req.SetPathValue("address", addr)
		w := httptest.NewRecorder()

		handler := HandleBalance(bc)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res BalanceJSON
		err := json.NewDecoder(w.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Equal(t, addr, res.Address)
		assert.True(t, res.Balance > 0)

		// Invalid address
		reqErr := httptest.NewRequest("GET", "/api/v1/balance/invalidaddress", nil)
		reqErr.SetPathValue("address", "invalidaddress")
		wErr := httptest.NewRecorder()
		handler.ServeHTTP(wErr, reqErr)
		assert.Equal(t, http.StatusBadRequest, wErr.Code)
	})

	t.Run("GET /api/v1/blocks/{hash}", func(t *testing.T) {
		hashes := bc.GetBlockHashes()
		assert.NotEmpty(t, hashes)
		hashHex := hex.EncodeToString(hashes[0])

		req := httptest.NewRequest("GET", "/api/v1/blocks/"+hashHex, nil)
		req.SetPathValue("hash", hashHex)
		w := httptest.NewRecorder()

		handler := HandleBlockByHash(bc)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res BlockJSON
		err := json.NewDecoder(w.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Equal(t, hashHex, res.Hash)

		// Invalid hash format
		reqErrFormat := httptest.NewRequest("GET", "/api/v1/blocks/nonhex", nil)
		reqErrFormat.SetPathValue("hash", "nonhex")
		wErrFormat := httptest.NewRecorder()
		handler.ServeHTTP(wErrFormat, reqErrFormat)
		assert.Equal(t, http.StatusBadRequest, wErrFormat.Code)

		// Block not found
		dummyHashHex := "0000000000000000000000000000000000000000000000000000000000000000"
		reqErrNotFound := httptest.NewRequest("GET", "/api/v1/blocks/"+dummyHashHex, nil)
		reqErrNotFound.SetPathValue("hash", dummyHashHex)
		wErrNotFound := httptest.NewRecorder()
		handler.ServeHTTP(wErrNotFound, reqErrNotFound)
		assert.Equal(t, http.StatusNotFound, wErrNotFound.Code)
	})

	t.Run("GET /metrics", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()

		apiServer := NewAPIServer(bc, mockSrv)
		apiServer.Mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "blockchain_height")
		assert.Contains(t, body, "blockchain_mempool_size")
	})

	t.Run("APIServer execution", func(t *testing.T) {
		apiServer := NewAPIServer(bc, mockSrv)
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		err := apiServer.Start(ctx, "localhost:18080")
		assert.NoError(t, err)
	})
}
