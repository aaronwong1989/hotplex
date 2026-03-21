package admin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer_NilLogger(t *testing.T) {
	// Should not panic when logger is nil
	srv := NewServer(nil, "9080", "token", time.Now(), nil)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.logger == nil {
		t.Error("expected logger to be set to default")
	}
}

func TestNewServer_WithSlogLogger(t *testing.T) {
	slogLogger := slog.Default()
	srv := NewServer(nil, "9080", "token", time.Now(), slogLogger)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.logger != slogLogger {
		t.Error("expected logger to be set to provided logger")
	}
}

func TestServer_Stop_GracefulShutdown(t *testing.T) {
	srv := NewServer(nil, "0", "", time.Now(), slog.Default())

	// Directly test Stop without starting
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Stop on unstarted server should be quick
	err := srv.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestServer_ErrChan(t *testing.T) {
	srv := NewServer(nil, "0", "", time.Now(), slog.Default())
	errCh := srv.ErrChan()

	if errCh == nil {
		t.Error("expected non-nil error channel")
	}

	// Channel should be ready to receive
	select {
	case _, ok := <-errCh:
		if ok {
			t.Log("Got value from errCh, channel is working")
		}
	default:
		t.Log("errCh is empty (expected before Start)")
	}
}

func TestServer_RoutesRegistered(t *testing.T) {
	srv := NewServer(nil, "0", "", time.Now(), slog.Default())

	// Check that routes are registered by making requests
	// Note: This tests the mux configuration indirectly
	if srv.server == nil {
		t.Fatal("expected http.Server to be initialized")
	}
	if srv.server.Handler == nil {
		t.Fatal("expected Handler to be set on http.Server")
	}
}

func TestServer_FieldsSet(t *testing.T) {
	startTime := time.Now()
	srv := NewServer(nil, "9080", "my-token", startTime, slog.Default())

	if srv.port != "9080" {
		t.Errorf("expected port 9080, got %s", srv.port)
	}
	if srv.token != "my-token" {
		t.Errorf("expected token 'my-token', got %s", srv.token)
	}
	if !srv.startTime.Equal(startTime) {
		t.Error("expected startTime to be set")
	}
	if srv.engine != nil {
		t.Error("expected nil engine")
	}
}

func TestServer_RouteConfig(t *testing.T) {
	// Test that all routes are properly configured
	srv := NewServer(nil, "0", "", time.Now(), slog.Default())

	// Create a test request router
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions", nil)
	rec := httptest.NewRecorder()

	// The router should be able to handle this
	srv.server.Handler.ServeHTTP(rec, req)

	// Should get a response (not 404 from mux, but 503 from handler since engine is nil)
	// This verifies the route is registered
	if rec.Code == http.StatusNotFound {
		t.Error("route should be registered, got 404")
	}
}

func TestServer_DeleteRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/sessions/test-id", nil)
	rec := httptest.NewRecorder()

	srv := NewServer(nil, "0", "", time.Now(), slog.Default())
	srv.server.Handler.ServeHTTP(rec, req)

	// With nil engine, should get 503 Service Unavailable
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestServer_StatsRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/stats", nil)
	rec := httptest.NewRecorder()

	srv := NewServer(nil, "0", "", time.Now(), slog.Default())
	srv.server.Handler.ServeHTTP(rec, req)

	// With nil engine, stats endpoint returns partial data (200)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestServer_ValidateRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/config/validate", nil)
	rec := httptest.NewRecorder()

	srv := NewServer(nil, "0", "", time.Now(), slog.Default())
	srv.server.Handler.ServeHTTP(rec, req)

	// Should get 400 Bad Request (invalid JSON)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestServer_HealthRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/health/detailed", nil)
	rec := httptest.NewRecorder()

	srv := NewServer(nil, "0", "", time.Now(), slog.Default())
	srv.server.Handler.ServeHTTP(rec, req)

	// Should get 200 OK (partial health)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestServer_SessionLogsRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions/nonexistent/logs", nil)
	rec := httptest.NewRecorder()

	srv := NewServer(nil, "0", "", time.Now(), slog.Default())
	srv.server.Handler.ServeHTTP(rec, req)

	// Should get 404 Not Found (no log file)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
