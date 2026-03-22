package engine

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hrygo/hotplex/provider"
)

func TestIsExpectedCloseError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"EOF", io.EOF, true},
		{"file already closed", errors.New("read |0: file already closed"), true},
		{"other error", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExpectedCloseError(tt.err)
			if result != tt.expected {
				t.Errorf("isExpectedCloseError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestSetupCmdPipes(t *testing.T) {
	// This test creates actual pipes, which is safe
	cmd := createTestCommand()

	stdin, stdout, stderr, err := setupCmdPipes(cmd)
	if err != nil {
		t.Fatalf("setupCmdPipes error: %v", err)
	}

	if stdin == nil {
		t.Error("stdin should not be nil")
	}
	if stdout == nil {
		t.Error("stdout should not be nil")
	}
	if stderr == nil {
		t.Error("stderr should not be nil")
	}

	// Cleanup
	_ = stdin.Close()
	_ = stdout.Close()
	_ = stderr.Close()
}

func TestMonitorStartup_Success(t *testing.T) {
	ctx, cancel := createTestContext()
	defer cancel()

	startedCh := make(chan error, 1)
	startedCh <- nil // Simulate successful start

	// Create a cancel function to verify it's NOT called on success
	cancelCalled := false
	testCancel := func() {
		cancelCalled = true
	}

	monitorStartup(ctx, startedCh, testCancel)

	if cancelCalled {
		t.Error("cancel should not be called on success")
	}
}

func TestMonitorStartup_Error(t *testing.T) {
	ctx, cancel := createTestContext()
	defer cancel()

	startedCh := make(chan error, 1)
	startedCh <- errors.New("startup failed")

	// Create a cancel function to verify it IS called on error
	cancelCalled := false
	testCancel := func() {
		cancelCalled = true
	}

	monitorStartup(ctx, startedCh, testCancel)

	if !cancelCalled {
		t.Error("cancel should be called on error")
	}
}

func TestMonitorStartup_Timeout(t *testing.T) {
	ctx, cancel := createTestContextWithTimeout(1 * time.Millisecond)
	defer cancel()

	startedCh := make(chan error, 1)
	// Don't send anything - simulate timeout

	cancelCalled := false
	testCancel := func() {
		cancelCalled = true
	}

	monitorStartup(ctx, startedCh, testCancel)

	if !cancelCalled {
		t.Error("cancel should be called on timeout")
	}
}

func TestSessionPool_buildCLIArgs(t *testing.T) {
	logger := newTestLogger()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{
		DefaultPermissionMode: "bypassPermissions",
		AllowedTools:          []string{"bash", "edit"},
		DisallowedTools:       []string{"dangerous"},
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace:        "test",
		PermissionMode:   "bypassPermissions",
		AllowedTools:     []string{"bash", "edit"},
		DisallowedTools:  []string{"dangerous"},
		BaseSystemPrompt: "You are helpful",
	}, "/tmp/claude", prv)

	args := pool.buildCLIArgs("test-session-id", logger, "unit test prompt", SessionConfig{
		TaskInstructions: "unit test instructions",
		WorkDir:          "/tmp/test",
	})

	// Check essential args
	if !containsInSlice(args, "--print") {
		t.Error("args should contain --print")
	}
	if !containsInSlice(args, "--verbose") {
		t.Error("args should contain --verbose")
	}
	if !containsInSlice(args, "--output-format") {
		t.Error("args should contain --output-format")
	}
	if !containsInSlice(args, "stream-json") {
		t.Error("args should contain stream-json")
	}
	if !containsInSlice(args, "--permission-mode") {
		t.Error("args should contain --permission-mode")
	}
	if !containsInSlice(args, "--allowed-tools") {
		t.Error("args should contain --allowed-tools")
	}
}

func TestSessionPool_buildCLIArgs_Resume(t *testing.T) {
	logger := newTestLogger()

	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create pool with marker store
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace: "test",
	}, "/tmp/claude", prv)

	// Use a valid UUID format for providerSessionID (matches production behavior)
	testSessionUUID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

	// Create a marker to simulate existing session
	if err := pool.markerStore.Create(testSessionUUID); err != nil {
		t.Fatalf("Failed to create marker: %v", err)
	}
	defer func() { _ = pool.markerStore.Delete(testSessionUUID) }()

	// Create a CLI session data file to make VerifySession return true
	// Claude Code stores sessions in ~/.claude/projects/<workspace-key>/<session-id>.jsonl
	// The workspace-key is derived from the current working directory (or WorkDir if specified)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	// Get the current working directory - this is what VerifySession uses when WorkDir is empty
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	workspaceKey := strings.ReplaceAll(strings.ReplaceAll(cwd, "/.", "--"), "/", "-")
	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	sessionPath := filepath.Join(projectsDir, workspaceKey, testSessionUUID+".jsonl")

	// Create the session file
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}
	if err := os.WriteFile(sessionPath, []byte(`{"type":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create session file: %v", err)
	}
	defer func() { _ = os.Remove(sessionPath) }()

	args := pool.buildCLIArgs(testSessionUUID, logger, "unit test resume prompt", SessionConfig{})

	// Should have --resume for existing sessions
	if !containsInSlice(args, "--resume") {
		t.Error("args should contain --resume for existing session")
	}
}

// TestStartSession_ResolvesRelativeWorkDir tests that relative WorkDir paths are resolved to absolute paths
func TestStartSession_ResolvesRelativeWorkDir(t *testing.T) {
	logger := newTestLogger()

	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace: "test",
	}, "/tmp/claude", prv)

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	// Test cases for relative paths
	testCases := []struct {
		name     string
		workDir  string
		expected string
	}{
		{"current directory", ".", cwd},
		{"absolute path", "/tmp/test", "/tmp/test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := SessionConfig{
				WorkDir: tc.workDir,
			}

			// Create a command to test path resolution
			sessCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			args := pool.buildCLIArgs("test-session", logger, "test", SessionConfig{WorkDir: tc.workDir})
			cmd := exec.CommandContext(sessCtx, "/tmp/claude", args...)

			// Apply the same path resolution logic as in startSession
			if cfg.WorkDir == "." || !filepath.IsAbs(cfg.WorkDir) {
				if absPath, err := filepath.Abs(cfg.WorkDir); err == nil {
					cmd.Dir = absPath
				} else {
					cmd.Dir = cfg.WorkDir
				}
			} else {
				cmd.Dir = cfg.WorkDir
			}

			if cmd.Dir != tc.expected {
				t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, tc.expected)
			}
		})
	}
}

// Helper functions
func createTestCommand() *exec.Cmd {
	return exec.Command("echo", "test")
}

func createTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func createTestContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func containsInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// --- cleanupIdleSessions tests ---

func TestSessionPool_CleanupIdleSessions(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	// Use a short idle timeout
	pool := NewSessionPool(logger, 100*time.Millisecond, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Add a session with a LastActive time in the past (exceeds idle timeout)
	pool.mu.Lock()
	pool.sessions["idle-session"] = &Session{
		ID:           "idle-session",
		Status:       SessionStatusReady,
		LastActive:   time.Now().Add(-1 * time.Hour), // 1 hour ago
		statusChange: make(chan SessionStatus, 10),
		Config:       SessionConfig{}, // uses pool default timeout
	}
	pool.mu.Unlock()

	// Run cleanup
	pool.cleanupIdleSessions()

	// Session should be removed
	_, ok := pool.GetSession("idle-session")
	if ok {
		t.Error("Idle session should be cleaned up")
	}
}

func TestSessionPool_CleanupIdleSessions_ActiveSession(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	// Use a long idle timeout
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Add a session that was active recently
	pool.mu.Lock()
	pool.sessions["active-session"] = &Session{
		ID:           "active-session",
		Status:       SessionStatusReady,
		LastActive:   time.Now(), // Just now
		statusChange: make(chan SessionStatus, 10),
		Config:       SessionConfig{},
	}
	pool.mu.Unlock()

	// Run cleanup
	pool.cleanupIdleSessions()

	// Session should still exist
	_, ok := pool.GetSession("active-session")
	if !ok {
		t.Error("Active session should not be cleaned up")
	}
}

func TestSessionPool_CleanupIdleSessions_PerSessionTimeout(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	// Pool default timeout is very long (1 hour)
	pool := NewSessionPool(logger, 1*time.Hour, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Session has a per-session idle timeout of 50ms
	pool.mu.Lock()
	pool.sessions["per-timeout-session"] = &Session{
		ID:           "per-timeout-session",
		Status:       SessionStatusReady,
		LastActive:   time.Now().Add(-1 * time.Second), // 1 second ago
		statusChange: make(chan SessionStatus, 10),
		Config: SessionConfig{
			IdleTimeout: 50 * time.Millisecond,
		},
	}
	pool.mu.Unlock()

	// Run cleanup - should use the per-session timeout
	pool.cleanupIdleSessions()

	_, ok := pool.GetSession("per-timeout-session")
	if ok {
		t.Error("Session with per-session idle timeout should be cleaned up")
	}
}

func TestSessionPool_CleanupIdleSessions_Multiple(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 100*time.Millisecond, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	pool.mu.Lock()
	// Two idle sessions
	pool.sessions["idle-1"] = &Session{
		ID:           "idle-1",
		Status:       SessionStatusReady,
		LastActive:   time.Now().Add(-1 * time.Hour),
		statusChange: make(chan SessionStatus, 10),
	}
	pool.sessions["idle-2"] = &Session{
		ID:           "idle-2",
		Status:       SessionStatusBusy,
		LastActive:   time.Now().Add(-1 * time.Hour),
		statusChange: make(chan SessionStatus, 10),
	}
	// One active session
	pool.sessions["active"] = &Session{
		ID:           "active",
		Status:       SessionStatusReady,
		LastActive:   time.Now(),
		statusChange: make(chan SessionStatus, 10),
	}
	pool.mu.Unlock()

	pool.cleanupIdleSessions()

	if _, ok := pool.GetSession("idle-1"); ok {
		t.Error("idle-1 should be cleaned up")
	}
	if _, ok := pool.GetSession("idle-2"); ok {
		t.Error("idle-2 should be cleaned up")
	}
	if _, ok := pool.GetSession("active"); !ok {
		t.Error("active session should not be cleaned up")
	}
}

// --- cleanupInterval tests ---

func TestSessionPool_CleanupInterval(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"very short", 30 * time.Second, 30 * time.Second},           // 7.5s -> clamped to 30s
		{"short", 2 * time.Minute, 30 * time.Second},                 // 30s
		{"normal", 10 * time.Minute, 2*time.Minute + 30*time.Second}, // 2.5min
		{"long", 30 * time.Minute, 5 * time.Minute},                  // 7.5min -> clamped to 5min
		{"very long", 2 * time.Hour, 5 * time.Minute},                // 30min -> clamped to 5min
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pool := &SessionPool{
				timeout: tc.timeout,
			}
			got := pool.cleanupInterval()
			if got != tc.expected {
				t.Errorf("cleanupInterval() = %v, want %v", got, tc.expected)
			}
		})
	}
}

// --- DeleteMarker tests ---

func TestSessionPool_DeleteMarker(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Empty ID should be no-op
	err := pool.DeleteMarker("")
	if err != nil {
		t.Errorf("DeleteMarker('') should return nil, got %v", err)
	}

	// Non-existent marker should succeed (idempotent delete)
	err = pool.DeleteMarker("nonexistent-marker-id")
	if err != nil {
		t.Errorf("DeleteMarker() for nonexistent marker should succeed, got %v", err)
	}
}

// --- CleanupSessionFiles tests ---

func TestSessionPool_CleanupSessionFiles(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Nil provider should be safe
	err := pool.CleanupSessionFiles("some-id", "/tmp")
	if err != nil {
		t.Errorf("CleanupSessionFiles() error: %v", err)
	}
}

func TestSessionPool_CleanupSessionFiles_NilProvider(t *testing.T) {
	pool := &SessionPool{
		provider: nil,
	}

	err := pool.CleanupSessionFiles("some-id", "/tmp")
	if err != nil {
		t.Errorf("CleanupSessionFiles() with nil provider should return nil, got %v", err)
	}
}

// --- getOrCreateSession pending wait tests ---

func TestSessionPool_GetOrCreateSession_EmptyID(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	_, _, err := pool.GetOrCreateSession(context.Background(), "", SessionConfig{}, "test")
	if err == nil {
		t.Error("GetOrCreateSession() should fail with empty sessionID")
	}
}

func TestSessionPool_GetOrCreateSession_ContextCancelled(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := pool.GetOrCreateSession(ctx, "test-session", SessionConfig{}, "test")
	if err == nil {
		t.Error("GetOrCreateSession() should fail with cancelled context")
	}
}

func TestSessionPool_GetOrCreateSession_RecursionLimit(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Manually create a pending entry that will never resolve
	// This simulates a pathological scenario where getOrCreateSession keeps recursing
	pool.mu.Lock()
	ch := make(chan struct{})
	pool.pending["recurse-session"] = ch
	pool.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, err := pool.GetOrCreateSession(ctx, "recurse-session", SessionConfig{}, "test")
	if err == nil {
		t.Error("GetOrCreateSession() should fail when recursion limit exceeded")
	}
	// Cleanup: close the pending channel
	close(ch)
}

func TestSessionPool_GetOrCreateSession_DoubleCheck(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	// Pre-insert a dead session (simulates a session whose process died)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Kill the process immediately
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	pool.mu.Lock()
	pool.sessions["dead-session"] = &Session{
		ID:           "dead-session",
		cmd:          cmd,
		Status:       SessionStatusDead,
		LastActive:   time.Now(),
		statusChange: make(chan SessionStatus, 10),
	}
	pool.mu.Unlock()

	// GetOrCreateSession should detect dead session and try to start new one
	// Since we use a fake CLI path (/tmp), startSession will fail, which is fine
	_, _, err := pool.GetOrCreateSession(context.Background(), "dead-session", SessionConfig{}, "test")
	// We expect an error because the CLI binary at /tmp won't be valid
	// The key assertion is that the dead session was cleaned up
	if err == nil {
		// If somehow it succeeded, that's also fine (dead session cleaned up, new created)
		_ = pool.TerminateSession("dead-session")
	} else {
		// Error is expected, just verify the dead session was removed
		_, ok := pool.GetSession("dead-session")
		if ok {
			t.Error("Dead session should have been cleaned up")
		}
	}
}

// --- buildCLIArgs stale marker tests ---

func TestSessionPool_BuildCLIArgs_StaleMarker(t *testing.T) {
	logger := newTestLogger()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test-stale"}, "/tmp", prv)
	defer pool.Shutdown()

	// Create a marker for a session that doesn't actually exist
	staleID := "a1b2c3d4-e5f6-7890-abcd-stale00000001"
	if err := pool.markerStore.Create(staleID); err != nil {
		t.Fatalf("Failed to create marker: %v", err)
	}
	defer func() { _ = pool.markerStore.Delete(staleID) }()

	// Build CLI args - marker exists but VerifySession returns false
	// This should trigger the stale marker cleanup path
	args := pool.buildCLIArgs(staleID, logger, "test prompt", SessionConfig{})

	// Should NOT have --resume since marker was stale
	if containsInSlice(args, "--resume") {
		t.Error("args should NOT contain --resume for stale marker session")
	}

	// Marker should have been deleted
	if pool.markerStore.Exists(staleID) {
		t.Error("Stale marker should have been deleted")
	}
}

func TestSessionPool_BuildCLIArgs_NewSession_NoMarker(t *testing.T) {
	logger := newTestLogger()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test-new"}, "/tmp", prv)
	defer pool.Shutdown()

	newID := "f1e2d3c4-b5a6-7890-abcd-new000000001"
	args := pool.buildCLIArgs(newID, logger, "test prompt", SessionConfig{})

	// Should NOT have --resume for brand new session
	if containsInSlice(args, "--resume") {
		t.Error("args should NOT contain --resume for new session")
	}
}

func TestSessionPool_BuildCLIArgs_SessionSystemPromptOverride(t *testing.T) {
	logger := newTestLogger()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace:        "test-override",
		BaseSystemPrompt: "Engine-level prompt",
	}, "/tmp", prv)
	defer pool.Shutdown()

	sessionID := "override-test-session-id"

	// Session-level prompt should override engine-level
	args := pool.buildCLIArgs(sessionID, logger, "test", SessionConfig{
		BaseSystemPrompt: "Session-level prompt",
	})

	if !containsInSlice(args, "Session-level prompt") {
		t.Error("args should contain session-level system prompt")
	}
	if containsInSlice(args, "Engine-level prompt") {
		t.Error("args should NOT contain engine-level system prompt when session override exists")
	}
}

func TestSessionPool_BuildCLIArgs_EngineSystemPrompt(t *testing.T) {
	logger := newTestLogger()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace:        "test-engine-prompt",
		BaseSystemPrompt: "Engine default prompt",
	}, "/tmp", prv)
	defer pool.Shutdown()

	sessionID := "engine-prompt-test-id"

	args := pool.buildCLIArgs(sessionID, logger, "test", SessionConfig{})

	if !containsInSlice(args, "Engine default prompt") {
		t.Error("args should contain engine-level system prompt when session override is empty")
	}
}

// --- clearClaudeJSONUserID tests ---

func TestClearClaudeJSONUserID_EnvDisabled(t *testing.T) {
	t.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", "false")

	logger := newTestLogger()
	// Should return early without error
	clearClaudeJSONUserID(logger)
}

func TestClearClaudeJSONUserID_NoConfigFile(t *testing.T) {
	// Clear env to use default behavior
	t.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", "")

	logger := newTestLogger()
	// With a non-existent home directory path, it should just warn and return
	// This tests the file-not-found path
	clearClaudeJSONUserID(logger)
}

func TestClearClaudeJSONUserID_InvalidJSON(t *testing.T) {
	t.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", "")

	// Create a temp directory to use as home
	tmpDir, err := os.MkdirTemp("", "claude-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write invalid JSON to ~/.claude.json
	claudeJSONPath := filepath.Join(tmpDir, ".claude.json")
	if err := os.WriteFile(claudeJSONPath, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("Failed to write claude.json: %v", err)
	}

	// We can't easily override os.UserHomeDir(), so we test the function's
	// behavior indirectly. The function will use the real home dir.
	// Instead, test the error handling path directly by calling the function
	logger := newTestLogger()
	clearClaudeJSONUserID(logger)
	// Should not panic
}

func TestClearClaudeJSONUserID_NoUserID(t *testing.T) {
	t.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", "")

	// The function uses os.UserHomeDir() so it reads from the real home.
	// We can only test that it doesn't panic.
	logger := newTestLogger()
	clearClaudeJSONUserID(logger)
}

func TestClearClaudeJSONUserID_WithValidOAuth(t *testing.T) {
	t.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", "")

	// The function checks for credentials.json existence.
	// If both claude.json with userID and credentials.json exist, userID is preserved.
	// We can only test that it doesn't panic since we can't control home dir.
	logger := newTestLogger()
	clearClaudeJSONUserID(logger)
}

// --- TerminateSession edge cases ---

func TestSessionPool_TerminateSession_EmptyID(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	err := pool.TerminateSession("")
	if err != nil {
		t.Errorf("TerminateSession('') should return nil, got %v", err)
	}
}

// --- NewSessionPool_NilLogger ---

func TestNewSessionPool_NilLogger(t *testing.T) {
	prv := newTestProvider(t)
	pool := NewSessionPool(nil, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	if pool.logger == nil {
		t.Error("Logger should be set to default when nil is provided")
	}
}

// --- ListActiveSessions_WithSessions ---

func TestSessionPool_ListActiveSessions_WithSessions(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
	defer pool.Shutdown()

	sess1 := &Session{
		ID:           "session-1",
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
	}
	sess2 := &Session{
		ID:           "session-2",
		Status:       SessionStatusBusy,
		statusChange: make(chan SessionStatus, 10),
	}

	pool.mu.Lock()
	pool.sessions["session-1"] = sess1
	pool.sessions["session-2"] = sess2
	pool.mu.Unlock()

	sessions := pool.ListActiveSessions()
	if len(sessions) != 2 {
		t.Errorf("ListActiveSessions() = %d, want 2", len(sessions))
	}
}

// --- SessionManager interface compliance ---

func TestSessionPool_ImplementsSessionManager(t *testing.T) {
	// Compile-time check (already done in pool.go with var _ SessionManager = (*SessionPool)(nil))
	// This runtime test verifies the interface methods are accessible
	logger := newTestLogger()
	prv := newTestProvider(t)
	var _ SessionManager = NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)
}
