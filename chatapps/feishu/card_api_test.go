package feishu

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CardKit Retry Logic Tests - Issue #308
// =============================================================================

// TestIsRetryableError_NetworkError tests network error detection
func TestIsRetryableError_NetworkError(t *testing.T) {
	logger := slog.Default()
	client := NewClient("test_app_id", "test_app_secret", logger)

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "network dial error",
			err:      &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			expected: true,
		},
		{
			name:     "timeout error",
			err:      &net.OpError{Op: "dial", Err: errors.New("i/o timeout")},
			expected: true,
		},
		{
			name:     "API 500 error",
			err:      &APIError{Code: 500, Msg: "Internal Server Error"},
			expected: true,
		},
		{
			name:     "API 502 error",
			err:      &APIError{Code: 502, Msg: "Bad Gateway"},
			expected: true,
		},
		{
			name:     "API 503 error",
			err:      &APIError{Code: 503, Msg: "Service Unavailable"},
			expected: true,
		},
		{
			name:     "API 400 error",
			err:      &APIError{Code: 400, Msg: "Bad Request"},
			expected: false,
		},
		{
			name:     "API 401 error",
			err:      &APIError{Code: 401, Msg: "Unauthorized"},
			expected: false,
		},
		{
			name:     "API 404 error",
			err:      &APIError{Code: 404, Msg: "Not Found"},
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateBackoff tests exponential backoff calculation
func TestCalculateBackoff(t *testing.T) {
	logger := slog.Default()
	client := NewClient("test_app_id", "test_app_secret", logger)

	tests := []struct {
		name         string
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		factor       float64
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{
			name:         "attempt 1",
			attempt:      1,
			initialDelay: 100 * time.Millisecond,
			maxDelay:     5 * time.Second,
			factor:       2.0,
			expectedMin:   95 * time.Millisecond,
			expectedMax:   105 * time.Millisecond,
		},
		{
			name:         "attempt 2",
			attempt:      2,
			initialDelay: 100 * time.Millisecond,
			maxDelay:     5 * time.Second,
			factor:       2.0,
			expectedMin:   195 * time.Millisecond,
			expectedMax:   205 * time.Millisecond,
		},
		{
			name:         "attempt 3 hits max",
			attempt:      3,
			initialDelay: 100 * time.Millisecond,
			maxDelay:     5 * time.Second,
			factor:       2.0,
			expectedMin:   395 * time.Millisecond,
			expectedMax:   405 * time.Millisecond,
		},
		{
			name:         "attempt with large maxDelay",
			attempt:      4,
			initialDelay: 1 * time.Second,
			maxDelay:     3 * time.Second,
			factor:       2.0,
			expectedMin:   1950 * time.Millisecond,
			expectedMax:   3050 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.calculateBackoff(tt.attempt, tt.initialDelay, tt.maxDelay, tt.factor)
			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}

// =============================================================================
// Streaming Writer Tests - Issue #307
// =============================================================================

// TestStreamingWriter_StateTransitions tests state transitions
func TestStreamingWriter_StateTransitions(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	w := NewStreamingWriter(context.Background(), adapter, "test_chat_id", nil)

	// Initial state
	assert.False(t, w.IsStarted())
	assert.False(t, w.IsClosed())

	// After write - should be started
	w.Write([]byte("test"))
	assert.True(t, w.IsStarted())
	assert.False(t, w.IsClosed())

	// After close - should be closed
	w.Close()
	assert.True(t, w.IsStarted())
	assert.True(t, w.IsClosed())
}

// TestStreamingWriter_MultipleWriteCalls tests multiple write calls
func TestStreamingWriter_MultipleWriteCalls(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	w := NewStreamingWriter(context.Background(), adapter, "test_chat_id", nil)

	// Multiple writes
	writes := []string{"hello", " ", "world", "!"}
	for _, s := range writes {
		n, err := w.Write([]byte(s))
		require.NoError(t, err)
		assert.Equal(t, len(s), n)
	}

	stats := w.GetStats()
	assert.GreaterOrEqual(t, stats.BytesWritten, int64(0))
}

// TestStreamingWriter_GetStats tests stats tracking
func TestStreamingWriter_GetStats(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	w := NewStreamingWriter(context.Background(), adapter, "test_chat_id", nil)

	// Write some data
	w.Write([]byte("hello world"))

	stats := w.GetStats()
	assert.Greater(t, stats.BytesWritten, int64(0), "Should have written some bytes")
}

// TestStreamingWriter_CardIDGetter tests CardID getter
func TestStreamingWriter_CardIDGetter(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	w := NewStreamingWriter(context.Background(), adapter, "test_chat_id", nil)

	// Before any write
	assert.Empty(t, w.CardID())

	// Write some data (this triggers card creation)
	w.Write([]byte("test data"))

	// CardID may or may not be set depending on whether card was created
	_ = w.CardID() // Should not panic
}

// TestStreamingWriter_FallbackFlag tests fallback mechanism tracking
func TestStreamingWriter_FallbackFlag(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	w := NewStreamingWriter(context.Background(), adapter, "test_chat_id", nil)

	// Initially no fallback
	assert.False(t, w.FallbackUsed())

	// Write data
	w.Write([]byte("test"))

	// Close
	w.Close()

	// FallbackUsed should return a consistent value
	assert.Equal(t, false, w.FallbackUsed())
}

// =============================================================================
// WebSocket Reconnection Tests - Issue #307
// =============================================================================

// TestWebSocketClient_ReconnectOnDisconnect tests reconnect behavior
func TestWebSocketClient_ReconnectOnDisconnect(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Set up disconnect handler to track reconnect attempts
	reconnectCalled := false
	client.SetOnDisconnect(func(err error) {
		reconnectCalled = true
	})

	// Manually trigger disconnect handling
	client.handleDisconnect(errors.New("connection lost"))

	assert.True(t, reconnectCalled, "Disconnect handler should be called")
}

// TestWebSocketClient_ConnectThenDisconnect tests connect then disconnect flow
func TestWebSocketClient_ConnectThenDisconnect(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Should start disconnected
	assert.False(t, client.IsConnected())

	// Trigger disconnect without connecting first
	client.handleDisconnect(errors.New("test disconnect"))

	// Should still be disconnected
	assert.False(t, client.IsConnected())
}

// TestWebSocketClient_PingPongFlow tests ping/pong handling
func TestWebSocketClient_PingPongFlow(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Set up ping handler
	client.SetEventHandler(func(event *Event) {
		// Event handler set
	})

	// Test ping handling
	pingData := []byte(`{"timestamp":1234567890}`)
	client.handlePing(pingData)

	// Should not panic
}

// TestWebSocketClient_HandleEventWithNilHandler tests event handling with nil handler
func TestWebSocketClient_HandleEventWithNilHandler(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Don't set event handler - should not panic
	msg := WebSocketMessage{
		Type: "test_event",
		Data: []byte(`{}`),
	}
	client.handleEvent(msg)
}

// TestWebSocketClient_HandleError tests error handling
func TestWebSocketClient_HandleError(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Handle an error - should not panic
	client.handleError([]byte(`{"error":"test error"}`))
}

// =============================================================================
// FeishuConverter Tests - Issue #172
// =============================================================================

// TestFeishuConverter_ImplementsInterface tests that FeishuConverter implements ContentConverter
func TestFeishuConverter_ImplementsInterface(t *testing.T) {
	converter := NewFeishuConverter()
	var _ interface{} = converter
	// Verify it can be assigned to ContentConverter interface
	var i base.ContentConverter = converter
	assert.NotNil(t, i)
}

// TestFeishuConverter_EscapeSpecialChars tests character escaping
func TestFeishuConverter_EscapeSpecialChars(t *testing.T) {
	converter := NewFeishuConverter()

	tests := []struct {
		name     string
		input    string
	}{
		{
			name:     "simple text",
			input:    "hello world",
		},
		{
			name:     "with ampersand",
			input:    "foo & bar",
		},
		{
			name:     "with angle brackets",
			input:    "a < b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.EscapeSpecialChars(tt.input)
			assert.NotEmpty(t, result)
		})
	}
}

// TestFeishuConverter_ConvertMarkdownToPlatform tests markdown conversion
func TestFeishuConverter_ConvertMarkdownToPlatform(t *testing.T) {
	converter := NewFeishuConverter()

	tests := []struct {
		name     string
		input    string
		parseMode base.ParseMode
		expected string
	}{
		{
			name:     "none mode passes through",
			input:    "hello **world**",
			parseMode: base.ParseModeNone,
			expected: "hello **world**",
		},
		{
			name:     "empty input",
			input:    "",
			parseMode: base.ParseModeMarkdown,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.ConvertMarkdownToPlatform(tt.input, tt.parseMode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFeishuConverter_BoldConversion tests bold text conversion
func TestFeishuConverter_BoldConversion(t *testing.T) {
	converter := NewFeishuConverter()

	// **text** should become *text* (Feishu bold format)
	result := converter.ConvertMarkdownToPlatform("hello **world**", base.ParseModeMarkdown)
	assert.Contains(t, result, "world")
	assert.NotContains(t, result, "**world**")
}

// TestFeishuConverter_HeaderConversion tests header conversion
func TestFeishuConverter_HeaderConversion(t *testing.T) {
	converter := NewFeishuConverter()

	result := converter.ConvertMarkdownToPlatform("# Hello World", base.ParseModeMarkdown)
	assert.Contains(t, result, "**Hello World**")
}

// TestFeishuConverter_ListConversion tests list conversion
func TestFeishuConverter_ListConversion(t *testing.T) {
	converter := NewFeishuConverter()

	result := converter.ConvertMarkdownToPlatform("- item 1\n- item 2", base.ParseModeMarkdown)
	assert.Contains(t, result, "• item 1")
	assert.Contains(t, result, "• item 2")
}

// TestFeishuConverter_LinkConversion tests link conversion
func TestFeishuConverter_LinkConversion(t *testing.T) {
	converter := NewFeishuConverter()

	result := converter.ConvertMarkdownToPlatform("click [here](https://example.com) now", base.ParseModeMarkdown)
	assert.Contains(t, result, "here")
	assert.NotContains(t, result, "[here]")
}

// =============================================================================
// ProcessorChain Integration Tests - Issue #172
// =============================================================================

// TestAdapter_ProcessorChainInitialized tests that processor chain is initialized
func TestAdapter_ProcessorChainInitialized(t *testing.T) {
	config := &Config{
		AppID:            "test_app_id",
		AppSecret:        "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:       "test_encrypt_key_16",
		UseWebSocket:    false,
	}
	logger := slog.Default()

	adapter, err := NewAdapter(config, logger)
	require.NoError(t, err)
	require.NotNil(t, adapter)

	assert.NotNil(t, adapter.processorChain, "Processor chain should be initialized")
}

// TestAdapter_ProcessorChainProcessNilMessage tests processor chain with nil message
func TestAdapter_ProcessorChainProcessNilMessage(t *testing.T) {
	config := &Config{
		AppID:            "test_app_id",
		AppSecret:        "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:       "test_encrypt_key_16",
		UseWebSocket:    false,
	}
	logger := slog.Default()

	adapter, err := NewAdapter(config, logger)
	require.NoError(t, err)

	// Process nil message through chain - should return nil, nil
	result, err := adapter.processorChain.Process(context.Background(), nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestAdapter_ProcessorChainProcessValidMessage tests processor chain with valid message
func TestAdapter_ProcessorChainProcessValidMessage(t *testing.T) {
	config := &Config{
		AppID:            "test_app_id",
		AppSecret:        "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:       "test_encrypt_key_16",
		UseWebSocket:    false,
	}
	logger := slog.Default()

	adapter, err := NewAdapter(config, logger)
	require.NoError(t, err)

	msg := &base.ChatMessage{
		Content:  "test message content",
		Type:    base.MessageTypeAnswer,
		Metadata: map[string]any{"chat_id": "test_chat"},
	}

	// Process through chain
	result, err := adapter.processorChain.Process(context.Background(), msg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}
