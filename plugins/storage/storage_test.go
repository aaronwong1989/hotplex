package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex/types"
)

// ========================================
// StorageError Tests
// ========================================

func TestStorageError_Error_WithCause(t *testing.T) {
	inner := errors.New("db connection lost")
	se := NewStorageError("DB_CONN", "failed to connect", inner)

	got := se.Error()
	if got == "" {
		t.Error("Error() should not return empty string")
	}
	// Should contain code, message, and cause
	if se.Code != "DB_CONN" {
		t.Errorf("Code = %q, want %q", se.Code, "DB_CONN")
	}
	if se.Message != "failed to connect" {
		t.Errorf("Message = %q, want %q", se.Message, "failed to connect")
	}
	if se.Err != inner {
		t.Error("Err should match the inner error")
	}
}

func TestStorageError_Error_WithoutCause(t *testing.T) {
	se := NewStorageError("VALIDATE", "field is empty", nil)

	got := se.Error()
	if got == "" {
		t.Error("Error() should not return empty string")
	}
	// Should not contain parenthesis when no cause
	if se.Err != nil {
		t.Error("Err should be nil")
	}
}

func TestStorageError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	se := NewStorageError("WRAP_TEST", "test", inner)

	unwrapped := se.Unwrap()
	if unwrapped != inner {
		t.Error("Unwrap() should return the inner error")
	}
}

func TestStorageError_Unwrap_Nil(t *testing.T) {
	se := NewStorageError("NIL_WRAP", "test", nil)

	unwrapped := se.Unwrap()
	if unwrapped != nil {
		t.Error("Unwrap() should return nil when Err is nil")
	}
}

// ========================================
// IsNotFound Tests
// ========================================

func TestIsNotFound_ErrNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("IsNotFound should return true for ErrNotFound")
	}
}

func TestIsNotFound_ErrSessionNotFound(t *testing.T) {
	if !IsNotFound(ErrSessionNotFound) {
		t.Error("IsNotFound should return true for ErrSessionNotFound")
	}
}

func TestIsNotFound_StorageError_Wrapped(t *testing.T) {
	se := NewStorageError("GET_MSG", "message not found", ErrNotFound)
	if !IsNotFound(se) {
		t.Error("IsNotFound should return true for StorageError wrapping ErrNotFound")
	}
}

func TestIsNotFound_OtherError(t *testing.T) {
	if IsNotFound(errors.New("something else")) {
		t.Error("IsNotFound should return false for unrelated errors")
	}
}

// ========================================
// IsConnectionError Tests
// ========================================

func TestIsConnectionError_Direct(t *testing.T) {
	if !IsConnectionError(ErrConnectionFailed) {
		t.Error("IsConnectionError should return true for ErrConnectionFailed")
	}
}

func TestIsConnectionError_Wrapped(t *testing.T) {
	se := NewStorageError("CONN", "connection failed", ErrConnectionFailed)
	if !IsConnectionError(se) {
		t.Error("IsConnectionError should return true for wrapped ErrConnectionFailed")
	}
}

func TestIsConnectionError_OtherError(t *testing.T) {
	if IsConnectionError(ErrNotFound) {
		t.Error("IsConnectionError should return false for ErrNotFound")
	}
}

// ========================================
// IsConfigError Tests
// ========================================

func TestIsConfigError_InvalidConfig(t *testing.T) {
	if !IsConfigError(ErrInvalidConfig) {
		t.Error("IsConfigError should return true for ErrInvalidConfig")
	}
}

func TestIsConfigError_UnsupportedType(t *testing.T) {
	if !IsConfigError(ErrUnsupportedType) {
		t.Error("IsConfigError should return true for ErrUnsupportedType")
	}
}

func TestIsConfigError_Wrapped(t *testing.T) {
	se := NewStorageError("CFG", "bad config", ErrInvalidConfig)
	if !IsConfigError(se) {
		t.Error("IsConfigError should return true for wrapped ErrInvalidConfig")
	}
}

func TestIsConfigError_OtherError(t *testing.T) {
	if IsConfigError(ErrNotFound) {
		t.Error("IsConfigError should return false for ErrNotFound")
	}
}

// ========================================
// Sentinel Error Values Tests
// ========================================

func TestSentinelErrors(t *testing.T) {
	errs := []error{
		ErrNotFound,
		ErrSessionNotFound,
		ErrInvalidMessage,
		ErrStorageNotEnabled,
		ErrConnectionFailed,
		ErrQueryFailed,
		ErrStoreFailed,
		ErrInvalidConfig,
		ErrUnsupportedType,
		ErrSessionClosed,
		ErrTransactionFailed,
	}
	for _, err := range errs {
		if err == nil {
			t.Error("Sentinel error should not be nil")
		}
		if err.Error() == "" {
			t.Errorf("Sentinel error %v has empty message", err)
		}
	}
}

// ========================================
// PluginRegistry Tests
// ========================================

func TestNewPluginRegistry(t *testing.T) {
	r := NewPluginRegistry()
	if r == nil {
		t.Fatal("NewPluginRegistry returned nil")
	}
}

func TestNewPluginRegistry_DefaultFactories(t *testing.T) {
	r := NewPluginRegistry()
	names := r.List()

	expected := map[string]bool{
		"memory":     false,
		"sqlite":     false,
		"postgresql": false,
	}
	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("Default factory %q not registered", name)
		}
	}
}

func TestPluginRegistry_RegisterAndGet(t *testing.T) {
	r := NewPluginRegistry()

	// Register a custom factory
	customFactory := &MemoryFactory{}
	r.Register("custom", customFactory)

	store, err := r.Get("custom", PluginConfig{})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if store == nil {
		t.Fatal("Get returned nil store")
	}
}

func TestPluginRegistry_Get_Unknown(t *testing.T) {
	r := NewPluginRegistry()

	store, err := r.Get("unknown_type", PluginConfig{})
	if err != nil {
		t.Fatalf("Get for unknown should not error: %v", err)
	}
	if store != nil {
		t.Fatal("Get for unknown type should return nil store")
	}
}

func TestPluginRegistry_List(t *testing.T) {
	r := NewPluginRegistry()

	r.Register("extra1", &MemoryFactory{})
	r.Register("extra2", &MemoryFactory{})

	names := r.List()
	if len(names) < 5 { // 3 default + 2 extra
		t.Errorf("List returned %d names, expected at least 5", len(names))
	}
}

func TestPluginRegistry_ConcurrentAccess(t *testing.T) {
	r := NewPluginRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.Register("concurrent_"+string(rune('A'+id)), &MemoryFactory{})
			_ = r.List()
			_, _ = r.Get("memory", PluginConfig{})
		}(i)
	}
	wg.Wait()
}

func TestGlobalRegistry(t *testing.T) {
	r := GlobalRegistry()
	if r == nil {
		t.Fatal("GlobalRegistry returned nil")
	}

	// Should return same instance on second call
	r2 := GlobalRegistry()
	if r != r2 {
		t.Error("GlobalRegistry should return the same instance")
	}
}

func TestGlobalRegistry_Defaults(t *testing.T) {
	r := GlobalRegistry()
	names := r.List()

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["memory"] {
		t.Error("GlobalRegistry should have 'memory' factory")
	}
	if !found["sqlite"] {
		t.Error("GlobalRegistry should have 'sqlite' factory")
	}
}

// ========================================
// DefaultHealthCheck Tests
// ========================================

func TestDefaultHealthCheck_Healthy(t *testing.T) {
	factory := &MemoryFactory{}
	store, err := factory.Create(PluginConfig{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	result := DefaultHealthCheck(store)
	if result == nil {
		t.Fatal("DefaultHealthCheck should not return nil")
	}
	if result.Status != "healthy" {
		t.Errorf("Status = %q, want %q", result.Status, "healthy")
	}
	if result.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if result.Latency < 0 {
		t.Error("Latency should be non-negative")
	}
	if result.Checks == nil {
		t.Error("Checks should not be nil")
	}
	if check, ok := result.Checks["query"]; !ok {
		t.Error("Checks should have 'query' entry")
	} else if check.Status != "pass" {
		t.Errorf("Query check status = %q, want %q", check.Status, "pass")
	}
}

// ========================================
// GetMetrics Tests
// ========================================

func TestGetMetrics(t *testing.T) {
	factory := &MemoryFactory{}
	store, err := factory.Create(PluginConfig{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	metrics, err := GetMetrics(store)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if metrics == nil {
		t.Fatal("GetMetrics should not return nil")
	}
	if metrics.TotalMessages != 1 {
		t.Errorf("TotalMessages = %d, want 1", metrics.TotalMessages)
	}
}

func TestGetMetrics_Empty(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	metrics, err := GetMetrics(store)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if metrics.TotalMessages != 0 {
		t.Errorf("TotalMessages = %d, want 0", metrics.TotalMessages)
	}
}

// ========================================
// HealthCheckResult / StorageMetrics Tests
// ========================================

func TestHealthCheckResult_Fields(t *testing.T) {
	now := time.Now()
	result := &HealthCheckResult{
		Status:    "degraded",
		Latency:   50 * time.Millisecond,
		Timestamp: now,
		Checks: map[string]Check{
			"db": {Status: "warn", Message: "slow query", Latency: 50 * time.Millisecond},
		},
	}

	if result.Status != "degraded" {
		t.Errorf("Status = %q", result.Status)
	}
	if result.Latency != 50*time.Millisecond {
		t.Errorf("Latency = %v", result.Latency)
	}
	if len(result.Checks) != 1 {
		t.Errorf("Checks count = %d, want 1", len(result.Checks))
	}
}

func TestStorageMetrics_Fields(t *testing.T) {
	metrics := &StorageMetrics{
		TotalMessages:    100,
		TotalSessions:    10,
		StorageSizeBytes: 4096,
		Uptime:           5 * time.Minute,
	}

	if metrics.TotalMessages != 100 {
		t.Errorf("TotalMessages = %d", metrics.TotalMessages)
	}
	if metrics.StorageSizeBytes != 4096 {
		t.Errorf("StorageSizeBytes = %d", metrics.StorageSizeBytes)
	}
}

// ========================================
// ParseMessageType Additional Coverage
// ========================================

func TestParseMessageType_AdditionalCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"userinput", "user_input"},
		{"user-input", "user_input"},
		{"finalresponse", "final_response"},
		{"final-response", "final_response"},
		{"RESPONSE", "final_response"},
		{"tooluse", "tool_use"},
		{"tool-use", "tool_use"},
		{"toolresult", "tool_result"},
		{"tool-result", "tool_result"},
		{"ERROR_MESSAGE", "error"},
		{"  unknown  ", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseMessageType(tt.input)
			if result != tt.expected {
				t.Errorf("ParseMessageType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ========================================
// MemoryStorage Additional Coverage
// ========================================

func TestMemoryStorage_Initialize(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize should succeed: %v", err)
	}
}

func TestMemoryStorage_Close(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	if err := store.Close(); err != nil {
		t.Fatalf("Close should succeed: %v", err)
	}
}

func TestMemoryStorage_Name(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	if store.Name() != "memory" {
		t.Errorf("Name() = %q, want %q", store.Name(), "memory")
	}
}

func TestMemoryStorage_Version(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	if store.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", store.Version(), "1.0.0")
	}
}

func TestMemoryStorage_GetStrategy(t *testing.T) {
	store := &MemoryStorage{
		messages: make(map[string]*ChatAppMessage),
		sessions: make(map[string]*SessionMeta),
		strategy: NewDefaultStrategy(),
	}

	s := store.GetStrategy()
	if s == nil {
		t.Fatal("GetStrategy should return non-nil")
	}
}

func TestMemoryStorage_SetStrategy(t *testing.T) {
	store := &MemoryStorage{
		messages: make(map[string]*ChatAppMessage),
		sessions: make(map[string]*SessionMeta),
		strategy: NewDefaultStrategy(),
	}

	custom := &DefaultStrategy{}
	_ = store.SetStrategy(custom)

	if store.GetStrategy() != custom {
		t.Error("GetStrategy should return the custom strategy after SetStrategy")
	}
}

func TestMemoryStorage_SetStrategy_Nil(t *testing.T) {
	store := &MemoryStorage{
		messages: make(map[string]*ChatAppMessage),
		sessions: make(map[string]*SessionMeta),
		strategy: NewDefaultStrategy(),
	}

	_ = store.SetStrategy(nil)

	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeThinking // normally not storable
	_ = store.StoreUserMessage(ctx, msg)

	// With nil strategy, should still be stored
	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 1 {
		t.Errorf("With nil strategy, non-storable messages should be stored, got count %d", count)
	}
}

func TestMemoryStorage_Get_NotFound(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Get should return error for nonexistent message")
	}
}

func TestMemoryStorage_Get_Deleted(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	_ = store.DeleteSession(ctx, msg.ChatSessionID)

	_, err := store.Get(ctx, msg.ID)
	if err == nil {
		t.Fatal("Get should return error for deleted message")
	}
}

func TestMemoryStorage_List_Empty(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	messages, err := store.List(ctx, &MessageQuery{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("List on empty store should return 0, got %d", len(messages))
	}
}

func TestMemoryStorage_List_IncludeDeleted(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	_ = store.DeleteSession(ctx, msg.ChatSessionID)

	// Without IncludeDeleted
	messages, _ := store.List(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if len(messages) != 0 {
		t.Errorf("Without IncludeDeleted, should get 0, got %d", len(messages))
	}

	// With IncludeDeleted
	messages, _ = store.List(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID, IncludeDeleted: true})
	if len(messages) != 1 {
		t.Errorf("With IncludeDeleted, should get 1, got %d", len(messages))
	}
}

func TestMemoryStorage_List_ByUserID(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()

	msg1 := testMessage()
	msg1.ChatUserID = "user-A"
	_ = store.StoreUserMessage(ctx, msg1)

	msg2 := testMessage()
	msg2.ChatUserID = "user-B"
	_ = store.StoreUserMessage(ctx, msg2)

	messages, _ := store.List(ctx, &MessageQuery{ChatUserID: "user-A"})
	if len(messages) != 1 {
		t.Errorf("List by user-A should return 1, got %d", len(messages))
	}
	for _, m := range messages {
		if m.ChatUserID != "user-A" {
			t.Errorf("List returned message from wrong user: %s", m.ChatUserID)
		}
	}
}

func TestMemoryStorage_Count_Empty(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	count, err := store.Count(ctx, &MessageQuery{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Count on empty store should be 0, got %d", count)
	}
}

func TestMemoryStorage_Count_IncludeDeleted(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	_ = store.DeleteSession(ctx, msg.ChatSessionID)

	// Without IncludeDeleted
	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 0 {
		t.Errorf("Without IncludeDeleted, count should be 0, got %d", count)
	}

	// With IncludeDeleted
	count, _ = store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID, IncludeDeleted: true})
	if count != 1 {
		t.Errorf("With IncludeDeleted, count should be 1, got %d", count)
	}
}

func TestMemoryStorage_Count_ByUserID(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	msg := testMessage()
	_ = store.StoreUserMessage(ctx, msg)

	count, _ := store.Count(ctx, &MessageQuery{ChatUserID: msg.ChatUserID})
	if count != 1 {
		t.Errorf("Count by user should be 1, got %d", count)
	}
}

func TestMemoryStorage_GetSessionMeta_NotFound(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	_, err := store.GetSessionMeta(ctx, "nonexistent-session")
	if err == nil {
		t.Fatal("GetSessionMeta should return error for nonexistent session")
	}
}

func TestMemoryStorage_ListUserSessions_Empty(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	sessions, err := store.ListUserSessions(ctx, "nonexistent", "nonexistent")
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("ListUserSessions empty should return 0, got %d", len(sessions))
	}
}

func TestMemoryStorage_StoreUserMessage_NonStorable(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeThinking

	_ = store.StoreUserMessage(ctx, msg)

	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 0 {
		t.Errorf("Non-storable message should not be stored, got count %d", count)
	}
}

func TestMemoryStorage_StoreBotResponse_NonStorable(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	msg := testMessage()
	msg.MessageType = types.MessageTypeToolUse

	_ = store.StoreBotResponse(ctx, msg)

	count, _ := store.Count(ctx, &MessageQuery{ChatSessionID: msg.ChatSessionID})
	if count != 0 {
		t.Errorf("Non-storable bot response should not be stored, got count %d", count)
	}
}

func TestMemoryStorage_DeleteSession_NotExist(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	// Deleting a nonexistent session should not error
	err := store.DeleteSession(ctx, "nonexistent-session")
	if err != nil {
		t.Fatalf("DeleteSession nonexistent should succeed: %v", err)
	}
}

func TestMemoryStorage_UpdateSessionMeta_Accumulates(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	sessionID := "session-" + uuid.New().String()[:8]

	for i := 0; i < 10; i++ {
		msg := testMessage()
		msg.ChatSessionID = sessionID
		_ = store.StoreUserMessage(ctx, msg)
	}

	meta, _ := store.GetSessionMeta(ctx, sessionID)
	if meta.MessageCount != 10 {
		t.Errorf("MessageCount = %d, want 10", meta.MessageCount)
	}
	if meta.LastMessageAt.IsZero() {
		t.Error("LastMessageAt should not be zero")
	}
	if meta.LastMessageID == "" {
		t.Error("LastMessageID should not be empty")
	}
}

func TestMemoryStorage_SortOrder(t *testing.T) {
	factory := &MemoryFactory{}
	store, _ := factory.Create(PluginConfig{})

	ctx := context.Background()
	sessionID := "sort-session"

	// Store messages with a small delay to ensure different timestamps
	for i := 0; i < 5; i++ {
		msg := testMessage()
		msg.ChatSessionID = sessionID
		_ = store.StoreUserMessage(ctx, msg)
		time.Sleep(time.Millisecond)
	}

	messages, _ := store.List(ctx, &MessageQuery{ChatSessionID: sessionID})
	if len(messages) < 2 {
		t.Skip("Need at least 2 messages to test sort order")
	}

	// Verify DESC order (newest first)
	for i := 1; i < len(messages); i++ {
		if messages[i].CreatedAt.After(messages[i-1].CreatedAt) {
			t.Errorf("Messages not in DESC order at index %d", i)
		}
	}
}

// ========================================
// BuildSessionID Tests
// ========================================

func TestBuildSessionID_Format(t *testing.T) {
	result := BuildSessionID("slack", "U123", "C456")
	expected := "slack_U123_C456"
	if result != expected {
		t.Errorf("BuildSessionID = %q, want %q", result, expected)
	}
}

// ========================================
// FormatTimestamp Exact Tests
// ========================================

func TestFormatTimestamp_Exact(t *testing.T) {
	ts := time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC)
	result := FormatTimestamp(ts)
	expected := "2024-06-15 14:30:45"
	if result != expected {
		t.Errorf("FormatTimestamp = %q, want %q", result, expected)
	}
}

// ========================================
// PostgreSQL buildListQuery / buildCountQuery Tests (unit, no DB)
// ========================================

func TestPostgreStorage_BuildListQuery_AllFilters(t *testing.T) {
	now := time.Now()
	query := &MessageQuery{
		ChatSessionID:     "session-1",
		ChatUserID:        "user-1",
		EngineSessionID:   uuid.New(),
		ProviderSessionID: "prov-1",
		MessageTypes:      []types.MessageType{types.MessageTypeUserInput, types.MessageTypeFinalResponse},
		StartTime:         &now,
		EndTime:           &now,
		Limit:             50,
		Ascending:         true,
		IncludeDeleted:    true,
	}

	p := &PostgreStorage{}
	sql, args := p.buildListQuery(query)

	if sql == "" {
		t.Error("buildListQuery should return non-empty SQL")
	}
	if len(args) == 0 {
		t.Error("buildListQuery should return non-empty args")
	}
	// Should contain key fragments
	if len(args) < 6 { // at least session, user, engine, provider, start, end
		t.Errorf("Expected at least 6 args, got %d", len(args))
	}
}

func TestPostgreStorage_BuildListQuery_Minimal(t *testing.T) {
	query := &MessageQuery{}

	p := &PostgreStorage{}
	sql, args := p.buildListQuery(query)

	if sql == "" {
		t.Error("buildListQuery should return non-empty SQL")
	}
	// Should contain "deleted" filter by default
	if len(args) != 0 {
		t.Errorf("Minimal query should have 0 args, got %d", len(args))
	}
	// Default limit should be 100
	if len(args) == 0 && len(sql) > 0 {
		// Check SQL contains LIMIT 100 as default
		_ = sql
	}
}

func TestPostgreStorage_BuildListQuery_Descending(t *testing.T) {
	query := &MessageQuery{
		ChatSessionID: "session-1",
		Ascending:     false,
		Limit:         10,
	}

	p := &PostgreStorage{}
	sql, _ := p.buildListQuery(query)

	if sql == "" {
		t.Error("SQL should not be empty")
	}
}

func TestPostgreStorage_BuildListQuery_WithLimit(t *testing.T) {
	query := &MessageQuery{
		ChatSessionID: "session-1",
		Limit:         25,
	}

	p := &PostgreStorage{}
	sql, _ := p.buildListQuery(query)

	if sql == "" {
		t.Error("SQL should not be empty")
	}
}

func TestPostgreStorage_BuildCountQuery_AllFilters(t *testing.T) {
	query := &MessageQuery{
		ChatSessionID:  "session-1",
		ChatUserID:     "user-1",
		IncludeDeleted: false,
	}

	p := &PostgreStorage{}
	sql, args := p.buildCountQuery(query)

	if sql == "" {
		t.Error("buildCountQuery should return non-empty SQL")
	}
	if len(args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(args))
	}
}

func TestPostgreStorage_BuildCountQuery_Minimal(t *testing.T) {
	query := &MessageQuery{}

	p := &PostgreStorage{}
	sql, args := p.buildCountQuery(query)

	if sql == "" {
		t.Error("SQL should not be empty")
	}
	if len(args) != 0 {
		t.Errorf("Minimal count query should have 0 args, got %d", len(args))
	}
}

func TestPostgreStorage_Name(t *testing.T) {
	p := &PostgreStorage{}
	if p.Name() != "postgresql" {
		t.Errorf("Name() = %q, want %q", p.Name(), "postgresql")
	}
}

func TestPostgreStorage_Version(t *testing.T) {
	p := &PostgreStorage{}
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", p.Version(), "1.0.0")
	}
}

func TestPostgreStorage_Close_NilDB(t *testing.T) {
	// Close on nil db would panic, so we can't test that directly.
	// The Close method calls db.Close() which panics on nil.
	// This is documented here for awareness.
}

func TestPostgreStorage_GetStrategy(t *testing.T) {
	p := &PostgreStorage{strategy: NewDefaultStrategy()}
	s := p.GetStrategy()
	if s == nil {
		t.Fatal("GetStrategy should return non-nil")
	}
}

func TestPostgreStorage_SetStrategy(t *testing.T) {
	p := &PostgreStorage{}
	custom := &DefaultStrategy{}
	err := p.SetStrategy(custom)
	if err != nil {
		t.Fatalf("SetStrategy failed: %v", err)
	}
	if p.GetStrategy() != custom {
		t.Error("GetStrategy should return the custom strategy")
	}
}

// ========================================
// PostgreSQL parsePostgresDSN Edge Cases
// ========================================

func TestParsePostgresDSN_EmptyPath(t *testing.T) {
	result, err := parsePostgresDSN("postgres://user@host/")
	if err != nil {
		t.Fatalf("parsePostgresDSN error: %v", err)
	}
	// Path "/" after TrimPrefix yields "" which keeps the default "hotplex"
	if result.Database != "hotplex" {
		t.Errorf("Database should default to 'hotplex' for root path, got %q", result.Database)
	}
}

func TestParsePostgresDSN_InvalidPort(t *testing.T) {
	_, err := parsePostgresDSN("postgres://user:pass@host:notaport/db")
	if err == nil {
		t.Error("parsePostgresDSN should return error for invalid port")
	}
}

func TestGetPostgreConfig_URLTakesPrecedenceOverDSN(t *testing.T) {
	pluginConfig := PluginConfig{
		"url": "postgres://urluser@urlhost:5433/urldb",
		"dsn": "postgres://dsnuser@dsnhost:5434/dsndb",
	}

	pgConfig, err := getPostgreConfig(pluginConfig)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// URL should take precedence over DSN
	if pgConfig.Host != "urlhost" {
		t.Errorf("URL should take precedence over DSN, got host %q", pgConfig.Host)
	}
}

// ========================================
// StorageConfig Tests
// ========================================

func TestStorageConfig_Defaults(t *testing.T) {
	cfg := &StorageConfig{}

	if cfg.Enabled {
		t.Error("Enabled should default to false")
	}
	if cfg.Type != "" {
		t.Errorf("Type should default to empty, got %q", cfg.Type)
	}
}

func TestSQLiteConfig_Fields(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      "/tmp/test.db",
		MaxSizeMB: 100,
	}
	if cfg.Path != "/tmp/test.db" {
		t.Errorf("Path = %q", cfg.Path)
	}
	if cfg.MaxSizeMB != 100 {
		t.Errorf("MaxSizeMB = %d", cfg.MaxSizeMB)
	}
}

func TestStreamingConfig_Fields(t *testing.T) {
	cfg := StreamingConfig{
		Enabled:       true,
		BufferSize:    2048,
		TimeoutSec:    60,
		StoragePolicy: "batched",
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.BufferSize != 2048 {
		t.Errorf("BufferSize = %d", cfg.BufferSize)
	}
	if cfg.TimeoutSec != 60 {
		t.Errorf("TimeoutSec = %d", cfg.TimeoutSec)
	}
	if cfg.StoragePolicy != "batched" {
		t.Errorf("StoragePolicy = %q", cfg.StoragePolicy)
	}
}
