package permission

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePermissionStore_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)

	botID := "U0AHRCL1KCM"
	err := store.Load(botID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Add whitelist entry
	err = store.AddWhitelist(botID, "Bash:rm.*-rf", "user1")
	if err != nil {
		t.Fatalf("AddWhitelist() error = %v", err)
	}

	// Save
	err = store.Save(botID)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Reload
	store2 := NewFilePermissionStore(dir)
	err = store2.Load(botID)
	if err != nil {
		t.Fatalf("Reload Load() error = %v", err)
	}

	wl := store2.GetWhitelist(botID)
	if len(wl) != 1 || wl[0] != "Bash:rm.*-rf" {
		t.Errorf("GetWhitelist() = %v, want [Bash:rm.*-rf]", wl)
	}
}

func TestFilePermissionStore_AddRemove(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "TESTBOT"

	_ = store.Load(botID)

	err := store.AddWhitelist(botID, "Bash:rm.*-rf", "user1")
	if err != nil {
		t.Fatalf("AddWhitelist() error = %v", err)
	}

	wl := store.GetWhitelist(botID)
	if len(wl) != 1 {
		t.Fatalf("expected 1 whitelist entry, got %d", len(wl))
	}

	err = store.RemoveWhitelist(botID, "Bash:rm.*-rf")
	if err != nil {
		t.Fatalf("RemoveWhitelist() error = %v", err)
	}

	wl = store.GetWhitelist(botID)
	if len(wl) != 0 {
		t.Errorf("expected 0 whitelist entries after remove, got %d", len(wl))
	}

	// Blacklist
	err = store.AddBlacklist(botID, "Bash:chmod 777", "user1")
	if err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}
	bl := store.GetBlacklist(botID)
	if len(bl) != 1 {
		t.Errorf("expected 1 blacklist entry, got %d", len(bl))
	}
}

func TestFilePermissionStore_IsAllowed(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "TESTBOT"

	_ = store.Load(botID)
	_ = store.AddWhitelist(botID, "Bash:rm.*-rf", "user1")
	_ = store.AddBlacklist(botID, "Bash:chmod 777", "user1")

	// Whitelist match
	allowed, why := store.IsAllowed(botID, "Bash", "rm -rf /tmp/test")
	if !allowed || why != "whitelist" {
		t.Errorf("IsAllowed(whitelist) = (%v, %q), want (true, whitelist)", allowed, why)
	}

	// Blacklist match
	allowed, why = store.IsAllowed(botID, "Bash", "chmod 777 /etc/passwd")
	if allowed || why != "blacklist" {
		t.Errorf("IsAllowed(blacklist) = (%v, %q), want (false, blacklist)", allowed, why)
	}

	// No match
	allowed, why = store.IsAllowed(botID, "Bash", "echo hello")
	if allowed || why != "" {
		t.Errorf("IsAllowed(nomatch) = (%v, %q), want (false, \"\")", allowed, why)
	}
}

func TestFilePermissionStore_DuplicatePattern(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "TESTBOT"
	_ = store.Load(botID)

	// Add same pattern twice
	_ = store.AddWhitelist(botID, "Bash:rm", "user1")
	_ = store.AddWhitelist(botID, "Bash:rm", "user2") // duplicate

	wl := store.GetWhitelist(botID)
	if len(wl) != 1 {
		t.Errorf("duplicate pattern should not create second entry, got %d", len(wl))
	}
}

func TestFilePermissionStore_FilePath(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	_ = store.Load("MYBOT")

	// Verify file path format
	store.mu.RLock()
	_, ok := store.data["MYBOT"]
	store.mu.RUnlock()

	if !ok {
		t.Error("bot data not loaded")
	}
}

func TestFilePermissionStore_FileWritten(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "TESTBOT"
	_ = store.Load(botID)
	_ = store.AddWhitelist(botID, "Bash:echo", "user1")
	_ = store.Save(botID)

	// Verify file exists
	path := filepath.Join(dir, botID, "permissions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("permissions.json not written: %v", err)
	}
	if len(data) == 0 {
		t.Error("permissions.json is empty")
	}
}

func TestFilePermissionStore_RemoveBlacklist(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "TESTBOT"
	_ = store.Load(botID)

	_ = store.AddBlacklist(botID, "Bash:rm -rf /", "user1")
	_ = store.AddBlacklist(botID, "Bash:wget", "user2")

	bl := store.GetBlacklist(botID)
	if len(bl) != 2 {
		t.Fatalf("expected 2 blacklist entries, got %d", len(bl))
	}

	// Remove one
	err := store.RemoveBlacklist(botID, "Bash:rm -rf /")
	if err != nil {
		t.Fatalf("RemoveBlacklist() error = %v", err)
	}

	bl = store.GetBlacklist(botID)
	if len(bl) != 1 {
		t.Errorf("expected 1 blacklist entry after remove, got %d", len(bl))
	}

	// Remove non-existent (no-op)
	err = store.RemoveBlacklist(botID, "Bash:nonexistent")
	if err != nil {
		t.Errorf("RemoveBlacklist(non-existent) error = %v, want nil", err)
	}

	// Remove last entry
	err = store.RemoveBlacklist(botID, "Bash:wget")
	if err != nil {
		t.Fatalf("RemoveBlacklist(last) error = %v", err)
	}
	bl = store.GetBlacklist(botID)
	if len(bl) != 0 {
		t.Errorf("expected 0 blacklist entries after removing all, got %d", len(bl))
	}
}

func TestFilePermissionStore_SaveWithoutLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "UNLOADED_BOT"

	// Save without Load should return error
	err := store.Save(botID)
	if err == nil {
		t.Error("Save() without Load() expected error, got nil")
	}
}

func TestFilePermissionStore_GetOrCreate_ExistingEntry(t *testing.T) {
	// Test the "get" branch of getOrCreate (entry already exists)
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "EXISTING_BOT"
	_ = store.Load(botID)

	// getOrCreate should return existing entry (not create new one)
	// This is implicitly tested via AddWhitelist path
	_ = store.AddWhitelist(botID, "Bash:ls", "user1")
	_ = store.AddWhitelist(botID, "Bash:pwd", "user2") // existing entry retrieved

	wl := store.GetWhitelist(botID)
	if len(wl) != 2 {
		t.Errorf("expected 2 whitelist entries, got %d", len(wl))
	}
}

func TestFilePermissionStore_GetOrCreate_NewEntry(t *testing.T) {
	// Test the "create" branch of getOrCreate (entry doesn't exist)
	dir := t.TempDir()
	store := NewFilePermissionStore(dir)
	botID := "NEW_BOT"

	// getOrCreate should create new entry without calling Load first
	_ = store.AddWhitelist(botID, "Bash:ls", "user1")

	wl := store.GetWhitelist(botID)
	if len(wl) != 1 {
		t.Errorf("expected 1 whitelist entry, got %d", len(wl))
	}
}
