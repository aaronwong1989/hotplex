package cmd

import (
	"os"
	"testing"
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
