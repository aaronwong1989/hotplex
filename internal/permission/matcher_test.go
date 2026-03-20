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

	// Add blacklist — use pattern that whitelist does NOT match
	_ = m.AddBlacklist(botID, "Bash:chmod 777", "user1")
	if d := m.Check(botID, "Bash", "chmod 777 /etc/passwd"); d != DecisionDeny {
		t.Errorf("Check(blacklist) = %v, want DecisionDeny", d)
	}

	// Whitelist pattern does NOT match blacklist command
	if d := m.Check(botID, "Bash", "chmod 777 /tmp"); d != DecisionDeny {
		t.Errorf("Check(blacklist only) = %v, want DecisionDeny", d)
	}

	// Whitelist still allows its own commands
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

func TestPermissionMatcher_RemoveWhitelist(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)
	botID := "TESTBOT"

	_ = m.AddWhitelist(botID, "Bash:rm.*-rf", "user1")
	_ = m.AddWhitelist(botID, "Bash:chmod", "user2")

	// Both should be allowed
	if d := m.Check(botID, "Bash", "rm -rf /tmp"); d != DecisionAllow {
		t.Errorf("Check before remove = %v, want DecisionAllow", d)
	}

	// Remove one
	_ = m.RemoveWhitelist(botID, "Bash:rm.*-rf")

	// Removed pattern no longer allowed
	if d := m.Check(botID, "Bash", "rm -rf /tmp"); d != DecisionUnknown {
		t.Errorf("Check after remove = %v, want DecisionUnknown", d)
	}

	// Other pattern still allowed
	if d := m.Check(botID, "Bash", "chmod 777"); d != DecisionAllow {
		t.Errorf("Check other pattern = %v, want DecisionAllow", d)
	}
}

func TestPermissionMatcher_RemoveBlacklist(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)
	botID := "TESTBOT"

	_ = m.AddBlacklist(botID, "Bash:rm -rf /", "user1")
	_ = m.AddBlacklist(botID, "Bash:wget", "user2")

	// Both should be denied
	if d := m.Check(botID, "Bash", "rm -rf /tmp"); d != DecisionDeny {
		t.Errorf("Check before remove = %v, want DecisionDeny", d)
	}

	// Remove one
	_ = m.RemoveBlacklist(botID, "Bash:rm -rf /")

	// Removed pattern no longer denied
	if d := m.Check(botID, "Bash", "rm -rf /tmp"); d != DecisionUnknown {
		t.Errorf("Check after remove = %v, want DecisionUnknown", d)
	}

	// Other pattern still denied
	if d := m.Check(botID, "Bash", "wget http://bad"); d != DecisionDeny {
		t.Errorf("Check other pattern = %v, want DecisionDeny", d)
	}
}

func TestPermissionMatcher_GetWhitelistGetBlacklist(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)
	botID := "TESTBOT"

	_ = m.AddWhitelist(botID, "Bash:echo", "user1")
	_ = m.AddBlacklist(botID, "Bash:rm", "user2")

	wl := m.GetWhitelist(botID)
	if len(wl) != 1 || wl[0] != "Bash:echo" {
		t.Errorf("GetWhitelist() = %v, want [Bash:echo]", wl)
	}

	bl := m.GetBlacklist(botID)
	if len(bl) != 1 || bl[0] != "Bash:rm" {
		t.Errorf("GetBlacklist() = %v, want [Bash:rm]", bl)
	}
}

func TestPermissionMatcher_IsAllowed(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)
	botID := "TESTBOT"

	_ = m.AddWhitelist(botID, "Bash:echo", "user1")
	_ = m.AddBlacklist(botID, "Bash:rm -rf", "user2")

	allowed, why := m.IsAllowed(botID, "Bash", "echo hello")
	if !allowed || why != "whitelist" {
		t.Errorf("IsAllowed(whitelist) = (%v, %q), want (true, whitelist)", allowed, why)
	}

	allowed, why = m.IsAllowed(botID, "Bash", "rm -rf /tmp")
	if allowed || why != "blacklist" {
		t.Errorf("IsAllowed(blacklist) = (%v, %q), want (false, blacklist)", allowed, why)
	}

	allowed, why = m.IsAllowed(botID, "Bash", "ls -la")
	if allowed || why != "" {
		t.Errorf("IsAllowed(nomatch) = (%v, %q), want (false, \"\")", allowed, why)
	}
}

func TestPermissionMatcher_IsAllowed_NoStore(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)

	// Bot with no store loaded — GetWhitelist/GetBlacklist return empty slice
	// (Load lazily creates empty entry on first access)
	wl := m.GetWhitelist("NOSTORE_BOT")
	if len(wl) != 0 {
		t.Errorf("GetWhitelist(no store) len = %d, want 0", len(wl))
	}

	bl := m.GetBlacklist("NOSTORE_BOT")
	if len(bl) != 0 {
		t.Errorf("GetBlacklist(no store) len = %d, want 0", len(bl))
	}

	// IsAllowed on uninitialized bot returns false, ""
	allowed, why := m.IsAllowed("NOSTORE_BOT", "Bash", "echo")
	if allowed || why != "" {
		t.Errorf("IsAllowed(no store) = (%v, %q), want (false, \"\")", allowed, why)
	}
}

func TestPermissionMatcher_Save_NoStore(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)

	// Save without store should be no-op (not error)
	err := m.Save("NOSTORE_BOT")
	if err != nil {
		t.Errorf("Save(no store) error = %v, want nil", err)
	}
}

func TestPermissionMatcher_Remove_NoStore(t *testing.T) {
	dir := t.TempDir()
	m := NewPermissionMatcher(dir)

	// Remove on non-existent store should be no-op
	err := m.RemoveWhitelist("NOSTORE_BOT", "Bash:anything")
	if err != nil {
		t.Errorf("RemoveWhitelist(no store) error = %v, want nil", err)
	}

	err = m.RemoveBlacklist("NOSTORE_BOT", "Bash:anything")
	if err != nil {
		t.Errorf("RemoveBlacklist(no store) error = %v, want nil", err)
	}
}
