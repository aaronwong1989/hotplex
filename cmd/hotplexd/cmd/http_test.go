package cmd

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

func TestDoAdminAPI_InvalidServerURL(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:99999", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats", nil)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestDoAdminAPI_InvalidAdminToken(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:99999", "")
	cmd.Flags().String("admin-token", "", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats", nil)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestDoAdminAPI_ConnectionRefused(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:59999", "")
	cmd.Flags().String("admin-token", "", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats", nil)
	if err == nil {
		t.Error("expected connection refused error")
	}
}

func TestDoAdminAPI_SetsAuthHeader(t *testing.T) {
	var capturedToken string
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("Authorization")
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedToken != "Bearer test-token" {
		t.Errorf("expected 'Bearer test-token', got '%s'", capturedToken)
	}
	if capturedPath != "/admin/v1/stats" {
		t.Errorf("expected path '/admin/v1/stats', got '%s'", capturedPath)
	}
}

func TestDoAdminAPI_NoToken(t *testing.T) {
	var capturedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedToken != "" {
		t.Errorf("expected empty Authorization header, got '%s'", capturedToken)
	}
}

func TestRunStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/v1/stats" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_sessions":   5,
			"active_sessions":  2,
			"stopped_sessions": 3,
			"uptime":           "2h30m",
			"memory_usage_mb":  64.5,
			"cpu_usage_percent": 12.3,
		})
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	err := runStatus(cmd, nil)

	_ = os.Stdout.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	outStr := string(out)

	if err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}
	if !strings.Contains(outStr, "Total Sessions") {
		t.Errorf("expected output to contain 'Total Sessions', got: %s", outStr)
	}
}

func TestRunStatus_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", server.URL, "")
	cmd.Flags().String("admin-token", "test-token", "")

	err := runStatus(cmd, nil)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestRunStatus_ConnectionFailure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:59999", "")
	cmd.Flags().String("admin-token", "test-token", "")

	err := runStatus(cmd, nil)
	if err == nil {
		t.Error("expected error for connection refused")
	}
}
