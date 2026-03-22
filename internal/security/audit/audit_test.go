package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hrygo/hotplex/internal/security"
)

// sampleEvent creates a test audit event
func sampleEvent(level security.DangerLevel, action security.AuditAction) *security.AuditEvent {
	return &security.AuditEvent{
		Timestamp: time.Now(),
		Input:     "test input",
		Operation: "test_op",
		Reason:    "test reason",
		Level:     level,
		Category:  "test",
		Action:    action,
		UserID:    "user-1",
		SessionID: "session-1",
		Source:    "test",
	}
}

// TestMemoryAuditStore_New tests constructor
func TestMemoryAuditStore_New(t *testing.T) {
	store := NewMemoryAuditStore(100)
	if store == nil {
		t.Fatal("NewMemoryAuditStore() returned nil")
	}

	// Zero capacity defaults to 1000
	store2 := NewMemoryAuditStore(0)
	if store2 == nil {
		t.Fatal("NewMemoryAuditStore(0) returned nil")
	}

	// Negative capacity defaults to 1000
	store3 := NewMemoryAuditStore(-5)
	if store3 == nil {
		t.Fatal("NewMemoryAuditStore(-5) returned nil")
	}
}

// TestMemoryAuditStore_Save_Query tests save and query
func TestMemoryAuditStore_Save_Query(t *testing.T) {
	store := NewMemoryAuditStore(100)
	ctx := context.Background()

	// Save events
	event := sampleEvent(security.DangerLevelModerate, security.AuditActionApproved)
	if err := store.Save(ctx, event); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Query without filter
	results, err := store.Query(ctx, security.AuditFilter{})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query() returned %d results, want 1", len(results))
	}
}

// TestMemoryAuditStore_Save_NilEvent tests nil event error
func TestMemoryAuditStore_Save_NilEvent(t *testing.T) {
	store := NewMemoryAuditStore(100)
	ctx := context.Background()

	err := store.Save(ctx, nil)
	if err == nil {
		t.Error("Save(nil) should return error")
	}
}

// TestMemoryAuditStore_Query_Filter tests filtering
func TestMemoryAuditStore_Query_Filter(t *testing.T) {
	store := NewMemoryAuditStore(100)
	ctx := context.Background()

	// Save events with different levels
	store.Save(ctx, sampleEvent(security.DangerLevelModerate, security.AuditActionApproved))
	store.Save(ctx, sampleEvent(security.DangerLevelCritical, security.AuditActionBlocked))
	store.Save(ctx, sampleEvent(security.DangerLevelHigh, security.AuditActionBlocked))

	// Filter by action
	results, _ := store.Query(ctx, security.AuditFilter{
		Actions: []security.AuditAction{security.AuditActionBlocked},
	})
	if len(results) != 2 {
		t.Errorf("Query(actions=[blocked]) returned %d, want 2", len(results))
	}

	// Filter by user
	results2, _ := store.Query(ctx, security.AuditFilter{
		UserID: "user-1",
	})
	if len(results2) != 3 {
		t.Errorf("Query(user=user-1) returned %d, want 3", len(results2))
	}

	// Filter by session
	results3, _ := store.Query(ctx, security.AuditFilter{
		SessionID: "nonexistent",
	})
	if len(results3) != 0 {
		t.Errorf("Query(session=nonexistent) returned %d, want 0", len(results3))
	}

	// Filter with limit
	results4, _ := store.Query(ctx, security.AuditFilter{
		Limit: 1,
	})
	if len(results4) != 1 {
		t.Errorf("Query(limit=1) returned %d, want 1", len(results4))
	}
}

// TestMemoryAuditStore_Query_TimeFilter tests time-based filtering
func TestMemoryAuditStore_Query_TimeFilter(t *testing.T) {
	store := NewMemoryAuditStore(100)
	ctx := context.Background()

	// Save old event
	oldEvent := sampleEvent(security.DangerLevelModerate, security.AuditActionApproved)
	oldEvent.Timestamp = time.Now().Add(-1 * time.Hour)
	store.Save(ctx, oldEvent)

	// Query with start time filter
	results, _ := store.Query(ctx, security.AuditFilter{
		StartTime: time.Now().Add(-30 * time.Minute),
	})
	if len(results) != 0 {
		t.Errorf("Query(start_time=30m ago) returned %d, want 0", len(results))
	}
}

// TestMemoryAuditStore_Stats tests statistics
func TestMemoryAuditStore_Stats(t *testing.T) {
	store := NewMemoryAuditStore(100)
	ctx := context.Background()

	// Save events
	store.Save(ctx, sampleEvent(security.DangerLevelModerate, security.AuditActionApproved))
	store.Save(ctx, sampleEvent(security.DangerLevelCritical, security.AuditActionBlocked))
	store.Save(ctx, sampleEvent(security.DangerLevelHigh, security.AuditActionBlocked))

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}

	if stats.TotalApproved != 1 {
		t.Errorf("TotalApproved = %d, want 1", stats.TotalApproved)
	}
	if stats.TotalBlocked != 2 {
		t.Errorf("TotalBlocked = %d, want 2", stats.TotalBlocked)
	}
}

// TestMemoryAuditStore_Close tests close (no-op)
func TestMemoryAuditStore_Close(t *testing.T) {
	store := NewMemoryAuditStore(100)
	if err := store.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

// TestFileAuditStore_New tests constructor
func TestFileAuditStore_New(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename := filepath.Join(tmpDir, "audit.jsonl")
	store, err := NewFileAuditStore(filename)
	if err != nil {
		t.Fatalf("NewFileAuditStore() error: %v", err)
	}
	if store == nil {
		t.Fatal("NewFileAuditStore() returned nil")
	}
	defer func() { _ = store.Close() }()
}

// TestFileAuditStore_New_EmptyFilename tests empty filename error
func TestFileAuditStore_New_EmptyFilename(t *testing.T) {
	_, err := NewFileAuditStore("")
	if err == nil {
		t.Error("NewFileAuditStore('') should return error")
	}
}

// TestFileAuditStore_New_NonexistentDir tests directory creation
func TestFileAuditStore_New_NonexistentDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create nested path that doesn't exist
	filename := filepath.Join(tmpDir, "nested", "path", "audit.jsonl")
	store, err := NewFileAuditStore(filename)
	if err != nil {
		t.Fatalf("NewFileAuditStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()
}

// TestFileAuditStore_Save_Query tests save and query
func TestFileAuditStore_Save_Query(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	filename := filepath.Join(tmpDir, "audit.jsonl")
	store, err := NewFileAuditStore(filename)
	if err != nil {
		t.Fatalf("NewFileAuditStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Save event
	event := sampleEvent(security.DangerLevelModerate, security.AuditActionApproved)
	if err := store.Save(ctx, event); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Query
	results, err := store.Query(ctx, security.AuditFilter{})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query() returned %d results, want 1", len(results))
	}
}

// TestFileAuditStore_Save_NilEvent tests nil event error
func TestFileAuditStore_Save_NilEvent(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "audit.jsonl"))
	defer func() { _ = store.Close() }()

	err := store.Save(context.Background(), nil)
	if err == nil {
		t.Error("Save(nil) should return error")
	}
}

// TestFileAuditStore_Query_NonexistentFile tests query on empty file
func TestFileAuditStore_Query_NonexistentFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "nonexistent.jsonl"))
	defer func() { _ = store.Close() }()

	results, err := store.Query(context.Background(), security.AuditFilter{})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Query() returned %d, want 0", len(results))
	}
}

// TestFileAuditStore_Query_Filter tests filtering
func TestFileAuditStore_Query_Filter(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "audit.jsonl"))
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Save events
	store.Save(ctx, sampleEvent(security.DangerLevelModerate, security.AuditActionApproved))
	store.Save(ctx, sampleEvent(security.DangerLevelCritical, security.AuditActionBlocked))

	// Filter by action
	results, _ := store.Query(ctx, security.AuditFilter{
		Actions: []security.AuditAction{security.AuditActionBlocked},
	})
	if len(results) != 1 {
		t.Errorf("Query(actions=[blocked]) returned %d, want 1", len(results))
	}

	// Filter with limit
	results2, _ := store.Query(ctx, security.AuditFilter{
		Limit: 1,
	})
	if len(results2) != 1 {
		t.Errorf("Query(limit=1) returned %d, want 1", len(results2))
	}
}

// TestFileAuditStore_Stats tests statistics
func TestFileAuditStore_Stats(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "audit.jsonl"))
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Save events
	store.Save(ctx, sampleEvent(security.DangerLevelModerate, security.AuditActionApproved))
	store.Save(ctx, sampleEvent(security.DangerLevelCritical, security.AuditActionBlocked))

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}

	if stats.TotalApproved != 1 {
		t.Errorf("TotalApproved = %d, want 1", stats.TotalApproved)
	}
	if stats.TotalBlocked != 1 {
		t.Errorf("TotalBlocked = %d, want 1", stats.TotalBlocked)
	}
}

// TestFileAuditStore_Stats_NonexistentFile tests stats on nonexistent file
func TestFileAuditStore_Stats_NonexistentFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "nonexistent.jsonl"))
	defer func() { _ = store.Close() }()

	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}
	if stats.TotalBlocked != 0 {
		t.Errorf("TotalBlocked = %d, want 0", stats.TotalBlocked)
	}
}

// TestFileAuditStore_Close tests close
func TestFileAuditStore_Close(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "audit.jsonl"))

	if err := store.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

// TestFileAuditStore_AutoTimestamp tests automatic timestamp
func TestFileAuditStore_AutoTimestamp(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "audit-test")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, _ := NewFileAuditStore(filepath.Join(tmpDir, "audit.jsonl"))
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Save event without timestamp
	event := &security.AuditEvent{
		Input:     "test",
		Operation: "op",
		Level:     security.DangerLevelModerate,
		Action:    security.AuditActionApproved,
	}

	if err := store.Save(ctx, event); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify timestamp was set
	results, _ := store.Query(ctx, security.AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("Query() returned %d, want 1", len(results))
	}
	if results[0].Timestamp.IsZero() {
		t.Error("Timestamp should be auto-set")
	}
	if results[0].ID == "" {
		t.Error("ID should be auto-set")
	}
}
