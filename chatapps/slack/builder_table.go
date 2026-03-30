// Package slack provides the Slack adapter implementation for the hotplex engine.
// Table builder for structured data display using Slack's native Table Block.
package slack

import (
	"fmt"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
)

// TableBuilder provides utilities for building Slack Table Blocks.
type TableBuilder struct {
	config TableConfig
}

// TableConfig configures table display behavior.
type TableConfig struct {
	MaxRows    int  // Maximum rows to display (performance protection)
	ShowHeader bool // Whether to show header row
}

// NewTableBuilder creates a new TableBuilder with given configuration.
func NewTableBuilder(config TableConfig) *TableBuilder {
	if config.MaxRows <= 0 {
		config.MaxRows = 10
	}
	return &TableBuilder{config: config}
}

// BuildStatsTable builds a Table Block for session statistics.
// Note: Session stats tables intentionally omit a header row regardless of ShowHeader
// (label/value rows are self-describing), unlike BuildCommandProgressTable.
func (tb *TableBuilder) BuildStatsTable(msg *base.ChatMessage) []slack.Block {
	native := slack.NewTableBlock("session_stats")
	native = native.WithColumnSettings(
		slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: false},
		slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: true},
	)

	metadata := msg.Metadata
	if metadata == nil {
		return []slack.Block{native}
	}

	addRow := func(label, value string) {
		native.AddRow(tb.cell(label), tb.cell(value))
	}
	addRowOpt := func(label string, value string, ok bool) {
		if ok {
			native.AddRow(tb.cell(label), tb.cell(value))
		}
	}

	if v := base.ExtractInt64(metadata, "total_duration_ms"); v > 0 {
		addRow("⏱️ Duration", base.FormatDuration(v))
	}
	if v := base.ExtractInt64(metadata, "thinking_duration_ms"); v > 0 {
		addRow("🧠 Think", base.FormatDuration(v))
	}
	if v := base.ExtractInt64(metadata, "tool_duration_ms"); v > 0 {
		addRow("🔧 Tools", base.FormatDuration(v))
	}
	if v := base.ExtractFloat64(metadata, "context_used_percent"); v > 0 {
		addRow("🧠 Context", fmt.Sprintf("%.0f%%", v))
	}
	if v := base.ExtractInt64(metadata, "input_tokens"); v > 0 {
		cache := base.ExtractInt64(metadata, "cache_read_tokens")
		addRow("⚡ Input", tb.formatTokenCell(v, cache))
	}
	if v := base.ExtractInt64(metadata, "output_tokens"); v > 0 {
		cache := base.ExtractInt64(metadata, "cache_write_tokens")
		addRow("⚡ Output", tb.formatTokenCell(v, cache))
	}
	if v := base.ExtractFloat64(metadata, "total_cost_usd"); v > 0 {
		addRow("💵 Cost", base.FormatCost(v))
	}
	addRowOpt("📝 Files", fmt.Sprintf("%d modified", base.ExtractInt64(metadata, "files_modified")), base.ExtractInt64(metadata, "files_modified") > 0)
	if v := base.ExtractString(metadata, "model_used"); v != "" {
		addRow("🤖 Model", v)
	}
	if v := base.ExtractString(metadata, "finish_reason"); v != "" {
		label := v
		switch v {
		case "end_turn":
			label = "✅ 正常结束"
		case "tool_use", "tool_calls":
		case "max_tokens":
			label = "⚠️ Token 超限"
		default:
			label = "❓ " + v
		}
		if v != "tool_use" && v != "tool_calls" {
			addRow("📋 结束", label)
		}
	}

	return []slack.Block{native}
}

// cell creates a simple text cell with empty block_id.
func (tb *TableBuilder) cell(text string) *slack.RichTextBlock {
	section := slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(text, nil),
	)
	return slack.NewRichTextBlock("", section)
}

// formatTokenCell formats token count with optional cache info.
func (tb *TableBuilder) formatTokenCell(total, cache int64) string {
	s := base.FormatTokenCount(total)
	if cache > 0 {
		s += fmt.Sprintf(" (cache: %s)", base.FormatTokenCount(cache))
	}
	return s
}

// BuildCommandProgressTable builds a Table Block for command progress.
func (tb *TableBuilder) BuildCommandProgressTable(msg *base.ChatMessage) []slack.Block {
	native := slack.NewTableBlock("command_progress")
	metadata := msg.Metadata
	if metadata == nil {
		return []slack.Block{native}
	}
	steps, ok := metadata["steps"].([]map[string]any)
	if !ok || len(steps) == 0 {
		return []slack.Block{native}
	}
	if tb.config.ShowHeader {
		native.AddRow(tb.cell("Step"), tb.cell("Status"), tb.cell("Details"))
	}
	for i, step := range steps {
		if i >= tb.config.MaxRows {
			break
		}
		num := fmt.Sprintf("%d/%d", i+1, len(steps))
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
		native.AddRow(tb.cell(num), tb.cell(status), tb.cell(details))
	}
	return []slack.Block{native}
}
