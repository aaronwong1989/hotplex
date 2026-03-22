package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewFileMarkerStore tests constructor
func TestNewFileMarkerStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markers-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewFileMarkerStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileMarkerStore() error: %v", err)
	}
	if store == nil {
		t.Fatal("NewFileMarkerStore() returned nil")
	}
	if store.Dir() != tmpDir {
		t.Errorf("Dir() = %q, want %q", store.Dir(), tmpDir)
	}
}

// TestNewFileMarkerStore_EmptyDir tests empty directory error
func TestNewFileMarkerStore_EmptyDir(t *testing.T) {
	_, err := NewFileMarkerStore("")
	if err == nil {
		t.Error("NewFileMarkerStore('') should return error")
	}
}

// TestNewDefaultFileMarkerStore tests default constructor
func TestNewDefaultFileMarkerStore(t *testing.T) {
	store := NewDefaultFileMarkerStore()
	if store == nil {
		t.Fatal("NewDefaultFileMarkerStore() returned nil")
	}
	if store.Dir() == "" {
		t.Error("Dir() should not be empty")
	}
}

// TestFileMarkerStore_Exists_Create_Delete tests basic CRUD
func TestFileMarkerStore_Exists_Create_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markers-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileMarkerStore(tmpDir)

	// Initially doesn't exist
	if store.Exists("session-1") {
		t.Error("Exists() should return false for new session")
	}

	// Create marker
	if err := store.Create("session-1"); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Now exists
	if !store.Exists("session-1") {
		t.Error("Exists() should return true after Create()")
	}

	// Delete marker
	if err := store.Delete("session-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// No longer exists
	if store.Exists("session-1") {
		t.Error("Exists() should return false after Delete()")
	}
}

// TestFileMarkerStore_Delete_NonExistent tests delete idempotency
func TestFileMarkerStore_Delete_NonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markers-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileMarkerStore(tmpDir)

	// Delete non-existent should not error
	if err := store.Delete("nonexistent"); err != nil {
		t.Errorf("Delete(nonexistent) error: %v", err)
	}
}

// TestFileMarkerStore_InvalidSessionID tests security validation
func TestFileMarkerStore_InvalidSessionID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markers-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileMarkerStore(tmpDir)

	// Path traversal attempts are mapped to INVALID_SESSION_ID.lock
	// This prevents path traversal by mapping all invalid IDs to a single safe path
	invalidIDs := []string{
		"../../../etc/passwd",
		"../../.ssh/authorized_keys",
		"session/../../../etc/shadow",
		"",
	}

	for _, id := range invalidIDs {
		// Create maps to safe path - no error
		_ = store.Create(id)
	}

	// Valid IDs should still work
	if err := store.Create("valid-session-123"); err != nil {
		t.Fatalf("Create(valid-session) error: %v", err)
	}
	if !store.Exists("valid-session-123") {
		t.Error("Exists(valid-session) should return true")
	}
}

// TestFileMarkerStore_MarkerPathSecurity tests path traversal prevention
func TestFileMarkerStore_MarkerPathSecurity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markers-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileMarkerStore(tmpDir)

	// Create with invalid ID should create safe marker file
	_ = store.Create("../../../etc/passwd")

	// Verify only safe file exists (INVALID_SESSION_ID.lock)
	entries, _ := os.ReadDir(tmpDir)
	safeFileExists := false
	for _, entry := range entries {
		if entry.Name() == "INVALID_SESSION_ID.lock" {
			safeFileExists = true
		}
		// Should not create files outside marker dir
		if entry.Name() == "passwd" || entry.Name() == "shadow" {
			t.Errorf("Path traversal created file: %s", entry.Name())
		}
	}

	if !safeFileExists {
		t.Error("Expected INVALID_SESSION_ID.lock to be created for invalid session ID")
	}
}

// TestInMemoryMarkerStore tests the in-memory implementation
func TestInMemoryMarkerStore(t *testing.T) {
	store := NewInMemoryMarkerStore()
	if store == nil {
		t.Fatal("NewInMemoryMarkerStore() returned nil")
	}

	// Initially doesn't exist
	if store.Exists("session-1") {
		t.Error("Exists() should return false for new session")
	}

	// Create
	if err := store.Create("session-1"); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Now exists
	if !store.Exists("session-1") {
		t.Error("Exists() should return true after Create()")
	}

	// Dir returns empty
	if store.Dir() != "" {
		t.Errorf("Dir() = %q, want empty", store.Dir())
	}

	// Delete
	if err := store.Delete("session-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// No longer exists
	if store.Exists("session-1") {
		t.Error("Exists() should return false after Delete()")
	}
}

// TestInMemoryMarkerStore_Delete_NonExistent tests idempotency
func TestInMemoryMarkerStore_Delete_NonExistent(t *testing.T) {
	store := NewInMemoryMarkerStore()

	// Delete non-existent should not error
	if err := store.Delete("nonexistent"); err != nil {
		t.Errorf("Delete(nonexistent) error: %v", err)
	}
}

// TestFileMarkerStore_Concurrent tests concurrent access
func TestFileMarkerStore_Concurrent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markers-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileMarkerStore(tmpDir)

	// Concurrent creates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			sessionID := filepath.Base(filepath.Join("concurrent", string(rune('a'+n))))
			_ = store.Create(sessionID)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
