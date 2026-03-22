// Package relay provides bot-to-bot communication across HotPlex instances.
package relay

import "time"

// RelayBinding binds a platform+chatID to a set of bot instances.
type RelayBinding struct {
	Platform string            `json:"platform"`
	ChatID   string            `json:"chat_id"`
	Bots     map[string]string `json:"bots"`
}

// RelayMessage extends bridgewire.WireMessage with relay-specific fields.
// All fields are omitempty to maintain backward compatibility with existing
// WireMessage serialization.
type RelayMessage struct {
	TaskID     string    `json:"task_id,omitempty"`
	From       string    `json:"from,omitempty"`
	To         string    `json:"to,omitempty"`
	Content    string    `json:"content,omitempty"`
	SessionKey string    `json:"session_key,omitempty"`
	Metadata   string    `json:"metadata,omitempty"`
	Status     string    `json:"status,omitempty"`
	Response   string    `json:"response,omitempty"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

// Task status constants.
const (
	TaskStatusWorking   = "working"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCanceled  = "canceled"
)
