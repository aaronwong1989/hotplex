package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	intengine "github.com/hrygo/hotplex/internal/engine"
)

// mockSessionManager implements SessionPoolInterface for testing.
type mockSessionManager struct {
	sessions map[string]*intengine.Session
}

func (m *mockSessionManager) ListActiveSessions() []*intengine.Session {
	result := make([]*intengine.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

func (m *mockSessionManager) GetSession(sessionID string) (*intengine.Session, bool) {
	s, ok := m.sessions[sessionID]
	return s, ok
}

func (m *mockSessionManager) TerminateSession(sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func TestListSessions(t *testing.T) {
	pool := &mockSessionManager{
		sessions: make(map[string]*intengine.Session),
	}

	handler := NewSessionHandler(pool)

	// Test empty list
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ListSessions(rec, req)

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

func TestGetSession_NotFound(t *testing.T) {
	pool := &mockSessionManager{
		sessions: make(map[string]*intengine.Session),
	}

	handler := NewSessionHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.GetSession(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestStopSession_NotFound(t *testing.T) {
	pool := &mockSessionManager{
		sessions: make(map[string]*intengine.Session),
	}

	handler := NewSessionHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/nonexistent/stop", nil)
	rec := httptest.NewRecorder()
	handler.StopSession(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestBatchStopSessions_EmptyList(t *testing.T) {
	pool := &mockSessionManager{
		sessions: make(map[string]*intengine.Session),
	}

	handler := NewSessionHandler(pool)

	body := `{"session_ids": [], "reason": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/batch-stop",
		bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	handler.BatchStopSessions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/sessions/test-session", "test-session"},
		{"/api/v1/sessions/", ""},
		// Note: /api/v1/sessions (without trailing slash and no ID) would not match
		// the session ID route in practice, as the router would match it to the list endpoint.
		// The function returns the last segment for any path with a slash.
		{"/api/v1/sessions/test", "test"},
		{"/sessions/abc-123", "abc-123"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			result := extractSessionID(req)
			if result != tt.expected {
				t.Errorf("extractSessionID(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}
