package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type TestConfig struct {
	Name    string   `yaml:"name"`
	Version string   `yaml:"version"`
	Tags    []string `yaml:"tags"`
}

func TestYAMLHotReloader(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialContent := `name: test
version: "1.0.0"
tags: ["dev"]
`
	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	var cfg TestConfig
	logger := slog.Default()

	// Create hot reloader
	reloader, err := NewYAMLHotReloader(configPath, &cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create hot reloader: %v", err)
	}

	// Verify initial load
	if cfg.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", cfg.Name)
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", cfg.Version)
	}

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Failed to start hot reloader: %v", err)
	}

	// Track reloads
	reloaded := make(chan struct{})
	reloader.OnReload(func(config any) {
		if updatedCfg, ok := config.(*TestConfig); ok {
			if updatedCfg.Version == "2.0.0" {
				close(reloaded)
			}
		}
	})

	// Modify the config file
	updatedContent := `name: test
version: "2.0.0"
tags: ["prod"]
`
	if err := os.WriteFile(configPath, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// Wait for reload with timeout
	select {
	case <-reloaded:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for config reload")
	}

	// Verify the config was reloaded
	reloadedCfg := reloader.Get().(*TestConfig)
	if reloadedCfg.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", reloadedCfg.Version)
	}

	// Cleanup
	cancel()
	_ = reloader.Close()
}

func TestYAMLHotReloaderInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create truly invalid YAML - this will fail to parse
	invalidContent := "name: [test" // Array without closing bracket
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	var cfg TestConfig
	logger := slog.Default()

	// Create hot reloader - should fail on initial load
	_, err := NewYAMLHotReloader(configPath, &cfg, logger)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestYAMLHotReloaderMissingFile(t *testing.T) {
	var cfg TestConfig
	logger := slog.Default()

	// Create hot reloader with non-existent file
	_, err := NewYAMLHotReloader("/nonexistent/path/config.yaml", &cfg, logger)
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestYAMLHotReloaderConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `name: concurrent
version: "1.0.0"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	var cfg TestConfig
	logger := slog.Default()

	reloader, err := NewYAMLHotReloader(configPath, &cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create hot reloader: %v", err)
	}

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = reloader.Get()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	_ = reloader.Close()
}
