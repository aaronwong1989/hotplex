package chatapps

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/chatapps/internal"
	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/provider"
)

// mockStatusProvider implements base.StatusProvider for testing
type mockStatusProvider struct {
	lastStatus   base.StatusType
	lastText     string
	updateCalled bool
}

func (m *mockStatusProvider) SetStatus(ctx context.Context, channelID, threadTS string, status base.StatusType, text string) error {
	m.lastStatus = status
	m.lastText = text
	m.updateCalled = true
	return nil
}

func (m *mockStatusProvider) ClearStatus(ctx context.Context, channelID, threadTS string) error {
	m.lastStatus = base.StatusIdle
	m.lastText = ""
	return nil
}

// TestHandleToolUse_CategorizedLabels tests that handleToolUse generates
// correct categorized status labels.
func TestHandleToolUse_CategorizedLabels(t *testing.T) {
	tests := []struct {
		name          string
		toolName      string
		expectedEmoji string
	}{
		{"file read", "Read", "📖"},
		{"file write", "Write", "✏️"},
		{"bash", "Bash", "⚡"},
		{"webfetch", "WebFetch", "🌐"},
		{"search", "Grep", "🔍"},
		{"unknown", "RandomTool", "🛠️"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create callback with mock status manager
			mockStatus := &mockStatusProvider{}
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			statusMgr := internal.NewStatusManager(mockStatus, logger)

			callback := &StreamCallback{
				ctx:       context.Background(),
				logger:    logger,
				metadata:  map[string]any{"channel_id": "test", "thread_ts": "123"},
				statusMgr: statusMgr,
			}

			// Create tool_use event
			data := &event.EventWithMeta{
				EventType: string(provider.EventTypeToolUse),
				Meta: &event.EventMeta{
					ToolName: tt.toolName,
				},
			}

			// Execute
			err := callback.handleToolUse(data)
			if err != nil {
				t.Fatalf("handleToolUse returned error: %v", err)
			}

			// Verify status was updated
			if !mockStatus.updateCalled {
				t.Error("Status was not updated")
			}

			// Verify emoji in status text
			if mockStatus.lastText == "" {
				t.Error("Status text is empty")
			}
		})
	}
}

// TestHandleToolResult_CategorizedLabels tests that handleToolResult generates
// correct categorized status labels for success and error cases.
func TestHandleToolResult_CategorizedLabels(t *testing.T) {
	tests := []struct {
		name          string
		toolName      string
		status        string
		expectedEmoji string
		expectSuccess bool
	}{
		{"success read", "Read", "", "📖", true},
		{"success bash", "Bash", "", "⚡", true},
		{"error tool", "Write", "error", "", false},
		{"unknown tool", "RandomTool", "", "🛠️", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatus := &mockStatusProvider{}
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			statusMgr := internal.NewStatusManager(mockStatus, logger)

			callback := &StreamCallback{
				ctx:       context.Background(),
				logger:    logger,
				metadata:  map[string]any{"channel_id": "test", "thread_ts": "123"},
				statusMgr: statusMgr,
			}

			// Create tool_result event
			data := &event.EventWithMeta{
				EventType: string(provider.EventTypeToolResult),
				EventData: "test output",
				Meta: &event.EventMeta{
					ToolName:   tt.toolName,
					Status:     tt.status,
					DurationMs: 100,
				},
			}

			// Execute
			err := callback.handleToolResult(data)
			if err != nil {
				t.Fatalf("handleToolResult returned error: %v", err)
			}

			// Verify status was updated
			if !mockStatus.updateCalled {
				t.Error("Status was not updated")
			}

			// Verify status text contains expected content
			if mockStatus.lastText == "" {
				t.Error("Status text is empty")
			}
		})
	}
}

// TestHandleToolResult_EmptyToolName tests that empty tool names are handled gracefully.
func TestHandleToolResult_EmptyToolName(t *testing.T) {
	mockStatus := &mockStatusProvider{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	statusMgr := internal.NewStatusManager(mockStatus, logger)

	callback := &StreamCallback{
		ctx:       context.Background(),
		logger:    logger,
		metadata:  map[string]any{"channel_id": "test", "thread_ts": "123"},
		statusMgr: statusMgr,
	}

	// Create tool_result event with empty tool name but has content
	data := &event.EventWithMeta{
		EventType: string(provider.EventTypeToolResult),
		EventData: "some output content",
		Meta: &event.EventMeta{
			ToolName:   "",
			DurationMs: 50,
		},
	}

	// Execute
	err := callback.handleToolResult(data)
	if err != nil {
		t.Fatalf("handleToolResult returned error: %v", err)
	}

	// Verify status was updated (should use fallback tool name)
	if !mockStatus.updateCalled {
		t.Error("Status was not updated for empty tool name")
	}
}

// TestHandleToolResult_SkipEmptyEvents tests that empty tool_result events are skipped.
func TestHandleToolResult_SkipEmptyEvents(t *testing.T) {
	mockStatus := &mockStatusProvider{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	statusMgr := internal.NewStatusManager(mockStatus, logger)

	callback := &StreamCallback{
		ctx:       context.Background(),
		logger:    logger,
		metadata:  map[string]any{"channel_id": "test", "thread_ts": "123"},
		statusMgr: statusMgr,
	}

	// Create empty tool_result event with nil Meta
	data := &event.EventWithMeta{
		EventType: string(provider.EventTypeToolResult),
		EventData: "",
		Meta:      nil,
	}

	// Execute
	err := callback.handleToolResult(data)
	if err != nil {
		t.Fatalf("handleToolResult returned error: %v", err)
	}

	// Verify status was NOT updated (empty event should be skipped)
	if mockStatus.updateCalled {
		t.Error("Status should not be updated for empty tool_result event")
	}
}

// TestHandleToolResult_NilMeta tests that nil Meta is handled gracefully.
func TestHandleToolResult_NilMeta(t *testing.T) {
	mockStatus := &mockStatusProvider{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	statusMgr := internal.NewStatusManager(mockStatus, logger)

	callback := &StreamCallback{
		ctx:       context.Background(),
		logger:    logger,
		metadata:  map[string]any{"channel_id": "test", "thread_ts": "123"},
		statusMgr: statusMgr,
	}

	// Create tool_result event with nil Meta but has content
	data := &event.EventWithMeta{
		EventType: string(provider.EventTypeToolResult),
		EventData: "some output",
		Meta:      nil,
	}

	// Execute - should not panic
	err := callback.handleToolResult(data)
	if err != nil {
		t.Fatalf("handleToolResult returned error: %v", err)
	}

	// Verify status was updated with fallback tool name
	if !mockStatus.updateCalled {
		t.Error("Status should be updated for event with content")
	}
}

// TestBuildToolUseStatusText tests the buildToolUseStatusText function.
func TestBuildToolUseStatusText(t *testing.T) {
	tests := []struct {
		name         string
		toolName     string
		inputSummary string
		wantContains string
		wantMaxLen   int
	}{
		{
			name:         "bash with command",
			toolName:     "Bash",
			inputSummary: "git status",
			wantContains: "git status",
			wantMaxLen:   50,
		},
		{
			name:         "git with commit",
			toolName:     "git_commit",
			inputSummary: "fix: resolve bug",
			wantContains: "fix: resolve bug",
			wantMaxLen:   50,
		},
		{
			name:         "skill tool",
			toolName:     "skill:simplify",
			inputSummary: "",
			wantContains: "skill/simplify",
			wantMaxLen:   50,
		},
		{
			name:         "file read with path",
			toolName:     "Read",
			inputSummary: "file: /path/to/main.go",
			wantContains: "file: /path/to/main.go",
			wantMaxLen:   50,
		},
		{
			name:         "long command truncated",
			toolName:     "Bash",
			inputSummary: "git log --oneline --graph --all --decorate --color",
			wantContains: "…",
			wantMaxLen:   60, // emoji + tool + | + long truncated summary
		},
		{
			name:         "no input summary fallback",
			toolName:     "RandomTool",
			inputSummary: "",
			wantContains: "RandomTool",
			wantMaxLen:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildToolUseStatusText(tt.toolName, tt.inputSummary)

			// Check max length
			if len(result) > tt.wantMaxLen {
				t.Errorf("buildToolUseStatusText() length %d exceeds max %d: %q", len(result), tt.wantMaxLen, result)
			}

			// Check contains expected text
			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("buildToolUseStatusText() should contain %q, got: %q", tt.wantContains, result)
			}
		})
	}
}
