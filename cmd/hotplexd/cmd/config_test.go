package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateConfigLocally_EmptyPath(t *testing.T) {
	err := validateConfigLocally("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestValidateConfigLocally_InvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(tmpFile, []byte("invalid: [yaml"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	err := validateConfigLocally(tmpFile)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidateConfigLocally_MissingServer(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "missing.yaml")
	if err := os.WriteFile(tmpFile, []byte("engine:\n  provider: claude-code\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	err := validateConfigLocally(tmpFile)
	if err == nil {
		t.Error("expected error for missing server field")
	}
}

func TestValidateConfigLocally_MissingEngine(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "missing.yaml")
	if err := os.WriteFile(tmpFile, []byte("server:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	err := validateConfigLocally(tmpFile)
	if err == nil {
		t.Error("expected error for missing engine field")
	}
}

func TestValidateConfigLocally_Valid(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "valid.yaml")
	content := "server:\n  port: 8080\nengine:\n  provider: claude-code\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	err := validateConfigLocally(tmpFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
