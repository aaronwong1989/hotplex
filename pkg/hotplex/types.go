package hotplex

import (
	"fmt"

	"github.com/hrygo/hotplex/pkg/internal/strutil"
)

// TruncateString truncates a string to a maximum length for logging.
// Uses rune-level truncation to avoid creating invalid UTF-8.
func TruncateString(s string, maxLen int) string {
	return strutil.Truncate(s, maxLen)
}

// SummarizeInput creates a human-readable summary of tool input.
// Uses rune-level truncation to avoid creating invalid UTF-8.
func SummarizeInput(input map[string]any) string {
	if input == nil {
		return ""
	}
	// Extract common fields for summary
	if command, ok := input["command"].(string); ok && command != "" {
		return TruncateString(command, 50)
	}
	if query, ok := input["query"].(string); ok && query != "" {
		return TruncateString(query, 50)
	}
	if path, ok := input["path"].(string); ok && path != "" {
		return "file: " + path
	}
	// Fallback to truncated string representation
	if len(input) == 0 {
		return ""
	}
	// Simple truncated representation
	str := fmt.Sprintf("%+v", input)
	return TruncateString(str, 100)
}

// StreamMessage represents a single event in the stream-json format emitted by the CLI.
type StreamMessage struct {
	Message      *AssistantMessage `json:"message,omitempty"`        // Nested assistant message details
	Input        map[string]any    `json:"input,omitempty"`          // Tool input arguments
	Type         string            `json:"type"`                     // Event type (e.g., "tool_use", "answer")
	Timestamp    string            `json:"timestamp,omitempty"`      // Event timestamp
	SessionID    string            `json:"session_id,omitempty"`     // Associated HotPlex session ID
	Role         string            `json:"role,omitempty"`           // Sender role (user/assistant)
	Name         string            `json:"name,omitempty"`           // Tool name or block name
	Output       string            `json:"output,omitempty"`         // Raw output text
	Status       string            `json:"status,omitempty"`         // Execution status
	Error        string            `json:"error,omitempty"`          // Error message if applicable
	Content      []ContentBlock    `json:"content,omitempty"`        // Array of content blocks
	Duration     int               `json:"duration_ms,omitempty"`    // Duration in milliseconds
	Subtype      string            `json:"subtype,omitempty"`        // Subtype for "result" message
	IsError      bool              `json:"is_error,omitempty"`       // Error flag for "result" message
	TotalCostUSD float64           `json:"total_cost_usd,omitempty"` // Computed cost for "result" message
	Usage        *UsageStats       `json:"usage,omitempty"`          // Token usage for "result" message
	Result       string            `json:"result,omitempty"`         // Final result text for "result" message
}

// UsageStats represents token usage from result messages.
type UsageStats struct {
	InputTokens           int32 `json:"input_tokens"`
	OutputTokens          int32 `json:"output_tokens"`
	CacheWriteInputTokens int32 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens  int32 `json:"cache_read_input_tokens,omitempty"`
}

// GetContentBlocks returns the content blocks, checking both direct and nested locations.
func (m *StreamMessage) GetContentBlocks() []ContentBlock {
	if m.Message != nil && len(m.Message.Content) > 0 {
		return m.Message.Content
	}
	return m.Content
}

// AssistantMessage represents the nested message structure in assistant
type AssistantMessage struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type,omitempty"`
	Role    string         `json:"role,omitempty"`
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentBlock represents a content block in stream-json format.
type ContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Name      string         `json:"name,omitempty"`
	ID        string         `json:"id,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"` // For tool_result blocks to reference tool_use
	Input     map[string]any `json:"input,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

// GetUnifiedToolID returns the tool ID, preferring ToolUseID over ID for tool_result matching.
func (b *ContentBlock) GetUnifiedToolID() string {
	if b.ToolUseID != "" {
		return b.ToolUseID
	}
	return b.ID
}

// Config defines the configuration and context for a HotPlex execution.
type Config struct {
	WorkDir          string // Absolute path to the working directory for the CLI
	ConversationID   int64  // Database conversation ID used for deterministic UUID v5 mapping
	SessionID        string // Session identifier (derived from ConversationID if left empty)
	UserID           int32  // User ID for logging, context, and isolation
	TaskSystemPrompt string // Task-specific system prompt injected at startup (merged with EngineOptions.BaseSystemPrompt)
	DeviceContext    string // JSON string containing device/browser context

	// Session specific boundaries
	SessionAllowedPaths []string // Path whitelist for file access (specific to this session)
}

// ProcessingPhase represents the current phase of agent processing.
type ProcessingPhase string

const (
	// PhaseAnalyzing is the initial analysis phase.
	PhaseAnalyzing ProcessingPhase = "analyzing"
	// PhasePlanning is the planning phase for multi-step tasks.
	PhasePlanning ProcessingPhase = "planning"
	// PhaseRetrieving is the information retrieval phase.
	PhaseRetrieving ProcessingPhase = "retrieving"
	// PhaseSynthesizing is the final response generation phase.
	PhaseSynthesizing ProcessingPhase = "synthesizing"
)

// PhaseChangeEvent represents a phase change event.
type PhaseChangeEvent struct {
	Phase            ProcessingPhase `json:"phase"`
	PhaseNumber      int             `json:"phase_number"`
	TotalPhases      int             `json:"total_phases"`
	EstimatedSeconds int             `json:"estimated_seconds"`
}

// ProgressEvent represents a progress update event.
type ProgressEvent struct {
	Percent              int `json:"percent"`
	EstimatedSeconds     int `json:"estimated_seconds"`
	EstimatedTimeSeconds int `json:"estimated_time_seconds"`
}

// Event type constants for streaming
const (
	// EventTypePhaseChange is the event type for phase changes.
	EventTypePhaseChange = "phase_change"
	// EventTypeProgress is the event type for progress updates.
	EventTypeProgress = "progress"
	// EventTypeThinking is the event type for thinking updates.
	EventTypeThinking = "thinking"
	// EventTypeToolUse is the event type for tool invocations.
	EventTypeToolUse = "tool_use"
	// EventTypeToolResult is the event type for tool results.
	EventTypeToolResult = "tool_result"
	// EventTypeAnswer is the event type for final answers.
	EventTypeAnswer = "answer"
	// EventTypeError is the event type for errors.
	EventTypeError = "error"
)
