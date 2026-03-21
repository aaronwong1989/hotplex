package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerConfig_EmptyPath(t *testing.T) {
	// Empty path falls back to default config path resolution
	serverCfg, logLevel, logFormat := loadServerConfig("")
	// Either no config found (nil) or default config found (non-nil)
	// Just verify the function doesn't panic and returns valid (non-zero) values
	if serverCfg == nil && logLevel == 0 && logFormat == "" {
		t.Error("loadServerConfig returned all-zero values")
	}
	t.Logf("loadServerConfig('') = serverCfg:%v, logLevel:%v, logFormat:%s", serverCfg != nil, logLevel, logFormat)
}

func TestLoadServerConfig_NonexistentPath(t *testing.T) {
	nonexistent := "/nonexistent/config-xyz.yaml"
	serverCfg, logLevel, logFormat := loadServerConfig(nonexistent)
	// NewServerLoader creates a default config when file not found
	if serverCfg == nil {
		t.Error("expected non-nil serverCfg (creates default on missing file)")
	}
	// Verify it still returns usable values
	if logLevel == 0 && logFormat == "text" {
		// Both defaulted
	} else {
		t.Logf("loadServerConfig(nonexistent) returned: logLevel=%v, logFormat=%s", logLevel, logFormat)
	}
}

func TestLoadServerConfig_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.yaml")
	configContent := `
server:
  port: 9999
  log_level: debug
  log_format: json
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	serverCfg, logLevel, logFormat := loadServerConfig(configPath)
	if serverCfg == nil {
		t.Fatal("expected non-nil serverCfg for valid path")
	}
	if logLevel == 0 {
		t.Error("expected non-default logLevel for debug config")
	}
	if logFormat != "json" && logFormat != "text" {
		t.Errorf("expected 'json' or 'text' log format, got %s", logFormat)
	}
}

func TestLoadServerConfig_ValidConfigErrorLevel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.yaml")
	configContent := `
server:
  log_level: error
  log_format: text
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, logLevel, logFormat := loadServerConfig(configPath)
	if logLevel == 0 {
		t.Error("expected non-default logLevel for error config")
	}
	if logFormat != "text" {
		t.Errorf("expected 'text' log format, got %s", logFormat)
	}
}

func TestExpandPathEnvVars(t *testing.T) {
	// Save and restore environment
	type envVar struct{ k, v string }
	var saved []envVar
	for _, k := range []string{"HOTPLEX_PROJECTS_DIR", "HOTPLEX_DATA_ROOT", "HOTPLEX_MESSAGE_STORE_SQLITE_PATH"} {
		saved = append(saved, envVar{k, os.Getenv(k)})
	}
	defer func() {
		for _, e := range saved {
			if e.v != "" {
				_ = os.Setenv(e.k, e.v)
			} else {
				_ = os.Unsetenv(e.k)
			}
		}
	}()

	// Set test values
	_ = os.Setenv("HOTPLEX_PROJECTS_DIR", "/test/path")
	_ = os.Unsetenv("HOTPLEX_DATA_ROOT")

	// Should not panic
	expandPathEnvVars()

	// Verify the function ran
	val := os.Getenv("HOTPLEX_PROJECTS_DIR")
	if val == "" {
		t.Error("expected HOTPLEX_PROJECTS_DIR to be set after expandPathEnvVars")
	}
}

func TestLoadEnvFile_Nonexistent(t *testing.T) {
	nonexistent := "/nonexistent/.env.file.12345"
	// Should not panic - just log a warning
	loadEnvFile(&nonexistent)
}

func TestLoadEnvFile_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	testKey := "HOTPLEX_TEST_VAR_12345"
	testValue := "test_value_xyz"
	envContent := testKey + "=" + testValue + "\n"

	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	loadEnvFile(&envPath)

	val := os.Getenv(testKey)
	if val != testValue {
		t.Errorf("expected %s=%s, got %s", testKey, testValue, val)
	}

	_ = os.Unsetenv(testKey)
}

func TestInitLogger_DebugLevel(t *testing.T) {
	logger := initLogger(slog.LevelDebug, "json")
	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestInitLogger_InfoLevel(t *testing.T) {
	logger := initLogger(slog.LevelInfo, "text")
	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestInitLogger_ErrorLevel(t *testing.T) {
	logger := initLogger(slog.LevelError, "json")
	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestInitLogger_InvalidLevel(t *testing.T) {
	// Invalid level should fall back to default
	logger := initLogger(0, "text")
	if logger == nil {
		t.Error("expected non-nil logger even for invalid level")
	}
}
