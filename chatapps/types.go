package chatapps

import (
	"context"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/engine"
	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/internal/permission"
	"github.com/hrygo/hotplex/types"
)

// EngineExecutor handles execution operations (ISP: execution-focused interface)
// Use this interface when you only need to execute prompts and manage configs
type EngineExecutor interface {
	Execute(ctx context.Context, cfg *types.Config, prompt string, callback event.Callback) error
	ValidateConfig(cfg *types.Config) error
	GetOptions() engine.EngineOptions
}

// SessionManager handles session lifecycle (ISP: session management interface)
// Use this interface when you only need session operations
type SessionManager interface {
	GetSession(sessionID string) (Session, bool)
	GetSessionStats(sessionID string) *SessionStats
	StopSession(sessionID string, reason string) error
	Close() error
}

// DangerController handles security operations (ISP: security interface)
// Use this interface when you only need danger detection and permission control
type DangerController interface {
	CheckDanger(prompt string) (blocked bool, operation, reason string)
	SetDangerAllowPaths(paths []string)
	SetDangerBypassEnabled(token string, enabled bool) error
	PermissionMatcher() *permission.PermissionMatcher
}

// ToolController handles tool permissions (ISP: tool management interface)
// Use this interface when you only need tool allowlist/blocklist management
type ToolController interface {
	SetAllowedTools(tools []string)
	SetDisallowedTools(tools []string)
	GetAllowedTools() []string
	GetDisallowedTools() []string
}

// Engine abstracts the engine functionality for dependency inversion
// Combines all specialized interfaces for backward compatibility
// New code should prefer specialized interfaces (EngineExecutor, SessionManager, etc.)
type Engine interface {
	EngineExecutor
	SessionManager
	DangerController
	ToolController
}

// Session abstracts session state and operations
type Session interface {
	ID() string
	Status() string
	CreatedAt() time.Time
	IsResumed() bool
}

// SessionStats holds session statistics
type SessionStats struct {
	SessionID     string
	Status        string
	TotalTokens   int64
	InputTokens   int64
	OutputTokens  int64
	CacheRead     int64
	CacheWrite    int64
	TotalCost     float64
	Duration      time.Duration
	ToolCallCount int
	ErrorCount    int
}

// Re-export interfaces from base for convenience
type (
	MessageOperations = base.MessageOperations
	SessionOperations = base.SessionOperations
)

type ParseMode = base.ParseMode

const (
	ParseModeNone     = base.ParseModeNone
	ParseModeMarkdown = base.ParseModeMarkdown
	ParseModeHTML     = base.ParseModeHTML
)

type ChatMessage = base.ChatMessage
type RichContent = base.RichContent
type Attachment = base.Attachment
type ChatAdapter = base.ChatAdapter
type MessageHandler = base.MessageHandler

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type SlackBlock map[string]any

type StreamHandler func(ctx context.Context, sessionID string, chunk string, isFinal bool) error

type StreamAdapter interface {
	ChatAdapter
	SendStreamMessage(ctx context.Context, sessionID string, msg *ChatMessage) (StreamHandler, error)
	UpdateMessage(ctx context.Context, sessionID, messageID string, msg *ChatMessage) error
}

func NewChatMessage(platform, sessionID, userID, content string) *ChatMessage {
	return &ChatMessage{
		Platform:  platform,
		SessionID: sessionID,
		UserID:    userID,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}
