// Package slack provides the Slack adapter implementation for the hotplex engine.
// Table builder for structured data display using Slack's native Table Block.
package slack

import (
	"fmt"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
)

// TableBuilder provides utilities for building Slack Table Blocks
// Supports structured data display with better readability than single-line format
type TableBuilder struct {
	config TableConfig
}

// TableConfig configures table display behavior
type TableConfig struct {
	MaxRows    int  // Maximum rows to display (performance protection)
	Compact    bool // Compact mode for mobile optimization
	ShowHeader bool // Whether to show header row
}

// NewTableBuilder creates a new TableBuilder with given configuration
func NewTableBuilder(config TableConfig) *TableBuilder {
	if config.MaxRows <= 0 {
		config.MaxRows = 10 // Default max rows
	}
	return &TableBuilder{config: config}
}

// BuildStatsTable builds a Table Block for session statistics
// Displays: Duration, Input/Output Tokens (with cache), Files Modified, Tool Calls
func (tb *TableBuilder) BuildStatsTable(msg *base.ChatMessage) *slack.TableBlock {
	table := slack.NewTableBlock("session_stats")

	// Configure column settings for proper rendering
	// Column 0: Label (left-aligned, not wrapped)
	// Column 1: Value (left-aligned, wrapped for long text)
	table = table.WithColumnSettings(
		slack.ColumnSetting{
			Align:     slack.ColumnAlignmentLeft,
			IsWrapped: false,
		},
		slack.ColumnSetting{
			Align:     slack.ColumnAlignmentLeft,
			IsWrapped: true,
		},
	)

	metadata := msg.Metadata

	if metadata == nil {
		return table
	}

	// Row 1: Duration
	if duration := base.ExtractInt64(metadata, "total_duration_ms"); duration > 0 {
		table.AddRow(
			tb.buildLabelCell("⏱️ Duration"),
			tb.buildValueCell(base.FormatDuration(duration)),
		)
	}

	// Row 2-3: Input/Output Tokens with cache info
	tokensIn := base.ExtractInt64(metadata, "input_tokens")
	tokensOut := base.ExtractInt64(metadata, "output_tokens")
	cacheRead := base.ExtractInt64(metadata, "cache_read_tokens")
	cacheWrite := base.ExtractInt64(metadata, "cache_write_tokens")

	if tokensIn > 0 {
		table.AddRow(
			tb.buildLabelCell("⚡ Input"),
			tb.buildTokenValueCell(tokensIn, cacheRead),
		)
	}

	if tokensOut > 0 {
		table.AddRow(
			tb.buildLabelCell("⚡ Output"),
			tb.buildTokenValueCell(tokensOut, cacheWrite),
		)
	}

	// Row 4: Files Modified
	if files := base.ExtractInt64(metadata, "files_modified"); files > 0 {
		table.AddRow(
			tb.buildLabelCell("📝 Files"),
			tb.buildValueCell(fmt.Sprintf("%d modified", files)),
		)
	}

	// Row 5: Tool Calls
	if tools := base.ExtractInt64(metadata, "tool_call_count"); tools > 0 {
		table.AddRow(
			tb.buildLabelCell("🔧 Tools"),
			tb.buildValueCell(fmt.Sprintf("%d calls", tools)),
		)
	}

	// Row 6: Model Used (from SSE ModelID metadata)
	if model := base.ExtractString(metadata, "model_used"); model != "" {
		table.AddRow(
			tb.buildLabelCell("🤖 Model"),
			tb.buildValueCell(model),
		)
	}

	// Row 7: Finish Reason (end_turn / tool_use / max_tokens)
	if reason := base.ExtractString(metadata, "finish_reason"); reason != "" {
		reasonLabel := reason
		switch reason {
		case "end_turn":
			reasonLabel = "✅ 正常结束"
		case "tool_use":
			reasonLabel = "🔧 工具调用"
		case "max_tokens":
			reasonLabel = "⚠️ Token 超限"
		}
		table.AddRow(
			tb.buildLabelCell("📋 结束原因"),
			tb.buildValueCell(reasonLabel),
		)
	}

	return table
}

// buildLabelCell creates a label cell (left column) with consistent styling
func (tb *TableBuilder) buildLabelCell(text string) *slack.RichTextBlock {
	section := slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(text, nil),
	)
	return slack.NewRichTextBlock("label", section)
}

// buildValueCell creates a value cell (right column) with consistent styling
func (tb *TableBuilder) buildValueCell(text string) *slack.RichTextBlock {
	section := slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(text, nil),
	)
	return slack.NewRichTextBlock("value", section)
}

// buildTokenValueCell formats token count with cache information
// Format: "100K (cache: 10K)" or "100K" if no cache
func (tb *TableBuilder) buildTokenValueCell(total, cache int64) *slack.RichTextBlock {
	text := base.FormatTokenCount(total)
	if cache > 0 {
		text += fmt.Sprintf(" (cache: %s)", base.FormatTokenCount(cache))
	}
	return tb.buildValueCell(text)
}

// BuildCommandProgressTable builds a Table Block for command progress tracking
// Displays multi-step command execution with status for each step
// Format: Step | Status | Details
func (tb *TableBuilder) BuildCommandProgressTable(msg *base.ChatMessage) *slack.TableBlock {
	table := slack.NewTableBlock("command_progress")
	metadata := msg.Metadata

	if metadata == nil {
		return table
	}

	// Get steps from metadata
	steps, ok := metadata["steps"].([]map[string]any)
	if !ok || len(steps) == 0 {
		return table
	}

	// Add header row
	if tb.config.ShowHeader {
		table.AddRow(
			tb.buildLabelCell("Step"),
			tb.buildLabelCell("Status"),
			tb.buildLabelCell("Details"),
		)
	}

	// Add step rows
	for i, step := range steps {
		if i >= tb.config.MaxRows {
			break
		}

		stepNum := fmt.Sprintf("%d/%d", i+1, len(steps))
		status := "⏸️ Pending"
		details := ""

		if s, ok := step["status"].(string); ok {
			switch s {
			case "done":
				status = "✅ Done"
			case "running":
				status = "🔄 Running"
			case "failed":
				status = "❌ Failed"
			}
		}
		if d, ok := step["details"].(string); ok {
			details = d
		}

		table.AddRow(
			tb.buildValueCell(stepNum),
			tb.buildValueCell(status),
			tb.buildValueCell(details),
		)
	}

	return table
}

// BuildToolCallsTable builds a Table Block for tool call summary
// Displays: Tool Name | Call Count | Success Rate
func (tb *TableBuilder) BuildToolCallsTable(msg *base.ChatMessage) *slack.TableBlock {
	table := slack.NewTableBlock("tool_calls")
	metadata := msg.Metadata

	if metadata == nil {
		return table
	}

	// Get tool calls from metadata
	toolCalls, ok := metadata["tool_calls"].([]map[string]any)
	if !ok || len(toolCalls) == 0 {
		return table
	}

	// Add header row
	if tb.config.ShowHeader {
		table.AddRow(
			tb.buildLabelCell("Tool"),
			tb.buildLabelCell("Calls"),
			tb.buildLabelCell("Success"),
		)
	}

	// Add tool rows
	for i, tool := range toolCalls {
		if i >= tb.config.MaxRows {
			break
		}

		name := ""
		calls := int64(0)
		success := "100%"

		if n, ok := tool["name"].(string); ok {
			name = n
		}
		if c, ok := tool["count"].(int64); ok {
			calls = c
		} else if c, ok := tool["count"].(int); ok {
			calls = int64(c)
		}
		if s, ok := tool["success_rate"].(string); ok {
			success = s
		}

		table.AddRow(
			tb.buildValueCell(name),
			tb.buildValueCell(fmt.Sprintf("%d", calls)),
			tb.buildValueCell(success),
		)
	}

	return table
}
