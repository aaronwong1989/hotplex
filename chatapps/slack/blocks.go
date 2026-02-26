package slack

import (
	"fmt"
	"unicode/utf8"
)

// =============================================================================
// Block Builders - Additional Block Types
// =============================================================================

// BuildImageBlock creates an image block
// Reference: https://api.slack.com/reference/block-kit/blocks#image
func BuildImageBlock(imageURL, altText, title string) []map[string]any {
	block := map[string]any{
		"type":      "image",
		"image_url": imageURL,
		"alt_text":  altText,
	}
	if title != "" {
		block["title"] = plainText(title)
	}
	return []map[string]any{block}
}

// BuildHeaderBlock creates a header block (text must be plain_text, max 150 chars)
// Reference: https://api.slack.com/reference/block-kit/blocks#header
func BuildHeaderBlock(text string) []map[string]any {
	truncated := text
	if len(truncated) > MaxPlainTextLen {
		truncated = truncated[:MaxPlainTextLen-3] + "..."
	}
	return []map[string]any{{
		"type": "header",
		"text": plainText(truncated),
	}}
}

// BuildDividerBlock creates a simple divider
// Reference: https://api.slack.com/reference/block-kit/blocks#divider
func BuildDividerBlock() []map[string]any {
	return []map[string]any{{
		"type": "divider",
	}}
}

// BuildFileBlock creates a file block (requires file to be uploaded first)
// Reference: https://api.slack.com/reference/block-kit/blocks#file
func BuildFileBlock(externalID string) []map[string]any {
	return []map[string]any{{
		"type":        "file",
		"external_id": externalID,
	}}
}

// BuildRichTextBlock creates a rich_text block for formatted content
// Reference: https://api.slack.com/reference/block-kit/blocks#rich_text
func BuildRichTextBlock(elements []map[string]any) []map[string]any {
	return []map[string]any{{
		"type":     "rich_text",
		"elements": elements,
	}}
}

// BuildRichTextSection creates a rich_text section element
func BuildRichTextSection(elements []map[string]any) map[string]any {
	return map[string]any{
		"type":     "rich_text_section",
		"elements": elements,
	}
}

// BuildRichTextSectionText creates text element for rich_text_section
func BuildRichTextSectionText(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
	}
}

// BuildRichTextSectionBold creates bold text element for rich_text_section
func BuildRichTextSectionBold(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
		"style": map[string]any{
			"bold": true,
		},
	}
}

// BuildRichTextSectionItalic creates italic text element for rich_text_section
func BuildRichTextSectionItalic(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
		"style": map[string]any{
			"italic": true,
		},
	}
}

// BuildRichTextSectionCode creates code text element for rich_text_section
func BuildRichTextSectionCode(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
		"style": map[string]any{
			"code": true,
		},
	}
}

// BuildRichTextSectionLink creates link text element for rich_text_section
func BuildRichTextSectionLink(text, url string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
		"style": map[string]any{
			"link": map[string]any{
				"url": url,
			},
		},
	}
}

// BuildInputBlock creates an input block for modals
// Reference: https://api.slack.com/reference/block-kit/blocks#input
func BuildInputBlock(label, placeholder string, element map[string]any, optional bool) map[string]any {
	block := map[string]any{
		"type":    "input",
		"label":   plainText(label),
		"element": element,
	}
	if placeholder != "" {
		element["placeholder"] = plainText(placeholder)
	}
	if optional {
		block["optional"] = true
	}
	return block
}

// BuildVideoBlock creates a video block (for apps with video permissions)
// Reference: https://api.slack.com/reference/block-kit/blocks#video
func BuildVideoBlock(videoURL, thumbnailURL, title, description string, authorName, providerName string) []map[string]any {
	block := map[string]any{
		"type":          "video",
		"video_url":     videoURL,
		"thumbnail_url": thumbnailURL,
		"alt_text":      "Video: " + title,
		"title":         plainText(title),
	}
	if description != "" {
		block["description"] = mrkdwnText(description)
	}
	if authorName != "" {
		block["author_name"] = authorName
	}
	if providerName != "" {
		block["provider_name"] = providerName
	}
	return []map[string]any{block}
}

// BuildSectionBlock creates a section block with text
// Reference: https://api.slack.com/reference/block-kit/blocks#section
func BuildSectionBlock(text string, fields []map[string]any, accessory map[string]any) []map[string]any {
	block := map[string]any{
		"type": "section",
	}
	if text != "" {
		block["text"] = mrkdwnText(text)
	}
	if len(fields) > 0 {
		if len(fields) > 10 {
			fields = fields[:10] // Max 10 fields
		}
		block["fields"] = fields
	}
	if accessory != nil {
		block["accessory"] = accessory
	}
	return []map[string]any{block}
}

// BuildSectionBlockWithFields creates a section block with only fields (no text)
func BuildSectionBlockWithFields(fields []map[string]any) []map[string]any {
	if len(fields) > 10 {
		fields = fields[:10]
	}
	return []map[string]any{{
		"type":   "section",
		"fields": fields,
	}}
}

// BuildContextBlock creates a context block with multiple elements
// Reference: https://api.slack.com/reference/block-kit/blocks#context
func BuildContextBlock(elements []map[string]any) []map[string]any {
	if len(elements) > 10 {
		elements = elements[:10] // Max 10 elements
	}
	return []map[string]any{{
		"type":     "context",
		"elements": elements,
	}}
}

// BuildActionsBlock creates an actions block with interactive elements
// Reference: https://api.slack.com/reference/block-kit/blocks#actions
func BuildActionsBlock(elements []map[string]any) []map[string]any {
	if len(elements) > 25 {
		elements = elements[:25] // Max 25 elements
	}
	return []map[string]any{{
		"type":     "actions",
		"elements": elements,
	}}
}

// BuildCallBlock creates a call block for huddles/meetings
// Reference: https://api.slack.com/reference/block-kit/blocks#call
func BuildCallBlock(callID string) []map[string]any {
	return []map[string]any{{
		"type":   "call",
		"call_id": callID,
	}}
}

// =============================================================================
// New Block Types (2025-2026 Additions)
// =============================================================================

// BuildMarkdownBlock creates a markdown block for rendering markdown content
// Reference: https://api.slack.com/reference/block-kit/blocks#markdown-block
func BuildMarkdownBlock(markdownText string) []map[string]any {
	// Slack markdown blocks support up to 12000 characters
	truncated := markdownText
	if utf8.RuneCountInString(truncated) > MaxMarkdownBlockLen {
		truncated = TruncateByRune(truncated, MaxMarkdownBlockLen-3) + "..."
	}
	return []map[string]any{{
		"type":     "markdown",
		"markdown": truncated,
	}}
}

// BuildPlanBlock creates a plan block for displaying structured plans
// Reference: https://api.slack.com/reference/block-kit/blocks#plan-block
func BuildPlanBlock(title string, sections []map[string]any) []map[string]any {
	// Truncate title to MaxPlainTextLen
	safeTitle := title
	if utf8.RuneCountInString(safeTitle) > MaxPlainTextLen {
		safeTitle = TruncateByRune(safeTitle, MaxPlainTextLen-3) + "..."
	}
	
	// Initialize sections if nil
	safeSections := sections
	if safeSections == nil {
		safeSections = []map[string]any{}
	}
	
	block := map[string]any{
		"type":     "plan",
		"title":    plainText(safeTitle),
		"sections": safeSections,
	}
	return []map[string]any{block}
}

// BuildPlanSection creates a section for plan block
func BuildPlanSection(sectionTitle string, items []map[string]any, status string) map[string]any {
	// Truncate title to MaxPlainTextLen
	safeTitle := sectionTitle
	if utf8.RuneCountInString(safeTitle) > MaxPlainTextLen {
		safeTitle = TruncateByRune(safeTitle, MaxPlainTextLen-3) + "..."
	}
	
	// Limit items to 50 per section
	safeItems := items
	if len(safeItems) > 50 {
		safeItems = safeItems[:50]
	}
	
	// Validate status if provided
	safeStatus := ""
	if status != "" {
		validStatuses := map[string]bool{
			"complete":    true,
			"in_progress": true,
			"not_started": true,
		}
		if validStatuses[status] {
			safeStatus = status
		}
	}
	
	section := map[string]any{
		"title": plainText(safeTitle),
		"items": safeItems,
	}
	if safeStatus != "" {
		section["status"] = safeStatus
	}
	return section
}

// BuildPlanItem creates an item for plan section
func BuildPlanItem(text string, itemType string) map[string]any {
	// Truncate text to MaxPlainTextLen
	safeText := text
	if utf8.RuneCountInString(safeText) > MaxPlainTextLen {
		safeText = TruncateByRune(safeText, MaxPlainTextLen-3) + "..."
	}
	
	// Validate item type
	safeType := itemType
	if itemType != "" {
		validTypes := map[string]bool{
			"task":    true,
			"note":    true,
			"warning": true,
		}
		if !validTypes[itemType] {
			safeType = "task" // Default to task
		}
	}
	
	return map[string]any{
		"text": plainText(safeText),
		"type": safeType,
	}
}

// BuildTableBlock creates a table block for displaying tabular data
// Reference: https://api.slack.com/reference/block-kit/blocks#table-block
func BuildTableBlock(headers []string, rows [][]string, columns int) []map[string]any {
	block := map[string]any{
		"type": "table",
	}
	
	// Build header row
	if len(headers) > 0 {
		headerCells := make([]map[string]any, len(headers))
		for i, h := range headers {
			headerCells[i] = map[string]any{
				"text": plainText(h),
				"type": "header",
			}
		}
		block["rows"] = []map[string]any{{"cells": headerCells}}
	}
	
	// Build data rows
	tableRows := make([]map[string]any, len(rows))
	for i, row := range rows {
		cells := make([]map[string]any, len(row))
		for j, cell := range row {
			cells[j] = map[string]any{
				"text": plainText(cell),
				"type": "cell",
			}
		}
		tableRows[i] = map[string]any{"cells": cells}
	}
	
	if len(rows) > 0 {
		if existingRows, ok := block["rows"].([]map[string]any); ok {
			block["rows"] = append(existingRows, tableRows...)
		} else {
			// No headers, initialize rows array
			block["rows"] = tableRows
		}
	}
	
	// Set column count
	if columns > 0 {
		block["columns"] = columns
	}
	
	return []map[string]any{block}
}

// BuildTaskCardBlock creates a task card block for task management
// Reference: https://api.slack.com/reference/block-kit/blocks#task-card-block
func BuildTaskCardBlock(title, description, assignee, dueDate, status string, actions []map[string]any) []map[string]any {
	// Validate and truncate title
	safeTitle := title
	if utf8.RuneCountInString(safeTitle) > MaxPlainTextLen {
		safeTitle = TruncateByRune(safeTitle, MaxPlainTextLen-3) + "..."
	}
	
	// Validate and truncate description
	safeDescription := description
	if utf8.RuneCountInString(safeDescription) > MaxSectionTextLen {
		safeDescription = TruncateByRune(safeDescription, MaxSectionTextLen-3) + "..."
	}
	
	// Validate status
	validStatuses := map[string]bool{
		"pending":     true,
		"in_progress": true,
		"completed":   true,
	}
	safeStatus := "pending" // Default
	if validStatuses[status] {
		safeStatus = status
	}
	
	// Limit actions to 25
	safeActions := actions
	if len(safeActions) > 25 {
		safeActions = safeActions[:25]
	}
	
	block := map[string]any{
		"type":        "task_card",
		"title":       plainText(safeTitle),
		"description": mrkdwnText(safeDescription),
		"status":      safeStatus,
	}
	
	if assignee != "" {
		block["assignee"] = assignee
	}
	if dueDate != "" {
		block["due_date"] = dueDate
	}
	if len(safeActions) > 0 {
		block["actions"] = safeActions
	}
	
	return []map[string]any{block}
}

// BuildContextActionsBlock creates a context actions block for quick actions
// Reference: https://api.slack.com/reference/block-kit/blocks#context-actions-block
func BuildContextActionsBlock(contextItems []map[string]any, actions []map[string]any) []map[string]any {
	return []map[string]any{{
		"type":    "context_actions",
		"context": contextItems,
		"actions": actions,
	}}
}

// BuildAdvancedLayout creates a complex layout using section/fields combinations
func BuildAdvancedLayout(title string, fields map[string]string, footer string) []map[string]any {
	var blocks []map[string]any
	
	// Add header if provided
	if title != "" {
		blocks = append(blocks, BuildHeaderBlock(title)...)
	}
	
	// Build fields for section
	if len(fields) > 0 {
		fieldBlocks := make([]map[string]any, 0, len(fields))
		for key, value := range fields {
			fieldBlocks = append(fieldBlocks, mrkdwnText(fmt.Sprintf("*%s*:\n%s", key, value)))
		}
		blocks = append(blocks, BuildSectionBlockWithFields(fieldBlocks)...)
	}
	
	// Add footer/context
	if footer != "" {
		blocks = append(blocks, BuildContextBlock([]map[string]any{mrkdwnText(footer)})...)
	}
	
	return blocks
}
