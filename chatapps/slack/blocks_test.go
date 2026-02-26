package slack

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildImageBlock(t *testing.T) {
	blocks := BuildImageBlock("https://example.com/img.png", "Alt text", "Title")
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "image" {
		t.Errorf("expected type 'image', got %v", block["type"])
	}
	if block["image_url"] != "https://example.com/img.png" {
		t.Errorf("expected image_url 'https://example.com/img.png', got %v", block["image_url"])
	}
	if block["alt_text"] != "Alt text" {
		t.Errorf("expected alt_text 'Alt text', got %v", block["alt_text"])
	}
	if block["title"].(map[string]any)["text"] != "Title" {
		t.Errorf("expected title 'Title', got %v", block["title"])
	}
}

func TestBuildHeaderBlock(t *testing.T) {
	blocks := BuildHeaderBlock("Test Header")
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "header" {
		t.Errorf("expected type 'header', got %v", block["type"])
	}
	text := block["text"].(map[string]any)
	if text["text"] != "Test Header" {
		t.Errorf("expected text 'Test Header', got %v", text["text"])
	}
}

func TestBuildHeaderBlock_Truncation(t *testing.T) {
	longText := string(make([]byte, 200))
	for i := range longText {
		longText = longText[:i] + "a" + longText[i+1:]
	}
	
	blocks := BuildHeaderBlock(longText)
	block := blocks[0]
	text := block["text"].(map[string]any)["text"].(string)
	
	if len(text) > MaxPlainTextLen {
		t.Errorf("expected text length <= %d, got %d", MaxPlainTextLen, len(text))
	}
}

func TestBuildDividerBlock(t *testing.T) {
	blocks := BuildDividerBlock()
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "divider" {
		t.Errorf("expected type 'divider', got %v", block["type"])
	}
}

func TestBuildFileBlock(t *testing.T) {
	blocks := BuildFileBlock("file123")
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "file" {
		t.Errorf("expected type 'file', got %v", block["type"])
	}
	if block["external_id"] != "file123" {
		t.Errorf("expected external_id 'file123', got %v", block["external_id"])
	}
}

func TestBuildRichTextBlock(t *testing.T) {
	elements := []map[string]any{
		BuildRichTextSectionText("Hello"),
		BuildRichTextSectionBold("World"),
	}
	
	blocks := BuildRichTextBlock(elements)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "rich_text" {
		t.Errorf("expected type 'rich_text', got %v", block["type"])
	}
	if len(block["elements"].([]map[string]any)) != 2 {
		t.Errorf("expected 2 elements, got %d", len(block["elements"].([]map[string]any)))
	}
}

func TestBuildSectionBlock(t *testing.T) {
	blocks := BuildSectionBlock("Hello *world*", nil, nil)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "section" {
		t.Errorf("expected type 'section', got %v", block["type"])
	}
	text := block["text"].(map[string]any)
	if text["text"] != "Hello *world*" {
		t.Errorf("expected text 'Hello *world*', got %v", text["text"])
	}
}

func TestBuildSectionBlock_WithFields(t *testing.T) {
	fields := []map[string]any{
		mrkdwnText("*Field 1*"),
		mrkdwnText("*Field 2*"),
	}
	
	blocks := BuildSectionBlockWithFields(fields)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if len(block["fields"].([]map[string]any)) != 2 {
		t.Errorf("expected 2 fields, got %d", len(block["fields"].([]map[string]any)))
	}
}

func TestBuildContextBlock(t *testing.T) {
	elements := []map[string]any{
		mrkdwnText("Context 1"),
		mrkdwnText("Context 2"),
	}
	
	blocks := BuildContextBlock(elements)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "context" {
		t.Errorf("expected type 'context', got %v", block["type"])
	}
}

func TestBuildActionsBlock(t *testing.T) {
	elements := []map[string]any{
		BuildButton("Click", "click_action", "value", ""),
	}
	
	blocks := BuildActionsBlock(elements)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "actions" {
		t.Errorf("expected type 'actions', got %v", block["type"])
	}
}

func TestBuildButton(t *testing.T) {
	button := BuildButton("Click Me", "click_action", "value", "primary")
	
	if button["type"] != "button" {
		t.Errorf("expected type 'button', got %v", button["type"])
	}
	if button["action_id"] != "click_action" {
		t.Errorf("expected action_id 'click_action', got %v", button["action_id"])
	}
	if button["value"] != "value" {
		t.Errorf("expected value 'value', got %v", button["value"])
	}
	if button["style"] != "primary" {
		t.Errorf("expected style 'primary', got %v", button["style"])
	}
}

func TestBuildButtonWithURL(t *testing.T) {
	button := BuildButtonWithURL("Open", "open_action", "https://example.com")
	
	if button["url"] != "https://example.com" {
		t.Errorf("expected url 'https://example.com', got %v", button["url"])
	}
}

func TestBuildOption(t *testing.T) {
	option := BuildOption("Label", "value")
	
	if option["text"].(map[string]any)["text"] != "Label" {
		t.Errorf("expected text 'Label', got %v", option["text"])
	}
	if option["value"] != "value" {
		t.Errorf("expected value 'value', got %v", option["value"])
	}
}

func TestBuildOptionGroup(t *testing.T) {
	options := []map[string]any{
		BuildOption("Option 1", "opt1"),
		BuildOption("Option 2", "opt2"),
	}
	
	group := BuildOptionGroup("Group Label", options)
	
	if group["label"].(map[string]any)["text"] != "Group Label" {
		t.Errorf("expected label 'Group Label', got %v", group["label"])
	}
	if len(group["options"].([]map[string]any)) != 2 {
		t.Errorf("expected 2 options, got %d", len(group["options"].([]map[string]any)))
	}
}

func TestBuildConfirmationDialog(t *testing.T) {
	confirm := BuildConfirmationDialog("Title", "Text", "Confirm", "Deny")
	
	if confirm["title"].(map[string]any)["text"] != "Title" {
		t.Errorf("expected title 'Title', got %v", confirm["title"])
	}
	if confirm["text"].(map[string]any)["text"] != "Text" {
		t.Errorf("expected text 'Text', got %v", confirm["text"])
	}
	if confirm["confirm"].(map[string]any)["text"] != "Confirm" {
		t.Errorf("expected confirm 'Confirm', got %v", confirm["confirm"])
	}
	if confirm["deny"].(map[string]any)["text"] != "Deny" {
		t.Errorf("expected deny 'Deny', got %v", confirm["deny"])
	}
}

func TestTruncateMrkdwn(t *testing.T) {
	// Test basic truncation
	short := "Hello"
	result := TruncateMrkdwn(short, 100)
	if result != short {
		t.Errorf("expected %q, got %q", short, result)
	}
	
	// Test truncation with ellipsis
	long := string(make([]byte, 100))
	for i := range long {
		long = long[:i] + "a" + long[i+1:]
	}
	result = TruncateMrkdwn(long, 50)
	if len(result) > 53 { // 50 + "..."
		t.Errorf("expected length <= 53, got %d", len(result))
	}
	if result[len(result)-3:] != "..." {
		t.Errorf("expected to end with '...', got %q", result[len(result)-3:])
	}
}

func TestTruncateMrkdwn_CodeBlock(t *testing.T) {
	text := "Here is code: ```some code here``` and more text"
	result := TruncateMrkdwn(text, 20)
	
	// Should not cut inside code block
	if len(result) > 23 {
		t.Errorf("expected length <= 23, got %d", len(result))
	}
}

func TestValidateBlocks(t *testing.T) {
	// Test valid blocks
	validBlocks := []map[string]any{
		{"type": "section", "text": mrkdwnText("Hello")},
	}
	
	err := ValidateBlocks(validBlocks, false)
	if err != nil {
		t.Errorf("expected no error for valid blocks, got %v", err)
	}
}

func TestValidateBlocks_MaxBlocks(t *testing.T) {
	// Test too many blocks
	blocks := make([]map[string]any, MaxBlocksLen+1)
	for i := range blocks {
		blocks[i] = map[string]any{"type": "section", "text": mrkdwnText("test")}
	}
	
	err := ValidateBlocks(blocks, false)
	if err == nil {
		t.Error("expected error for too many blocks")
	}
}

func TestValidateSectionBlock(t *testing.T) {
	// Test missing text and fields
	block := map[string]any{"type": "section"}
	err := ValidateBlock(block, 0)
	if err == nil {
		t.Error("expected error for section without text or fields")
	}
}

func TestValidateActionsBlock(t *testing.T) {
	// Test too many elements
	elements := make([]map[string]any, 26)
	for i := range elements {
		elements[i] = BuildButton("Btn", "action", "val", "")
	}
	
	block := map[string]any{
		"type":     "actions",
		"elements": elements,
	}
	
	err := ValidateBlock(block, 0)
	if err == nil {
		t.Error("expected error for actions with > 25 elements")
	}
}

func TestBuildDatePicker(t *testing.T) {
	picker := BuildDatePicker("Select date", "date_action", "2024-01-01")
	
	if picker["type"] != "datepicker" {
		t.Errorf("expected type 'datepicker', got %v", picker["type"])
	}
	if picker["initial_date"] != "2024-01-01" {
		t.Errorf("expected initial_date '2024-01-01', got %v", picker["initial_date"])
	}
}

func TestBuildPlainTextInput(t *testing.T) {
	input := BuildPlainTextInput("Enter text", "input_action", "default", true, 100, 10)
	
	if input["type"] != "plain_text_input" {
		t.Errorf("expected type 'plain_text_input', got %v", input["type"])
	}
	if input["multiline"] != true {
		t.Errorf("expected multiline true, got %v", input["multiline"])
	}
	if input["max_length"] != 100 {
		t.Errorf("expected max_length 100, got %v", input["max_length"])
	}
}

func TestBuildRadioButtons(t *testing.T) {
	options := []map[string]any{
		BuildOption("Option 1", "opt1"),
		BuildOption("Option 2", "opt2"),
	}
	
	radio := BuildRadioButtons(options, "radio_action", nil)
	
	if radio["type"] != "radio_buttons" {
		t.Errorf("expected type 'radio_buttons', got %v", radio["type"])
	}
	if len(radio["options"].([]map[string]any)) != 2 {
		t.Errorf("expected 2 options, got %d", len(radio["options"].([]map[string]any)))
	}
}

func TestBuildCheckboxes(t *testing.T) {
	options := []map[string]any{
		BuildOption("Check 1", "chk1"),
		BuildOption("Check 2", "chk2"),
	}
	
	checkboxes := BuildCheckboxes(options, "checkbox_action", nil)
	
	if checkboxes["type"] != "checkboxes" {
		t.Errorf("expected type 'checkboxes', got %v", checkboxes["type"])
	}
}

func TestBuildStaticSelect(t *testing.T) {
	options := []map[string]any{
		BuildOption("Option 1", "opt1"),
		BuildOption("Option 2", "opt2"),
	}
	
	selectMenu := BuildStaticSelect(options, "Select...", "select_action", nil)
	
	if selectMenu["type"] != "static_select" {
		t.Errorf("expected type 'static_select', got %v", selectMenu["type"])
	}
	if len(selectMenu["options"].([]map[string]any)) != 2 {
		t.Errorf("expected 2 options, got %d", len(selectMenu["options"].([]map[string]any)))
	}
}

func TestBuildMarkdownBlock(t *testing.T) {
	blocks := BuildMarkdownBlock("# Hello **World**")
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "markdown" {
		t.Errorf("expected type 'markdown', got %v", block["type"])
	}
}

func TestBuildPlanBlock(t *testing.T) {
	items := []map[string]any{
		BuildPlanItem("Task 1", "task"),
	}
	sections := []map[string]any{
		BuildPlanSection("Phase 1", items, "in_progress"),
	}
	
	blocks := BuildPlanBlock("Project Plan", sections)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "plan" {
		t.Errorf("expected type 'plan', got %v", block["type"])
	}
}

func TestBuildTableBlock(t *testing.T) {
	headers := []string{"Name", "Age", "City"}
	rows := [][]string{
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}
	
	blocks := BuildTableBlock(headers, rows, 3)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "table" {
		t.Errorf("expected type 'table', got %v", block["type"])
	}
}

func TestBuildTaskCardBlock(t *testing.T) {
	actions := []map[string]any{
		BuildButton("Complete", "complete", "task1", "primary"),
	}
	
	blocks := BuildTaskCardBlock("Task Title", "Description", "U123", "2024-12-31", "pending", actions)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if block["type"] != "task_card" {
		t.Errorf("expected type 'task_card', got %v", block["type"])
	}
}

func TestBuildFileInput(t *testing.T) {
	element := BuildFileInput("file_action", []string{"pdf", "doc"}, 5)
	
	if element["type"] != "file_input" {
		t.Errorf("expected type 'file_input', got %v", element["type"])
	}
	if element["max_files"] != 5 {
		t.Errorf("expected max_files 5, got %v", element["max_files"])
	}
}

func TestBuildFeedbackButtons(t *testing.T) {
	element := BuildFeedbackButtons("thumbs_up", "thumbs_down")
	
	if element["type"] != "feedback_buttons" {
		t.Errorf("expected type 'feedback_buttons', got %v", element["type"])
	}
}

func TestBuildRichTextInput(t *testing.T) {
	element := BuildRichTextInput("rich_action", "Enter text", "initial", true)
	
	if element["type"] != "rich_text_input" {
		t.Errorf("expected type 'rich_text_input', got %v", element["type"])
	}
	if element["focus_on_load"] != true {
		t.Errorf("expected focus_on_load true, got %v", element["focus_on_load"])
	}
}

func TestBuildExternalSelectWithMinQuery(t *testing.T) {
	element := BuildExternalSelectWithMinQuery("Select...", "ext_action", 3, nil)
	
	if element["type"] != "external_select" {
		t.Errorf("expected type 'external_select', got %v", element["type"])
	}
	if element["min_query_length"] != 3 {
		t.Errorf("expected min_query_length 3, got %v", element["min_query_length"])
	}
}

func TestBuildAccessibilityLabel(t *testing.T) {
	label := BuildAccessibilityLabel("Click to open")
	
	if label["accessibility_label"] != "Click to open" {
		t.Errorf("expected accessibility_label 'Click to open', got %v", label["accessibility_label"])
	}
}

func TestValidateButtonURLLength(t *testing.T) {
	// Valid URL
	shortURL := "https://example.com/short"
	if err := ValidateButtonURLLength(shortURL); err != nil {
		t.Errorf("expected no error for short URL, got %v", err)
	}
	
	// Too long URL
	longURL := string(make([]byte, 3001))
	for i := range longURL {
		longURL = longURL[:i] + "a" + longURL[i+1:]
	}
	if err := ValidateButtonURLLength(longURL); err == nil {
		t.Error("expected error for long URL")
	}
}

func TestValidateAccessibilityLabel(t *testing.T) {
	// Valid label
	shortLabel := "Click me"
	if err := ValidateAccessibilityLabel(shortLabel); err != nil {
		t.Errorf("expected no error for short label, got %v", err)
	}
	
	// Too long label
	longLabel := string(make([]byte, 76))
	for i := range longLabel {
		longLabel = longLabel[:i] + "a" + longLabel[i+1:]
	}
	if err := ValidateAccessibilityLabel(longLabel); err == nil {
		t.Error("expected error for long label")
	}
}

func TestValidateTableBlock(t *testing.T) {
	// Valid table
	validTable := map[string]any{
		"rows":    []map[string]any{{"cells": []map[string]any{}}},
		"columns": 3,
	}
	if err := ValidateTableBlock(validTable); err != nil {
		t.Errorf("expected no error for valid table, got %v", err)
	}
	
	// Too many rows
	tooManyRows := make([]map[string]any, 1001)
	invalidTable := map[string]any{"rows": tooManyRows}
	if err := ValidateTableBlock(invalidTable); err == nil {
		t.Error("expected error for too many rows")
	}
}

func TestValidatePlanBlock(t *testing.T) {
	// Valid plan
	validPlan := map[string]any{
		"sections": []map[string]any{{}},
	}
	if err := ValidatePlanBlock(validPlan); err != nil {
		t.Errorf("expected no error for valid plan, got %v", err)
	}
	
	// Too many sections
	tooManySections := make([]map[string]any, 26)
	invalidPlan := map[string]any{"sections": tooManySections}
	if err := ValidatePlanBlock(invalidPlan); err == nil {
		t.Error("expected error for too many sections")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestBuildTableBlock_EmptyHeaders(t *testing.T) {
	// Test case: No headers, only rows - should not panic
	headers := []string{}
	rows := [][]string{{"Data1", "Data2"}, {"Data3", "Data4"}}
	
	blocks := BuildTableBlock(headers, rows, 2)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	rowsData, ok := block["rows"].([]map[string]any)
	if !ok {
		t.Fatal("expected rows to be []map[string]any")
	}
	
	// Should have 2 data rows (no header row)
	if len(rowsData) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rowsData))
	}
}

func TestBuildTableBlock_EmptyRows(t *testing.T) {
	// Test case: Headers only, no data rows
	headers := []string{"Col1", "Col2"}
	rows := [][]string{}
	
	blocks := BuildTableBlock(headers, rows, 2)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	rowsData, ok := block["rows"].([]map[string]any)
	if !ok {
		t.Fatal("expected rows to be []map[string]any")
	}
	
	// Should have 1 header row only
	if len(rowsData) != 1 {
		t.Errorf("expected 1 row, got %d", len(rowsData))
	}
}

func TestBuildTableBlock_EmptyAll(t *testing.T) {
	// Test case: Both headers and rows empty
	headers := []string{}
	rows := [][]string{}
	
	blocks := BuildTableBlock(headers, rows, 0)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	if _, ok := block["rows"]; ok {
		t.Error("expected no rows field when both headers and rows are empty")
	}
}

func TestBuildMarkdownBlock_Truncation(t *testing.T) {
	// Test case: Markdown text exceeding limit
	longText := string(make([]byte, 12001))
	for i := range longText {
		longText = longText[:i] + "a" + longText[i+1:]
	}
	
	blocks := BuildMarkdownBlock(longText)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	markdown := block["markdown"].(string)
	
	// Should be truncated with ellipsis
	if len(markdown) > 12000 {
		t.Errorf("expected markdown length <= 12000, got %d", len(markdown))
	}
	if !strings.HasSuffix(markdown, "...") {
		t.Error("expected truncated markdown to end with '...'")
	}
}

func TestBuildMarkdownBlock_Empty(t *testing.T) {
	// Test case: Empty markdown
	blocks := BuildMarkdownBlock("")
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	markdown := block["markdown"].(string)
	if markdown != "" {
		t.Errorf("expected empty markdown, got %q", markdown)
	}
}

func TestBuildPlanBlock_NilSections(t *testing.T) {
	// Test case: Nil sections should not panic
	blocks := BuildPlanBlock("Test Plan", nil)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	sections, ok := block["sections"].([]map[string]any)
	if !ok {
		t.Fatal("expected sections to be []map[string]any")
	}
	
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestBuildPlanBlock_LongTitle(t *testing.T) {
	// Test case: Title exceeding MaxPlainTextLen
	longTitle := string(make([]byte, 200))
	for i := range longTitle {
		longTitle = longTitle[:i] + "a" + longTitle[i+1:]
	}
	
	blocks := BuildPlanBlock(longTitle, []map[string]any{})
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	titleObj, ok := block["title"].(map[string]any)
	if !ok {
		t.Fatal("expected title to be map[string]any")
	}
	
	title := titleObj["text"].(string)
	if utf8.RuneCountInString(title) > MaxPlainTextLen {
		t.Errorf("expected title length <= %d, got %d", MaxPlainTextLen, utf8.RuneCountInString(title))
	}
}

func TestBuildPlanSection_StatusValidation(t *testing.T) {
	tests := []struct {
		name   string
		status string
		valid  bool
	}{
		{"valid complete", "complete", true},
		{"valid in_progress", "in_progress", true},
		{"valid not_started", "not_started", true},
		{"invalid status", "unknown", false},
		{"empty status", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := BuildPlanSection("Test", []map[string]any{}, tt.status)
			
			if tt.valid {
				if section["status"] != tt.status {
					t.Errorf("expected status %q, got %q", tt.status, section["status"])
				}
			} else if tt.status != "" {
				// Invalid status should be omitted
				if _, exists := section["status"]; exists {
					t.Errorf("expected no status field for invalid status %q", tt.status)
				}
			}
		})
	}
}

func TestBuildPlanItem_TypeValidation(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		expected string
	}{
		{"valid task", "task", "task"},
		{"valid note", "note", "note"},
		{"valid warning", "warning", "warning"},
		{"invalid type", "unknown", "task"}, // Defaults to task
		{"empty type", "", "task"},          // Defaults to task
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := BuildPlanItem("Test Text", tt.itemType)
			
			if item["type"] != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, item["type"])
			}
		})
	}
}

func TestBuildTaskCardBlock_StatusValidation(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{"valid pending", "pending", "pending"},
		{"valid in_progress", "in_progress", "in_progress"},
		{"valid completed", "completed", "completed"},
		{"invalid status", "unknown", "pending"}, // Defaults to pending
		{"empty status", "", "pending"},          // Defaults to pending
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := BuildTaskCardBlock("Title", "Description", "", "", tt.status, nil)
			
			if len(blocks) != 1 {
				t.Fatalf("expected 1 block, got %d", len(blocks))
			}
			
			block := blocks[0]
			if block["status"] != tt.expected {
				t.Errorf("expected status %q, got %q", tt.expected, block["status"])
			}
		})
	}
}

func TestBuildTaskCardBlock_TooManyActions(t *testing.T) {
	// Create 30 actions (exceeds limit of 25)
	actions := make([]map[string]any, 30)
	for i := 0; i < 30; i++ {
		actions[i] = BuildButton(fmt.Sprintf("Btn%d", i), fmt.Sprintf("action_%d", i), "val", "")
	}
	
	blocks := BuildTaskCardBlock("Title", "Description", "", "", "pending", actions)
	
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	block := blocks[0]
	blockActions, ok := block["actions"].([]map[string]any)
	if !ok {
		t.Fatal("expected actions to be []map[string]any")
	}
	
	// Should be limited to 25
	if len(blockActions) > 25 {
		t.Errorf("expected max 25 actions, got %d", len(blockActions))
	}
}

func TestBuildPlanSection_TooManyItems(t *testing.T) {
	// Create 60 items (exceeds limit of 50)
	items := make([]map[string]any, 60)
	for i := 0; i < 60; i++ {
		items[i] = BuildPlanItem(fmt.Sprintf("Item%d", i), "task")
	}
	
	section := BuildPlanSection("Test Section", items, "")
	
	itemsResult, ok := section["items"].([]map[string]any)
	if !ok {
		t.Fatal("expected items to be []map[string]any")
	}
	
	// Should be limited to 50
	if len(itemsResult) > 50 {
		t.Errorf("expected max 50 items, got %d", len(itemsResult))
	}
}

func TestBuildURLSource_SafeURL(t *testing.T) {
	// Test with safe HTTPS URL
	blocks, err := BuildURLSource("action_id", "https://example.com/data")
	
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if blocks["type"] != "url_source" {
		t.Errorf("expected type 'url_source', got %v", blocks["type"])
	}
}

func TestBuildURLSource_DangerousScheme(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"javascript", "javascript:alert(1)"},
		{"data", "data:text/html,<script>alert(1)</script>"},
		{"file", "file:///etc/passwd"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildURLSource("action_id", tt.url)
			
			if err == nil {
				t.Errorf("expected error for dangerous URL scheme %s", tt.name)
			}
			if !strings.Contains(err.Error(), "dangerous") {
				t.Errorf("expected 'dangerous' in error, got %q", err.Error())
			}
		})
	}
}


func TestValidateTableBlock_ZeroRows(t *testing.T) {
	block := map[string]any{
		"type":    "table",
		"rows":    []map[string]any{},
		"columns": 3,
	}
	
	err := ValidateTableBlock(block)
	
	if err == nil {
		t.Error("expected error for zero rows")
	}
	if !strings.Contains(err.Error(), "must have at least 1 row") {
		t.Errorf("expected 'must have at least 1 row' in error, got %q", err.Error())
	}
}

func TestValidateTaskCardBlock_MissingStatus(t *testing.T) {
	block := map[string]any{
		"type": "task_card",
		"title": map[string]any{"text": "Test", "type": "plain_text"},
	}
	
	err := ValidateTaskCardBlock(block)
	
	if err == nil {
		t.Error("expected error for missing status")
	}
	if !strings.Contains(err.Error(), "status is required") {
		t.Errorf("expected 'status is required' in error, got %q", err.Error())
	}
}
