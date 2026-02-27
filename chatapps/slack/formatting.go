package slack

import (
	"fmt"
	"strings"
)

// =============================================================================
// Mrkdwn Formatting Utilities
// =============================================================================

// MrkdwnFormatter provides utilities for converting Markdown to Slack mrkdwn format
type MrkdwnFormatter struct{}

// NewMrkdwnFormatter creates a new MrkdwnFormatter
func NewMrkdwnFormatter() *MrkdwnFormatter {
	return &MrkdwnFormatter{}
}

// Format converts Markdown text to Slack mrkdwn format
// Handles: bold, italic, strikethrough, code blocks, links
// Order: links -> bold -> italic -> strikethrough -> escape (to preserve URLs and code)
func (f *MrkdwnFormatter) Format(text string) string {
	if text == "" {
		return ""
	}

	result := text

	// 1. Convert Links first (before escaping, to preserve URLs)
	result = f.convertLinks(result)

	// 2. Convert Bold: **text** or __text__ -> *text*
	result = f.convertBold(result)

	// 3. Convert Italic: *text* or _text_ -> _text_
	result = f.convertItalic(result)

	// 4. Convert Strikethrough: ~~text~~ -> ~text~
	result = f.convertStrikethrough(result)

	// 5. Escape special characters last (preserves code blocks)
	result = f.escapeSpecialChars(result)

	return result
}

// escapeSpecialChars escapes & < > for mrkdwn safely, preserving code blocks and Slack syntax
// Only skips valid Slack syntax: <!...>, <@...>, <#...>, <url|text>
func (f *MrkdwnFormatter) escapeSpecialChars(text string) string {
	var result strings.Builder
	inCodeBlock := false
	inInlineCode := false

	for i := 0; i < len(text); i++ {
		// Check for code block boundaries (```)
		if strings.HasPrefix(text[i:], "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString("```")
			i += 2 // Skip the next two backticks
			continue
		}

		// Check for inline code boundaries (`)
		if text[i] == '`' && (i == 0 || text[i-1] != '\\') {
			inInlineCode = !inInlineCode
		}

		// Skip Slack special syntax: <!here>, <@user>, <#channel>, <url|text>
		if text[i] == '<' && !inCodeBlock && !inInlineCode {
			// Find closing >
			endIdx := -1
			for j := i + 1; j < len(text); j++ {
				if text[j] == '>' {
					endIdx = j
					break
				}
			}
			if endIdx != -1 {
				inner := text[i+1 : endIdx]
				// Only skip if it's valid Slack syntax
				if len(inner) > 0 && (inner[0] == '!' || inner[0] == '@' || inner[0] == '#' ||
					strings.Contains(inner, "|")) {
					result.WriteString(text[i : endIdx+1])
					i = endIdx
					continue
				}
			}
		}

		// Only escape special characters outside of code
		if !inCodeBlock && !inInlineCode {
			switch text[i] {
			case '&':
				result.WriteString("&amp;")
			case '<':
				result.WriteString("&lt;")
			case '>':
				result.WriteString("&gt;")
			default:
				result.WriteByte(text[i])
			}
			continue
		}

		// Preserve characters inside code as-is
		result.WriteByte(text[i])
	}
	return result.String()
}

// convertBold converts **text** or __text__ to *text*
func (f *MrkdwnFormatter) convertBold(text string) string {
	var result strings.Builder
	inCodeBlock := false
	i := 0
	for i < len(text) {
		// Toggle code block state
		if strings.HasPrefix(text[i:], "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString("```")
			i += 3
			continue
		}
		if inCodeBlock {
			result.WriteByte(text[i])
			i++
			continue
		}

		// Handle ** or __
		if (strings.HasPrefix(text[i:], "**") || strings.HasPrefix(text[i:], "__")) && i+2 < len(text) {
			marker := text[i : i+2]
			endIdx := strings.Index(text[i+2:], marker)
			if endIdx != -1 {
				content := text[i+2 : i+2+endIdx]
				result.WriteByte('*')
				result.WriteString(content)
				result.WriteByte('*')
				i += 4 + endIdx
				continue
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// convertItalic converts _text_ to _text_
// Does NOT convert *text* (Slack uses * for bold, not italic)
func (f *MrkdwnFormatter) convertItalic(text string) string {
	var result strings.Builder
	inCodeBlock := false
	i := 0
	for i < len(text) {
		// Toggle code block state
		if strings.HasPrefix(text[i:], "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString("```")
			i += 3
			continue
		}
		if inCodeBlock {
			result.WriteByte(text[i])
			i++
			continue
		}

		// Handle _text_ (but not __text__ which is handled by convertBold)
		if text[i] == '_' && i+1 < len(text) && text[i+1] != '_' {
			endIdx := strings.Index(text[i+1:], "_")
			if endIdx != -1 {
				content := text[i+1 : i+1+endIdx]
				result.WriteByte('_')
				result.WriteString(content)
				result.WriteByte('_')
				i += 2 + endIdx
				continue
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// convertStrikethrough converts ~~text~~ to ~text~
func (f *MrkdwnFormatter) convertStrikethrough(text string) string {
	var result strings.Builder
	inCodeBlock := false
	i := 0
	for i < len(text) {
		// Toggle code block state
		if strings.HasPrefix(text[i:], "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString("```")
			i += 3
			continue
		}
		if inCodeBlock {
			result.WriteByte(text[i])
			i++
			continue
		}

		// Handle ~~
		if strings.HasPrefix(text[i:], "~~") {
			endIdx := strings.Index(text[i+2:], "~~")
			if endIdx != -1 {
				content := text[i+2 : i+2+endIdx]
				result.WriteByte('~')
				result.WriteString(content)
				result.WriteByte('~')
				i += 4 + endIdx
				continue
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// convertLinks converts [text](url) to <url|text>
func (f *MrkdwnFormatter) convertLinks(text string) string {
	var result strings.Builder
	i := 0
	for i < len(text) {
		// Check for [text](url) pattern
		if text[i] == '[' {
			closeBracket := strings.Index(text[i+1:], "]")
			if closeBracket != -1 {
				linkText := text[i+1 : i+1+closeBracket]
				openParen := i + 1 + closeBracket + 1
				if openParen < len(text) && text[openParen] == '(' {
					closeParen := strings.Index(text[openParen+1:], ")")
					if closeParen != -1 {
						url := text[openParen+1 : openParen+1+closeParen]
						result.WriteString(fmt.Sprintf("<%s|%s>", url, linkText))
						i = openParen + 1 + closeParen + 1
						continue
					}
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// FormatCodeBlock formats a code block with optional language
func (f *MrkdwnFormatter) FormatCodeBlock(code, language string) string {
	if language == "" {
		return fmt.Sprintf("```\n%s\n```", code)
	}
	return fmt.Sprintf("```%s\n%s\n```", language, code)
}

// =============================================================================
// Slack Special Syntax Formatters
// =============================================================================

// FormatChannelMention creates a channel mention: <#C123|channel-name>
func FormatChannelMention(channelID, channelName string) string {
	return fmt.Sprintf("<#%s|%s>", channelID, channelName)
}

// FormatChannelMentionByID creates a channel mention with just ID: <#C123>
func FormatChannelMentionByID(channelID string) string {
	return fmt.Sprintf("<#%s>", channelID)
}

// FormatUserMention creates a user mention: <@U123|username>
func FormatUserMention(userID, userName string) string {
	return fmt.Sprintf("<@%s|%s>", userID, userName)
}

// FormatUserMentionByID creates a user mention with just ID: <@U123>
func FormatUserMentionByID(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}

// FormatSpecialMention creates a special mention: <!here>, <!channel>, <!everyone>
func FormatSpecialMention(mentionType string) string {
	// mentionType: "here", "channel", "everyone"
	return fmt.Sprintf("<!%s>", mentionType)
}

// FormatHereMention creates a @here mention
func FormatHereMention() string {
	return "<!here>"
}

// FormatChannelMention creates a @channel mention
func FormatChannelAllMention() string {
	return "<!channel>"
}

// FormatEveryoneMention creates a @everyone mention
func FormatEveryoneMention() string {
	return "<!everyone>"
}

// FormatDateTime creates a date formatting: <!date^timestamp^format|fallback>
// Reference: https://api.slack.com/reference/surfaces/formatting#date-formatting
func FormatDateTime(timestamp int64, format, fallback string) string {
	return fmt.Sprintf("<!date^%d^%s|%s>", timestamp, format, fallback)
}

// FormatDateTimeWithLink creates a date formatting with link: <!date^timestamp^format^link|fallback>
func FormatDateTimeWithLink(timestamp int64, format, linkURL, fallback string) string {
	return fmt.Sprintf("<!date^%d^%s^%s|%s>", timestamp, format, linkURL, fallback)
}

// FormatDate creates a simple date formatting
func FormatDate(timestamp int64) string {
	return FormatDateTime(timestamp, "{date}", "Unknown date")
}

// FormatDateShort creates a short date formatting (e.g., "Jan 1, 2024")
func FormatDateShort(timestamp int64) string {
	return FormatDateTime(timestamp, "{date_short}", "Unknown date")
}

// FormatDateLong creates a long date formatting (e.g., "Monday, January 1, 2024")
func FormatDateLong(timestamp int64) string {
	return FormatDateTime(timestamp, "{date_long}", "Unknown date")
}

// FormatTime creates a time formatting (e.g., "2:30 PM")
func FormatTime(timestamp int64) string {
	return FormatDateTime(timestamp, "{time}", "Unknown time")
}

// FormatDateTimeCombined creates combined date and time formatting
func FormatDateTimeCombined(timestamp int64) string {
	return FormatDateTime(timestamp, "{date} at {time}", "Unknown datetime")
}

// FormatURL creates a link: <url|text> or <url>
func FormatURL(url, text string) string {
	if text == "" {
		return fmt.Sprintf("<%s>", url)
	}
	return fmt.Sprintf("<%s|%s>", url, text)
}

// FormatEmail creates an email link
func FormatEmail(email string) string {
	return fmt.Sprintf("<mailto:%s|%s>", email, email)
}

// FormatCommand creates a command formatting
func FormatCommand(command string) string {
	return fmt.Sprintf("</%s>", command)
}

// FormatSubteamMention creates a user group mention: <!subteam^S123|@group>
func FormatSubteamMention(subteamID, subteamHandle string) string {
	return fmt.Sprintf("<!subteam^%s|%s>", subteamID, subteamHandle)
}

// FormatObject creates an object mention (for boards, clips, etc.)
func FormatObject(objectType, objectID, objectText string) string {
	return fmt.Sprintf("<%s://%s|%s>", objectType, objectID, objectText)
}
