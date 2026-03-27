package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Comprehensive tests for MarkdownConverter to improve coverage

func TestMarkdownConverter_TableWithSurroundingText(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableTables: true,
		MaxTableRows: 20,
	})

	input := `Before the table

| Name | Age |
|------|-----|
| Alice | 30 |

After the table`

	blocks := converter.ConvertToBlocks(input)
	assert.GreaterOrEqual(t, len(blocks), 3, "Should have before text, table, and after text")
}

func TestMarkdownConverter_TableWithNoHeader(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableTables: true,
		MaxTableRows: 20,
	})

	input := `|------|-----|
| Alice | 30 |`

	blocks := converter.ConvertToBlocks(input)
	// No valid table without header
	assert.GreaterOrEqual(t, len(blocks), 1)
}

func TestMarkdownConverter_TableExceedsMaxRows(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableTables: true,
		MaxTableRows: 2,
	})

	input := `| Name |
|------|
| R1   |
| R2   |
| R3   |
| R4   |`

	blocks := converter.ConvertToBlocks(input)
	// Should still create table but with limited rows
	assert.GreaterOrEqual(t, len(blocks), 1)
}

func TestMarkdownConverter_CodeBlockWithBackticks(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableCodeBlocks:  true,
		MaxCodeBlockLines: 100,
	})

	input := "```\ncode block\n```"

	blocks := converter.ConvertToBlocks(input)
	assert.Equal(t, 1, len(blocks))
}

func TestMarkdownConverter_CodeBlockNotClosed(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableCodeBlocks:  true,
		MaxCodeBlockLines: 100,
	})

	input := "```\nunclosed code"

	blocks := converter.ConvertToBlocks(input)
	assert.GreaterOrEqual(t, len(blocks), 1)
}

func TestMarkdownConverter_CodeBlockExceedsLines(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableCodeBlocks:  true,
		MaxCodeBlockLines: 2,
	})

	input := "```\nline1\nline2\nline3\n```"

	blocks := converter.ConvertToBlocks(input)
	// Should fallback to text when exceeds limit
	assert.Equal(t, 1, len(blocks))
}

func TestMarkdownConverter_QuoteBlock(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableQuotes: true,
	})

	input := "> This is a quote\n> Another line"

	blocks := converter.ConvertToBlocks(input)
	assert.Equal(t, 2, len(blocks))
}

func TestMarkdownConverter_MixedContent(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableTables:      true,
		EnableCodeBlocks:  true,
		EnableQuotes:      true,
		EnableLists:       true,
		MaxTableRows:      20,
		MaxCodeBlockLines: 100,
	})

	input := `# Header

> Important quote

` + "```" + `
code here
` + "```" + `

- List item 1
- List item 2

| Col1 | Col2 |
|------|------|
| A    | B    |
`

	blocks := converter.ConvertToBlocks(input)
	assert.GreaterOrEqual(t, len(blocks), 6, "Should create multiple blocks")
}

func TestMarkdownConverter_InlineCode(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	input := "This has `inline code` in text"

	blocks := converter.ConvertToBlocks(input)
	assert.Equal(t, 1, len(blocks))
}

func TestMarkdownConverter_EmptyInput(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	blocks := converter.ConvertToBlocks("")
	assert.Equal(t, 0, len(blocks))
}

func TestMarkdownConverter_OnlyNewlines(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	blocks := converter.ConvertToBlocks("\n\n\n")
	assert.Equal(t, 0, len(blocks))
}

func TestMarkdownConverter_AllFeaturesDisabled(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableTables:     false,
		EnableCodeBlocks: false,
		EnableQuotes:     false,
		EnableLists:      false,
	})

	input := `| Table |
|-------|
| Cell  |

` + "```" + `
code
` + "```" + `

> quote

- list`

	blocks := converter.ConvertToBlocks(input)
	// All should be plain text
	assert.GreaterOrEqual(t, len(blocks), 1)
}

func TestMarkdownConverter_TableParsingEdgeCases(t *testing.T) {
	t.Run("Empty cells", func(t *testing.T) {
		header, rows, found := DetectMarkdownTable("| | |\n|---|---|\n| | |")
		assert.True(t, found)
		assert.Equal(t, []string{"", ""}, header)
		assert.Equal(t, 1, len(rows))
	})

	t.Run("Colon alignment", func(t *testing.T) {
		header, rows, found := DetectMarkdownTable("| Name |\n|:----|\n| Test |")
		assert.True(t, found)
		assert.Equal(t, []string{"Name"}, header)
		assert.Equal(t, 1, len(rows))
	})

	t.Run("Multiple spaces", func(t *testing.T) {
		header, rows, found := DetectMarkdownTable("|  A  |  B  |\n|-----|-----|\n|  1  |  2  |")
		assert.True(t, found)
		assert.Equal(t, []string{"A", "B"}, header)
		assert.Equal(t, 1, len(rows))
	})
}

func TestConvertTableToSlackBlock_EdgeCases(t *testing.T) {
	t.Run("Empty table", func(t *testing.T) {
		table := ConvertTableToSlackBlock([]string{}, [][]string{})
		assert.NotNil(t, table)
	})

	t.Run("Header only", func(t *testing.T) {
		table := ConvertTableToSlackBlock([]string{"A", "B"}, [][]string{})
		assert.NotNil(t, table)
		assert.Equal(t, 1, len(table.Rows))
	})

	t.Run("Single row", func(t *testing.T) {
		table := ConvertTableToSlackBlock([]string{"Name"}, [][]string{{"Alice"}})
		assert.NotNil(t, table)
		assert.Equal(t, 2, len(table.Rows))
	})
}

func TestMarkdownConverter_ListVariations(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{
		EnableLists: true,
	})

	t.Run("Mixed markers", func(t *testing.T) {
		input := "- Dash\n* Asterisk\n+ Plus"
		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 3, len(blocks))
	})

	t.Run("Ordered list", func(t *testing.T) {
		input := "1. First\n2. Second\n3. Third"
		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 3, len(blocks))
	})

	t.Run("Nested list (not supported)", func(t *testing.T) {
		input := "- Item\n  - Nested"
		blocks := converter.ConvertToBlocks(input)
		// Should handle as regular text or flat list
		assert.GreaterOrEqual(t, len(blocks), 1)
	})
}

func TestMarkdownConverter_TextWithMarkdown(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	input := "This has **bold** and *italic* text"

	blocks := converter.ConvertToBlocks(input)
	assert.Equal(t, 1, len(blocks))
}

func TestMarkdownConverter_MultipleParagraphs(t *testing.T) {
	converter := NewMarkdownConverter(ConverterConfig{})

	input := "First paragraph\n\nSecond paragraph\n\nThird paragraph"

	blocks := converter.ConvertToBlocks(input)
	assert.Equal(t, 1, len(blocks))
}
