// Package slack provides the Slack adapter implementation for the hotplex engine.
// Answer message builders for Slack Block Kit.
package slack

import (
	"fmt"
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
)

// Display configuration constants
const (
	// MaxAnswerDisplay is the threshold for collapsed answer display
	// Content longer than this will be collapsed to avoid cluttering the UI
	MaxAnswerDisplay = 1000

	// MaxThinkingDisplay is the threshold for thinking/reasoning message truncation
	// Thinking content is typically shorter than answers
	MaxThinkingDisplay = 500

	// DefaultBoundarySearchLimit is the max characters to search for sentence boundary
	// when finding a good cut point for collapsing answer content
	DefaultBoundarySearchLimit = 100

	// ThinkingBoundarySearchLimit is the search limit for thinking messages (shorter)
	ThinkingBoundarySearchLimit = 50
)

// AnswerMessageBuilder builds answer-related Slack messages
type AnswerMessageBuilder struct {
	config    *Config
	formatter *MrkdwnFormatter
}

// NewAnswerMessageBuilder creates a new AnswerMessageBuilder
func NewAnswerMessageBuilder(config *Config, formatter *MrkdwnFormatter) *AnswerMessageBuilder {
	return &AnswerMessageBuilder{
		config:    config,
		formatter: formatter,
	}
}

// BuildAnswerMessage builds a message for AI answer
func (b *AnswerMessageBuilder) BuildAnswerMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		return nil
	}

	// 1. Process Markdown if enabled (default: true)
	formattedContent := content
	markdownEnabled := BoolValue(b.config.Features.Markdown.Enabled, true)
	if markdownEnabled {
		formattedContent = b.formatter.Format(content)
	}

	// 2. Check if content is too long for comfortable reading
	// Use collapsible display for long content to improve readability
	if len(formattedContent) > MaxAnswerDisplay {
		return b.buildCollapsedAnswerBlocks(formattedContent, MaxAnswerDisplay)
	}

	// 3. Check if chunking is enabled (default: true) for very long content
	chunkingEnabled := BoolValue(b.config.Features.Chunking.Enabled, true)
	maxChars := b.config.Features.Chunking.MaxChars
	if maxChars <= 0 {
		maxChars = 3500 // Default safe limit
	}

	if chunkingEnabled && len(formattedContent) > maxChars {
		return b.buildChunkedAnswerBlocks(formattedContent, maxChars)
	}

	mrkdwn := slack.NewTextBlockObject("mrkdwn", formattedContent, false, false)
	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// buildCollapsedAnswerBlocks creates collapsed blocks for long content
// Shows first maxChars characters with a "show more" indicator
func (b *AnswerMessageBuilder) buildCollapsedAnswerBlocks(content string, maxChars int) []slack.Block {
	// Find a good break point (end of sentence or paragraph)
	cutPoint := maxChars
	remaining := content[maxChars:]

	// Try to find end of sentence (., !, ?) followed by space or newline
	if idx := findSentenceBoundary(remaining, DefaultBoundarySearchLimit); idx > 0 {
		cutPoint = maxChars + idx + 1
	}

	// Ensure we don't exceed content length
	if cutPoint > len(content) {
		cutPoint = len(content)
	}

	displayContent := content[:cutPoint]
	totalLen := len(content)

	// Add "show more" indicator
	indicator := fmt.Sprintf("\n\n---\n📄 _显示部分内容 (%d/%d 字符，已自动截断)_", cutPoint, totalLen)
	displayContent += indicator

	mrkdwn := slack.NewTextBlockObject("mrkdwn", displayContent, false, false)
	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// findSentenceBoundary finds the end of a sentence in text
// Returns index of sentence boundary (relative to start of text)
func findSentenceBoundary(text string, maxSearch int) int {
	end := len(text)
	if maxSearch > 0 && maxSearch < end {
		end = maxSearch
	}

	for i := 0; i < end; i++ {
		c := text[i]
		if c == '.' || c == '!' || c == '?' {
			// Check if followed by space or newline
			if i+1 < len(text) {
				next := text[i+1]
				if next == ' ' || next == '\n' || next == '\r' {
					return i
				}
			}
		}
	}
	return -1
}

// buildChunkedAnswerBlocks splits very long content into chunks
func (b *AnswerMessageBuilder) buildChunkedAnswerBlocks(content string, maxChars int) []slack.Block {
	var blocks []slack.Block

	chunks := b.chunkText(content, maxChars)
	for i, chunk := range chunks {
		if i > 0 {
			// Add divider between chunks
			blocks = append(blocks, slack.NewDividerBlock())
		}
		mrkdwn := slack.NewTextBlockObject("mrkdwn", chunk, false, false)
		blocks = append(blocks, slack.NewSectionBlock(mrkdwn, nil, nil))
	}

	return blocks
}

// chunkText splits text into chunks at word boundaries
func (b *AnswerMessageBuilder) chunkText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	currentChunk := ""

	for _, line := range lines {
		if len(currentChunk)+len(line)+1 > maxLen {
			if currentChunk != "" {
				chunks = append(chunks, currentChunk)
				currentChunk = ""
			}
		}
		if currentChunk != "" {
			currentChunk += "\n"
		}
		currentChunk += line
	}

	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// BuildErrorMessage builds a message for errors
// Implements EventTypeError per spec - uses quote format for emphasis
func (b *AnswerMessageBuilder) BuildErrorMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		content = "An error occurred"
	}

	// Use quote format (> ) per spec for emphasis
	// Split content by newlines and add > prefix to each line
	lines := strings.Split(content, "\n")
	var quotedLines []string
	for _, line := range lines {
		quotedLines = append(quotedLines, "> "+line)
	}
	quotedContent := strings.Join(quotedLines, "\n")

	text := ":warning: *Error*\n" + quotedContent
	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// BuildThinkingMessage builds a collapsed message for thinking/reasoning content
// Thinking content is shown collapsed by default to avoid cluttering the conversation
func (b *AnswerMessageBuilder) BuildThinkingMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		return nil
	}

	// Format thinking content with markdown if enabled
	formattedContent := content
	markdownEnabled := BoolValue(b.config.Features.Markdown.Enabled, true)
	if markdownEnabled {
		formattedContent = b.formatter.Format(content)
	}

	// Truncate if too long (>MaxThinkingDisplay chars for thinking)
	if len(formattedContent) > MaxThinkingDisplay {
		cutPoint := MaxThinkingDisplay
		if idx := findSentenceBoundary(formattedContent[cutPoint:], ThinkingBoundarySearchLimit); idx > 0 {
			cutPoint += idx + 1
		}
		if cutPoint > len(formattedContent) {
			cutPoint = len(formattedContent)
		}
		formattedContent = formattedContent[:cutPoint] + "..."
	}

	// Build collapsed thinking block with distinctive styling
	headerText := slack.NewTextBlockObject("plain_text", "💭 推理过程", false, false)
	header := slack.NewHeaderBlock(headerText)

	contentText := slack.NewTextBlockObject("mrkdwn", formattedContent, false, false)
	section := slack.NewSectionBlock(contentText, nil, nil)

	// Add context showing it's collapsed reasoning
	contextText := slack.NewTextBlockObject("mrkdwn", "_🧠 AI 的推理过程（已折叠）_", false, false)
	context := slack.NewContextBlock("", contextText)

	return []slack.Block{header, section, context}
}
