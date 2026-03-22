package admin

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEventBuffer_PushAndGet(t *testing.T) {
	buf := NewEventBuffer(100)

	event := AdminEvent{
		EventID:   uuid.New().String(),
		Timestamp: time.Now(),
		Type:      "test.event",
		Payload:   map[string]interface{}{"key": "value"},
	}

	buf.Push(event)

	events := buf.GetEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != "test.event" {
		t.Errorf("expected event type 'test.event', got %s", events[0].Type)
	}
}

func TestEventBuffer_Overflow(t *testing.T) {
	buf := NewEventBuffer(3)

	// Push 5 events into a buffer of size 3
	for i := 0; i < 5; i++ {
		buf.Push(AdminEvent{
			EventID: uuid.New().String(),
			Type:    "event",
			Payload: map[string]interface{}{"index": i},
		})
	}

	events := buf.GetEvents()
	if len(events) != 3 {
		t.Errorf("expected 3 events (max size), got %d", len(events))
	}

	// Should have the 3 most recent events (indices 2, 3, 4)
	idx := events[0].Payload["index"].(int)
	if idx != 2 {
		t.Errorf("expected first event index 2, got %d", idx)
	}
}

func TestEventBuffer_Pagination(t *testing.T) {
	buf := NewEventBuffer(100)

	// Push 10 events
	for i := 0; i < 10; i++ {
		buf.Push(AdminEvent{
			EventID: uuid.New().String(),
			Type:    "event",
			Payload: map[string]interface{}{"index": i},
		})
	}

	// Get first page
	events, total := buf.GetEventsPaginated(3, 0)
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Get second page
	events, total = buf.GetEventsPaginated(3, 3)
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Get last page (partial)
	events, _ = buf.GetEventsPaginated(5, 8)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestEventBuffer_Size(t *testing.T) {
	buf := NewEventBuffer(10)

	if buf.Size() != 0 {
		t.Errorf("expected size 0, got %d", buf.Size())
	}
	if buf.MaxSize() != 10 {
		t.Errorf("expected max size 10, got %d", buf.MaxSize())
	}

	buf.Push(AdminEvent{Type: "test"})
	if buf.Size() != 1 {
		t.Errorf("expected size 1, got %d", buf.Size())
	}
}

func TestEventBuffer_Clear(t *testing.T) {
	buf := NewEventBuffer(10)

	buf.Push(AdminEvent{Type: "test1"})
	buf.Push(AdminEvent{Type: "test2"})

	if buf.Size() != 2 {
		t.Errorf("expected size 2, got %d", buf.Size())
	}

	buf.Clear()

	if buf.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", buf.Size())
	}
}

func TestEventBuffer_DefaultSize(t *testing.T) {
	buf := NewEventBuffer(0)
	if buf.MaxSize() != 10000 {
		t.Errorf("expected default size 10000, got %d", buf.MaxSize())
	}

	buf2 := NewEventBuffer(-5)
	if buf2.MaxSize() != 10000 {
		t.Errorf("expected default size 10000 for negative, got %d", buf2.MaxSize())
	}
}
