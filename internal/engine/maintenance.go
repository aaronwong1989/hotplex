package engine

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// clearClaudeJSONUserID removes the userID field from ~/.claude.json when:
//   - userID exists in ~/.claude.json
//   - credentials.json does NOT exist (OAuth is not fully configured)
//
// Background: Claude Code 2.1.81 introduced a behavior change where userID in
// ~/.claude.json triggers OAuth authentication. In containerized environments,
// OAuth cannot complete (no browser), causing "Not logged in · Please run /login".
// Removing orphaned userID forces Claude Code to use ANTHROPIC_AUTH_TOKEN=PROXY_MANAGED
// from settings.json, which routes correctly through the local proxy.
//
// In valid OAuth setups (userID + credentials.json), userID is preserved.
//
// Controlled by HOTPLEX_CLAUDE_CLEAR_USERID=true (default) or false to disable.
func clearClaudeJSONUserID(logger *slog.Logger) {
	// Allow operators to disable this cleanup
	if os.Getenv("HOTPLEX_CLAUDE_CLEAR_USERID") == "false" {
		logger.Debug("HOTPLEX_CLAUDE_CLEAR_USERID=false, skipping userID cleanup")
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("Failed to get home directory for Claude config cleanup", "error", err)
		return
	}

	claudeJSONPath := filepath.Join(homeDir, ".claude.json")
	credentialsPath := filepath.Join(homeDir, ".claude", "credentials.json")

	// Read existing file
	data, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("Claude config not found, skipping userID cleanup", "path", claudeJSONPath)
			return
		}
		logger.Warn("Failed to read Claude config", "path", claudeJSONPath, "error", err)
		return
	}

	// Parse JSON
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.Warn("Failed to parse Claude config as JSON, skipping userID cleanup",
			"path", claudeJSONPath, "error", err)
		return
	}

	// Check if userID exists
	if _, exists := cfg["userID"]; !exists {
		return
	}

	// Check if credentials.json exists — if so, this is a valid OAuth setup; keep userID
	if _, err := os.Stat(credentialsPath); err == nil {
		logger.Debug("Valid OAuth setup detected (credentials.json exists), preserving userID",
			"userID_path", claudeJSONPath,
			"credentials_path", credentialsPath)
		return
	}

	// credentials.json does not exist — userID is orphaned, clear it
	delete(cfg, "userID")

	// Add hasCompletedOnboarding to prevent onboarding prompts
	cfg["hasCompletedOnboarding"] = json.RawMessage("true")

	cleaned, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Warn("Failed to serialize cleaned Claude config", "error", err)
		return
	}

	if err := os.WriteFile(claudeJSONPath, cleaned, 0644); err != nil {
		logger.Warn("Failed to write cleaned Claude config", "path", claudeJSONPath, "error", err)
		return
	}

	logger.Info("Cleared orphaned userID and set hasCompletedOnboarding=true (no credentials.json found)",
		"path", claudeJSONPath)
}
