// Package adminapi defines the shared domain for admin API servers.
// Both internal/admin (CLI) and internal/server/admin (Webhook API) import this package
// to share common types, error codes, and utilities.
//
// Architecture (SOLID/DRY):
//
//	  ┌─────────────────────┐      ┌───────────────────────────┐
//	  │   internal/admin     │      │ internal/server/admin      │
//	  │   (Admin CLI)       │      │   (Webhook API)            │
//	  │   gorilla/mux       │      │   net/http                 │
//	  │   /admin/v1/...     │      │   /api/v1/admin/...        │
//	  └──────────┬──────────┘      └─────────────┬─────────────┘
//	              │                               │
//	              └──────────┬────────────────────┘
//	                         ▼
//	           ┌─────────────────────────┐
//	           │   internal/adminapi       │
//	           │   (Shared Domain)        │
//	           │   • ErrorCode            │
//	           │   • MapSessionStatus     │
//	           │   • writeJSON/writeError │
//	           │   • Session mapping      │
//	           └─────────────────────────┘
package adminapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	intengine "github.com/hrygo/hotplex/internal/engine"
)

// ErrorCode represents a canonical admin API error code.
type ErrorCode string

const (
	ErrCodeAuthFailed          ErrorCode = "AUTH_FAILED"
	ErrCodeForbidden           ErrorCode = "FORBIDDEN"
	ErrCodeNotFound            ErrorCode = "NOT_FOUND"
	ErrCodeInvalidRequest      ErrorCode = "INVALID_REQUEST"
	ErrCodeServerError         ErrorCode = "SERVER_ERROR"
	ErrCodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrCodeEngineNotInitialized ErrorCode = "ENGINE_NOT_INITIALIZED"
)

// ErrorResponse is the standard error envelope shared by all admin APIs.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the code, message, and optional details of an error.
type ErrorDetail struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewError creates an ErrorResponse with the given code and message.
func NewError(code ErrorCode, message string) ErrorResponse {
	return ErrorResponse{Error: ErrorDetail{Code: code, Message: message}}
}

// NewErrorWithDetails creates an ErrorResponse with additional details.
func NewErrorWithDetails(code ErrorCode, message string, details map[string]interface{}) ErrorResponse {
	return ErrorResponse{Error: ErrorDetail{Code: code, Message: message, Details: details}}
}

// MapSessionStatus maps an internal engine session status to a canonical string.
func MapSessionStatus(status intengine.SessionStatus) string {
	switch status {
	case intengine.SessionStatusStarting:
		return "starting"
	case intengine.SessionStatusReady:
		return "idle"
	case intengine.SessionStatusBusy:
		return "running"
	case intengine.SessionStatusDead:
		return "dead"
	default:
		return "unknown"
	}
}

// ResolvePlatform resolves the platform from a session ID prefix.
// Session IDs follow the format: {platform}-{uuid}.
// e.g., "ws-abc123", "slack-def456", "admin-ghi789"
func ResolvePlatform(sessionID string) string {
	if sessionID == "" {
		return "unknown"
	}
	for i := 0; i < len(sessionID); i++ {
		if sessionID[i] == '-' {
			prefix := sessionID[:i]
			switch prefix {
			case "ws", "websocket":
				return "websocket"
			case "slack":
				return "slack"
			case "tg", "telegram":
				return "telegram"
			case "feishu", "lark":
				return "feishu"
			case "admin":
				return "admin"
			default:
				return prefix
			}
		}
	}
	return "unknown"
}

// SessionSummary holds fields shared by all session-listing responses.
type SessionSummary struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	LastActive  time.Time `json:"last_active"`
	Platform    string    `json:"platform"`
}

// ToSessionSummary converts an internal engine.Session to a SessionSummary.
func ToSessionSummary(s *intengine.Session) SessionSummary {
	return SessionSummary{
		ID:         s.ID,
		Status:     MapSessionStatus(s.GetStatus()),
		CreatedAt:  s.CreatedAt,
		LastActive: s.GetLastActive(),
		Platform:   ResolvePlatform(s.ID),
	}
}

// WriteJSON writes a JSON response with the given status code.
// Errors during encoding are logged but not surfaced to the caller.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("adminapi: failed to encode JSON response", "error", err)
	}
}

// WriteError writes a JSON error response.
// Errors during encoding are logged but the response is still written.
func WriteError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := NewError(code, message)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("adminapi: failed to encode error response", "error", err)
	}
}
