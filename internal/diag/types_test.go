package diag

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if cfg.LogSizeLimit != 20*1024 {
		t.Errorf("Expected LogSizeLimit to be 20480, got %d", cfg.LogSizeLimit)
	}
	if cfg.ConversationSizeLimit != 20*1024 {
		t.Errorf("Expected ConversationSizeLimit to be 20480, got %d", cfg.ConversationSizeLimit)
	}
	if cfg.GitHubRepo != "hrygo/hotplex" {
		t.Errorf("Expected GitHubRepo to be hrygo/hotplex, got %s", cfg.GitHubRepo)
	}
	if len(cfg.GitHubLabels) != 2 {
		t.Errorf("Expected 2 GitHubLabels, got %d", len(cfg.GitHubLabels))
	}
}

func TestBaseTrigger(t *testing.T) {
	errInfo := &ErrorInfo{
		Type:      ErrorTypeExit,
		Message:   "test error",
		ExitCode:  1,
		Timestamp: time.Now(),
	}

	trigger := NewBaseTrigger(TriggerAuto, "session-123", errInfo).
		SetPlatform("slack").
		SetUserID("user-456").
		SetChannelID("channel-789").
		SetThreadID("thread-012")

	if trigger.Type() != TriggerAuto {
		t.Errorf("Expected Type to be TriggerAuto, got %s", trigger.Type())
	}
	if trigger.SessionID() != "session-123" {
		t.Errorf("Expected SessionID to be session-123, got %s", trigger.SessionID())
	}
	if trigger.Error() != errInfo {
		t.Error("Expected Error to be the same pointer")
	}
	if trigger.Platform() != "slack" {
		t.Errorf("Expected Platform to be slack, got %s", trigger.Platform())
	}
	if trigger.UserID() != "user-456" {
		t.Errorf("Expected UserID to be user-456, got %s", trigger.UserID())
	}
	if trigger.ChannelID() != "channel-789" {
		t.Errorf("Expected ChannelID to be channel-789, got %s", trigger.ChannelID())
	}
	if trigger.ThreadID() != "thread-012" {
		t.Errorf("Expected ThreadID to be thread-012, got %s", trigger.ThreadID())
	}
}

func TestDiagContext(t *testing.T) {
	now := time.Now()
	ctx := &DiagContext{
		OriginalSessionID: "sess-001",
		Platform:          "telegram",
		UserID:            "user-001",
		ChannelID:         "chat-001",
		ThreadID:          "thread-001",
		Trigger:           TriggerCommand,
		Error: &ErrorInfo{
			Type:       ErrorTypeTimeout,
			Message:    "timeout occurred",
			Timestamp:  now,
		},
		Conversation: &ConversationData{
			Processed:    "hello world",
			MessageCount: 5,
			IsSummarized: true,
		},
		Logs: []byte("log line 1\nlog line 2"),
		Environment: &EnvInfo{
			HotPlexVersion: "v0.22.0",
			GoVersion:      "go1.25",
			OS:             "linux",
			Arch:           "amd64",
			Uptime:         10 * time.Second,
		},
		Timestamp: now,
	}

	if ctx.OriginalSessionID != "sess-001" {
		t.Errorf("Expected OriginalSessionID, got %s", ctx.OriginalSessionID)
	}
	if ctx.Platform != "telegram" {
		t.Errorf("Expected Platform, got %s", ctx.Platform)
	}
	if ctx.Trigger != TriggerCommand {
		t.Errorf("Expected TriggerCommand, got %s", ctx.Trigger)
	}
	if ctx.Error.Type != ErrorTypeTimeout {
		t.Errorf("Expected ErrorTypeTimeout, got %s", ctx.Error.Type)
	}
	if ctx.Conversation.MessageCount != 5 {
		t.Errorf("Expected MessageCount 5, got %d", ctx.Conversation.MessageCount)
	}
	if len(ctx.Logs) == 0 {
		t.Error("Expected Logs to be non-empty")
	}
	if ctx.Environment.HotPlexVersion != "v0.22.0" {
		t.Errorf("Expected HotPlexVersion, got %s", ctx.Environment.HotPlexVersion)
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		expected string
	}{
		{"exit", ErrorTypeExit, "exit"},
		{"timeout", ErrorTypeTimeout, "timeout"},
		{"waf", ErrorTypeWAF, "waf"},
		{"panic", ErrorTypePanic, "panic"},
		{"unknown", ErrorTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.errType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.errType)
			}
		})
	}
}

func TestDiagTriggers(t *testing.T) {
	if string(TriggerAuto) != "auto" {
		t.Errorf("Expected auto, got %s", TriggerAuto)
	}
	if string(TriggerCommand) != "command" {
		t.Errorf("Expected command, got %s", TriggerCommand)
	}
}
