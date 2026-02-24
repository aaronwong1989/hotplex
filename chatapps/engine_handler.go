package chatapps

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hrygo/hotplex/engine"
	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/provider"
	"github.com/hrygo/hotplex/types"
)

// EngineHolder holds the Engine instance and configuration for ChatApps integration
type EngineHolder struct {
	engine           *engine.Engine
	logger           *slog.Logger
	adapters         *AdapterManager
	defaultWorkDir   string
	defaultTaskInstr string
}

// NewEngineHolder creates a new EngineHolder with the given options
func NewEngineHolder(opts EngineHolderOptions) (*EngineHolder, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 30 * time.Minute
	}

	engineOpts := engine.EngineOptions{
		Timeout:         opts.Timeout,
		IdleTimeout:     opts.IdleTimeout,
		Namespace:       opts.Namespace,
		PermissionMode:  opts.PermissionMode,
		AllowedTools:    opts.AllowedTools,
		DisallowedTools: opts.DisallowedTools,
		Logger:          logger,
	}

	eng, err := engine.NewEngine(engineOpts)
	if err != nil {
		return nil, fmt.Errorf("create engine: %w", err)
	}

	return &EngineHolder{
		engine:           eng,
		logger:           logger,
		adapters:         opts.Adapters,
		defaultWorkDir:   opts.DefaultWorkDir,
		defaultTaskInstr: opts.DefaultTaskInstr,
	}, nil
}

// EngineHolderOptions configures the EngineHolder
type EngineHolderOptions struct {
	Logger           *slog.Logger
	Adapters         *AdapterManager
	Timeout          time.Duration
	IdleTimeout      time.Duration
	Namespace        string
	PermissionMode   string
	AllowedTools     []string
	DisallowedTools  []string
	DefaultWorkDir   string
	DefaultTaskInstr string
}

// GetEngine returns the underlying Engine instance
func (h *EngineHolder) GetEngine() *engine.Engine {
	return h.engine
}

// GetAdapterManager returns the AdapterManager for sending messages
func (h *EngineHolder) GetAdapterManager() *AdapterManager {
	return h.adapters
}

// StreamCallback implements event.Callback to receive Engine events and forward to ChatApp
type StreamCallback struct {
	ctx       context.Context
	sessionID string
	platform  string
	adapters  *AdapterManager
	logger    *slog.Logger
	mu        sync.Mutex
	isFirst   bool
}

// NewStreamCallback creates a new StreamCallback
func NewStreamCallback(ctx context.Context, sessionID, platform string, adapters *AdapterManager, logger *slog.Logger) *StreamCallback {
	return &StreamCallback{
		ctx:       ctx,
		sessionID: sessionID,
		platform:  platform,
		adapters:  adapters,
		logger:    logger,
		isFirst:   true,
	}
}

// Handle implements event.Callback
func (c *StreamCallback) Handle(eventType string, data any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch provider.ProviderEventType(eventType) {
	case provider.EventTypeThinking:
		return c.handleThinking(data)
	case provider.EventTypeToolUse:
		return c.handleToolUse(data)
	case provider.EventTypeToolResult:
		return c.handleToolResult(data)
	case provider.EventTypeAnswer:
		return c.handleAnswer(data)
	case provider.EventTypeError:
		return c.handleError(data)
	default:
		// Check for specific engine/extended events
		if eventType == "danger_block" {
			return c.handleDangerBlock(data)
		}
		c.logger.Debug("Ignoring unknown event", "type", eventType)
	}
	return nil
}

func (c *StreamCallback) handleThinking(_ any) error {
	if c.isFirst {
		c.isFirst = false
		return c.sendMessage(string(provider.EventTypeThinking), string(provider.EventTypeThinking))
	}
	return nil
}

func (c *StreamCallback) handleToolUse(data any) error {
	msg := string(provider.EventTypeToolUse)
	if m, ok := data.(*event.EventWithMeta); ok {
		if m.Meta != nil && m.Meta.ToolName != "" {
			msg = m.Meta.ToolName
		}
		if m.EventData != "" {
			truncated := m.EventData
			if len(truncated) > 100 {
				truncated = truncated[0:100] + "..."
			}
			msg += fmt.Sprintf("\n```\n%s\n```", truncated)
		}
	}
	return c.sendMessage(msg, string(provider.EventTypeToolUse))
}

func (c *StreamCallback) handleToolResult(data any) error {
	msg := string(provider.EventTypeToolResult)
	if m, ok := data.(*event.EventWithMeta); ok {
		if m.Meta != nil && m.Meta.Status == "error" {
			msg = "error"
			if m.Meta.ErrorMsg != "" {
				msg += ": " + m.Meta.ErrorMsg
			}
		} else if m.EventData != "" {
			truncated := m.EventData
			if len(truncated) > 200 {
				truncated = truncated[0:200] + "..."
			}
			msg = fmt.Sprintf("```\n%s\n```", truncated)
		}
	}
	return c.sendMessage(msg, string(provider.EventTypeToolResult))
}

func (c *StreamCallback) handleAnswer(data any) error {
	var content string
	switch v := data.(type) {
	case string:
		content = v
	case *event.EventWithMeta:
		content = v.EventData
	default:
		content = fmt.Sprintf("%v", data)
	}

	if content == "" {
		return nil
	}

	return c.sendMessage(content, string(provider.EventTypeAnswer))
}

func (c *StreamCallback) handleError(data any) error {
	var errMsg string
	switch v := data.(type) {
	case string:
		errMsg = v
	case error:
		errMsg = v.Error()
	case *event.EventWithMeta:
		errMsg = v.EventData
		if errMsg == "" && v.Meta != nil {
			errMsg = v.Meta.ErrorMsg
		}
	default:
		errMsg = fmt.Sprintf("%v", data)
	}

	return c.sendMessage(errMsg, string(provider.EventTypeError))
}

func (c *StreamCallback) handleDangerBlock(data any) error {
	var reason string
	switch v := data.(type) {
	case string:
		reason = v
	default:
		reason = "security_block"
	}
	return c.sendMessage(reason, "security_block")
}

func (c *StreamCallback) sendMessage(content string, eventType string) error {
	if c.adapters == nil {
		c.logger.Debug("No adapters, skipping message send", "platform", c.platform)
		return nil
	}

	msg := &ChatMessage{
		Platform:  c.platform,
		SessionID: c.sessionID,
		Content:   content,
		Metadata: map[string]any{
			"stream":     true,
			"event_type": eventType,
		},
	}

	return c.adapters.SendMessage(c.ctx, c.platform, c.sessionID, msg)
}

// EngineMessageHandler implements MessageHandler and integrates with Engine
type EngineMessageHandler struct {
	engine         *engine.Engine
	adapters       *AdapterManager
	workDirFn      func(sessionID string) string
	taskInstrFn    func(sessionID string) string
	systemPromptFn func(sessionID, platform string) string
	configLoader   *ConfigLoader
	logger         *slog.Logger
}

// NewEngineMessageHandler creates a new EngineMessageHandler
func NewEngineMessageHandler(engine *engine.Engine, adapters *AdapterManager, opts ...EngineMessageHandlerOption) *EngineMessageHandler {
	h := &EngineMessageHandler{
		engine:   engine,
		adapters: adapters,
		logger:   slog.Default(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// EngineMessageHandlerOption configures the EngineMessageHandler
type EngineMessageHandlerOption func(*EngineMessageHandler)

func WithWorkDirFn(fn func(sessionID string) string) EngineMessageHandlerOption {
	return func(h *EngineMessageHandler) {
		h.workDirFn = fn
	}
}

func WithTaskInstrFn(fn func(sessionID string) string) EngineMessageHandlerOption {
	return func(h *EngineMessageHandler) {
		h.taskInstrFn = fn
	}
}

func WithLogger(logger *slog.Logger) EngineMessageHandlerOption {
	return func(h *EngineMessageHandler) {
		h.logger = logger
	}
}

func WithConfigLoader(loader *ConfigLoader) EngineMessageHandlerOption {
	return func(h *EngineMessageHandler) {
		h.configLoader = loader
	}
}

// Handle implements MessageHandler
func (h *EngineMessageHandler) Handle(ctx context.Context, msg *ChatMessage) error {
	// Determine work directory
	workDir := h.workDirFn(msg.SessionID)
	if workDir == "" {
		workDir = "/tmp/hotplex-chatapps"
	}

	// Determine task instructions
	taskInstr := h.taskInstrFn(msg.SessionID)
	if taskInstr == "" && h.configLoader != nil {
		taskInstr = h.configLoader.GetTaskInstructions(msg.Platform)
	}
	if taskInstr == "" {
		taskInstr = "You are a helpful AI assistant. Execute user commands and provide clear feedback."
	}

	// Determine system prompt
	systemPrompt := ""
	if h.systemPromptFn != nil {
		systemPrompt = h.systemPromptFn(msg.SessionID, msg.Platform)
	}
	if systemPrompt == "" && h.configLoader != nil {
		systemPrompt = h.configLoader.GetSystemPrompt(msg.Platform)
	}

	// Combine task instructions with system prompt
	fullInstructions := taskInstr
	if systemPrompt != "" {
		fullInstructions = systemPrompt + "\n\n" + taskInstr
	}

	// Build config
	cfg := &types.Config{
		WorkDir:          workDir,
		SessionID:        msg.SessionID,
		TaskInstructions: fullInstructions,
	}

	// Create stream callback
	callback := NewStreamCallback(ctx, msg.SessionID, msg.Platform, h.adapters, h.logger)
	wrappedCallback := func(eventType string, data any) error {
		return callback.Handle(eventType, data)
	}

	// Execute with Engine
	h.logger.Info("Executing prompt via Engine",
		"session_id", msg.SessionID,
		"platform", msg.Platform,
		"prompt_len", len(msg.Content))

	err := h.engine.Execute(ctx, cfg, msg.Content, wrappedCallback)
	if err != nil {
		h.logger.Error("Engine execution failed",
			"session_id", msg.SessionID,
			"error", err)

		// Send error message back
		if h.adapters != nil {
			errMsg := &ChatMessage{
				Platform:  msg.Platform,
				SessionID: msg.SessionID,
				Content:   err.Error(),
				Metadata: map[string]any{
					"event_type": string(provider.EventTypeError),
				},
			}
			if err := h.adapters.SendMessage(ctx, msg.Platform, msg.SessionID, errMsg); err != nil {
				h.logger.Error("Failed to send error message", "session_id", msg.SessionID, "error", err)
			}
		}
		return err
	}

	return nil
}
