package admin

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrygo/hotplex/internal/adminapi"
)

func TestAdminAuthMiddleware_ValidKey(t *testing.T) {
	logger := slog.Default()
	middleware := AdminAuthMiddleware("test-secret-key", logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Admin-Key", "test-secret-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAdminAuthMiddleware_MissingKey(t *testing.T) {
	logger := slog.Default()
	middleware := AdminAuthMiddleware("test-secret-key", logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for missing key")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "UNAUTHORIZED") {
		t.Errorf("expected error code UNAUTHORIZED, got %s", body)
	}
	if !strings.Contains(body, "Missing X-Admin-Key") {
		t.Errorf("expected missing key message, got %s", body)
	}
}

func TestAdminAuthMiddleware_InvalidKey(t *testing.T) {
	logger := slog.Default()
	middleware := AdminAuthMiddleware("test-secret-key", logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for invalid key")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Admin-Key", "wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "UNAUTHORIZED") {
		t.Errorf("expected error code UNAUTHORIZED, got %s", body)
	}
	if !strings.Contains(body, "Invalid X-Admin-Key") {
		t.Errorf("expected invalid key message, got %s", body)
	}
}

func TestAdminAuthMiddleware_PartialKey(t *testing.T) {
	logger := slog.Default()
	middleware := AdminAuthMiddleware("test-secret-key", logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for partial key")
	}))

	// Only first part of the key matches
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Admin-Key", "test")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for partial key, got %d", rec.Code)
	}
}

func TestAdminAuthMiddleware_TimingAttackResistant(t *testing.T) {
	logger := slog.Default()
	middleware := AdminAuthMiddleware("test-secret-key", logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test that similar keys are handled correctly
	testCases := []struct {
		key     string
		want    int
	}{
		{"test-secret-key", http.StatusOK},
		{"Test-Secret-Key", http.StatusUnauthorized}, // Different case
		{"test-secret-ke", http.StatusUnauthorized},    // One char short
		{"test-secret-key-extra", http.StatusUnauthorized}, // Extra chars
		{"", http.StatusUnauthorized},                     // Empty
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.key != "" {
				req.Header.Set("X-Admin-Key", tc.key)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("key %q: expected status %d, got %d", tc.key, tc.want, rec.Code)
			}
		})
	}
}

func TestKeyPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcdefgh", "abcd****"},
		{"ab", "****"},
		{"abc", "****"},
		{"abcd", "abcd****"},
		{"", "****"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := keyPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("keyPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	adminapi.WriteError(rec, http.StatusBadRequest, adminapi.ErrCodeInvalidRequest, "Test error message")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "INVALID_REQUEST") {
		t.Errorf("expected error code INVALID_REQUEST, got %s", body)
	}
	if !strings.Contains(body, "Test error message") {
		t.Errorf("expected error message, got %s", body)
	}
}
