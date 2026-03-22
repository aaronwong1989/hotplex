package relay

import "time"

// RelayResponse is the result of a relay send operation.
type RelayResponse struct {
	TaskID    string        `json:"task_id"`
	Status    string        `json:"status"`
	Content   string        `json:"content,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}
