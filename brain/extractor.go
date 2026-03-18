package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ExtractedConfig represents configuration pulled from a CLI tool.
type ExtractedConfig struct {
	APIKey   string
	Endpoint string
	Model    string
}

// ConfigExtractor defines the interface for extracting config from external CLI tools.
type ConfigExtractor interface {
	Extract() (*ExtractedConfig, error)
}

// ClaudeCodeExtractor extracts config from Claude Code CLI settings.
type ClaudeCodeExtractor struct {
	ConfigPath string
}

// NewClaudeCodeExtractor creates a new extractor for Claude Code CLI.
func NewClaudeCodeExtractor() *ClaudeCodeExtractor {
	home, _ := os.UserHomeDir()
	return &ClaudeCodeExtractor{
		ConfigPath: filepath.Join(home, ".claude", "settings.json"),
	}
}

// settingsJSON represents the structure of ~/.claude/settings.json
type settingsJSON struct {
	Env   map[string]string `json:"env"`
	Model string            `json:"model"`
}

func (e *ClaudeCodeExtractor) Extract() (*ExtractedConfig, error) {
	data, err := os.ReadFile(e.ConfigPath)
	if err != nil {
		return nil, err
	}

	var settings settingsJSON
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	config := &ExtractedConfig{}

	// Extract API Key / Auth Token
	if token, ok := settings.Env["ANTHROPIC_AUTH_TOKEN"]; ok {
		if token == "PROXY_MANAGED" {
			// If it's proxy managed, we generate a dummy key in the correct format
			// to satisfy the client initialization, as the actual auth is handled by the proxy.
			config.APIKey = "sk-ant-managed-dummy-000000000000000000000000000000000000000000000000000000"
		} else {
			config.APIKey = token
		}
	} else if key, ok := settings.Env["ANTHROPIC_API_KEY"]; ok {
		config.APIKey = key
	}

	// Extract Endpoint
	if baseURL, ok := settings.Env["ANTHROPIC_BASE_URL"]; ok {
		config.Endpoint = baseURL
	}

	// Extract Model
	if settings.Model != "" {
		config.Model = settings.Model
	}

	return config, nil
}
