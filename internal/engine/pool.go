package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex/internal/panicx"
	"github.com/hrygo/hotplex/internal/persistence"
	"github.com/hrygo/hotplex/internal/sys"
	"github.com/hrygo/hotplex/provider"
)

// SessionPool implements the SessionManager as a thread-safe singleton.
// It orchestrates the lifecycle of multiple CLI processes, ensuring that
// idle processes are garbage collected to conserve system memory.
var _ SessionManager = (*SessionPool)(nil)

type SessionPool struct {
	sessions     map[string]*Session
	mu           sync.RWMutex
	logger       *slog.Logger
	timeout      time.Duration // Time after which an idle session is eligible for termination
	opts         EngineOptions // Global constraints shared by all sessions in the pool
	cliPath      string        // Resolved path to the CLI binary
	provider     provider.Provider
	done         chan struct{} // Internal signal for shutting down background workers
	shutdownOnce sync.Once     // Ensures Shutdown is only executed once
	markerStore  persistence.SessionMarkerStore
	pending      map[string]chan struct{}
	wg           sync.WaitGroup // Waits for background goroutines to finish

	// streamStore protects uncommitted stream data before session termination
	streamStore StreamDataSaver
}

// StreamDataSaver protects uncommitted stream data before session termination.
// Implemented by chatapps/base.StreamMessageStore to flush incomplete buffers.
type StreamDataSaver interface {
	// GetBuffer retrieves the stream buffer for a session.
	// Returns nil if no buffer exists.
	GetBuffer(sessionID string) any

	// SaveIncompleteStream saves uncommitted stream data before termination.
	// This is a synchronous operation to ensure data is persisted before
	// the session is destroyed.
	SaveIncompleteStream(ctx context.Context, sessionID string, buffer any) error
}

// blockedEnvPrefixes contains environment variable prefixes that should be filtered
// out for security reasons to prevent injection attacks via environment variables.

// NewSessionPool creates a new session manager with default file-based marker storage.
func NewSessionPool(logger *slog.Logger, timeout time.Duration, opts EngineOptions, cliPath string, prv provider.Provider) *SessionPool {
	if logger == nil {
		logger = slog.Default()
	}

	sm := &SessionPool{
		sessions:    make(map[string]*Session),
		logger:      logger,
		timeout:     timeout,
		opts:        opts,
		cliPath:     cliPath,
		provider:    prv,
		done:        make(chan struct{}),
		markerStore: persistence.NewDefaultFileMarkerStore(),
		pending:     make(map[string]chan struct{}),
	}

	// Start background workers with WaitGroup for graceful shutdown
	sm.wg.Add(2)
	go sm.cleanupLoop()
	go sm.maintenanceLoop()

	return sm
}

// GetOrCreateSession returns an existing session or starts a new one.
func (sm *SessionPool) GetOrCreateSession(ctx context.Context, sessionID string, cfg SessionConfig, prompt string) (*Session, bool, error) {
	return sm.getOrCreateSession(ctx, sessionID, cfg, prompt, 0)
}

// maxRecursionDepth limits how many times we can recursively call getOrCreateSession
// to prevent stack overflow in pathological concurrent scenarios
const maxRecursionDepth = 5

func (sm *SessionPool) getOrCreateSession(ctx context.Context, sessionID string, cfg SessionConfig, prompt string, depth int) (*Session, bool, error) {
	// Validate required parameters
	if sessionID == "" {
		return nil, false, fmt.Errorf("sessionID cannot be empty")
	}

	// Prevent infinite recursion in pathological concurrent scenarios
	if depth > maxRecursionDepth {
		return nil, false, fmt.Errorf("session creation recursion limit exceeded for session %s", sessionID)
	}

	// 1. Check existing
	sm.mu.RLock()
	if sess, ok := sm.sessions[sessionID]; ok {
		if sess.IsAlive() {
			sm.mu.RUnlock()
			sess.Touch()
			return sess, false, nil
		}
	}
	sm.mu.RUnlock()

	// 2. Slow path: Handle creation or wait for pending
	sm.mu.Lock()
	// Double check
	if sess, ok := sm.sessions[sessionID]; ok {
		if sess.IsAlive() {
			sm.mu.Unlock()
			sess.Touch()
			return sess, false, nil
		}
		_ = sm.cleanupSessionLocked(sessionID)
	}

	// Check if already being created
	if ch, ok := sm.pending[sessionID]; ok {
		sm.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-ch:
			// Creation finished, recurse to check result with depth tracking
			return sm.getOrCreateSession(ctx, sessionID, cfg, prompt, depth+1)
		}
	}

	// Not being created, start it
	ch := make(chan struct{})
	sm.pending[sessionID] = ch
	sm.mu.Unlock()

	// Ensure we cleanup the pending marker on exit
	defer func() {
		sm.mu.Lock()
		delete(sm.pending, sessionID)
		close(ch)
		sm.mu.Unlock()
	}()

	// startSession is heavy, but now doesn't block other sessionIDs
	sess, err := sm.startSession(ctx, sessionID, cfg, prompt)
	if err != nil {
		return nil, false, err
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = sess
	sm.mu.Unlock()

	return sess, true, nil
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
	if sessionID == "" {
		return nil // Nothing to terminate
	}
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

// DeleteMarker removes the HotPlex session marker file, preventing future resumption.
func (sm *SessionPool) DeleteMarker(providerSessionID string) error {
	if providerSessionID == "" {
		return nil
	}
	return sm.markerStore.Delete(providerSessionID)
}

// CleanupSessionFiles proxies the cleanup call to the underlying provider.
func (sm *SessionPool) CleanupSessionFiles(providerSessionID string, workDir string) error {
	if sm.provider != nil {
		return sm.provider.CleanupSession(providerSessionID, workDir)
	}
	return nil
}

// SetStreamStore sets the stream data saver for protecting uncommitted stream data
// before session termination. This is typically called by chatapps adapters
// that enable message storage with streaming.
func (sm *SessionPool) SetStreamStore(saver StreamDataSaver) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.streamStore = saver
	sm.logger.Info("Stream store configured for session pool",
		"namespace", sm.opts.Namespace)
}

// cleanupSessionLocked stops the process and removes from map. Caller must hold lock.
func (sm *SessionPool) cleanupSessionLocked(sessionID string) error {
	sess, ok := sm.sessions[sessionID]
	if !ok {
		return nil
	}

	delete(sm.sessions, sessionID)

	sm.logger.Info("Terminating session and sweeping OS process group",
		"namespace", sm.opts.Namespace,
		"session_id", sessionID,
		"provider_session_id", sess.ProviderSessionID)

	// Save uncommitted stream data before termination (if streamStore is configured)
	if sm.streamStore != nil {
		if buffer := sm.streamStore.GetBuffer(sessionID); buffer != nil {
			sm.logger.Info("Saving incomplete stream buffer before session termination",
				"session_id", sessionID,
				"provider_session_id", sess.ProviderSessionID)

			// Synchronous save to ensure data is persisted before session is destroyed
			// Use background context since the request context may be cancelled
			saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := sm.streamStore.SaveIncompleteStream(saveCtx, sessionID, buffer); err != nil {
				sm.logger.Error("Failed to save incomplete stream buffer",
					"session_id", sessionID,
					"provider_session_id", sess.ProviderSessionID,
					"error", err)
				// Continue with termination even if save fails
			} else {
				sm.logger.Info("Successfully saved incomplete stream buffer",
					"session_id", sessionID,
					"provider_session_id", sess.ProviderSessionID)
			}
		}
	}

	// Hold session lock to prevent race with WriteInput
	sess.mu.Lock()
	sess.close()
	sess.mu.Unlock()

	// Cancel context to kill process if using CommandContext
	if sess.cancel != nil {
		sess.cancel()
	}

	// Force kill if needed (pass jobHandle for Windows Job Object termination)
	sys.KillProcessGroup(sess.cmd, sess.jobHandle)

	// Reap the zombie process so the OS process-table entry is cleared.
	// sess.Wait() uses sync.Once to ensure only one reaper executes,
	// eliminating double-wait races between sync and async paths.
	_ = sess.Wait()

	return nil
}

// startSession initializes the OS process (Cold Start).
func (sm *SessionPool) startSession(ctx context.Context, sessionID string, cfg SessionConfig, prompt string) (*Session, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("request context cancelled: %w", ctx.Err())
	}

	sessCtx, cancel := context.WithCancel(context.Background())
	var success bool
	defer func() {
		if !success {
			cancel()
		}
	}()

	startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startupCancel()

	startedCh := make(chan error, 1)
	defer close(startedCh)

	go monitorStartup(startupCtx, startedCh, cancel)

	// Use direct string concatenation for better performance
	// Always use deterministic SHA1 for consistent session ID mapping
	// This ensures end-to-end session traceability - no random UUIDs.
	// Per-session Namespace override takes precedence over pool default.
	ns := cfg.Namespace
	if ns == "" {
		ns = sm.opts.Namespace
	}
	uniqueStr := ns + ":session:" + sessionID
	providerSessionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(uniqueStr)).String()

	sessLog := sm.logger.With(
		"namespace", ns,
		"session_id", sessionID,
		"provider_session_id", providerSessionID,
	)

	// Check if this is a resume BEFORE building CLI args
	// This is critical because buildCLIArgs may create the marker
	isResuming := sm.markerStore.Exists(providerSessionID)

	args := sm.buildCLIArgs(providerSessionID, sessLog, prompt, cfg)
	cmd := exec.CommandContext(sessCtx, sm.cliPath, args...)

	// Clear CLAUDECODE env var to allow nested CLI sessions
	// CLI refuses to start if it detects it's running inside another Claude Code session
	cmd.Env = slices.DeleteFunc(slices.Clone(os.Environ()), func(env string) bool {
		return strings.HasPrefix(env, "CLAUDECODE=")
	})

	// Resolve relative paths (like ".") to absolute paths
	// First clean the path to resolve . and .. elements, then convert to absolute
	if cfg.WorkDir == "." || !filepath.IsAbs(cfg.WorkDir) {
		cleaned := filepath.Clean(cfg.WorkDir)
		if absPath, err := filepath.Abs(cleaned); err == nil {
			cmd.Dir = absPath
		} else {
			cmd.Dir = cleaned // Fallback to cleaned path if error
		}
	} else {
		// For absolute paths, also clean to resolve . and .. elements
		cmd.Dir = filepath.Clean(cfg.WorkDir)
	}

	// Setup process attributes and get job handle (Windows) or zero (Unix)
	jobHandle, err := sys.SetupCmdSysProcAttr(cmd)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("setup sys proc attr: %w", err)
	}

	stdin, stdout, stderr, err := setupCmdPipes(cmd)
	if err != nil {
		cancel()
		sys.CloseJobHandle(jobHandle) // Cleanup job handle on error
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		startedCh <- err
		// Close pipes to prevent resource leak on start failure
		if stdin != nil {
			_ = stdin.Close()
		}
		if stdout != nil {
			_ = stdout.Close()
		}
		if stderr != nil {
			_ = stderr.Close()
		}
		sys.CloseJobHandle(jobHandle) // Cleanup job handle on error
		return nil, fmt.Errorf("cmd start: %w", err)
	}

	// Assign process to Job Object on Windows
	if jobHandle != 0 {
		if err := sys.AssignProcessToJob(jobHandle, cmd.Process); err != nil {
			sessLog.Warn("failed to assign process to Job Object", "error", err)
			// Continue anyway - process is still running, will be killed via taskkill fallback
		}
	}

	// Create marker AFTER CLI process successfully starts
	// This prevents creating markers for failed session starts
	if !isResuming {
		if err := sm.markerStore.Create(providerSessionID); err != nil {
			sessLog.Error("Session will not be persistent - marker creation failed",
				"error", err,
				"provider_session_id", providerSessionID,
				"impact", "Session cannot be resumed after daemon restart")
		} else {
			sessLog.Info("Created session marker after successful CLI start", "provider_session_id", providerSessionID)
		}
	}

	startedCh <- nil

	sessLog.Info("OS Process started (Cold Start)",
		"pid", cmd.Process.Pid,
		"pgid", cmd.Process.Pid)

	sess := &Session{
		ID:                sessionID,
		ProviderSessionID: providerSessionID,
		Config:            cfg,
		cmd:               cmd,
		stdin:             stdin,
		stdout:            stdout,
		stderr:            stderr,
		cancel:            cancel,
		jobHandle:         jobHandle,
		CreatedAt:         time.Now(),
		LastActive:        time.Now(),
		Status:            SessionStatusStarting,
		TaskInstructions:  cfg.TaskInstructions,
		statusChange:      make(chan SessionStatus, 10),
		logger:            sessLog,
		IsResuming:        isResuming,
	}

	// Open session log file for stderr persistence
	if err := sess.OpenLogFile(); err != nil {
		sessLog.Warn("Failed to open session log file", "error", err)
	}

	go sess.ReadStdout()
	go sess.ReadStderr()

	panicx.SafeGo(sessLog, func() {
		err := sess.Wait()
		if sess.GetStatus() != SessionStatusDead {
			sessLog.Warn("Session OS process exited unexpectedly", "exit_error", err, "is_resuming", isResuming)
			// If this was a resume attempt that failed, delete the stale marker and CLI session file
			// This allows the next request to create a fresh session with the SAME deterministic ID
			if isResuming {
				// Delete marker first
				if delErr := sm.markerStore.Delete(providerSessionID); delErr != nil {
					sessLog.Warn("Failed to delete stale session marker", "error", delErr)
				} else {
					sessLog.Info("Deleted stale session marker after failed resume", "provider_session_id", providerSessionID)
				}
				// Also clean up the CLI session file
				// Next request will use the same deterministic SHA1 ID but create a fresh session
				if cleanupErr := sm.provider.CleanupSession(providerSessionID, sess.Config.WorkDir); cleanupErr != nil {
					sessLog.Warn("Failed to cleanup CLI session file after failed resume", "error", cleanupErr)
				} else {
					sessLog.Info("Cleaned up CLI session file after failed resume", "provider_session_id", providerSessionID)
				}
				// NOTE: We do NOT use random UUID here. Next request will use the same
				// deterministic SHA1 ID to create a fresh session, maintaining end-to-end consistency.
			}
			// Update status to Dead and notify callback
			sess.SetStatus(SessionStatusDead)
			if cb := sess.GetCallback(); cb != nil {
				_ = cb("runner_exit", nil)
			}
		}
	})

	sess.waitForReady(sessCtx, DefaultReadyTimeout)
	success = true
	return sess, nil
}

func (sm *SessionPool) buildCLIArgs(providerSessionID string, sessLog *slog.Logger, prompt string, cfg SessionConfig) []string {
	// Determine system prompt: session-level override takes precedence over engine-level
	baseSystemPrompt := cfg.BaseSystemPrompt
	if baseSystemPrompt == "" {
		baseSystemPrompt = sm.opts.BaseSystemPrompt
	}

	// Build ProviderSessionOptions
	opts := &provider.ProviderSessionOptions{
		WorkDir:                    cfg.WorkDir,
		PermissionMode:             sm.opts.PermissionMode,
		DangerouslySkipPermissions: sm.opts.DangerouslySkipPermissions,
		AllowedTools:               sm.opts.AllowedTools,
		DisallowedTools:            sm.opts.DisallowedTools,
		BaseSystemPrompt:           baseSystemPrompt,
		TaskInstructions:           cfg.TaskInstructions,
		InitialPrompt:              prompt,
		SessionID:                  providerSessionID,
	}

	// Check if we need to resume using marker store
	if sm.markerStore.Exists(providerSessionID) {
		// Verify that the CLI session data actually exists before attempting resume
		// This prevents "No conversation found with session ID" errors when:
		// - Marker exists but CLI session data was deleted/expired
		// - Container restart lost session data but marker file persists
		if sm.provider.VerifySession(providerSessionID, cfg.WorkDir) {
			// Both marker and CLI file exist - safe to resume
			opts.ResumeSession = true
			opts.ProviderSessionID = providerSessionID
			sessLog.Info("Resuming existing persistent CLI session")
		} else {
			// Marker exists but CLI session data is missing - clean up and create fresh session
			sessLog.Warn("Marker exists but CLI session data not found, creating fresh session",
				"provider_session_id", providerSessionID)

			// Delete stale marker
			if err := sm.markerStore.Delete(providerSessionID); err != nil {
				sessLog.Warn("Failed to delete stale marker", "error", err)
			}

			// Cleanup any stale CLI session files
			if err := sm.provider.CleanupSession(providerSessionID, cfg.WorkDir); err != nil {
				sessLog.Warn("Failed to cleanup stale CLI session file", "error", err)
			}

			opts.ProviderSessionID = providerSessionID
			sessLog.Info("Creating new CLI session (marker was stale)")
		}
	} else {
		opts.ProviderSessionID = providerSessionID

		// Critical: Delete stale CLI session file before starting new session
		// This prevents "Session ID is already in use" errors when:
		// - /reset deleted the marker but not the CLI session file
		// - Daemon restart with stale CLI session files on disk
		if err := sm.provider.CleanupSession(providerSessionID, cfg.WorkDir); err != nil {
			sessLog.Warn("Failed to cleanup stale CLI session file", "error", err)
		}

		sessLog.Info("Creating new persistent CLI session")
		// NOTE: Marker will be created AFTER CLI successfully starts in startSession()
	}

	return sm.provider.BuildCLIArgs(providerSessionID, opts)
}

func setupCmdPipes(cmd *exec.Cmd) (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		_ = stdin.Close()
		return nil, nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	return stdin, stdout, stderr, nil
}

func monitorStartup(startupCtx context.Context, startedCh <-chan error, cancel context.CancelFunc) {
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
			cancel()
		}
	}
}

// cleanupLoop runs periodic cleanup of idle sessions and stale marker files.
func (sm *SessionPool) cleanupLoop() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.cleanupInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanupIdleSessions()
			// NOTE: We do NOT cleanup stale markers here because:
			// 1. Correct cleanup happens on-demand in buildCLIArgs when user accesses
			// 2. User may continue session after a long time - no time-based expiration
			// 3. We cannot verify CLI file existence without knowing the workDir
		case <-sm.done:
			return
		}
	}
}

// cleanupInterval returns the dynamic interval for cleanup checks.
// It scales with the session timeout: interval = timeout / 4,
// clamped to [1min, 5min].
func (sm *SessionPool) cleanupInterval() time.Duration {
	interval := sm.timeout / 4
	if interval > 5*time.Minute {
		interval = 5 * time.Minute
	}
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	return interval
}

// cleanupIdleSessions removes sessions that have exceeded the idle timeout.
func (sm *SessionPool) cleanupIdleSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for sessionID, sess := range sm.sessions {
		idleTimeout := sess.Config.IdleTimeout
		if idleTimeout == 0 {
			idleTimeout = sm.timeout // pool default
		}
		idleTime := now.Sub(sess.GetLastActive())
		if idleTime > idleTimeout {
			sm.logger.Info("Session idle timeout, terminating",
				"namespace", sm.opts.Namespace,
				"session_id", sessionID,
				"provider_session_id", sess.ProviderSessionID,
				"idle_duration", idleTime,
				"idle_timeout", idleTimeout)
			_ = sm.cleanupSessionLocked(sessionID)
		}
	}
}

// Shutdown gracefully stops the session manager and all active sessions.
func (sm *SessionPool) Shutdown() {
	sm.shutdownOnce.Do(func() {
		close(sm.done)
	})

	// Wait for background goroutines (cleanupLoop, maintenanceLoop) to finish
	sm.wg.Wait()

	sm.mu.Lock()

	// Collect callbacks to invoke outside of locks to prevent deadlock
	type callbackEntry struct {
		cb      Callback
		sessLog *slog.Logger
	}
	callbacks := make([]callbackEntry, 0, len(sm.sessions))

	// Mark all sessions as Dead and collect callbacks
	for _, sess := range sm.sessions {
		sess.mu.Lock()
		sess.Status = SessionStatusDead
		if sess.callback != nil {
			callbacks = append(callbacks, callbackEntry{cb: sess.callback, sessLog: sess.logger})
		}
		sess.mu.Unlock()
	}

	// Release pool lock before invoking callbacks
	sm.mu.Unlock()

	// Invoke callbacks outside of locks
	for _, entry := range callbacks {
		if err := entry.cb("runner_exit", nil); err != nil && entry.sessLog != nil {
			entry.sessLog.Debug("Shutdown: callback error", "error", err)
		}
	}

	// Re-acquire lock for cleanup
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Terminate all sessions
	for sessionID := range sm.sessions {
		_ = sm.cleanupSessionLocked(sessionID)
	}
}

// maintenanceLoop runs periodic maintenance tasks.
// This includes cleaning up orphaned Claude userID entries every 10 minutes.
func (sm *SessionPool) maintenanceLoop() {
	defer sm.wg.Done()

	// Run once immediately on startup
	clearClaudeJSONUserID(sm.logger)

	// Then run every 10 minutes
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			clearClaudeJSONUserID(sm.logger)
		case <-sm.done:
			return
		}
	}
}
