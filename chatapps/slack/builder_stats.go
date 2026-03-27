// Package slack provides the Slack adapter implementation for the hotplex engine.
// Stats message builders for Slack Block Kit.
package slack

import (
	"fmt"
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
)

// StatsMessageBuilder builds stats-related Slack messages (SessionStats, CommandProgress, CommandComplete)
type StatsMessageBuilder struct {
	config *Config
}

// NewStatsMessageBuilder creates a new StatsMessageBuilder with configuration
func NewStatsMessageBuilder(config *Config) *StatsMessageBuilder {
	return &StatsMessageBuilder{config: config}
}

// BuildSessionStatsMessage builds a message for session statistics
// Implements EventTypeResult (Turn Complete) per spec - compact single-line format
// With table feature enabled: uses Slack TableBlock for better UX
// Display format: ⏱️ duration • 🧠 context% • ⚡ tokens in/out • 📝 files • 🔧 tools
func (b *StatsMessageBuilder) BuildSessionStatsMessage(msg *base.ChatMessage) []slack.Block {
	// Check if table format is enabled
	if b.isTableEnabled() {
		return b.BuildSessionStatsTable(msg)
	}

	// Fallback to compact single-line format
	var blocks []slack.Block

	// Build compact stats line: ⏱️ duration • 🧠 context% • ⚡ tokens in/out • 📝 files • 🔧 tools
	if msg.Metadata != nil {
		var stats []string

		// Total Duration
		if duration := extractInt64(msg.Metadata, "total_duration_ms"); duration > 0 {
			stats = append(stats, "⏱️ "+FormatDuration(duration))
		}

		// Context Window Usage Percentage
		// Shows how much of the 200K context window is used
		if ctxPercent := extractFloat64(msg.Metadata, "context_used_percent"); ctxPercent > 0 {
			stats = append(stats, fmt.Sprintf("🧠 %.0f%%", ctxPercent))
		}

		// Tokens (simplified display - just input/output, no cache)
		tokensIn := extractInt64(msg.Metadata, "input_tokens")
		tokensOut := extractInt64(msg.Metadata, "output_tokens")
		if tokensIn > 0 || tokensOut > 0 {
			stats = append(stats, fmt.Sprintf("⚡ %s/%s",
				formatTokenCount(tokensIn), formatTokenCount(tokensOut)))
		}

		// Files modified
		if files := extractInt64(msg.Metadata, "files_modified"); files > 0 {
			stats = append(stats, fmt.Sprintf("📝 %d files", files))
		}

		// Tool calls
		if tools := extractInt64(msg.Metadata, "tool_call_count"); tools > 0 {
			stats = append(stats, fmt.Sprintf("🔧 %d tools", tools))
		}

		if len(stats) > 0 {
			statsText := slack.NewTextBlockObject("mrkdwn", strings.Join(stats, " • "), false, false)
			blocks = append(blocks, slack.NewContextBlock("", statsText))
		}
	}

	return blocks
}

// extractInt64 extracts int64 value from metadata, supporting both int32 and int64 types
func extractInt64(metadata map[string]any, key string) int64 {
	if v, ok := metadata[key].(int64); ok {
		return v
	}
	if v, ok := metadata[key].(int32); ok {
		return int64(v)
	}
	return 0
}

// extractFloat64 extracts float64 value from metadata
func extractFloat64(metadata map[string]any, key string) float64 {
	if v, ok := metadata[key].(float64); ok {
		return v
	}
	return 0
}

// formatTokenCount formats token count in compact form (1.2K, 1.00M)
// Uses proper threshold: K for < 1M, M for >= 1M
// DEPRECATED: Use base.FormatTokenCount instead
// This is kept for backward compatibility during migration
func formatTokenCount(count int64) string {
	return base.FormatTokenCount(count)
}

// BuildCommandProgressMessage builds a message for command progress updates
// Implements EventTypeCommandProgress per spec (17)
// Block type: section + context + actions
func (b *StatsMessageBuilder) BuildCommandProgressMessage(msg *base.ChatMessage) []slack.Block {
	title := msg.Content
	if title == "" {
		title = "Executing command..."
	}

	// Get command name from metadata
	commandName := ""
	if msg.Metadata != nil {
		if cmd, ok := msg.Metadata["command"].(string); ok {
			commandName = cmd
		}
	}

	headerText := "⚙️ " + commandName
	if commandName == "" {
		headerText = "⚙️ Executing"
	}

	mrkdwn := slack.NewTextBlockObject("mrkdwn", headerText+"\n"+title, false, false)

	var blocks []slack.Block
	blocks = append(blocks, slack.NewSectionBlock(mrkdwn, nil, nil))

	// Add progress steps from metadata if available
	if msg.Metadata != nil {
		if steps, ok := msg.Metadata["steps"].([]string); ok && len(steps) > 0 {
			var stepTexts []string
			for i, step := range steps {
				stepTexts = append(stepTexts, fmt.Sprintf("○ Step %d: %s", i+1, step))
			}
			stepsText := strings.Join(stepTexts, "\n")
			stepsObj := slack.NewTextBlockObject("mrkdwn", stepsText, false, false)
			blocks = append(blocks, slack.NewSectionBlock(stepsObj, nil, nil))

			// Per spec: context block with progress indicator
			progressText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Progress: %d steps", len(steps)), false, false)
			blocks = append(blocks, slack.NewContextBlock("", progressText))
		}
	}

	// Per spec: do not add cancel button for command progress messages
	// Command execution cannot be cancelled by user
	return blocks
}

// BuildCommandCompleteMessage builds a single-line compact Context Block for command completion
// Format: ⚡ {cmd} 执行完成 ({completed}/{total} | 耗时: {dur})
func (b *StatsMessageBuilder) BuildCommandCompleteMessage(msg *base.ChatMessage) []slack.Block {
	title := msg.Content
	if title == "" {
		title = "Command completed"
	}

	commandName := ""
	var durationMs int64
	var completedSteps, totalSteps int
	if msg.Metadata != nil {
		if cmd, ok := msg.Metadata["command"].(string); ok {
			commandName = cmd
		}
		if dur, ok := msg.Metadata["duration_ms"].(int64); ok {
			durationMs = dur
		}
		if completed, ok := msg.Metadata["completed_steps"].(int); ok {
			completedSteps = completed
		}
		if total, ok := msg.Metadata["total_steps"].(int); ok {
			totalSteps = total
		}
	}

	line := "⚡ "
	if commandName != "" {
		line += "`" + commandName + "` "
	}
	line += title

	var extras []string
	if totalSteps > 0 {
		extras = append(extras, fmt.Sprintf("%d/%d steps", completedSteps, totalSteps))
	}
	if durationMs > 0 {
		extras = append(extras, "⏱️ "+FormatDuration(durationMs))
	}
	if len(extras) > 0 {
		line += "  |  " + strings.Join(extras, "  |  ")
	}

	text := slack.NewTextBlockObject("mrkdwn", line, false, false)
	return []slack.Block{slack.NewContextBlock("", text)}
}

// isTableEnabled checks if table format is enabled in configuration
// Default: true (opt-out strategy - enabled unless explicitly disabled)
func (b *StatsMessageBuilder) isTableEnabled() bool {
	if b.config == nil {
		return true // Default enabled
	}
	if b.config.Features.Markdown.TableConfig == nil {
		return true // Default enabled
	}
	return BoolValue(b.config.Features.Markdown.TableConfig.Enabled, true)
}

// BuildSessionStatsTable builds a table format message for session statistics
// Uses TableBuilder for Slack TableBlock rendering
func (b *StatsMessageBuilder) BuildSessionStatsTable(msg *base.ChatMessage) []slack.Block {
	tableBuilder := NewTableBuilder(TableConfig{
		MaxRows:    getMaxTableRows(b.config),
		Compact:    false,
		ShowHeader: false,
	})
	table := tableBuilder.BuildStatsTable(msg)
	if table == nil {
		return nil
	}
	return []slack.Block{table}
}

// BuildCommandProgressTable builds a table format message for command progress
func (b *StatsMessageBuilder) BuildCommandProgressTable(msg *base.ChatMessage) []slack.Block {
	tableBuilder := NewTableBuilder(TableConfig{
		MaxRows:    getMaxTableRows(b.config),
		Compact:    false,
		ShowHeader: false,
	})
	table := tableBuilder.BuildCommandProgressTable(msg)
	if table == nil {
		return nil
	}
	return []slack.Block{table}
}

// BuildToolCallsTable builds a table format message for tool calls
func (b *StatsMessageBuilder) BuildToolCallsTable(msg *base.ChatMessage) []slack.Block {
	tableBuilder := NewTableBuilder(TableConfig{
		MaxRows:    getMaxTableRows(b.config),
		Compact:    false,
		ShowHeader: false,
	})
	table := tableBuilder.BuildToolCallsTable(msg)
	if table == nil {
		return nil
	}
	return []slack.Block{table}
}

// getMaxTableRows returns the max table rows limit from config (default: 20)
func getMaxTableRows(config *Config) int {
	if config == nil || config.Features.Markdown.TableConfig == nil {
		return 20
	}
	if config.Features.Markdown.TableConfig.MaxRows <= 0 {
		return 20
	}
	return config.Features.Markdown.TableConfig.MaxRows
}
