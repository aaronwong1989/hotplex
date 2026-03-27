package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMarkdownConverter(t *testing.T) {
	config := ConverterConfig{
		EnableTables:      true,
		EnableCodeBlocks:  true,
		EnableQuotes:     true,
		EnableLists:      true,
	}

	converter := NewMarkdownConverter(config)

	assert.NotNil(t, converter)
	assert.True(t, converter.config.EnableTables)
	assert.True(t, converter.config.EnableCodeBlocks)
	assert.Equal(t, 20, converter.config.MaxTableRows)
}

func TestNewMarkdownConverter_Defaults(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	assert.Equal(t, 20, converter.config.MaxTableRows)
	assert.Equal(t, 100, converter.config.MaxCodeBlockLines)
}

func TestIsTableSeparator(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	tests := []struct {
		line     string
		expected bool
	}{
		{"|---|---|", true},
		{"| --- | --- |", true},
		{"|:---|:---|", true},
		{"| Header 1 | Header 2 |", false},
		{"Not a table", false},
		{"", false},
	}

	for _, tt := range tests {
		result := converter.isTableSeparator(tt.line)
		assert.Equal(t, tt.expected, result, "Line: %q", tt.line)
	}
}

func TestDetectMarkdownTable_SimpleTable(t *testing.T) {
	text := "| Name | Age | City |\n|------|-----|------|\n| Alice | 30  | NYC  |\n| Bob   | 25  | LA   |"

	header, rows, found := DetectMarkdownTable(text)

	assert.True(t, found)
	assert.Equal(t, []string{"Name", "Age", "City"}, header)
	assert.Len(t, rows, 2)
	assert.Equal(t, []string{"Alice", "30", "NYC"}, rows[0])
	assert.Equal(t, []string{"Bob", "25", "LA"}, rows[1])
}

func TestDetectMarkdownTable_NoTable(t *testing.T) {
	text := "This is just regular text\nNo table here"

	_, _, found := DetectMarkdownTable(text)

	assert.False(t, found)
}

func TestDetectMarkdownTable_EmptyInput(t *testing.T) {
	_, _, found := DetectMarkdownTable("")

	assert.False(t, found)
}

func TestParseTableRow(t *testing.T) {
	tests := []struct {
		line     string
		expected []string
	}{
		{"| A | B | C |", []string{"A", "B", "C"}},
		{"|  One  |  Two  |  Three  |", []string{"One", "Two", "Three"}},
		{"|Single|", []string{"Single"}},
	}

	for _, tt := range tests {
		result := parseTableRow(tt.line)
		assert.Equal(t, tt.expected, result)
	}
}

func TestConvertTableToSlackBlock_Basic(t *testing.T) {
	header := []string{"Name", "Age"}
	rows := [][]string{
		{"Alice", "30"},
		{"Bob", "25"},
	}

	table := ConvertTableToSlackBlock(header, rows)

	assert.NotNil(t, table)
	assert.Equal(t, "markdown_table", table.BlockID)
	assert.Len(t, table.Rows, 3) // Header + 2 data rows
}
