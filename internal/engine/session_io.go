package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/hrygo/hotplex/internal/sys"
	"github.com/hrygo/hotplex/provider"
)

// SessionCallback handles streaming events from an active session.
// This is the concrete callback signature used by HTTPSessionIO goroutines.
// Events are dispatched as they occur, allowing real-time UI updates.
type SessionCallback func(eventType string, data any) error

// SessionIO abstracts the I/O transport between HotPlex and the AI agent backend.
// Two implementations:
//   - CLISessionIO: stdin/stdout/stderr pipes to a subprocess
//   - HTTPSessionIO: HTTP client to opencode serve
//
// Session holds a concrete SessionIO and calls its methods without nil checks.
type SessionIO interface {
	// WriteInput serializes msg as JSON and delivers it to the agent session.
	WriteInput(msg map[string]any) error

	// Close releases all resources held by the SessionIO.
	// Close is idempotent and goroutine-safe.
	Close() error

	// Logger returns the session logger.
	Logger() *slog.Logger

	// IsCLI returns true if this is a CLI subprocess transport.
	// Used by cleanupSessionLocked to determine cleanup strategy.
	IsCLI() bool

	// IsAlive returns true if the transport is still active.
	// For CLI: checks if the process is still running.
	// For HTTP: checks if the session connection is still open.
	IsAlive() bool
}

// CLISessionIO wraps os pipe handles for CLI subprocess transport.
type CLISessionIO struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	cancel func()
	logger *slog.Logger
}

// NewCLISessionIO creates a new CLISessionIO from pipe handles.
func NewCLISessionIO(
	cmd *exec.Cmd,
	stdin io.WriteCloser,
	stdout, stderr io.ReadCloser,
	cancel func(),
	logger *slog.Logger,
) *CLISessionIO {
	if logger == nil {
		logger = slog.Default()
	}
	return &CLISessionIO{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		cancel: cancel,
		logger: logger,
	}
}

// WriteInput implements SessionIO.
func (c *CLISessionIO) WriteInput(msg map[string]any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

// Stdout returns the stdout reader for the Session's read goroutines.
func (c *CLISessionIO) Stdout() io.ReadCloser { return c.stdout }

// Stderr returns the stderr reader for the Session's read goroutines.
func (c *CLISessionIO) Stderr() io.ReadCloser { return c.stderr }

// Close implements SessionIO. It closes all pipes and cancels the subprocess context.
func (c *CLISessionIO) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
		c.stdout = nil
	}
	if c.stderr != nil {
		_ = c.stderr.Close()
		c.stderr = nil
	}
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	return nil
}

// Logger implements SessionIO.
func (c *CLISessionIO) Logger() *slog.Logger { return c.logger }

// IsCLI implements SessionIO. Returns true for CLI transport.
func (c *CLISessionIO) IsCLI() bool { return true }

// IsAlive implements SessionIO. Returns true if the CLI process is still running.
func (c *CLISessionIO) IsAlive() bool {
	if c.cmd == nil || c.cmd.Process == nil {
		return false
	}
	return sys.IsProcessAlive(c.cmd.Process)
}

// HTTPSessionIO sends prompts via HTTP POST and receives events via SSE.
type HTTPSessionIO struct {
	transport provider.Transport
	sessionID string
	logger    *slog.Logger

	// callback handles session events (runner_exit, raw_line).
	// Set via SetCallback before StartReading is called.
	callback SessionCallback

	// eventsCh is the subscribed SSE event channel from transport.
	// Each HTTPSessionIO gets its own channel for fan-out.
	eventsCh <-chan string

	// startReadingGate blocks StartReading until StartReading is explicitly called.
	// This ensures the SSE reader does not begin before the session callback is set,
	// preventing event loss during the session creation window (Connect → CreateSession → SetCallback).
	startReadingGate chan struct{}

	mu       sync.Mutex
	closed   bool
	cancelFn func()
}

// NewHTTPSessionIO creates a new HTTPSessionIO wrapping a Transport and session ID.
// The callback must be set via SetCallback before StartReading is called.
func NewHTTPSessionIO(transport provider.Transport, sessionID string, cancelFn func(), logger *slog.Logger) *HTTPSessionIO {
	if logger == nil {
		logger = slog.Default()
	}

	// Subscribe to SSE events for fan-out
	var eventsCh <-chan string
	if subscriber, ok := transport.(interface{ Subscribe() <-chan string }); ok {
		eventsCh = subscriber.Subscribe()
	} else {
		// Fallback to deprecated Events() method
		eventsCh = transport.Events()
		logger.Warn("Transport does not implement Subscribe(), using deprecated Events()", "session_id", sessionID)
	}

	return &HTTPSessionIO{
		transport: transport,
		sessionID: sessionID,
		logger:    logger.With("session_id", sessionID),
		eventsCh:  eventsCh,
		cancelFn:  cancelFn,
		// Gate is created open in NewHTTPSessionIO (for CLISessionStarter compatibility).
		// HTTPSessionStarter.StartSession creates HTTPSessionIO with an open gate, but
		// will close it immediately after CreateSession returns, before returning to caller.
		// The SSE goroutine (runner.go) will then start reading after SetCallback.
		startReadingGate: make(chan struct{}),
	}
}

// SetCallback sets the session callback for event dispatch.
// It also closes the startReadingGate to unblock the SSE reader goroutine.
func (h *HTTPSessionIO) SetCallback(cb SessionCallback) {
	h.callback = cb
	// Signal the SSE reader to start. This happens in runner.go after
	// GetOrCreateSession returns and before WriteInput is called.
	// The goroutine was already started in session_starter.go via SafeGo(httpIO.StartReading),
	// but blocked here until the callback is ready.
	close(h.startReadingGate)
}

// WriteInput implements SessionIO.
// It sends the message via HTTP POST to the opencode serve session.
func (h *HTTPSessionIO) WriteInput(msg map[string]any) error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.logger.Error("WriteInput rejected: session closed", "session_id", h.sessionID)
		return fmt.Errorf("session closed")
	}
	h.mu.Unlock()

	h.logger.Info("HTTPSessionIO.WriteInput sending message",
		"session_id", h.sessionID,
		"msg_keys", getMapKeys(msg),
		"msg_json", mustMarshalJSON(msg))

	if err := h.transport.Send(context.Background(), h.sessionID, msg); err != nil {
		h.logger.Error("HTTPSessionIO.WriteInput failed",
			"session_id", h.sessionID,
			"error", err)
		return err
	}

	h.logger.Info("HTTPSessionIO.WriteInput succeeded", "session_id", h.sessionID)
	return nil
}

func mustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(b)
}

func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Close implements SessionIO. It cancels the SSE context and deletes the server session.
func (h *HTTPSessionIO) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true

	// Unsubscribe from transport events
	if unsub, ok := h.transport.(interface{ Unsubscribe(<-chan string) }); ok && h.eventsCh != nil {
		unsub.Unsubscribe(h.eventsCh)
		h.eventsCh = nil
	}

	if h.cancelFn != nil {
		h.cancelFn()
		h.cancelFn = nil
	}
	// Use timeout context to prevent hanging on server unresponsiveness.
	// 5 seconds is sufficient for server-side session cleanup.
	deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.transport.DeleteSession(deleteCtx, h.sessionID); err != nil {
		h.logger.Warn("Failed to delete HTTP session on close",
			"session_id", h.sessionID,
			"error", err)
	}
	return nil
}

// Logger implements SessionIO.
func (h *HTTPSessionIO) Logger() *slog.Logger { return h.logger }

// IsCLI implements SessionIO. Returns false for HTTP transport.
func (h *HTTPSessionIO) IsCLI() bool { return false }

// IsAlive implements SessionIO. Returns true if the HTTP session is not closed.
func (h *HTTPSessionIO) IsAlive() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.closed
}

// StartReading starts the SSE event dispatch loop.
// It blocks on startReadingGate until SetCallback is called (in runner.go),
// ensuring the SSE reader never processes events without a valid callback.
// Call via panicx.SafeGo.
func (h *HTTPSessionIO) StartReading() {
	// Gate wait: ensures the SSE reader does not begin processing events
	// before the session callback is set in runner.go.
	// Events arriving on the SSE channel before this point are buffered
	// by the transport's 64-channel subscriber buffer.
	h.logger.Debug("StartReading: waiting for gate (callback setup)")
	select {
	case <-h.startReadingGate:
		h.logger.Debug("StartReading: gate opened, callback is ready")
	case <-time.After(30 * time.Second):
		h.logger.Error("StartReading: timeout waiting for gate, exiting")
		return
	}

	h.mu.Lock()
	cb := h.callback
	eventsCh := h.eventsCh
	h.mu.Unlock()

	h.logger.Debug("StartReading executing",
		"has_events_channel", eventsCh != nil,
		"has_callback", cb != nil)

	defer func() {
		h.logger.Info("StartReading exiting, calling runner_exit")
		if cb != nil {
			_ = cb("runner_exit", nil)
		}
		if err := h.Close(); err != nil {
			h.logger.Warn("Failed to close session", "error", err)
		}
	}()

	if eventsCh == nil {
		h.logger.Warn("StartReading: events channel is nil, exiting")
		return
	}

	eventCount := 0
	for line := range eventsCh {
		eventCount++
		if line == "" {
			continue
		}
		h.logger.Debug("StartReading received event", "event_number", eventCount, "event_length", len(line))
		if cb != nil {
			if err := cb("raw_line", line); err != nil {
				h.logger.Error("callback returned error", "error", err, "event_number", eventCount)
			} else {
				h.logger.Debug("callback succeeded", "event_number", eventCount)
			}
		} else {
			h.logger.Warn("StartReading: callback is nil, dropping event", "event_number", eventCount)
		}
	}
	h.logger.Info("StartReading: events channel closed", "total_events", eventCount)
}

// ReadStderrFromHTTPSession is a no-op for HTTP sessions (no stderr).
func ReadStderrFromHTTPSession(_ *slog.Logger) {}

var _ SessionIO = (*CLISessionIO)(nil)
var _ SessionIO = (*HTTPSessionIO)(nil)
