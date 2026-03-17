package feishu

import (
	"context"
	"io"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

// =============================================================================
// StreamingWriter Tests
// =============================================================================

// TestStreamingWriter_BasicFlow tests the basic streaming flow
func TestStreamingWriter_BasicFlow(t *testing.T) {
	// Create mock adapter
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	// Create streaming writer
	ctx := context.Background()
	var completedMessageID string
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", func(messageID string) {
		completedMessageID = messageID
	})

	// Verify initial state
	assert.False(t, writer.IsStarted())
	assert.False(t, writer.IsClosed())

	// Write some content
	content1 := "Hello, "
	n, err := writer.Write([]byte(content1))
	require.NoError(t, err)
	assert.Equal(t, len(content1), n)

	// After first write, stream should be started
	assert.True(t, writer.IsStarted())

	// Write more content
	content2 := "World!"
	n, err = writer.Write([]byte(content2))
	require.NoError(t, err)
	assert.Equal(t, len(content2), n)

	// Close the writer
	err = writer.Close()
	require.NoError(t, err)

	// Verify closed state
	assert.True(t, writer.IsClosed())
	assert.NotEmpty(t, completedMessageID)

	// Get stats
	stats := writer.GetStats()
	assert.Equal(t, int64(len(content1)+len(content2)), stats.BytesWritten)
	assert.True(t, stats.IntegrityOK)
}

// TestStreamingWriter_MultipleWrites tests multiple small writes
func TestStreamingWriter_MultipleWrites(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)
	defer writer.Close()

	// Write multiple small chunks
	chunks := []string{"A", "B", "C", "D", "E"}
	var totalBytes int
	for _, chunk := range chunks {
		n, err := writer.Write([]byte(chunk))
		require.NoError(t, err)
		assert.Equal(t, len(chunk), n)
		totalBytes += len(chunk)
	}

	// Verify stats
	stats := writer.GetStats()
	assert.Equal(t, int64(totalBytes), stats.BytesWritten)
}

// TestStreamingWriter_EmptyWrite tests writing empty content
func TestStreamingWriter_EmptyWrite(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)
	defer writer.Close()

	// Write empty content
	n, err := writer.Write([]byte{})
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Stream should not be started
	assert.False(t, writer.IsStarted())
}

// TestStreamingWriter_WriteAfterClose tests writing after stream is closed
func TestStreamingWriter_WriteAfterClose(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)

	// Close the writer
	err := writer.Close()
	require.NoError(t, err)

	// Try to write after close
	_, err = writer.Write([]byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already closed")
}

// TestStreamingWriter_DoubleClose tests closing the writer multiple times
func TestStreamingWriter_DoubleClose(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)

	// First close
	err := writer.Close()
	require.NoError(t, err)

	// Second close should not error
	err = writer.Close()
	require.NoError(t, err)
}

// TestStreamingWriter_StoreCallback tests the store callback functionality
func TestStreamingWriter_StoreCallback(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)

	var storedContent string
	writer.SetStoreCallback(func(content string) {
		storedContent = content
	})

	// Write content
	content := "Test content for storage"
	_, err := writer.Write([]byte(content))
	require.NoError(t, err)

	// Close should trigger store callback
	err = writer.Close()
	require.NoError(t, err)

	// Verify callback was called with correct content
	assert.Equal(t, content, storedContent)
}

// TestStreamingWriter_Interface tests that StreamingWriter implements io.WriteCloser
func TestStreamingWriter_Interface(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)

	// Verify interface compliance
	var _ io.Writer = writer
	var _ io.Closer = writer
	var _ io.WriteCloser = writer
	var _ base.StreamWriter = writer
}

// TestStreamingWriter_MessageTS tests MessageTS method
func TestStreamingWriter_MessageTS(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)

	// Initially empty
	assert.Empty(t, writer.MessageTS())

	// Write to start stream
	_, err := writer.Write([]byte("test"))
	require.NoError(t, err)

	// Should have message ID now
	assert.NotEmpty(t, writer.MessageTS())
	assert.Equal(t, writer.MessageID(), writer.MessageTS())
}

// TestStreamingWriter_FallbackUsed tests FallbackUsed method
func TestStreamingWriter_FallbackUsed(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, logger),
		client:  &MockFeishuClient{},
	}

	ctx := context.Background()
	writer := NewStreamingWriter(ctx, adapter, "test_chat_id", nil)
	defer writer.Close()

	// Feishu's streaming doesn't use fallback (card updates have no limit)
	assert.False(t, writer.FallbackUsed())
}

// =============================================================================
// Mock Client for Testing
// =============================================================================

// MockFeishuClient implements FeishuAPIClient for testing
type MockFeishuClient struct{}

func (m *MockFeishuClient) GetAppTokenWithContext(ctx context.Context) (string, int, error) {
	return "mock_token", 7200, nil
}

func (m *MockFeishuClient) SendMessage(ctx context.Context, token, chatID, msgType string, content map[string]string) (string, error) {
	return "mock_message_id", nil
}

func (m *MockFeishuClient) SendTextMessage(ctx context.Context, token, chatID, text string) (string, error) {
	return "mock_message_id", nil
}

func (m *MockFeishuClient) SendInteractiveMessage(ctx context.Context, token, chatID, cardJSON string) (string, error) {
	return "mock_message_id", nil
}

func (m *MockFeishuClient) CreateCard(ctx context.Context, token string, card *CardTemplate) (string, error) {
	return "mock_card_id", nil
}

func (m *MockFeishuClient) UpdateCard(ctx context.Context, token, cardID string, card *CardTemplate, sequence int) error {
	return nil
}

func (m *MockFeishuClient) SendCardMessage(ctx context.Context, token, chatID, cardID string) (string, error) {
	return "mock_message_id", nil
}
