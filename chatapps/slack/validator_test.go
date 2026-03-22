package slack

import (
	"strings"
	"testing"
)

// --- ValidationError tests ---

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		BlockType: "section",
		Field:     "text",
		Message:   "exceeds 3000 characters",
	}
	s := err.Error()
	if !strings.Contains(s, "section") || !strings.Contains(s, "text") {
		t.Errorf("error string should contain block type and field: %s", s)
	}
}

// --- ValidateBlocks tests ---

func TestValidateBlocks_Empty(t *testing.T) {
	err := ValidateBlocks([]map[string]any{}, false)
	if err != nil {
		t.Errorf("empty blocks should be valid: %v", err)
	}
}

func TestValidateBlocks_ExceedsMax(t *testing.T) {
	blocks := make([]map[string]any, 51)
	for i := range blocks {
		blocks[i] = map[string]any{"type": "divider"}
	}
	err := ValidateBlocks(blocks, false)
	if err == nil {
		t.Error("should fail with 51 blocks")
	}

	// Modal allows 100
	err = ValidateBlocks(blocks, true)
	if err != nil {
		t.Errorf("modal should allow 51 blocks: %v", err)
	}
}

func TestValidateBlocks_ModalExceedsMax(t *testing.T) {
	blocks := make([]map[string]any, 101)
	for i := range blocks {
		blocks[i] = map[string]any{"type": "divider"}
	}
	err := ValidateBlocks(blocks, true)
	if err == nil {
		t.Error("should fail with 101 blocks in modal")
	}
}

// --- ValidateBlock tests ---

func TestValidateBlock_MissingType(t *testing.T) {
	err := ValidateBlock(map[string]any{}, 0)
	if err == nil {
		t.Error("should fail for missing type")
	}
}

func TestValidateBlock_UnknownType(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "unknown"}, 0)
	if err == nil {
		t.Error("should fail for unknown type")
	}
}

func TestValidateBlock_BlockIDTooLong(t *testing.T) {
	longID := strings.Repeat("a", 256)
	err := ValidateBlock(map[string]any{"type": "divider", "block_id": longID}, 0)
	if err == nil {
		t.Error("should fail for block_id > 255 chars")
	}
}

func TestValidateBlock_Divider(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "divider"}, 0)
	if err != nil {
		t.Errorf("divider should be valid: %v", err)
	}
}

func TestValidateBlock_Markdown(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "markdown", "markdown": "text"}, 0)
	if err != nil {
		t.Errorf("markdown should be valid: %v", err)
	}
}

func TestValidateBlock_ContextActions(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "context_actions"}, 0)
	if err != nil {
		t.Errorf("context_actions should be valid: %v", err)
	}
}

// --- Section block tests ---

func TestValidateBlock_Section_WithText(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type": "section",
		"text": map[string]any{"type": "mrkdwn", "text": "hello"},
	}, 0)
	if err != nil {
		t.Errorf("section with text should be valid: %v", err)
	}
}

func TestValidateBlock_Section_NoTextNoFields(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "section"}, 0)
	if err == nil {
		t.Error("section without text or fields should fail")
	}
}

func TestValidateBlock_Section_TextTooLong(t *testing.T) {
	longText := strings.Repeat("a", 3001)
	err := ValidateBlock(map[string]any{
		"type": "section",
		"text": map[string]any{"text": longText},
	}, 0)
	if err == nil {
		t.Error("section text > 3000 should fail")
	}
}

func TestValidateBlock_Section_FieldsTooMany(t *testing.T) {
	fields := make([]any, 11)
	for i := range fields {
		fields[i] = map[string]any{"text": "field"}
	}
	err := ValidateBlock(map[string]any{
		"type":   "section",
		"fields": fields,
	}, 0)
	if err == nil {
		t.Error("section with > 10 fields should fail")
	}
}

func TestValidateBlock_Section_FieldTextTooLong(t *testing.T) {
	fields := []any{
		map[string]any{"text": strings.Repeat("a", 2001)},
	}
	err := ValidateBlock(map[string]any{
		"type":   "section",
		"fields": fields,
	}, 0)
	if err == nil {
		t.Error("section field text > 2000 should fail")
	}
}

func TestValidateBlock_Section_WithAccessory(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":      "section",
		"text":      map[string]any{"text": "hello"},
		"accessory": map[string]any{"type": "button", "text": map[string]any{"text": "ok"}},
	}, 0)
	if err != nil {
		t.Errorf("section with button accessory should be valid: %v", err)
	}
}

func TestValidateBlock_Section_WithFields(t *testing.T) {
	fields := []any{
		map[string]any{"text": "field1"},
		map[string]any{"text": "field2"},
	}
	err := ValidateBlock(map[string]any{
		"type":   "section",
		"fields": fields,
	}, 0)
	if err != nil {
		t.Errorf("section with fields should be valid: %v", err)
	}
}

// --- Header block tests ---

func TestValidateBlock_Header_Valid(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type": "header",
		"text": map[string]any{"text": "Hello"},
	}, 0)
	if err != nil {
		t.Errorf("header should be valid: %v", err)
	}
}

func TestValidateBlock_Header_MissingText(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "header"}, 0)
	if err == nil {
		t.Error("header without text should fail")
	}
}

func TestValidateBlock_Header_TextTooLong(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type": "header",
		"text": map[string]any{"text": strings.Repeat("a", 151)},
	}, 0)
	if err == nil {
		t.Error("header text > 150 runes should fail")
	}
}

// --- Context block tests ---

func TestValidateBlock_Context_Valid(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":     "context",
		"elements": []any{map[string]any{"type": "mrkdwn", "text": "info"}},
	}, 0)
	if err != nil {
		t.Errorf("context should be valid: %v", err)
	}
}

func TestValidateBlock_Context_MissingElements(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "context"}, 0)
	if err == nil {
		t.Error("context without elements should fail")
	}
}

func TestValidateBlock_Context_TooManyElements(t *testing.T) {
	elems := make([]any, 11)
	for i := range elems {
		elems[i] = map[string]any{"type": "mrkdwn", "text": "info"}
	}
	err := ValidateBlock(map[string]any{
		"type":     "context",
		"elements": elems,
	}, 0)
	if err == nil {
		t.Error("context with > 10 elements should fail")
	}
}

// --- Actions block tests ---

func TestValidateBlock_Actions_Valid(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type": "actions",
		"elements": []any{
			map[string]any{"type": "button", "text": map[string]any{"text": "ok"}},
		},
	}, 0)
	if err != nil {
		t.Errorf("actions should be valid: %v", err)
	}
}

func TestValidateBlock_Actions_MissingElements(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "actions"}, 0)
	if err == nil {
		t.Error("actions without elements should fail")
	}
}

func TestValidateBlock_Actions_TooMany(t *testing.T) {
	elems := make([]any, 26)
	for i := range elems {
		elems[i] = map[string]any{"type": "button", "text": map[string]any{"text": "ok"}}
	}
	err := ValidateBlock(map[string]any{
		"type":     "actions",
		"elements": elems,
	}, 0)
	if err == nil {
		t.Error("actions with > 25 elements should fail")
	}
}

// --- Image block tests ---

func TestValidateBlock_Image_Valid(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":      "image",
		"image_url": "https://example.com/img.png",
		"alt_text":  "image",
	}, 0)
	if err != nil {
		t.Errorf("image should be valid: %v", err)
	}
}

func TestValidateBlock_Image_MissingURL(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "image", "alt_text": "img"}, 0)
	if err == nil {
		t.Error("image without image_url should fail")
	}
}

func TestValidateBlock_Image_MissingAlt(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "image", "image_url": "https://example.com"}, 0)
	if err == nil {
		t.Error("image without alt_text should fail")
	}
}

// --- File block tests ---

func TestValidateBlock_File_Valid(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":        "file",
		"external_id": "F123",
	}, 0)
	if err != nil {
		t.Errorf("file should be valid: %v", err)
	}
}

func TestValidateBlock_File_MissingExternalID(t *testing.T) {
	err := ValidateBlock(map[string]any{"type": "file"}, 0)
	if err == nil {
		t.Error("file without external_id should fail")
	}
}

// --- Input block tests ---

func TestValidateBlock_Input_Valid(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":    "input",
		"label":   map[string]any{"type": "plain_text", "text": "Label"},
		"element": map[string]any{"type": "plain_text_input"},
	}, 0)
	if err != nil {
		t.Errorf("input should be valid: %v", err)
	}
}

func TestValidateBlock_Input_MissingLabel(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":    "input",
		"element": map[string]any{"type": "plain_text_input"},
	}, 0)
	if err == nil {
		t.Error("input without label should fail")
	}
}

func TestValidateBlock_Input_MissingElement(t *testing.T) {
	err := ValidateBlock(map[string]any{
		"type":  "input",
		"label": map[string]any{"type": "plain_text", "text": "Label"},
	}, 0)
	if err == nil {
		t.Error("input without element should fail")
	}
}

// --- Interaction element tests ---

func TestValidateInteractionElement_MissingType(t *testing.T) {
	err := validateInteractionElement(map[string]any{})
	if err == nil {
		t.Error("element without type should fail")
	}
}

func TestValidateInteractionElement_ActionIDTooLong(t *testing.T) {
	longID := strings.Repeat("a", 256)
	err := validateInteractionElement(map[string]any{
		"type":      "button",
		"action_id": longID,
		"text":      map[string]any{"text": "ok"},
	})
	if err == nil {
		t.Error("action_id > 255 should fail")
	}
}

func TestValidateInteractionElement_ButtonValid(t *testing.T) {
	err := validateInteractionElement(map[string]any{
		"type": "button",
		"text": map[string]any{"text": "click me"},
	})
	if err != nil {
		t.Errorf("button should be valid: %v", err)
	}
}

func TestValidateInteractionElement_ButtonTextTooLong(t *testing.T) {
	err := validateInteractionElement(map[string]any{
		"type": "button",
		"text": map[string]any{"text": strings.Repeat("a", 76)},
	})
	if err == nil {
		t.Error("button text > 75 runes should fail")
	}
}

func TestValidateInteractionElement_ButtonMissingText(t *testing.T) {
	err := validateInteractionElement(map[string]any{"type": "button"})
	if err == nil {
		t.Error("button without text should fail")
	}
}

func TestValidateInteractionElement_SelectPlaceholderTooLong(t *testing.T) {
	err := validateInteractionElement(map[string]any{
		"type":        "static_select",
		"placeholder": map[string]any{"text": strings.Repeat("a", 151)},
	})
	if err == nil {
		t.Error("select placeholder > 150 runes should fail")
	}
}

func TestValidateInteractionElement_Datepicker(t *testing.T) {
	err := validateInteractionElement(map[string]any{"type": "datepicker"})
	if err != nil {
		t.Errorf("datepicker should be valid: %v", err)
	}
}

func TestValidateInteractionElement_PlainTextInput(t *testing.T) {
	err := validateInteractionElement(map[string]any{"type": "plain_text_input"})
	if err != nil {
		t.Errorf("plain_text_input should be valid: %v", err)
	}
}

func TestValidateInteractionElement_RadioButtonsValid(t *testing.T) {
	err := validateInteractionElement(map[string]any{
		"type": "radio_buttons",
		"options": []any{
			map[string]any{"text": map[string]any{"text": "opt1"}},
		},
	})
	if err != nil {
		t.Errorf("radio_buttons should be valid: %v", err)
	}
}

func TestValidateInteractionElement_RadioButtonsTooMany(t *testing.T) {
	opts := make([]any, 11)
	for i := range opts {
		opts[i] = map[string]any{"text": map[string]any{"text": "opt"}}
	}
	err := validateInteractionElement(map[string]any{
		"type":    "radio_buttons",
		"options": opts,
	})
	if err == nil {
		t.Error("radio_buttons with > 10 options should fail")
	}
}

func TestValidateInteractionElement_RadioButtonOptionTextTooLong(t *testing.T) {
	err := validateInteractionElement(map[string]any{
		"type": "radio_buttons",
		"options": []any{
			map[string]any{"text": map[string]any{"text": strings.Repeat("a", 151)}},
		},
	})
	if err == nil {
		t.Error("radio_button option text > 150 runes should fail")
	}
}

func TestValidateInteractionElement_UnknownElement(t *testing.T) {
	err := validateInteractionElement(map[string]any{"type": "unknown_element"})
	if err == nil {
		t.Error("unknown element type should fail")
	}
}

func TestValidateInteractionElement_WorkflowButton(t *testing.T) {
	err := validateInteractionElement(map[string]any{
		"type": "workflow_button",
		"text": map[string]any{"text": "run"},
	})
	if err != nil {
		t.Errorf("workflow_button should be valid: %v", err)
	}
}

func TestValidateInteractionElement_IconButton(t *testing.T) {
	err := validateInteractionElement(map[string]any{"type": "icon_button"})
	if err != nil {
		t.Errorf("icon_button should be valid: %v", err)
	}
}

// --- ValidateTextObject tests ---

func TestValidateTextObject_PlainText(t *testing.T) {
	err := ValidateTextObject(map[string]any{"type": "plain_text", "text": "hello"})
	if err != nil {
		t.Errorf("plain_text should be valid: %v", err)
	}
}

func TestValidateTextObject_Mrkdwn(t *testing.T) {
	err := ValidateTextObject(map[string]any{"type": "mrkdwn", "text": "hello"})
	if err != nil {
		t.Errorf("mrkdwn should be valid: %v", err)
	}
}

func TestValidateTextObject_MissingType(t *testing.T) {
	err := ValidateTextObject(map[string]any{"text": "hello"})
	if err == nil {
		t.Error("text object without type should fail")
	}
}

func TestValidateTextObject_InvalidType(t *testing.T) {
	err := ValidateTextObject(map[string]any{"type": "html", "text": "hello"})
	if err == nil {
		t.Error("invalid text type should fail")
	}
}

func TestValidateTextObject_MissingText(t *testing.T) {
	err := ValidateTextObject(map[string]any{"type": "plain_text"})
	if err == nil {
		t.Error("text object without text should fail")
	}
}

func TestValidateTextObject_PlainTextTooLong(t *testing.T) {
	err := ValidateTextObject(map[string]any{"type": "plain_text", "text": strings.Repeat("a", 151)})
	if err == nil {
		t.Error("plain_text > 150 runes should fail")
	}
}

func TestValidateTextObject_MrkdwnTooLong(t *testing.T) {
	err := ValidateTextObject(map[string]any{"type": "mrkdwn", "text": strings.Repeat("a", 3001)})
	if err == nil {
		t.Error("mrkdwn > 3000 runes should fail")
	}
}

// --- TruncateMrkdwn tests ---

func TestTruncateMrkdwn_ShortText(t *testing.T) {
	text := "hello"
	result := TruncateMrkdwn(text, 100)
	if result != text {
		t.Errorf("short text should not be truncated: %q", result)
	}
}

func TestTruncateMrkdwn_Truncate(t *testing.T) {
	text := strings.Repeat("a", 100)
	result := TruncateMrkdwn(text, 50)
	if !strings.HasSuffix(result, "...") {
		t.Error("truncated text should end with ...")
	}
	if len(result) != 53 {
		t.Errorf("expected 53 chars (50 + ...), got %d", len(result))
	}
}

func TestTruncateMrkdwn_CodeBlock(t *testing.T) {
	text := "prefix ```code```suffix" + strings.Repeat("a", 100)
	result := TruncateMrkdwn(text, 20)
	// Should handle code block correctly (even count)
	if strings.Contains(result, "```") && !strings.HasSuffix(result, "...") {
		t.Error("truncated in code block should be handled")
	}
}

// --- ValidateButtonURLLength tests ---

func TestValidateButtonURLLength_Valid(t *testing.T) {
	if err := ValidateButtonURLLength("https://example.com"); err != nil {
		t.Errorf("short URL should be valid: %v", err)
	}
}

func TestValidateButtonURLLength_TooLong(t *testing.T) {
	longURL := strings.Repeat("a", 3001)
	if err := ValidateButtonURLLength(longURL); err == nil {
		t.Error("URL > 3000 chars should fail")
	}
}

// --- ValidateAccessibilityLabel tests ---

func TestValidateAccessibilityLabel_Valid(t *testing.T) {
	if err := ValidateAccessibilityLabel("click here"); err != nil {
		t.Errorf("short label should be valid: %v", err)
	}
}

func TestValidateAccessibilityLabel_TooLong(t *testing.T) {
	if err := ValidateAccessibilityLabel(strings.Repeat("a", 76)); err == nil {
		t.Error("label > 75 chars should fail")
	}
}

// --- ValidateFileInput tests ---

func TestValidateFileInput_Valid(t *testing.T) {
	err := ValidateFileInput(map[string]any{
		"type": "file_input",
	})
	if err != nil {
		t.Errorf("file_input should be valid: %v", err)
	}
}

func TestValidateFileInput_WrongType(t *testing.T) {
	err := ValidateFileInput(map[string]any{
		"type": "button",
	})
	if err == nil {
		t.Error("wrong type should fail")
	}
}

func TestValidateFileInput_InvalidMaxFiles(t *testing.T) {
	err := ValidateFileInput(map[string]any{
		"type":      "file_input",
		"max_files": 0,
	})
	if err == nil {
		t.Error("max_files 0 should fail")
	}

	err = ValidateFileInput(map[string]any{
		"type":      "file_input",
		"max_files": 11,
	})
	if err == nil {
		t.Error("max_files > 10 should fail")
	}
}

func TestValidateFileInput_ValidMaxFiles(t *testing.T) {
	err := ValidateFileInput(map[string]any{
		"type":      "file_input",
		"max_files": 5,
	})
	if err != nil {
		t.Errorf("max_files 5 should be valid: %v", err)
	}
}

func TestValidateFileInput_TooManyFiletypes(t *testing.T) {
	fts := make([]string, 11)
	for i := range fts {
		fts[i] = "pdf"
	}
	err := ValidateFileInput(map[string]any{
		"type":      "file_input",
		"filetypes": fts,
	})
	if err == nil {
		t.Error("> 10 filetypes should fail")
	}
}

func TestValidateFileInput_InvalidFiletype(t *testing.T) {
	err := ValidateFileInput(map[string]any{
		"type":      "file_input",
		"filetypes": []string{"exe"},
	})
	if err == nil {
		t.Error("invalid filetype should fail")
	}
}

func TestValidateFileInput_ValidFiletypes(t *testing.T) {
	err := ValidateFileInput(map[string]any{
		"type":      "file_input",
		"filetypes": []string{"pdf", "docx", ".txt"},
	})
	if err != nil {
		t.Errorf("valid filetypes should pass: %v", err)
	}
}

// --- ValidateRichTextInput tests ---

func TestValidateRichTextInput_Valid(t *testing.T) {
	err := ValidateRichTextInput(map[string]any{
		"type":          "rich_text_input",
		"initial_value": "hello",
	})
	if err != nil {
		t.Errorf("rich_text_input should be valid: %v", err)
	}
}

func TestValidateRichTextInput_TooLong(t *testing.T) {
	err := ValidateRichTextInput(map[string]any{
		"type":          "rich_text_input",
		"initial_value": strings.Repeat("a", 3001),
	})
	if err == nil {
		t.Error("initial_value > 3000 runes should fail")
	}
}

// --- ValidateTableBlock tests ---

func TestValidateTableBlock_Valid(t *testing.T) {
	err := ValidateTableBlock(map[string]any{
		"type": "table",
		"rows": []map[string]any{{}},
	})
	if err != nil {
		t.Errorf("table should be valid: %v", err)
	}
}

func TestValidateTableBlock_MissingRows(t *testing.T) {
	err := ValidateTableBlock(map[string]any{"type": "table"})
	if err == nil {
		t.Error("table without rows should fail")
	}
}

func TestValidateTableBlock_EmptyRows(t *testing.T) {
	err := ValidateTableBlock(map[string]any{
		"type": "table",
		"rows": []map[string]any{},
	})
	if err == nil {
		t.Error("table with empty rows should fail")
	}
}

func TestValidateTableBlock_TooManyRows(t *testing.T) {
	rows := make([]map[string]any, 1001)
	for i := range rows {
		rows[i] = map[string]any{}
	}
	err := ValidateTableBlock(map[string]any{
		"type": "table",
		"rows": rows,
	})
	if err == nil {
		t.Error("table with > 1000 rows should fail")
	}
}

func TestValidateTableBlock_TooManyColumns(t *testing.T) {
	err := ValidateTableBlock(map[string]any{
		"type":    "table",
		"rows":    []map[string]any{{}},
		"columns": 13,
	})
	if err == nil {
		t.Error("table with > 12 columns should fail")
	}
}

// --- ValidatePlanBlock tests ---

func TestValidatePlanBlock_Valid(t *testing.T) {
	err := ValidatePlanBlock(map[string]any{
		"type":     "plan",
		"sections": []map[string]any{{}},
	})
	if err != nil {
		t.Errorf("plan should be valid: %v", err)
	}
}

func TestValidatePlanBlock_MissingSections(t *testing.T) {
	err := ValidatePlanBlock(map[string]any{"type": "plan"})
	if err == nil {
		t.Error("plan without sections should fail")
	}
}

func TestValidatePlanBlock_TooManySections(t *testing.T) {
	sections := make([]map[string]any, 26)
	for i := range sections {
		sections[i] = map[string]any{}
	}
	err := ValidatePlanBlock(map[string]any{
		"type":     "plan",
		"sections": sections,
	})
	if err == nil {
		t.Error("plan with > 25 sections should fail")
	}
}

// --- ValidateTaskCardBlock tests ---

func TestValidateTaskCardBlock_Valid(t *testing.T) {
	for _, status := range []string{"pending", "in_progress", "completed"} {
		err := ValidateTaskCardBlock(map[string]any{"type": "task_card", "status": status})
		if err != nil {
			t.Errorf("task_card with status %s should be valid: %v", status, err)
		}
	}
}

func TestValidateTaskCardBlock_MissingStatus(t *testing.T) {
	err := ValidateTaskCardBlock(map[string]any{"type": "task_card"})
	if err == nil {
		t.Error("task_card without status should fail")
	}
}

func TestValidateTaskCardBlock_EmptyStatus(t *testing.T) {
	err := ValidateTaskCardBlock(map[string]any{"type": "task_card", "status": ""})
	if err == nil {
		t.Error("task_card with empty status should fail")
	}
}

func TestValidateTaskCardBlock_InvalidStatus(t *testing.T) {
	err := ValidateTaskCardBlock(map[string]any{"type": "task_card", "status": "invalid"})
	if err == nil {
		t.Error("task_card with invalid status should fail")
	}
}

// --- ValidateComplete tests ---

func TestValidateComplete_UnknownType(t *testing.T) {
	err := ValidateComplete(map[string]any{"type": "divider"})
	if err != nil {
		t.Errorf("unknown type should pass (no type-specific validation): %v", err)
	}
}

// --- ValidateButtonComplete tests ---

func TestValidateButtonComplete_Valid(t *testing.T) {
	err := ValidateButtonComplete(map[string]any{
		"type":      "button",
		"text":      map[string]any{"type": "plain_text", "text": "click"},
		"action_id": "action_1",
	})
	if err != nil {
		t.Errorf("button should be valid: %v", err)
	}
}

func TestValidateButtonComplete_MissingText(t *testing.T) {
	err := ValidateButtonComplete(map[string]any{
		"type":      "button",
		"action_id": "action_1",
	})
	if err == nil {
		t.Error("button without text should fail")
	}
}

func TestValidateButtonComplete_MissingActionID(t *testing.T) {
	err := ValidateButtonComplete(map[string]any{
		"type": "button",
		"text": map[string]any{"type": "plain_text", "text": "click"},
	})
	if err == nil {
		t.Error("button without action_id should fail")
	}
}

// --- ValidateImageComplete tests ---

func TestValidateImageComplete_Valid(t *testing.T) {
	err := ValidateImageComplete(map[string]any{
		"image_url": "https://example.com/img.png",
		"alt_text":  "image",
	})
	if err != nil {
		t.Errorf("image should be valid: %v", err)
	}
}

func TestValidateImageComplete_MissingURL(t *testing.T) {
	err := ValidateImageComplete(map[string]any{"alt_text": "image"})
	if err == nil {
		t.Error("image without image_url should fail")
	}
}

func TestValidateImageComplete_MissingAlt(t *testing.T) {
	err := ValidateImageComplete(map[string]any{"image_url": "https://example.com"})
	if err == nil {
		t.Error("image without alt_text should fail")
	}
}

func TestValidateImageComplete_AltTooLong(t *testing.T) {
	err := ValidateImageComplete(map[string]any{
		"image_url": "https://example.com",
		"alt_text":  strings.Repeat("a", 2001),
	})
	if err == nil {
		t.Error("alt_text > 2000 runes should fail")
	}
}

// --- ValidateFileComplete tests ---

func TestValidateFileComplete_Valid(t *testing.T) {
	err := ValidateFileComplete(map[string]any{"external_id": "F123"})
	if err != nil {
		t.Errorf("file should be valid: %v", err)
	}
}

func TestValidateFileComplete_MissingID(t *testing.T) {
	err := ValidateFileComplete(map[string]any{})
	if err == nil {
		t.Error("file without external_id should fail")
	}
}

func TestValidateFileComplete_IDTooLong(t *testing.T) {
	err := ValidateFileComplete(map[string]any{"external_id": strings.Repeat("a", 256)})
	if err == nil {
		t.Error("external_id > 255 runes should fail")
	}
}

// --- ValidateBlockWithDetails tests ---

func TestValidateBlockWithDetails_Valid(t *testing.T) {
	errs := ValidateBlockWithDetails(map[string]any{
		"type": "divider",
	}, 0)
	if len(errs) != 0 {
		t.Errorf("divider should have no errors: %v", errs)
	}
}

func TestValidateBlockWithDetails_Invalid(t *testing.T) {
	errs := ValidateBlockWithDetails(map[string]any{}, 0)
	if len(errs) == 0 {
		t.Error("invalid block should have errors")
	}
}
