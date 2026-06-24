package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// APIServer represents the REST API server for the blockchain node
type APIServer struct {
	bc  *core.Blockchain
	srv ServerInterface
	Mux *http.ServeMux
}

// NewAPIServer creates a new APIServer instance
func NewAPIServer(bc *core.Blockchain, srv ServerInterface) *APIServer {
	mux := http.NewServeMux()

	// Register default routes
	mux.HandleFunc("GET /api/v1/blocks", HandleBlocks(bc))
	mux.HandleFunc("GET /api/v1/blocks/{hash}", HandleBlockByHash(bc))
	mux.HandleFunc("GET /api/v1/height", HandleHeight(bc))
	mux.HandleFunc("GET /api/v1/balance/{address}", HandleBalance(bc))
	mux.HandleFunc("GET /api/v1/mempool", HandleMempool(srv))
	mux.Handle("GET /metrics", promhttp.Handler())

	return &APIServer{
		bc:  bc,
		srv: srv,
		Mux: mux,
	}
}

// Start starts the HTTP REST API server on the given address.
// It will shut down gracefully when the passed context is cancelled.
func (api *APIServer) Start(ctx context.Context, addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: api.Mux,
	}

	// Goroutine for graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
