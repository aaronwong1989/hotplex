package slack

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SendMessageOptions contains options for sending messages
type SendMessageOptions struct {
	ChannelID string
	ThreadTS  string
	Markdown  bool // Whether to convert markdown to mrkdwn
}

// SendMessageWithOptions sends a message with full options support
// This includes: message chunking, thread support, rate limiting
func (a *Adapter) SendMessageWithOptions(ctx context.Context, sessionID string, msg *MessageContent, opts SendMessageOptions) error {
	if a.config.BotToken == "" {
		return fmt.Errorf("slack bot token not configured")
	}

	if opts.ChannelID == "" {
		return fmt.Errorf("channel_id is required")
	}

	// Process text content
	text := msg.Content
	if opts.Markdown {
		text = convertMarkdownToMrkdwn(text)
	}

	// Handle message chunking if needed
	if utf8.RuneCountInString(text) > SlackTextLimit {
		return a.sendWithChunking(ctx, opts.ChannelID, text, opts.ThreadTS, opts.Markdown)
	}

	// Send single message
	return a.SendToChannel(ctx, opts.ChannelID, text, opts.ThreadTS)
}

// sendWithChunking sends a long message in chunks
func (a *Adapter) sendWithChunking(ctx context.Context, channelID, text, threadTS string, isMarkdown bool) error {
	var chunks []string
	if isMarkdown {
		chunks = ChunkMessageMarkdown(text, SlackTextLimit)
	} else {
		chunks = chunkMessage(text, SlackTextLimit)
	}

	for i, chunk := range chunks {
		// For the first chunk, don't use thread_ts unless it's a reply
		var chunkThreadTS string
		if i > 0 || threadTS != "" {
			chunkThreadTS = threadTS
			// For first chunk in a thread reply, also add thread_ts
			if i == 0 && threadTS != "" {
				chunkThreadTS = threadTS
			}
		}

		if err := a.SendToChannel(ctx, channelID, chunk, chunkThreadTS); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d: %w", i+1, len(chunks), err)
		}
	}

	return nil
}

// MessageContent represents the content of a message to send
type MessageContent struct {
	Content string
}

// convertMarkdownToMrkdwn converts Markdown text to Slack's mrkdwn format
func convertMarkdownToMrkdwn(text string) string {
	// Escape special characters first
	result := escapeSlackChars(text)

	// Convert bold: **text** -> *text*
	result = convertBold(result)

	// Convert italic: *text* -> _text_ (using manual parsing, not regex)
	result = convertItalicManual(result)

	// Convert code blocks: ```code``` -> ```code```
	result = convertCodeBlocks(result)

	// Convert links: [text](url) -> <url|text>
	result = convertLinks(result)

	return result
}

// escapeSlackChars escapes special characters for Slack
func escapeSlackChars(text string) string {
	result := strings.Builder{}
	result.Grow(len(text))

	for _, r := range text {
		switch r {
		case '&':
			result.WriteString("&amp;")
		case '<':
			result.WriteString("&lt;")
		case '>':
			result.WriteString("&gt;")
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// convertBold converts **text** to *text* using manual parsing
func convertBold(text string) string {
	result := strings.Builder{}
	result.Grow(len(text))

	i := 0
	for i < len(text) {
		// Check for ** marker
		if i+1 < len(text) && text[i] == '*' && text[i+1] == '*' {
			// Find closing **
			end := strings.Index(text[i+2:], "**")
			if end != -1 {
				// Write opening *
				result.WriteByte('*')
				// Write content
				result.WriteString(text[i+2 : i+2+end])
				// Write closing *
				result.WriteByte('*')
				i += 4 + end
				continue
			}
		}
		result.WriteByte(text[i])
		i++
	}

	return result.String()
}

// convertItalicManual converts *text* to _text_ using manual parsing (not regex)
// This avoids Go's lack of lookbehind support
func convertItalicManual(text string) string {
	result := strings.Builder{}
	result.Grow(len(text))

	i := 0
	inBold := false // Track if we're inside bold markers

	for i < len(text) {
		if text[i] == '*' {
			// Check for ** (bold markers)
			if i+1 < len(text) && text[i+1] == '*' {
				inBold = !inBold
				result.WriteString("**")
				i += 2
				continue
			}

			// Single * - check if it's a standalone italic marker
			// Skip if it's part of bold or at word boundary
			if !inBold {
				// Check if previous char is not whitespace or start of string
				prevIsWord := i > 0 && text[i-1] != ' ' && text[i-1] != '\n' && text[i-1] != '\t'
				// Check if next char is not whitespace or end of string
				nextIsWord := i+1 < len(text) && text[i+1] != ' ' && text[i+1] != '\n' && text[i+1] != '\t' && text[i+1] != '*'

				if prevIsWord && nextIsWord {
					// This is an italic marker, convert to _
					result.WriteByte('_')
					i++
					continue
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}

	return result.String()
}

// convertCodeBlocks converts ```code``` to ```code```
func convertCodeBlocks(text string) string {
	// Triple backticks are preserved in mrkdwn
	// Just ensure proper formatting
	return text
}

// convertLinks converts [text](url) to <url|text> using manual parsing
func convertLinks(text string) string {
	result := strings.Builder{}
	result.Grow(len(text))

	i := 0
	for i < len(text) {
		if text[i] == '[' {
			// Find closing ]
			textEnd := strings.Index(text[i:], "]")
			if textEnd != -1 && i+textEnd+1 < len(text) && text[i+textEnd+1] == '(' {
				// Find closing )
				urlStart := i + textEnd + 2
				urlEnd := strings.Index(text[urlStart:], ")")
				if urlEnd != -1 {
					linkText := text[i+1 : i+textEnd]
					linkURL := text[urlStart : urlStart+urlEnd]
					// Write <url|text>
					result.WriteByte('<')
					result.WriteString(linkURL)
					result.WriteByte('|')
					result.WriteString(linkText)
					result.WriteByte('>')
					i = urlStart + urlEnd + 1
					continue
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}

	return result.String()
}
