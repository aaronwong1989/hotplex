package engine

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hrygo/hotplex/provider"
)

// newTestLogger creates a logger for testing
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

// newTestProvider creates a ClaudeCodeProvider for testing
func newTestProvider(t *testing.T) provider.Provider {
	t.Helper()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, newTestLogger())
	if err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}
	return prv
}

func TestSessionStatus_String(t *testing.T) {
	statuses := []SessionStatus{
		SessionStatusStarting,
		SessionStatusReady,
		SessionStatusBusy,
		SessionStatusDead,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("SessionStatus %v has empty string representation", s)
		}
	}
}

func TestSession_IsAlive_NilProcess(t *testing.T) {
	sess := &Session{
		Status: SessionStatusStarting,
	}

	if sess.IsAlive() {
		t.Error("IsAlive() should return false for nil process")
	}
}

func TestSession_Touch(t *testing.T) {
	sess := &Session{
		LastActive: time.Time{}, // Zero time
	}

	before := time.Now()
	sess.Touch()
	after := time.Now()

	if sess.LastActive.Before(before) || sess.LastActive.After(after) {
		t.Errorf("Touch() didn't update LastActive correctly: %v", sess.LastActive)
	}
}

func TestSession_SetStatus(t *testing.T) {
	sess := &Session{
		Status:       SessionStatusStarting,
		statusChange: make(chan SessionStatus, 10),
	}

	sess.SetStatus(SessionStatusReady)

	if sess.Status != SessionStatusReady {
		t.Errorf("Status = %v, want ready", sess.Status)
	}

	// Check status change was broadcast
	select {
	case s := <-sess.statusChange:
		if s != SessionStatusReady {
			t.Errorf("statusChange = %v, want ready", s)
		}
	default:
		t.Error("statusChange channel should have received status update")
	}
}

func TestSession_GetStatus(t *testing.T) {
	sess := &Session{
		Status: SessionStatusBusy,
	}

	if sess.GetStatus() != SessionStatusBusy {
		t.Errorf("GetStatus() = %v, want busy", sess.GetStatus())
	}
}

func TestSession_SetCallback(t *testing.T) {
	sess := &Session{}

	cb := func(eventType string, data any) error { return nil }
	sess.SetCallback(cb)

	if sess.callback == nil {
		t.Error("SetCallback() didn't set callback")
	}
}

func TestSession_GetCallback(t *testing.T) {
	cb := func(eventType string, data any) error { return nil }
	sess := &Session{
		callback: cb,
	}

	got := sess.GetCallback()
	if got == nil {
		t.Error("GetCallback() returned nil")
	}
}

func TestSession_GetLastActive(t *testing.T) {
	now := time.Now()
	sess := &Session{
		LastActive: now,
	}

	got := sess.GetLastActive()
	if !got.Equal(now) {
		t.Errorf("GetLastActive() = %v, want %v", got, now)
	}
}

func TestSession_GetStatusChange(t *testing.T) {
	ch := make(chan SessionStatus, 10)
	sess := &Session{
		statusChange: ch,
	}

	got := sess.GetStatusChange()
	if got == nil {
		t.Error("GetStatusChange() returned nil")
	}
}

func TestSession_SetExt_GetExt(t *testing.T) {
	sess := &Session{}

	// Set extension data
	data := map[string]string{"key": "value"}
	sess.SetExt(data)

	// Get extension data
	got := sess.GetExt()
	if got == nil {
		t.Error("GetExt() returned nil")
	}

	gotMap, ok := got.(map[string]string)
	if !ok {
		t.Fatalf("GetExt() returned wrong type: %T", got)
	}
	if gotMap["key"] != "value" {
		t.Errorf("GetExt() = %v, want {key: value}", gotMap)
	}
}

func TestSession_WriteInput_InvalidJSON(t *testing.T) {
	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
	}

	// WriteInput should handle invalid types gracefully
	// (functions cannot be marshaled to JSON)
	msg := map[string]any{
		"func": func() {},
	}

	err := sess.WriteInput(msg)
	if err == nil {
		t.Error("WriteInput() should fail for unmarshalable data")
	}
}

func TestSessionPool_GetSession(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Get nonexistent session
	_, ok := pool.GetSession("nonexistent")
	if ok {
		t.Error("GetSession() should return false for nonexistent session")
	}
}

func TestSessionPool_ListActiveSessions(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Should return empty list
	sessions := pool.ListActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("ListActiveSessions() = %d sessions, want 0", len(sessions))
	}
}

func TestSessionPool_TerminateSession_Nonexistent(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Terminating nonexistent session should be a no-op
	err := pool.TerminateSession("nonexistent")
	if err != nil {
		t.Errorf("TerminateSession() error: %v", err)
	}
}

func TestSessionPool_Shutdown(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Shutdown should be safe to call
	pool.Shutdown()

	// Second shutdown should be safe (idempotent)
	pool.Shutdown()
}

func TestSession_close(t *testing.T) {
	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
	}

	// Call close
	sess.close()

	if sess.Status != SessionStatusDead {
		t.Errorf("Status = %v, want dead", sess.Status)
	}

	// Channel should be closed
	select {
	case _, ok := <-sess.statusChange:
		if ok {
			t.Error("statusChange channel should be closed")
		}
	default:
		t.Error("statusChange channel should be closed")
	}
}

func TestSession_close_Idempotent(t *testing.T) {
	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
	}

	// Call close twice - should not panic
	sess.close()
	sess.close() // Second call should be safe

	if sess.Status != SessionStatusDead {
		t.Errorf("Status = %v, want dead", sess.Status)
	}
}

func TestSession_WriteInput_Valid(t *testing.T) {
	// Create a pipe to capture stdin
	r, w := io.Pipe()
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		stdin:        w,
	}

	// Write valid JSON
	msg := map[string]any{
		"type":    "user",
		"message": "hello",
	}

	// Read in goroutine
	var received []byte
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		received = buf[:n]
		close(done)
	}()

	err := sess.WriteInput(msg)
	if err != nil {
		t.Errorf("WriteInput() error: %v", err)
	}

	// Wait for read
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for stdin write")
	}

	// Verify message was written (just check it's not empty)
	if len(received) == 0 {
		t.Error("Expected some data to be written to stdin")
	}
}

func TestSession_SetStatus_ClosedChannel(t *testing.T) {
	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		closed:       true, // Already closed
	}

	// Should not panic or block
	sess.SetStatus(SessionStatusBusy)

	// Status should still be updated
	if sess.Status != SessionStatusBusy {
		t.Errorf("Status = %v, want busy", sess.Status)
	}
}

func TestSession_isAliveLocked(t *testing.T) {
	t.Run("nil cmd", func(t *testing.T) {
		sess := &Session{
			Status: SessionStatusReady,
		}
		if sess.isAliveLocked() {
			t.Error("isAliveLocked should return false for nil cmd")
		}
	})

	t.Run("dead status", func(t *testing.T) {
		sess := &Session{
			Status: SessionStatusDead,
		}
		if sess.isAliveLocked() {
			t.Error("isAliveLocked should return false for dead status")
		}
	})
}

func TestSessionPool_Shutdown_WithSessions(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Add mock sessions directly to the pool
	pool.mu.Lock()
	pool.sessions["session-1"] = &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
	}
	pool.sessions["session-2"] = &Session{
		Status:       SessionStatusBusy,
		statusChange: make(chan SessionStatus, 10),
	}
	pool.mu.Unlock()

	// Shutdown should clean up all sessions
	pool.Shutdown()

	// Verify all sessions are removed
	pool.mu.RLock()
	if len(pool.sessions) != 0 {
		t.Errorf("Expected 0 sessions after shutdown, got %d", len(pool.sessions))
	}
	pool.mu.RUnlock()
}

func TestSessionPool_CleanupSessionLocked(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Add mock session
	pool.mu.Lock()
	pool.sessions["test-session"] = &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
	}
	pool.mu.Unlock()

	// Cleanup the session
	pool.mu.Lock()
	err := pool.cleanupSessionLocked("test-session")
	pool.mu.Unlock()

	if err != nil {
		t.Errorf("cleanupSessionLocked error: %v", err)
	}

	// Verify session is removed
	pool.mu.RLock()
	if _, ok := pool.sessions["test-session"]; ok {
		t.Error("Session should be removed after cleanup")
	}
	pool.mu.RUnlock()

	pool.Shutdown()
}

func TestSessionPool_CleanupSessionLocked_NonExistent(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Cleanup non-existent session should return nil
	pool.mu.Lock()
	err := pool.cleanupSessionLocked("non-existent")
	pool.mu.Unlock()

	if err != nil {
		t.Errorf("cleanupSessionLocked for non-existent session should return nil, got %v", err)
	}

	pool.Shutdown()
}

func TestSessionPool_Shutdown_WithCallback(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	callbackCalled := false
	cb := func(eventType string, data any) error {
		if eventType == "runner_exit" {
			callbackCalled = true
		}
		return nil
	}

	// Add mock session with callback
	pool.mu.Lock()
	pool.sessions["test-session"] = &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		callback:     cb,
	}
	pool.mu.Unlock()

	// Shutdown should call runner_exit callback
	pool.Shutdown()

	if !callbackCalled {
		t.Error("Expected runner_exit callback to be called")
	}
}

func TestSessionPool_ListActiveSessions_Multiple(t *testing.T) {
	logger := newTestLogger()
	prv := newTestProvider(t)
	pool := NewSessionPool(logger, 30*time.Minute, EngineOptions{Namespace: "test"}, "/tmp", prv)

	// Initially empty
	sessions := pool.ListActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("ListActiveSessions() = %d, want 0", len(sessions))
	}

	pool.Shutdown()
}

func TestEngineOptions_Defaults(t *testing.T) {
	opts := EngineOptions{}

	// Check zero values
	if opts.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0", opts.Timeout)
	}
	if opts.IdleTimeout != 0 {
		t.Errorf("IdleTimeout = %v, want 0", opts.IdleTimeout)
	}
	if opts.Namespace != "" {
		t.Errorf("Namespace = %q, want empty", opts.Namespace)
	}
}

// TestSession_ReadStderr_LogFileWriteError tests error handling when log file write fails
func TestSession_ReadStderr_LogFileWriteError(t *testing.T) {
	logger := newTestLogger()

	// Create a pipe to simulate stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Create session with a read-only log file (writing will fail)
	se := &Session{
		ID:                "test-stderr-error",
		ProviderSessionID: "test-provider",
		Status:            SessionStatusBusy,
		logger:            logger,
		stderr:            r,
		statusChange:      make(chan SessionStatus, 10),
	}

	// Create a temp file and open it read-only so writes will fail
	tmpFile, err := os.CreateTemp("", "session-log-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Open read-only to cause write error
	se.logFile, err = os.Open(tmpPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	// Clean up after test
	if se.logFile != nil {
		defer func() { _ = se.logFile.Close() }()
	}
	defer func() { _ = os.Remove(tmpPath) }()

	// Write to pipe and close it to trigger scanner
	go func() {
		if _, err := w.Write([]byte("test error line\n")); err != nil {
			t.Logf("Write error (expected in goroutine): %v", err)
		}
		if err := w.Close(); err != nil {
			t.Logf("Close error (expected in goroutine): %v", err)
		}
	}()

	// Call ReadStderr - should handle the write error gracefully
	se.ReadStderr()
}

// --- waitForReady tests ---

func TestSession_waitForReady_TransitionsToReady(t *testing.T) {
	// Create a session with a truly alive process (sleep is short-lived but alive during check)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sleep process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	sess := &Session{
		ID:           "wait-ready-alive",
		cmd:          cmd,
		Status:       SessionStatusStarting,
		statusChange: make(chan SessionStatus, 10),
		logger:       newTestLogger(),
	}

	sess.waitForReady(ctx, 3*time.Second)

	// Wait for transition
	select {
	case status := <-sess.GetStatusChange():
		if status != SessionStatusReady {
			t.Errorf("Expected Ready status, got %v", status)
		}
	case <-time.After(5 * time.Second):
		// Check current status even if channel didn't receive
		if sess.GetStatus() != SessionStatusReady {
			t.Errorf("waitForReady timed out, status = %v", sess.GetStatus())
		}
	}
}

func TestSession_waitForReady_Timeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Session with nil cmd -> isAliveLocked returns false -> will timeout
	sess := &Session{
		ID:           "wait-ready-timeout",
		cmd:          nil,
		Status:       SessionStatusStarting,
		statusChange: make(chan SessionStatus, 10),
		logger:       newTestLogger(),
	}

	sess.waitForReady(ctx, 200*time.Millisecond)

	// Give it time to process
	time.Sleep(300 * time.Millisecond)

	if sess.GetStatus() != SessionStatusDead {
		t.Errorf("Expected Dead status after timeout, got %v", sess.GetStatus())
	}
}

func TestSession_waitForReady_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	sess := &Session{
		ID:           "wait-ready-cancel",
		cmd:          nil,
		Status:       SessionStatusStarting,
		statusChange: make(chan SessionStatus, 10),
		logger:       newTestLogger(),
	}

	// Cancel immediately
	cancel()

	sess.waitForReady(ctx, 5*time.Second)

	// Should not transition to Ready or Dead since context was cancelled first
	time.Sleep(100 * time.Millisecond)
	if sess.GetStatus() != SessionStatusStarting {
		// Starting is fine - context cancelled before deadline
		t.Logf("Status after context cancel: %v (acceptable)", sess.GetStatus())
	}
}

func TestSession_waitForReady_AlreadyDead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sess := &Session{
		ID:           "wait-ready-dead",
		cmd:          nil,
		Status:       SessionStatusDead, // Already dead
		statusChange: make(chan SessionStatus, 10),
		logger:       newTestLogger(),
	}

	sess.waitForReady(ctx, 1*time.Second)

	// Should remain dead
	time.Sleep(100 * time.Millisecond)
	if sess.GetStatus() != SessionStatusDead {
		t.Errorf("Expected Dead status, got %v", sess.GetStatus())
	}
}

// --- ReadStdout tests ---

func TestSession_ReadStdout_DispatchesLines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	var mu sync.Mutex
	var receivedLines []string

	cb := func(eventType string, data any) error {
		mu.Lock()
		defer mu.Unlock()
		if eventType == "raw_line" {
			if line, ok := data.(string); ok {
				receivedLines = append(receivedLines, line)
			}
		}
		return nil
	}

	sess := &Session{
		ID:           "test-read-stdout",
		stdout:       r,
		logger:       newTestLogger(),
		callback:     cb,
		statusChange: make(chan SessionStatus, 10),
	}

	// Write some lines then close
	go func() {
		_, _ = w.Write([]byte(`{"type":"assistant","content":"hello"}
{"type":"result","content":"done"}` + "\n"))
		_ = w.Close()
	}()

	sess.ReadStdout()

	mu.Lock()
	defer mu.Unlock()
	if len(receivedLines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %v", len(receivedLines), receivedLines)
	}
}

func TestSession_ReadStdout_NilStdout(t *testing.T) {
	sess := &Session{
		ID:     "test-read-stdout-nil",
		stdout: nil,
		logger: newTestLogger(),
	}

	// Should not panic
	sess.ReadStdout()
}

func TestSession_ReadStdout_CallsRunnerExitOnCompletion(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	var mu sync.Mutex
	callbackEvents := []string{}

	cb := func(eventType string, data any) error {
		mu.Lock()
		defer mu.Unlock()
		callbackEvents = append(callbackEvents, eventType)
		return nil
	}

	sess := &Session{
		ID:           "test-exit-callback",
		stdout:       r,
		logger:       newTestLogger(),
		callback:     cb,
		statusChange: make(chan SessionStatus, 10),
	}

	go func() {
		_ = w.Close()
	}()

	sess.ReadStdout()

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, ev := range callbackEvents {
		if ev == "runner_exit" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected runner_exit callback, got events: %v", callbackEvents)
	}
}

func TestSession_ReadStdout_SkipsEmptyLines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	var mu sync.Mutex
	lineCount := 0

	cb := func(eventType string, data any) error {
		mu.Lock()
		defer mu.Unlock()
		if eventType == "raw_line" {
			lineCount++
		}
		return nil
	}

	sess := &Session{
		ID:           "test-empty-lines",
		stdout:       r,
		logger:       newTestLogger(),
		callback:     cb,
		statusChange: make(chan SessionStatus, 10),
	}

	go func() {
		_, _ = w.Write([]byte("line1\n\n\nline2\n"))
		_ = w.Close()
	}()

	sess.ReadStdout()

	mu.Lock()
	defer mu.Unlock()
	if lineCount != 2 {
		t.Errorf("Expected 2 non-empty lines, got %d", lineCount)
	}
}

func TestSession_ReadStdout_NilCallback(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	sess := &Session{
		ID:           "test-nil-callback",
		stdout:       r,
		logger:       newTestLogger(),
		callback:     nil,
		statusChange: make(chan SessionStatus, 10),
	}

	go func() {
		_, _ = w.Write([]byte(`{"type":"assistant","content":"data"}` + "\n"))
		_ = w.Close()
	}()

	// Should not panic with nil callback
	sess.ReadStdout()
}

func TestSession_ReadStderr_NilStderr(t *testing.T) {
	sess := &Session{
		ID:     "test-read-stderr-nil",
		stderr: nil,
		logger: newTestLogger(),
	}

	// Should not panic
	sess.ReadStderr()
}

func TestSession_ReadStderr_NilLogger(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	sess := &Session{
		ID:     "test-nil-logger",
		stderr: r,
		logger: nil,
	}

	go func() {
		_, _ = w.Write([]byte("error line\n"))
		_ = w.Close()
	}()

	// Should not panic with nil logger
	sess.ReadStderr()
}

// --- close with pipes ---

func TestSession_close_WithPipes(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "session-close-test-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	logFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		stdin:        stdinW,
		stdout:       stdoutR,
		stderr:       stderrR,
		logFile:      logFile,
	}

	// Close the opposite ends to simulate process closing its stdout/stderr writes
	// and simulate nobody reading from process stdin
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); _ = stdinR.Close() }()
	go func() { defer wg.Done(); _ = stdoutW.Close() }()
	go func() { defer wg.Done(); _ = stderrW.Close() }()

	sess.close()

	wg.Wait()

	if sess.Status != SessionStatusDead {
		t.Errorf("Status = %v, want dead", sess.Status)
	}
	if sess.closed != true {
		t.Error("closed flag should be true")
	}
	// Verify statusChange channel is closed
	select {
	case _, ok := <-sess.statusChange:
		if ok {
			t.Error("statusChange should be closed")
		}
	default:
		t.Error("statusChange should be closed")
	}
}

// --- NewTestSession ---

func TestNewTestSession(t *testing.T) {
	sess := NewTestSession("test-id", SessionStatusReady)

	if sess.ID != "test-id" {
		t.Errorf("ID = %q, want %q", sess.ID, "test-id")
	}
	if sess.ProviderSessionID != "test-provider-session" {
		t.Errorf("ProviderSessionID = %q, want %q", sess.ProviderSessionID, "test-provider-session")
	}
	if sess.Status != SessionStatusReady {
		t.Errorf("Status = %v, want ready", sess.Status)
	}
	if sess.statusChange == nil {
		t.Error("statusChange should not be nil")
	}
}

// --- OpenLogFile ---

func TestSession_OpenLogFile(t *testing.T) {
	// Save original SessionLogDir and restore after test
	origDir := SessionLogDir
	defer func() { SessionLogDir = origDir }()

	tmpDir, err := os.MkdirTemp("", "session-log-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	SessionLogDir = tmpDir

	sess := &Session{
		ID:           "test-log-session",
		statusChange: make(chan SessionStatus, 10),
	}

	err = sess.OpenLogFile()
	if err != nil {
		t.Fatalf("OpenLogFile() error: %v", err)
	}

	if sess.logFile == nil {
		t.Error("logFile should not be nil after OpenLogFile")
	}

	// Second call should be no-op (idempotent)
	err = sess.OpenLogFile()
	if err != nil {
		t.Errorf("OpenLogFile() second call error: %v", err)
	}

	// Clean up
	_ = sess.logFile.Close()
}

func TestSession_OpenLogFile_InvalidSessionID(t *testing.T) {
	origDir := SessionLogDir
	defer func() { SessionLogDir = origDir }()

	tmpDir, err := os.MkdirTemp("", "session-log-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	SessionLogDir = tmpDir

	tests := []struct {
		name string
		id   string
	}{
		{"dot", "."},
		{"dotdot", ".."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sess := &Session{
				ID:           tc.id,
				statusChange: make(chan SessionStatus, 10),
			}

			err := sess.OpenLogFile()
			if err == nil {
				t.Error("OpenLogFile() should fail for invalid session ID")
			}
		})
	}
}

// --- GetLogPath ---

func TestSession_GetLogPath(t *testing.T) {
	origDir := SessionLogDir
	defer func() { SessionLogDir = origDir }()

	SessionLogDir = "/tmp/test-logs"

	sess := &Session{
		ID: "my-session",
	}

	expected := "/tmp/test-logs/my-session.log"
	got := sess.GetLogPath()

	if got != expected {
		t.Errorf("GetLogPath() = %q, want %q", got, expected)
	}
}

// --- writeToLogFile ---

func TestSession_writeToLogFile_NilLogFile(t *testing.T) {
	sess := &Session{
		ID:           "test-nil-logfile",
		logger:       newTestLogger(),
		statusChange: make(chan SessionStatus, 10),
	}

	// Should not panic with nil logFile
	sess.writeToLogFile("test line")
}

func TestSession_writeToLogFile_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "session-write-log-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	sess := &Session{
		ID:           "test-write-success",
		logger:       newTestLogger(),
		logFile:      tmpFile,
		statusChange: make(chan SessionStatus, 10),
	}

	sess.writeToLogFile("test message line")

	_ = tmpFile.Close()

	// Read back and verify
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), "test message line") {
		t.Errorf("Log file does not contain expected message, got: %s", string(data))
	}
}

// --- isAliveLocked with live process ---

func TestSession_isAliveLocked_LiveProcess(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	sess := &Session{
		cmd:    cmd,
		Status: SessionStatusReady,
	}

	if !sess.isAliveLocked() {
		t.Error("isAliveLocked should return true for live process")
	}
}

func TestSession_isAliveLocked_NilProcessPtr(t *testing.T) {
	sess := &Session{
		cmd:    &exec.Cmd{},
		Status: SessionStatusReady,
	}

	if sess.isAliveLocked() {
		t.Error("isAliveLocked should return false for nil Process")
	}
}

// --- SetStatus channel full ---

func TestSession_SetStatus_ChannelFull(t *testing.T) {
	// Use unbuffered channel (capacity 0) so default case triggers
	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 1),
	}

	// Fill the channel
	sess.statusChange <- SessionStatusStarting

	// Now SetStatus should not block even though channel is full
	sess.SetStatus(SessionStatusBusy)

	if sess.GetStatus() != SessionStatusBusy {
		t.Errorf("Status = %v, want busy", sess.GetStatus())
	}
}

// --- WriteInput updates LastActive ---

func TestSession_WriteInput_UpdatesLastActive(t *testing.T) {
	r, w := io.Pipe()
	defer func() { _ = r.Close() }()

	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		stdin:        w,
		LastActive:   time.Now().Add(-1 * time.Hour),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		_, _ = r.Read(buf)
	}()

	msg := map[string]any{"type": "user", "message": "test"}
	_ = sess.WriteInput(msg)

	wg.Wait()

	if sess.LastActive.Before(time.Now().Add(-1 * time.Minute)) {
		t.Error("WriteInput should update LastActive")
	}
}

func TestSession_WriteInput_NilStdin_Panics(t *testing.T) {
	// WriteInput does not guard against nil stdin - it panics.
	// This test documents that behavior.
	defer func() {
		if r := recover(); r == nil {
			t.Error("WriteInput() should panic with nil stdin")
		}
	}()

	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		stdin:        nil,
	}

	msg := map[string]any{"type": "user", "message": "test"}
	_ = sess.WriteInput(msg)
}

func TestSession_WriteInput_SetsBusyStatus(t *testing.T) {
	r, w := io.Pipe()
	defer func() { _ = r.Close(); _ = w.Close() }()

	sess := &Session{
		Status:       SessionStatusReady,
		statusChange: make(chan SessionStatus, 10),
		stdin:        w,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		_, _ = r.Read(buf)
	}()

	msg := map[string]any{"type": "user", "message": "test"}
	_ = sess.WriteInput(msg)
	wg.Wait()

	if sess.Status != SessionStatusBusy {
		t.Errorf("Status = %v, want busy", sess.Status)
	}
}

// --- ReadStderr reads and logs lines ---

func TestSession_ReadStderr_DispatchesLines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "stderr-test-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp log: %v", err)
	}
	logPath := tmpFile.Name()
	_ = tmpFile.Close()

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open log: %v", err)
	}
	defer func() {
		_ = logFile.Close()
		_ = os.Remove(logPath)
	}()

	sess := &Session{
		ID:                "stderr-dispatch",
		ProviderSessionID: "stderr-provider",
		Config:            SessionConfig{WorkDir: "/tmp/test"},
		stderr:            r,
		logger:            newTestLogger(),
		logFile:           logFile,
		statusChange:      make(chan SessionStatus, 10),
	}

	go func() {
		_, _ = w.Write([]byte("error line 1\nerror line 2\n"))
		_ = w.Close()
	}()

	sess.ReadStderr()

	_ = logFile.Close()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "error line 1") || !strings.Contains(content, "error line 2") {
		t.Errorf("Log file should contain error lines, got: %s", content)
	}
}

func TestSession_ReadStderr_SkipsEmptyLines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	lineCount := 0
	tmpFile, err := os.CreateTemp("", "stderr-skip-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp log: %v", err)
	}
	logPath := tmpFile.Name()
	_ = tmpFile.Close()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open log: %v", err)
	}

	sess := &Session{
		ID:           "stderr-skip-empty",
		stderr:       r,
		logger:       newTestLogger(),
		logFile:      logFile,
		statusChange: make(chan SessionStatus, 10),
	}

	go func() {
		_, _ = w.Write([]byte("line1\n\n\nline2\n"))
		_ = w.Close()
	}()

	sess.ReadStderr()

	_ = logFile.Close()
	data, _ := os.ReadFile(logPath)
	_ = os.Remove(logPath)

	// Each non-empty line should produce a log entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lineCount++
		}
	}

	if lineCount != 2 {
		t.Errorf("Expected 2 non-empty log entries, got %d", lineCount)
	}
}
