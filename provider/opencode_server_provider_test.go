package provider

import (
	"log/slog"
	"os"
	"testing"
)

// TestOpenCodeServerProvider_Metadata tests the metadata of the HTTP server provider
func TestOpenCodeServerProvider_Metadata(t *testing.T) {
	enabled := true
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider, err := NewOpenCodeServerProvider(ProviderConfig{
		Type:    ProviderTypeOpenCodeServer,
		Enabled: &enabled,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create OpenCode Server provider: %v", err)
	}

	meta := provider.Metadata()
	if meta.Type != ProviderTypeOpenCodeServer {
		t.Errorf("Expected type %s, got %s", ProviderTypeOpenCodeServer, meta.Type)
	}
	if !meta.Features.SupportsSSE {
		t.Error("Expected SupportsSSE to be true")
	}
	if !meta.Features.SupportsHTTPAPI {
		t.Error("Expected SupportsHTTPAPI to be true")
	}
}

// TestOpenCodeServerProvider_BuildCLIArgs tests that CLI args returns nil for server mode
func TestOpenCodeServerProvider_BuildCLIArgs(t *testing.T) {
	enabled := true
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider, err := NewOpenCodeServerProvider(ProviderConfig{
		Type:    ProviderTypeOpenCodeServer,
		Enabled: &enabled,
		OpenCode: &OpenCodeConfig{
			Provider: "anthropic",
			Model:    "claude-3-5-sonnet",
		},
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	opts := &ProviderSessionOptions{
		WorkDir: "/tmp/test",
	}

	args := provider.BuildCLIArgs("test-session", opts)
	// Server mode should not have CLI args
	if args != nil {
		t.Errorf("Expected nil CLI args for server mode, got %v", args)
	}
}

// TestOpenCodeServerProvider_DetectTurnEnd tests turn end detection
func TestOpenCodeServerProvider_DetectTurnEnd(t *testing.T) {
	enabled := true
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider, err := NewOpenCodeServerProvider(ProviderConfig{
		Type:    ProviderTypeOpenCodeServer,
		Enabled: &enabled,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		event *ProviderEvent
		want  bool
	}{
		{&ProviderEvent{Type: EventTypeAnswer, Content: "some answer"}, false},
		{&ProviderEvent{Type: EventTypeResult}, true},
		{&ProviderEvent{Type: EventTypeError}, true},
	}

	for _, tt := range tests {
		got := provider.DetectTurnEnd(tt.event)
		if got != tt.want {
			t.Errorf("DetectTurnEnd(%s) = %v, want %v", tt.event.Type, got, tt.want)
		}
	}
}

// TestOpenCodeServerProvider_ToolPartParsing tests v1.3.2 nested state extraction
func TestOpenCodeServerProvider_ToolPartParsing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider, err := NewOpenCodeServerProvider(ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name       string
		rawEvent   string
		wantType   ProviderEventType
		wantTool   string
		wantID     string
		wantStatus string
	}{
		{
			name:       "v1.3.2 nested state running",
			rawEvent:   `{"type":"message.part.updated","properties":{"part":{"id":"tool-123","type":"tool","tool":"Bash","state":{"status":"running"},"input":{"command":"ls"}}}}`,
			wantType:   EventTypeToolUse,
			wantTool:   "Bash",
			wantID:     "tool-123",
			wantStatus: "running",
		},
		{
			name:       "v1.3.2 nested state completed",
			rawEvent:   `{"type":"message.part.updated","properties":{"part":{"id":"tool-456","type":"tool","tool":"Read","state":{"status":"completed"},"output":"file content"}}}`,
			wantType:   EventTypeToolResult,
			wantTool:   "Read",
			wantID:     "tool-456",
			wantStatus: "success",
		},
		{
			name:       "v1.3.2 nested state error",
			rawEvent:   `{"type":"message.part.updated","properties":{"part":{"id":"tool-789","type":"tool","tool":"Bash","state":{"status":"error"},"error":"command failed"}}}`,
			wantType:   EventTypeToolResult,
			wantTool:   "Bash",
			wantID:     "tool-789",
			wantStatus: "error",
		},
		{
			name:       "legacy top-level status",
			rawEvent:   `{"type":"message.part.updated","properties":{"part":{"id":"tool-legacy","type":"tool","name":"Bash","status":"running","input":{"command":"pwd"}}}}`,
			wantType:   EventTypeToolUse,
			wantTool:   "Bash",
			wantID:     "tool-legacy",
			wantStatus: "running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := provider.ParseEvent(tt.rawEvent)
			if err != nil {
				t.Fatalf("ParseEvent() error = %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
			}

			evt := events[0]
			if evt.Type != tt.wantType {
				t.Errorf("ParseEvent() type = %s, want %s", evt.Type, tt.wantType)
			}
			if evt.ToolName != tt.wantTool {
				t.Errorf("ParseEvent() toolName = %s, want %s", evt.ToolName, tt.wantTool)
			}
			if evt.ToolID != tt.wantID {
				t.Errorf("ParseEvent() toolID = %s, want %s", evt.ToolID, tt.wantID)
			}
			if evt.Status != tt.wantStatus {
				t.Errorf("ParseEvent() status = %s, want %s", evt.Status, tt.wantStatus)
			}
		})
	}
}

// TestOpenCodeServerProvider_SessionStatusIdle tests that session.status idle
// triggers a turn-end (EventTypeResult). This is critical for session_stats delivery.
func TestOpenCodeServerProvider_SessionStatusIdle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	p, err := NewOpenCodeServerProvider(ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	rawEvt := `{"type":"session.status","properties":{"status":{"type":"idle"}}}`
	events, err := p.ParseEvent(rawEvt)
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}
	if events[0].Type != EventTypeResult {
		t.Errorf("ParseEvent() type = %s, want %s", events[0].Type, EventTypeResult)
	}
	if !p.DetectTurnEnd(events[0]) {
		t.Error("DetectTurnEnd() = false for session.status idle, want true")
	}
}

// TestOpenCodeServerProvider_MessageUpdatedTokenParsing tests that message.updated
// events correctly parse token usage with SDK field names (input/output).
func TestOpenCodeServerProvider_MessageUpdatedTokenParsing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	p, err := NewOpenCodeServerProvider(ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	rawEvt := `{"type":"message.updated","properties":{"info":{"id":"msg-1","modelID":"claude-sonnet-4-5","finish":"end_turn","tokens":{"input":1200,"output":350},"cost":0.01}}}`
	events, err := p.ParseEvent(rawEvt)
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}
	if events[0].Type != EventTypeResult {
		t.Errorf("ParseEvent() type = %s, want %s", events[0].Type, EventTypeResult)
	}
	if events[0].Metadata.InputTokens != 1200 {
		t.Errorf("InputTokens = %d, want 1200", events[0].Metadata.InputTokens)
	}
	if events[0].Metadata.OutputTokens != 350 {
		t.Errorf("OutputTokens = %d, want 350", events[0].Metadata.OutputTokens)
	}
}

// TestOpenCodeServerProvider_StepFinishTokenParsing tests that step-finish parts
// correctly parse token usage from the "tokens" JSON field (not "usage").
func TestOpenCodeServerProvider_StepFinishTokenParsing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	p, err := NewOpenCodeServerProvider(ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	rawEvt := `{"type":"message.part.updated","properties":{"part":{"type":"step-finish","step_number":1,"total_steps":3,"reason":"end_turn","tokens":{"input":500,"output":200,"reasoning":50,"cache":{"read":100,"write":50}}}}}`
	events, err := p.ParseEvent(rawEvt)
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}
	if events[0].Type != EventTypeStepFinish {
		t.Errorf("ParseEvent() type = %s, want %s", events[0].Type, EventTypeStepFinish)
	}
	if events[0].Metadata.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500", events[0].Metadata.InputTokens)
	}
	if events[0].Metadata.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", events[0].Metadata.OutputTokens)
	}
	if events[0].Metadata.CacheReadTokens != 100 {
		t.Errorf("CacheReadTokens = %d, want 100", events[0].Metadata.CacheReadTokens)
	}
	if events[0].Metadata.CacheWriteTokens != 50 {
		t.Errorf("CacheWriteTokens = %d, want 50", events[0].Metadata.CacheWriteTokens)
	}
}
