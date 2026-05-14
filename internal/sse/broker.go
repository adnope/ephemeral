package sse

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Broker manages all active SSE client connections.
// A sync.Map is used for concurrent-safe subscriber management
// without a global mutex on the broadcast hot path.
type Broker struct {
	subscribers sync.Map // map[chan Event]struct{}
}

// NewBroker creates a new SSE broker.
func NewBroker() *Broker {
	return &Broker{}
}

// Subscribe registers a new client and returns its event channel.
func (b *Broker) Subscribe() chan Event {
	ch := make(chan Event, 4) // buffered: slow clients don't block Broadcast
	b.subscribers.Store(ch, struct{}{})
	return ch
}

// Unsubscribe removes a client and closes its channel.
func (b *Broker) Unsubscribe(ch chan Event) {
	b.subscribers.Delete(ch)
	close(ch)
}

// Broadcast sends an event to all connected clients.
// Non-blocking: drops events for slow subscribers.
func (b *Broker) Broadcast(e Event) {
	b.subscribers.Range(func(key, _ any) bool {
		ch := key.(chan Event)
		select {
		case ch <- e:
		default:
			// Subscriber is too slow; event dropped. Not fatal for SSE.
		}
		return true
	})
}

// ServeSSE is the http.HandlerFunc for GET /events.
func (b *Broker) ServeSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable Nginx/Caddy buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Keepalive: prevent proxy timeouts (every 25s)
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %d\n\n", event.Type, event.ID)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return // client disconnected
		}
	}
}
