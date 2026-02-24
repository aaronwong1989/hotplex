package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

type Adapter struct {
	*base.Adapter
	config          Config
	eventPath       string
	interactivePath string
	sender          *base.SenderWithMutex
	webhook         *base.WebhookRunner
	socketMode      *SocketModeConnection
}

func NewAdapter(config Config, logger *slog.Logger, opts ...base.AdapterOption) *Adapter {
	// Validate config
	if err := config.Validate(); err != nil {
		logger.Error("Invalid Slack config", "error", err)
	}

	a := &Adapter{
		config:          config,
		eventPath:       "/webhook/events",
		interactivePath: "/webhook/interactive",
		sender:          base.NewSenderWithMutex(),
		webhook:         base.NewWebhookRunner(logger),
	}

	// Initialize Socket Mode if configured
	if config.IsSocketMode() {
		a.socketMode = NewSocketModeConnection(SocketModeConfig{
			AppToken: config.AppToken,
			BotToken: config.BotToken,
		}, logger)

		// Register message handler
		a.socketMode.RegisterHandler("message", a.handleSocketModeEvent)
	}

	handlers := make(map[string]http.HandlerFunc)

	// Only register HTTP handlers if NOT in Socket Mode
	if !config.IsSocketMode() {
		handlers[a.eventPath] = a.handleEvent
		handlers[a.interactivePath] = a.handleInteractive
	}

	// Build HTTP handler map
	for path, handler := range handlers {
		opts = append(opts, base.WithHTTPHandler(path, handler))
	}

	a.Adapter = base.NewAdapter("slack", base.Config{
		ServerAddr:   config.ServerAddr,
		SystemPrompt: config.SystemPrompt,
	}, logger, opts...)

	// Set default sender if BotToken is configured
	if config.BotToken != "" {
		a.sender.SetSender(a.defaultSender)
	}

	return a
}

func (a *Adapter) SendMessage(ctx context.Context, sessionID string, msg *base.ChatMessage) error {
	return a.sender.SendMessage(ctx, sessionID, msg)
}

func (a *Adapter) SetSender(fn func(ctx context.Context, sessionID string, msg *base.ChatMessage) error) {
	a.sender.SetSender(fn)
}

// defaultSender sends message via Slack API
func (a *Adapter) defaultSender(ctx context.Context, sessionID string, msg *base.ChatMessage) error {
	if a.config.BotToken == "" {
		return fmt.Errorf("slack bot token not configured")
	}

	// Extract channel_id from session or message metadata
	channelID := a.extractChannelID(sessionID, msg)
	if channelID == "" {
		return fmt.Errorf("channel_id not found in session")
	}

	// Extract thread_ts from metadata if present
	threadTS := ""
	if msg.Metadata != nil {
		if ts, ok := msg.Metadata["thread_ts"].(string); ok {
			threadTS = ts
		}
	}

	return a.SendToChannel(ctx, channelID, msg.Content, threadTS)
}

// extractChannelID extracts channel_id from session or message metadata
func (a *Adapter) extractChannelID(sessionID string, msg *base.ChatMessage) string {
	if msg.Metadata == nil {
		return ""
	}
	if channelID, ok := msg.Metadata["channel_id"].(string); ok {
		return channelID
	}
	return ""
}

type Event struct {
	Token     string          `json:"token"`
	TeamID    string          `json:"team_id"`
	APIAppID  string          `json:"api_app_id"`
	Type      string          `json:"type"`
	EventID   string          `json:"event_id"`
	EventTime int64           `json:"event_time"`
	Event     json.RawMessage `json:"event"`
	Challenge string          `json:"challenge"`
}

type MessageEvent struct {
	Type        string `json:"type"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	User        string `json:"user"`
	Text        string `json:"text"`
	TS          string `json:"ts"`
	EventTS     string `json:"event_ts"`
	BotID       string `json:"bot_id,omitempty"`
	SubType     string `json:"subtype,omitempty"`
	ThreadTS    string `json:"thread_ts,omitempty"`      // Thread identifier
	ParentUser  string `json:"parent_user_id,omitempty"` // Parent message user
}

func (a *Adapter) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.Logger().Error("Read body failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if a.config.SigningSecret != "" {
		signature := r.Header.Get("X-Slack-Signature")
		timestamp := r.Header.Get("X-Slack-Request-Timestamp")
		if signature == "" || timestamp == "" || !a.verifySignature(body, timestamp, signature) {
			a.Logger().Warn("Invalid signature")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		a.Logger().Error("Parse event failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if event.Challenge != "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(event.Challenge))
		return
	}

	if event.Token != a.config.BotToken && event.Token != a.config.AppToken {
		a.Logger().Warn("Invalid token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if event.Type == "event_callback" {
		a.handleEventCallback(r.Context(), event.Event)
	}

	w.WriteHeader(http.StatusOK)
}

func (a *Adapter) handleEventCallback(ctx context.Context, eventData json.RawMessage) {
	var msgEvent MessageEvent
	if err := json.Unmarshal(eventData, &msgEvent); err != nil {
		a.Logger().Error("Parse message event failed", "error", err)
		return
	}

	if msgEvent.BotID != "" || (msgEvent.SubType != "" && msgEvent.SubType != "message_changed") {
		return
	}

	if msgEvent.Text == "" {
		return
	}

	sessionID := a.GetOrCreateSession(msgEvent.Channel+":"+msgEvent.User, msgEvent.User)

	msg := &base.ChatMessage{
		Platform:  "slack",
		SessionID: sessionID,
		UserID:    msgEvent.User,
		Content:   msgEvent.Text,
		MessageID: msgEvent.TS,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"channel_id":   msgEvent.Channel,
			"channel_type": msgEvent.ChannelType,
		},
	}

	// Add thread info if present
	if msgEvent.ThreadTS != "" {
		msg.Metadata["thread_ts"] = msgEvent.ThreadTS
	}

	a.webhook.Run(ctx, a.Handler(), msg)

	a.webhook.Run(ctx, a.Handler(), msg)
}

// Stop waits for pending webhook goroutines to complete
func (a *Adapter) Stop() error {
	// Stop Socket Mode if active
	if a.socketMode != nil {
		_ = a.socketMode.Stop()
	}

	a.webhook.Stop()
	return a.Adapter.Stop()
}

// Start starts the adapter (overrides base.Adapter.Start to support Socket Mode)
func (a *Adapter) Start(ctx context.Context) error {
	// Start Socket Mode if configured
	if a.socketMode != nil {
		if err := a.socketMode.Start(ctx); err != nil {
			return fmt.Errorf("failed to start socket mode: %w", err)
		}
	}

	return a.Adapter.Start(ctx)
}

// handleSocketModeEvent handles incoming events from Socket Mode
func (a *Adapter) handleSocketModeEvent(eventType string, data json.RawMessage) {
	var msgEvent MessageEvent
	if err := json.Unmarshal(data, &msgEvent); err != nil {
		a.Logger().Error("Parse socket mode message event failed", "error", err)
		return
	}

	// Skip bot messages and certain subtypes
	if msgEvent.BotID != "" || (msgEvent.SubType != "" && msgEvent.SubType != "message_changed") {
		return
	}

	if msgEvent.Text == "" {
		return
	}

	sessionID := a.GetOrCreateSession(msgEvent.Channel+":"+msgEvent.User, msgEvent.User)

	msg := &base.ChatMessage{
		Platform:  "slack",
		SessionID: sessionID,
		UserID:    msgEvent.User,
		Content:   msgEvent.Text,
		MessageID: msgEvent.TS,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"channel_id":   msgEvent.Channel,
			"channel_type": msgEvent.ChannelType,
		},
	}

	// Add thread info if present
	if msgEvent.ThreadTS != "" {
		msg.Metadata["thread_ts"] = msgEvent.ThreadTS
	}

	a.webhook.Run(context.Background(), a.Handler(), msg)
}

func (a *Adapter) handleInteractive(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.Logger().Error("Read body failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	a.Logger().Debug("Interactive payload received", "body", string(body))

	w.WriteHeader(http.StatusOK)
}

func (a *Adapter) verifySignature(body []byte, timestamp, signature string) bool {
	parsedTS := strings.TrimPrefix(timestamp, "v0=")
	var ts int64
	if _, err := fmt.Sscanf(parsedTS, "%d", &ts); err != nil {
		a.Logger().Warn("Failed to parse timestamp", "timestamp", parsedTS)
		return false
	}

	now := time.Now().Unix()
	if now-ts > 60*5 {
		a.Logger().Warn("Timestamp too old")
		return false
	}

	baseString := fmt.Sprintf("v0:%s:%s", parsedTS, string(body))
	h := hmac.New(sha256.New, []byte(a.config.SigningSecret))
	h.Write([]byte(baseString))
	signatureComputed := "v0=" + hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(signatureComputed), []byte(signature))
}

func (a *Adapter) SendToChannel(ctx context.Context, channelID, text, threadTS string) error {
	// Retry configuration for rate limiting
	retryConfig := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}

	return retryWithBackoff(ctx, retryConfig, func() error {
		return a.sendToChannelOnce(ctx, channelID, text, threadTS)
	})
}

// sendToChannelOnce sends a single message to Slack (without retry)
func (a *Adapter) sendToChannelOnce(ctx context.Context, channelID, text, threadTS string) error {
	payload := map[string]any{
		"channel": channelID,
		"text":    text,
	}

	// Add thread_ts if provided to reply in thread
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.config.BotToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for rate limit (429)
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("rate limited: 429")
	}

	if resp.StatusCode >= 400 {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("send failed with status %d (failed to read body: %v)", resp.StatusCode, err)
		}
		return fmt.Errorf("send failed: %d %s", resp.StatusCode, string(respBody))
	}

	return nil
}
