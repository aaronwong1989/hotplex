package slack

import (
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
)

func TestNewTableBuilder(t *testing.T) {
	config := TableConfig{
		MaxRows:    5,
		Compact:    false,
		ShowHeader: true,
	}

	builder := NewTableBuilder(config)

	assert.NotNil(t, builder)
	assert.Equal(t, 5, builder.config.MaxRows)
	assert.False(t, builder.config.Compact)
	assert.True(t, builder.config.ShowHeader)
}

func TestNewTableBuilder_DefaultMaxRows(t *testing.T) {
	// When MaxRows is 0 or negative, should use default (10)
	builder := NewTableBuilder(TableConfig{})

	assert.Equal(t, 10, builder.config.MaxRows)
}

func TestBuildStatsTable_BasicStats(t *testing.T) {
	builder := NewTableBuilder(TableConfig{
		MaxRows:    5,
		Compact:    false,
		ShowHeader: false,
	})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms":  int64(125000), // 2m5s
			"input_tokens":       int64(120000),
			"output_tokens":      int64(35000),
			"cache_read_tokens":  int64(80000),
			"cache_write_tokens": int64(20000),
			"tool_call_count":    int64(5),
			"files_modified":     int64(3),
		},
	}

	table := builder.BuildStatsTable(msg)

	assert.NotNil(t, table)
	assert.Equal(t, "session_stats", table.BlockID)
	assert.Equal(t, slack.MBTTable, table.Type)
	assert.Len(t, table.Rows, 5) // Duration, Input, Output, Files, Tools
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

	table := builder.BuildStatsTable(msg)

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
			// No cache tokens
		},
	}

	table := builder.BuildStatsTable(msg)

	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 3)
}

func TestBuildStatsTable_PartialStats(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	// Only tokens
	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"input_tokens":  int64(100000),
			"output_tokens": int64(50000),
		},
	}

	table := builder.BuildStatsTable(msg)

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

	table := builder.BuildStatsTable(msg)

	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 0) // No rows when metadata is nil
}

func TestBuildStatsTable_EmptyMetadata(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:     base.MessageTypeSessionStats,
		Content:  "",
		Metadata: map[string]any{},
	}

	table := builder.BuildStatsTable(msg)

	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 0)
}

func TestBuildStatsTable_Int32Types(t *testing.T) {
	// Verify int32 types are handled correctly (from SessionStatsData)
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"total_duration_ms": int64(125000),
			"input_tokens":      int32(120000), // int32
			"output_tokens":     int32(35000),  // int32
			"tool_call_count":   int32(5),      // int32
			"files_modified":    int32(3),      // int32
		},
	}

	table := builder.BuildStatsTable(msg)

	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 5)
}

func TestTableBuilder_LabelAndValueCells(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	labelCell := builder.buildLabelCell("⏱️ Duration")
	assert.NotNil(t, labelCell)
	assert.Equal(t, "label", labelCell.BlockID)

	valueCell := builder.buildValueCell("2m5s")
	assert.NotNil(t, valueCell)
	assert.Equal(t, "value", valueCell.BlockID)
}

func TestTableBuilder_TokenValueCell(t *testing.T) {
	builder := NewTableBuilder(TableConfig{MaxRows: 10})

	// Without cache
	cell := builder.buildTokenValueCell(120000, 0)
	assert.NotNil(t, cell)

	// With cache
	cellWithCache := builder.buildTokenValueCell(120000, 80000)
	assert.NotNil(t, cellWithCache)
}

func TestBuildSessionStatsTable_FromStatsMessageBuilder(t *testing.T) {
	// Test the integration with StatsMessageBuilder
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

	// Should be a TableBlock
	tableBlock, ok := blocks[0].(*slack.TableBlock)
	assert.True(t, ok)
	assert.Equal(t, slack.MBTTable, tableBlock.Type)
	assert.Len(t, tableBlock.Rows, 5)
}

func TestBuildCommandProgressTable_Basic(t *testing.T) {
	builder := NewTableBuilder(TableConfig{
		MaxRows:    10,
		Compact:    false,
		ShowHeader: true,
	})

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

	table := builder.BuildCommandProgressTable(msg)

	assert.NotNil(t, table)
	// With header + 4 data rows = 5 total rows
	assert.Len(t, table.Rows, 5)
}

func TestBuildToolCallsTable_Basic(t *testing.T) {
	builder := NewTableBuilder(TableConfig{
		MaxRows:    10,
		Compact:    false,
		ShowHeader: true,
	})

	msg := &base.ChatMessage{
		Type:    base.MessageTypeSessionStats,
		Content: "",
		Metadata: map[string]any{
			"tool_calls": []map[string]any{
				{"name": "Read", "count": int64(15), "success_rate": "100%"},
				{"name": "Edit", "count": int64(8), "success_rate": "100%"},
				{"name": "Bash", "count": int64(5), "success_rate": "80%"},
			},
		},
	}

	table := builder.BuildToolCallsTable(msg)

	assert.NotNil(t, table)
	assert.Len(t, table.Rows, 4) // Header + 3 tool rows
}
