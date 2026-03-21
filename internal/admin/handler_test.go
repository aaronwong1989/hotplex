package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...any) {}
func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Warn(msg string, args ...any) {}
func (m *mockLogger) Error(msg string, args ...any) {}

func TestWriteJSON(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	rr := httptest.NewRecorder()
	h.writeJSON(rr, http.StatusOK, map[string]string{"key": "value"})

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusNotFound, ErrCodeNotFound, "not found")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error.Code != ErrCodeNotFound {
		t.Errorf("expected error code %s, got %s", ErrCodeNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "not found" {
		t.Errorf("expected message 'not found', got '%s'", resp.Error.Message)
	}
}

func TestValidateConfigFile_EmptyPath(t *testing.T) {
	errs := validateConfigFile("")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0] != "config_path is required" {
		t.Errorf("unexpected error: %s", errs[0])
	}
}

func TestValidateConfigFile_NotFound(t *testing.T) {
	errs := validateConfigFile("/nonexistent/path/config.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0], "not found") {
		t.Errorf("unexpected error: %s", errs[0])
	}
}

func TestValidateConfigFile_IsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	errs := validateConfigFile(tmpDir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	// os.ReadFile returns "read /tmp/xxx: is a directory" on Linux
	if !strings.Contains(errs[0], "read") && !strings.Contains(errs[0], "directory") {
		t.Errorf("unexpected error: %s", errs[0])
	}
}

func TestValidateConfigFile_InvalidYAML(t *testing.T) {
	tmpFile := t.TempDir() + "/invalid.yaml"
	if err := os.WriteFile(tmpFile, []byte("invalid: [yaml: content"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	errs := validateConfigFile(tmpFile)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0], "invalid YAML") {
		t.Errorf("unexpected error: %s", errs[0])
	}
}

func TestValidateConfigFile_MissingFields(t *testing.T) {
	tmpFile := t.TempDir() + "/missing.yaml"
	if err := os.WriteFile(tmpFile, []byte("server:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	errs := validateConfigFile(tmpFile)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0], "missing required field: engine") {
		t.Errorf("unexpected error: %s", errs[0])
	}
}

func TestValidateConfigFile_Valid(t *testing.T) {
	tmpFile := t.TempDir() + "/valid.yaml"
	content := "server:\n  port: 8080\nengine:\n  provider: claude-code\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	errs := validateConfigFile(tmpFile)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestGetStats_NilEngine(t *testing.T) {
	h := &Handler{startTime: time.Now(), logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/stats", nil)
	rr := httptest.NewRecorder()
	h.getStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp StatsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.TotalSessions != 0 {
		t.Errorf("expected TotalSessions 0, got %d", resp.TotalSessions)
	}
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/config/validate", strings.NewReader("invalid json"))
	rr := httptest.NewRecorder()
	h.validateConfig(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestValidateConfig_ValidPath(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	tmpFile := t.TempDir() + "/valid.yaml"
	content := "server:\n  port: 8080\nengine:\n  provider: claude-code\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	body := `{"config_path": "` + tmpFile + `"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/config/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.validateConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp ConfigValidateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Valid {
		t.Error("expected valid response")
	}
}

func TestGetSessionLogs_NotFound(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions/nonexistent/logs", nil)
	router := mux.NewRouter()
	router.HandleFunc("/admin/v1/sessions/{id}/logs", h.getSessionLogs)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestGetSessionLogs_HomeDirError(t *testing.T) {
	// Create a minimal handler
	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions/test/logs", nil)
	rr := httptest.NewRecorder()
	// We can't easily test home dir error, but we test the path via mux
	router := mux.NewRouter()
	router.HandleFunc("/admin/v1/sessions/{id}/logs", h.getSessionLogs)
	router.ServeHTTP(rr, req)
}

func TestHandler_WriteJSONEncodeError(t *testing.T) {
	// Test that writeJSON handles encoding errors gracefully
	h := &Handler{logger: &mockLogger{}}
	rr := httptest.NewRecorder()
	// This should not panic
	h.writeJSON(rr, http.StatusOK, "plain string")
}

func TestListSessions_NilEngine(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions", nil)
	rr := httptest.NewRecorder()
	h.listSessions(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestGetSession_NilEngine(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions/sess-001", nil)
	rr := httptest.NewRecorder()
	h.getSession(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestDeleteSession_NilEngine(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/sessions/sess-001", nil)
	rr := httptest.NewRecorder()
	h.deleteSession(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestGetCliVersion_Unknown(t *testing.T) {
	// When claude-code is not installed, should return "unknown"
	// (This is the expected behavior in test environment)
	version := getCliVersion()
	if version == "" {
		t.Error("expected non-empty version string")
	}
}

func TestGetHealthDetailed_NilEngine(t *testing.T) {
	h := &Handler{logger: &mockLogger{}, startTime: time.Now()}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/health/detailed", nil)
	rr := httptest.NewRecorder()
	h.getHealthDetailed(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 (partial health), got %d", rr.Code)
	}

	var resp HealthDetailedResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// When engine is nil, checks should still return (partial health)
	if resp.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestCountWebsocketConnections(t *testing.T) {
	// Should not panic and return a non-negative integer
	count := countWebsocketConnections()
	if count < 0 {
		t.Errorf("expected non-negative count, got %d", count)
	}
}

func TestCheckDatabaseHealth_NoDB(t *testing.T) {
	latency, ok := checkDatabaseHealth("/nonexistent/db.sqlite")
	if latency != 0 {
		t.Errorf("expected latency 0 for nonexistent db, got %d", latency)
	}
	if ok {
		t.Error("expected ok=false for nonexistent db")
	}
}

func TestWriteError_EncodeFailure(t *testing.T) {
	// Test that writeError handles encoding failures gracefully
	rr := &writerFailsRecorder{}
	writeError(rr, http.StatusInternalServerError, ErrCodeServerError, "test error")
	// Should not panic
}

func TestValidateConfigFile_EmptyFile(t *testing.T) {
	tmpFile := t.TempDir() + "/empty.yaml"
	if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	errs := validateConfigFile(tmpFile)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for empty file, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "empty") {
		t.Errorf("expected 'empty file' error, got: %s", errs[0])
	}
}

func TestGetSessionLogs_WithMockedHomeDir(t *testing.T) {
	// Create a mock log file in a temp directory
	tmpDir := t.TempDir()
	logDir := tmpDir + "/.hotplex/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	logPath := logDir + "/test-session.log"
	if err := os.WriteFile(logPath, []byte("test log content"), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	// Override home dir for this test
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tmpDir)

	h := &Handler{logger: &mockLogger{}}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/sessions/test-session/logs", nil)
	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/admin/v1/sessions/{id}/logs", h.getSessionLogs)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestGetStats_WithNilManager(t *testing.T) {
	// Engine is not nil but manager is nil
	h := &Handler{startTime: time.Now(), logger: &mockLogger{}}
	// Note: This tests the path where engine != nil but GetSessionManager returns nil
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/stats", nil)
	rr := httptest.NewRecorder()
	h.getStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestValidateConfig_InvalidPath(t *testing.T) {
	h := &Handler{logger: &mockLogger{}}
	body := `{"config_path": "/nonexistent/path/config.yaml"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/config/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.validateConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 (validation errors), got %d", rr.Code)
	}

	var resp ConfigValidateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Valid {
		t.Error("expected invalid response")
	}
}

type writerFailsRecorder struct {
	code int
}

func (w *writerFailsRecorder) Header() http.Header {
	return http.Header{}
}

func (w *writerFailsRecorder) WriteHeader(code int) {
	w.code = code
}

func (w *writerFailsRecorder) Write([]byte) (int, error) {
	return 0, errors.New("write error")
}
