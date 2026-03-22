package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	intengine "github.com/hrygo/hotplex/internal/engine"
)

type mockSessionForAudit struct{}

func (m *mockSessionForAudit) GetSession(sessionID string) (*intengine.Session, bool) {
	return nil, false
}

func TestGetEvents_EmptyBuffer(t *testing.T) {
	buf := NewEventBuffer(100)
	handler := NewAuditHandler(buf, &mockSessionForAudit{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil)
	rec := httptest.NewRecorder()
	handler.getEvents(rec, req)

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
	if len(resp.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(resp.Events))
	}
}

func TestGetEvents_WithEvents(t *testing.T) {
	buf := NewEventBuffer(100)
	handler := NewAuditHandler(buf, &mockSessionForAudit{})

	buf.Push(AdminEvent{EventID: "evt-1", Type: "session_start", Timestamp: time.Now()})
	buf.Push(AdminEvent{EventID: "evt-2", Type: "session_end", Timestamp: time.Now()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil)
	rec := httptest.NewRecorder()
	handler.getEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp EventsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
}

func TestGetEvents_Pagination(t *testing.T) {
	buf := NewEventBuffer(100)
	handler := NewAuditHandler(buf, &mockSessionForAudit{})

	for i := 0; i < 5; i++ {
		buf.Push(AdminEvent{EventID: "evt-" + string(rune('0'+i)), Type: "test", Timestamp: time.Now()})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events?limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	handler.getEvents(rec, req)

	var resp EventsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Total)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
	if resp.NextCursor != "2" {
		t.Errorf("expected next_cursor '2', got '%s'", resp.NextCursor)
	}
}

func TestGetEvents_NilBuffer(t *testing.T) {
	handler := NewAuditHandler(nil, &mockSessionForAudit{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil)
	rec := httptest.NewRecorder()
	handler.getEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp EventsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected total 0 for nil buffer, got %d", resp.Total)
	}
}

func TestGetTranscript_SessionNotFound(t *testing.T) {
	buf := NewEventBuffer(100)
	handler := NewAuditHandler(buf, &mockSessionForAudit{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/sessions/not-found/transcript", nil)
	rec := httptest.NewRecorder()
	handler.getTranscript(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	var resp AdminError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %s", resp.Error.Code)
	}
}

func TestGetTranscript_MissingSessionID(t *testing.T) {
	buf := NewEventBuffer(100)
	handler := NewAuditHandler(buf, &mockSessionForAudit{})

	// Double slash results in empty segment - extractSessionID returns ""
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/sessions//transcript", nil)
	rec := httptest.NewRecorder()
	handler.getTranscript(rec, req)

	// Empty session ID returns 404 (session not found)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestPushEvent(t *testing.T) {
	buf := NewEventBuffer(100)
	handler := NewAuditHandler(buf, &mockSessionForAudit{})

	event := AdminEvent{Type: "test_event", EventID: "test-123"}
	handler.PushEvent(event)

	if buf.Size() != 1 {
		t.Errorf("expected buffer size 1, got %d", buf.Size())
	}
}

func TestPushEvent_NilBuffer(t *testing.T) {
	handler := NewAuditHandler(nil, &mockSessionForAudit{})

	event := AdminEvent{Type: "test_event"}
	handler.PushEvent(event)
}
