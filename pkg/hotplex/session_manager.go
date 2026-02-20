package hotplex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// SessionStatus defines the current state of a session.
type SessionStatus string

const (
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusReady    SessionStatus = "ready"
	SessionStatusBusy     SessionStatus = "busy"
	SessionStatusDead     SessionStatus = "dead"
)

// Scanner buffer sizes for CLI output parsing.
const (
	scannerInitialBufSize = 256 * 1024       // 256 KB
	scannerMaxBufSize     = 10 * 1024 * 1024 // 10 MB
)

// Session lifecycle constants.
const (
	defaultReadyTimeout  = 10 * time.Second // Maximum time to wait for session to be ready
	statusBusyDuration   = 2 * time.Second  // Duration to keep session in Busy state after input
	cleanupCheckInterval = 1 * time.Minute  // Interval between idle session cleanup checks
)

// Session represents a persistent Hot-Multiplexing process of Claude Code CLI.
// It wraps the OS process, standard I/O pipes, and synchronization primitives.
type Session struct {
	ID          string // Unique identifier for the persistent session (user provided)
	CCSessionID string // The deterministic UUID formatted string passed to Claude CLI
	Config      Config // Configuration and execution context for the session
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	cancel      context.CancelFunc
	CreatedAt   time.Time     // Timestamp of session creation
	LastActive  time.Time     // Timestamp of last interaction (for idle GC)
	Status      SessionStatus // Current state of the session (ready, busy, dead, starting)

	mu               sync.RWMutex
	statusResetTimer *time.Timer // Timer for resetting status from Busy to Ready

	// Multiplexing fields
	callback Callback
	logger   *slog.Logger // Needed for background readers
}

// SessionManager defines the interface for managing the persistent process pool.
type SessionManager interface {
	// GetOrCreateSession retrieves an active session or starts a new one if it doesn't exist.
	GetOrCreateSession(ctx context.Context, sessionID string, cfg Config) (*Session, error)
	// GetSession retrieves an active session by its ID without starting a new one.
	GetSession(sessionID string) (*Session, bool)
	// TerminateSession forcefully kills a session and cleans up its resources.
	TerminateSession(sessionID string) error
	// ListActiveSessions returns a list of all currently active sessions managed by the pool.
	ListActiveSessions() []*Session
	// Shutdown gracefully stops all sessions and the manager itself.
	Shutdown()
}

// SessionPool implements SessionManager.
// It serves as a global process pool, maintaining active Node.js processes
// and performing idle garbage collection (GC) to free up memory.
type SessionPool struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	logger   *slog.Logger
	timeout  time.Duration // Idle timeout
	opts     EngineOptions // Global engine settings for security and isolation
	done     chan struct{} // Shutdown signal
}

// NewSessionPool creates a new session manager.
func NewSessionPool(logger *slog.Logger, timeout time.Duration, opts EngineOptions) *SessionPool {
	if logger == nil {
		logger = slog.Default()
	}
	sm := &SessionPool{
		sessions: make(map[string]*Session),
		logger:   logger,
		timeout:  timeout,
		opts:     opts,
		done:     make(chan struct{}),
	}

	// Start idle session cleanup goroutine (per spec 6: 30m idle timeout)
	go sm.cleanupLoop()

	return sm
}

// GetOrCreateSession returns an existing session or starts a new one.
func (sm *SessionPool) GetOrCreateSession(ctx context.Context, sessionID string, cfg Config) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session exists and is alive
	if sess, ok := sm.sessions[sessionID]; ok {
		if sess.IsAlive() {
			sess.Touch()
			return sess, nil
		}
		// If dead, cleanup and recreate
		_ = sm.cleanupSessionLocked(sessionID) //nolint:errcheck // cleanup on dead session
	}

	// Create new session
	sess, err := sm.startSession(ctx, sessionID, cfg)
	if err != nil {
		return nil, err
	}

	sm.sessions[sessionID] = sess
	return sess, nil
}

// GetSession retrieves an active session.
func (sm *SessionPool) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sess, ok := sm.sessions[sessionID]
	return sess, ok
}

// TerminateSession stops and removes a session.
func (sm *SessionPool) TerminateSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.cleanupSessionLocked(sessionID)
}

// ListActiveSessions returns all active sessions.
func (sm *SessionPool) ListActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	list := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		list = append(list, s)
	}
	return list
}

// cleanupSessionLocked stops the process and removes from map. Caller must hold lock.
func (sm *SessionPool) cleanupSessionLocked(sessionID string) error {
	sess, ok := sm.sessions[sessionID]
	if !ok {
		return nil
	}

	delete(sm.sessions, sessionID)

	sm.logger.Info("Terminating session and sweeping OS process group", "session_id", sessionID)

	// Stop the status reset timer and clean up session resources
	// Hold session lock to prevent race with WriteInput
	sess.mu.Lock()
	sess.close()
	sess.mu.Unlock()

	// Cancel context to kill process if using CommandContext
	if sess.cancel != nil {
		sess.cancel()
	}

	// Force kill if needed
	if sess.cmd != nil && sess.cmd.Process != nil {
		// Use specific signal or Kill
		// We set Setpgid = true, so we negate the PID to kill the process group
		_ = syscall.Kill(-sess.cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck // force terminate entire process tree
	}

	return nil
}

// startSession initializes the OS process (Cold Start). Caller must hold lock.
func (sm *SessionPool) startSession(ctx context.Context, sessionID string, cfg Config) (*Session, error) {
	// Early exit if request context is already cancelled
	if ctx.Err() != nil {
		return nil, fmt.Errorf("request context cancelled: %w", ctx.Err())
	}

	cliPath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude Code CLI not found: %w", err)
	}

	// Prepare context with cancellation.
	// We intentionally use context.Background() instead of the request ctx
	// because the session should outlive the HTTP request that created it.
	sessCtx, cancel := context.WithCancel(context.Background())
	// Ensure cancel is called only on error paths to keep the process alive on success
	var success bool
	defer func() {
		if !success {
			cancel()
		}
	}()

	// Use a startup timeout to prevent indefinite hangs during process start
	// We monitor startup in a goroutine and cancel if it takes too long
	startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startupCancel()

	// Channel to signal successful startup or failure
	startedCh := make(chan error, 1)

	// Ensure we signal completion even on early return
	// This prevents goroutine leak if function returns before startup completes
	defer close(startedCh)

	// Goroutine to monitor startup timeout
	// If startup takes longer than the timeout, cancel the session
	go func() {
		select {
		case err := <-startedCh:
			if err != nil {
				cancel()
			}
		case <-startupCtx.Done():
			select {
			case err := <-startedCh:
				if err != nil {
					cancel()
				}
			default:
				// Startup timed out and no success signal was sent
				cancel()
			}
		}
	}()

	// Deterministic UUID generation mapping from the string sessionID
	// We mix in the Engine's Namespace to ensure proper isolation across applications.
	// This ensures that Claude CLI always receives a valid UUID format,
	// while callers can use arbitrary strings.
	uniqueStr := fmt.Sprintf("%s:session:%s", sm.opts.Namespace, sessionID)
	ccSessionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(uniqueStr)).String()

	// Build arguments
	// We always force --output-format stream-json and --print

	// Check if first call logic is needed?
	// The session manager just starts the process.
	// Persistence: --session-id is key.

	// We will use "Resume" logic if we trust the session ID persistence on disk,
	// OR we always treat it as "maybe resume".
	// The CLI handles "resume" vs "new" based on session ID existence?
	// Actually CLI has --resume <id> vs --session-id <id>.
	// Let's stick to --session-id for creation and --resume for re-connection?
	// Wait, spec says: Args: --print --verbose --output-format stream-json --session-id <sid>
	args := []string{
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--session-id", ccSessionID,
	}

	// 1. Permission Mode (Strictly enforced from EngineOptions)
	if sm.opts.PermissionMode != "" {
		args = append(args, "--permission-mode", sm.opts.PermissionMode)
	}

	// 2. Allowed Paths (Merged: Global + Session)
	// Example: Claude CLI currently does NOT support --allow-read native flags yet.
	// We keep the arrays merged for future WAF or plugin usage parsing.
	var allowed []string
	allowed = append(allowed, sm.opts.GlobalAllowedPaths...)
	allowed = append(allowed, cfg.SessionAllowedPaths...)
	// for _, p := range allowed {
	// 	if p != "" {
	// 		args = append(args, "--allow-read", p, "--allow-write", p) // Pending CLI support
	// 	}
	// }

	// 3. Forbidden Paths (Strictly enforced from EngineOptions)
	for _, p := range sm.opts.ForbiddenPaths {
		if p != "" {
			// args = append(args, "--deny-read", p, "--deny-write", p) // Pending CLI support
			sm.logger.Debug("Forbidden path registered (WAF enforcement only)", "path", p)
		}
	}

	// Note: We don't pass the initial prompt here. The prompt will be injected via stdin later
	// OR passed as argument. BUT we want a persistent session.
	// If we pass a prompt arg, it runs and might exit?
	// CC Runner usually waits for input if interactive?
	// Spec says: "Process starts... hangs waiting for stdin".
	// Depending on CC CLI behavior, if no prompt provided, does it start REPL?
	// We assume passing no prompt starts REPL mode or waits.

	// 4. System Prompt (Merged: Base + Task)
	var combinedPrompt string
	if sm.opts.BaseSystemPrompt != "" {
		combinedPrompt = sm.opts.BaseSystemPrompt
	}
	if cfg.TaskSystemPrompt != "" {
		if combinedPrompt != "" {
			combinedPrompt += "\n\n"
		}
		combinedPrompt += cfg.TaskSystemPrompt
	}

	if combinedPrompt != "" {
		args = append(args, "--append-system-prompt", combinedPrompt)
	}

	cmd := exec.CommandContext(sessCtx, cliPath, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Env = append(os.Environ(), "CLAUDE_DISABLE_TELEMETRY=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Isolate into new process group for clean tree kill

	// Create pipes with proper cleanup on error paths
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser

	stdin, err = cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err = cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close() //nolint:errcheck // cleanup on error path
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close() //nolint:errcheck // cleanup on error path
		_ = stdin.Close()  //nolint:errcheck // cleanup on error path
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		startedCh <- err // Signal startup failed
		return nil, fmt.Errorf("cmd start: %w", err)
	}

	// Signal that startup succeeded
	startedCh <- nil

	sm.logger.Info("OS Process started (Cold Start)",
		"session_id", sessionID,
		"cc_session_id", ccSessionID,
		"pid", cmd.Process.Pid,
		"pgid", cmd.Process.Pid) // PGID is the same as PID since we use Setpgid: true

	sess := &Session{
		ID:          sessionID,
		CCSessionID: ccSessionID,
		Config:      cfg,
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		cancel:      cancel,
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
		Status:      SessionStatusStarting,
		logger:      sm.logger,
	}

	// Start background readers for multiplexing
	go sess.readStdout()
	go sess.readStderr()

	// Monitor process exit to prevent zombies and log unexpected crashes
	go func() {
		err := cmd.Wait()
		if sm.logger != nil {
			sm.logger.Warn("Session OS process exited unexpectedly",
				"session_id", sessionID,
				"exit_error", err)
		}
	}()

	// Start status transition monitor: Starting -> Ready
	sess.waitForReady(sessCtx, defaultReadyTimeout)

	success = true
	return sess, nil
}

// isAliveLocked checks if the process is still running. Caller must hold lock.
func (s *Session) isAliveLocked() bool {
	if s.cmd == nil || s.cmd.Process == nil || s.Status == SessionStatusDead {
		return false
	}
	if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// IsAlive checks if the process is still running.
func (s *Session) IsAlive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isAliveLocked()
}

// Touch updates LastActive time.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActive = time.Now()
}

// SetStatus updates the session status with proper locking.
func (s *Session) SetStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// GetStatus returns the current session status.
func (s *Session) GetStatus() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// waitForReady monitors the session and transitions from Starting to Ready
// when the process is confirmed alive and responsive.
// The context parameter allows cancellation if the session is terminated early.
func (s *Session) waitForReady(ctx context.Context, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				// Context cancelled - session terminated or request cancelled
				return
			case <-ticker.C:
				s.mu.Lock()
				if s.Status == SessionStatusDead {
					s.mu.Unlock()
					return
				}
				if s.isAliveLocked() {
					s.Status = SessionStatusReady
					s.mu.Unlock()
					return
				}
				s.mu.Unlock()
			}
		}
		// Timeout - mark as dead if still not alive
		s.mu.Lock()
		if s.Status == SessionStatusStarting {
			s.Status = SessionStatusDead
		}
		s.mu.Unlock()
	}()
}

// WriteInput injects a JSON message to Stdin.
// Transitions session to Busy during write, back to Ready after completion.
func (s *Session) WriteInput(msg map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Set status to Busy while processing input
	// Must be done under lock to prevent race with cleanup
	s.Status = SessionStatusBusy

	// Reset existing timer if any (prevents goroutine accumulation)
	if s.statusResetTimer != nil {
		// Stop the timer and check if it was already fired
		if !s.statusResetTimer.Stop() {
			// Timer already fired - callback may be running or about to run
			// Release lock briefly to allow callback to complete if it's holding lock
			s.mu.Unlock()
			time.Sleep(50 * time.Millisecond) // Give callback time to complete
			s.mu.Lock()
		}
	}

	// Schedule status recovery to Ready after a short delay
	// This allows the session to be marked busy while the CLI processes the input
	// Callback acquires lock to prevent race with WriteInput
	s.statusResetTimer = time.AfterFunc(statusBusyDuration, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.isAliveLocked() {
			s.Status = SessionStatusReady
		}
	})

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Append newline as protocol often requires it (JSONL)
	data = append(data, '\n')

	_, err = s.stdin.Write(data)
	if err != nil {
		return err
	}

	s.LastActive = time.Now()
	return nil
}

// close releases resources held by the session.
// Must be called with session lock held.
func (s *Session) close() {
	// Stop the status reset timer if exists
	// Use a local copy to avoid holding lock during Stop()
	if s.statusResetTimer != nil {
		timer := s.statusResetTimer
		s.statusResetTimer = nil
		// Timer.Stop is safe to call multiple times and from different goroutines
		timer.Stop()
	}
}

// SetCallback registers the callback to handle stream events for the current turn.
func (s *Session) SetCallback(cb Callback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callback = cb
}

// readStdout asynchronously reads CLI stdout, parses JSON, and dispatches callbacks.
func (s *Session) readStdout() {
	if s.stdout == nil {
		return
	}

	scanner := bufio.NewScanner(s.stdout)
	buf := make([]byte, 0, scannerInitialBufSize)
	scanner.Buffer(buf, scannerMaxBufSize)

	// Ensure doneChan is closed on exit to prevent callers from hanging indefinitely
	// This handles cases where the scanner aborts due to ErrTooLong, process crash, or EOF.
	defer func() {
		s.mu.RLock()
		cb := s.callback
		s.mu.RUnlock()

		if cb != nil {
			_ = cb("runner_exit", nil)
		}

		// If scanner exited with error, the process is likely dead or in a bad state
		if err := scanner.Err(); err != nil {
			if s.logger != nil {
				s.logger.Error("Session stdout scanner error", "session_id", s.ID, "error", err)
			}
			s.mu.Lock()
			s.Status = SessionStatusDead
			s.mu.Unlock()
		}
	}()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		s.mu.RLock()
		cb := s.callback
		s.mu.RUnlock()

		if cb != nil {
			if err := cb("raw_line", line); err != nil {
				s.logger.Debug("readStdout: dispatch callback error", "error", err)
			}
		}
	}

	if err := scanner.Err(); err != nil && s.logger != nil {
		s.logger.Error("Session stdout scanner error", "session_id", s.ID, "error", err)
	}
}

// readStderr asynchronously reads CLI stderr to prevent buffer deadlocks.
func (s *Session) readStderr() {
	if s.stderr == nil {
		return
	}

	scanner := bufio.NewScanner(s.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if s.logger != nil {
			s.logger.Warn("Session stderr", "session_id", s.ID, "stderr", line)
		}
	}

	if err := scanner.Err(); err != nil && s.logger != nil {
		s.logger.Error("Session stderr scanner error", "session_id", s.ID, "error", err)
	}
}

// cleanupLoop runs periodic cleanup of idle sessions.
// Runs every minute and terminates sessions that have been idle longer than timeout.
func (sm *SessionPool) cleanupLoop() {
	ticker := time.NewTicker(cleanupCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanupIdleSessions()
		case <-sm.done:
			return
		}
	}
}

// cleanupIdleSessions removes sessions that have exceeded the idle timeout.
func (sm *SessionPool) cleanupIdleSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for sessionID, sess := range sm.sessions {
		idleTime := now.Sub(sess.LastActive)
		if idleTime > sm.timeout {
			sm.logger.Info("Session idle timeout, terminating",
				"session_id", sessionID,
				"idle_duration", idleTime,
				"timeout", sm.timeout)
			_ = sm.cleanupSessionLocked(sessionID) //nolint:errcheck // cleanup on idle timeout
		}
	}
}

// Shutdown gracefully stops the session manager and all active sessions.
func (sm *SessionPool) Shutdown() {
	close(sm.done)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Mark all sessions as Dead and signal runner_exit to unblock waiting callers
	for _, sess := range sm.sessions {
		sess.mu.Lock()
		sess.Status = SessionStatusDead
		if sess.callback != nil {
			_ = sess.callback("runner_exit", nil)
		}
		sess.mu.Unlock()
	}

	// Terminate all sessions (kill processes, cancel contexts)
	for sessionID := range sm.sessions {
		_ = sm.cleanupSessionLocked(sessionID) //nolint:errcheck // cleanup on shutdown
	}
}
