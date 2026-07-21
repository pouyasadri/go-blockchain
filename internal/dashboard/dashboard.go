package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/firewall"
	"github.com/pouyasadri/go-blockchain/internal/indexer"
)

// Server represents the embedded web dashboard server
type Server struct {
	store      *indexer.IndexStore
	fw         *firewall.Firewall
	sseBroker  *SSEBroker
	renderer   *TemplateRenderer
	httpServer *http.Server
	port       string
}

// NewServer creates a new Dashboard Server
func NewServer(store *indexer.IndexStore, fw *firewall.Firewall, port string) (*Server, error) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		return nil, err
	}

	broker := NewSSEBroker()

	s := &Server{
		store:     store,
		fw:        fw,
		sseBroker: broker,
		renderer:  renderer,
		port:      port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/events", broker.ServeHTTP)
	mux.HandleFunc("/partials/metrics", s.handlePartialMetrics)
	mux.HandleFunc("/partials/escrows", s.handlePartialEscrows)
	mux.HandleFunc("/partials/services", s.handlePartialServices)
	mux.HandleFunc("/partials/firewall", s.handlePartialFirewall)
	mux.HandleFunc("/partials/blocks", s.handlePartialBlocks)
	mux.Handle("/assets/", http.StripPrefix("/assets/", AssetsHandler()))

	s.httpServer = &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // Unset timeout for SSE streaming
	}

	return s, nil
}

// Start launches the dashboard web server
func (s *Server) Start(ctx context.Context) error {
	log.Printf("[Dashboard] Web server active at http://localhost%s", s.port)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("dashboard server failed: %w", err)
	}
	return nil
}

// NotifyBlockProcessed triggers real-time SSE updates to dashboard clients
func (s *Server) NotifyBlockProcessed() {
	var buf bytes.Buffer
	RenderMetricsPartial(&buf, s.store.GetMetrics())
	s.sseBroker.BroadcastEvent("metrics", buf.String())

	buf.Reset()
	RenderEscrowsPartial(&buf, s.store.GetActiveEscrows())
	s.sseBroker.BroadcastEvent("escrows", buf.String())

	buf.Reset()
	RenderBlocksPartial(&buf, s.store.GetRecentBlocks())
	s.sseBroker.BroadcastEvent("blocks", buf.String())

	if s.fw != nil {
		buf.Reset()
		budget, spent := s.fw.GetSessionStats()
		RenderFirewallPartial(&buf, budget, spent)
		s.sseBroker.BroadcastEvent("firewall", buf.String())
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.renderer.RenderIndex(w)
}

func (s *Server) handlePartialMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	RenderMetricsPartial(w, s.store.GetMetrics())
}

func (s *Server) handlePartialEscrows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	RenderEscrowsPartial(w, s.store.GetActiveEscrows())
}

func (s *Server) handlePartialServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	RenderServicesPartial(w, s.store.ListServices())
}

func (s *Server) handlePartialFirewall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var budget, spent int64 = 50000, 0
	if s.fw != nil {
		budget, spent = s.fw.GetSessionStats()
	}
	RenderFirewallPartial(w, budget, spent)
}

func (s *Server) handlePartialBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	RenderBlocksPartial(w, s.store.GetRecentBlocks())
}
