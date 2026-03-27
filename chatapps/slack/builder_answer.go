// Package slack provides the Slack adapter implementation for the hotplex engine.
// Answer message builders for Slack Block Kit.
package slack

import (
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
)

// AnswerMessageBuilder builds answer-related Slack messages
type AnswerMessageBuilder struct {
	config            *Config
	formatter         *MrkdwnFormatter
	markdownConverter *MarkdownConverter
}

// NewAnswerMessageBuilder creates a new AnswerMessageBuilder
func NewAnswerMessageBuilder(config *Config, formatter *MrkdwnFormatter) *AnswerMessageBuilder {
	// Initialize MarkdownConverter with feature flags
	converterConfig := ConverterConfig{
		EnableTables:      isMarkdownTableEnabled(config),
		EnableCodeBlocks:  isCodeBlockEnabled(config),
		EnableQuotes:      isQuoteEnabled(config),
		EnableLists:       false, // Lists not yet implemented
		MaxTableRows:      getMaxTableRows(config),
		MaxCodeBlockLines: getMaxCodeBlockLines(config),
	}

	return &AnswerMessageBuilder{
		config:            config,
		formatter:         formatter,
		markdownConverter: NewMarkdownConverter(converterConfig),
	}
}

// BuildAnswerMessage builds a message for AI answer
func (b *AnswerMessageBuilder) BuildAnswerMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		return nil
	}

	// Check if enhanced Markdown features are enabled
	if b.hasEnhancedMarkdown() {
		// Use MarkdownConverter for table/code block/quote support
		return b.markdownConverter.ConvertToBlocks(content)
	}

	// Fallback to traditional MrkdwnFormatter flow
	// 1. Process Markdown if enabled (default: true)
	formattedContent := content
	markdownEnabled := BoolValue(b.config.Features.Markdown.Enabled, true)
	if markdownEnabled {
		formattedContent = b.formatter.Format(content)
	}

	// 2. Check if chunking is enabled (default: true)
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

// buildChunkedAnswerBlocks splits long content into chunks
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

// hasEnhancedMarkdown checks if any enhanced Markdown features are enabled
func (b *AnswerMessageBuilder) hasEnhancedMarkdown() bool {
	return isMarkdownTableEnabled(b.config) ||
		isCodeBlockEnabled(b.config) ||
		isQuoteEnabled(b.config)
}

// isMarkdownFeatureEnabled is a generic helper to check if a Markdown feature is enabled
// Returns true by default (opt-out strategy)
func isMarkdownFeatureEnabled(config *Config, getEnabled func() *bool) bool {
	if config == nil {
		return true // Default enabled
	}
	enabled := getEnabled()
	return BoolValue(enabled, true)
}

// isMarkdownTableEnabled checks if table conversion is enabled
// Default: true (opt-out strategy - return true unless explicitly disabled)
func isMarkdownTableEnabled(config *Config) bool {
	return isMarkdownFeatureEnabled(config, func() *bool {
		if config.Features.Markdown.TableConfig == nil {
			return nil
		}
		return config.Features.Markdown.TableConfig.Enabled
	})
}

// isCodeBlockEnabled checks if code block enhancement is enabled
// Default: true (opt-out strategy)
func isCodeBlockEnabled(config *Config) bool {
	return isMarkdownFeatureEnabled(config, func() *bool {
		if config.Features.Markdown.CodeBlockConfig == nil {
			return nil
		}
		return config.Features.Markdown.CodeBlockConfig.Enabled
	})
}

// isQuoteEnabled checks if quote enhancement is enabled
// Default: true (opt-out strategy)
func isQuoteEnabled(config *Config) bool {
	return isMarkdownFeatureEnabled(config, func() *bool {
		if config.Features.Markdown.QuoteConfig == nil {
			return nil
		}
		return config.Features.Markdown.QuoteConfig.Enabled
	})
}

// getMaxCodeBlockLines returns the max code block lines limit (default: 100)
func getMaxCodeBlockLines(config *Config) int {
	if config == nil || config.Features.Markdown.CodeBlockConfig == nil {
		return 100
	}
	if config.Features.Markdown.CodeBlockConfig.MaxLines <= 0 {
		return 100
	}
	return config.Features.Markdown.CodeBlockConfig.MaxLines
}
