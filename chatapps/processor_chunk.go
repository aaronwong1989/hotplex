package chatapps

import (
	"context"
	"log/slog"
	"unicode/utf8"

	"github.com/hrygo/hotplex/chatapps/base"
)

// ChunkProcessor splits long messages into chunks that fit within platform limits.
// It delegates platform-specific chunking to an injected Chunker.
type ChunkProcessor struct {
	logger   *slog.Logger
	chunker  base.Chunker
	maxChars int
}

// ChunkInfo holds metadata about chunked messages.
type ChunkInfo struct {
	TotalChunks  int
	CurrentChunk int
	ThreadTS     string
	ChannelID    string
}

// ChunkProcessorOptions configures the ChunkProcessor.
type ChunkProcessorOptions struct {
	MaxChars int // Maximum characters per chunk (default: 4000 for Slack)
	Chunker  base.Chunker
}

// NewChunkProcessor creates a new ChunkProcessor.
func NewChunkProcessor(logger *slog.Logger, opts ChunkProcessorOptions) *ChunkProcessor {
	if logger == nil {
		logger = slog.Default()
	}

	chunker := opts.Chunker
	if chunker == nil {
		chunker = base.NewDefaultChunker(opts.MaxChars)
	}

	return &ChunkProcessor{
		logger:   logger,
		chunker:  chunker,
		maxChars: chunker.MaxChars(),
	}
}

// Name returns the processor name.
func (p *ChunkProcessor) Name() string {
	return "ChunkProcessor"
}

// Order returns the processor order (runs after format conversion).
func (p *ChunkProcessor) Order() int {
	return int(OrderChunk)
}

// Process splits the message into chunks if it exceeds the character limit.
// Returns either a single message or a slice of messages.
func (p *ChunkProcessor) Process(ctx context.Context, msg *base.ChatMessage) (*base.ChatMessage, error) {
	if msg == nil {
		return nil, nil
	}

	// Get content to check length
	content := msg.Content
	if content == "" {
		// If no text content but has blocks, check blocks content
		if msg.RichContent != nil && len(msg.RichContent.Blocks) > 0 {
			// For blocks, we don't chunk - return as-is
			return msg, nil
		}
		return msg, nil
	}

	// Check if chunking is needed
	contentLen := utf8.RuneCountInString(content)
	if contentLen <= p.maxChars {
		return msg, nil
	}

	p.logger.Debug("ChunkProcessor: message exceeds limit, chunking",
		"content_len", contentLen,
		"max_chars", p.maxChars)

	// Get thread_ts and channel_id from metadata for all chunks (unified keys with Slack fallback)
	channelID, threadTS, _ := base.SlackMetadata(msg.Metadata)

	// Delegate to injected chunker for platform-specific logic
	chunks := p.chunker.ChunkText(content, p.maxChars)

	if len(chunks) == 1 {
		// Should not happen, but handle gracefully
		return msg, nil
	}

	firstChunk := chunks[0]
	extraChunks := chunks[1:]

	// Update the message with first chunk
	msg.Content = firstChunk

	// Add chunk info to metadata
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}

	msg.Metadata["chunk_info"] = ChunkInfo{
		TotalChunks:  len(chunks),
		CurrentChunk: 1,
		ThreadTS:     threadTS,
		ChannelID:    channelID,
	}

	// Store extra chunks in metadata for adapter to send
	if len(extraChunks) > 0 {
		msg.Metadata["extra_chunks"] = extraChunks
	}

	p.logger.Debug("ChunkProcessor: created chunks",
		"total", len(chunks),
		"first_len", len(firstChunk))

	return msg, nil
}

// GetExtraChunks returns any extra chunks stored in message metadata.
// Returns nil if no extra chunks exist.
func GetExtraChunks(msg *base.ChatMessage) []string {
	if msg == nil || msg.Metadata == nil {
		return nil
	}

	chunks, ok := msg.Metadata["extra_chunks"].([]string)
	if !ok {
		return nil
	}

	// Clear the extra chunks after retrieval to prevent double-sending
	delete(msg.Metadata, "extra_chunks")

	return chunks
}

// GetChunkInfo returns the ChunkInfo from message metadata.
func GetChunkInfo(msg *base.ChatMessage) *ChunkInfo {
	if msg == nil || msg.Metadata == nil {
		return nil
	}

	info, ok := msg.Metadata["chunk_info"].(ChunkInfo)
	if !ok {
		return nil
	}

	return &info
}
