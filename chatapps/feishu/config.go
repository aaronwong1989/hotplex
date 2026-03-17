package feishu

// Config holds Feishu (Lark) adapter configuration
type Config struct {
	// App credentials
	AppID     string `json:"app_id" yaml:"app_id"`
	AppSecret string `json:"app_secret" yaml:"app_secret"`

	// Event subscription
	VerificationToken string `json:"verification_token" yaml:"verification_token"`
	EncryptKey        string `json:"encrypt_key" yaml:"encrypt_key"`

	// Server configuration
	ServerAddr    string `json:"server_addr" yaml:"server_addr"`
	MaxMessageLen int    `json:"max_message_len" yaml:"max_message_len"`

	// WebSocket configuration
	UseWebSocket bool `json:"use_websocket" yaml:"use_websocket"` // 启用 WebSocket 长连接模式

	// System prompt
	SystemPrompt string `json:"system_prompt" yaml:"system_prompt"`
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.AppID == "" {
		return ErrConfigMissingAppID
	}
	if c.AppSecret == "" {
		return ErrConfigMissingAppSecret
	}
	if c.VerificationToken == "" {
		return ErrConfigMissingVerificationToken
	}
	if c.EncryptKey == "" {
		return ErrConfigMissingEncryptKey
	}
	if c.ServerAddr == "" {
		c.ServerAddr = ":8082" // Default Feishu port
	}
	if c.MaxMessageLen <= 0 {
		c.MaxMessageLen = 4096 // Default Feishu limit
	}
	return nil
}
