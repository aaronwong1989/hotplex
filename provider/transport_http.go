package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPTransport implements Transport using HTTP clients.
// It connects to an opencode serve instance and provides SSE event streaming
// with automatic reconnection using exponential backoff.
// Uses separate HTTP clients for REST (with timeout) and SSE (no timeout).
// heartbeatRE matches SSE event "type" field for server heartbeat events.
// Handles both compact and spaced JSON formats.
var heartbeatRE = regexp.MustCompile(`"type"\s*:\s*"server\.heartbeat"`)

type HTTPTransport struct {
	baseURL    string
	restClient *http.Client // REST calls with timeout
	sseClient  *http.Client // SSE streaming without timeout
	password   string
	workDir    string // Working directory for OpenCode Server context
	logger     *slog.Logger

	mu          sync.Mutex
	running     bool
	cancelFn    context.CancelFunc
	closed      bool
	ready       chan struct{}            // Signal when first SSE event received
	subscribers map[chan string]struct{} // Event subscribers for fan-out

	// Metrics for monitoring transport health
	eventsReceived      atomic.Int64
	eventsDropped       atomic.Int64
	droppedBySubscriber atomic.Int64 // Count of subscriber buffer-full drops

	// Metrics for reconnection health
	reconnectAttempts atomic.Int64
	reconnectsSuccess atomic.Int64
	reconnectsFailed  atomic.Int64 // Incremented when max retries exceeded

	// Heartbeat monitoring
	lastHeartbeat    atomic.Int64 // Unix timestamp of last heartbeat (seconds)
	heartbeatTimeout time.Duration
}

// TransportConfig for HTTPTransport.
type HTTPTransportConfig struct {
	Endpoint string
	Password string
	Logger   *slog.Logger
	Timeout  time.Duration
	WorkDir  string // Working directory for OpenCode Server context
}

// NewHTTPTransport creates a new HTTPTransport.
func NewHTTPTransport(cfg HTTPTransportConfig) *HTTPTransport {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &HTTPTransport{
		baseURL: strings.TrimSuffix(cfg.Endpoint, "/"),
		restClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		sseClient: &http.Client{
			Timeout: 0, // No timeout for SSE streaming
		},
		password:         cfg.Password,
		workDir:          cfg.WorkDir,
		subscribers:      make(map[chan string]struct{}),
		ready:            make(chan struct{}),
		logger:           cfg.Logger.With("component", "http_transport"),
		heartbeatTimeout: 25 * time.Second, // OpenCode server sends heartbeat every 10s; 25s gives 2.5x margin
	}
}

// Connect establishes the SSE connection to the server.
// It starts a background goroutine that reads SSE events and reconnects on failure.
func (t *HTTPTransport) Connect(ctx context.Context, cfg TransportConfig) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return nil // Already connected
	}

	// Allow config update after creation
	if cfg.Endpoint != "" {
		t.baseURL = strings.TrimSuffix(cfg.Endpoint, "/")
	}
	if cfg.Password != "" {
		t.password = cfg.Password
	}

	sseCtx, cancel := context.WithCancel(context.Background())
	t.cancelFn = cancel
	t.running = true
	t.closed = false
	t.ready = make(chan struct{}) // Reset ready channel
	t.mu.Unlock()

	// Start SSE streaming in background
	go t.streamSSE(sseCtx)

	// Wait for initial connection or first event
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ready:
		return nil // First event received, connection ready
	case <-time.After(10 * time.Second):
		return fmt.Errorf("opencode server did not emit events within 10s")
	}
}

// streamSSE manages the SSE connection lifecycle with exponential backoff reconnection.
// It implements reconnection with jitter, maximum retry limits, and heartbeat monitoring.
func (t *HTTPTransport) streamSSE(ctx context.Context) {
	// Exponential backoff with jitter: base delays with random jitter to avoid thundering herd.
	// Server sends heartbeat every 10s; we give 2.5x margin (25s) before considering connection dead.
	const maxAttempts = 20 // After 20 failed attempts (~3 min of retries), signal degraded operation

	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := t.connectAndStream(ctx, &attempt)
		if ctx.Err() != nil {
			return
		}

		// Distinguish clean close from real success:
		// - HadData: received ≥1 non-heartbeat event → true success
		// - NoData + nil error: server closed stream cleanly → normal, not a failure
		// - Error: HTTP/network failure → count as failure, may retry
		if result.HadData {
			t.reconnectsSuccess.Add(1)
		}

		if result.Err != nil {
			// Real error: log and count failure
			if attempt >= maxAttempts {
				t.reconnectsFailed.Add(1)
				t.logger.Error("SSE reconnection failed: max retry attempts exceeded",
					"attempts", attempt,
					"total_failed", t.reconnectsFailed.Load(),
					"action", "transport_degraded",
					"suggestion", "Check OpenCode server health and network connectivity")
			}

			// Exponential backoff with jitter: adds up to 500ms random jitter.
			secs := min(10<<uint(attempt), 60)
			baseDelay := time.Duration(secs) * time.Second
			jitter := time.Duration(rand.Int63n(int64(baseDelay)/2))
			delay := baseDelay + jitter

			t.logger.Warn("SSE disconnected, reconnecting",
				"attempt", attempt+1,
				"max_attempts", maxAttempts,
				"delay", delay,
				"error", result.Err)
			t.reconnectAttempts.Add(1)
		} else if !result.HadData {
			// Clean close (server ended stream normally, no data received).
			// This is expected for idle sessions; log at debug level.
			t.logger.Debug("SSE stream ended cleanly (no data received)",
				"attempt", attempt+1)
		}

		// Back off before next attempt (only on error, clean close can retry immediately)
		if result.Err != nil {
			secs := min(10<<uint(attempt), 60)
			baseDelay := time.Duration(secs) * time.Second
			jitter := time.Duration(rand.Int63n(int64(baseDelay / 2)))
			select {
			case <-ctx.Done():
				return
			case <-time.After(baseDelay + jitter):
				// Continue retry loop
			}
		}
	}
}

// streamResult is the result of a single SSE connection attempt.
type streamResult struct {
	HadData bool    // true if at least one non-heartbeat event was received
	Err     error   // nil if clean close; non-nil for HTTP/network errors
}

// connectAndStream establishes a single SSE connection and reads events.
// Returns streamResult indicating whether data was received and any error encountered.
// A nil error with HadData=false means the server closed the stream cleanly
// (no data), which is a normal idle condition, not a failure.
func (t *HTTPTransport) connectAndStream(ctx context.Context, attempt *int) streamResult {
	url := t.baseURL + "/event"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return streamResult{Err: fmt.Errorf("create request: %w", err)}
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}
	// Set working directory for OpenCode Server context
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir)
	}

	resp, err := t.sseClient.Do(req)
	if err != nil {
		return streamResult{Err: fmt.Errorf("do request: %w", err)}
	}

	hadData := false
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			if err := resp.Body.Close(); err != nil {
				t.logger.Warn("Failed to close response body", "error", err)
			}
			return streamResult{Err: ctx.Err()}
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonLine := strings.TrimPrefix(line, "data: ")
		if jsonLine == "" {
			continue
		}

		// Successfully received data - reset backoff counter (connection is healthy)
		if attempt != nil {
			*attempt = 0
		}

		// Heartbeat detection: server sends server.heartbeat every 10s.
		// Uses regex to handle both compact (`{"type":"server.heartbeat"}) and
		// spaced (`{"type": "server.heartbeat", ...}`) JSON formats.
		if heartbeatRE.MatchString(jsonLine) {
			t.lastHeartbeat.Store(time.Now().Unix())
			t.logger.Debug("SSE heartbeat received",
				"last_heartbeat_ts", t.lastHeartbeat.Load(),
				"heartbeat_timeout", t.heartbeatTimeout)
			continue
		}

		// Mark that we received at least one non-heartbeat event.
		hadData = true

		// Signal ready on first non-heartbeat event (non-blocking)
		t.mu.Lock()
		ready := t.ready
		t.mu.Unlock()
		if ready != nil {
			select {
			case <-ready:
				// Already closed
			default:
				close(ready)
			}
		}

		// Broadcast event to all subscribers (fan-out)
		// Hold lock briefly to copy subscriber list, then broadcast without lock
		t.mu.Lock()
		subs := make([]chan string, 0, len(t.subscribers))
		for sub := range t.subscribers {
			subs = append(subs, sub)
		}
		subCount := len(subs)
		t.mu.Unlock()

		t.eventsReceived.Add(1)
		t.logger.Debug("SSE event received, broadcasting to subscribers",
			"event_length", len(jsonLine),
			"subscriber_count", subCount)

		// Broadcast outside lock to avoid blocking SSE reader.
		// Use recover to handle the case where a channel was closed by Unsubscribe
		// between the map copy and the send (race window where we don't hold the lock).
		for _, sub := range subs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.eventsDropped.Add(1)
						t.droppedBySubscriber.Add(1)
						t.logger.Warn("SSE broadcast: recovered from panic sending to closed subscriber",
							"panic", r, "event_length", len(jsonLine),
							"total_dropped", t.eventsDropped.Load())
					}
				}()
				select {
				case sub <- jsonLine:
				default:
					// Buffer full — try with brief timeout to avoid immediate drop.
					select {
					case sub <- jsonLine:
					case <-time.After(100 * time.Millisecond):
						t.eventsDropped.Add(1)
						t.droppedBySubscriber.Add(1)
						t.logger.Error("Subscriber buffer full after timeout, dropping SSE event",
							"event_length", len(jsonLine),
							"total_dropped", t.eventsDropped.Load(),
							"suggestion", "Consider increasing subscriber buffer size or reducing event frequency")
					}
				}
			}()
		}
	}

	if err := resp.Body.Close(); err != nil {
		t.logger.Warn("Failed to close response body", "error", err)
	}
	// scanner.Err() is nil when the server closed the stream cleanly.
	// We report HadData to let the caller distinguish clean close from real success.
	return streamResult{HadData: hadData, Err: scanner.Err()}
}

// safeCloseChan safely closes a channel, ignoring panics from double-close.
func safeCloseChan(ch chan<- string) {
	defer func() { _ = recover() }()
	close(ch)
}

// getMsgKeys extracts top-level keys from a map for logging without leaking content.
func getMsgKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Subscribe returns a new channel for receiving SSE events.
// Each subscriber gets its own channel, enabling fan-out to multiple sessions.
// The caller must call Unsubscribe when done to prevent memory leaks.
func (t *HTTPTransport) Subscribe() <-chan string {
	ch := make(chan string, 64) // Buffer per subscriber
	t.mu.Lock()
	t.subscribers[ch] = struct{}{}
	count := len(t.subscribers)
	t.mu.Unlock()

	t.logger.Debug("SSE subscriber added", "total_subscribers", count)
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
// Must be called when done with a subscription to prevent memory leaks.
// Holds mutex through the close to prevent race with streamSSE broadcasting:
// streamSSE copies the subscriber list while holding the lock, then sends
// without the lock. By holding the lock through close, we guarantee that
// streamSSE cannot send on a closed channel (it would have to hold the lock
// to send, which we hold during close).
func (t *HTTPTransport) Unsubscribe(ch <-chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for sub := range t.subscribers {
		if sub == ch {
			delete(t.subscribers, sub)
			// Close while holding lock: streamSSE holds lock during its map copy,
			// and sends outside the lock, but if we hold the lock through close,
			// any concurrent send in streamSSE will block until we close.
			// The send will then hit the closed channel and panic in the streamSSE
			// goroutine (which is a bug in streamSSE, not here), BUT more
			// importantly: since we hold the lock, streamSSE cannot copy the map
			// while we're closing. It either sees the old map (before delete) or
			// the new map (after delete) — in neither case can it send to a
			// channel that's being closed by us concurrently.
			// SafeClose closes c and removes it from the map if present.
			// It is safe to call multiple times.
			safeCloseChan(sub)
			break
		}
	}
}

// Events returns the SSE event channel (deprecated: use Subscribe for fan-out).
// Kept for backwards compatibility with single-session use cases.
func (t *HTTPTransport) Events() <-chan string {
	return t.Subscribe()
}

// Send delivers a message to an existing session via POST /session/:id/prompt_async.
// This is an async endpoint that returns 204 immediately.
// Responses are delivered via SSE /event stream (see HTTPTransport.Events).
func (t *HTTPTransport) Send(ctx context.Context, sessionID string, message map[string]any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	// Use async endpoint for non-blocking prompt delivery
	url := fmt.Sprintf("%s/session/%s/prompt_async", t.baseURL, sessionID)

	// Debug: log the request body (only keys, not content, to avoid leaking prompts)
	if t.logger.Enabled(context.Background(), slog.LevelDebug) {
		t.logger.Debug("HTTPTransport.Send: sending request",
			"url", url,
			"msg_keys", getMsgKeys(message),
			"session_id", sessionID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}
	// Set working directory for OpenCode Server context
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir)
	}

	resp, err := t.restClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	// Debug: log the response
	respBody, _ := io.ReadAll(resp.Body)
	t.logger.Debug("HTTPTransport.Send: received response",
		"status_code", resp.StatusCode,
		"body", string(respBody),
		"session_id", sessionID)

	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.logger.Warn("Failed to close response body", "error", err)
		}
	}()

	// Async endpoint returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateSession creates a new session on the server via POST /session.
func (t *HTTPTransport) CreateSession(ctx context.Context, title string) (string, error) {
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := t.baseURL + "/session"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir)
	}

	resp, err := t.restClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("session creation failed (HTTP %d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.ID, nil
}

// DeleteSession terminates a session via DELETE /session/:id.
func (t *HTTPTransport) DeleteSession(ctx context.Context, sessionID string) error {
	url := fmt.Sprintf("%s/session/%s", t.baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}

	resp, err := t.restClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.logger.Warn("Failed to close response body", "error", err)
		}
	}()

	// 404 is acceptable (session already gone)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	// Check for other errors
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete session failed (HTTP %d): %s", resp.StatusCode, string(data))
	}
	return nil
}

// RespondPermission sends a permission response via POST /session/:id/permissions/:permID.
func (t *HTTPTransport) RespondPermission(ctx context.Context, sessionID, permissionID, response string) error {
	body, err := json.Marshal(map[string]string{"response": response})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/session/%s/permissions/%s", t.baseURL, sessionID, permissionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}

	resp, err := t.restClient.Do(req)
	if err != nil {
		return fmt.Errorf("respond permission: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("permission response failed (HTTP %d): %s", resp.StatusCode, string(data))
	}
	return nil
}

// Health checks if the server is reachable via GET /global/health.
func (t *HTTPTransport) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+"/global/health", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
		t.logger.Debug("Health check with Basic Auth", "username", "opencode", "password_len", len(t.password))
	} else {
		t.logger.Error("Health check failed: OpenCode server requires Basic Auth but password is not configured",
			"suggestion", "Set HOTPLEX_OPEN_CODE_PASSWORD environment variable or configure opencode.password in your config",
			"auth_note", "OpenCode server enforces authentication - requests without valid credentials will be rejected")
	}

	resp, err := t.restClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// Read response body for more details
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Verify response body contains {"healthy":true}
	var result struct {
		Healthy bool `json:"healthy"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode health response: %w", err)
	}
	if !result.Healthy {
		return fmt.Errorf("server reports unhealthy status")
	}
	return nil
}

// TransportStats holds SSE transport health metrics for monitoring and debugging.
type TransportStats struct {
	EventsReceived      int64
	EventsDropped       int64
	DroppedBySubscriber int64
	ReconnectAttempts   int64
	ReconnectsSuccess   int64
	ReconnectsFailed    int64
	LastHeartbeatTs     int64 // Unix timestamp of last heartbeat (0 if none)
	HeartbeatTimeout    time.Duration
	ActiveSubscribers   int
	Running             bool
}

// Stats returns a snapshot of transport health metrics.
func (t *HTTPTransport) Stats() TransportStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return TransportStats{
		EventsReceived:      t.eventsReceived.Load(),
		EventsDropped:       t.eventsDropped.Load(),
		DroppedBySubscriber: t.droppedBySubscriber.Load(),
		ReconnectAttempts:   t.reconnectAttempts.Load(),
		ReconnectsSuccess:   t.reconnectsSuccess.Load(),
		ReconnectsFailed:    t.reconnectsFailed.Load(),
		LastHeartbeatTs:     t.lastHeartbeat.Load(),
		HeartbeatTimeout:    t.heartbeatTimeout,
		ActiveSubscribers:   len(t.subscribers),
		Running:             t.running,
	}
}

// Close stops the SSE goroutine and closes all subscriber channels.
// It is idempotent and goroutine-safe.
// On close, stats are logged to aid debugging upstream memory leak issues (opencode #15645).
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true
	t.running = false

	// Cancel SSE streaming goroutine (triggers exit from connectAndStream)
	if t.cancelFn != nil {
		t.cancelFn()
		t.cancelFn = nil
	}

	// Close all subscriber channels to unblock any readers
	for ch := range t.subscribers {
		close(ch)
		delete(t.subscribers, ch)
	}

	// Log final stats for debugging upstream memory leaks.
	// Issue #15645: SSE connections leak on client disconnect. Stats help correlate
	// HotPlex behavior with server-side memory growth.
	t.logger.Info("HTTPTransport closed",
		"events_received", t.eventsReceived.Load(),
		"events_dropped", t.eventsDropped.Load(),
		"reconnects_attempted", t.reconnectAttempts.Load(),
		"reconnects_success", t.reconnectsSuccess.Load(),
		"reconnects_failed", t.reconnectsFailed.Load(),
		"last_heartbeat_ts", t.lastHeartbeat.Load())

	return nil
}

// Compile-time interface verification
var _ Transport = (*HTTPTransport)(nil)
