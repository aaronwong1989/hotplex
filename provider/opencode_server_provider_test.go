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
