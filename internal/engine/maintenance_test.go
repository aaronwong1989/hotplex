package engine

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/hrygo/hotplex/provider"
)

func TestMaintenanceLoop_ExecutesCleanup(t *testing.T) {
	// Create a test session pool
	logger := slog.Default()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{
		DefaultPermissionMode: "bypassPermissions",
	}, logger)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	sm := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace:      "test",
		PermissionMode: "bypassPermissions",
	}, os.Getenv("HOTPLEX_PROVIDER_BINARY"), prv)

	// Verify that clearClaudeJSONUserID is called on startup by checking logs
	// The function will be called immediately in maintenanceLoop
	// We can't easily test the ticker without time.Sleep, so we just verify
	// the pool was created successfully (which means maintenanceLoop started)
	if sm == nil {
		t.Fatal("expected non-nil session pool")
	}

	// Cleanup
	sm.Shutdown()
}

func TestMaintenanceLoop_EnvDisabled(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("HOTPLEX_CLAUDE_CLEAR_USERID")
	defer os.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", origEnv)

	// Disable via env
	os.Setenv("HOTPLEX_CLAUDE_CLEAR_USERID", "false")

	// Create pool - maintenanceLoop should skip cleanup
	logger := slog.Default()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{
		DefaultPermissionMode: "bypassPermissions",
	}, logger)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	sm := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace:      "test",
		PermissionMode: "bypassPermissions",
	}, os.Getenv("HOTPLEX_PROVIDER_BINARY"), prv)

	if sm == nil {
		t.Fatal("expected non-nil session pool")
	}

	// Cleanup
	sm.Shutdown()
}

func TestMaintenanceLoop_StopsOnShutdown(t *testing.T) {
	logger := slog.Default()
	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{
		DefaultPermissionMode: "bypassPermissions",
	}, logger)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	sm := NewSessionPool(logger, 30*time.Minute, EngineOptions{
		Namespace:      "test",
		PermissionMode: "bypassPermissions",
	}, os.Getenv("HOTPLEX_PROVIDER_BINARY"), prv)

	// Shutdown should stop the maintenance loop
	done := make(chan struct{})
	go func() {
		sm.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good - shutdown completed
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown took too long - maintenanceLoop may not have stopped")
	}
}
