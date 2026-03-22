package base

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex/types"

	"log/slog"
)

func TestMessageContextBuilder_Build(t *testing.T) {
	mc, err := NewMessageContextBuilder().
		WithChatSession("chat-1", "feishu", "user-1", "bot-1", "channel-1", "thread-1").
		WithEngineSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"), "ns1").
		WithProviderSession("provider-1", "claude").
		WithMessage(types.MessageTypeUserInput, DirectionUserToBot, "hello").
		WithMetadata("key1", "value1").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.ChatSessionID != "chat-1" {
		t.Errorf("expected chat-1, got %s", mc.ChatSessionID)
	}
	if mc.Content != "hello" {
		t.Errorf("expected hello, got %s", mc.Content)
	}
	if mc.Metadata["key1"] != "value1" {
		t.Error("metadata key1 not set")
	}
}

func TestMessageContextBuilder_MissingFields(t *testing.T) {
	// Missing engine session ID
	_, err := NewMessageContextBuilder().
		WithChatSession("chat-1", "feishu", "user-1", "bot-1", "channel-1", "thread-1").
		WithProviderSession("provider-1", "claude").
		WithMessage(types.MessageTypeUserInput, DirectionUserToBot, "hello").
		Build()

	if err != ErrMissingEngineSessionID {
		t.Errorf("expected ErrMissingEngineSessionID, got %v", err)
	}

	// Missing provider session ID
	_, err = NewMessageContextBuilder().
		WithChatSession("chat-1", "feishu", "user-1", "bot-1", "channel-1", "thread-1").
		WithEngineSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"), "ns1").
		WithMessage(types.MessageTypeUserInput, DirectionUserToBot, "hello").
		Build()

	if err != ErrMissingProviderSessionID {
		t.Errorf("expected ErrMissingProviderSessionID, got %v", err)
	}

	// Missing content
	_, err = NewMessageContextBuilder().
		WithChatSession("chat-1", "feishu", "user-1", "bot-1", "channel-1", "thread-1").
		WithEngineSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"), "ns1").
		WithProviderSession("provider-1", "claude").
		WithMessage(types.MessageTypeUserInput, DirectionUserToBot, "").
		Build()

	if err != ErrMissingContent {
		t.Errorf("expected ErrMissingContent, got %v", err)
	}
}

func TestMessageContext_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *MessageContext
		wantErr error
	}{
		{
			"valid",
			&MessageContext{
				ChatSessionID:     "chat-1",
				EngineSessionID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				ProviderSessionID: "provider-1",
				Content:           "hello",
			},
			nil,
		},
		{
			"missing_chat_session_id",
			&MessageContext{},
			ErrMissingChatSessionID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewMessageContextBuilder_NilMetadata(t *testing.T) {
	b := NewMessageContextBuilder()
	if b.ctx == nil {
		t.Fatal("expected non-nil ctx")
	}
	if b.ctx.Metadata == nil {
		t.Fatal("metadata should be initialized")
	}
}

func TestMessageContextBuilder_Chaining(t *testing.T) {
	b := NewMessageContextBuilder()
	result := b.WithChatSession("a", "b", "c", "d", "e", "f")
	if result != b {
		t.Error("builder should return itself for chaining")
	}
}

func TestBoolValue(t *testing.T) {
	tr := true
	fa := false

	if BoolValue(nil, true) != true {
		t.Error("nil ptr should return default true")
	}
	if BoolValue(nil, false) != false {
		t.Error("nil ptr should return default false")
	}
	if BoolValue(&tr, false) != true {
		t.Error("should return true")
	}
	if BoolValue(&fa, true) != false {
		t.Error("should return false")
	}
}

func TestMessageContext_MessageDirection(t *testing.T) {
	if DirectionUserToBot != "user_to_bot" {
		t.Errorf("expected 'user_to_bot', got %s", DirectionUserToBot)
	}
	if DirectionBotToUser != "bot_to_user" {
		t.Errorf("expected 'bot_to_user', got %s", DirectionBotToUser)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	logger := &testLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := withRetry(ctx, logger, "test", func() error {
		return errors.New("should not retry")
	})
	if err == nil {
		t.Error("expected context error")
	}
}

func TestWithRetry_Success(t *testing.T) {
	logger := &testLogger{}
	calls := 0

	err := withRetry(context.Background(), logger, "test", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_Exhausted(t *testing.T) {
	logger := &testLogger{}

	err := withRetry(context.Background(), logger, "test", func() error {
		return errors.New("persistent error")
	})
	if err == nil {
		t.Error("expected error after retry exhaustion")
	}
	if !errors.Is(err, ErrStorageRetryExhausted) {
		t.Errorf("expected ErrStorageRetryExhausted, got %v", err)
	}
}

func TestWithRetry_EventualSuccess(t *testing.T) {
	logger := &testLogger{}
	calls := 0

	err := withRetry(context.Background(), logger, "test", func() error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestNewMessageStorePlugin_NilStore(t *testing.T) {
	_, err := NewMessageStorePlugin(MessageStorePluginConfig{
		Store: nil,
	})
	if err != ErrNilStore {
		t.Errorf("expected ErrNilStore, got %v", err)
	}
}

func TestNewMessageStorePlugin_NilSessionManager(t *testing.T) {
	_, err := NewMessageStorePlugin(MessageStorePluginConfig{
		Store:          &mockStorage{},
		SessionManager: nil,
	})
	if err != ErrNilSessionManager {
		t.Errorf("expected ErrNilSessionManager, got %v", err)
	}
}

func TestSlogLogger(t *testing.T) {
	l := &slogLogger{logger: slog.Default()}
	// Should not panic
	l.Warn("test warn", "key", "value")
	l.Error("test error", "key", "value")
}

func TestRetryConfig_Defaults(t *testing.T) {
	if DefaultRetryConfig.MaxAttempts != 3 {
		t.Errorf("expected 3, got %d", DefaultRetryConfig.MaxAttempts)
	}
	if DefaultRetryConfig.Multiplier != 2.0 {
		t.Errorf("expected 2.0, got %f", DefaultRetryConfig.Multiplier)
	}
}

func TestErrStorageRetryExhausted(t *testing.T) {
	if ErrStorageRetryExhausted.Error() != "storage retry exhausted" {
		t.Errorf("unexpected error message: %s", ErrStorageRetryExhausted.Error())
	}
}

func TestMessageTypeFiltered(t *testing.T) {
	if ErrMessageTypeFiltered.Error() != "message type not allowed" {
		t.Errorf("unexpected error message: %s", ErrMessageTypeFiltered.Error())
	}
}

func TestMessageFilterError(t *testing.T) {
	err := &MessageFilterError{msg: "custom filter"}
	if err.Error() != "custom filter" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// testLogger implements Logger for testing
type testLogger struct {
	warns  []string
	errors []string
}

func (l *testLogger) Warn(msg string, args ...any)  { l.warns = append(l.warns, msg) }
func (l *testLogger) Error(msg string, args ...any) { l.errors = append(l.errors, msg) }
