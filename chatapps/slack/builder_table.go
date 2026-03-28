// Package slack provides the Slack adapter implementation for the hotplex engine.
// Table builder for structured data display using Slack's native Table Block.
//
// Correct Table Block cell schema per Slack API:
//   - rich_text cells: { "type": "rich_text", "elements": [{ "type": "rich_text_section", "elements": [...] }] }
//   - NO block_id field in cells (slack-go SDK's RichTextBlock.MarshalJSON incorrectly emits it,
//     causing Slack API "invalid_blocks" error).
//
// Solution: Use a custom tableBlock wrapper with MarshalJSON that serializes cells
// without block_id, while still accepting []*RichTextBlock from the SDK for type safety.
package slack

import (
	"encoding/json"
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

// tableCell serializes as a Slack Table Block cell without block_id.
// Slack API only accepts { "type": "rich_text", "elements": [...] } — no block_id.
type tableCell struct {
	Type     string                 `json:"type"`
	Elements []tableRichTextSection `json:"elements"`
}

type tableRichTextSection struct {
	Type     string                     `json:"type"`
	Elements []tableRichTextSectionElem `json:"elements"`
}

// tableRichTextSectionElem is a discriminated union of element types.
// Only fields relevant for JSON serialization are included.
type tableRichTextSectionElem struct {
	Type      string    `json:"type"`
	Text      string    `json:"text,omitempty"`
	URL       string    `json:"url,omitempty"`
	ChannelID string    `json:"channel_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Unicode   string    `json:"unicode,omitempty"`
	SkinTone  int       `json:"skin_tone,omitempty"`
	Style     *styleRef `json:"style,omitempty"`
}

type styleRef struct {
	Bold   bool `json:"bold,omitempty"`
	Italic bool `json:"italic,omitempty"`
	Strike bool `json:"strike,omitempty"`
	Code   bool `json:"code,omitempty"`
}

// tableBlock wraps slack.TableBlock for correct JSON serialization.
// It implements json.Marshaler so that when embedded in a blocks array,
// cells serialize without the block_id field that Slack rejects.
type tableBlock struct {
	*slack.TableBlock
}

// MarshalJSON produces Slack API-compliant JSON for a Table Block.
// The SDK's TableBlock.MarshalJSON would emit block_id inside cells,
// which Slack rejects with "invalid_blocks". We override it here.
func (t tableBlock) MarshalJSON() ([]byte, error) {
	// Build a plain struct with the correct JSON shape.
	type alias struct {
		Type    string                `json:"type"`
		BlockID string                `json:"block_id,omitempty"`
		Rows    [][]tableCell         `json:"rows"`
		ColSets []slack.ColumnSetting `json:"column_settings,omitempty"`
	}

	cells := make([][]tableCell, 0, len(t.TableBlock.Rows))
	for _, row := range t.TableBlock.Rows {
		var rowCells []tableCell
		for _, cell := range row {
			if cell == nil {
				continue
			}
			elems := extractCellElements(cell)
			rowCells = append(rowCells, tableCell{
				Type:     string(cell.Type),
				Elements: elems,
			})
		}
		cells = append(cells, rowCells)
	}

	obj := alias{
		Type:    string(t.Type),
		BlockID: t.BlockID,
		Rows:    cells,
	}
	if len(t.ColumnSettings) > 0 {
		obj.ColSets = t.ColumnSettings
	}
	return json.Marshal(obj)
}

// extractCellElements converts a RichTextBlock's generic []RichTextElement
// into a []tableRichTextSection for JSON serialization.
// Only handles the element types used by this builder.
func extractCellElements(cell *slack.RichTextBlock) []tableRichTextSection {
	if cell == nil || len(cell.Elements) == 0 {
		return nil
	}
	var sections []tableRichTextSection
	for _, elem := range cell.Elements {
		rtSec, ok := elem.(*slack.RichTextSection)
		if !ok {
			continue
		}
		var elems []tableRichTextSectionElem
		for _, se := range rtSec.Elements {
			switch v := se.(type) {
			case slack.RichTextSectionTextElement:
				e := tableRichTextSectionElem{Type: "text"}
				e.Text = v.Text
				if v.Style != nil {
					e.Style = &styleRef{
						Bold:   v.Style.Bold,
						Italic: v.Style.Italic,
						Strike: v.Style.Strike,
						Code:   v.Style.Code,
					}
				}
				elems = append(elems, e)
			case slack.RichTextSectionLinkElement:
				e := tableRichTextSectionElem{Type: "link"}
				e.URL = v.URL
				e.Text = v.Text
				elems = append(elems, e)
			case slack.RichTextSectionChannelElement:
				elems = append(elems, tableRichTextSectionElem{Type: "channel", ChannelID: v.ChannelID})
			case slack.RichTextSectionUserElement:
				elems = append(elems, tableRichTextSectionElem{Type: "user", UserID: v.UserID})
			case slack.RichTextSectionEmojiElement:
				e := tableRichTextSectionElem{Type: "emoji", Name: v.Name, Unicode: v.Unicode}
				if v.SkinTone != 0 {
					e.SkinTone = v.SkinTone
				}
				elems = append(elems, e)
			}
		}
		sections = append(sections, tableRichTextSection{
			Type:     string(rtSec.Type),
			Elements: elems,
		})
	}
	return sections
}

// BuildStatsTable builds a Table Block for session statistics.
// Note: Session stats tables intentionally omit a header row regardless of ShowHeader
// (label/value rows are self-describing), unlike BuildCommandProgressTable and BuildToolCallsTable.
func (tb *TableBuilder) BuildStatsTable(msg *base.ChatMessage) []slack.Block {
	native := slack.NewTableBlock("session_stats")
	native = native.WithColumnSettings(
		slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: false},
		slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: true},
	)

	metadata := msg.Metadata
	if metadata == nil {
		return []slack.Block{tableBlock{TableBlock: native}}
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
	if v := base.ExtractInt64(metadata, "input_tokens"); v > 0 {
		cache := base.ExtractInt64(metadata, "cache_read_tokens")
		addRow("⚡ Input", tb.formatTokenCell(v, cache))
	}
	if v := base.ExtractInt64(metadata, "output_tokens"); v > 0 {
		cache := base.ExtractInt64(metadata, "cache_write_tokens")
		addRow("⚡ Output", tb.formatTokenCell(v, cache))
	}
	addRowOpt("📝 Files", fmt.Sprintf("%d modified", base.ExtractInt64(metadata, "files_modified")), base.ExtractInt64(metadata, "files_modified") > 0)
	addRowOpt("🔧 Tools", fmt.Sprintf("%d calls", base.ExtractInt64(metadata, "tool_call_count")), base.ExtractInt64(metadata, "tool_call_count") > 0)
	if v := base.ExtractString(metadata, "model_used"); v != "" {
		addRow("🤖 Model", v)
	}
	if v := base.ExtractString(metadata, "finish_reason"); v != "" {
		label := v
		switch v {
		case "end_turn":
			label = "✅ 正常结束"
		case "tool_use":
			label = "🔧 工具调用"
		case "max_tokens":
			label = "⚠️ Token 超限"
		}
		addRow("📋 结束原因", label)
	}

	return []slack.Block{tableBlock{TableBlock: native}}
}

// cell creates a simple text cell with empty block_id (Slack API requirement).
func (tb *TableBuilder) cell(text string) *slack.RichTextBlock {
	section := slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(text, nil),
	)
	return slack.NewRichTextBlock("", section) // block_id intentionally empty
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
		return []slack.Block{tableBlock{TableBlock: native}}
	}
	steps, ok := metadata["steps"].([]map[string]any)
	if !ok || len(steps) == 0 {
		return []slack.Block{tableBlock{TableBlock: native}}
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
	return []slack.Block{tableBlock{TableBlock: native}}
}

// BuildToolCallsTable builds a Table Block for tool call summary.
func (tb *TableBuilder) BuildToolCallsTable(msg *base.ChatMessage) []slack.Block {
	native := slack.NewTableBlock("tool_calls")
	metadata := msg.Metadata
	if metadata == nil {
		return []slack.Block{tableBlock{TableBlock: native}}
	}
	toolCalls, ok := metadata["tool_calls"].([]map[string]any)
	if !ok || len(toolCalls) == 0 {
		return []slack.Block{tableBlock{TableBlock: native}}
	}
	if tb.config.ShowHeader {
		native.AddRow(tb.cell("Tool"), tb.cell("Calls"), tb.cell("Success"))
	}
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
		native.AddRow(tb.cell(name), tb.cell(fmt.Sprintf("%d", calls)), tb.cell(success))
	}
	return []slack.Block{tableBlock{TableBlock: native}}
}
