package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPTransport implements Transport using HTTP clients.
// It connects to an opencode serve instance and provides SSE event streaming
// with automatic reconnection using exponential backoff.
// Uses separate HTTP clients for REST (with timeout) and SSE (no timeout).
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
		password:    cfg.Password,
		workDir:     cfg.WorkDir,
		subscribers: make(map[chan string]struct{}),
		ready:       make(chan struct{}),
		logger:      cfg.Logger.With("component", "http_transport"),
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
func (t *HTTPTransport) streamSSE(ctx context.Context) {
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := t.connectAndStream(ctx, &attempt)
		if ctx.Err() != nil {
			return
		}

		delay := backoff[min(attempt, len(backoff)-1)]
		t.logger.Warn("SSE disconnected, reconnecting",
			"attempt", attempt, "delay", delay, "error", err)
		attempt++

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// connectAndStream establishes a single SSE connection and reads events.
// It resets the backoff attempt counter on successful data receipt.
func (t *HTTPTransport) connectAndStream(ctx context.Context, attempt *int) error {
	url := t.baseURL + "/event"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
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
		return fmt.Errorf("do request: %w", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			if err := resp.Body.Close(); err != nil {
				t.logger.Warn("Failed to close response body", "error", err)
			}
			return ctx.Err()
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

		// Successfully received data - reset backoff counter
		if attempt != nil {
			*attempt = 0
		}

		// Signal ready on first event (non-blocking)
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

		t.logger.Debug("SSE event received, broadcasting to subscribers",
			"event_length", len(jsonLine),
			"subscriber_count", subCount)

		// Broadcast outside lock to avoid blocking SSE reader
		for _, sub := range subs {
			select {
			case sub <- jsonLine:
				// Success
			default:
				// Buffer full - try with brief timeout to avoid immediate drop
				select {
				case sub <- jsonLine:
					// Success after brief wait
				case <-time.After(50 * time.Millisecond):
					t.logger.Warn("Subscriber buffer full after timeout, dropping event",
						"event_length", len(jsonLine))
				}
			}
		}
	}

	if err := resp.Body.Close(); err != nil {
		t.logger.Warn("Failed to close response body", "error", err)
	}
	return scanner.Err()
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
func (t *HTTPTransport) Unsubscribe(ch <-chan string) {
	t.mu.Lock()
	// Find and remove subscriber while holding lock
	var found chan string
	for sub := range t.subscribers {
		if sub == ch {
			found = sub
			delete(t.subscribers, sub)
			break
		}
	}
	t.mu.Unlock() // Release lock before closing to avoid race with broadcast

	// Close channel outside lock to prevent panic if broadcast is writing
	if found != nil {
		close(found)
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

	// Debug: log the request body
	t.logger.Debug("HTTPTransport.Send: sending request",
		"url", url,
		"body", string(body),
		"session_id", sessionID)

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
	// Set working directory for OpenCode Server context
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir)
	}
	// Set working directory for OpenCode Server context
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
		t.logger.Warn("Health check WITHOUT Basic Auth - will likely fail")
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

// Close stops the SSE goroutine and closes all subscriber channels.
// It is idempotent and goroutine-safe.
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true
	t.running = false

	if t.cancelFn != nil {
		t.cancelFn()
		t.cancelFn = nil
	}

	// Close all subscriber channels
	for ch := range t.subscribers {
		close(ch)
		delete(t.subscribers, ch)
	}

	return nil
}

// Compile-time interface verification
var _ Transport = (*HTTPTransport)(nil)
