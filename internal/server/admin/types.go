package admin

import (
	"time"

	"github.com/hrygo/hotplex/engine"
	intengine "github.com/hrygo/hotplex/internal/engine"
	"github.com/hrygo/hotplex/internal/adminapi"
)

// AdminSession represents a session in the admin API.
// Status is mapped from engine.SessionStatus:
//   - SessionStatusStarting -> "starting"
//   - SessionStatusReady -> "idle"
//   - SessionStatusBusy -> "running"
//   - SessionStatusDead -> "dead"
type AdminSession struct {
	SessionID    string            `json:"session_id"`
	Status       string            `json:"status"`
	Platform     string            `json:"platform"`
	CreatedAt    time.Time         `json:"created_at"`
	LastActiveAt time.Time         `json:"last_active_at"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// AdminSessionStats represents execution statistics aligned with engine.SessionStats.
type AdminSessionStats struct {
	SessionID             string   `json:"session_id"`
	TotalDurationMs      int64    `json:"total_duration_ms"`
	ThinkingDurationMs   int64    `json:"thinking_duration_ms"`
	ToolDurationMs       int64    `json:"tool_duration_ms"`
	GenerationDurationMs int64    `json:"generation_duration_ms"`
	InputTokens          int64    `json:"input_tokens"`
	OutputTokens         int64    `json:"output_tokens"`
	CacheReadTokens      int64    `json:"cache_read_tokens"`
	CacheWriteTokens     int64    `json:"cache_write_tokens"`
	ToolCallCount        int64    `json:"tool_call_count"`
	ToolsUsed            []string `json:"tools_used,omitempty"`
	FilesModified        int64    `json:"files_modified"`
	FilePaths            []string `json:"file_paths,omitempty"`
	ErrorCount           int64    `json:"error_count"`
}

// AdminEvent represents an audit event in the ring buffer.
type AdminEvent struct {
	EventID   string                 `json:"event_id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Actor     string                 `json:"actor,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// AdminHealth represents the overall system health.
type AdminHealth struct {
	Status     string                     `json:"status"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
	Subsystems map[string]SubsystemHealth `json:"subsystems"`
}

// SubsystemHealth represents the health of a single subsystem.
type SubsystemHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Re-export shared error types from adminapi for convenience.
type (
	ErrorCode     = adminapi.ErrorCode
	ErrorResponse = adminapi.ErrorResponse
	ErrorDetail   = adminapi.ErrorDetail
)

// Error code constants re-exported from adminapi.
const (
	ErrCodeAuthFailed          = adminapi.ErrCodeAuthFailed
	ErrCodeForbidden           = adminapi.ErrCodeForbidden
	ErrCodeNotFound            = adminapi.ErrCodeNotFound
	ErrCodeInvalidRequest      = adminapi.ErrCodeInvalidRequest
	ErrCodeServerError         = adminapi.ErrCodeServerError
	ErrCodeUnauthorized        = adminapi.ErrCodeUnauthorized
	ErrCodeEngineNotInitialized = adminapi.ErrCodeEngineNotInitialized
)

// AdminError is an alias for the shared ErrorResponse.
type AdminError = ErrorResponse

// BatchStopRequest is the request body for batch session stop.
type BatchStopRequest struct {
	SessionIDs []string `json:"session_ids"`
	Reason    string   `json:"reason"`
}

// BatchStopResponse is the response for batch session stop.
type BatchStopResponse struct {
	Stopped   []string          `json:"stopped"`
	NotFound []string           `json:"not_found"`
	Failed    []BatchStopFailed `json:"failed"`
}

// BatchStopFailed represents a failed session stop.
type BatchStopFailed struct {
	SessionID string `json:"session_id"`
	Error     string `json:"error"`
}

// StopRequest is the request body for single session stop.
type StopRequest struct {
	Reason string `json:"reason"`
}

// StopResponse is the response for single session stop.
type StopResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// DrainRequest is the request body for entering drain mode.
type DrainRequest struct {
	Message string `json:"message"`
}

// DrainResponse is the response for drain mode operations.
type DrainResponse struct {
	Status         string `json:"status"`
	ActiveSessions int    `json:"active_sessions,omitempty"`
	Message        string `json:"message,omitempty"`
}

// SessionsResponse is the response for listing sessions.
type SessionsResponse struct {
	Sessions []AdminSession `json:"sessions"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
}

// EventsResponse is the response for listing events.
type EventsResponse struct {
	Events     []AdminEvent `json:"events"`
	NextCursor string       `json:"next_cursor,omitempty"`
	Total      int64        `json:"total"`
}

// TranscriptResponse is the response for session transcript.
type TranscriptResponse struct {
	SessionID string            `json:"session_id"`
	Messages  []TranscriptMsg `json:"messages"`
}

// TranscriptMsg represents a single message in a transcript.
type TranscriptMsg struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

// ToolsResponse is the response for tools list endpoints.
type ToolsResponse struct {
	Tools []string `json:"tools"`
	Source string  `json:"source"`
}

// ConfigResponse is the response for config endpoint.
type ConfigResponse struct {
	Config any `json:"config"`
}

// NewAdminError creates a new AdminError with the given code and message.
// Deprecated: Use adminapi.NewError directly.
func NewAdminError(code ErrorCode, message string) AdminError {
	return adminapi.NewError(code, message)
}

// NewAdminErrorWithDetails creates a new AdminError with details.
// Deprecated: Use adminapi.NewErrorWithDetails directly.
func NewAdminErrorWithDetails(code ErrorCode, message string, details map[string]interface{}) AdminError {
	return adminapi.NewErrorWithDetails(code, message, details)
}

// MapSessionToAdminSession converts an internal engine.Session to AdminSession.
func MapSessionToAdminSession(s *intengine.Session) AdminSession {
	return AdminSession{
		SessionID:     s.ID,
		Status:        adminapi.MapSessionStatus(s.GetStatus()),
		Platform:      adminapi.ResolvePlatform(s.ID),
		CreatedAt:     s.CreatedAt,
		LastActiveAt: s.GetLastActive(),
	}
}

// MapSessionStatsToAdminStats converts engine.SessionStats to AdminSessionStats.
func MapSessionStatsToAdminStats(stats *engine.SessionStats) AdminSessionStats {
	if stats == nil {
		return AdminSessionStats{}
	}

	// Collect tools used from the map
	toolsUsed := make([]string, 0, len(stats.ToolsUsed))
	for tool := range stats.ToolsUsed {
		toolsUsed = append(toolsUsed, tool)
	}

	return AdminSessionStats{
		SessionID:             stats.SessionID,
		TotalDurationMs:       stats.TotalDurationMs,
		ThinkingDurationMs:    stats.ThinkingDurationMs,
		ToolDurationMs:        stats.ToolDurationMs,
		GenerationDurationMs:   stats.GenerationDurationMs,
		InputTokens:           int64(stats.InputTokens),
		OutputTokens:         int64(stats.OutputTokens),
		CacheReadTokens:      int64(stats.CacheReadTokens),
		CacheWriteTokens:     int64(stats.CacheWriteTokens),
		ToolCallCount:        int64(stats.ToolCallCount),
		ToolsUsed:            toolsUsed,
		FilesModified:        int64(stats.FilesModified),
		FilePaths:            stats.FilePaths,
	}
}

