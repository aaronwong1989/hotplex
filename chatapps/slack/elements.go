package slack

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// BuildButton creates a button element
func BuildButton(text, actionID, value string, style string) map[string]any {
	button := map[string]any{
		"type":      "button",
		"text":      plainText(text),
		"action_id": actionID,
		"value":     value,
	}
	if style == "primary" || style == "danger" {
		button["style"] = style
	}
	return button
}

// BuildButtonWithURL creates a button that opens a URL
func BuildButtonWithURL(text, actionID, url string) map[string]any {
	return map[string]any{
		"type":      "button",
		"text":      plainText(text),
		"action_id": actionID,
		"url":       url,
	}
}

// BuildDatePicker creates a date picker
func BuildDatePicker(placeholder, actionID string, initialDate string) map[string]any {
	element := map[string]any{
		"type":        "datepicker",
		"placeholder": plainText(placeholder),
		"action_id":   actionID,
	}
	if initialDate != "" {
		element["initial_date"] = initialDate
	}
	return element
}

// BuildPlainTextInput creates a plain text input field
func BuildPlainTextInput(placeholder, actionID string, initialValue string, multiline bool, maxLength int, minLength int) map[string]any {
	element := map[string]any{
		"type":      "plain_text_input",
		"action_id": actionID,
	}
	if placeholder != "" {
		element["placeholder"] = plainText(placeholder)
	}
	if initialValue != "" {
		element["initial_value"] = initialValue
	}
	if multiline {
		element["multiline"] = true
	}
	if maxLength > 0 {
		element["max_length"] = maxLength
	}
	if minLength > 0 {
		element["min_length"] = minLength
	}
	return element
}

// BuildRadioButtons creates a radio button group
func BuildRadioButtons(options []map[string]any, actionID string, initialOption map[string]any) map[string]any {
	element := map[string]any{
		"type":      "radio_buttons",
		"options":   options,
		"action_id": actionID,
	}
	if initialOption != nil {
		element["initial_option"] = initialOption
	}
	return element
}

// BuildCheckboxes creates a checkbox group
func BuildCheckboxes(options []map[string]any, actionID string, initialOptions []map[string]any) map[string]any {
	element := map[string]any{
		"type":      "checkboxes",
		"options":   options,
		"action_id": actionID,
	}
	if len(initialOptions) > 0 {
		element["initial_options"] = initialOptions
	}
	return element
}

// BuildStaticSelect creates a static select menu
func BuildStaticSelect(options []map[string]any, placeholder, actionID string, initialOption map[string]any) map[string]any {
	element := map[string]any{
		"type":        "static_select",
		"options":     options,
		"placeholder": plainText(placeholder),
		"action_id":   actionID,
	}
	if initialOption != nil {
		element["initial_option"] = initialOption
	}
	return element
}

// BuildButtonWithValidation creates a button with full validation
func BuildButtonWithValidation(text, actionID, value string, style string) (map[string]any, error) {
	if err := ValidateActionID(actionID); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(value) > 2000 {
		return nil, fmt.Errorf("button value too long: %d chars (max 2000)", utf8.RuneCountInString(value))
	}
	return BuildButton(text, actionID, value, style), nil
}

// BuildButtonWithURLValidation creates a button with URL validation
func BuildButtonWithURLValidation(text, actionID, rawURL string) (map[string]any, error) {
	if err := ValidateActionID(actionID); err != nil {
		return nil, err
	}
	if err := ValidateButtonURL(rawURL); err != nil {
		return nil, err
	}
	return BuildButtonWithURL(text, actionID, rawURL), nil
}

// BuildPlainTextInputWithValidation creates a plain text input with validation
func BuildPlainTextInputWithValidation(placeholder, actionID, initialValue string, multiline bool, maxLength, minLength int) (map[string]any, error) {
	if err := ValidateActionID(actionID); err != nil {
		return nil, err
	}
	if initialValue != "" {
		initialValue = ValidateInitialValue(initialValue, 3000)
	}
	return BuildPlainTextInput(placeholder, actionID, initialValue, multiline, maxLength, minLength), nil
}

// BuildOptionWithValidation creates an option with validation
func BuildOptionWithValidation(label, value string) (map[string]any, error) {
	if err := ValidateOptionValue(value); err != nil {
		return nil, err
	}
	return BuildOption(label, value), nil
}

// =============================================================================
// New Element Types (2025-2026 Additions)
// =============================================================================

// BuildFileInput creates a file input element for file uploads
// Reference: https://api.slack.com/reference/block-kit/block-elements#file-input-element
func BuildFileInput(actionID string, filetypes []string, maxFiles int) map[string]any {
	element := map[string]any{
		"type":      "file_input",
		"action_id": actionID,
	}
	if len(filetypes) > 0 {
		element["filetypes"] = filetypes // e.g., ["pdf", "doc", "docx"]
	}
	if maxFiles > 0 {
		element["max_files"] = maxFiles
	}
	return element
}

// BuildWorkflowButton creates a button that triggers a Slack workflow
// Reference: https://api.slack.com/reference/block-kit/block-elements#workflow-button-element
func BuildWorkflowButton(text string, workflow struct{ ID string; }, style string) map[string]any {
	button := map[string]any{
		"type":      "workflow_button",
		"text":      plainText(text),
		"workflow":  workflow,
	}
	if style == "primary" || style == "danger" {
		button["style"] = style
	}
	return button
}

// BuildIconButton creates an icon-only button (minimal UI)
// Reference: https://api.slack.com/reference/block-kit/block-elements#icon-button-element
func BuildIconButton(iconName string, actionID string, confirm map[string]any) map[string]any {
	button := map[string]any{
		"type":      "icon_button",
		"icon":      iconName, // Slack emoji name or custom icon
		"action_id": actionID,
	}
	if confirm != nil {
		button["confirm"] = confirm
	}
	return button
}

// BuildFeedbackButtons creates thumbs up/down feedback buttons
// Reference: https://api.slack.com/reference/block-kit/block-elements#feedback-buttons-element
func BuildFeedbackButtons(positiveActionID, negativeActionID string) map[string]any {
	return map[string]any{
		"type": "feedback_buttons",
		"positive_action_id": positiveActionID,
		"negative_action_id": negativeActionID,
	}
}

// BuildRichTextInput creates a rich text input field
// Reference: https://api.slack.com/reference/block-kit/block-elements#rich-text-input-element
func BuildRichTextInput(actionID, placeholder string, initialValue string, focusOnLoad bool) map[string]any {
	element := map[string]any{
		"type":        "rich_text_input",
		"action_id":   actionID,
		"placeholder": plainText(placeholder),
	}
	if initialValue != "" {
		element["initial_value"] = initialValue
	}
	if focusOnLoad {
		element["focus_on_load"] = true
	}
	return element
}

// BuildURLSource creates a URL source element for external data
// Reference: https://api.slack.com/reference/block-kit/block-elements#url-source-element
// SECURITY: Validates URL scheme to prevent SSRF and XSS attacks
func BuildURLSource(actionID, url string) (map[string]any, error) {
	// Validate action_id
	if err := ValidateActionID(actionID); err != nil {
		return nil, err
	}
	
	// SECURITY: Validate URL to prevent SSRF/XSS
	if err := ValidateButtonURL(url); err != nil {
		return nil, fmt.Errorf("invalid URL for url_source: %w", err)
	}
	
	// Additional security: block dangerous schemes
	dangerousSchemes := []string{"javascript:", "data:", "file:", "vbscript:"}
	for _, scheme := range dangerousSchemes {
		if strings.HasPrefix(strings.ToLower(url), scheme) {
			return nil, fmt.Errorf("dangerous URL scheme detected: %s", scheme)
		}
	}
	
	return map[string]any{
		"type":      "url_source",
		"action_id": actionID,
		"url":       url,
	}, nil
}

// BuildURLSourceUnsafe creates a URL source element without validation (use with caution)
// Only use this when you have already validated the URL through other means
func BuildURLSourceUnsafe(actionID, url string) map[string]any {
	return map[string]any{
		"type":      "url_source",
		"action_id": actionID,
		"url":       url,
	}
}

// BuildMultiEmailInput creates a multi-email input field
func BuildMultiEmailInput(actionID, placeholder string, maxValues int) map[string]any {
	element := map[string]any{
		"type":        "email_input",
		"action_id":   actionID,
		"placeholder": plainText(placeholder),
	}
	if maxValues > 0 {
		element["max_values"] = maxValues
	}
	return element
}

// BuildExternalSelectWithMinQuery creates an external select with minimum query length
func BuildExternalSelectWithMinQuery(placeholder, actionID string, minQueryLength int, initialOption map[string]any) map[string]any {
	element := map[string]any{
		"type":            "external_select",
		"placeholder":     plainText(placeholder),
		"action_id":       actionID,
		"min_query_length": minQueryLength,
	}
	if initialOption != nil {
		element["initial_option"] = initialOption
	}
	return element
}

// BuildConversationsSelectWithFilter creates a conversations select with filter
func BuildConversationsSelectWithFilter(placeholder, actionID string, filter map[string]any, defaultToCurrentConversation bool) map[string]any {
	element := map[string]any{
		"type":        "conversations_select",
		"placeholder": plainText(placeholder),
		"action_id":   actionID,
		"filter":      filter,
	}
	if defaultToCurrentConversation {
		element["default_to_current_conversation"] = true
	}
	return element
}

// BuildResponseURLPlaceholder creates a placeholder for response URLs
func BuildResponseURLPlaceholder(actionID string) map[string]any {
	return map[string]any{
		"type":      "response_url_placeholder",
		"action_id": actionID,
	}
}

// BuildElementWithValidation wraps element creation with validation
func BuildElementWithValidation(elemType, actionID string, validateFunc func() error) (map[string]any, error) {
	if err := ValidateActionID(actionID); err != nil {
		return nil, err
	}
	if validateFunc != nil {
		if err := validateFunc(); err != nil {
			return nil, err
		}
	}
	return map[string]any{"type": elemType, "action_id": actionID}, nil
}
