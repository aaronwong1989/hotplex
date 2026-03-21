package admin

import "time"

// SessionInfo represents a session in admin API responses.
type SessionInfo struct {
	ID             string    `json:"id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	LastActive     time.Time `json:"last_active"`
	Provider       string    `json:"provider"`
	CliVersion     string    `json:"cli_version,omitempty"`
	WorkDir        string    `json:"work_dir,omitempty"`
	InputTokens    int64     `json:"input_tokens,omitempty"`
	OutputTokens   int64     `json:"output_tokens,omitempty"`
	DurationSecs   int64     `json:"duration_seconds,omitempty"`
}

// SessionListResponse is the response for GET /admin/v1/sessions.
type SessionListResponse struct {
	Sessions []*SessionInfo `json:"sessions"`
	Total    int            `json:"total"`
}

// SessionDetailResponse is the response for GET /admin/v1/sessions/:id.
type SessionDetailResponse struct {
	ID        string        `json:"id"`
	Status    string        `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	LastActive time.Time   `json:"last_active"`
	Provider  string       `json:"provider"`
	Config    SessionConfig `json:"config,omitempty"`
	Stats     SessionStats `json:"stats,omitempty"`
}

// SessionConfig contains session configuration details.
type SessionConfig struct {
	Provider string `json:"provider"`
	WorkDir  string `json:"work_dir"`
}

// SessionStats contains session statistics.
type SessionStats struct {
	InputTokens    int64 `json:"input_tokens"`
	OutputTokens   int64 `json:"output_tokens"`
	DurationSecs   int64 `json:"duration_seconds"`
}

// SessionLogsResponse is the response for GET /admin/v1/sessions/:id/logs.
type SessionLogsResponse struct {
	SessionID    string    `json:"session_id"`
	LogPath      string    `json:"log_path"`
	SizeBytes    int64    `json:"size_bytes"`
	LastModified time.Time `json:"last_modified"`
}

// SessionDeleteResponse is the response for DELETE /admin/v1/sessions/:id.
type SessionDeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// StatsResponse is the response for GET /admin/v1/stats.
type StatsResponse struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	StoppedSessions int     `json:"stopped_sessions"`
	Uptime          string  `json:"uptime"`
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	CpuUsagePercent float64 `json:"cpu_usage_percent"`
}

// ConfigValidateRequest is the request for POST /admin/v1/config/validate.
type ConfigValidateRequest struct {
	ConfigPath string `json:"config_path"`
}

// ConfigValidateResponse is the response for POST /admin/v1/config/validate.
type ConfigValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

// HealthDetailedResponse is the response for GET /admin/v1/health/detailed.
type HealthDetailedResponse struct {
	Status   string           `json:"status"`
	Checks   HealthChecks     `json:"checks"`
	Details  HealthDetails    `json:"details"`
}

// HealthChecks contains health check results.
type HealthChecks struct {
	Database             bool `json:"database"`
	Config               bool `json:"config"`
	CliAvailable         bool `json:"cli_available"`
	WebsocketConnections int `json:"websocket_connections"`
}

// HealthDetails contains detailed health information.
type HealthDetails struct {
	DatabaseLatencyMs int    `json:"database_latency_ms"`
	CliVersion       string `json:"cli_version"`
	ConfigFile       string `json:"config_file"`
}

// ErrorCode represents admin API error codes.
type ErrorCode string

const (
	ErrCodeAuthFailed     ErrorCode = "AUTH_FAILED"
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrCodeInvalidRequest ErrorCode = "INVALID_REQUEST"
	ErrCodeServerError    ErrorCode = "SERVER_ERROR"
)

// ErrorResponse is the standard error response format.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code    ErrorCode              `json:"code"`
	Message string                `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}
