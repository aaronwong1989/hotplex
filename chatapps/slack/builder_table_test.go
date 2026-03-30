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

// getNativeTable extracts the underlying *slack.TableBlock from a []slack.Block.
func getNativeTable(blocks []slack.Block) *slack.TableBlock {
	if len(blocks) == 0 {
		return nil
	}
	if tb, ok := blocks[0].(*slack.TableBlock); ok {
		return tb
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
	assert.Len(t, table.Rows, 4) // Duration + Input + Output + Files (no Tools row in table)
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

// TestBuildStatsTable_NoBlockIdInCells verifies that SDK serialization
// correctly omits block_id in cells (empty string + omitempty).
func TestBuildStatsTable_NoBlockIdInCells(t *testing.T) {
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

	// Verify cells have empty BlockID
	table := getNativeTable(blocks)
	for i, row := range table.Rows {
		for j, cell := range row {
			assert.Equal(t, "", cell.BlockID,
				"cell [%d][%d] must have empty BlockID, got %q", i, j, cell.BlockID)
		}
	}

	// Verify JSON output only contains the table's own block_id
	payload, err := json.Marshal(blocks[0])
	assert.NoError(t, err)
	jsonStr := string(payload)
	assert.Equal(t, 1, strings.Count(jsonStr, `"block_id"`),
		"JSON should only contain 1 block_id (the table's own id), got: %s", jsonStr)
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

	// Should be native *slack.TableBlock
	tb, ok := blocks[0].(*slack.TableBlock)
	assert.True(t, ok, "BuildSessionStatsTable must return *slack.TableBlock")
	assert.Equal(t, slack.MBTTable, tb.Type)
	assert.Len(t, tb.Rows, 4) // Duration + Input + Output + Files (no Tools row in table)
}
