package slack

import "fmt"

type Config struct {
	BotToken      string
	AppToken      string
	SigningSecret string
	SystemPrompt  string
	// Mode: "http" (default) or "socket" for WebSocket connection
	Mode string
}

// Validate checks the configuration based on the selected mode
func (c *Config) Validate() error {
	if c.BotToken == "" {
		return fmt.Errorf("bot token is required")
	}

	switch c.Mode {
	case "", "http":
		// HTTP Mode requires SigningSecret
		if c.SigningSecret == "" {
			return fmt.Errorf("signing secret is required for HTTP mode")
		}
	case "socket":
		// Socket Mode requires AppToken
		if c.AppToken == "" {
			return fmt.Errorf("app token is required for Socket mode")
		}
	default:
		return fmt.Errorf("invalid mode: %s (use 'http' or 'socket')", c.Mode)
	}

	return nil
}

// IsSocketMode returns true if Socket Mode is enabled
func (c *Config) IsSocketMode() bool {
	return c.Mode == "socket"
}
