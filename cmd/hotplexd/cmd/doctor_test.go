package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestCheckEnvVars_AllSet(t *testing.T) {
	original := os.Getenv("HOTPLEX_PROJECTS_DIR")
	defer func() { _ = os.Setenv("HOTPLEX_PROJECTS_DIR", original) }()

	_ = os.Setenv("HOTPLEX_PROJECTS_DIR", "/projects")
	passed, detail := checkEnvVars()
	if !passed {
		t.Errorf("expected checkEnvVars to pass, got: %s", detail)
	}
}

func TestCheckEnvVars_Missing(t *testing.T) {
	original := os.Getenv("HOTPLEX_PROJECTS_DIR")
	defer func() { _ = os.Setenv("HOTPLEX_PROJECTS_DIR", original) }()

	_ = os.Unsetenv("HOTPLEX_PROJECTS_DIR")
	passed, detail := checkEnvVars()
	if passed {
		t.Error("expected checkEnvVars to fail when env var is missing")
	}
	if detail == "" {
		t.Error("expected non-empty detail for missing env var")
	}
}

func getServerPort(srv *httptest.Server) string {
	u, _ := url.Parse(srv.URL)
	return u.Port()
}

func TestCheckAdminAPIHealth_ServerRespondsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("HOTPLEX_ADMIN_PORT", getServerPort(srv))

	passed, detail := checkAdminAPIHealth()
	if !passed {
		t.Errorf("expected health check to pass, got: %s", detail)
	}
	if detail != "Admin API is healthy" {
		t.Errorf("unexpected detail: %s", detail)
	}
}

func TestCheckAdminAPIHealth_ServerReturnsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "internal error")
	}))
	defer srv.Close()

	t.Setenv("HOTPLEX_ADMIN_PORT", getServerPort(srv))

	passed, detail := checkAdminAPIHealth()
	if passed {
		t.Error("expected health check to fail for non-200 status")
	}
	if detail == "" {
		t.Error("expected non-empty detail for non-200 status")
	}
}

func TestCheckAdminAPIHealth_ServerUnreachable(t *testing.T) {
	// Use a server that refuses connections
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	t.Setenv("HOTPLEX_ADMIN_PORT", getServerPort(srv))

	passed, detail := checkAdminAPIHealth()
	if passed {
		t.Error("expected health check to fail when server is unreachable")
	}
	if detail == "" {
		t.Error("expected non-empty detail for unreachable server")
	}
}

func TestCheckAdminAPIHealth_ServerTimeout(t *testing.T) {
	// Note: Sleep duration is 3s, slightly above the 2s production timeout in checkAdminAPIHealth.
	// This ensures the context deadline is exceeded before the handler responds.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("HOTPLEX_ADMIN_PORT", getServerPort(srv))

	passed, _ := checkAdminAPIHealth()
	if passed {
		t.Error("expected health check to fail on timeout")
	}
}
