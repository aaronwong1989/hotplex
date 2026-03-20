package slack

import (
	"fmt"
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/slack-go/slack"
)

// BuildPermissionCardBlocks builds Slack blocks for a permission request card.
// ActionID format: perm_{action}:{sessionID}:{messageID}
// Tool+command context is stored in GlobalPermissionContext for callback retrieval.
func BuildPermissionCardBlocks(botID, sessionID, msgID, tool, command, userID string) []slack.Block {
	headerText := slack.NewTextBlockObject(
		slack.PlainTextType,
		"🚨 权限请求",
		false, false,
	)
	header := slack.NewHeaderBlock(headerText)

	toolText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*工具:* `%s`", tool), false, false)
	cmdText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*命令:* ```%s```", command), false, false)
	sessionText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Session:* `%s`", sessionID), false, false)

	section := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{toolText, cmdText, sessionText},
		nil,
	)

	divider := slack.NewDividerBlock()

	// Store pending tool+command in memory so callback can retrieve it.
	// Key: "actionID -> tool:command"
	allowOnceID := MakePermissionActionID("allow_once", sessionID, msgID)
	allowAlwaysID := MakePermissionActionID("allow_always", sessionID, msgID)
	denyOnceID := MakePermissionActionID("deny_once", sessionID, msgID)
	denyAllID := MakePermissionActionID("deny_all", sessionID, msgID)

	base.GlobalPermissionContext.Store(allowOnceID, fmt.Sprintf("%s:%s", tool, command))
	base.GlobalPermissionContext.Store(allowAlwaysID, fmt.Sprintf("%s:%s", tool, command))
	base.GlobalPermissionContext.Store(denyOnceID, fmt.Sprintf("%s:%s", tool, command))
	base.GlobalPermissionContext.Store(denyAllID, fmt.Sprintf("%s:%s", tool, command))

	allowOnce := slack.NewButtonBlockElement(
		allowOnceID,
		"allow_once",
		slack.NewTextBlockObject(slack.PlainTextType, "✅ Allow Once", false, false),
	)
	allowAlways := slack.NewButtonBlockElement(
		allowAlwaysID,
		"allow_always",
		slack.NewTextBlockObject(slack.PlainTextType, "🔒 Allow Always", false, false),
	)
	denyOnce := slack.NewButtonBlockElement(
		denyOnceID,
		"deny_once",
		slack.NewTextBlockObject(slack.PlainTextType, "🚫 Deny Once", false, false),
	)
	denyAll := slack.NewButtonBlockElement(
		denyAllID,
		"deny_all",
		slack.NewTextBlockObject(slack.PlainTextType, "⛔ Deny All", false, false),
	)

	return []slack.Block{header, section, divider,
		slack.NewActionBlock("permission_actions", allowOnce, allowAlways, denyOnce, denyAll),
	}
}

// BuildPermissionResultBlocks builds the result card (updated after user decision).
func BuildPermissionResultBlocks(decision, tool, command string) []slack.Block {
	var emoji, title, description string
	switch decision {
	case "allow", "allow_once":
		emoji = "✅"
		title = "已允许（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被允许。", tool)
	case "allow_always":
		emoji = "🔒"
		title = "已永久允许"
		description = fmt.Sprintf("`%s` 已被添加到白名单，后续无需审批。", tool)
	case "deny", "deny_once":
		emoji = "🚫"
		title = "已拒绝（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被拒绝。", tool)
	case "deny_all":
		emoji = "⛔"
		title = "已永久拒绝"
		description = fmt.Sprintf("`%s` 已被添加到黑名单，后续请求将被自动拦截。", tool)
	default:
		emoji = "⏳"
		title = "已取消"
		description = "操作已取消。"
	}

	headerText := slack.NewTextBlockObject(
		slack.PlainTextType,
		fmt.Sprintf("%s %s", emoji, title),
		false, false,
	)
	header := slack.NewHeaderBlock(headerText)

	descText := slack.NewTextBlockObject(
		slack.MarkdownType,
		fmt.Sprintf("%s\n\n*命令:* ```%s```", description, command),
		false, false,
	)
	section := slack.NewSectionBlock(nil, []*slack.TextBlockObject{descText}, nil)

	return []slack.Block{header, section}
}

// BuildPermissionDeniedCard builds a read-only card for CLI permission_denials.
func BuildPermissionDeniedCard(tool, command, reason string) []slack.Block {
	headerText := slack.NewTextBlockObject(
		slack.PlainTextType,
		"⚠️ 权限被拒绝（CLI）",
		false, false,
	)
	header := slack.NewHeaderBlock(headerText)

	toolText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*工具:* `%s`", tool), false, false)
	cmdText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*命令:* ```%s```", command), false, false)
	reasonText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*原因:* %s\n\n请联系管理员调整权限配置。", reason), false, false)

	section := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{toolText, cmdText, reasonText},
		nil,
	)

	return []slack.Block{header, section}
}

// MakePermissionActionID constructs the action ID for permission buttons.
// Format: perm_{action}:{sessionID}:{messageID}
func MakePermissionActionID(action, sessionID, msgID string) string {
	return fmt.Sprintf("perm_%s:%s:%s", action, sessionID, msgID)
}

// ParsePermissionActionID parses the action ID back into components.
func ParsePermissionActionID(actionID string) (action, sessionID, msgID string, ok bool) {
	parts := strings.Split(actionID, ":")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
