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
		if duration := base.ExtractInt64(msg.Metadata, "total_duration_ms"); duration > 0 {
			stats = append(stats, "⏱️ "+base.FormatDuration(duration))
		}

		// Context Window Usage Percentage
		// Shows how much of the 200K context window is used
		if ctxPercent := base.ExtractFloat64(msg.Metadata, "context_used_percent"); ctxPercent > 0 {
			stats = append(stats, fmt.Sprintf("🧠 %.0f%%", ctxPercent))
		}

		// Tokens (with cache information)
		tokensIn := base.ExtractInt64(msg.Metadata, "input_tokens")
		tokensOut := base.ExtractInt64(msg.Metadata, "output_tokens")
		cacheRead := base.ExtractInt64(msg.Metadata, "cache_read_tokens")
		cacheWrite := base.ExtractInt64(msg.Metadata, "cache_write_tokens")
		if tokensIn > 0 || tokensOut > 0 {
			tokenStr := fmt.Sprintf("⚡ %s/%s",
				base.FormatTokenCount(tokensIn), base.FormatTokenCount(tokensOut))
			// Add cache info if available
			if cacheRead > 0 || cacheWrite > 0 {
				cacheParts := []string{}
				if cacheRead > 0 {
					cacheParts = append(cacheParts, fmt.Sprintf("r:%s", base.FormatTokenCount(cacheRead)))
				}
				if cacheWrite > 0 {
					cacheParts = append(cacheParts, fmt.Sprintf("w:%s", base.FormatTokenCount(cacheWrite)))
				}
				tokenStr += fmt.Sprintf(" (cache: %s)", strings.Join(cacheParts, ", "))
			}
			stats = append(stats, tokenStr)
		}

		// Cost
		if cost := base.ExtractFloat64(msg.Metadata, "total_cost_usd"); cost > 0 {
			stats = append(stats, "💵 "+base.FormatCost(cost))
		}

		// Files modified
		if files := base.ExtractInt64(msg.Metadata, "files_modified"); files > 0 {
			stats = append(stats, fmt.Sprintf("📝 %d files", files))
		}

		// Tool calls
		if tools := base.ExtractInt64(msg.Metadata, "tool_call_count"); tools > 0 {
			stats = append(stats, fmt.Sprintf("🔧 %d tools", tools))
		}

		// Model used (from SSE ModelID)
		if model := base.ExtractString(msg.Metadata, "model_used"); model != "" {
			stats = append(stats, "🤖 "+model)
		}

		// Finish reason
		if reason := base.ExtractString(msg.Metadata, "finish_reason"); reason != "" {
			reasonLabel := reason
			switch reason {
			case "end_turn":
				reasonLabel = "✅ 正常结束"
			case "tool_use":
				reasonLabel = "🔧 工具调用"
			case "max_tokens":
				reasonLabel = "⚠️ Token 超限"
			}
			stats = append(stats, reasonLabel)
		}

		if len(stats) > 0 {
			statsText := slack.NewTextBlockObject("mrkdwn", strings.Join(stats, " • "), false, false)
			blocks = append(blocks, slack.NewContextBlock("", statsText))
		}
	}

	return blocks
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
		extras = append(extras, "⏱️ "+base.FormatDuration(durationMs))
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
