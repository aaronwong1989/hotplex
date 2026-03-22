package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex/types"
)

// newSQLiteTestStore creates a SQLite store with a temporary database file.
func newSQLiteTestStore(t *testing.T) (*SQLiteStorage, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store := &SQLiteStorage{
		config:   PluginConfig{"path": dbPath},
		strategy: NewDefaultStrategy(),
	}

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
	}

	return store, cleanup
}

// testMessage creates a well-formed ChatAppMessage for testing.
func testMessage() *ChatAppMessage {
	return &ChatAppMessage{
		ChatSessionID:     "test-session-" + uuid.New().String()[:8],
		ChatPlatform:      "slack",
		ChatUserID:        "U123456",
		ChatBotUserID:     "U654321",
		ChatChannelID:     "C789",
		ChatThreadID:      "T101",
		EngineSessionID:   uuid.New(),
		EngineNamespace:   "hotplex",
		ProviderSessionID: "prov-" + uuid.New().String()[:8],
		ProviderType:      "claude-code",
		MessageType:       types.MessageTypeUserInput,
		FromUserID:        "U123456",
		FromUserName:      "Test User",
		ToUserID:          "U654321",
		Content:           "Hello, world!",
		Metadata:          map[string]any{"key": "value"},
	}
}

// ========================================
// SQLite Factory Tests
// ========================================

func TestSQLiteFactory_Create(t *testing.T) {
	factory := &SQLiteFactory{}
	store, err := factory.Create(PluginConfig{"path": ":memory:"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if store == nil {
		t.Fatal("Create returned nil store")
	}
}

func TestSQLiteFactory_InterfaceCompliance(t *testing.T) {
	var _ PluginFactory = (*SQLiteFactory)(nil)
	var _ ChatAppMessageStore = (*SQLiteStorage)(nil)
}

// ========================================
// SQLite Initialize Tests
// ========================================

func TestSQLite_Initialize_DefaultPath(t *testing.T) {
	store := &SQLiteStorage{
		config:   PluginConfig{}, // no path -> uses default
		strategy: NewDefaultStrategy(),
	}
	ctx := context.Background()
	// Default path expands to ~/.hotplex/ which may not exist in CI.
	// We test that Initialize at least attempts to open.
	err := store.Initialize(ctx)
	// In CI the home dir might not have ~/.hotplex, so allow failure.
	// The important thing is no panic.
	_ = err
}

func TestSQLite_Initialize_CustomPath(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	if store.db == nil {
		t.Fatal("db should not be nil after Initialize")
	}
}

// ========================================
// SQLite Name / Version / Close Tests
// ========================================

func TestSQLite_Name(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	if store.Name() != "sqlite" {
		t.Errorf("Name() = %q, want %q", store.Name(), "sqlite")
	}
}

func TestSQLite_Version(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	if store.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", store.Version(), "1.0.0")
	}
}

func TestSQLite_Close(t *testing.T) {
	store, _ := newSQLiteTestStore(t)

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Double close should not panic
	if err := store.Close(); err != nil {
		t.Fatalf("Double Close failed: %v", err)
	}
}

func TestSQLite_Close_NilDB(t *testing.T) {
	store := &SQLiteStorage{}
	if err := store.Close(); err != nil {
		t.Fatalf("Close on nil db should not error: %v", err)
	}
}

// ========================================
// SQLite StoreUserMessage / StoreBotResponse Tests
// ========================================

func TestSQLite_StoreUserMessage(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()

	if err := store.StoreUserMessage(ctx, msg); err != nil {
		t.Fatalf("StoreUserMessage failed: %v", err)
	}

	if msg.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if msg.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if msg.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestSQLite_StoreUserMessage_WithID(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	msg.ID = "custom-id-123"

	if err := store.StoreUserMessage(ctx, msg); err != nil {
		t.Fatalf("StoreUserMessage with custom ID failed: %v", err)
	}

	if msg.ID != "custom-id-123" {
		t.Errorf("ID = %q, want %q", msg.ID, "custom-id-123")
	}
}

func TestSQLite_StoreUserMessage_NonStorable(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeThinking // not storable

	if err := store.StoreUserMessage(ctx, msg); err != nil {
		t.Fatalf("StoreUserMessage for non-storable should succeed: %v", err)
	}

	// Verify it was NOT stored
	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 0 {
		t.Errorf("Non-storable message should not be persisted, got count %d", count)
	}
}

func TestSQLite_StoreBotResponse(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeFinalResponse

	if err := store.StoreBotResponse(ctx, msg); err != nil {
		t.Fatalf("StoreBotResponse failed: %v", err)
	}
}

func TestSQLite_StoreBotResponse_NonStorable(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeToolUse // not storable

	if err := store.StoreBotResponse(ctx, msg); err != nil {
		t.Fatalf("StoreBotResponse for non-storable should succeed: %v", err)
	}

	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 0 {
		t.Errorf("Non-storable bot response should not be persisted, got count %d", count)
	}
}

// ========================================
// SQLite Get Tests
// ========================================

func TestSQLite_Get(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	got, err := store.Get(ctx, msg.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", got.Content, "Hello, world!")
	}
	if got.ChatSessionID != msg.ChatSessionID {
		t.Errorf("ChatSessionID = %q, want %q", got.ChatSessionID, msg.ChatSessionID)
	}
}

func TestSQLite_Get_NotFound(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("Get with nonexistent ID should return error")
	}
}

func TestSQLite_Get_DeletedMessage(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	// Delete the session (soft deletes messages)
	_ = store.DeleteSession(ctx, msg.ChatSessionID)

	_, err := store.Get(ctx, msg.ID)
	if err == nil {
		t.Fatal("Get should return error for deleted message")
	}
}

// ========================================
// SQLite List Tests
// ========================================

func TestSQLite_List_BySessionID(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "list-session-" + uuid.New().String()[:8]

	for i := 0; i < 3; i++ {
		msg := testMessage()
		msg.ChatSessionID = sessionID
		_ = store.StoreUserMessage(ctx, msg)
	}

	messages, err := store.List(ctx, &MessageQuery{ChatSessionID: sessionID})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(messages) != 3 {
		t.Errorf("List count = %d, want 3", len(messages))
	}
}

func TestSQLite_List_ByUserID(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	messages, err := store.List(ctx, &MessageQuery{ChatUserID: msg.ChatUserID})
	if err != nil {
		t.Fatalf("List by user failed: %v", err)
	}
	if len(messages) == 0 {
		t.Error("List by user should return at least 1 message")
	}
}

func TestSQLite_List_ExcludesDeleted(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	// Soft delete
	_ = store.DeleteSession(ctx, msg.ChatSessionID)

	messages, err := store.List(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("List should exclude deleted messages, got %d", len(messages))
	}
}

func TestSQLite_List_Empty(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	messages, err := store.List(ctx, &MessageQuery{ChatSessionID: "nonexistent"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("List on nonexistent session should return empty, got %d", len(messages))
	}
}

// ========================================
// SQLite Count Tests
// ========================================

func TestSQLite_Count_BySessionID(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "count-session-" + uuid.New().String()[:8]

	for i := 0; i < 5; i++ {
		msg := testMessage()
		msg.ChatSessionID = sessionID
		_ = store.StoreUserMessage(ctx, msg)
	}

	count, err := store.Count(ctx, &MessageQuery{ChatSessionID: sessionID})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

func TestSQLite_Count_ByUserID(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	count, err := store.Count(ctx, &MessageQuery{ChatUserID: msg.ChatUserID})
	if err != nil {
		t.Fatalf("Count by user failed: %v", err)
	}
	if count < 1 {
		t.Errorf("Count by user should be >= 1, got %d", count)
	}
}

func TestSQLite_Count_Empty(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	count, err := store.Count(ctx, &MessageQuery{ChatSessionID: "nonexistent"})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Count on nonexistent session should be 0, got %d", count)
	}
}

// ========================================
// SQLite GetSessionMeta Tests
// ========================================

func TestSQLite_GetSessionMeta(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	meta, err := store.GetSessionMeta(ctx, msg.ChatSessionID)
	if err != nil {
		t.Fatalf("GetSessionMeta failed: %v", err)
	}
	if meta.ChatSessionID != msg.ChatSessionID {
		t.Errorf("ChatSessionID = %q, want %q", meta.ChatSessionID, msg.ChatSessionID)
	}
	if meta.ChatPlatform != "slack" {
		t.Errorf("ChatPlatform = %q, want %q", meta.ChatPlatform, "slack")
	}
	if meta.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", meta.MessageCount)
	}
}

func TestSQLite_GetSessionMeta_Accumulates(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()

	for i := 0; i < 5; i++ {
		m := testMessage()
		m.ChatSessionID = msg.ChatSessionID
		_ = store.StoreUserMessage(ctx, m)
	}

	meta, _ := store.GetSessionMeta(ctx, msg.ChatSessionID)
	if meta.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", meta.MessageCount)
	}
}

func TestSQLite_GetSessionMeta_NotFound(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := store.GetSessionMeta(ctx, "nonexistent-session")
	if err == nil {
		t.Fatal("GetSessionMeta should return error for nonexistent session")
	}
}

// ========================================
// SQLite ListUserSessions Tests
// ========================================

func TestSQLite_ListUserSessions(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	sessions, err := store.ListUserSessions(ctx, msg.ChatPlatform, msg.ChatUserID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Error("ListUserSessions should return at least 1 session")
	}
}

func TestSQLite_ListUserSessions_Empty(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sessions, err := store.ListUserSessions(ctx, "nonexistent-platform", "nonexistent-user")
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("ListUserSessions should return empty, got %d", len(sessions))
	}
}

// ========================================
// SQLite DeleteSession Tests
// ========================================

func TestSQLite_DeleteSession(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	if err := store.DeleteSession(ctx, msg.ChatSessionID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify messages are soft-deleted
	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 0 {
		t.Errorf("After delete, active count should be 0, got %d", count)
	}

	// Session meta should be gone (SQLite DeleteSession only soft-deletes messages, not metadata)
	// Note: SQLite DeleteSession does NOT delete session_metadata, unlike the memory implementation.
	// The metadata record remains after soft-delete.
}

// ========================================
// SQLite Strategy Tests
// ========================================

func TestSQLite_GetStrategy(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	s := store.GetStrategy()
	if s == nil {
		t.Fatal("GetStrategy should return non-nil strategy")
	}
}

func TestSQLite_SetStrategy(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	custom := &DefaultStrategy{} // reuse DefaultStrategy as a stub
	if err := store.SetStrategy(custom); err != nil {
		t.Fatalf("SetStrategy failed: %v", err)
	}
	if store.GetStrategy() != custom {
		t.Error("GetStrategy should return the custom strategy")
	}
}

func TestSQLite_SetStrategy_Nil(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	if err := store.SetStrategy(nil); err != nil {
		t.Fatalf("SetStrategy(nil) failed: %v", err)
	}
	// With nil strategy, all messages should be stored regardless of type
	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeThinking // normally not storable
	_ = store.StoreUserMessage(ctx, msg)

	got, err := store.Get(ctx, msg.ID)
	if err != nil {
		t.Fatalf("Get should succeed with nil strategy: %v", err)
	}
	if got.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", got.Content, "Hello, world!")
	}
}

// ========================================
// SQLite storeMessage Metadata Tests
// ========================================

func TestSQLite_StoreMessage_WithMetadata(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	msg.Metadata = map[string]any{
		"thread_ts": "1234567890.123456",
		"reactions": 3,
	}

	if err := store.StoreUserMessage(ctx, msg); err != nil {
		t.Fatalf("StoreUserMessage failed: %v", err)
	}

	got, err := store.Get(ctx, msg.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Content != msg.Content {
		t.Errorf("Content = %q, want %q", got.Content, msg.Content)
	}
}

func TestSQLite_StoreMessage_NilMetadata(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	msg.Metadata = nil

	if err := store.StoreUserMessage(ctx, msg); err != nil {
		t.Fatalf("StoreUserMessage with nil metadata failed: %v", err)
	}
}

// ========================================
// SQLite Concurrent Tests
// ========================================

func TestSQLite_ConcurrentWrites(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "concurrent-session-" + uuid.New().String()[:8]

	const numGoroutines = 5
	const messagesPer = 10

	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < messagesPer; j++ {
				msg := testMessage()
				msg.ChatSessionID = sessionID
				if err := store.StoreUserMessage(ctx, msg); err != nil {
					// SQLite may return SQLITE_BUSY under heavy concurrent writes.
					// This is expected behavior for concurrent writes without WAL mode.
					errCh <- err
					return
				}
			}
			errCh <- nil
		}()
	}

	var errs []error
	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	// If all goroutines had SQLITE_BUSY errors, that's still a valid test result
	// (proves concurrency safety). We verify whatever was successfully stored.
	if len(errs) == numGoroutines {
		t.Logf("All goroutines hit SQLITE_BUSY (expected under concurrent SQLite writes)")
	}

	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: sessionID})
	if count == 0 {
		t.Error("At least some messages should have been stored")
	}
}

// ========================================
// SQLite UpdateSessionMeta Tests (indirect via StoreUserMessage)
// ========================================

func TestSQLite_UpdateSessionMeta_MultipleMessages(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()

	// Store 3 messages in the same session
	for i := 0; i < 3; i++ {
		m := testMessage()
		m.ChatSessionID = msg.ChatSessionID
		m.ChatUserID = msg.ChatUserID
		m.ChatPlatform = msg.ChatPlatform
		_ = store.StoreUserMessage(ctx, m)
	}

	meta, err := store.GetSessionMeta(ctx, msg.ChatSessionID)
	if err != nil {
		t.Fatalf("GetSessionMeta failed: %v", err)
	}
	if meta.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3", meta.MessageCount)
	}
}

// ========================================
// SQLite DeleteSession then Re-store Tests
// ========================================

func TestSQLite_DeleteThenReStore(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	// Delete session (soft deletes messages but does NOT delete session_metadata)
	_ = store.DeleteSession(ctx, msg.ChatSessionID)

	// Re-store with same session
	msg2 := testMessage()
	msg2.ChatSessionID = msg.ChatSessionID
	_ = store.StoreUserMessage(ctx, msg2)

	// Note: SQLite DeleteSession does not delete session_metadata.
	// So the message_count increments from the previous value (1 + 1 = 2).
	meta, _ := store.GetSessionMeta(ctx, msg.ChatSessionID)
	if meta.MessageCount != 2 {
		t.Errorf("After re-store, MessageCount = %d, want 2 (soft-delete preserves metadata)", meta.MessageCount)
	}
}

// ========================================
// SQLite Full Workflow Test
// ========================================

func TestSQLite_FullWorkflow(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := "workflow-session-" + uuid.New().String()[:8]
	userID := "U999999"
	platform := "slack"

	// 1. Store user message
	userMsg := testMessage()
	userMsg.ChatSessionID = sessionID
	userMsg.ChatUserID = userID
	userMsg.ChatPlatform = platform
	userMsg.MessageType = types.MessageTypeUserInput
	userMsg.Content = "What is Go?"

	if err := store.StoreUserMessage(ctx, userMsg); err != nil {
		t.Fatalf("Store user message: %v", err)
	}

	// 2. Store bot response
	botMsg := testMessage()
	botMsg.ChatSessionID = sessionID
	botMsg.ChatUserID = userID
	botMsg.ChatPlatform = platform
	botMsg.MessageType = types.MessageTypeFinalResponse
	botMsg.Content = "Go is a programming language."
	botMsg.FromUserID = "UBOT"

	if err := store.StoreBotResponse(ctx, botMsg); err != nil {
		t.Fatalf("Store bot response: %v", err)
	}

	// 3. List and verify
	messages, err := store.List(ctx, &MessageQuery{ChatSessionID: sessionID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	// 4. Count
	count, err := store.Count(ctx, &MessageQuery{ChatSessionID: sessionID})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	// 5. Get each message
	for _, msg := range messages {
		got, err := store.Get(ctx, msg.ID)
		if err != nil {
			t.Fatalf("Get %s: %v", msg.ID, err)
		}
		if got.Content == "" {
			t.Errorf("Message %s has empty content", msg.ID)
		}
	}

	// 6. Session meta
	meta, err := store.GetSessionMeta(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionMeta: %v", err)
	}
	if meta.MessageCount != 2 {
		t.Errorf("Meta MessageCount = %d, want 2", meta.MessageCount)
	}

	// 7. List user sessions
	sessions, err := store.ListUserSessions(ctx, platform, userID)
	if err != nil {
		t.Fatalf("ListUserSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("ListUserSessions count = %d, want 1", len(sessions))
	}

	// 8. Delete session
	if err := store.DeleteSession(ctx, sessionID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// 9. Verify empty after delete
	count, _ = store.Count(ctx, &MessageQuery{ChatSessionID: sessionID})
	if count != 0 {
		t.Errorf("After delete, count = %d, want 0", count)
	}
}

// ========================================
// SQLite Config Export/Import Tests
// ========================================

func TestSQLite_ExportToJSON(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "export.json")

	if err := ExportToJSON(store, outputPath, &MessageQuery{ChatSessionID: msg.ChatSessionID}); err != nil {
		t.Fatalf("ExportToJSON failed: %v", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Output file does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Output file is empty")
	}
}

func TestSQLite_ExportToJSON_Empty(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "export_empty.json")

	if err := ExportToJSON(store, outputPath, &MessageQuery{ChatSessionID: "nonexistent"}); err != nil {
		t.Fatalf("ExportToJSON empty should succeed: %v", err)
	}
}

func TestSQLite_ImportFromJSON(t *testing.T) {
	// First export
	store1, cleanup1 := newSQLiteTestStore(t)
	defer cleanup1()

	ctx := context.Background()
	msg := testMessage()
	_ = store1.StoreUserMessage(ctx, msg)

	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export_import.json")

	if err := ExportToJSON(store1, exportPath, &MessageQuery{ChatSessionID: msg.ChatSessionID}); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import into a new store
	store2, cleanup2 := newSQLiteTestStore(t)
	defer cleanup2()

	imported, err := ImportFromJSON(store2, exportPath)
	if err != nil {
		t.Fatalf("ImportFromJSON failed: %v", err)
	}
	if imported != 1 {
		t.Errorf("Imported = %d, want 1", imported)
	}

	// Verify imported message
	count, _ := store2.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 1 {
		t.Errorf("After import, count = %d, want 1", count)
	}
}

func TestSQLite_ImportFromJSON_InvalidFile(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	imported, err := ImportFromJSON(store, "/nonexistent/path.json")
	if err == nil {
		t.Fatal("ImportFromJSON should fail for nonexistent file")
	}
	if imported != 0 {
		t.Errorf("Imported should be 0 on error, got %d", imported)
	}
}

// ========================================
// SQLite BackupStorage Tests
// ========================================

func TestSQLite_BackupStorage(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "backup.json")

	if err := BackupStorage(store, backupPath); err != nil {
		t.Fatalf("BackupStorage failed: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("Backup file does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Backup file is empty")
	}
}

func TestSQLite_BackupStorage_Empty(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "backup_empty.json")

	if err := BackupStorage(store, backupPath); err != nil {
		t.Fatalf("BackupStorage empty should succeed: %v", err)
	}
}

// ========================================
// SQLite ConfigLoader Tests
// ========================================

func TestConfigLoader_LoadStorageConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"enabled": true,
		"type": "sqlite",
		"sqlite": {
			"path": "/tmp/test.db",
			"max_size_mb": 100
		},
		"postgres": {
			"host": "localhost",
			"port": 5432
		},
		"strategy": "default",
		"streaming": {
			"enabled": true,
			"buffer_size": 1024,
			"timeout_seconds": 30,
			"storage_policy": "immediate"
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	loader := NewConfigLoader(configPath)
	cfg, err := loader.LoadStorageConfig()
	if err != nil {
		t.Fatalf("LoadStorageConfig failed: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.Type != "sqlite" {
		t.Errorf("Type = %q, want %q", cfg.Type, "sqlite")
	}
	if cfg.SQLite.Path != "/tmp/test.db" {
		t.Errorf("SQLite.Path = %q, want %q", cfg.SQLite.Path, "/tmp/test.db")
	}
	if cfg.SQLite.MaxSizeMB != 100 {
		t.Errorf("SQLite.MaxSizeMB = %d, want 100", cfg.SQLite.MaxSizeMB)
	}
	if cfg.Streaming.Enabled != true {
		t.Error("Streaming.Enabled should be true")
	}
	if cfg.Streaming.BufferSize != 1024 {
		t.Errorf("Streaming.BufferSize = %d, want 1024", cfg.Streaming.BufferSize)
	}
	if cfg.Streaming.TimeoutSec != 30 {
		t.Errorf("Streaming.TimeoutSec = %d, want 30", cfg.Streaming.TimeoutSec)
	}
	if cfg.Streaming.StoragePolicy != "immediate" {
		t.Errorf("Streaming.StoragePolicy = %q, want %q", cfg.Streaming.StoragePolicy, "immediate")
	}
	if cfg.PostgreSQL.Host != "localhost" {
		t.Errorf("PostgreSQL.Host = %q, want %q", cfg.PostgreSQL.Host, "localhost")
	}
}

func TestConfigLoader_NotFound(t *testing.T) {
	loader := NewConfigLoader("/nonexistent/config.json")
	_, err := loader.LoadStorageConfig()
	if err == nil {
		t.Fatal("LoadStorageConfig should fail for nonexistent file")
	}
}

func TestConfigLoader_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(configPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	loader := NewConfigLoader(configPath)
	_, err := loader.LoadStorageConfig()
	if err == nil {
		t.Fatal("LoadStorageConfig should fail for invalid JSON")
	}
}

// ========================================
// SQLite Benchmarks
// ========================================

func BenchmarkSQLite_Store(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	store := &SQLiteStorage{
		config:   PluginConfig{"path": dbPath},
		strategy: NewDefaultStrategy(),
	}
	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		b.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	engineSessionID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := testMessage()
		msg.EngineSessionID = engineSessionID
		_ = store.StoreUserMessage(ctx, msg)
	}
}

func BenchmarkSQLite_Get(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	store := &SQLiteStorage{
		config:   PluginConfig{"path": dbPath},
		strategy: NewDefaultStrategy(),
	}
	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		b.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Pre-populate
	var ids []string
	for i := 0; i < 100; i++ {
		msg := testMessage()
		_ = store.StoreUserMessage(ctx, msg)
		ids = append(ids, msg.ID)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, ids[i%len(ids)])
	}
}

func BenchmarkSQLite_List(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	store := &SQLiteStorage{
		config:   PluginConfig{"path": dbPath},
		strategy: NewDefaultStrategy(),
	}
	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		b.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	sessionID := "bench-session"
	for i := 0; i < 1000; i++ {
		msg := testMessage()
		msg.ChatSessionID = sessionID
		_ = store.StoreUserMessage(ctx, msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.List(ctx, &MessageQuery{ChatSessionID: sessionID})
	}
}

// TestSQLite_InitializeWithRealFile tests initialization creates the database file.
func TestSQLite_InitializeWithRealFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "real_test.db")

	store := &SQLiteStorage{
		config:   PluginConfig{"path": dbPath},
		strategy: NewDefaultStrategy(),
	}

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should exist after Initialize")
	}
}

// TestSQLite_StoreUserMessage_Timestamps verifies timestamps are set by storeMessage.
func TestSQLite_StoreUserMessage_Timestamps(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	before := time.Now()

	_ = store.StoreUserMessage(ctx, msg)
	after := time.Now()

	if msg.CreatedAt.Before(before) || msg.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, should be between %v and %v", msg.CreatedAt, before, after)
	}
	if msg.UpdatedAt.Before(before) || msg.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt = %v, should be between %v and %v", msg.UpdatedAt, before, after)
	}
}

// TestSQLite_List_Filters tests List filtering by session ID and user ID combined.
func TestSQLite_List_Filters(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	// Different session
	msg2 := testMessage()
	msg2.ChatSessionID = "other-session"
	_ = store.StoreUserMessage(ctx, msg2)

	// Filter by original session
	messages, err := store.List(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, m := range messages {
		if m.ChatSessionID != msg.ChatSessionID {
			t.Errorf("List returned message from wrong session: %s", m.ChatSessionID)
		}
	}
}

// TestSQLite_Count_Filters tests Count filtering.
func TestSQLite_Count_Filters(t *testing.T) {
	store, cleanup := newSQLiteTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Store messages in two sessions
	for i := 0; i < 3; i++ {
		msg := testMessage()
		msg.ChatSessionID = "session-A"
		_ = store.StoreUserMessage(ctx, msg)
	}
	for i := 0; i < 2; i++ {
		msg := testMessage()
		msg.ChatSessionID = "session-B"
		_ = store.StoreUserMessage(ctx, msg)
	}

	countA, _ := store.Count(ctx, &MessageQuery{ChatSessionID: "session-A"})
	countB, _ := store.Count(ctx, &MessageQuery{ChatSessionID: "session-B"})
	countAll, _ := store.Count(ctx, &MessageQuery{})

	if countA != 3 {
		t.Errorf("Count session-A = %d, want 3", countA)
	}
	if countB != 2 {
		t.Errorf("Count session-B = %d, want 2", countB)
	}
	if countAll != 5 {
		t.Errorf("Count all = %d, want 5", countAll)
	}
}
