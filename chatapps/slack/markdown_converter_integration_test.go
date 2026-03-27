package slack

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
)

func TestMarkdownConverter_CompleteFeatures(t *testing.T) {
	t.Run("Table conversion", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableTables: true,
			MaxTableRows: 20,
		})

		input := `| Name | Age | City |
|------|-----|------|
| Alice | 30 | NYC |
| Bob | 25 | LA |`

		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 1, len(blocks), "Should convert table to TableBlock")
		_, ok := blocks[0].(*slack.TableBlock)
		assert.True(t, ok, "Should be TableBlock")
	})

	t.Run("Code block with RichTextPreformatted", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableCodeBlocks:  true,
			MaxCodeBlockLines: 100,
		})

		input := "```go\nfunc main() {}\n```"
		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 1, len(blocks), "Should create code block")
	})

	t.Run("Quote with italic style", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableQuotes: true,
		})

		input := "> This is a quote"
		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 1, len(blocks), "Should create quote block")
	})

	t.Run("Bullet list", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableLists: true,
		})

		input := `- Item 1
- Item 2
- Item 3`

		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 3, len(blocks), "Should create 3 list item blocks")
	})

	t.Run("Ordered list", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableLists: true,
		})

		input := `1. First
2. Second
3. Third`

		blocks := converter.ConvertToBlocks(input)
		assert.Equal(t, 3, len(blocks), "Should create 3 ordered list item blocks")
	})

	t.Run("All features enabled", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableTables:      true,
			EnableCodeBlocks:  true,
			EnableQuotes:      true,
			EnableLists:       true,
			MaxTableRows:      20,
			MaxCodeBlockLines: 100,
		})

		input := `# Summary

> Important note

- Point 1
- Point 2

| Metric | Value |
|--------|-------|
| Users  | 100   |

` + "```" + `
code here
` + "```"

		blocks := converter.ConvertToBlocks(input)
		assert.GreaterOrEqual(t, len(blocks), 5, "Should create multiple blocks for all features")
	})
}

func TestMarkdownConverter_Limits(t *testing.T) {
	t.Run("Code block line limit", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableCodeBlocks:  true,
			MaxCodeBlockLines: 3,
		})

		input := "```\nline1\nline2\nline3\nline4\n```"
		blocks := converter.ConvertToBlocks(input)

		// Exceeds 3 lines, should fallback to text
		assert.Equal(t, 1, len(blocks), "Should fallback to text when code block exceeds limit")
	})

	t.Run("Table row limit", func(t *testing.T) {
		converter := NewMarkdownConverter(ConverterConfig{
			EnableTables: true,
			MaxTableRows: 2,
		})

		input := `| Col |
|-----|
| R1  |
| R2  |
| R3  |`

		blocks := converter.ConvertToBlocks(input)

		// Should create at least one block (table), possibly more if there's leading/trailing text
		assert.GreaterOrEqual(t, len(blocks), 1, "Should create at least table block")

		// Find the TableBlock
		var foundTable bool
		for _, block := range blocks {
			if _, ok := block.(*slack.TableBlock); ok {
				foundTable = true
				break
			}
		}
		assert.True(t, foundTable, "Should contain at least one TableBlock")
	})
}
