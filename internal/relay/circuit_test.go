package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// RelayCircuitBreaker
// ---------------------------------------------------------------------------

func TestNewRelayCircuitBreaker_NonNil(t *testing.T) {
	cb := NewRelayCircuitBreaker()
	if cb == nil {
		t.Fatal("NewRelayCircuitBreaker returned nil")
	}
}

func TestRelayCircuitBreaker_Get_SameInstance(t *testing.T) {
	cb := NewRelayCircuitBreaker()

	b1 := cb.Get("agent-a")
	b2 := cb.Get("agent-a")

	if b1 != b2 {
		t.Error("Get should return the same circuit breaker instance for the same name")
	}
}

func TestRelayCircuitBreaker_Get_DifferentNames(t *testing.T) {
	cb := NewRelayCircuitBreaker()

	b1 := cb.Get("agent-a")
	b2 := cb.Get("agent-b")

	if b1 == b2 {
		t.Error("Get should return different instances for different names")
	}
}

func TestRelayCircuitBreaker_Get_Concurrent(t *testing.T) {
	cb := NewRelayCircuitBreaker()

	const goroutines = 50
	done := make(chan *struct{}, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			cb.Get("concurrent-agent")
			done <- nil
		}()
	}

	// Wait for all goroutines to finish.
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// All should be the same instance.
	b := cb.Get("concurrent-agent")
	if b == nil {
		t.Error("expected non-nil breaker")
	}
}

func TestRelayCircuitBreaker_Call_Success(t *testing.T) {
	cb := NewRelayCircuitBreaker()

	result, err := cb.Call(context.Background(), "agent-ok", func() (any, error) {
		return "hello", nil
	})

	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "hello" {
		t.Errorf("result = %v, want %q", result, "hello")
	}
}

func TestRelayCircuitBreaker_Call_Error(t *testing.T) {
	cb := NewRelayCircuitBreaker()

	expectedErr := errors.New("send failed")
	_, err := cb.Call(context.Background(), "agent-err", func() (any, error) {
		return nil, expectedErr
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("err = %v, want %v", err, expectedErr)
	}
}

func TestRelayCircuitBreaker_TripsAfterConsecutiveFailures(t *testing.T) {
	cb := NewRelayCircuitBreaker()

	// Trip the breaker with 5 consecutive failures.
	for i := 0; i < 5; i++ {
		_, _ = cb.Call(context.Background(), "trip-agent", func() (any, error) {
			return nil, errors.New("fail")
		})
	}

	// Next call should be rejected by the open breaker.
	_, err := cb.Call(context.Background(), "trip-agent", func() (any, error) {
		return "should not reach", nil
	})
	if err == nil {
		t.Fatal("expected open circuit error, got nil")
	}
}

// ---------------------------------------------------------------------------
// RelaySender
// ---------------------------------------------------------------------------

func TestNewRelaySender_NonNil(t *testing.T) {
	s := NewRelaySender("test-token")
	if s == nil {
		t.Fatal("NewRelaySender returned nil")
	}
}

func TestRelaySender_Send_Success(t *testing.T) {
	var receivedAuth string
	var receivedMethod string
	var receivedPath string
	var receivedContentType string
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewRelaySender("my-secret-token")
	msg := &RelayMessage{
		TaskID:  "task-1",
		To:      "agent-a",
		Content: "hello relay",
		Status:  TaskStatusWorking,
	}

	err := sender.Send(context.Background(), msg, server.URL)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", receivedMethod, http.MethodPost)
	}
	if receivedPath != "/relay" {
		t.Errorf("path = %q, want %q", receivedPath, "/relay")
	}
	if receivedAuth != "Bearer my-secret-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer my-secret-token")
	}
	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", receivedContentType, "application/json")
	}

	// Verify body is valid JSON with expected fields.
	var parsed map[string]any
	if err := jsonUnmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if parsed["task_id"] != "task-1" {
		t.Errorf("task_id = %v, want %q", parsed["task_id"], "task-1")
	}
	if parsed["content"] != "hello relay" {
		t.Errorf("content = %v, want %q", parsed["content"], "hello relay")
	}
}

func TestRelaySender_Send_NoToken(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewRelaySender("")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "t1", Content: "hi"}, server.URL)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if receivedAuth != "" {
		t.Errorf("Authorization should be empty when no token, got %q", receivedAuth)
	}
}

func TestRelaySender_Send_RetriesOn5xx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewRelaySender("token")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "retry", Content: "test"}, server.URL)
	if err != nil {
		t.Fatalf("Send after retries: %v", err)
	}

	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestRelaySender_Send_NoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	sender := NewRelaySender("token")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "no-retry", Content: "test"}, server.URL)
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}

	// Should only attempt once (no retry for 4xx except 429).
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1", attempts.Load())
	}
}

func TestRelaySender_Send_RetryOn429(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewRelaySender("token")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "429-retry", Content: "test"}, server.URL)
	if err != nil {
		t.Fatalf("Send after 429 retry: %v", err)
	}

	if attempts.Load() != 2 {
		t.Errorf("attempts = %d, want 2", attempts.Load())
	}
}

func TestRelaySender_Send_ExhaustsRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	sender := NewRelaySender("token")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "exhaust", Content: "test"}, server.URL)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}

	// 1 initial + 3 retries = 4 total
	if attempts.Load() != 4 {
		t.Errorf("attempts = %d, want 4", attempts.Load())
	}
}

func TestRelaySender_Send_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	sender := NewRelaySender("token")
	err := sender.Send(ctx, &RelayMessage{TaskID: "canceled", Content: "test"}, server.URL)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestRelaySender_Send_ServerUnreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TCP timeout test in short mode")
	}
	sender := NewRelaySender("token")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "unreachable", Content: "test"}, "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestRelaySender_Send_ConnectionRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TCP timeout test in short mode")
	}
	// Use a port that is definitely not listening.
	sender := NewRelaySender("token")
	err := sender.Send(context.Background(), &RelayMessage{TaskID: "refused", Content: "test"}, "http://localhost:59999")
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
}

// ---------------------------------------------------------------------------
// RelayManager
// ---------------------------------------------------------------------------

func TestNewRelayManager(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))
	if rm == nil {
		t.Fatal("NewRelayManager returned nil")
	}
}

func TestRelayManager_AddBinding(t *testing.T) {
	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	err := rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots: map[string]string{
			"agent-a": "http://localhost:8080",
		},
	})
	if err != nil {
		t.Fatalf("AddBinding: %v", err)
	}

	bindings := rm.ListBindings()
	if len(bindings) != 1 {
		t.Fatalf("ListBindings len = %d, want 1", len(bindings))
	}
}

func TestRelayManager_AddBinding_NilBinding(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))

	err := rm.AddBinding(nil)
	if err == nil {
		t.Fatal("expected error for nil binding, got nil")
	}
}

func TestRelayManager_AddBinding_EmptyChatID(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))

	err := rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "",
		Bots:     map[string]string{"a": "http://a"},
	})
	if err == nil {
		t.Fatal("expected error for empty chat_id, got nil")
	}
}

func TestRelayManager_AddBinding_NilBots(t *testing.T) {
	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	// Bots map is nil; AddBinding should initialize it.
	err := rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     nil,
	})
	if err != nil {
		t.Fatalf("AddBinding with nil Bots: %v", err)
	}

	bindings := rm.ListBindings()
	if len(bindings) != 1 {
		t.Fatalf("ListBindings len = %d, want 1", len(bindings))
	}
}

func TestRelayManager_RemoveBinding(t *testing.T) {
	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	_ = rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     map[string]string{"agent-a": "http://localhost:8080"},
	})

	err := rm.RemoveBinding("slack", "C123")
	if err != nil {
		t.Fatalf("RemoveBinding: %v", err)
	}

	bindings := rm.ListBindings()
	if len(bindings) != 0 {
		t.Errorf("ListBindings after remove = %d, want 0", len(bindings))
	}
}

func TestRelayManager_RemoveBinding_NotFound(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))

	err := rm.RemoveBinding("slack", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent binding, got nil")
	}
}

func TestRelayManager_RemoveBinding_WrongPlatform(t *testing.T) {
	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	_ = rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     map[string]string{"agent-a": "http://a"},
	})

	err := rm.RemoveBinding("telegram", "C123")
	if err == nil {
		t.Fatal("expected error for wrong platform, got nil")
	}
}

func TestRelayManager_Send_Success(t *testing.T) {
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	_ = rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots: map[string]string{
			"agent-a": server.URL,
		},
	})

	resp, err := rm.Send(context.Background(), "agent-a", "hello")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Status != "sent" {
		t.Errorf("Status = %q, want %q", resp.Status, "sent")
	}
	if resp.TaskID == "" {
		t.Error("TaskID should not be empty")
	}

	// Verify the message was delivered correctly.
	var parsed map[string]any
	if err := jsonUnmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if parsed["to"] != "agent-a" {
		t.Errorf("to = %v, want %q", parsed["to"], "agent-a")
	}
	if parsed["content"] != "hello" {
		t.Errorf("content = %v, want %q", parsed["content"], "hello")
	}
	if parsed["status"] != TaskStatusWorking {
		t.Errorf("status = %v, want %q", parsed["status"], TaskStatusWorking)
	}
}

func TestRelayManager_Send_EmptyToAgent(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))

	_, err := rm.Send(context.Background(), "", "content")
	if err == nil {
		t.Fatal("expected error for empty toAgent, got nil")
	}
}

func TestRelayManager_Send_EmptyContent(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))

	_, err := rm.Send(context.Background(), "agent-x", "")
	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}
}

func TestRelayManager_Send_AgentNotFound(t *testing.T) {
	rm := NewRelayManager(NewRelaySender("token"))

	_, err := rm.Send(context.Background(), "nonexistent-agent", "hello")
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
}

func TestRelayManager_Send_ServerError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	_ = rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     map[string]string{"agent-a": server.URL},
	})

	// Will retry and eventually fail.
	_, err := rm.Send(context.Background(), "agent-a", "hello")
	if err == nil {
		t.Fatal("expected error for server failure, got nil")
	}
}

func TestRelayManager_MultipleAgentsSameBinding(t *testing.T) {
	var called atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	rm := newRelayManagerWithDir(dir)

	_ = rm.AddBinding(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots: map[string]string{
			"agent-a": server.URL,
			"agent-b": server.URL,
		},
	})

	// Send to agent-a.
	_, err := rm.Send(context.Background(), "agent-a", "msg-a")
	if err != nil {
		t.Fatalf("Send agent-a: %v", err)
	}

	// Send to agent-b.
	_, err = rm.Send(context.Background(), "agent-b", "msg-b")
	if err != nil {
		t.Fatalf("Send agent-b: %v", err)
	}

	if called.Load() != 2 {
		t.Errorf("server calls = %d, want 2", called.Load())
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newRelayManagerWithDir creates a RelayManager backed by a temp directory.
func newRelayManagerWithDir(dir string) *RelayManager {
	store := newBindingStoreWithPath(dir)
	return &RelayManager{
		sender:  NewRelaySender("test-token"),
		cb:      NewRelayCircuitBreaker(),
		store:   store,
		byAgent: make(map[string]string),
	}
}

// jsonUnmarshal is a test helper that wraps json.Unmarshal.
func jsonUnmarshal(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
