package dashboard

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client represents a connected SSE subscriber
type Client struct {
	id     string
	sendCh chan string
}

// SSEBroker handles Server-Sent Events broadcasting
type SSEBroker struct {
	mu         sync.RWMutex
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan string
}

// NewSSEBroker creates and starts a new SSEBroker instance
func NewSSEBroker() *SSEBroker {
	b := &SSEBroker{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan string, 100),
	}
	go b.listen()
	return b
}

func (b *SSEBroker) listen() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client.id] = client
			b.mu.Unlock()
			log.Printf("[SSE Broker] Client %s connected (%d total)", client.id, len(b.clients))

		case client := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[client.id]; ok {
				delete(b.clients, client.id)
				close(client.sendCh)
				log.Printf("[SSE Broker] Client %s disconnected (%d total)", client.id, len(b.clients))
			}
			b.mu.Unlock()

		case msg := <-b.broadcast:
			b.mu.RLock()
			for _, client := range b.clients {
				select {
				case client.sendCh <- msg:
				default:
					// Drop if channel buffer full to avoid blocking broker
				}
			}
			b.mu.RUnlock()
		}
	}
}

// BroadcastEvent sends a named SSE event with HTML payload to all subscribers,
// formatting multi-line HTML string properly according to W3C EventSource specifications.
func (b *SSEBroker) BroadcastEvent(event string, htmlData string) {
	lines := strings.Split(htmlData, "\n")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("event: %s\n", event))
	for _, line := range lines {
		sb.WriteString("data: ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	b.broadcast <- sb.String()
}

// ServeHTTP handles the SSE streaming connection
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID := fmt.Sprintf("client_%p", r)
	if id := r.URL.Query().Get("id"); id != "" {
		clientID = fmt.Sprintf("client_%s", id)
	}

	client := &Client{
		id:     clientID,
		sendCh: make(chan string, 50),
	}

	b.register <- client
	defer func() {
		b.unregister <- client
	}()

	// Send initial connection heartbeat
	fmt.Fprintf(w, "event: connected\ndata: {\"status\": \"ok\"}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			// Ping heartbeat to keep connection alive
			_, err := io.WriteString(w, ": heartbeat\n\n")
			if err != nil {
				return
			}
			flusher.Flush()
		case msg, ok := <-client.sendCh:
			if !ok {
				return
			}
			_, err := io.WriteString(w, msg)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
