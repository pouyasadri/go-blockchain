package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/indexer"
	"github.com/stretchr/testify/assert"
)

func TestDashboardServerRoutes(t *testing.T) {
	store := indexer.NewIndexStore()
	srv, err := NewServer(store, nil, ":8089")
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)

	// Test Index route
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "AI Micro-payment Settlement Engine")

	// Test Partials
	req = httptest.NewRequest("GET", "/partials/metrics", nil)
	w = httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Blocks Indexed")

	req = httptest.NewRequest("GET", "/partials/blocks", nil)
	w = httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
