package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/provider"
	"github.com/slack-go/slack"
)

// MessageBuilder builds Slack-specific messages from platform-agnostic ChatMessage
type MessageBuilder struct {
	formatter *MrkdwnFormatter
}

// NewMessageBuilder creates a new MessageBuilder
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		formatter: NewMrkdwnFormatter(),
	}
}

// Build builds Slack blocks from a ChatMessage based on its type
func (b *MessageBuilder) Build(msg *base.ChatMessage) []slack.Block {
	switch msg.Type {
	case base.MessageTypeThinking:
		return b.BuildThinkingMessage(msg)
	case base.MessageTypeToolUse:
		return b.BuildToolUseMessage(msg)
	case base.MessageTypeToolResult:
		return b.BuildToolResultMessage(msg)
	case base.MessageTypeAnswer:
		return b.BuildAnswerMessage(msg)
	case base.MessageTypeError:
		return b.BuildErrorMessage(msg)
	case base.MessageTypePlanMode:
		return b.BuildPlanModeMessage(msg)
	case base.MessageTypeExitPlanMode:
		return b.BuildExitPlanModeMessage(msg)
	case base.MessageTypeAskUserQuestion:
		return b.BuildAskUserQuestionMessage(msg)
	case base.MessageTypeDangerBlock:
		return b.BuildDangerBlockMessage(msg)
	case base.MessageTypeSessionStats:
		return b.BuildSessionStatsMessage(msg)
	default:
		// Default to answer message for unknown types
		return b.BuildAnswerMessage(msg)
	}
}

// =============================================================================
// Thinking Message (AI is reasoning)
// =============================================================================

// BuildThinkingMessage builds a status indicator for thinking state
func (b *MessageBuilder) BuildThinkingMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		content = "Thinking..."
	}

	text := slack.NewTextBlockObject("mrkdwn", ":hourglass_flowing_sand: "+content, false, false)
	return []slack.Block{
		slack.NewSectionBlock(text, nil, nil),
	}
}

// =============================================================================
// Tool Use Message (Tool invocation started)
// =============================================================================

// BuildToolUseMessage builds a message for tool invocation
func (b *MessageBuilder) BuildToolUseMessage(msg *base.ChatMessage) []slack.Block {
	toolName := msg.Content
	if toolName == "" {
		toolName = "Unknown Tool"
	}

	// Extract tool input from RichContent if available
	input := ""
	if msg.RichContent != nil {
		for _, block := range msg.RichContent.Blocks {
			if blockMap, ok := block.(map[string]any); ok {
				if blockMap["type"] == "section" {
					if fields, ok := blockMap["fields"].([]any); ok && len(fields) > 0 {
						if textObj, ok := fields[0].(map[string]any); ok {
							if text, ok := textObj["text"].(string); ok {
								input = text
							}
						}
					}
				}
			}
		}
	}

	// Truncate input if too long
	if len(input) > 500 {
		input = input[:500] + "..."
	}

	text := fmt.Sprintf(":hammer_and_wrench: *Using tool:* `%s`", toolName)
	if input != "" {
		text += fmt.Sprintf("\n```\n%s\n```", input)
	}

	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)
	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// =============================================================================
// Tool Result Message (Tool execution completed)
// =============================================================================

// BuildToolResultMessage builds a message for tool execution result
func (b *MessageBuilder) BuildToolResultMessage(msg *base.ChatMessage) []slack.Block {
	var blocks []slack.Block

	// Check metadata for success status
	success := true
	if msg.Metadata != nil {
		if s, ok := msg.Metadata["success"].(bool); ok {
			success = s
		}
	}

	// Get duration from metadata
	durationMs := int64(0)
	if msg.Metadata != nil {
		if d, ok := msg.Metadata["duration_ms"].(int64); ok {
			durationMs = d
		}
	}

	icon := ":white_check_mark:"
	if !success {
		icon = ":x:"
	}

	durationStr := ""
	if durationMs > 0 {
		if durationMs > 1000 {
			durationStr = fmt.Sprintf(" (%.2fs)", float64(durationMs)/1000)
		} else {
			durationStr = fmt.Sprintf(" (%dms)", durationMs)
		}
	}

	content := msg.Content
	// Truncate if too long
	if len(content) > 3000 {
		content = content[:3000] + "\n... (truncated)"
	}

	statusText := fmt.Sprintf("%s Tool completed%s", icon, durationStr)
	statusObj := slack.NewTextBlockObject("mrkdwn", statusText, false, false)
	blocks = append(blocks, slack.NewSectionBlock(statusObj, nil, nil))

	if content != "" {
		// Format as code block
		codeText := slack.NewTextBlockObject("mrkdwn", "```\n"+content+"\n```", false, false)
		blocks = append(blocks, slack.NewSectionBlock(codeText, nil, nil))
	}

	return blocks
}

// =============================================================================
// Answer Message (Final text output)
// =============================================================================

// BuildAnswerMessage builds a message for AI answer
func (b *MessageBuilder) BuildAnswerMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		return nil
	}

	// Convert Markdown to mrkdwn
	formattedContent := b.formatter.Format(content)

	// Check if content is too long for a single message
	if len(formattedContent) > 4000 {
		// Split into chunks
		return b.buildChunkedAnswerBlocks(formattedContent)
	}

	mrkdwn := slack.NewTextBlockObject("mrkdwn", formattedContent, false, false)
	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// buildChunkedAnswerBlocks splits long content into chunks
func (b *MessageBuilder) buildChunkedAnswerBlocks(content string) []slack.Block {
	var blocks []slack.Block

	chunks := b.chunkText(content, 3500)
	for i, chunk := range chunks {
		if i > 0 {
			// Add divider between chunks
			blocks = append(blocks, slack.NewDividerBlock())
		}
		mrkdwn := slack.NewTextBlockObject("mrkdwn", chunk, false, false)
		blocks = append(blocks, slack.NewSectionBlock(mrkdwn, nil, nil))
	}

	return blocks
}

// chunkText splits text into chunks at word boundaries
func (b *MessageBuilder) chunkText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	currentChunk := ""

	for _, line := range lines {
		if len(currentChunk)+len(line)+1 > maxLen {
			if currentChunk != "" {
				chunks = append(chunks, currentChunk)
				currentChunk = ""
			}
		}
		if currentChunk != "" {
			currentChunk += "\n"
		}
		currentChunk += line
	}

	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// =============================================================================
// Error Message
// =============================================================================

// BuildErrorMessage builds a message for errors
func (b *MessageBuilder) BuildErrorMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		content = "An error occurred"
	}

	// Add error emoji prefix
	text := ":warning: *Error*\n" + content
	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// =============================================================================
// Plan Mode Message
// =============================================================================

// BuildPlanModeMessage builds a message for plan mode
func (b *MessageBuilder) BuildPlanModeMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		content = "Planning..."
	}

	text := ":mag_right: *Plan Mode*\n" + content
	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// =============================================================================
// Exit Plan Mode Message (Requesting user approval)
// =============================================================================

// BuildExitPlanModeMessage builds a message for exit plan mode
func (b *MessageBuilder) BuildExitPlanModeMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		content = "Plan generated. Waiting for approval."
	}

	text := ":clipboard: *Plan Ready*\n" + content

	// Add approve/deny buttons
	approveBtn := slack.NewButtonBlockElement("plan_approve", "approve",
		slack.NewTextBlockObject("plain_text", "Approve", false, true))
	approveBtn.Style = "primary"

	denyBtn := slack.NewButtonBlockElement("plan_deny", "deny",
		slack.NewTextBlockObject("plain_text", "Deny", false, true))
	denyBtn.Style = "danger"

	actionBlock := slack.NewActionBlock("plan_actions", approveBtn, denyBtn)

	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
		slack.NewDividerBlock(),
		actionBlock,
	}
}

// =============================================================================
// Ask User Question Message
// =============================================================================

// BuildAskUserQuestionMessage builds a message for user questions
func (b *MessageBuilder) BuildAskUserQuestionMessage(msg *base.ChatMessage) []slack.Block {
	question := msg.Content
	if question == "" {
		question = "Please provide more information."
	}

	text := ":question: *Question*\n" + question
	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	blocks := []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}

	// Add options as buttons if available in metadata
	if msg.Metadata != nil {
		if options, ok := msg.Metadata["options"].([]string); ok && len(options) > 0 {
			var buttons []slack.BlockElement
			for i, option := range options {
				btn := slack.NewButtonBlockElement(fmt.Sprintf("question_option_%d", i), option,
					slack.NewTextBlockObject("plain_text", option, false, false))
				buttons = append(buttons, btn)
			}
			if len(buttons) > 0 {
				blocks = append(blocks, slack.NewActionBlock("question_options", buttons...))
			}
		}
	}

	return blocks
}

// =============================================================================
// Danger Block Message
// =============================================================================

// BuildDangerBlockMessage builds a message for dangerous operations
func (b *MessageBuilder) BuildDangerBlockMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		content = "This operation requires confirmation."
	}

	text := ":rotating_light: *Confirmation Required*\n" + content

	// Add confirm/cancel buttons
	confirmBtn := slack.NewButtonBlockElement("danger_confirm", "confirm",
		slack.NewTextBlockObject("plain_text", "Confirm", false, true))
	confirmBtn.Style = "danger"

	cancelBtn := slack.NewButtonBlockElement("danger_cancel", "cancel",
		slack.NewTextBlockObject("plain_text", "Cancel", false, true))

	actionBlock := slack.NewActionBlock("danger_actions", confirmBtn, cancelBtn)

	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
		slack.NewDividerBlock(),
		actionBlock,
	}
}

// =============================================================================
// Session Stats Message
// =============================================================================

// BuildSessionStatsMessage builds a message for session statistics
func (b *MessageBuilder) BuildSessionStatsMessage(msg *base.ChatMessage) []slack.Block {
	content := msg.Content
	if content == "" {
		return nil
	}

	text := ":chart_with_upwards_trend: *Session Stats*\n" + content
	mrkdwn := slack.NewTextBlockObject("mrkdwn", text, false, false)

	return []slack.Block{
		slack.NewSectionBlock(mrkdwn, nil, nil),
	}
}

// =============================================================================
// Plan Approval/Denial Messages (Interactive Callbacks)
// =============================================================================

// BuildPlanApprovedBlock builds blocks to show after plan is approved
func (b *MessageBuilder) BuildPlanApprovedBlock() []slack.Block {
	text := slack.NewTextBlockObject("mrkdwn", "✅ *Plan Approved*\n\nClaude is now executing the plan...", false, false)
	return []slack.Block{
		slack.NewSectionBlock(text, nil, nil),
	}
}

// BuildPlanCancelledBlock builds blocks to show after plan is cancelled
func (b *MessageBuilder) BuildPlanCancelledBlock(reason string) []slack.Block {
	text := slack.NewTextBlockObject("mrkdwn", "❌ *Plan Cancelled*", false, false)
	blocks := []slack.Block{
		slack.NewSectionBlock(text, nil, nil),
	}

	if reason != "" {
		reasonText := slack.NewTextBlockObject("mrkdwn", "Reason: "+reason, false, false)
		blocks = append(blocks, slack.NewSectionBlock(reasonText, nil, nil))
	}

	return blocks
}

// =============================================================================
// Permission Request Messages (Interactive Callbacks)
// =============================================================================

// BuildPermissionRequestMessage builds Slack blocks for a permission request
// Displays tool name, command preview, and approval/denial buttons
func (b *MessageBuilder) BuildPermissionRequestMessage(req *provider.PermissionRequest, sessionID string) []slack.Block {
	tool, input := req.GetToolAndInput()

	// Sanitize and truncate commands for preview
	safeInput := SanitizeCommand(input)
	displayInput := safeInput
	if RuneCount(displayInput) > 500 {
		displayInput = TruncateByRune(displayInput, 497) + "..."
	}

	var blocks []slack.Block

	// Header
	headerText := slack.NewTextBlockObject("plain_text", "⚠️ Permission Request", true, false)
	blocks = append(blocks, slack.NewHeaderBlock(headerText))

	// Tool information
	if tool != "" {
		toolText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tool:* `%s`", tool), false, false)
		blocks = append(blocks, slack.NewSectionBlock(toolText, nil, nil))
	}

	// Command/Action preview
	if displayInput != "" {
		cmdText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Command:*\n```\n%s\n```", displayInput), false, false)
		blocks = append(blocks, slack.NewSectionBlock(cmdText, nil, nil))
	}

	// Decision reason (if available)
	if req.Decision != nil && req.Decision.Reason != "" {
		reasonText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Reason:* %s", req.Decision.Reason), false, false)
		blocks = append(blocks, slack.NewContextBlock("", []slack.MixedElement{
			reasonText,
		}...))
	}

	// Session info
	sessionText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Session: `%s`", sessionID), false, false)
	blocks = append(blocks, slack.NewContextBlock("", []slack.MixedElement{
		sessionText,
	}...))

	// Action buttons with validated block_id
	blockID := ValidateBlockID(fmt.Sprintf("perm_%s", req.MessageID))

	approveBtn := slack.NewButtonBlockElement("perm_allow", fmt.Sprintf("allow:%s:%s", sessionID, req.MessageID),
		slack.NewTextBlockObject("plain_text", "✅ Allow", true, false))
	approveBtn.Style = "primary"

	denyBtn := slack.NewButtonBlockElement("perm_deny", fmt.Sprintf("deny:%s:%s", sessionID, req.MessageID),
		slack.NewTextBlockObject("plain_text", "🚫 Deny", true, false))
	denyBtn.Style = "danger"

	blocks = append(blocks, slack.NewActionBlock(blockID, approveBtn, denyBtn))

	return blocks
}

// BuildPermissionApprovedMessage builds blocks to show after permission is approved
func (b *MessageBuilder) BuildPermissionApprovedMessage(tool, input string) []slack.Block {
	// Truncate for display
	displayInput := input
	if len(displayInput) > 200 {
		displayInput = displayInput[:197] + "..."
	}

	text := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("✅ *Permission Granted*\n\nTool: `%s`\nCommand: `%s`", tool, displayInput), false, false)
	return []slack.Block{
		slack.NewSectionBlock(text, nil, nil),
	}
}

// BuildPermissionDeniedMessage builds blocks to show after permission is denied
func (b *MessageBuilder) BuildPermissionDeniedMessage(tool, input, reason string) []slack.Block {
	// Truncate for display
	displayInput := input
	if len(displayInput) > 200 {
		displayInput = displayInput[:197] + "..."
	}

	text := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🚫 *Permission Denied*\n\nTool: `%s`\nCommand: `%s`", tool, displayInput), false, false)
	blocks := []slack.Block{
		slack.NewSectionBlock(text, nil, nil),
	}

	if reason != "" {
		reasonText := slack.NewTextBlockObject("mrkdwn", "Reason: "+reason, false, false)
		blocks = append(blocks, slack.NewContextBlock("", []slack.MixedElement{
			reasonText,
		}...))
	}

	return blocks
}

// =============================================================================
// Helper: Extract tool metadata from provider event
// =============================================================================

// ExtractToolInfo extracts tool name and input from ChatMessage metadata
func ExtractToolInfo(msg *base.ChatMessage) (toolName, input string) {
	toolName = msg.Content

	if msg.Metadata != nil {
		if name, ok := msg.Metadata["tool_name"].(string); ok {
			toolName = name
		}
		if in, ok := msg.Metadata["input"].(string); ok {
			input = in
		}
	}

	return toolName, input
}

// =============================================================================
// Constants for compatibility
// =============================================================================

// ToolResultDurationThreshold is the threshold for showing duration
const ToolResultDurationThreshold = 500 // ms

// IsLongRunningTool checks if a tool is considered long-running
func IsLongRunningTool(durationMs int64) bool {
	return durationMs > ToolResultDurationThreshold
}

// FormatDuration formats duration for display
func FormatDuration(durationMs int64) string {
	if durationMs > 1000 {
		return fmt.Sprintf("%.2fs", float64(durationMs)/1000)
	}
	return fmt.Sprintf("%dms", durationMs)
}

// ParseProviderEventType converts provider event type to base message type
func ParseProviderEventType(eventType provider.ProviderEventType) base.MessageType {
	switch eventType {
	case provider.EventTypeThinking:
		return base.MessageTypeThinking
	case provider.EventTypeToolUse:
		return base.MessageTypeToolUse
	case provider.EventTypeToolResult:
		return base.MessageTypeToolResult
	case provider.EventTypeAnswer:
		return base.MessageTypeAnswer
	case provider.EventTypeError:
		return base.MessageTypeError
	case provider.EventTypePlanMode:
		return base.MessageTypePlanMode
	case provider.EventTypeExitPlanMode:
		return base.MessageTypeExitPlanMode
	case provider.EventTypeAskUserQuestion:
		return base.MessageTypeAskUserQuestion
	default:
		return base.MessageTypeAnswer
	}
}

// TimeToSlackTimestamp converts time.Time to Slack timestamp format
func TimeToSlackTimestamp(t time.Time) string {
	return fmt.Sprintf("%d.%d", t.Unix(), t.Nanosecond()/1000000)
}
