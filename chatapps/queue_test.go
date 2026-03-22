package chatapps

import (
	"context"
	"testing"
	"time"

	"log/slog"
)

func TestMessageQueue_EnqueueDequeue(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 10, 10, 1)

	msg := NewChatMessage("slack", "sess1", "U1", "hello")
	err := q.Enqueue("slack", "sess1", msg)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if q.Size() != 1 {
		t.Errorf("expected size 1, got %d", q.Size())
	}

	got, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue returned not ok")
	}
	if got.Platform != "slack" || got.SessionID != "sess1" {
		t.Errorf("unexpected message: %+v", got)
	}

	if q.Size() != 0 {
		t.Errorf("expected size 0, got %d", q.Size())
	}
}

func TestMessageQueue_DequeueEmpty(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 10, 10, 1)

	_, ok := q.Dequeue()
	if ok {
		t.Error("should return false for empty queue")
	}
}

func TestMessageQueue_Full(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 1, 10, 1)

	msg := NewChatMessage("slack", "sess1", "U1", "hello")
	err := q.Enqueue("slack", "sess1", msg)
	if err != nil {
		t.Fatalf("first Enqueue failed: %v", err)
	}

	err = q.Enqueue("slack", "sess2", msg)
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}
}

func TestMessageQueue_DLQ(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 10, 5, 1)

	msg1 := &QueuedMessage{Platform: "slack", SessionID: "s1", Retries: 3, CreatedAt: time.Now()}
	q.AddToDLQ(msg1)
	msg2 := &QueuedMessage{Platform: "slack", SessionID: "s2", Retries: 3, CreatedAt: time.Now()}
	q.AddToDLQ(msg2)

	if q.DLQLen() != 2 {
		t.Errorf("expected DLQ len 2, got %d", q.DLQLen())
	}

	dlq := q.GetDLQ()
	if len(dlq) != 2 {
		t.Errorf("expected 2 DLQ messages, got %d", len(dlq))
	}
}

func TestMessageQueue_DLQ_Overflow(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 10, 2, 1)

	for i := 0; i < 4; i++ {
		q.AddToDLQ(&QueuedMessage{Platform: "slack", SessionID: "s1", Retries: 3, CreatedAt: time.Now()})
	}

	if q.DLQLen() != 2 {
		t.Errorf("expected DLQ capped at 2, got %d", q.DLQLen())
	}
}

func TestMessageQueue_Requeue(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 10, 10, 1)

	msg := &QueuedMessage{Platform: "slack", SessionID: "s1", Retries: 1}
	err := q.Requeue(msg)
	if err != nil {
		t.Fatalf("Requeue failed: %v", err)
	}
	if q.Size() != 1 {
		t.Errorf("expected size 1, got %d", q.Size())
	}
}

func TestMessageQueue_Requeue_Full(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 0, 10, 1)

	err := q.Requeue(&QueuedMessage{Platform: "slack"})
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull for maxSize=0, got %v", err)
	}
}

func TestMessageQueue_StartStop(t *testing.T) {
	q := NewMessageQueue(slog.Default(), 10, 10, 1)

	sendFn := func(ctx context.Context, platform, sessionID string, msg *ChatMessage) error {
		return nil
	}

	q.Start(func(string) (ChatAdapter, bool) { return nil, false }, sendFn)

	// Give workers a moment to start
	time.Sleep(50 * time.Millisecond)

	q.Stop()
}

func TestQueueError(t *testing.T) {
	err := &QueueError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", err.Error())
	}
}

func TestQueuedMessage_Fields(t *testing.T) {
	msg := &QueuedMessage{
		Platform:  "slack",
		SessionID: "sess1",
		Message:   NewChatMessage("slack", "sess1", "U1", "hello"),
		Retries:   2,
		CreatedAt: time.Now(),
	}
	if msg.Platform != "slack" {
		t.Errorf("expected platform slack, got %s", msg.Platform)
	}
	if msg.Retries != 2 {
		t.Errorf("expected retries 2, got %d", msg.Retries)
	}
}
