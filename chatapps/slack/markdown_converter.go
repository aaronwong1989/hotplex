// Package slack provides the Slack adapter implementation for the hotplex engine.
// Markdown converter - converts Markdown to Slack Block Kit components.
package slack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
)

// MarkdownConverter converts Markdown to Slack Block Kit components.
// Provides enhanced UI/UX compared to plain mrkdwn text.
type MarkdownConverter struct {
	config ConverterConfig
}

// ConverterConfig configures the Markdown converter behavior
type ConverterConfig struct {
	EnableTables      bool // Convert Markdown tables to TableBlock
	EnableCodeBlocks  bool // Use RichTextPreformatted for code blocks
	EnableQuotes      bool // Use RichTextQuote for blockquotes
	EnableLists       bool // Use RichTextList for lists
	MaxTableRows      int  // Maximum table rows (performance protection)
	MaxCodeBlockLines int  // Maximum code block lines
}

// NewMarkdownConverter creates a new MarkdownConverter with configuration
func NewMarkdownConverter(config ConverterConfig) *MarkdownConverter {
	if config.MaxTableRows <= 0 {
		config.MaxTableRows = 20
	}
	if config.MaxCodeBlockLines <= 0 {
		config.MaxCodeBlockLines = 100
	}
	return &MarkdownConverter{config: config}
}

// ConvertToBlocks converts Markdown text to Slack Blocks
// Detects and converts tables, code blocks, quotes, and lists
func (mc *MarkdownConverter) ConvertToBlocks(markdown string) []slack.Block {
	var blocks []slack.Block
	var pendingText strings.Builder

	lines := strings.Split(markdown, "\n")
	var inCodeBlock bool
	var codeBlockLines []string
	var i int

	for i < len(lines) {
		line := lines[i]

		// Detect code block start/end
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				// Code block start
				mc.flushText(&pendingText, &blocks)
				inCodeBlock = true
				codeBlockLines = []string{}
			} else {
				// Code block end
				inCodeBlock = false
				if mc.config.EnableCodeBlocks && len(codeBlockLines) <= mc.config.MaxCodeBlockLines {
					blocks = append(blocks, mc.buildCodeBlock(codeBlockLines))
				} else {
					// Fallback to plain text
					pendingText.WriteString("```\n")
					for _, l := range codeBlockLines {
						pendingText.WriteString(l + "\n")
					}
					pendingText.WriteString("```\n")
				}
				codeBlockLines = nil
			}
			i++
			continue
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			i++
			continue
		}

		// Detect table: look for separator line (|---|---|)
		if mc.config.EnableTables && mc.isTableSeparator(line) {
			// Parse table: header is previous line, rows are following lines
			if i > 0 {
				headerLine := lines[i-1]
				if strings.HasPrefix(strings.TrimSpace(headerLine), "|") {
					// Valid table: remove header from pending text
					// The header line was added in the previous iteration
					pendingTextStr := strings.TrimSuffix(pendingText.String(), headerLine+"\n")
					pendingText.Reset()
					pendingText.WriteString(pendingTextStr)
					mc.flushText(&pendingText, &blocks)

					// Parse header and rows
					header := parseTableRow(headerLine)
					var rows [][]string

					// Parse data rows (lines after separator)
					rowIdx := i + 1
					for rowIdx < len(lines) && rowIdx-i-1 < mc.config.MaxTableRows {
						rowLine := strings.TrimSpace(lines[rowIdx])
						if rowLine == "" || !strings.HasPrefix(rowLine, "|") {
							break
						}
						rows = append(rows, parseTableRow(rowLine))
						rowIdx++
					}

					// Convert to Slack TableBlock
					if len(header) > 0 && len(rows) > 0 {
						blocks = append(blocks, ConvertTableToSlackBlock(header, rows))
						i = rowIdx // Skip parsed table lines
						continue
					}
				}
			}
			// Not a valid table, flush pending text and treat separator as regular text
			mc.flushText(&pendingText, &blocks)
			pendingText.WriteString(line + "\n")
			i++
			continue
		}

		// Detect blockquote
		if mc.config.EnableQuotes && strings.HasPrefix(line, "> ") {
			mc.flushText(&pendingText, &blocks)
			quoteText := strings.TrimPrefix(line, "> ")
			blocks = append(blocks, mc.buildQuote(quoteText))
			i++
			continue
		}

		// Detect list
		if mc.config.EnableLists && mc.isListLine(line) {
			mc.flushText(&pendingText, &blocks)
			listBlocks := mc.parseList(lines, &i)
			blocks = append(blocks, listBlocks...)
			continue
		}

		// Regular text
		pendingText.WriteString(line + "\n")
		i++
	}

	// Flush remaining text
	mc.flushText(&pendingText, &blocks)

	// Handle code block not closed
	if inCodeBlock && len(codeBlockLines) > 0 {
		pendingText.WriteString("```\n")
		for _, l := range codeBlockLines {
			pendingText.WriteString(l + "\n")
		}
		pendingText.WriteString("```\n")
		mc.flushText(&pendingText, &blocks)
	}

	return blocks
}

// buildCodeBlock creates a RichTextBlock containing a preformatted code section
func (mc *MarkdownConverter) buildCodeBlock(lines []string) *slack.RichTextBlock {
	// Create a preformatted section (code block with border)
	section := slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(strings.Join(lines, "\n"), &slack.RichTextSectionTextStyle{
			Code: true,
		}),
	)
	// Wrap in RichTextBlock
	return slack.NewRichTextBlock("code_block", section)
}

// buildQuote creates a RichTextBlock containing a quote section
func (mc *MarkdownConverter) buildQuote(text string) *slack.RichTextBlock {
	// Create a quote section with italic style
	section := slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement(text, &slack.RichTextSectionTextStyle{
			Italic: true,
		}),
	)
	// Wrap in RichTextBlock
	return slack.NewRichTextBlock("quote", section)
}

// isTableSeparator checks if line is a Markdown table separator (|---|---|)
func (mc *MarkdownConverter) isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return false
	}
	// Check if it contains only |, -, and spaces
	for _, r := range trimmed {
		if r != '|' && r != '-' && r != ' ' && r != ':' {
			return false
		}
	}
	return strings.Count(trimmed, "-") >= 3 // At least 3 dashes to be a separator
}

// flushText flushes pending text as a regular section block
func (mc *MarkdownConverter) flushText(pendingText *strings.Builder, blocks *[]slack.Block) {
	if pendingText.Len() == 0 {
		return
	}

	text := strings.TrimSpace(pendingText.String())
	if text == "" {
		return
	}

	// Convert Markdown to mrkdwn
	mrkdwn := convertMarkdownToSlackMrkdwn(text)
	mrkdwnObj := slack.NewTextBlockObject("mrkdwn", mrkdwn, false, false)
	*blocks = append(*blocks, slack.NewSectionBlock(mrkdwnObj, nil, nil))

	pendingText.Reset()
}

// DetectMarkdownTable detects if text contains a Markdown table
// Returns table lines (header, separator, rows) if found
func DetectMarkdownTable(text string) (header []string, rows [][]string, found bool) {
	lines := strings.Split(text, "\n")
	var tableLines []string
	var inTable bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inTable {
				break // Table ended
			}
			continue
		}

		// Check if line looks like a table row
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			inTable = true
			tableLines = append(tableLines, trimmed)
		} else if inTable {
			break // Table ended
		}
	}

	if len(tableLines) < 2 {
		return nil, nil, false
	}

	// Parse header
	header = parseTableRow(tableLines[0])

	// Skip separator (line 1)
	// Parse rows
	for i := 2; i < len(tableLines); i++ {
		row := parseTableRow(tableLines[i])
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	return header, rows, true
}

// parseTableRow parses a Markdown table row into cells
func parseTableRow(line string) []string {
	// Remove leading and trailing |
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	// Split by |
	cells := strings.Split(line, "|")
	var result []string
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		result = append(result, trimmed)
	}
	return result
}

// ConvertTableToSlackBlock converts a Markdown table to Slack TableBlock
func ConvertTableToSlackBlock(header []string, rows [][]string) *slack.TableBlock {
	table := slack.NewTableBlock("markdown_table")

	// Add header row if present
	if len(header) > 0 {
		var headerCells []*slack.RichTextBlock
		for _, cell := range header {
			section := slack.NewRichTextSection(
				slack.NewRichTextSectionTextElement(cell, nil),
			)
			headerCells = append(headerCells, slack.NewRichTextBlock("header", section))
		}
		table.AddRow(headerCells...)
	}

	// Add data rows
	for _, row := range rows {
		var rowCells []*slack.RichTextBlock
		for _, cell := range row {
			section := slack.NewRichTextSection(
				slack.NewRichTextSectionTextElement(cell, nil),
			)
			rowCells = append(rowCells, slack.NewRichTextBlock("cell", section))
		}
		table.AddRow(rowCells...)
	}

	return table
}

// isListLine checks if a line is a list item (unordered or ordered)
func (mc *MarkdownConverter) isListLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "+ ") ||
		regexp.MustCompile(`^\d+\.\s`).MatchString(trimmed)
}

// parseList parses a list (consecutive list items) and returns RichTextBlocks
// Supports both unordered (-, *, +) and ordered (1., 2., etc.) lists
func (mc *MarkdownConverter) parseList(lines []string, idx *int) []slack.Block {
	var items []string
	var listType string // "bullet" or "ordered"

	// Parse consecutive list items
	for *idx < len(lines) {
		line := strings.TrimSpace(lines[*idx])
		if !mc.isListLine(line) {
			break
		}

		// Determine list type from first item
		if listType == "" {
			if regexp.MustCompile(`^\d+\.\s`).MatchString(line) {
				listType = "ordered"
			} else {
				listType = "bullet"
			}
		}

		// Extract item text (remove marker)
		var text string
		if listType == "ordered" {
			// Remove "N. " prefix
			re := regexp.MustCompile(`^\d+\.\s+(.*)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				text = matches[1]
			}
		} else {
			// Remove "- ", "* ", "+ " prefix
			if strings.HasPrefix(line, "- ") {
				text = strings.TrimPrefix(line, "- ")
			} else if strings.HasPrefix(line, "* ") {
				text = strings.TrimPrefix(line, "* ")
			} else if strings.HasPrefix(line, "+ ") {
				text = strings.TrimPrefix(line, "+ ")
			}
		}

		if text != "" {
			items = append(items, text)
		}
		(*idx)++
	}

	if len(items) == 0 {
		return nil
	}

	// Build RichTextList
	return mc.buildRichTextList(items, listType)
}

// buildRichTextList creates RichTextBlock with list formatting
// Uses Slack's native list rendering with bullet points or numbers
func (mc *MarkdownConverter) buildRichTextList(items []string, listType string) []slack.Block {
	var blocks []slack.Block

	for i, item := range items {
		var prefix string
		if listType == "ordered" {
			prefix = fmt.Sprintf("%d. ", i+1)
		} else {
			prefix = "• "
		}

		// Convert Markdown inline formatting to mrkdwn
		formattedItem := convertMarkdownToSlackMrkdwn(item)
		text := prefix + formattedItem

		mrkdwnObj := slack.NewTextBlockObject("mrkdwn", text, false, false)
		blocks = append(blocks, slack.NewSectionBlock(mrkdwnObj, nil, nil))
	}

	return blocks
}
