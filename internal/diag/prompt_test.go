package diag

import (
	"strings"
	"testing"
	"time"
)

func TestBuildDiagnosticPrompt(t *testing.T) {
	now := time.Now()
	diagCtx := &DiagContext{
		OriginalSessionID: "sess-001",
		Platform:          "slack",
		UserID:            "user-001",
		ChannelID:         "channel-001",
		ThreadID:          "thread-001",
		Trigger:           TriggerAuto,
		Error: &ErrorInfo{
			Type:       ErrorTypeExit,
			Message:    "process exited with code 1",
			ExitCode:   1,
			StackTrace: "goroutine 1:\n    main.main()\n    /main.go:10",
			Timestamp:  now,
		},
		Conversation: &ConversationData{
			Processed:    "Hello\nWorld",
			MessageCount: 2,
			IsSummarized: false,
		},
		Logs:        []byte("2024/01/01 10:00:00 starting\n2024/01/01 10:00:01 error occurred"),
		Environment: &EnvInfo{
			HotPlexVersion: "v0.22.0",
			GoVersion:      "go1.25",
			OS:             "linux",
			Arch:           "amd64",
			Uptime:         5 * time.Minute,
		},
		Timestamp: now,
	}

	prompt := BuildDiagnosticPrompt(diagCtx)

	// Check that prompt contains expected sections
	if !strings.Contains(prompt, "Session Information") {
		t.Error("Expected Session Information section")
	}
	if !strings.Contains(prompt, "sess-001") {
		t.Error("Expected session ID in prompt")
	}
	if !strings.Contains(prompt, "slack") {
		t.Error("Expected platform in prompt")
	}
	if !strings.Contains(prompt, "Error Details") {
		t.Error("Expected Error Details section")
	}
	if !strings.Contains(prompt, "process exited with code 1") {
		t.Error("Expected error message in prompt")
	}
	if !strings.Contains(prompt, "Environment") {
		t.Error("Expected Environment section")
	}
	if !strings.Contains(prompt, "v0.22.0") {
		t.Error("Expected version in prompt")
	}
	if !strings.Contains(prompt, "Recent Conversation") {
		t.Error("Expected Conversation section")
	}
	if !strings.Contains(prompt, "Recent Logs") {
		t.Error("Expected Logs section")
	}
	if !strings.Contains(prompt, "Your Task") {
		t.Error("Expected Task section")
	}
}

func TestBuildDiagnosticPromptMinimal(t *testing.T) {
	diagCtx := &DiagContext{
		OriginalSessionID: "sess-min",
		Platform:          "telegram",
		Trigger:           TriggerCommand,
		Timestamp:         time.Now(),
	}

	prompt := BuildDiagnosticPrompt(diagCtx)

	if !strings.Contains(prompt, "sess-min") {
		t.Error("Expected session ID in prompt")
	}
	if !strings.Contains(prompt, "telegram") {
		t.Error("Expected platform in prompt")
	}
	// Should not panic with nil error
	_ = prompt
}

func TestBuildDiagnosticPromptWithNilFields(t *testing.T) {
	diagCtx := &DiagContext{
		OriginalSessionID: "sess-nil",
		Platform:          "slack",
		Trigger:           TriggerAuto,
		Timestamp:         time.Now(),
		// Error, Environment, Conversation, Logs are nil
	}

	// Should not panic
	prompt := BuildDiagnosticPrompt(diagCtx)
	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}
}
