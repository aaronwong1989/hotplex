package provider

import "encoding/json"

// OpenCode Part types (from research results).
const (
	OCPartText       = "text"
	OCPartReasoning  = "reasoning"
	OCPartTool       = "tool"
	OCPartStepStart  = "step-start"
	OCPartStepFinish = "step-finish"
)

// SSE Event types from opencode serve.
const (
	OCEventMessagePartUpdated = "message.part.updated"
	OCEventMessageUpdated     = "message.updated"
	OCEventSessionStatus      = "session.status"
	OCEventSessionIdle        = "session.idle"
	OCEventSessionError       = "session.error"
	OCEventPermissionUpdated  = "permission.updated"
)

// OCPart represents a single part in an OpenCode message.
type OCPart struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	// text / reasoning
	Text string `json:"text,omitempty"`

	// tool (v1.3.2+ uses nested state and tool field)
	Tool   string         `json:"tool,omitempty"`
	Input  map[string]any `json:"input,omitempty"`
	Output string         `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
	State  *OCPartState   `json:"state,omitempty"`

	// step
	StepNumber int    `json:"step_number,omitempty"`
	TotalSteps int    `json:"total_steps,omitempty"`
	Reason     string `json:"reason,omitempty"`

	// token usage (SDK field name is "tokens")
	Tokens *OCUsage `json:"tokens,omitempty"`

	// Legacy compatibility (fallback for older versions)
	Status string `json:"status,omitempty"`
	Name   string `json:"name,omitempty"`
}

// OCPartState represents the nested state structure in v1.3.2+ tool parts.
type OCPartState struct {
	Status string `json:"status,omitempty"` // pending, running, completed, error
}

// Tool status constants for type safety
const (
	ToolStatusPending   = "pending"
	ToolStatusRunning   = "running"
	ToolStatusCompleted = "completed"
	ToolStatusError     = "error"
)

// GetStatus returns the effective status from nested state (v1.3.2+) or legacy field.
// Supports backward compatibility with older OpenCode versions.
func (p *OCPart) GetStatus() string {
	if p.State != nil && p.State.Status != "" {
		return p.State.Status
	}
	return p.Status
}

// GetToolName returns the effective tool name from Tool field (v1.3.2+) or legacy Name field.
// Supports backward compatibility with older OpenCode versions.
func (p *OCPart) GetToolName() string {
	if p.Tool != "" {
		return p.Tool
	}
	return p.Name
}

// OCUsage represents token usage information.
// JSON tags match OpenCode SDK: input, output, reasoning, cache.
type OCUsage struct {
	Input     int32         `json:"input,omitempty"`
	Output    int32         `json:"output,omitempty"`
	Reasoning int32         `json:"reasoning,omitempty"`
	Cache     *OCCacheUsage `json:"cache,omitempty"`
}

// OCCacheUsage represents cache token usage.
type OCCacheUsage struct {
	Read  int32 `json:"read,omitempty"`
	Write int32 `json:"write,omitempty"`
}

// OCMessage represents the output message structure from OpenCode.
type OCMessage struct {
	ID      string   `json:"id,omitempty"`
	Role    string   `json:"role,omitempty"`
	Parts   []OCPart `json:"parts,omitempty"`
	Content string   `json:"content,omitempty"`
	Status  string   `json:"status,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// OCSSEEvent is the SSE "data:" payload from opencode serve.
// Wraps a JSON object with a "type" field and "properties" field.
type OCSSEEvent struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties,omitempty"`
}

// OCPartUpdateProps is the properties payload for message.part.updated events.
type OCPartUpdateProps struct {
	Part  OCPart `json:"part"`
	Delta string `json:"delta,omitempty"`
}

// OCSessionStatusProps is the properties payload for session.status events.
type OCSessionStatusProps struct {
	Status OCSessionState `json:"status"`
}

// OCSessionState describes the current state of an OpenCode session.
type OCSessionState struct {
	Type    string `json:"type"`
	Attempt int    `json:"attempt,omitempty"`
}

// OCSessionErrorProps is the properties payload for session.error events.
type OCSessionErrorProps struct {
	Error OCError `json:"error"`
}

// OCError represents an error from the OpenCode server.
type OCError struct {
	Name    string         `json:"name"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// OCPermissionProps is the properties payload for permission.updated events.
type OCPermissionProps struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
	Title     string `json:"title,omitempty"`
}

// OCAssistantMessage is the info payload for message.updated events.
type OCAssistantMessage struct {
	ID        string   `json:"id,omitempty"`
	SessionID string   `json:"sessionID,omitempty"`
	Role      string   `json:"role,omitempty"`
	ModelID   string   `json:"modelID,omitempty"`
	Cost      float64  `json:"cost,omitempty"`
	Tokens    OCUsage  `json:"tokens"`
	Finish    string   `json:"finish,omitempty"`
	Error     *OCError `json:"error,omitempty"`
}

// OCSession represents an OpenCode server-side session.
type OCSession struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID,omitempty"`
	Directory string `json:"directory,omitempty"`
	Title     string `json:"title"`
	Version   string `json:"version,omitempty"`
}

// FinishReason represents the reason why a step finished
type FinishReason string

// ReasonToolCalls matches MiniMax API's stop_reason value for tool-use termination.
// OpenAI-compatible API uses "tool_use"; MiniMax uses "tool_calls".
var ReasonToolCalls FinishReason = "tool_calls"

const (
	ReasonMaxTokens FinishReason = "max_tokens"
	ReasonToolUse   FinishReason = "tool_use"
	ReasonEndTurn   FinishReason = "end_turn"
)

// finishReasonMessages maps finish reasons to user-friendly messages
// Using map instead of switch for OCP compliance (easy to extend)
var finishReasonMessages = map[FinishReason]string{
	ReasonMaxTokens: "⚠️ Token 限制达到，建议增加配额",
	ReasonToolUse:   "🔧 需要执行工具调用",
	ReasonToolCalls: "🔧 需要执行工具调用",
	ReasonEndTurn:   "✅ 回答生成完毕",
}

// OCTimeStamp represents a timestamp from OpenCode.
type OCTimeStamp struct {
	Created int64 `json:"created,omitempty"`
	Updated int64 `json:"updated,omitempty"`
}
