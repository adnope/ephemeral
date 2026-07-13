package sse

import (
	"net/http/httptest"
	"testing"

	"github.com/adnope/ephemeral/internal/domain"
)

func TestBrokerReplaysEventsAfterLastEventID(t *testing.T) {
	broker := NewBroker()
	for id := int64(1); id <= 3; id++ {
		broker.Broadcast(domain.Event{Type: "item:new", ID: id})
	}

	sub := broker.subscribe(1)
	defer broker.unsubscribe(sub.events)

	if sub.reset {
		t.Fatal("subscribe() requested reset for retained events")
	}
	if len(sub.replay) != 2 {
		t.Fatalf("len(replay) = %d, want 2", len(sub.replay))
	}
	if sub.replay[0].Sequence != 2 || sub.replay[0].Event.ID != 2 {
		t.Fatalf("first replay event = %+v, want sequence 2 and item 2", sub.replay[0])
	}
	if sub.replay[1].Sequence != 3 || sub.replay[1].Event.ID != 3 {
		t.Fatalf("second replay event = %+v, want sequence 3 and item 3", sub.replay[1])
	}
}

func TestBrokerRequestsReconciliationWhenReplayGapIsTooOld(t *testing.T) {
	broker := NewBroker()
	for id := int64(1); id <= eventHistorySize+2; id++ {
		broker.Broadcast(domain.Event{Type: "item:new", ID: id})
	}

	sub := broker.subscribe(1)
	defer broker.unsubscribe(sub.events)

	if !sub.reset {
		t.Fatal("subscribe() did not request reset for an event outside retained history")
	}
	if len(sub.replay) != 0 {
		t.Fatalf("len(replay) = %d, want 0 after replay gap", len(sub.replay))
	}
}

func TestBrokerDisconnectsSlowSubscriberInsteadOfSilentlyDroppingEvents(t *testing.T) {
	broker := NewBroker()
	sub := broker.subscribe(0)

	for id := int64(1); id <= subscriberBufferSize+1; id++ {
		broker.Broadcast(domain.Event{Type: "item:updated", ID: id})
	}

	for range subscriberBufferSize {
		if _, ok := <-sub.events; !ok {
			t.Fatal("subscriber closed before buffered events were readable")
		}
	}
	if _, ok := <-sub.events; ok {
		t.Fatal("slow subscriber remained connected after its buffer filled")
	}
}

func TestWriteEventIncludesSequenceForEventSourceRecovery(t *testing.T) {
	output := httptest.NewRecorder()
	writeEvent(output, sequencedEvent{
		Sequence: 42,
		Event:    domain.Event{Type: "item:deleted", ID: 17},
	})

	want := "id: 42\nevent: item:deleted\ndata: 17\n\n"
	if got := output.Body.String(); got != want {
		t.Fatalf("writeEvent() = %q, want %q", got, want)
	}
}

func TestBrokerRequestsReconciliationAfterServerRestart(t *testing.T) {
	broker := NewBroker()
	sub := broker.subscribe(99)
	defer broker.unsubscribe(sub.events)

	if !sub.reset {
		t.Fatal("subscribe() did not request reset for an ID newer than this broker")
	}
}

func TestWriteResetAdvancesEventSourceCursor(t *testing.T) {
	output := httptest.NewRecorder()
	writeReset(output, 73)

	want := "id: 73\nevent: stream:reset\ndata: reconcile\n\n"
	if got := output.Body.String(); got != want {
		t.Fatalf("writeReset() = %q, want %q", got, want)
	}
}

func TestBrokerRetainsBoundedEventHistory(t *testing.T) {
	broker := NewBroker()
	for id := int64(1); id <= eventHistorySize*2; id++ {
		broker.Broadcast(domain.Event{Type: "item:new", ID: id})
	}

	if got := len(broker.history); got != eventHistorySize {
		t.Fatalf("len(history) = %d, want %d", got, eventHistorySize)
	}
	wantOldest := uint64(eventHistorySize + 1)
	if got := broker.history[0].Sequence; got != wantOldest {
		t.Fatalf("oldest sequence = %d, want %d", got, wantOldest)
	}
	if got := broker.history[len(broker.history)-1].Sequence; got != eventHistorySize*2 {
		t.Fatalf("newest sequence = %d, want %d", got, eventHistorySize*2)
	}
}
