package session

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func captureStdout(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestRunList_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/v1/sessions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		sessions := []map[string]any{
			{
				"id":          "sess-001",
				"status":      "ready",
				"created_at":  "2026-03-21T08:00:00Z",
				"last_active": "2026-03-21T08:10:00Z",
			},
			{
				"id":          "sess-002",
				"status":      "busy",
				"created_at":  "2026-03-21T08:05:00Z",
				"last_active": "2026-03-21T08:15:00Z",
			},
		}
		resp := map[string]any{"sessions": sessions, "total": 2}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	output := captureStdout(func() {
		_ = runList(cmd, nil)
	})

	if !strings.Contains(output, "sess-001") {
		t.Errorf("expected output to contain session id 'sess-001', got: %s", output)
	}
	if !strings.Contains(output, "sess-002") {
		t.Errorf("expected output to contain session id 'sess-002', got: %s", output)
	}
}

func TestRunList_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	err := runList(cmd, nil)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestRunList_ConnectionFailure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:59999", "")
	cmd.Flags().String("admin-token", "test-token", "")

	err := runList(cmd, nil)
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestRunKill_Success(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Session sess-001 terminated"})
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	err := runKill(cmd, []string{"sess-001"})
	if err != nil {
		t.Fatalf("runKill() error = %v", err)
	}
	_ = captureStdout(func() {}) // capture output to avoid polluting test logs

	if capturedMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got %s", capturedMethod)
	}
	if capturedPath != "/admin/v1/sessions/sess-001" {
		t.Errorf("expected path '/admin/v1/sessions/sess-001', got %s", capturedPath)
	}
}

func TestRunKill_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runKill(cmd, []string{"sess-001"})
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestRunKill_ServerErrorWithMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "session not found"})
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runKill(cmd, []string{"sess-002"})
	if err == nil {
		t.Error("expected error for failed kill")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("expected error containing 'session not found', got: %v", err)
	}
}

func TestRunKill_ConnectionFailure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:59999", "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runKill(cmd, []string{"sess-001"})
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestRunLogs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"session_id":    "sess-001",
			"log_path":      "/tmp/hotplex/sessions/sess-001/session.log",
			"size_bytes":    1024,
			"last_modified": "2026-03-21T08:15:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	output := captureStdout(func() {
		err := runLogs(cmd, []string{"sess-001"})
		if err != nil {
			t.Fatalf("runLogs() error = %v", err)
		}
	})

	if !strings.Contains(output, "Session ID") {
		t.Errorf("expected output to contain 'Session ID', got: %s", output)
	}
}

func TestRunLogs_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runLogs(cmd, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for not found response")
	}
}

func TestRunLogs_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runLogs(cmd, []string{"sess-001"})
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestRunLogs_ConnectionFailure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:59999", "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runLogs(cmd, []string{"sess-001"})
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestRunLogs_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")
	cmd.SetOut(&strings.Builder{})

	err := runLogs(cmd, []string{"sess-001"})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}
