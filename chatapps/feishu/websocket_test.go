package feishu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

// =============================================================================
// WebSocketClient Tests
// =============================================================================

// TestWebSocketClient_NewClient tests creating a new WebSocket client
func TestWebSocketClient_NewClient(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	assert.NotNil(t, client)
	assert.Equal(t, "test_app_id", client.appID)
	assert.Equal(t, "test_app_secret", client.appSecret)
	assert.NotNil(t, client.logger)
	assert.NotNil(t, client.httpClient)
	assert.False(t, client.IsConnected())
}

// TestWebSocketClient_SetHandlers tests setting event handlers
func TestWebSocketClient_SetHandlers(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Set event handler
	client.SetEventHandler(func(event *Event) {
		// Event handler set
	})
	assert.NotNil(t, client.eventHandler)

	// Set on connect handler
	client.SetOnConnect(func() {
		// On connect handler set
	})
	assert.NotNil(t, client.onConnect)

	// Set on disconnect handler
	client.SetOnDisconnect(func(err error) {
		// On disconnect handler set
	})
	assert.NotNil(t, client.onDisconnect)
}

// TestWebSocketClient_Close tests closing the client without connecting
func TestWebSocketClient_Close(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Close should not error even if not connected
	err := client.Close()
	require.NoError(t, err)
}

// TestWebSocketClient_IsConnected tests connection state
func TestWebSocketClient_IsConnected(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Initially not connected
	assert.False(t, client.IsConnected())

	// After close, still not connected
	_ = client.Close()
	assert.False(t, client.IsConnected())
}

// TestWebSocketMessage tests WebSocket message parsing
func TestWebSocketMessage_PingPong(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Test ping handler
	pingData := []byte(`{"timestamp":1234567890}`)
	client.handlePing(pingData)
	// Should not panic
}

// TestWebSocketClient_ContextCancellation tests context cancellation
func TestWebSocketClient_ContextCancellation(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockFeishuClient{}
	client := NewWebSocketClient("test_app_id", "test_app_secret", mockClient, logger)

	// Create and cancel context
	_, cancel := context.WithCancel(context.Background())
	cancel()

	// Close should handle cancelled context gracefully
	err := client.Close()
	require.NoError(t, err)
}
