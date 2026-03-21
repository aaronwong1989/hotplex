package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

func TestDoAdminAPI_InvalidServerURL(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:99999", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats")
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestDoAdminAPI_InvalidAdminToken(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:99999", "")
	cmd.Flags().String("admin-token", "", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats")
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestDoAdminAPI_ConnectionRefused(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "http://localhost:59999", "")
	cmd.Flags().String("admin-token", "", "")

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats")
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

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats")
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

	_, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedToken != "" {
		t.Errorf("expected empty Authorization header, got '%s'", capturedToken)
	}
}
