package slack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
)

func TestNewTableBuilder(t *testing.T) {
	config := TableConfig{
		MaxRows:    5,
		ShowHeader: true,
	}

	builder := NewTableBuilder(config)

	assert.NotNil(t, builder)
	assert.Equal(t, 5, builder.config.MaxRows)
	assert.True(t, builder.config.ShowHeader)
}

func TestNewTableBuilder_DefaultMaxRows(t *testing.T) {
	builder := NewTableBuilder(TableConfig{})
	assert.Equal(t, 10, builder.config.MaxRows)
}

// getNativeTable extracts the underlying *slack.TableBlock from a []slack.Block returned by builder methods.
func getNativeTable(blocks []slack.Block) *slack.TableBlock {
	if len(blocks) == 0 {
		return nil
	}
	if tb, ok := blocks[0].(tableBlock); ok {
		return tb.TableBlock
	}
	return nil
}

func TestBuildStatsTable_BasicStats(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 5, ShowHeader: false})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms":  int64(125000),
			"input_tokens":       int64(120000),
			"output_tokens":      int64(35000),
			"cache_read_tokens":  int64(80000),
			"cache_write_tokens": int64(20000),
			"tool_call_count":    int64(5),
			"files_modified":     int64(3),
		},
	}

	blocks := builder.BuildStatsTable(msg)
	table := getNativeTable(blocks)

	assert.NotNil(t, table)
	assert.Equal(t, "session_stats", table.BlockID)
	assert.Equal(t, slack.MBTTable, table.Type)
	assert.Len(t, table.Rows, 4) // Duration + Input + Output + Files (no Tools row in table) // Duration, Input, Output, Files, Tools
}

func TestBuildStatsTable_WithCacheTokens(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms":  int64(60000),
			"input_tokens":       int64(100000),
			"output_tokens":      int64(50000),
			"cache_read_tokens":  int64(60000),
			"cache_write_tokens": int64(30000),
		},
	}

	table := getNativeTable(builder.BuildStatsTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 3) // Duration, Input, Output
}

func TestBuildStatsTable_WithoutCacheTokens(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms": int64(125000),
			"input_tokens":      int64(120000),
			"output_tokens":     int64(35000),
		},
	}

	table := getNativeTable(builder.BuildStatsTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 3)
}

func TestBuildStatsTable_PartialStats(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"input_tokens":  int64(100000),
			"output_tokens": int64(50000),
		},
	}

	table := getNativeTable(builder.BuildStatsTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 2) // Only Input and Output
}

func TestBuildStatsTable_NilMetadata(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:     base.MessageTypeSessionStats,
		Content:  "",
		Metadata: nil,
	}

	table := getNativeTable(builder.BuildStatsTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 0)
}

func TestBuildStatsTable_EmptyMetadata(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:     base.MessageTypeSessionStats,
		Content:  "",
		Metadata: map[string]any{},
	}

	table := getNativeTable(builder.BuildStatsTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 0)
}

func TestBuildStatsTable_Int32Types(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms": int64(125000),
			"input_tokens":      int32(120000),
			"output_tokens":     int32(35000),
			"tool_call_count":   int32(5),
			"files_modified":    int32(3),
		},
	}

	table := getNativeTable(builder.BuildStatsTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 4) // Duration + Input + Output + Files (no Tools row in table)
}

// TestBuildStatsTable_NoBlockIdInJSON verifies the critical fix:
// Table Block cells must NOT include block_id in the JSON payload.
// Slack API rejects cells with block_id set.
func TestBuildStatsTable_NoBlockIdInJSON(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms": int64(60000),
			"input_tokens":      int64(1000),
			"output_tokens":     int64(200),
		},
	}

	blocks := builder.BuildStatsTable(msg)
	assert.Len(t, blocks, 1)

	// Marshal the tableBlock and verify no block_id in cells
	payload, err := json.Marshal(blocks[0])
	assert.NoError(t, err)
	jsonStr := string(payload)

	// Should be a table block
	assert.Contains(t, jsonStr, `"type":"table"`)
	assert.Contains(t, jsonStr, `"rows"`)
	assert.Contains(t, jsonStr, `"rich_text"`)
	assert.Contains(t, jsonStr, `"rich_text_section"`)

	// Must NOT contain block_id in the JSON (the root block_id is fine)
	assert.NotContains(t, jsonStr, `"block_id":"label"`, "cell must not have block_id='label'")
	assert.NotContains(t, jsonStr, `"block_id":"value"`, "cell must not have block_id='value'")
}

func TestBuildCommandProgressTable_Basic(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10, ShowHeader: true})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeCommandProgress,
		Content: "",
		Metadata: map[string]any{
			"steps": []map[string]any{
				{"status": "done", "details": "Version bump"},
				{"status": "running", "details": "Git commit"},
				{"status": "pending", "details": "CI verification"},
				{"status": "pending", "details": "GitHub release"},
			},
		},
	}

	table := getNativeTable(builder.BuildCommandProgressTable(msg))
	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 5) // Header + 4 data rows
}

func TestBuildToolCallsTable_Removed(t *testing.T) {
	// BuildToolCallsTable is stubbed — metadata["tool_calls"] is never populated.
	// Engine does not provide per-tool call statistics; tool_call_count is used instead.
	builder := NewTableBuilder(TableConfig{MaxRows: 10, ShowHeader: true})
	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"tool_calls": []map[string]any{
				{"name": "Read", "count": int64(15), "success_rate": "100%"},
			},
		},
	}
	blocks := builder.BuildToolCallsTable(msg)
	assert.Nil(t, blocks)
}

func TestBuildSessionStatsTable_FromStatsMessageBuilder(t *testing.T) {
	config := &Config{
		Features: FeaturesConfig{
			Markdown: MarkdownConfig{
				TableConfig: &TableConversionConfig{
					Enabled: PtrBool(true),
					MaxRows: 20,
				},
			},
		},
	}
	statsBuilder := NewStatsMessageBuilder(config)

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms": int64(125000),
			"input_tokens":      int64(120000),
			"output_tokens":     int64(35000),
			"tool_call_count":   int64(5),
			"files_modified":    int64(3),
		},
	}

	blocks := statsBuilder.BuildSessionStatsTable(msg)

	assert.NotNil(t, blocks)
	assert.Len(t, blocks, 1)

	// Should be tableBlock (our wrapper), not plain *slack.TableBlock
	tb, ok := blocks[0].(tableBlock)
	assert.True(t, ok, "BuildSessionStatsTable must return tableBlock")
	assert.Equal(t, slack.MBTTable, tb.Type)
	assert.Len(t, tb.Rows, 4) // Duration + Input + Output + Files (no Tools row in table)
}

func TestTableCell_EmptyBlockID(t *testing.T) {
	// Verify that cells created with cell() have empty block_id
	builder := NewTableBuilder(TableConfig{MaxRows: 10})
	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms": int64(5000),
			"input_tokens":      int64(100),
			"output_tokens":     int64(50),
		},
	}
	blocks := builder.BuildStatsTable(msg)
	table := getNativeTable(blocks)
	assert.NotNil(t, table)

	// Check all cells have empty BlockID (via the native tableBlock)
	for i, row := range table.Rows {
		for j, cell := range row {
			assert.Equal(t, "", cell.BlockID,
				"cell [%d][%d] must have empty BlockID, got %q", i, j, cell.BlockID)
		}
	}

	// Verify JSON output does not contain cell block_ids
	payload, err := json.Marshal(blocks[0])
	assert.NoError(t, err)
	jsonStr := string(payload)
	// Count block_id occurrences - should be exactly 1 (the table block's own block_id)
	assert.Equal(t, 1, strings.Count(jsonStr, `"block_id"`),
		"JSON should only contain 1 block_id (the table's own id), got: %s", jsonStr)
}
