package diag

import (
	"time"
)

// DiagTrigger indicates how the diagnosis was triggered.
type DiagTrigger string

const (
	TriggerAuto    DiagTrigger = "auto"     // Automatic from error hooks
	TriggerCommand DiagTrigger = "command"  // Manual via /diagnose command
)

// ErrorType categorizes the type of error that occurred.
type ErrorType string

const (
	ErrorTypeExit    ErrorType = "exit"      // CLI process exited abnormally
	ErrorTypeTimeout ErrorType = "timeout"  // Session timeout
	ErrorTypeWAF     ErrorType = "waf"      // Security/WAF violation
	ErrorTypePanic   ErrorType = "panic"    // Panic or crash
	ErrorTypeUnknown ErrorType = "unknown"  // Unknown error
)

// ErrorInfo contains details about the error.
type ErrorInfo struct {
	Type       ErrorType   `json:"type"`
	Message   string      `json:"message"`
	ExitCode  int         `json:"exit_code,omitempty"`
	StackTrace string     `json:"stack_trace,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// EnvInfo contains environment information.
type EnvInfo struct {
	HotPlexVersion string        `json:"hotplex_version"`
	GoVersion      string        `json:"go_version"`
	OS             string        `json:"os"`
	Arch           string        `json:"arch"`
	Uptime         time.Duration `json:"uptime"`
}

// ConversationData contains processed conversation history.
type ConversationData struct {
	Processed    string `json:"processed"`
	MessageCount int    `json:"message_count"`
	IsSummarized bool   `json:"is_summarized"`
}

// DiagContext contains all information for diagnosis.
type DiagContext struct {
	OriginalSessionID string           `json:"original_session_id"`
	Platform          string           `json:"platform"`
	UserID            string           `json:"user_id"`
	ChannelID         string           `json:"channel_id"`
	ThreadID          string           `json:"thread_id"`
	Trigger           DiagTrigger      `json:"trigger"`
	Error             *ErrorInfo       `json:"error"`
	Conversation      *ConversationData `json:"conversation"`
	Logs              []byte           `json:"logs"`
	Environment       *EnvInfo         `json:"environment"`
	Timestamp         time.Time        `json:"timestamp"`
}

// Trigger is the interface for diagnosis triggers.
type Trigger interface {
	Type() DiagTrigger
	SessionID() string
	Error() *ErrorInfo
	Platform() string
	UserID() string
	ChannelID() string
	ThreadID() string
}

// BaseTrigger provides a basic Trigger implementation.
type BaseTrigger struct {
	triggerType DiagTrigger
	sessionID   string
	err        *ErrorInfo
	platform   string
	userID     string
	channelID  string
	threadID   string
}

func NewBaseTrigger(triggerType DiagTrigger, sessionID string, err *ErrorInfo) *BaseTrigger {
	return &BaseTrigger{triggerType: triggerType, sessionID: sessionID, err: err}
}

func (t *BaseTrigger) Type() DiagTrigger    { return t.triggerType }
func (t *BaseTrigger) SessionID() string    { return t.sessionID }
func (t *BaseTrigger) Error() *ErrorInfo    { return t.err }
func (t *BaseTrigger) Platform() string     { return t.platform }
func (t *BaseTrigger) UserID() string       { return t.userID }
func (t *BaseTrigger) ChannelID() string    { return t.channelID }
func (t *BaseTrigger) ThreadID() string     { return t.threadID }

func (t *BaseTrigger) SetPlatform(p string) *BaseTrigger {
	t.platform = p
	return t
}

func (t *BaseTrigger) SetUserID(u string) *BaseTrigger {
	t.userID = u
	return t
}

func (t *BaseTrigger) SetChannelID(c string) *BaseTrigger {
	t.channelID = c
	return t
}

func (t *BaseTrigger) SetThreadID(tid string) *BaseTrigger {
	t.threadID = tid
	return t
}

// Config contains diagnostic configuration.
type Config struct {
	Enabled               bool
	LogSizeLimit          int
	ConversationSizeLimit int
	GitHubRepo            string
	GitHubLabels          []string
}

func DefaultConfig() *Config {
	return &Config{
		Enabled:               true,
		LogSizeLimit:          20 * 1024,
		ConversationSizeLimit: 20 * 1024,
		GitHubRepo:            "hrygo/hotplex",
		GitHubLabels:          []string{"bug", "auto-diagnosed"},
	}
}
