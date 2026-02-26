package slack

// =============================================================================
// Composition Object Builders
// =============================================================================

// BuildOption creates an option object for select menus, radio buttons, checkboxes
// Reference: https://api.slack.com/reference/block-kit/composition-objects#option
func BuildOption(label, value string) map[string]any {
	return map[string]any{
		"text":  plainText(label),
		"value": value,
	}
}

// BuildOptionWithDescription creates an option with description
func BuildOptionWithDescription(label, value, description string) map[string]any {
	return map[string]any{
		"text":        plainText(label),
		"value":       value,
		"description": plainText(description),
	}
}

// BuildOptionGroup creates an option group for select menus
// Reference: https://api.slack.com/reference/block-kit/composition-objects#option-group
func BuildOptionGroup(label string, options []map[string]any) map[string]any {
	return map[string]any{
		"label":   plainText(label),
		"options": options,
	}
}

// BuildConfirmationDialog creates a confirmation dialog
// Reference: https://api.slack.com/reference/block-kit/composition-objects#confirm
func BuildConfirmationDialog(title, text, confirmText, denyText string) map[string]any {
	return map[string]any{
		"title":  plainText(title),
		"text":   plainText(text),
		"confirm": plainText(confirmText),
		"deny":    plainText(denyText),
	}
}

// BuildConfirmationDialogWithStyle creates a confirmation dialog with style (primary/danger)
func BuildConfirmationDialogWithStyle(title, text, confirmText, denyText, style string) map[string]any {
	confirmation := BuildConfirmationDialog(title, text, confirmText, denyText)
	if style == "primary" || style == "danger" {
		confirmation["style"] = style
	}
	return confirmation
}

// BuildMrkdwnText creates a mrkdwn text object
// Reference: https://api.slack.com/reference/block-kit/composition-objects#text
func BuildMrkdwnText(text string, verbatim bool) map[string]any {
	obj := map[string]any{
		"type": "mrkdwn",
		"text": text,
	}
	if verbatim {
		obj["verbatim"] = true
	}
	return obj
}

// BuildPlainText creates a plain_text text object
func BuildPlainText(text string, emoji bool) map[string]any {
	return map[string]any{
		"type":  "plain_text",
		"text":  text,
		"emoji": emoji,
	}
}

// BuildFilter creates a filter for conversations_select
// Reference: https://api.slack.com/reference/block-kit/composition-objects#filter_conversations
func BuildFilter(includeTypes []string, excludeExternalSharedChannels bool) map[string]any {
	filter := map[string]any{}
	if len(includeTypes) > 0 {
		filter["include"] = includeTypes
	}
	if excludeExternalSharedChannels {
		filter["exclude_external_shared_channels"] = true
	}
	return filter
}

// BuildDispatchActionConfig creates a dispatch action config for input elements
// Reference: https://api.slack.com/reference/block-kit/composition-objects#dispatch_action_config
func BuildDispatchActionConfig(triggerActions []string) map[string]any {
	return map[string]any{
		"trigger_actions_on": triggerActions,
	}
}

// BuildFocusOnLoad sets focus on an input element when a modal loads
func WithFocusOnLoad(element map[string]any) map[string]any {
	element["focus_on_load"] = true
	return element
}

// BuildAccessoryImage creates an image accessory for section blocks
// Reference: https://api.slack.com/reference/block-kit/composition-objects#image
func BuildAccessoryImage(imageURL, altText string) map[string]any {
	return map[string]any{
		"type":      "image",
		"image_url": imageURL,
		"alt_text":  altText,
	}
}

// BuildRichTextList creates a rich_text_list element
// Reference: https://api.slack.com/reference/block-kit/block-elements#rich_text_list
func BuildRichTextList(elements []map[string]any, style string) map[string]any {
	return map[string]any{
		"type":     "rich_text_list",
		"elements": elements,
		"style":    style, // "bullet" or "ordered"
	}
}

// BuildRichTextPreformatted creates a rich_text_preformatted element
func BuildRichTextPreformatted(elements []map[string]any) map[string]any {
	return map[string]any{
		"type":     "rich_text_preformatted",
		"elements": elements,
	}
}

// BuildRichTextQuote creates a rich_text_quote element
func BuildRichTextQuote(elements []map[string]any) map[string]any {
	return map[string]any{
		"type":     "rich_text_quote",
		"elements": elements,
	}
}

// BuildLink creates a link for rich_text_section
func BuildLink(text, url string) map[string]any {
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

// BuildUserMention creates a user mention for rich_text_section
func BuildUserMention(userID string) map[string]any {
	return map[string]any{
		"type": "user",
		"user_id": userID,
	}
}

// BuildChannelMention creates a channel mention for rich_text_section
func BuildChannelMention(channelID string) map[string]any {
	return map[string]any{
		"type": "channel",
		"channel_id": channelID,
	}
}

// BuildEmoji creates an emoji element for rich_text_section
func BuildEmoji(name string) map[string]any {
	return map[string]any{
		"type": "emoji",
		"name": name,
	}
}

// ValidateAll validates all fields in the composition object

// =============================================================================
// Additional Composition Objects
// =============================================================================

// BuildTriggerObject creates a trigger composition object
func BuildTriggerObject(interactiveTrigger string) map[string]any {
	return map[string]any{
		"type": "trigger",
		"interactive_trigger": interactiveTrigger,
	}
}

// BuildWorkflowObject creates a workflow composition object
func BuildWorkflowObject(workflowID string, inputs map[string]any) map[string]any {
	workflow := map[string]any{
		"id": workflowID,
	}
	if len(inputs) > 0 {
		workflow["inputs"] = inputs
	}
	return map[string]any{
		"type":     "workflow",
		"workflow": workflow,
	}
}

// BuildSlackFileObject creates a slack_file composition object
func BuildSlackFileObject(id string, url string) map[string]any {
	file := map[string]any{}
	if id != "" {
		file["id"] = id
	}
	if url != "" {
		file["url"] = url
	}
	return map[string]any{
		"type":       "slack_file",
		"slack_file": file,
	}
}

// BuildRichTextElement creates a rich text element
func BuildRichTextElement(elementType string, text string) map[string]any {
	return map[string]any{
		"type": elementType,
		"text": text,
	}
}

// BuildRichTextRange creates a rich text range for styling
func BuildRichTextRange(start, end int, style map[string]any) map[string]any {
	return map[string]any{
		"start": start,
		"end":   end,
		"style": style,
	}
}

// BuildStyleObject creates a style object for rich text
func BuildStyleObject(bold, italic, strike, code bool) map[string]any {
	style := map[string]any{}
	if bold {
		style["bold"] = true
	}
	if italic {
		style["italic"] = true
	}
	if strike {
		style["strike"] = true
	}
	if code {
		style["code"] = true
	}
	return style
}

// BuildConversationFilter creates a filter for conversations select
func BuildConversationFilter(include []string, excludeBotUsers, excludeExternalSharedChannels bool) map[string]any {
	filter := map[string]any{}
	if len(include) > 0 {
		filter["include"] = include // "im", "mpim", "private", "public"
	}
	if excludeBotUsers {
		filter["exclude_bot_users"] = true
	}
	if excludeExternalSharedChannels {
		filter["exclude_external_shared_channels"] = true
	}
	return filter
}

// BuildAccessibilityLabel creates accessibility label for button
func BuildAccessibilityLabel(label string) map[string]any {
	return map[string]any{
		"accessibility_label": label,
	}
}

// BuildDeliveryPolicy creates a delivery policy for messages
func BuildDeliveryPolicy(persistence string) map[string]any {
	return map[string]any{
		"type": "delivery_policy",
		"persistence": persistence, // "default", "ephemeral"
	}
}
