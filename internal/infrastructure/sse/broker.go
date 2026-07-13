package sse

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

const (
	subscriberBufferSize = 64
	eventHistorySize     = 256
)

type sequencedEvent struct {
	Sequence uint64
	Event    domain.Event
}

type subscription struct {
	events chan sequencedEvent
	replay []sequencedEvent
	reset  bool
	cursor uint64
}

type Broker struct {
	mu          sync.Mutex
	subscribers map[chan sequencedEvent]struct{}
	history     []sequencedEvent
	nextID      uint64
	closed      bool
}

func NewBroker() *Broker {
	return &Broker{subscribers: make(map[chan sequencedEvent]struct{})}
}

func (b *Broker) subscribe(lastEventID uint64) subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan sequencedEvent, subscriberBufferSize)
	if b.closed {
		close(ch)
		return subscription{events: ch, reset: true, cursor: b.nextID}
	}

	replay, reset := b.replayAfter(lastEventID)
	b.subscribers[ch] = struct{}{}

	return subscription{
		events: ch,
		replay: replay,
		reset:  reset,
		cursor: b.nextID,
	}
}

func (b *Broker) replayAfter(lastEventID uint64) ([]sequencedEvent, bool) {
	if lastEventID == 0 {
		return nil, false
	}
	if lastEventID > b.nextID {
		return nil, true
	}
	if len(b.history) == 0 {
		return nil, lastEventID != b.nextID
	}

	oldestID := b.history[0].Sequence
	if lastEventID < oldestID-1 {
		return nil, true
	}

	first := len(b.history)
	for i, event := range b.history {
		if event.Sequence > lastEventID {
			first = i
			break
		}
	}
	if first == len(b.history) {
		return nil, false
	}

	replay := make([]sequencedEvent, len(b.history)-first)
	copy(replay, b.history[first:])
	return replay, false
}

func (b *Broker) unsubscribe(ch chan sequencedEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; !ok {
		return
	}
	delete(b.subscribers, ch)
	close(ch)
}

func (b *Broker) Broadcast(event domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.nextID++
	sequenced := sequencedEvent{Sequence: b.nextID, Event: event}
	b.history = append(b.history, sequenced)
	if len(b.history) > eventHistorySize {
		copy(b.history, b.history[len(b.history)-eventHistorySize:])
		b.history = b.history[:eventHistorySize]
	}

	for ch := range b.subscribers {
		select {
		case ch <- sequenced:
		default:
			delete(b.subscribers, ch)
			close(ch)
		}
	}
}

func (b *Broker) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subscribers {
		delete(b.subscribers, ch)
		close(ch)
	}
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

	lastEventID, _ := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64)
	sub := b.subscribe(lastEventID)
	defer b.unsubscribe(sub.events)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	if sub.reset {
		writeReset(w, sub.cursor)
		flusher.Flush()
	}
	for _, event := range sub.replay {
		writeEvent(w, event)
		flusher.Flush()
	}

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-sub.events:
			if !ok {
				return
			}
			writeEvent(w, event)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeEvent(w http.ResponseWriter, event sequencedEvent) {
	_, _ = fmt.Fprintf(
		w,
		"id: %d\nevent: %s\ndata: %d\n\n",
		event.Sequence,
		event.Event.Type,
		event.Event.ID,
	)
}

func writeReset(w http.ResponseWriter, cursor uint64) {
	_, _ = fmt.Fprintf(w, "id: %d\nevent: stream:reset\ndata: reconcile\n\n", cursor)
}
