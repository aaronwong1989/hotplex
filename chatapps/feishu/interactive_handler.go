package feishu

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/engine"
)

// InteractiveEvent represents a Feishu interactive event
type InteractiveEvent struct {
	Header *InteractiveHeader    `json:"header"`
	Event  *InteractiveEventData `json:"event"`
	Token  string                `json:"token"`
}

// InteractiveHeader represents the event header
type InteractiveHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

// InteractiveEventData represents the event data
type InteractiveEventData struct {
	Message *InteractiveMessage `json:"message"`
	User    *InteractiveUser    `json:"user"`
	Action  *InteractiveAction  `json:"action"`
}

// InteractiveMessage represents the message in the event
type InteractiveMessage struct {
	MessageID   string `json:"message_id"`
	ChatID      string `json:"chat_id"`
	MessageType string `json:"message_type"`
}

// InteractiveUser represents the user who triggered the event
type InteractiveUser struct {
	UserID string `json:"user_id"`
}

// InteractiveAction represents the button action
type InteractiveAction struct {
	Value string `json:"value"`
	Tag   string `json:"tag"`
}

// ActionValue represents the decoded action value from button click
type ActionValue struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id,omitempty"`
}

// InteractiveHandler handles Feishu interactive card callbacks
type InteractiveHandler struct {
	eventHandler *EventHandler
	adapter      *Adapter
	logger       *slog.Logger
	eng          *engine.Engine
	botID        string
}

// NewInteractiveHandler creates a new interactive handler
func NewInteractiveHandler(adapter *Adapter) *InteractiveHandler {
	eh := NewEventHandler(adapter)
	return &InteractiveHandler{
		eventHandler: eh,
		adapter:      adapter,
		logger:       eh.logger,
	}
}

// SetEngine injects the engine and botID into the handler.
func (h *InteractiveHandler) SetEngine(eng *engine.Engine, botID string) {
	h.eng = eng
	h.botID = botID
}

// HandleInteractive handles incoming interactive events
func (h *InteractiveHandler) HandleInteractive(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := base.ReadBody(r)
	if err != nil {
		h.logger.Error("Read body failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Verify signature
	if err := h.adapter.verifySignature(r, body); err != nil {
		h.logger.Warn("Invalid signature", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse event
	var event InteractiveEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error("Parse event failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Handle URL verification
	if event.Header.EventType == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"challenge":"` + event.Token + `"}`))
		return
	}

	// Handle interactive message reply
	if event.Header.EventType == "im.message.reply" {
		// Pass request context to enable cancellation when HTTP connection drops
		h.handleButtonCallbackInternal(r.Context(), &event)
		h.eventHandler.WriteOKResponse(w)
		return
	}

	// Unknown event type
	h.logger.Debug("Ignoring unknown event type", "type", event.Header.EventType)
	h.eventHandler.WriteOKResponse(w)
}

// handleButtonCallbackInternal handles button callback without HTTP response.
// Passes request context for proper cancellation of async operations.
func (h *InteractiveHandler) handleButtonCallbackInternal(ctx context.Context, event *InteractiveEvent) {
	if event.Event.Action == nil || event.Event.Action.Value == "" {
		h.logger.Warn("Missing action value")
		return
	}

	h.logger.Info("Button callback received",
		"value", event.Event.Action.Value,
		"user_id", event.Event.User.UserID,
	)

	// Decode action value - try ActionValueWithContext first for new perm_* format
	var av ActionValueWithContext
	if err := json.Unmarshal([]byte(event.Event.Action.Value), &av); err != nil {
		h.logger.Error("Decode action value failed", "error", err)
		return
	}

	// Route based on action type
	switch av.Action {
	case "permission_request":
		// Legacy permission flow (old format)
		avSimple := ActionValue{SessionID: av.SessionID, MessageID: av.MessageID}
		h.handlePermissionCallbackInternal(ctx, event, &avSimple)
	case "perm_allow_once", "perm_allow_always", "perm_deny_once", "perm_deny_all":
		// New 4-button permission flow
		h.handleNewPermissionCallback(ctx, event, &av)
	default:
		h.logger.Warn("Unknown action type", "action", av.Action)
	}
}

// handlePermissionCallbackInternal handles the legacy permission approval flow.
// Kept for backwards compatibility with old permission_request card format.
func (h *InteractiveHandler) handlePermissionCallbackInternal(ctx context.Context, event *InteractiveEvent, _ *ActionValue) {
	chatID := event.Event.Message.ChatID
	if chatID == "" {
		h.logger.Error("Missing chat_id in event")
		return
	}

	resultText := "✅ 已允许执行"

	token, err := h.adapter.GetAppTokenWithContext(ctx)
	if err != nil {
		h.logger.Error("Get token failed", "error", err)
		return
	}

	_, err = h.adapter.client.SendTextMessage(ctx, token, chatID, resultText)
	if err != nil {
		h.logger.Error("Send confirmation failed", "error", err)
	}

	// Update permission card async with request context for cancellation
	go func() {
		if err := h.UpdatePermissionCard(ctx, event.Event.Message.MessageID, chatID, "approved"); err != nil {
			h.logger.Error("Update permission card failed", "error", err)
		}
	}()
}

// handleNewPermissionCallback handles the new 4-button permission flow.
// Action values: perm_allow_once, perm_allow_always, perm_deny_once, perm_deny_all.
func (h *InteractiveHandler) handleNewPermissionCallback(ctx context.Context, event *InteractiveEvent, av *ActionValueWithContext) {
	userID := event.Event.User.UserID
	chatID := event.Event.Message.ChatID
	sessionID := av.SessionID
	msgID := av.MessageID
	toolCmd := av.Tool + ":" + av.Command

	h.logger.Info("New permission callback received",
		"user_id", userID,
		"chat_id", chatID,
		"action", av.Action,
		"session_id", sessionID,
	)

	// Extract action type (strip "perm_" prefix)
	actionType := strings.TrimPrefix(av.Action, "perm_")

	// Determine behavior (allow/deny)
	behavior := "deny"
	if actionType == "allow_once" || actionType == "allow_always" {
		behavior = "allow"
	}

	// CRITICAL: Resolve pending permission registry BEFORE WriteInput.
	// This unblocks the goroutine in engine_handler.go that waits for user decision.
	if h.eng != nil {
		decision := base.PermissionDecision{Allow: behavior == "allow", Reason: actionType}
		if resolved := base.GlobalPermissionRegistry.ResolvePermission(sessionID, decision); !resolved {
			h.logger.Warn("No pending permission found in registry, WriteInput may not be received",
				"session_id", sessionID)
		}
	}

	// Send to engine via session input
	if h.eng != nil {
		if sess, ok := h.eng.GetSession(sessionID); ok {
			response := map[string]any{
				"type":       "permission_response",
				"message_id": msgID,
				"behavior":   behavior,
			}
			_ = sess.WriteInput(response)
		}
	}

	// Persist whitelist/blacklist for _always/_all variants
	if h.eng != nil && (actionType == "allow_always" || actionType == "deny_all") && toolCmd != ":" {
		pm := h.eng.PermissionMatcher()
		if pm != nil {
			if actionType == "allow_always" {
				_ = pm.AddWhitelist(h.botID, toolCmd, userID)
				// Add tool to AllowedTools for future sessions
				allowedTools := h.eng.GetAllowedTools()
				if !slices.Contains(allowedTools, av.Tool) {
					h.eng.SetAllowedTools(append(allowedTools, av.Tool))
				}
			} else {
				_ = pm.AddBlacklist(h.botID, toolCmd, userID)
			}
		}
	}

	// Send result card asynchronously with a bounded timeout.
	// Derived from request context so it's cancelled if HTTP connection drops,
	// but with its own timeout so it doesn't block on the request lifetime.
	go func() {
		ctxChild, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		token, err := h.adapter.GetAppTokenWithContext(ctxChild)
		if err != nil {
			h.logger.Error("Get token failed", "error", err)
			return
		}

		resultCard := BuildPermissionResultCard(actionType, av.Tool, av.Command)
		cardJSON, err := json.Marshal(resultCard)
		if err != nil {
			h.logger.Error("Marshal result card failed", "error", err)
			return
		}

		_, err = h.adapter.client.SendMessage(ctxChild, token, chatID, "interactive", map[string]string{
			"config": string(cardJSON),
		})
		if err != nil {
			h.logger.Error("Send result card failed", "error", err)
		}
	}()
}

// UpdatePermissionCard updates a permission card with the result
func (h *InteractiveHandler) UpdatePermissionCard(ctx context.Context, messageID, chatID, result string) error {
	token, err := h.adapter.GetAppTokenWithContext(ctx)
	if err != nil {
		return err
	}

	var cardTemplate, title, description string
	switch strings.ToLower(result) {
	case "approved", "allow":
		cardTemplate = CardTemplateGreen
		title = "✅ 已允许"
		description = "权限已批准，操作将继续执行"
	case "denied", "deny":
		cardTemplate = CardTemplateRed
		title = "❌ 已拒绝"
		description = "权限已拒绝，操作已取消"
	default:
		cardTemplate = Grey
		title = "⏸️ 已取消"
		description = "操作已取消"
	}

	resultCard := &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: cardTemplate,
			Title: &Text{
				Content: title,
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: description,
					Tag:     TextTypeLarkMD,
				},
			},
			{
				Type: ElementNote,
				Elements: []CardElement{
					{
						Type: ElementMarkdown,
						Text: &Text{
							Content: "操作时间：" + time.Now().Format("2006-01-02 15:04:05"),
							Tag:     TextTypeLarkMD,
						},
					},
				},
			},
		},
	}

	cardJSON, err := json.Marshal(resultCard)
	if err != nil {
		return err
	}

	_, err = h.adapter.client.SendMessage(ctx, token, chatID, "interactive", map[string]string{
		"config": string(cardJSON),
	})
	if err != nil {
		return err
	}

	h.logger.Info("Permission card updated",
		"message_id", messageID,
		"chat_id", chatID,
		"result", result,
	)

	return nil
}

// EncodeActionValue encodes an action value for button callback
func EncodeActionValue(action, sessionID, messageID string) (string, error) {
	value := ActionValue{
		Action:    action,
		SessionID: sessionID,
		MessageID: messageID,
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// DecodeActionValue decodes an action value from button callback
func DecodeActionValue(value string) (*ActionValue, error) {
	var actionValue ActionValue
	if err := json.Unmarshal([]byte(value), &actionValue); err != nil {
		return nil, err
	}

	return &actionValue, nil
}
