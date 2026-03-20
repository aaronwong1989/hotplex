package feishu

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// ActionValueWithContext extends ActionValue with tool+command for permission cards.
type ActionValueWithContext struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id,omitempty"`
	Tool      string `json:"tool"`
	Command   string `json:"command"`
}

// EncodeActionValueWithContext encodes an action value with tool+command context for persistence.
func EncodeActionValueWithContext(action, sessionID, msgID, tool, command string) (string, error) {
	value := ActionValueWithContext{
		Action:    action,
		SessionID: sessionID,
		MessageID: msgID,
		Tool:      tool,
		Command:   command,
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// Note: For Feishu, the tool+command is carried in the JSON payload itself,
	// so DecodeActionValueWithContext can extract it directly without GlobalPermissionContext.
	return string(data), nil
}

// DecodeActionValueWithContext decodes an action value with context.
func DecodeActionValueWithContext(value string) (*ActionValueWithContext, error) {
	var av ActionValueWithContext
	if err := json.Unmarshal([]byte(value), &av); err != nil {
		return nil, err
	}
	return &av, nil
}

// BuildPermissionCard builds a Feishu interactive card for permission request.
// Uses base.PermissionCardData (shared with Slack). Tool+command context is embedded
// in the button value JSON so the callback can retrieve it without GlobalPermissionContext.
func BuildPermissionCard(data base.PermissionCardData) *CardTemplate {
	// Encode action with tool+command context for persistence lookup
	allowOnceValue, _ := EncodeActionValueWithContext("perm_allow_once", data.SessionID, data.MessageID, data.Tool, data.Command)
	allowAlwaysValue, _ := EncodeActionValueWithContext("perm_allow_always", data.SessionID, data.MessageID, data.Tool, data.Command)
	denyOnceValue, _ := EncodeActionValueWithContext("perm_deny_once", data.SessionID, data.MessageID, data.Tool, data.Command)
	denyAllValue, _ := EncodeActionValueWithContext("perm_deny_all", data.SessionID, data.MessageID, data.Tool, data.Command)

	return &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: CardTemplateOrange,
			Title: &Text{
				Content: "🚨 权限请求",
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: fmt.Sprintf("**工具:** `%s`\n\n**命令:**\n```%s```\n\n**Session:** `%s`", data.Tool, data.Command, data.SessionID),
					Tag:     TextTypeLarkMD,
				},
			},
			// Divider using plain text line
			{
				Type: ElementDiv,
				Text: &Text{
					Content: "─────────────────",
					Tag:     TextTypePlainText,
				},
			},
			{
				Type: ElementNote,
				Elements: []CardElement{
					{
						Type: ElementMarkdown,
						Text: &Text{
							Content: fmt.Sprintf("操作人: %s  |  %s", data.UserID, time.Now().Format("15:04:05")),
							Tag:     TextTypeLarkMD,
						},
					},
				},
			},
			// Buttons go inside a CardElement with Type: ElementAction
			{
				Type: ElementAction,
				Actions: []CardAction{
					{
						Type: ButtonTypePrimary,
						Text: &Text{
							Content: "✅ Allow Once",
							Tag:     TextTypePlainText,
						},
						Value: allowOnceValue,
					},
					{
						Type: ButtonTypePrimary,
						Text: &Text{
							Content: "🔒 Allow Always",
							Tag:     TextTypePlainText,
						},
						Value: allowAlwaysValue,
					},
				},
			},
			{
				Type: ElementAction,
				Actions: []CardAction{
					{
						Type: ButtonTypeDanger,
						Text: &Text{
							Content: "🚫 Deny Once",
							Tag:     TextTypePlainText,
						},
						Value: denyOnceValue,
					},
					{
						Type: ButtonTypeDanger,
						Text: &Text{
							Content: "⛔ Deny All",
							Tag:     TextTypePlainText,
						},
						Value: denyAllValue,
					},
				},
			},
		},
	}
}

// BuildPermissionResultCard builds the result card after user decision.
func BuildPermissionResultCard(decision, tool, command string) *CardTemplate {
	var template, title, description string
	switch decision {
	case "allow", "allow_once":
		template = CardTemplateGreen
		title = "✅ 已允许（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被允许。", tool)
	case "allow_always":
		template = CardTemplateGreen
		title = "🔒 已永久允许"
		description = fmt.Sprintf("`%s` 已被添加到白名单，后续无需审批。", tool)
	case "deny", "deny_once":
		template = CardTemplateRed
		title = "🚫 已拒绝（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被拒绝。", tool)
	case "deny_all":
		template = CardTemplateRed
		title = "⛔ 已永久拒绝"
		description = fmt.Sprintf("`%s` 已被添加到黑名单，后续请求将被自动拦截。", tool)
	default:
		template = CardTemplateOrange
		title = "⏳ 已取消"
		description = "操作已取消。"
	}

	return &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: template,
			Title: &Text{
				Content: title,
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: fmt.Sprintf("%s\n\n**命令:**\n```%s```", description, command),
					Tag:     TextTypeLarkMD,
				},
			},
			{
				Type: ElementNote,
				Elements: []CardElement{
					{
						Type: ElementMarkdown,
						Text: &Text{
							Content: "决策时间：" + time.Now().Format("2006-01-02 15:04:05"),
							Tag:     TextTypeLarkMD,
						},
					},
				},
			},
		},
	}
}

// BuildPermissionDeniedCard builds a read-only card for CLI permission_denials.
func BuildPermissionDeniedCard(tool, command, reason string) *CardTemplate {
	return &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: CardTemplateOrange,
			Title: &Text{
				Content: "⚠️ 权限被拒绝（CLI）",
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: fmt.Sprintf("**工具:** `%s`\n\n**命令:**\n```%s```\n\n**原因:** %s\n\n请联系管理员调整权限配置。", tool, command, reason),
					Tag:     TextTypeLarkMD,
				},
			},
		},
	}
}
