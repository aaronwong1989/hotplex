package engine

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/hrygo/hotplex/event"
	intengine "github.com/hrygo/hotplex/internal/engine"
	"github.com/hrygo/hotplex/provider"
	"github.com/hrygo/hotplex/types"
)

func TestEngine_createEventBridge(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	prv, _ := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	engine := &Engine{
		opts:     EngineOptions{Namespace: "test"},
		logger:   logger,
		provider: prv,
	}

	cfg := &types.Config{
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}

	stats := &SessionStats{SessionID: "test-session"}
	doneChan := make(chan struct{})

	// Create event bridge
	cb := engine.createEventBridge(cfg, nil, stats, doneChan)

	if cb == nil {
		t.Fatal("createEventBridge returned nil")
	}

	// Test runner_exit event
	err := cb("runner_exit", nil)
	if err != nil {
		t.Errorf("runner_exit callback error: %v", err)
	}

	// doneChan should be closed after runner_exit
	select {
	case <-doneChan:
		// Expected
	default:
		t.Error("doneChan should be closed after runner_exit")
	}
}

func TestEngine_createEventBridge_RawLine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	prv, _ := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	engine := &Engine{
		opts:     EngineOptions{Namespace: "test"},
		logger:   logger,
		provider: prv,
	}

	cfg := &types.Config{
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}

	stats := &SessionStats{SessionID: "test-session"}
	doneChan := make(chan struct{})

	var received string
	userCb := func(eventType string, data any) error {
		if eventType == "answer" {
			if s, ok := data.(string); ok {
				received = s
			} else if ev, ok := data.(*event.EventWithMeta); ok {
				received = ev.EventData
			}
		}
		return nil
	}

	cb := engine.createEventBridge(cfg, userCb, stats, doneChan)

	// Test raw_line event with invalid JSON (should be passed as answer)
	err := cb("raw_line", "not valid json")
	if err != nil {
		t.Errorf("raw_line callback error: %v", err)
	}

	if received != "not valid json" {
		t.Errorf("received = %q, want 'not valid json'", received)
	}
}

func TestEngine_createEventBridge_NonStreamMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	prv, _ := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	engine := &Engine{
		opts:     EngineOptions{Namespace: "test"},
		logger:   logger,
		provider: prv,
	}

	cfg := &types.Config{
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}

	stats := &SessionStats{SessionID: "test-session"}
	doneChan := make(chan struct{})

	var received string
	userCb := func(eventType string, data any) error {
		received = eventType
		return nil
	}

	cb := engine.createEventBridge(cfg, userCb, stats, doneChan)

	// Test non-types.StreamMessage data (legacy path)
	err := cb("custom_event", "some data")
	if err != nil {
		t.Errorf("non-types.StreamMessage callback error: %v", err)
	}

	if received != "custom_event" {
		t.Errorf("received = %q, want 'custom_event'", received)
	}
}

func TestEngine_createEventBridge_RawLineNotString(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	prv, _ := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	engine := &Engine{
		opts:     EngineOptions{Namespace: "test"},
		logger:   logger,
		provider: prv,
	}

	cfg := &types.Config{
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}

	stats := &SessionStats{SessionID: "test-session"}
	doneChan := make(chan struct{})

	cb := engine.createEventBridge(cfg, nil, stats, doneChan)

	// Test raw_line with non-string data - should be silently ignored
	err := cb("raw_line", 12345)
	if err != nil {
		t.Errorf("raw_line with non-string error: %v", err)
	}
}

func TestEngine_waitForSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	eng := &Engine{
		opts:   EngineOptions{Namespace: "test"},
		logger: logger,
	}

	t.Run("session ready", func(t *testing.T) {
		sess := intengine.NewTestSession("test", intengine.SessionStatusReady)

		ctx := context.Background()
		err := eng.waitForSession(ctx, sess, "test-session")
		if err != nil {
			t.Errorf("waitForSession error: %v", err)
		}
	})

	t.Run("session busy", func(t *testing.T) {
		sess := intengine.NewTestSession("test", intengine.SessionStatusBusy)

		ctx := context.Background()
		err := eng.waitForSession(ctx, sess, "test-session")
		if err != nil {
			t.Errorf("waitForSession error: %v", err)
		}
	})

	t.Run("session dead", func(t *testing.T) {
		sess := intengine.NewTestSession("test", intengine.SessionStatusDead)

		ctx := context.Background()
		err := eng.waitForSession(ctx, sess, "test-session")
		if err == nil {
			t.Error("waitForSession should fail for dead session")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		sess := intengine.NewTestSession("test", intengine.SessionStatusStarting)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := eng.waitForSession(ctx, sess, "test-session")
		if err != context.Canceled {
			t.Errorf("waitForSession error = %v, want context.Canceled", err)
		}
	})
}

func TestEngine_waitForSession_StatusChange(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	eng := &Engine{
		opts:   EngineOptions{Namespace: "test"},
		logger: logger,
	}

	sess := intengine.NewTestSession("test", intengine.SessionStatusStarting)

	ctx := context.Background()

	// Send status change in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		sess.SetStatus(intengine.SessionStatusReady)
	}()

	err := eng.waitForSession(ctx, sess, "test-session")
	if err != nil {
		t.Errorf("waitForSession error: %v", err)
	}
}
