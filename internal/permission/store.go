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
	GetWhitelist(botID string) []string
	GetBlacklist(botID string) []string
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
	defer s.mu.Unlock()

	pf, ok := s.data[botID]
	if !ok {
		return fmt.Errorf("botID %q not loaded", botID)
	}
	// Deep-copy under write lock to avoid data races during marshal.
	// Hold lock for entire operation to prevent write-write races:
	// between Unlock() and os.WriteFile, another goroutine could call
	// AddWhitelist/AddBlacklist (holding Lock), modify s.data, then Save()
	// again, causing this write's data to be lost.
	pfCopy := &PermissionsFile{
		BotID:     pf.BotID,
		Whitelist: append([]PatternEntry{}, pf.Whitelist...),
		Blacklist: append([]PatternEntry{}, pf.Blacklist...),
	}

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
		p := Pattern{Value: e.Pattern}
		if p.Match(tool, input) {
			return true, "whitelist"
		}
	}

	// Check blacklist
	for _, e := range pf.Blacklist {
		p := Pattern{Value: e.Pattern}
		if p.Match(tool, input) {
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
