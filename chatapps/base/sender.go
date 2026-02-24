package base

import (
	"context"
	"errors"
	"sync"
)

// ErrSenderNotConfigured is returned when SendMessage is called
// but no sender function has been configured.
const ErrSenderNotConfigured = "sender not configured"

// SenderFunc is the function signature for sending messages to a platform.
type SenderFunc func(ctx context.Context, sessionID string, msg *ChatMessage) error

// SenderWithMutex provides thread-safe sender management for chat adapters.
// This eliminates the duplicate sender/senderMu pattern across all adapters.
type SenderWithMutex struct {
	sender SenderFunc
	mu     sync.RWMutex
}

// NewSenderWithMutex creates a new SenderWithMutex with no sender configured.
func NewSenderWithMutex() *SenderWithMutex {
	return &SenderWithMutex{}
}

// NewSenderWithMutexFunc creates a new SenderWithMutex with an initial sender.
func NewSenderWithMutexFunc(sender SenderFunc) *SenderWithMutex {
	return &SenderWithMutex{
		sender: sender,
	}
}

// SetSender sets the sender function. Thread-safe.
func (s *SenderWithMutex) SetSender(fn SenderFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sender = fn
}

// SendMessage sends a message using the configured sender. Thread-safe.
// Returns ErrSenderNotConfigured if no sender has been set.
func (s *SenderWithMutex) SendMessage(ctx context.Context, sessionID string, msg *ChatMessage) error {
	s.mu.RLock()
	sender := s.sender
	s.mu.RUnlock()

	if sender == nil {
		return errors.New(ErrSenderNotConfigured)
	}
	return sender(ctx, sessionID, msg)
}

// HasSender returns true if a sender has been configured.
func (s *SenderWithMutex) HasSender() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sender != nil
}

// Sender returns the current sender function (may be nil).
// Note: This does not acquire the lock, so the caller should ensure
// thread-safety if the sender might be modified concurrently.
func (s *SenderWithMutex) Sender() SenderFunc {
	return s.sender
}
