package slack

import (
	"fmt"
	"strings"

	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/provider"
)

// BlockBuilder builds Slack Block Kit messages for various event types
type BlockBuilder struct{}

// NewBlockBuilder creates a new BlockBuilder instance
func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{}
}

// =============================================================================
// Text Object Helpers
// =============================================================================

// mrkdwnText creates a mrkdwn text object
// Used for section, context blocks that support formatting
func mrkdwnText(text string) map[string]any {
	return map[string]any{
		"type": "mrkdwn",
		"text": text,
	}
}

// plainText creates a plain_text text object
// Used for header, button text that doesn't support formatting
func plainText(text string) map[string]any {
	return map[string]any{
		"type":  "plain_text",
		"text":  text,
		"emoji": true,
	}
}

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
func (f *MrkdwnFormatter) Format(text string) string {
	if text == "" {
		return ""
	}

	result := text

	// 1. Escape special characters first (except inside code blocks)
	result = f.escapeSpecialChars(result)

	// 2. Convert Bold: **text** or __text__ -> *text*
	result = f.convertBold(result)

	// 3. Convert Italic: *text* or _text_ -> _text_
	result = f.convertItalic(result)

	// 4. Convert Strikethrough: ~~text~~ -> ~text~
	result = f.convertStrikethrough(result)

	// 5. Convert Links: [text](url) -> <url|text>
	result = f.convertLinks(result)

	return result
}

// escapeSpecialChars escapes & < > for mrkdwn safely
func (f *MrkdwnFormatter) escapeSpecialChars(text string) string {
	// Simple replacement is safe if done before other conversions
	result := strings.ReplaceAll(text, "&", "&amp;")
	result = strings.ReplaceAll(result, "<", "&lt;")
	result = strings.ReplaceAll(result, ">", "&gt;")
	return result
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

// convertItalic converts *text* or _text_ to _text_
func (f *MrkdwnFormatter) convertItalic(text string) string {
	var result strings.Builder
	inCodeBlock := false
	i := 0
	for i < len(text) {
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

		// Handle * or _ (but avoid matching markers that are part of other words if possible)
		if (text[i] == '*' || text[i] == '_') && i+1 < len(text) {
			marker := string(text[i])
			// Find next marker, but ensure it's not immediately followed by the same marker (which would be bold)
			endIdx := -1
			for j := i + 1; j < len(text); j++ {
				if string(text[j]) == marker {
					// Check if it's double
					if j+1 < len(text) && string(text[j+1]) == marker {
						j++ // skip
						continue
					}
					endIdx = j
					break
				}
			}

			if endIdx != -1 {
				content := text[i+1 : endIdx]
				result.WriteByte('_')
				result.WriteString(content)
				result.WriteByte('_')
				i = endIdx + 1
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

		if strings.HasPrefix(text[i:], "~~") && i+2 < len(text) {
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
		if text[i] == '[' {
			closeBracket := strings.Index(text[i:], "]")
			if closeBracket != -1 && i+closeBracket+1 < len(text) && text[i+closeBracket+1] == '(' {
				closeParen := strings.Index(text[i+closeBracket+1:], ")")
				if closeParen != -1 {
					linkText := text[i+1 : i+closeBracket]
					linkURL := text[i+closeBracket+2 : i+closeBracket+1+closeParen]
					result.WriteByte('<')
					result.WriteString(linkURL)
					result.WriteByte('|')
					result.WriteString(linkText)
					result.WriteByte('>')
					i += closeBracket + closeParen + 2
					continue
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// FormatCodeBlock formats code with optional language hint
func (f *MrkdwnFormatter) FormatCodeBlock(code, language string) string {
	if language == "" {
		return fmt.Sprintf("```\n%s\n```", code)
	}
	return fmt.Sprintf("```%s\n%s\n```", language, code)
}

// =============================================================================
// Block Builders - Event Type Mappings
// =============================================================================

// BuildThinkingBlock builds a context block for thinking status
// Used for: provider.EventTypeThinking
// Strategy: Send immediately (not aggregated) for instant feedback
func (b *BlockBuilder) BuildThinkingBlock(content string) []map[string]any {
	// Use actual thinking content if available, fallback to default
	displayText := content
	if displayText == "" {
		displayText = "Thinking..."
	}

	return []map[string]any{
		{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(fmt.Sprintf(":brain: _%s_", displayText)),
			},
		},
	}
}

// BuildToolUseBlock builds a section block for tool invocation
// Used for: provider.EventTypeToolUse
// Strategy: Can be aggregated with similar tool events
func (b *BlockBuilder) BuildToolUseBlock(toolName, input string, truncated bool) []map[string]any {
	// Format input as code block
	formattedInput := fmt.Sprintf("```%s```", input)

	// Add truncation indicator if needed
	if truncated {
		formattedInput += "\n*_Output truncated..._*"
	}

	return []map[string]any{
		{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf(":hammer_and_wrench: *Using tool:* `%s`", toolName)),
			"fields": []map[string]any{
				mrkdwnText("*Input:*\n" + formattedInput),
			},
		},
	}
}

// BuildToolResultBlock builds a section block for tool execution result
// Used for: provider.EventTypeToolResult
// Strategy: Can be aggregated, includes optional button to expand output
func (b *BlockBuilder) BuildToolResultBlock(success bool, durationMs int64, output string, hasButton bool) []map[string]any {
	var blocks []map[string]any

	// Build status text
	statusEmoji := ":white_check_mark:"
	statusText := "*Completed*"
	if !success {
		statusEmoji = ":x:"
		statusText = "*Failed*"
	}

	resultBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText(fmt.Sprintf("%s %s", statusEmoji, statusText)),
	}

	// Add output preview if available (truncated to 300 chars for better context)
	if output != "" {
		previewLen := 300
		preview := output
		if len(output) > previewLen {
			preview = output[:previewLen] + "..."
		}
		resultBlock["fields"] = []map[string]any{
			mrkdwnText("*Output:*\n```\n" + preview + "\n```"),
		}
	}

	blocks = append(blocks, resultBlock)

	// Add metadata context block (Duration)
	if durationMs > 0 {
		blocks = append(blocks, map[string]any{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(fmt.Sprintf(":timer_clock: *Duration:* %s", formatDuration(durationMs))),
			},
		})
	}

	// Add action button if requested
	if hasButton && success {
		actionBlock := map[string]any{
			"type": "actions",
			"elements": []map[string]any{
				{
					"type":      "button",
					"text":      plainText("View Full Output"),
					"action_id": "view_tool_output",
					"value":     "expand_output",
				},
			},
		}
		blocks = append(blocks, actionBlock)
	}

	return blocks
}

// BuildErrorBlock builds blocks for error messages
// Used for: provider.EventTypeError, danger_block
// Strategy: Send immediately (not aggregated) for critical feedback
func (b *BlockBuilder) BuildErrorBlock(message string, isDangerBlock bool) []map[string]any {
	var blocks []map[string]any

	// Header block with emoji (Slack doesn't support style: danger for headers)
	headerEmoji := ":warning:"
	if isDangerBlock {
		headerEmoji = ":x:"
	}

	headerBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText(fmt.Sprintf("*%s Execution Error*", headerEmoji)),
	}
	blocks = append(blocks, headerBlock)

	// Error message as section with mrkdwn
	errorBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText(fmt.Sprintf("> %s", message)),
	}
	blocks = append(blocks, errorBlock)

	return blocks
}

// BuildAnswerBlock builds a section block for AI answer text
// Used for: provider.EventTypeAnswer
// Strategy: Stream updates via chat.update, supports mrkdwn formatting
func (b *BlockBuilder) BuildAnswerBlock(content string) []map[string]any {
	// Format content with mrkdwn
	formatter := NewMrkdwnFormatter()
	formattedContent := formatter.Format(content)

	return []map[string]any{
		{
			"type": "section",
			"text": mrkdwnText(formattedContent),
			// Enable expand for AI Assistant apps
			"expand": true,
		},
	}
}

// BuildStatsBlock builds a section block with statistics
// Used for: provider.EventTypeResult (end of turn)
// Strategy: Send as final summary
func (b *BlockBuilder) BuildStatsBlock(stats *event.EventMeta) []map[string]any {
	if stats == nil {
		return []map[string]any{}
	}

	var fields []map[string]any

	// Duration field
	if stats.TotalDurationMs > 0 {
		fields = append(fields, mrkdwnText(fmt.Sprintf("*Duration:*\n%s", formatDuration(stats.TotalDurationMs))))
	}

	// Token usage field
	if stats.InputTokens > 0 || stats.OutputTokens > 0 {
		tokenStr := fmt.Sprintf("%d in / %d out", stats.InputTokens, stats.OutputTokens)
		if stats.CacheReadTokens > 0 {
			tokenStr += fmt.Sprintf(" (cache: %d)", stats.CacheReadTokens)
		}
		fields = append(fields, mrkdwnText(fmt.Sprintf("*Tokens:*\n%s", tokenStr)))
	}

	// Cost field (if available)
	// Note: Cost tracking depends on provider implementation

	if len(fields) == 0 {
		return []map[string]any{}
	}

	return []map[string]any{
		{
			"type":   "section",
			"fields": fields,
		},
	}
}

// BuildDividerBlock creates a simple divider
func (b *BlockBuilder) BuildDividerBlock() []map[string]any {
	return []map[string]any{
		{
			"type": "divider",
		},
	}
}

// =============================================================================
// Session Statistics Blocks - Enhanced UI
// =============================================================================

// SessionStatsStyle defines the visual style for session statistics
type SessionStatsStyle string

const (
	// StatsStyleCompact - Minimal single-line summary
	StatsStyleCompact SessionStatsStyle = "compact"
	// StatsStyleCard - Rich card with emoji indicators (recommended)
	StatsStyleCard SessionStatsStyle = "card"
	// StatsStyleDetailed - Full breakdown with all metrics
	StatsStyleDetailed SessionStatsStyle = "detailed"
)

// BuildSessionStatsBlock builds a rich statistics summary block
// Used for: session_stats events at end of each turn
// Strategy: Send as final summary with visual polish
func (b *BlockBuilder) BuildSessionStatsBlock(stats *event.SessionStatsData, style SessionStatsStyle) []map[string]any {
	if stats == nil {
		return []map[string]any{}
	}

	switch style {
	case StatsStyleCompact:
		return b.buildCompactStats(stats)
	case StatsStyleDetailed:
		return b.buildDetailedStats(stats)
	case StatsStyleCard:
		fallthrough
	default:
		return b.buildCardStats(stats)
	}
}

// buildCompactStats creates a minimal single-line summary
func (b *BlockBuilder) buildCompactStats(stats *event.SessionStatsData) []map[string]any {
	if stats.TotalTokens == 0 && stats.TotalDurationMs == 0 {
		return []map[string]any{}
	}

	var parts []string

	// Duration
	if stats.TotalDurationMs > 0 {
		parts = append(parts, fmt.Sprintf("⏱️ %s", formatDuration(stats.TotalDurationMs)))
	}

	// Tokens
	if stats.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("📊 %d tokens", stats.TotalTokens))
	}

	// Cost (only if > 0)
	if stats.TotalCostUSD > 0.0001 {
		parts = append(parts, fmt.Sprintf("💰 $%.4f", stats.TotalCostUSD))
	}

	// Tools (only if used)
	if len(stats.ToolsUsed) > 0 {
		parts = append(parts, fmt.Sprintf("🔧 %d tools", len(stats.ToolsUsed)))
	}

	// Files (only if modified)
	if stats.FilesModified > 0 {
		parts = append(parts, fmt.Sprintf("📁 %d files", stats.FilesModified))
	}

	if len(parts) == 0 {
		return []map[string]any{}
	}

	return []map[string]any{
		{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(strings.Join(parts, " • ")),
			},
		},
	}
}

// buildCardStats creates a visually appealing card-style summary (recommended)
func (b *BlockBuilder) buildCardStats(stats *event.SessionStatsData) []map[string]any {
	if stats.TotalTokens == 0 && stats.TotalDurationMs == 0 {
		return []map[string]any{}
	}

	var blocks []map[string]any

	// Header with session complete indicator
	headerBlock := map[string]any{
		"type": "header",
		"text": plainText("✅ Session Complete"),
	}
	blocks = append(blocks, headerBlock)

	// Build metrics grid (2 columns for better space usage)
	var fields []map[string]any

	// Row 1: Duration + Tokens
	fields = append(fields, mrkdwnText(fmt.Sprintf("*⏱️ Duration*\n%s", formatDuration(stats.TotalDurationMs))))
	fields = append(fields, mrkdwnText(fmt.Sprintf("*📊 Tokens*\n%d in / %d out", stats.InputTokens, stats.OutputTokens)))

	// Row 2: Cost + Model (if available)
	if stats.TotalCostUSD > 0.0001 {
		fields = append(fields, mrkdwnText(fmt.Sprintf("*💰 Cost*\n$%.4f", stats.TotalCostUSD)))
	} else {
		fields = append(fields, mrkdwnText("*💰 Cost*\n_Usage-based_"))
	}

	if stats.ModelUsed != "" {
		modelShort := stats.ModelUsed
		if len(modelShort) > 20 {
			modelShort = modelShort[:17] + "..."
		}
		fields = append(fields, mrkdwnText(fmt.Sprintf("*🤖 Model*\n%s", modelShort)))
	}

	// Row 3: Tools (if used)
	if len(stats.ToolsUsed) > 0 {
		toolsStr := strings.Join(stats.ToolsUsed, ", ")
		if len(toolsStr) > 40 {
			toolsStr = toolsStr[:37] + "..."
		}
		fields = append(fields, mrkdwnText(fmt.Sprintf("*🔧 Tools Used*\n%s", toolsStr)))
	}

	// Row 4: Files (if modified)
	if stats.FilesModified > 0 {
		fields = append(fields, mrkdwnText(fmt.Sprintf("*📁 Files Modified*\n%d file(s)", stats.FilesModified)))
	}

	// Ensure even number of fields for proper 2-column layout
	if len(fields)%2 != 0 {
		fields = append(fields, mrkdwnText("*_*\n_")) // Empty placeholder
	}

	if len(fields) > 0 {
		fieldsBlock := map[string]any{
			"type":   "section",
			"fields": fields,
		}
		blocks = append(blocks, fieldsBlock)
	}

	// Add cache info if present
	if stats.CacheReadTokens > 0 || stats.CacheWriteTokens > 0 {
		cacheText := "📦 *Cache: "
		if stats.CacheReadTokens > 0 {
			cacheText += fmt.Sprintf("read %d", stats.CacheReadTokens)
		}
		if stats.CacheWriteTokens > 0 {
			if stats.CacheReadTokens > 0 {
				cacheText += ", "
			}
			cacheText += fmt.Sprintf("write %d", stats.CacheWriteTokens)
		}
		cacheText += "*"

		cacheBlock := map[string]any{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(cacheText),
			},
		}
		blocks = append(blocks, cacheBlock)
	}

	// Add file paths if available (up to 3)
	if len(stats.FilePaths) > 0 {
		var filesText string
		maxFiles := 3
		if len(stats.FilePaths) > maxFiles {
			filesText = fmt.Sprintf("📄 *%d files modified:* ", len(stats.FilePaths))
			for i := 0; i < maxFiles; i++ {
				if i > 0 {
					filesText += ", "
				}
				// Extract just filename from path
				parts := strings.Split(stats.FilePaths[i], "/")
				filesText += "`" + parts[len(parts)-1] + "`"
			}
			filesText += " _and more_"
		} else {
			filesText = "📄 *Files modified:* "
			for i, p := range stats.FilePaths {
				if i > 0 {
					filesText += ", "
				}
				parts := strings.Split(p, "/")
				filesText += "`" + parts[len(parts)-1] + "`"
			}
		}

		filesBlock := map[string]any{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(filesText),
			},
		}
		blocks = append(blocks, filesBlock)
	}

	return blocks
}

// buildDetailedStats creates a comprehensive breakdown with all metrics
func (b *BlockBuilder) buildDetailedStats(stats *event.SessionStatsData) []map[string]any {
	if stats.TotalTokens == 0 && stats.TotalDurationMs == 0 {
		return []map[string]any{}
	}

	var blocks []map[string]any

	// Header
	headerBlock := map[string]any{
		"type": "header",
		"text": plainText("📊 Session Statistics Report"),
	}
	blocks = append(blocks, headerBlock)

	// Performance Section
	perfBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText("*⚡ Performance*"),
	}
	blocks = append(blocks, perfBlock)

	var perfFields []map[string]any
	perfFields = append(perfFields, mrkdwnText(fmt.Sprintf("*Total Duration*\n%s", formatDuration(stats.TotalDurationMs))))

	if stats.ThinkingDurationMs > 0 {
		perfFields = append(perfFields, mrkdwnText(fmt.Sprintf("*Thinking*\n%s", formatDuration(stats.ThinkingDurationMs))))
	}
	if stats.ToolDurationMs > 0 {
		perfFields = append(perfFields, mrkdwnText(fmt.Sprintf("*Tool Execution*\n%s", formatDuration(stats.ToolDurationMs))))
	}
	if stats.GenerationDurationMs > 0 {
		perfFields = append(perfFields, mrkdwnText(fmt.Sprintf("*Generation*\n%s", formatDuration(stats.GenerationDurationMs))))
	}

	if len(perfFields)%2 != 0 {
		perfFields = append(perfFields, mrkdwnText("*_*\n_"))
	}
	perfBlock["fields"] = perfFields

	// Token Usage Section
	tokenBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText("*📈 Token Usage*"),
	}
	blocks = append(blocks, tokenBlock)

	var tokenFields []map[string]any
	tokenFields = append(tokenFields, mrkdwnText(fmt.Sprintf("*Input*\n%d tokens", stats.InputTokens)))
	tokenFields = append(tokenFields, mrkdwnText(fmt.Sprintf("*Output*\n%d tokens", stats.OutputTokens)))
	tokenFields = append(tokenFields, mrkdwnText(fmt.Sprintf("*Total*\n%d tokens", stats.TotalTokens)))

	if stats.CacheReadTokens > 0 {
		tokenFields = append(tokenFields, mrkdwnText(fmt.Sprintf("*Cache Read*\n%d tokens", stats.CacheReadTokens)))
	}
	if stats.CacheWriteTokens > 0 {
		tokenFields = append(tokenFields, mrkdwnText(fmt.Sprintf("*Cache Write*\n%d tokens", stats.CacheWriteTokens)))
	}

	if len(tokenFields)%2 != 0 {
		tokenFields = append(tokenFields, mrkdwnText("*_*\n_"))
	}
	tokenBlock["fields"] = tokenFields

	// Cost Section
	if stats.TotalCostUSD > 0 {
		costBlock := map[string]any{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("*💰 Total Cost*: `$%.4f USD`", stats.TotalCostUSD)),
		}
		blocks = append(blocks, costBlock)
	}

	// Tools Section
	if len(stats.ToolsUsed) > 0 {
		toolsBlock := map[string]any{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("*🔧 Tools Invoked* (%d total)*\n`%s`",
				stats.ToolCallCount, strings.Join(stats.ToolsUsed, "`, `"))),
		}
		blocks = append(blocks, toolsBlock)
	}

	// Files Section
	if len(stats.FilePaths) > 0 {
		filesBlock := map[string]any{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("*📁 Files Modified* (%d total)*\n`%s`",
				len(stats.FilePaths), strings.Join(stats.FilePaths, "`, `"))),
		}
		blocks = append(blocks, filesBlock)
	}

	return blocks
}

// =============================================================================
// Helper Functions
// =============================================================================

// formatDuration converts milliseconds to human-readable duration
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	return fmt.Sprintf("%.1fs", seconds)
}

// TruncateText truncates text to max length with ellipsis
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// =============================================================================
// Permission Request Blocks (Issue #39)
// =============================================================================

// BuildPermissionRequestBlocks builds Slack Block Kit for a Claude Code permission request.
// It displays the tool name, command preview, and approval/denial buttons.
func BuildPermissionRequestBlocks(req *provider.PermissionRequest, sessionID string) []map[string]any {
	tool, input := req.GetToolAndInput()

	// Truncate long commands for preview
	displayInput := input
	if len(displayInput) > 500 {
		displayInput = displayInput[:497] + "..."
	}

	blocks := []map[string]any{}

	// Header
	blocks = append(blocks, map[string]any{
		"type": "header",
		"text": plainText("⚠️ Permission Request"),
	})

	// Tool information
	if tool != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("*Tool:* `%s`", tool)),
		})
	}

	// Command/Action preview
	if displayInput != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("*Command:*\n```\n%s\n```", displayInput)),
		})
	}

	// Decision reason (if available)
	if req.Decision != nil && req.Decision.Reason != "" {
		blocks = append(blocks, map[string]any{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(fmt.Sprintf("*Reason:* %s", req.Decision.Reason)),
			},
		})
	}

	// Session info
	blocks = append(blocks, map[string]any{
		"type": "context",
		"elements": []map[string]any{
			mrkdwnText(fmt.Sprintf("Session: `%s`", sessionID)),
		},
	})

	// Action buttons
	blocks = append(blocks, map[string]any{
		"type":     "actions",
		"block_id": fmt.Sprintf("perm_%s", req.MessageID),
		"elements": []map[string]any{
			{
				"type":      "button",
				"text":      plainText("✅ Allow"),
				"action_id": "perm_allow",
				"style":     "primary",
				"value":     fmt.Sprintf("allow:%s:%s", sessionID, req.MessageID),
			},
			{
				"type":      "button",
				"text":      plainText("🚫 Deny"),
				"action_id": "perm_deny",
				"style":     "danger",
				"value":     fmt.Sprintf("deny:%s:%s", sessionID, req.MessageID),
			},
		},
	})

	return blocks
}

// BuildPermissionApprovedBlocks builds blocks to show after permission is approved.
func BuildPermissionApprovedBlocks(tool, input string) []map[string]any {
	// Truncate for display
	displayInput := input
	if len(displayInput) > 200 {
		displayInput = displayInput[:197] + "..."
	}

	return []map[string]any{
		{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("✅ *Permission Granted*\n\nTool: `%s`\nCommand: `%s`", tool, displayInput)),
		},
	}
}

// BuildPermissionDeniedBlocks builds blocks to show after permission is denied.
func BuildPermissionDeniedBlocks(tool, input, reason string) []map[string]any {
	// Truncate for display
	displayInput := input
	if len(displayInput) > 200 {
		displayInput = displayInput[:197] + "..."
	}

	blocks := []map[string]any{
		{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf("🚫 *Permission Denied*\n\nTool: `%s`\nCommand: `%s`", tool, displayInput)),
		},
	}

	if reason != "" {
		blocks = append(blocks, map[string]any{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(fmt.Sprintf("Reason: %s", reason)),
			},
		})
	}

	return blocks
}
