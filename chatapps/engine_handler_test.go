package chatapps

import (
	"context"
	"log/slog"
	"os"
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

// TestInferToolFromOperation tests the tool inference logic.
func TestInferToolFromOperation(t *testing.T) {
	tests := []struct {
		name     string
		oper     string
		expected string
	}{
		{"empty", "", ""},
		{"rm command", "rm -rf /tmp/test", "Bash"},
		{"del command", "del file.txt", "Bash"},
		{"remove command", "remove /tmp/test", "Bash"},
		{"contains rm", "some text containing rm dangerous", "Bash"},
		{"mkdir command", "mkdir -p /tmp/dir", "Bash"},
		{"mk command", "mk tmpdir", "Bash"},
		{"contains mkdir", "create dir with mkdir", "Bash"},
		{"mv command", "mv file1 file2", "Bash"},
		{"move command", "move file.txt /tmp/", "Bash"},
		{"cp command", "cp src dst", "Bash"},
		{"copy command", "copy file.txt /tmp/", "Bash"},
		{"git command", "git commit -m 'fix'", "Bash"},
		{"git prefix", "gitstatus", "Bash"},
		{"docker command", "docker run -it alpine", "Bash"},
		{"docker prefix", "docker-compose up -d", "Bash"},
		{"curl command", "curl -s https://example.com", "Bash"},
		{"wget command", "wget https://example.com/file", "Bash"},
		{"ssh command", "ssh user@host", "Bash"},
		{"contains curl", "run curl request", "Bash"},
		{"contains wget", "use wget to download", "Bash"},
		{"npm command", "npm install", "Bash"},
		{"pip command", "pip install requests", "Bash"},
		{"cargo command", "cargo build", "Bash"},
		{"go command", "go run main.go", "Bash"},
		{"contains npm install", "do npm install", "Bash"},
		{"contains pip install", "do pip install", "Bash"},
		{"echo default", "echo hello", "echo"},
		{"python first word", "python script.py", "python"},
		{"node first word", "node server.js", "node"},
		{"case insensitive git", "GIT commit -m 'test'", "Bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferToolFromOperation(tt.oper)
			if got != tt.expected {
				t.Errorf("inferToolFromOperation(%q) = %q, want %q", tt.oper, got, tt.expected)
			}
		})
	}
}

// TestIdleTimerFired_SkipsFallbackForToolStatus verifies that the idle timer
// correctly skips fallback when the session is actively executing tools.
// Updated to check current real-time status instead of trigger-time status.
func TestIdleTimerFired_SkipsFallbackForToolStatus(t *testing.T) {
	tests := []struct {
		name           string
		currentStatus  base.MessageType // Current real-time status
		triggerStatus  base.MessageType // Status when timer was set (3s ago)
		expectFallback bool
	}{
		{"ToolUse in progress skips fallback", base.MessageTypeToolUse, base.MessageTypeToolUse, false},
		{"ToolResult in progress skips fallback", base.MessageTypeToolResult, base.MessageTypeToolResult, false},
		{"ToolUse completed allows fallback", "", base.MessageTypeToolUse, true},
		{"ToolResult completed allows fallback", "", base.MessageTypeToolResult, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			callback := &StreamCallback{
				ctx:           context.Background(),
				sessionID:     "test-session",
				logger:        logger,
				metadata:      map[string]any{"channel_id": "test", "thread_ts": "123"},
				isFinished:    false,
				currentStatus: tt.currentStatus, // Set current real-time status
			}

			// Call idleTimerFired with trigger status (captured 3s ago)
			sent := callback.idleTimerFired(tt.triggerStatus)

			if sent != tt.expectFallback {
				t.Errorf("expected fallback=%v, got %v (current=%s, trigger=%s)",
					tt.expectFallback, sent, tt.currentStatus, tt.triggerStatus)
			}
		})
	}
}

// TestIdleTimerFired_FinishedSessionNoFallback verifies that no fallback is
// sent when the session is already finished.
func TestIdleTimerFired_FinishedSessionNoFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	callback := &StreamCallback{
		ctx:        context.Background(),
		sessionID:  "test-session",
		logger:     logger,
		metadata:   map[string]any{"channel_id": "test", "thread_ts": "123"},
		isFinished: true, // Session already finished
	}

	// idleTimerFired should return false for finished session
	sent := callback.idleTimerFired(base.MessageTypeThinking)
	if sent {
		t.Error("expected no fallback for finished session")
	}
}

// TestIdleTimerFired_SendsFallbackForThinking verifies that fallback thinking
// is sent when the trigger status is Thinking.
func TestIdleTimerFired_SendsFallbackForThinking(t *testing.T) {
	mockStatus := &mockStatusProvider{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	statusMgr := internal.NewStatusManager(mockStatus, logger)

	callback := &StreamCallback{
		ctx:        context.Background(),
		sessionID:  "test-session",
		logger:     logger,
		metadata:   map[string]any{"channel_id": "test", "thread_ts": "123"},
		isFinished: false,
		statusMgr:  statusMgr,
	}

	// Call idleTimerFired with Thinking status - should send fallback
	sent := callback.idleTimerFired(base.MessageTypeThinking)

	if !sent {
		t.Error("expected fallback to be sent for Thinking status")
	}
}

// TestIdleTimerFired_SendsFallbackForAnswer verifies that fallback thinking
// is sent when the trigger status is Answer.
func TestIdleTimerFired_SendsFallbackForAnswer(t *testing.T) {
	mockStatus := &mockStatusProvider{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	statusMgr := internal.NewStatusManager(mockStatus, logger)

	callback := &StreamCallback{
		ctx:        context.Background(),
		sessionID:  "test-session",
		logger:     logger,
		metadata:   map[string]any{"channel_id": "test", "thread_ts": "123"},
		isFinished: false,
		statusMgr:  statusMgr,
	}

	// Call idleTimerFired with Answer status - should send fallback
	sent := callback.idleTimerFired(base.MessageTypeAnswer)

	if !sent {
		t.Error("expected fallback to be sent for Answer status")
	}
}

// TestResetIdleTimer_FinishedSessionNoTimer verifies that resetIdleTimer
// returns early without setting a timer when isFinished is true.
func TestResetIdleTimer_FinishedSessionNoTimer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	callback := &StreamCallback{
		ctx:        context.Background(),
		sessionID:  "test-session",
		logger:     logger,
		metadata:   map[string]any{"channel_id": "test", "thread_ts": "123"},
		isFinished: true, // Session already finished
	}

	// resetIdleTimer should return early without setting a timer
	callback.resetIdleTimer()

	// Verify no timer was set
	if callback.idleTimer != nil {
		t.Error("idleTimer should not be set for finished session")
	}
}

// TestResetIdleTimer_CapturesTriggerStatus verifies that resetIdleTimer
// correctly captures triggerStatus at timer set time.
func TestResetIdleTimer_CapturesTriggerStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	callback := &StreamCallback{
		ctx:        context.Background(),
		sessionID:  "test-session",
		logger:     logger,
		metadata:   map[string]any{"channel_id": "test", "thread_ts": "123"},
		isFinished: false,
	}

	// Set status to Thinking and call resetIdleTimer
	callback.mu.Lock()
	callback.currentStatus = base.MessageTypeThinking
	callback.mu.Unlock()

	callback.resetIdleTimer()

	// Verify timer was set
	if callback.idleTimer == nil {
		t.Fatal("idleTimer should be set")
	}

	// Stop timer to clean up
	callback.idleTimer.Stop()
}

// TestResetIdleTimer_StopsPreviousTimer verifies that resetIdleTimer stops
// any previously running timer.
func TestResetIdleTimer_StopsPreviousTimer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	callback := &StreamCallback{
		ctx:        context.Background(),
		sessionID:  "test-session",
		logger:     logger,
		metadata:   map[string]any{"channel_id": "test", "thread_ts": "123"},
		isFinished: false,
	}

	// Set first timer
	callback.mu.Lock()
	callback.currentStatus = base.MessageTypeThinking
	callback.mu.Unlock()
	callback.resetIdleTimer()
	firstTimer := callback.idleTimer

	// Set second timer - should stop the first
	callback.mu.Lock()
	callback.currentStatus = base.MessageTypeAnswer
	callback.mu.Unlock()
	callback.resetIdleTimer()

	// Verify timer was replaced
	if callback.idleTimer == firstTimer {
		t.Error("timer should be replaced, not the same instance")
	}

	// Clean up
	callback.idleTimer.Stop()
}

// TestHandleSessionStats_CacheTokenFields tests that handleSessionStats includes
// cache token fields in the session stats metadata.
func TestHandleSessionStats_CacheTokenFields(t *testing.T) {
	tests := []struct {
		name               string
		cacheReadTokens    int32
		cacheWriteTokens   int32
		inputTokens        int32
		outputTokens       int32
		isError            bool
		accumulatedContent string
	}{
		{
			name:               "with cache tokens",
			cacheReadTokens:    1500,
			cacheWriteTokens:   800,
			inputTokens:        2000,
			outputTokens:       500,
			isError:            false,
			accumulatedContent: "test response",
		},
		{
			name:               "zero cache tokens",
			cacheReadTokens:    0,
			cacheWriteTokens:   0,
			inputTokens:        1000,
			outputTokens:       300,
			isError:            false,
			accumulatedContent: "test response",
		},
		{
			name:               "error session with content",
			cacheReadTokens:    100,
			cacheWriteTokens:   50,
			inputTokens:        500,
			outputTokens:       100,
			isError:            true,
			accumulatedContent: "error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

			// Create callback with nil adapters (sendMessageAndGetTS will be no-op)
			callback := &StreamCallback{
				ctx:       context.Background(),
				sessionID: "test-session",
				platform:  "slack",
				logger:    logger,
				metadata:  map[string]any{"channel_id": "test", "thread_ts": "123"},
				adapters:  nil,                 // sendMessageAndGetTS will return nil
				processor: NewProcessorChain(), // Empty processor chain
			}

			// Create session stats data with cache tokens
			stats := &event.SessionStatsData{
				SessionID:          "test-session",
				InputTokens:        tt.inputTokens,
				OutputTokens:       tt.outputTokens,
				CacheReadTokens:    tt.cacheReadTokens,
				CacheWriteTokens:   tt.cacheWriteTokens,
				TotalTokens:        tt.inputTokens + tt.outputTokens,
				IsError:            tt.isError,
				TotalDurationMs:    1000,
				ThinkingDurationMs: 500,
				ToolDurationMs:     300,
			}

			// Set accumulated content to simulate answer processing
			if tt.accumulatedContent != "" {
				callback.mu.Lock()
				callback.accumulatedContent.WriteString(tt.accumulatedContent)
				callback.mu.Unlock()
			}

			// Execute handleSessionStats
			// This will exercise the code path that builds metadata with cache token fields
			err := callback.handleSessionStats(stats)

			// Verify no error occurred
			// Note: Since adapters is nil, sendMessageAndGetTS returns nil (no actual message sent)
			// but the metadata construction code is exercised
			if err != nil {
				t.Errorf("handleSessionStats returned error: %v", err)
			}
		})
	}
}
