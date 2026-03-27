package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	timeout      time.Duration  // Time after which an idle session is eligible for termination
	opts         EngineOptions  // Global constraints shared by all sessions in the pool
	starter      SessionStarter // Creates and starts sessions (CLI or HTTP)
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
func NewSessionPool(logger *slog.Logger, timeout time.Duration, opts EngineOptions, starter SessionStarter, prv provider.Provider) *SessionPool {
	if logger == nil {
		logger = slog.Default()
	}

	sm := &SessionPool{
		sessions:    make(map[string]*Session),
		logger:      logger,
		timeout:     timeout,
		opts:        opts,
		starter:     starter,
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
	sess, err := sm.starter.StartSession(ctx, sessionID, cfg, prompt, nil)
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

	sm.logger.Info("Terminating session",
		"namespace", sm.opts.Namespace,
		"session_id", sessionID,
		"provider_session_id", sess.ProviderSessionID)

	// Save uncommitted stream data before termination (if streamStore is configured).
	if sm.streamStore != nil {
		if buffer := sm.streamStore.GetBuffer(sessionID); buffer != nil {
			sm.logger.Info("Saving incomplete stream buffer before session termination",
				"session_id", sessionID,
				"provider_session_id", sess.ProviderSessionID)

			saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := sm.streamStore.SaveIncompleteStream(saveCtx, sessionID, buffer); err != nil {
				sm.logger.Error("Failed to save incomplete stream buffer",
					"session_id", sessionID,
					"provider_session_id", sess.ProviderSessionID,
					"error", err)
			} else {
				sm.logger.Info("Successfully saved incomplete stream buffer",
					"session_id", sessionID,
					"provider_session_id", sess.ProviderSessionID)
			}
		}
	}

	// Hold session lock to prevent race with WriteInput.
	sess.mu.Lock()
	sess.close()
	sess.mu.Unlock()

	// For CLI sessions: kill process group and reap zombie.
	if sess.io != nil && sess.io.IsCLI() {
		// Cancel context to kill process if using CommandContext.
		if sess.cancel != nil {
			sess.cancel()
		}

		// Force kill if needed (pass jobHandle for Windows Job Object termination).
		sys.KillProcessGroup(sess.cmd, sess.jobHandle)

		// Reap the zombie process so the OS process-table entry is cleared.
		_ = sess.Wait()
	}

	return nil
}

// buildCLIArgs is a convenience method that delegates to the CLI starter's buildCLIArgs.
// It exists for backward compatibility with existing tests.
func (sm *SessionPool) buildCLIArgs(providerSessionID string, sessLog *slog.Logger, prompt string, cfg SessionConfig) []string {
	if cliStarter, ok := sm.starter.(*CLISessionStarter); ok {
		return cliStarter.buildCLIArgs(providerSessionID, sessLog, prompt, cfg)
	}
	return nil
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
