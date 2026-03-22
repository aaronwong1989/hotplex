package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAdminServer(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{})
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.mux == nil {
		t.Error("expected non-nil mux")
	}
	if server.healthHandler == nil {
		t.Error("expected non-nil healthHandler")
	}
	if server.sessionHandler == nil {
		t.Error("expected non-nil sessionHandler")
	}
	if server.configHandler == nil {
		t.Error("expected non-nil configHandler")
	}
	if server.auditHandler == nil {
		t.Error("expected non-nil auditHandler")
	}
	if server.eventBuffer == nil {
		t.Error("expected non-nil eventBuffer")
	}
}

func TestNewAdminServer_WithNilEngine(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		EventBufferSize: 500,
	})
	if server.eventBuffer.MaxSize() != 500 {
		t.Errorf("expected event buffer size 500, got %d", server.eventBuffer.MaxSize())
	}
}

// Test paths use /api/v1/admin/... without trailing slashes to match mux patterns.
// mux pattern: /api/v1/admin/ strips to remaining path.
// registered handler: GET /health (without trailing slash).

func TestAdminServer_ServeHTTP_Health(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "test-key-123",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/health", nil)
	req.Header.Set("X-Admin-Key", "test-key-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp AdminHealth
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestAdminServer_ServeHTTP_Health_Unauthorized(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "test-key-123",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAdminServer_ServeHTTP_Health_InvalidKey(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "test-key-123",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/health", nil)
	req.Header.Set("X-Admin-Key", "wrong-key")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAdminServer_ServeHTTP_Events(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "test-key-123",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil)
	req.Header.Set("X-Admin-Key", "test-key-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp EventsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

func TestAdminServer_ServeHTTP_Sessions(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "test-key-123",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/sessions", nil)
	req.Header.Set("X-Admin-Key", "test-key-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp SessionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

func TestAdminServer_GetHealthHandler(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{})

	h := server.GetHealthHandler()
	if h == nil {
		t.Error("expected non-nil health handler")
	}
}

func TestAdminServer_GetEventBuffer(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{})

	buf := server.GetEventBuffer()
	if buf == nil {
		t.Error("expected non-nil event buffer")
	}
}

func TestAdminServer_DrainEnterAndStatus(t *testing.T) {
	// Test enter drain and get status with nil engine
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "drain-key",
	})

	// Enter drain
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/drain", nil)
	req.Header.Set("X-Admin-Key", "drain-key")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 from enterDrain, got %d", rec.Code)
	}

	// Get drain status
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/drain", nil)
	req2.Header.Set("X-Admin-Key", "drain-key")
	rec2 := httptest.NewRecorder()
	server.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected status 200 from getDrainStatus, got %d", rec2.Code)
	}
}

func TestAdminServer_ExitDrain_NotDraining(t *testing.T) {
	// With nil engine, exitDrain should return 400 (not in drain mode)
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "drain-key",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/drain", nil)
	req.Header.Set("X-Admin-Key", "drain-key")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when not draining, got %d", rec.Code)
	}
}

func TestAdminServer_ConfigEndpoints(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "config-key",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	req.Header.Set("X-Admin-Key", "config-key")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 for nil engine, got %d", rec.Code)
	}
}

func TestAdminServer_Metrics(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{
		AdminKey: "metrics-key",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	req.Header.Set("X-Admin-Key", "metrics-key")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty metrics body")
	}
}

func TestAdminServer_NotFound(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nonexistent", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestAdminServer_PushEvent(t *testing.T) {
	server := NewAdminServer(AdminServerOptions{})

	event := AdminEvent{
		EventID:   "test-event-1",
		Type:      "session_start",
		SessionID: "test-session",
		Timestamp: time.Now(),
		Actor:     "admin",
	}
	server.eventBuffer.Push(event)

	if server.eventBuffer.Size() != 1 {
		t.Errorf("expected buffer size 1, got %d", server.eventBuffer.Size())
	}
}
