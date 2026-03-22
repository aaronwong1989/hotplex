package base

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestSenderWithMutex_SetAndGet(t *testing.T) {
	s := NewSenderWithMutex()

	if s.HasSender() {
		t.Error("should not have sender initially")
	}

	called := false
	s.SetSender(func(ctx context.Context, sessionID string, msg *ChatMessage) error {
		called = true
		return nil
	})

	if !s.HasSender() {
		t.Error("should have sender after SetSender")
	}

	msg := &ChatMessage{Content: "hello"}
	err := s.SendMessage(context.Background(), "sess1", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("sender should have been called")
	}
}

func TestSenderWithMutex_NotConfigured(t *testing.T) {
	s := NewSenderWithMutex()

	err := s.SendMessage(context.Background(), "sess1", &ChatMessage{})
	if err == nil {
		t.Fatal("expected error for unconfigured sender")
	}
	if err.Error() != ErrSenderNotConfigured {
		t.Errorf("expected ErrSenderNotConfigured, got %q", err.Error())
	}
}

func TestSenderWithMutex_NewWithFunc(t *testing.T) {
	var called bool
	s := NewSenderWithMutexFunc(func(ctx context.Context, sessionID string, msg *ChatMessage) error {
		called = true
		return nil
	})

	err := s.SendMessage(context.Background(), "sess1", &ChatMessage{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("sender should have been called")
	}
}

func TestSenderWithMutex_SenderError(t *testing.T) {
	testErr := errors.New("send failed")
	s := NewSenderWithMutexFunc(func(ctx context.Context, sessionID string, msg *ChatMessage) error {
		return testErr
	})

	err := s.SendMessage(context.Background(), "sess1", &ChatMessage{})
	if err != testErr {
		t.Errorf("expected testErr, got %v", err)
	}
}

func TestSenderWithMutex_Concurrent(t *testing.T) {
	s := NewSenderWithMutex()
	var wg sync.WaitGroup

	// Concurrent SetSender and SendMessage
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SetSender(func(ctx context.Context, sessionID string, msg *ChatMessage) error {
				return nil
			})
		}()
	}

	wg.Wait()

	if !s.HasSender() {
		t.Error("should have sender after concurrent sets")
	}
}

func TestSenderWithMutex_SenderMethod(t *testing.T) {
	s := NewSenderWithMutex()
	if s.Sender() != nil {
		t.Error("initial sender should be nil")
	}

	s.SetSender(func(ctx context.Context, sessionID string, msg *ChatMessage) error {
		return nil
	})
	if s.Sender() == nil {
		t.Error("sender should not be nil after set")
	}
}
