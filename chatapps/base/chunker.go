package base

import (
	"fmt"
	"unicode/utf8"
)

// ChunkerConfig holds configuration for message chunking.
type ChunkerConfig struct {
	// MaxLen is the maximum character limit per chunk.
	// If 0, defaults to DefaultChunkLimit.
	MaxLen int

	// PreserveWords attempts to break at word boundaries.
	// If false, may break in the middle of words.
	PreserveWords bool

	// AddNumbering prefixes chunks with [1/N] notation.
	AddNumbering bool
}

// DefaultChunkLimit is the default maximum chunk size.
const DefaultChunkLimit = 4000

// ChunkMessage splits a text message into chunks that fit within the limit.
// It attempts to split at word boundaries to avoid breaking words.
// Each chunk is prefixed with [chunkNum/totalChunks] if numbering is enabled.
func ChunkMessage(text string, cfg ChunkerConfig) []string {
	limit := cfg.MaxLen
	if limit <= 0 {
		limit = DefaultChunkLimit
	}

	if text == "" || utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	runes := []rune(text)
	totalRunes := len(runes)

	// Reserve space for numbering prefix if enabled
	chunkSize := limit
	if cfg.AddNumbering {
		chunkSize = limit - 15 // Reserve space for "[999/999]\n" prefix
	}

	var chunks []string

	for i := 0; i < totalRunes; i += chunkSize {
		end := i + chunkSize
		if end > totalRunes {
			end = totalRunes
		}

		// Try to break at word boundary if enabled
		if cfg.PreserveWords && end < totalRunes {
			chunkRunes := runes[i:end]

			// Find last newline in chunk
			lastNewline := -1
			for j := len(chunkRunes) - 1; j >= 0; j-- {
				if chunkRunes[j] == '\n' {
					lastNewline = j
					break
				}
			}
			if lastNewline > 0 {
				end = i + lastNewline + 1
			} else {
				// Find last space in chunk
				lastSpace := -1
				for j := len(chunkRunes) - 1; j >= 0; j-- {
					if chunkRunes[j] == ' ' {
						lastSpace = j
						break
					}
				}
				if lastSpace > len(chunkRunes)/2 {
					end = i + lastSpace
				}
			}
		}

		chunks = append(chunks, string(runes[i:end]))
	}

	// Add chunk numbering if enabled
	if !cfg.AddNumbering || len(chunks) <= 1 {
		return chunks
	}

	result := make([]string, len(chunks))
	for i, chunk := range chunks {
		result[i] = fmt.Sprintf("[%d/%d]\n%s", i+1, len(chunks), chunk)
	}
	return result
}

// ChunkMessageSimple splits text by byte length without word boundary preservation.
// Use this for platforms that don't need word-aware splitting.
func ChunkMessageSimple(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = DefaultChunkLimit
	}

	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > maxLen {
		chunks = append(chunks, text[:maxLen])
		text = text[maxLen:]
	}
	if len(text) > 0 {
		chunks = append(chunks, text)
	}
	return chunks
}

// Chunker 将消息分块以满足平台字符限制
type Chunker interface {
	// ChunkText 将文本分块，每块不超过平台限制
	ChunkText(text string, limit int) []string

	// MaxChars 返回平台单条消息的最大字符数
	MaxChars() int
}

// DefaultChunker 使用标准 Markdown 感知的分块算法，默认使用 Slack 限制
type DefaultChunker struct {
	maxChars int
}

// NewDefaultChunker 创建默认分块器
func NewDefaultChunker(maxChars int) *DefaultChunker {
	if maxChars <= 0 {
		maxChars = DefaultChunkLimit
	}
	return &DefaultChunker{maxChars: maxChars}
}

// ChunkText 使用标准算法分块
func (c *DefaultChunker) ChunkText(text string, limit int) []string {
	return ChunkMessage(text, ChunkerConfig{MaxLen: limit, PreserveWords: true, AddNumbering: true})
}

// MaxChars 返回配置的最大字符数
func (c *DefaultChunker) MaxChars() int { return c.maxChars }
