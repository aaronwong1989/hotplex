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
	"time"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex/internal/panicx"
	"github.com/hrygo/hotplex/internal/persistence"
	"github.com/hrygo/hotplex/internal/sys"
	"github.com/hrygo/hotplex/provider"
)

// SessionStarter creates and manages agent sessions.
// Two implementations:
//   - CLISessionStarter: spawns a local CLI subprocess
//   - HTTPSessionStarter: connects to an HTTP/SSE server
type SessionStarter interface {
	// StartSession starts a new session and returns it fully initialized.
	// The callback is invoked for each raw event from the session.
	StartSession(ctx context.Context, sessionID string, cfg SessionConfig, prompt string, cb Callback) (*Session, error)

	// TransportType returns the transport type identifier ("cli" or "http").
	TransportType() string

	// CleanupOnError cleans up any partially-initialized session resources
	// when StartSession returns an error. This is needed because HTTPSessionStarter
	// starts the StartReading goroutine before returning; if StartSession returns
	// an error, the goroutine's 30s timeout is the only cleanup unless this is called.
	// For CLISessionStarter this is a no-op.
	CleanupOnError()
}

// CLISessionStarter spawns CLI subprocess sessions.
type CLISessionStarter struct {
	cliPath     string
	provider    provider.Provider
	markerStore persistence.SessionMarkerStore
	logger      *slog.Logger
	opts        EngineOptions
}

// Compile-time interface verification
var _ SessionStarter = (*CLISessionStarter)(nil)

// NewCLISessionStarter creates a CLISessionStarter from pool dependencies.
func NewCLISessionStarter(
	cliPath string,
	provider provider.Provider,
	markerStore persistence.SessionMarkerStore,
	logger *slog.Logger,
	opts EngineOptions,
) *CLISessionStarter {
	if logger == nil {
		logger = slog.Default()
	}
	return &CLISessionStarter{
		cliPath:     cliPath,
		provider:    provider,
		markerStore: markerStore,
		logger:      logger,
		opts:        opts,
	}
}

// CleanupOnError is a no-op for CLISessionStarter.
// CLI sessions are cleaned up via Session.close() + sys.KillProcessGroup in cleanupSessionLocked.
func (s *CLISessionStarter) CleanupOnError() {}

// TransportType implements SessionStarter.
func (s *CLISessionStarter) TransportType() string { return "cli" }

// StartSession starts a CLI subprocess session.
func (s *CLISessionStarter) StartSession(
	ctx context.Context,
	sessionID string,
	cfg SessionConfig,
	prompt string,
	cb Callback,
) (*Session, error) {
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

	go s.monitorStartup(startupCtx, startedCh, cancel)

	// Deterministic UUID v5 from namespace + sessionID for end-to-end traceability.
	ns := cfg.Namespace
	if ns == "" {
		ns = s.opts.Namespace
	}
	uniqueStr := ns + ":session:" + sessionID
	providerSessionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(uniqueStr)).String()

	sessLog := s.logger.With(
		"namespace", ns,
		"session_id", sessionID,
		"provider_session_id", providerSessionID,
	)

	// Check resume before building args (marker creation depends on this).
	isResuming := s.markerStore.Exists(providerSessionID)

	args := s.buildCLIArgs(providerSessionID, sessLog, prompt, cfg)
	cmd := exec.CommandContext(sessCtx, s.cliPath, args...)

	// Clear CLAUDECODE env var to allow nested CLI sessions.
	cmd.Env = slices.DeleteFunc(slices.Clone(os.Environ()), func(env string) bool {
		return strings.HasPrefix(env, "CLAUDECODE=")
	})

	// Resolve relative paths to absolute.
	if cfg.WorkDir == "." || !filepath.IsAbs(cfg.WorkDir) {
		cleaned := filepath.Clean(cfg.WorkDir)
		if absPath, err := filepath.Abs(cleaned); err == nil {
			cmd.Dir = absPath
		} else {
			cmd.Dir = cleaned
		}
	} else {
		cmd.Dir = filepath.Clean(cfg.WorkDir)
	}

	// Create work directory if it does not exist.
	// Without this, cmd.Start() fails silently when cmd.Dir points to a
	// non-existent path (the process exits with exit_code=0 and exit_error=<nil>).
	if cmd.Dir != "" {
		if err := os.MkdirAll(cmd.Dir, 0755); err != nil {
			cancel()
			return nil, fmt.Errorf("create work directory %q: %w", cmd.Dir, err)
		}
	}

	// Setup process attributes and job handle (Windows).
	jobHandle, err := sys.SetupCmdSysProcAttr(cmd)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("setup sys proc attr: %w", err)
	}

	stdin, stdout, stderr, err := setupCmdPipes(cmd)
	if err != nil {
		cancel()
		sys.CloseJobHandle(jobHandle)
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		startedCh <- err
		s.closePipesOnError(stdin, stdout, stderr)
		sys.CloseJobHandle(jobHandle)
		return nil, fmt.Errorf("cmd start: %w", err)
	}

	// Assign to Windows Job Object.
	if jobHandle != 0 {
		if err := sys.AssignProcessToJob(jobHandle, cmd.Process); err != nil {
			sessLog.Warn("failed to assign process to Job Object", "error", err)
		}
	}

	// Create marker after successful start (not before).
	if !isResuming {
		if err := s.markerStore.Create(providerSessionID); err != nil {
			sessLog.Error("Session will not be persistent - marker creation failed",
				"error", err,
				"provider_session_id", providerSessionID,
				"impact", "Session cannot be resumed after daemon restart")
		} else {
			sessLog.Info("Created session marker after successful CLI start",
				"provider_session_id", providerSessionID)
		}
	}

	startedCh <- nil

	sessLog.Info("OS Process started (Cold Start)",
		"pid", cmd.Process.Pid,
		"pgid", cmd.Process.Pid)

	// Build SessionIO and Session.
	cliIO := NewCLISessionIO(cmd, stdin, stdout, stderr, cancel, sessLog)

	sess := &Session{
		ID:                    sessionID,
		ProviderSessionID:     providerSessionID,
		Config:                cfg,
		cmd:                   cmd,
		io:                    cliIO,
		cancel:                cancel,
		jobHandle:             jobHandle,
		CreatedAt:             time.Now(),
		LastActive:            time.Now(),
		Status:                SessionStatusStarting,
		TaskInstructions:      cfg.TaskInstructions,
		statusChange:          make(chan SessionStatus, 10),
		logger:                sessLog,
		IsResuming:            isResuming,
		FirstMessageOnSession: !isResuming, // New session needs BuildInputMessage; resumed session already has context
		callback:              cb,
	}

	if err := sess.OpenLogFile(); err != nil {
		sessLog.Warn("Failed to open session log file", "error", err)
	}

	go sess.ReadStdout()
	go sess.ReadStderr()

	panicx.SafeGo(sessLog, func() {
		err := sess.Wait()
		if sess.GetStatus() != SessionStatusDead {
			sessLog.Warn("Session OS process exited unexpectedly",
				"exit_error", err, "is_resuming", isResuming)
			if isResuming {
				if delErr := s.markerStore.Delete(providerSessionID); delErr != nil {
					sessLog.Warn("Failed to delete stale session marker", "error", delErr)
				} else {
					sessLog.Info("Deleted stale session marker after failed resume",
						"provider_session_id", providerSessionID)
				}
				if cleanupErr := s.provider.CleanupSession(providerSessionID, sess.Config.WorkDir); cleanupErr != nil {
					sessLog.Warn("Failed to cleanup CLI session file after failed resume",
						"error", cleanupErr)
				} else {
					sessLog.Info("Cleaned up CLI session file after failed resume",
						"provider_session_id", providerSessionID)
				}
			}
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

func (s *CLISessionStarter) closePipesOnError(stdin io.WriteCloser, stdout, stderr io.ReadCloser) {
	if stdin != nil {
		_ = stdin.Close()
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	if stderr != nil {
		_ = stderr.Close()
	}
}

func (s *CLISessionStarter) monitorStartup(startupCtx context.Context, startedCh <-chan error, cancel context.CancelFunc) {
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

func (s *CLISessionStarter) buildCLIArgs(providerSessionID string, sessLog *slog.Logger, prompt string, cfg SessionConfig) []string {
	baseSystemPrompt := cfg.BaseSystemPrompt
	if baseSystemPrompt == "" {
		baseSystemPrompt = s.opts.BaseSystemPrompt
	}

	opts := &provider.ProviderSessionOptions{
		WorkDir:                    cfg.WorkDir,
		PermissionMode:             s.opts.PermissionMode,
		DangerouslySkipPermissions: s.opts.DangerouslySkipPermissions,
		AllowedTools:               s.opts.AllowedTools,
		DisallowedTools:            s.opts.DisallowedTools,
		BaseSystemPrompt:           baseSystemPrompt,
		TaskInstructions:           cfg.TaskInstructions,
		InitialPrompt:              prompt,
		SessionID:                  providerSessionID,
	}

	// BaseSystemPrompt 注入规则（Claude Code 专用）：
	// - 冷启动 + 会话初次创建：注入（持久化到会话上下文）
	// - Resume / 热复用：不注入（会话已有上下文，重复注入会破坏对话连贯性）
	isNewSession := !s.markerStore.Exists(providerSessionID)
	if !isNewSession && s.provider.VerifySession(providerSessionID, cfg.WorkDir) {
		opts.ResumeSession = true
		opts.ProviderSessionID = providerSessionID
		sessLog.Info("Resuming existing persistent CLI session")
		// 清除 BaseSystemPrompt：resume 时不应重新注入
		opts.BaseSystemPrompt = ""
	} else {
		if !isNewSession {
			sessLog.Warn("Marker exists but CLI session data not found, creating fresh session",
				"provider_session_id", providerSessionID)
			if err := s.markerStore.Delete(providerSessionID); err != nil {
				sessLog.Warn("Failed to delete stale marker", "error", err)
			}
			if err := s.provider.CleanupSession(providerSessionID, cfg.WorkDir); err != nil {
				sessLog.Warn("Failed to cleanup stale CLI session file", "error", err)
			}
		}
		opts.ProviderSessionID = providerSessionID
		// 新建会话：保留 BaseSystemPrompt（将在 BuildCLIArgs 中注入为 --append-system-prompt）
		sessLog.Info("Creating new persistent CLI session")
	}

	return s.provider.BuildCLIArgs(providerSessionID, opts)
}

// HTTPSessionStarter connects to opencode serve via HTTP/SSE.
type HTTPSessionStarter struct {
	transport   provider.Transport
	provider    provider.Provider
	logger      *slog.Logger
	opts        EngineOptions
	pendingIO   *HTTPSessionIO // Set when StartReading goroutine is started; cleared on success or cleanup
	markerStore persistence.SessionMarkerStore
}

// Compile-time interface verification
var _ SessionStarter = (*HTTPSessionStarter)(nil)

// NewHTTPSessionStarter creates an HTTPSessionStarter wrapping a Transport.
// The markerStore is used to detect session resumption across daemon restarts,
// mirroring the CLI session recovery mechanism.
func NewHTTPSessionStarter(transport provider.Transport, provider provider.Provider, markerStore persistence.SessionMarkerStore, logger *slog.Logger, opts EngineOptions) *HTTPSessionStarter {
	if logger == nil {
		logger = slog.Default()
	}
	return &HTTPSessionStarter{
		transport:   transport,
		provider:    provider,
		markerStore: markerStore,
		logger:      logger,
		opts:        opts,
	}
}

// TransportType implements SessionStarter.
func (s *HTTPSessionStarter) TransportType() string { return "http" }

// StartSession creates a new HTTP/SSE session.
func (s *HTTPSessionStarter) StartSession(
	ctx context.Context,
	sessionID string,
	cfg SessionConfig,
	prompt string,
	cb Callback,
) (*Session, error) {
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

	ns := cfg.Namespace
	if ns == "" {
		ns = s.opts.Namespace
	}
	uniqueStr := ns + ":session:" + sessionID
	providerSessionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(uniqueStr)).String()

	sessLog := s.logger.With(
		"namespace", ns,
		"session_id", sessionID,
		"provider_session_id", providerSessionID,
	)

	// Build HTTPSessionIO first to subscribe to SSE events BEFORE Connect().
	// This ensures we don't miss early events (e.g., session_created).
	// Pass empty session ID for now, will update after CreateSession.
	httpIO := NewHTTPSessionIO(s.transport, "", cancel, sessLog)

	// Detect if this is a resumed session using the client-side marker store.
	// Note: True HTTP session recovery (reconnecting to an existing server session with its
	// conversation history) requires the server-side session ID which is not persisted.
	// The marker only tracks whether BuildInputMessage was already sent for this sessionID.
	isResuming := s.markerStore.Exists(providerSessionID)

	if !isResuming {
		sessLog.Debug("Creating marker for new HTTP session",
			"provider_session_id", providerSessionID)
		if err := s.markerStore.Create(providerSessionID); err != nil {
			sessLog.Warn("Failed to create HTTP session marker",
				"error", err, "impact", "System prompt may be re-injected on daemon restart")
		}
	} else {
		sessLog.Info("HTTP session marker found; BuildInputMessage will be skipped on first turn",
			"provider_session_id", providerSessionID)
	}

	// Establish SSE connection to OpenCode server.
	if err := s.transport.Connect(sessCtx, provider.TransportConfig{}); err != nil {
		cancel()
		return nil, fmt.Errorf("connect transport: %w", err)
	}

	// Create HTTP session on the server.
	ocSessionID, err := s.transport.CreateSession(sessCtx, providerSessionID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create HTTP session: %w", err)
	}

	sessLog.Info("HTTP session created",
		"server_session_id", ocSessionID)

	// Update HTTPSessionIO with the real session ID
	httpIO.sessionID = ocSessionID

	// Build Session object.

	sess := &Session{
		ID:                    sessionID,
		ProviderSessionID:     providerSessionID,
		Config:                cfg,
		io:                    httpIO,
		cancel:                cancel,
		CreatedAt:             time.Now(),
		LastActive:            time.Now(),
		Status:                SessionStatusStarting,
		TaskInstructions:      cfg.TaskInstructions,
		statusChange:          make(chan SessionStatus, 10),
		logger:                sessLog,
		FirstMessageOnSession: !isResuming, // Skip BuildInputMessage on resumed sessions (marker exists)
		callback:              nil,  // Will be set by runner.go before StartReading
	}

	// Start SSE event reader goroutine (blocked by httpIO.startReadingGate).
	// The gate is closed in runner.go when SetCallback is called.
	// This ordering ensures no events are lost between session creation and callback setup:
	//   1. Connect() → SSE connection established, events buffered (64-channel)
	//   2. CreateSession() → server creates session, events for this session start arriving
	//   3. Session returned to caller
	//   4. SetCallback() → callback set AND gate closed → StartReading unblocks
	//   5. Buffered events are processed with correct callback
	//
	// Safe to call: StartReading has internal timeout (30s) that calls Close() to clean up
	// the subscriber channel even if SetCallback never arrives.
	// Store httpIO in pendingIO so CleanupOnError can close it if StartSession returns an error.
	s.pendingIO = httpIO
	panicx.SafeGo(sessLog, func() {
		httpIO.StartReading()
	})

	// Wait for session to become ready (HTTP sessions become ready immediately).
	sessLog.Debug("Calling waitForReady for HTTP session")
	sess.waitForReady(sessCtx, DefaultReadyTimeout)
	sessLog.Debug("waitForReady returned", "session_status", sess.Status)

	success = true
	s.pendingIO = nil // Clear: session successfully returned; pool manages lifecycle
	return sess, nil
}

// CleanupOnError cleans up any partially-initialized HTTPSessionIO when StartSession
// returns an error. This is necessary because HTTPSessionStarter starts the StartReading
// goroutine before returning, and if StartSession returns an error, the goroutine's
// 30s timeout is the only automatic cleanup without this.
// Safe to call multiple times (idempotent).
func (s *HTTPSessionStarter) CleanupOnError() {
	if s.pendingIO == nil {
		return
	}
	s.logger.Debug("HTTPSessionStarter.CleanupOnError: closing pending HTTPSessionIO")
	_ = s.pendingIO.Close()
	s.pendingIO = nil
}
