package chatapps

import (
	"context"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

func TestChunkProcessor_ShortMessage(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{MaxChars: 4000})

	msg := &base.ChatMessage{Content: "short message"}
	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "short message" {
		t.Errorf("content should be unchanged, got %q", result.Content)
	}
}

func TestChunkProcessor_NilMessage(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{MaxChars: 100})

	result, err := p.Process(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for nil message")
	}
}

func TestChunkProcessor_EmptyContent(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{MaxChars: 100})

	msg := &base.ChatMessage{Content: ""}
	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}
}

func TestChunkProcessor_BlocksNoChunk(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{MaxChars: 100})

	msg := &base.ChatMessage{
		RichContent: &base.RichContent{Blocks: []any{"block1", "block2"}},
	}
	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestChunkProcessor_LongMessage(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{MaxChars: 100})

	// Create a message that exceeds 100 chars
	content := make([]byte, 200)
	for i := range content {
		content[i] = 'a'
	}
	msg := &base.ChatMessage{
		Content: string(content),
		Metadata: map[string]any{
			base.KeyRoomID: "C123",
			"thread_ts":    "12345.6789",
			"channel_id":   "C123",
		},
	}

	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// First chunk should be in Content
	if result.Content == "" {
		t.Error("first chunk should have content")
	}

	// Should have chunk_info in metadata
	info := GetChunkInfo(result)
	if info == nil {
		t.Fatal("expected chunk info in metadata")
	}
	if info.TotalChunks <= 1 {
		t.Errorf("expected multiple chunks, got %d", info.TotalChunks)
	}
	if info.CurrentChunk != 1 {
		t.Errorf("expected current chunk 1, got %d", info.CurrentChunk)
	}

	// Should have extra_chunks in metadata
	extra := GetExtraChunks(result)
	if extra == nil {
		t.Error("expected extra chunks in metadata")
	}

	// After retrieval, extra_chunks should be cleared
	extra2 := GetExtraChunks(result)
	if extra2 != nil {
		t.Error("extra_chunks should be cleared after retrieval")
	}
}

func TestGetExtraChunks_NilMessage(t *testing.T) {
	chunks := GetExtraChunks(nil)
	if chunks != nil {
		t.Error("expected nil for nil message")
	}
}

func TestGetExtraChunks_NoMetadata(t *testing.T) {
	msg := &base.ChatMessage{}
	chunks := GetExtraChunks(msg)
	if chunks != nil {
		t.Error("expected nil for message without metadata")
	}
}

func TestGetExtraChunks_WrongType(t *testing.T) {
	msg := &base.ChatMessage{Metadata: map[string]any{"extra_chunks": "not a slice"}}
	chunks := GetExtraChunks(msg)
	if chunks != nil {
		t.Error("expected nil for wrong type")
	}
}

func TestGetChunkInfo_NilMessage(t *testing.T) {
	info := GetChunkInfo(nil)
	if info != nil {
		t.Error("expected nil for nil message")
	}
}

func TestGetChunkInfo_NoMetadata(t *testing.T) {
	info := GetChunkInfo(&base.ChatMessage{})
	if info != nil {
		t.Error("expected nil for message without metadata")
	}
}

func TestGetChunkInfo_WrongType(t *testing.T) {
	info := GetChunkInfo(&base.ChatMessage{Metadata: map[string]any{"chunk_info": "not ChunkInfo"}})
	if info != nil {
		t.Error("expected nil for wrong type")
	}
}

func TestChunkProcessor_Name(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{})
	if p.Name() != "ChunkProcessor" {
		t.Errorf("expected 'ChunkProcessor', got %q", p.Name())
	}
}

func TestChunkProcessor_Order(t *testing.T) {
	p := NewChunkProcessor(nil, ChunkProcessorOptions{})
	if p.Order() <= 0 {
		t.Errorf("expected positive order, got %d", p.Order())
	}
}
