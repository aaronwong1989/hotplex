package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hrygo/hotplex/engine"
)

func TestGetHealth(t *testing.T) {
	handler := NewHealthHandler(nil, time.Now(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/health", nil)
	rec := httptest.NewRecorder()
	handler.getHealth(rec, req)

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

func TestGetMetrics(t *testing.T) {
	handler := NewHealthHandler(nil, time.Now(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	rec := httptest.NewRecorder()
	handler.getMetrics(rec, req)

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

	// Check for expected Prometheus format
	if !stringsContain(body, "hotplex_uptime_seconds") {
		t.Error("expected hotplex_uptime_seconds metric")
	}
}

func TestEnterDrain(t *testing.T) {
	handler := NewHealthHandler(nil, time.Now(), nil)

	body := `{"message": "Scheduled maintenance"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/drain",
		bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	handler.enterDrain(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Note: With nil engine, IsDraining() returns false
	// Full integration test requires actual Engine
}

func TestExitDrain_NotDraining(t *testing.T) {
	handler := NewHealthHandler(nil, time.Now(), nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/drain", nil)
	rec := httptest.NewRecorder()
	handler.exitDrain(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestExitDrain_NoEngine(t *testing.T) {
	// With nil engine, exitDrain should return bad request since IsDraining returns false
	handler := NewHealthHandler(nil, time.Now(), nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/drain", nil)
	rec := httptest.NewRecorder()
	handler.exitDrain(rec, req)

	// Without engine, IsDraining returns false, so returns 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestGetDrainStatus_NotDraining(t *testing.T) {
	handler := NewHealthHandler(nil, time.Now(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/drain", nil)
	rec := httptest.NewRecorder()
	handler.getDrainStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp DrainResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "active" {
		t.Errorf("expected status 'active', got %s", resp.Status)
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{1 * time.Second, "1s"},
		{30 * time.Second, "30s"},
		{1*time.Minute + 30*time.Second, "1m 30s"},
		{2*time.Hour + 15*time.Minute + 45*time.Second, "2h 15m 45s"},
	}

	for _, tt := range tests {
		result := formatUptime(tt.duration)
		if result != tt.expected {
			t.Errorf("formatUptime(%v) = %q, want %q", tt.duration, result, tt.expected)
		}
	}
}

func stringsContain(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var _ = engine.Engine{}
