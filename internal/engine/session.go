package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hrygo/hotplex/internal/panicx"
	"github.com/hrygo/hotplex/internal/sys"
)

// SessionLogDir is the directory for session log files.
// Defaults to ~/.hotplex/logs/
var SessionLogDir = filepath.Join(os.Getenv("HOME"), ".hotplex", "logs")

// Session represents a persistent, hot-multiplexed instance of an AI CLI agent.
// It manages the underlying OS process group, handles streaming I/O via full-duplex pipes,
// and tracks the operational readiness and lifecycle status of the agent sandbox.
type Session struct {
	ID                string        // Internal SDK identifier (provided by the user)
	ProviderSessionID string        // The deterministic UUID (v5) passed to CLI for persistent DB storage
	Config            SessionConfig // Snapshot of the configuration used to initialize the session
	TaskInstructions  string        // Persistent instructions for the session

	// io abstracts the I/O transport (CLI pipes or HTTP/SSE).
	// It replaces the individual stdin/stdout/stderr/cancel/cmd/jobHandle fields.
	io SessionIO

	// CLI-specific fields (used only when io.IsCLI() == true).
	// These are set by CLISessionStarter and cleaned up via sys.KillProcessGroup.
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	jobHandle uintptr // Windows Job Object handle (0 on Unix)

	CreatedAt    time.Time
	LastActive   time.Time
	Status       SessionStatus
	statusChange chan SessionStatus

	// reapOnce ensures cmd.Wait() is called exactly once — both the synchronous
	// cleanup path (cleanupSessionLocked) and the async SafeGo goroutine call
	// Wait(), and sync.Once guarantees only the first caller executes.
	reapOnce sync.Once

	mu     sync.RWMutex
	closed bool

	callback   Callback
	logger     *slog.Logger
	logFile    *os.File // Session-specific log file for stderr persistence
	ext        any      // Extension payload for consumer packages
	IsResuming            bool   // True if session was resumed from persistent marker
	FirstMessageOnSession bool   // True until the first BuildInputMessage is sent (for HTTP hot-multiplexing gate)
}

// IsAlive checks if the process is still running.
func (s *Session) IsAlive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isAliveLocked()
}

// Wait reaps the process exactly once using sync.Once. Both the synchronous
// cleanup path (cleanupSessionLocked) and the async SafeGo goroutine (startSession)
// call this method; sync.Once ensures only the first call executes cmd.Wait(),
// and subsequent calls return immediately without contending for the
// exec.Cmd internal mutex or duplicating the reap.
func (s *Session) Wait() error {
	var err error
	s.reapOnce.Do(func() {
		if s.cmd != nil {
			err = s.cmd.Wait() //nolint:errcheck
		}
	})
	return err
}

// isAliveLocked checks if the session is still active. Caller must hold lock.
// For CLI sessions: checks if the process is still running.
// For HTTP sessions: checks if the SessionIO is still open.
func (s *Session) isAliveLocked() bool {
	if s.Status == SessionStatusDead {
		return false
	}
	if s.io != nil {
		return s.io.IsAlive()
	}
	// Fallback for tests that manually construct Session with cmd
	if s.cmd != nil && s.cmd.Process != nil {
		return sys.IsProcessAlive(s.cmd.Process)
	}
	return false
}

// Touch updates LastActive time.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActive = time.Now()
}

// GetLastActive returns the last active time with proper locking.
func (s *Session) GetLastActive() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastActive
}

// SetStatus updates the session status with proper locking.
func (s *Session) SetStatus(status SessionStatus) {
	s.mu.Lock()
	s.Status = status
	if s.closed {
		s.mu.Unlock()
		return
	}
	select {
	case s.statusChange <- status:
	default:
	}
	s.mu.Unlock()
}

// GetStatus returns the current session status.
func (s *Session) GetStatus() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// GetStatusChange returns the status change channel for waiting on status updates.
func (s *Session) GetStatusChange() <-chan SessionStatus {
	return s.statusChange
}

// waitForReady monitors the session and transitions from Starting to Ready
// when the process is confirmed alive and responsive.
func (s *Session) waitForReady(ctx context.Context, timeout time.Duration) {
	panicx.SafeGo(s.logger, func() {
		deadlineTimer := time.NewTimer(timeout)
		defer deadlineTimer.Stop()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-deadlineTimer.C:
				s.mu.Lock()
				if s.Status == SessionStatusStarting {
					s.logger.Warn("waitForReady: timeout, marking session as dead")
					s.Status = SessionStatusDead
				}
				s.mu.Unlock()
				return
			case <-ticker.C:
				s.mu.Lock()
				if s.Status == SessionStatusDead {
					s.mu.Unlock()
					return
				}
				alive := s.isAliveLocked()
				if alive {
					s.Status = SessionStatusReady
					s.logger.Info("Session ready")
					if !s.closed {
						select {
						case s.statusChange <- SessionStatusReady:
						default:
						}
					}
					s.mu.Unlock()
					return
				}
				s.mu.Unlock()
			}
		}
	})
}

// WriteInput injects a JSON message via the SessionIO transport.
func (s *Session) WriteInput(msg map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = SessionStatusBusy
	select {
	case s.statusChange <- SessionStatusBusy:
	default:
	}

	if err := s.io.WriteInput(msg); err != nil {
		if s.logger != nil {
			s.logger.Error("WriteInput failed", "error", err)
		}
		return fmt.Errorf("session write: %w", err)
	}

	s.LastActive = time.Now()
	return nil
}

// close releases resources held by the session.
// IMPORTANT: Caller must hold s.mu lock before calling this method.
func (s *Session) close() {
	s.Status = SessionStatusDead

	// Delegate to SessionIO for resource cleanup.
	// For CLISessionIO: closes stdin/stdout/stderr pipes.
	// For HTTPSessionIO: cancels SSE context and deletes server session.
	if s.io != nil {
		_ = s.io.Close()
	}

	// Close session log file.
	if s.logFile != nil {
		_ = s.logFile.Close()
		s.logFile = nil
	}

	if !s.closed {
		s.closed = true
		close(s.statusChange)
	}
}

// SetCallback registers the callback to handle stream events for the current turn.
func (s *Session) SetCallback(cb Callback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callback = cb
}

// SetIOCallback propagates the callback to the underlying HTTP I/O layer.
// This is required for HTTP sessions where HTTPSessionIO.StartReading() is blocked
// by a gate until SetCallback is called. Without this, StartReading() times out
// after 30 seconds and all SSE events are silently dropped.
func (s *Session) SetIOCallback(cb Callback) {
	if s.io == nil {
		return
	}
	// HTTPSessionIO has its own SetCallback which closes the startReadingGate.
	// CLISessionIO does not have this method (it uses Session.callback directly).
	if httpIO, ok := s.io.(*HTTPSessionIO); ok {
		httpIO.SetCallback(func(eventType string, data any) error { return cb(eventType, data) })
	}
}

// GetCallback returns the current callback.
func (s *Session) GetCallback() Callback {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.callback
}

// SetExt attaches external state to the session.
func (s *Session) SetExt(data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ext = data
}

// GetExt retrieves the external state attached to the session.
func (s *Session) GetExt() any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ext
}

// isExpectedCloseError checks if the error is an expected pipe closure during normal shutdown.
func isExpectedCloseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if strings.Contains(err.Error(), "file already closed") {
		return true
	}
	return false
}

// ReadStdout asynchronously reads CLI stdout, parses JSON, and dispatches callbacks.
// For CLI sessions, this reads from the subprocess stdout pipe.
// For HTTP sessions, this is a no-op (events are dispatched via HTTPSessionIO goroutines).
func (s *Session) ReadStdout() {
	defer panicx.Recover(s.logger, "ReadStdout")

	// No-op for HTTP sessions (HTTPSessionIO handles event dispatch internally).
	if s.io == nil || !s.io.IsCLI() {
		return
	}

	cliIO, ok := s.io.(*CLISessionIO)
	if !ok {
		return
	}
	stdout := cliIO.Stdout()
	if stdout == nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, ScannerInitialBufSize)
	scanner.Buffer(buf, ScannerMaxBufSize)

	defer func() {
		cb := s.GetCallback()
		if cb != nil {
			_ = cb("runner_exit", nil)
		}

		if err := scanner.Err(); err != nil && !isExpectedCloseError(err) {
			if s.logger != nil {
				s.logger.Error("Session stdout scanner error", "error", err)
			}
			s.SetStatus(SessionStatusDead)
		}
	}()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		cb := s.GetCallback()
		if cb != nil {
			if err := cb("raw_line", line); err != nil {
				s.logger.Debug("ReadStdout: dispatch callback error", "error", err)
			}
		}
	}

	if err := scanner.Err(); err != nil && s.logger != nil && !isExpectedCloseError(err) {
		s.logger.Error("Session stdout scanner error", "error", err)
	}
}

// ReadStderr asynchronously reads CLI stderr to prevent buffer deadlocks.
// For CLI sessions, this reads from the subprocess stderr pipe.
// For HTTP sessions, this is a no-op.
func (s *Session) ReadStderr() {
	defer panicx.Recover(s.logger, "ReadStderr")

	// No-op for HTTP sessions.
	if s.io == nil || !s.io.IsCLI() {
		return
	}

	cliIO, ok := s.io.(*CLISessionIO)
	if !ok {
		return
	}
	stderr := cliIO.Stderr()
	if stderr == nil {
		return
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Structured logging with session context
		if s.logger != nil {
			s.logger.Warn("session_stderr",
				"session_id", s.ID,
				"provider_session_id", s.ProviderSessionID,
				"workdir", s.Config.WorkDir,
				"content", line)
		}
		// Write to session log file for persistence with proper locking
		s.writeToLogFile(line)
	}

	if err := scanner.Err(); err != nil && s.logger != nil && !isExpectedCloseError(err) {
		s.logger.Error("Session stderr scanner error",
			"session_id", s.ID,
			"error", err)
	}
}

// writeToLogFile writes a line to the session log file with proper mutex protection.
func (s *Session) writeToLogFile(line string) {
	s.mu.RLock()
	logFile := s.logFile
	s.mu.RUnlock()

	if logFile != nil {
		if _, err := fmt.Fprintf(logFile, "[%s] %s\n", time.Now().Format(time.RFC3339), line); err != nil && s.logger != nil {
			s.logger.Warn("Failed to write to session log file", "error", err, "session_id", s.ID)
		}
	}
}

// NewTestSession creates a Session for testing purposes.
// This should only be used in test code.
func NewTestSession(id string, status SessionStatus) *Session {
	return &Session{
		ID:                id,
		ProviderSessionID: "test-provider-session",
		Status:            status,
		statusChange:      make(chan SessionStatus, 10),
	}
}

// OpenLogFile opens a log file for this session in SessionLogDir.
func (s *Session) OpenLogFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.logFile != nil {
		return nil
	}
	if err := os.MkdirAll(SessionLogDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	// SECURITY: Sanitize session ID to prevent path traversal attacks
	// filepath.Base extracts only the final component, removing any directory separators
	safeID := filepath.Base(s.ID)
	if safeID == "." || safeID == ".." {
		return fmt.Errorf("invalid session ID for log file: %s", s.ID)
	}

	logPath := filepath.Join(SessionLogDir, safeID+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	s.logFile = f
	return nil
}

// GetLogPath returns the path to the session log file.
func (s *Session) GetLogPath() string {
	return filepath.Join(SessionLogDir, s.ID+".log")
}
