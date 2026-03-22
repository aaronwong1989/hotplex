package admin

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventBuffer is a thread-safe ring buffer for audit events.
// It stores the last maxSize events in memory for fast access.
type EventBuffer struct {
	mu      sync.RWMutex
	events  []AdminEvent
	maxSize int
	head    int // Points to the oldest event
	count   int // Number of events in buffer
}

// NewEventBuffer creates a new event buffer with the given maximum size.
func NewEventBuffer(maxSize int) *EventBuffer {
	if maxSize <= 0 {
		maxSize = 10000 // Default to 10,000 events
	}
	return &EventBuffer{
		events:  make([]AdminEvent, maxSize),
		maxSize: maxSize,
		head:    0,
		count:   0,
	}
}

// Push adds a new event to the buffer.
// If the buffer is full, the oldest event is overwritten.
func (b *EventBuffer) Push(event AdminEvent) {
	if event.EventID == "" {
		event.EventID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Write at head position (overwrites oldest)
	b.events[b.head] = event
	b.head = (b.head + 1) % b.maxSize
	if b.count < b.maxSize {
		b.count++
	}
}

// GetEvents returns all events in chronological order (oldest first).
func (b *EventBuffer) GetEvents() []AdminEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return []AdminEvent{}
	}

	result := make([]AdminEvent, b.count)

	// Calculate starting position (oldest event)
	start := 0
	if b.count == b.maxSize {
		start = b.head
	}

	for i := 0; i < b.count; i++ {
		idx := (start + i) % b.maxSize
		result[i] = b.events[idx]
	}

	return result
}

// GetEventsPaginated returns events with pagination support.
func (b *EventBuffer) GetEventsPaginated(limit, offset int) ([]AdminEvent, int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := int64(b.count)
	if total == 0 {
		return []AdminEvent{}, 0
	}

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Calculate starting position (oldest event)
	start := 0
	if b.count == b.maxSize {
		start = b.head
	}

	// Apply offset
	startIdx := offset
	if startIdx >= b.count {
		return []AdminEvent{}, total
	}

	// Apply limit
	endIdx := startIdx + limit
	if endIdx > b.count {
		endIdx = b.count
	}

	result := make([]AdminEvent, endIdx-startIdx)
	for i := 0; i < len(result); i++ {
		idx := (start + startIdx + i) % b.maxSize
		result[i] = b.events[idx]
	}

	return result, total
}

// Size returns the current number of events in the buffer.
func (b *EventBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// MaxSize returns the maximum capacity of the buffer.
func (b *EventBuffer) MaxSize() int {
	return b.maxSize
}

// Clear removes all events from the buffer.
func (b *EventBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.head = 0
	b.count = 0
}
