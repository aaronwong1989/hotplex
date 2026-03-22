package relay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// RelayMessage JSON round-trip
// ---------------------------------------------------------------------------

func TestRelayMessage_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  RelayMessage
	}{
		{
			name: "full message",
			msg: RelayMessage{
				TaskID:     "task-123",
				From:       "agent-alpha",
				To:         "agent-beta",
				Content:    "hello world",
				SessionKey: "slack:U123:U456:C789",
				Metadata:   `{"key":"value"}`,
				Status:     TaskStatusWorking,
				Response:   "ack",
				Error:      "",
				CreatedAt:  parseTime(t, "2026-03-22T10:00:00Z"),
			},
		},
		{
			name: "minimal message - omitempty fields omitted",
			msg: RelayMessage{
				TaskID:  "task-min",
				Content: "hi",
			},
		},
		{
			name: "message with error",
			msg: RelayMessage{
				TaskID:    "task-err",
				To:        "agent-x",
				Content:   "do work",
				Status:    TaskStatusFailed,
				Error:     "connection refused",
				CreatedAt: parseTime(t, "2026-03-22T12:00:00Z"),
			},
		},
		{
			name: "message with all statuses",
			msg: RelayMessage{
				TaskID: "task-all",
				Status: TaskStatusCompleted,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded RelayMessage
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.TaskID != tt.msg.TaskID {
				t.Errorf("TaskID = %q, want %q", decoded.TaskID, tt.msg.TaskID)
			}
			if decoded.From != tt.msg.From {
				t.Errorf("From = %q, want %q", decoded.From, tt.msg.From)
			}
			if decoded.To != tt.msg.To {
				t.Errorf("To = %q, want %q", decoded.To, tt.msg.To)
			}
			if decoded.Content != tt.msg.Content {
				t.Errorf("Content = %q, want %q", decoded.Content, tt.msg.Content)
			}
			if decoded.SessionKey != tt.msg.SessionKey {
				t.Errorf("SessionKey = %q, want %q", decoded.SessionKey, tt.msg.SessionKey)
			}
			if decoded.Metadata != tt.msg.Metadata {
				t.Errorf("Metadata = %q, want %q", decoded.Metadata, tt.msg.Metadata)
			}
			if decoded.Status != tt.msg.Status {
				t.Errorf("Status = %q, want %q", decoded.Status, tt.msg.Status)
			}
			if decoded.Response != tt.msg.Response {
				t.Errorf("Response = %q, want %q", decoded.Response, tt.msg.Response)
			}
			if decoded.Error != tt.msg.Error {
				t.Errorf("Error = %q, want %q", decoded.Error, tt.msg.Error)
			}
			if !decoded.CreatedAt.Equal(tt.msg.CreatedAt) {
				t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, tt.msg.CreatedAt)
			}
		})
	}
}

func TestRelayMessage_OmitemptyFields(t *testing.T) {
	msg := RelayMessage{TaskID: "t1"}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	omitemptyFields := []string{"from", "to", "content", "session_key", "metadata", "status", "response", "error"}
	for _, field := range omitemptyFields {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when empty", field)
		}
	}

	// TaskID is always present
	if _, ok := raw["task_id"]; !ok {
		t.Error("task_id should be present")
	}
}

// ---------------------------------------------------------------------------
// RelayResponse JSON round-trip
// ---------------------------------------------------------------------------

func TestRelayResponse_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		resp RelayResponse
	}{
		{
			name: "full response",
			resp: RelayResponse{
				TaskID:    "resp-123",
				Status:    "sent",
				Content:   "result data",
				Timestamp: parseTime(t, "2026-03-22T10:30:00Z"),
				Error:     "",
				Duration:  1500000000, // 1.5s
			},
		},
		{
			name: "error response",
			resp: RelayResponse{
				TaskID:    "resp-err",
				Status:    "failed",
				Timestamp: parseTime(t, "2026-03-22T10:31:00Z"),
				Error:     "timeout",
				Duration:  10000000000, // 10s
			},
		},
		{
			name: "minimal response",
			resp: RelayResponse{
				TaskID:    "resp-min",
				Status:    "ok",
				Timestamp: parseTime(t, "2026-03-22T10:32:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded RelayResponse
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.TaskID != tt.resp.TaskID {
				t.Errorf("TaskID = %q, want %q", decoded.TaskID, tt.resp.TaskID)
			}
			if decoded.Status != tt.resp.Status {
				t.Errorf("Status = %q, want %q", decoded.Status, tt.resp.Status)
			}
			if decoded.Content != tt.resp.Content {
				t.Errorf("Content = %q, want %q", decoded.Content, tt.resp.Content)
			}
			if !decoded.Timestamp.Equal(tt.resp.Timestamp) {
				t.Errorf("Timestamp = %v, want %v", decoded.Timestamp, tt.resp.Timestamp)
			}
			if decoded.Error != tt.resp.Error {
				t.Errorf("Error = %q, want %q", decoded.Error, tt.resp.Error)
			}
			if decoded.Duration != tt.resp.Duration {
				t.Errorf("Duration = %v, want %v", decoded.Duration, tt.resp.Duration)
			}
		})
	}
}

func TestRelayResponse_OmitemptyFields(t *testing.T) {
	resp := RelayResponse{TaskID: "r1", Status: "ok"}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	if _, ok := raw["content"]; ok {
		t.Error("content should be omitted when empty")
	}
	if _, ok := raw["error"]; ok {
		t.Error("error should be omitted when empty")
	}
}

// ---------------------------------------------------------------------------
// RelayBinding JSON round-trip
// ---------------------------------------------------------------------------

func TestRelayBinding_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		binding RelayBinding
	}{
		{
			name: "binding with bots",
			binding: RelayBinding{
				Platform: "slack",
				ChatID:   "C12345",
				Bots: map[string]string{
					"agent-alpha": "http://localhost:8080/relay",
					"agent-beta":  "http://localhost:8081/relay",
				},
			},
		},
		{
			name: "binding with single bot",
			binding: RelayBinding{
				Platform: "telegram",
				ChatID:   "T67890",
				Bots: map[string]string{
					"agent-gamma": "http://localhost:9090/relay",
				},
			},
		},
		{
			name: "binding with empty bots map",
			binding: RelayBinding{
				Platform: "discord",
				ChatID:   "D11111",
				Bots:     map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.binding)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded RelayBinding
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Platform != tt.binding.Platform {
				t.Errorf("Platform = %q, want %q", decoded.Platform, tt.binding.Platform)
			}
			if decoded.ChatID != tt.binding.ChatID {
				t.Errorf("ChatID = %q, want %q", decoded.ChatID, tt.binding.ChatID)
			}
			if len(decoded.Bots) != len(tt.binding.Bots) {
				t.Errorf("Bots len = %d, want %d", len(decoded.Bots), len(tt.binding.Bots))
			}
			for k, v := range tt.binding.Bots {
				if decoded.Bots[k] != v {
					t.Errorf("Bots[%q] = %q, want %q", k, decoded.Bots[k], v)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Task status constants
// ---------------------------------------------------------------------------

func TestTaskStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"working", TaskStatusWorking},
		{"completed", TaskStatusCompleted},
		{"failed", TaskStatusFailed},
		{"canceled", TaskStatusCanceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status == "" {
				t.Error("status constant should not be empty")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BindingStore
// ---------------------------------------------------------------------------

func TestBindingStore_NewWithTempDir(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	if store == nil {
		t.Fatal("NewBindingStore returned nil")
	}

	bindings := store.List()
	if len(bindings) != 0 {
		t.Errorf("new store should have 0 bindings, got %d", len(bindings))
	}
}

func TestBindingStore_AddAndList(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	binding := &RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots: map[string]string{
			"agent-a": "http://localhost:8080",
		},
	}

	if err := store.Add(binding); err != nil {
		t.Fatalf("Add: %v", err)
	}

	bindings := store.List()
	if len(bindings) != 1 {
		t.Fatalf("List len = %d, want 1", len(bindings))
	}

	if bindings[0].Platform != "slack" {
		t.Errorf("Platform = %q, want %q", bindings[0].Platform, "slack")
	}
	if bindings[0].ChatID != "C123" {
		t.Errorf("ChatID = %q, want %q", bindings[0].ChatID, "C123")
	}
	if bindings[0].Bots["agent-a"] != "http://localhost:8080" {
		t.Errorf("Bots[agent-a] = %q, want %q", bindings[0].Bots["agent-a"], "http://localhost:8080")
	}
}

func TestBindingStore_AddOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	binding1 := &RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots: map[string]string{
			"agent-a": "http://localhost:8080",
		},
	}
	binding2 := &RelayBinding{
		Platform: "telegram",
		ChatID:   "C123",
		Bots: map[string]string{
			"agent-b": "http://localhost:9090",
		},
	}

	_ = store.Add(binding1)
	_ = store.Add(binding2)

	bindings := store.List()
	if len(bindings) != 1 {
		t.Fatalf("List len = %d, want 1 (overwrite)", len(bindings))
	}

	if bindings[0].Platform != "telegram" {
		t.Errorf("Platform after overwrite = %q, want %q", bindings[0].Platform, "telegram")
	}
}

func TestBindingStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	_ = store.Add(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     map[string]string{"a": "http://a"},
	})

	if err := store.Delete("C123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	bindings := store.List()
	if len(bindings) != 0 {
		t.Errorf("List len after delete = %d, want 0", len(bindings))
	}
}

func TestBindingStore_DeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("Delete nonexistent should return error, got nil")
	}
}

func TestBindingStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	// Create store, add binding, let it go out of scope.
	store1 := newBindingStoreWithPath(dir)
	_ = store1.Add(&RelayBinding{
		Platform: "slack",
		ChatID:   "C999",
		Bots: map[string]string{
			"agent-x": "http://x:8080",
		},
	})

	// Reload from disk.
	store2 := newBindingStoreWithPath(dir)
	bindings := store2.List()
	if len(bindings) != 1 {
		t.Fatalf("reloaded store len = %d, want 1", len(bindings))
	}
	if bindings[0].ChatID != "C999" {
		t.Errorf("ChatID = %q, want %q", bindings[0].ChatID, "C999")
	}
	if bindings[0].Bots["agent-x"] != "http://x:8080" {
		t.Errorf("Bots[agent-x] = %q, want %q", bindings[0].Bots["agent-x"], "http://x:8080")
	}
}

func TestBindingStore_PersistenceMultipleBindings(t *testing.T) {
	dir := t.TempDir()

	store1 := newBindingStoreWithPath(dir)
	_ = store1.Add(&RelayBinding{Platform: "slack", ChatID: "C1", Bots: map[string]string{"a": "http://a"}})
	_ = store1.Add(&RelayBinding{Platform: "telegram", ChatID: "C2", Bots: map[string]string{"b": "http://b"}})
	_ = store1.Add(&RelayBinding{Platform: "discord", ChatID: "C3", Bots: map[string]string{"c": "http://c"}})

	store2 := newBindingStoreWithPath(dir)
	bindings := store2.List()
	if len(bindings) != 3 {
		t.Fatalf("reloaded store len = %d, want 3", len(bindings))
	}
}

func TestBindingStore_DeleteAndReload(t *testing.T) {
	dir := t.TempDir()

	store1 := newBindingStoreWithPath(dir)
	_ = store1.Add(&RelayBinding{Platform: "slack", ChatID: "C1", Bots: map[string]string{"a": "http://a"}})
	_ = store1.Add(&RelayBinding{Platform: "telegram", ChatID: "C2", Bots: map[string]string{"b": "http://b"}})
	_ = store1.Delete("C1")

	store2 := newBindingStoreWithPath(dir)
	bindings := store2.List()
	if len(bindings) != 1 {
		t.Fatalf("len after delete+reload = %d, want 1", len(bindings))
	}
	if bindings[0].ChatID != "C2" {
		t.Errorf("ChatID = %q, want %q", bindings[0].ChatID, "C2")
	}
}

func TestBindingStore_AtomicWriteFileFormat(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	_ = store.Add(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     map[string]string{"agent-a": "http://a:8080"},
	})

	path := filepath.Join(dir, "bindings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bindings.json: %v", err)
	}

	var bf bindingsFile
	if err := json.Unmarshal(data, &bf); err != nil {
		t.Fatalf("parse bindings.json: %v", err)
	}

	if bf.Version != bindingsSchemaVersion {
		t.Errorf("Version = %d, want %d", bf.Version, bindingsSchemaVersion)
	}
	if len(bf.Bindings) != 1 {
		t.Fatalf("Bindings len = %d, want 1", len(bf.Bindings))
	}
	if bf.Bindings[0].ChatID != "C123" {
		t.Errorf("ChatID = %q, want %q", bf.Bindings[0].ChatID, "C123")
	}
}

func TestBindingStore_LoadNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	// No bindings.json file exists; store should be empty without error.
	bindings := store.List()
	if len(bindings) != 0 {
		t.Errorf("empty dir should produce 0 bindings, got %d", len(bindings))
	}
}

func TestBindingStore_LoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bindings.json")

	// Write invalid JSON.
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	store := newBindingStoreWithPath(dir)
	// Load fails silently (NewBindingStore ignores load error), store should be empty.
	bindings := store.List()
	if len(bindings) != 0 {
		t.Errorf("corrupt file should produce 0 bindings, got %d", len(bindings))
	}
}

func TestBindingStore_AtomicWritePermissions(t *testing.T) {
	dir := t.TempDir()
	store := newBindingStoreWithPath(dir)

	_ = store.Add(&RelayBinding{
		Platform: "slack",
		ChatID:   "C123",
		Bots:     map[string]string{"a": "http://a"},
	})

	path := filepath.Join(dir, "bindings.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat bindings.json: %v", err)
	}

	// File should be readable (0644)
	if info.Mode().Perm()&0o644 != 0o644 {
		t.Errorf("file permissions = %o, want 0644", info.Mode().Perm())
	}
}

// newBindingStoreWithPath creates a BindingStore using a specific directory
// instead of the default $HOME/.hotplex/relay path.
func newBindingStoreWithPath(dir string) *BindingStore {
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "bindings.json")
	bs := &BindingStore{path: path, bindings: make(map[string]*RelayBinding)}
	_ = bs.load()
	return bs
}

// parseTime parses an RFC3339 timestamp or fails the test.
func parseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}
