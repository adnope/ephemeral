package sse

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type Broker struct {
	subscribers sync.Map
}

func NewBroker() *Broker {
	return &Broker{}
}

func (b *Broker) Subscribe() chan domain.Event {
	ch := make(chan domain.Event, 4)
	b.subscribers.Store(ch, struct{}{})
	return ch
}

func (b *Broker) Unsubscribe(ch chan domain.Event) {
	if _, loaded := b.subscribers.LoadAndDelete(ch); loaded {
		close(ch)
	}
}

func (b *Broker) Broadcast(event domain.Event) {
	b.subscribers.Range(func(key, _ any) bool {
		ch, ok := key.(chan domain.Event)
		if !ok {
			return true
		}
		select {
		case ch <- event:
		default:
		}
		return true
	})
}

func (b *Broker) Shutdown() {
	b.subscribers.Range(func(key, _ any) bool {
		ch, ok := key.(chan domain.Event)
		if ok {
			b.Unsubscribe(ch)
		}
		return true
	})
}

func (b *Broker) ServeSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %d\n\n", event.Type, event.ID)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
