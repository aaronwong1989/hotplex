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
