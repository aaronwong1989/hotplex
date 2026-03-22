package base

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/hrygo/hotplex/provider"
)

// PlatformConfig contains platform-specific configuration loaded from YAML.
// Defined here to allow platform factories in subpackages to reference it
// without import cycles.
type PlatformConfig struct {
	Inherits         string                      `yaml:"inherits"`
	Platform         string                      `yaml:"platform"`
	Mode             string                      `yaml:"mode"`
	SystemPrompt     string                      `yaml:"system_prompt"`
	TaskInstructions string                      `yaml:"task_instructions"`
	Engine           EngineConfig                `yaml:"engine"`
	Provider         provider.ProviderConfig     `yaml:"provider"`
	Security         SecurityConfig              `yaml:"security"`
	Features         FeaturesConfig              `yaml:"features"`
	Session          SessionConfig               `yaml:"session"`
	MessageStore     MessageStoreConfig          `yaml:"message_store,omitempty"`
	Options          map[string]any             `yaml:"options,omitempty"`
	SourceFile       string                      `yaml:"-"`
}

type SecurityConfig struct {
	VerifySignature *bool             `yaml:"verify_signature"`
	Permission      PermissionConfig  `yaml:"permission"`
	Owner          *OwnerConfig      `yaml:"owner,omitempty"`
}

type PermissionConfig struct {
	DMPolicy              string                 `yaml:"dm_policy"`
	GroupPolicy           string                 `yaml:"group_policy"`
	BotUserID             string                 `yaml:"bot_user_id"`
	BroadcastResponse     string                 `yaml:"broadcast_response"`
	AllowedUsers          []string               `yaml:"allowed_users"`
	BlockedUsers          []string               `yaml:"blocked_users"`
	SlashCommandRateLimit float64                `yaml:"slash_command_rate_limit"`
	ThreadOwnership       *ThreadOwnershipConfig `yaml:"thread_ownership,omitempty"`
}

type FeaturesConfig struct {
	Chunking  ChunkingConfig  `yaml:"chunking"`
	Threading ThreadingConfig `yaml:"threading"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Markdown  MarkdownConfig  `yaml:"markdown"`
}

type ChunkingConfig struct {
	Enabled  *bool `yaml:"enabled"`
	MaxChars int   `yaml:"max_chars"`
}

type ThreadingConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type RateLimitConfig struct {
	Enabled     *bool `yaml:"enabled"`
	MaxAttempts int   `yaml:"max_attempts"`
	BaseDelayMs int   `yaml:"base_delay_ms"`
	MaxDelayMs  int   `yaml:"max_delay_ms"`
}

type MarkdownConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type OwnerConfig struct {
	Primary string   `yaml:"primary"`
	Trusted []string `yaml:"trusted_users"`
	Policy  string   `yaml:"policy"`
}

type ThreadOwnershipConfig struct {
	Enabled *bool        `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
	Persist *bool        `yaml:"persist"`
}

type SessionConfig struct {
	Timeout         time.Duration `yaml:"timeout"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

type EngineConfig struct {
	Timeout         time.Duration `yaml:"timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	WorkDir         string        `yaml:"work_dir"`
	AllowedTools    []string      `yaml:"allowed_tools"`
	DisallowedTools []string      `yaml:"disallowed_tools"`
}

type MessageStoreConfig struct {
	Enabled   *bool          `yaml:"enabled"`
	Type      string         `yaml:"type"`
	SQLite    SQLiteConfig   `yaml:"sqlite"`
	Postgres  PostgresConfig `yaml:"postgres"`
	Strategy  string         `yaml:"strategy"`
	Streaming StreamingConfig `yaml:"streaming"`
}

type SQLiteConfig struct {
	Path      string `yaml:"path"`
	MaxSizeMB int    `yaml:"max_size_mb"`
}

type PostgresConfig struct {
	DSN            string `yaml:"dsn"`
	MaxConnections int    `yaml:"max_connections"`
	Level          int    `yaml:"level"`
}

type StreamingConfig struct {
	Enabled       *bool        `yaml:"enabled"`
	BufferSize    int          `yaml:"buffer_size"`
	Timeout       time.Duration `yaml:"timeout"`
	StoragePolicy string       `yaml:"storage_policy"`
}

// BoolValue returns the bool value if ptr is non-nil, otherwise returns defaultVal.
func BoolValue(ptr *bool, defaultVal bool) bool {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// AdapterFactory defines the interface for platform adapter factories.
type AdapterFactory interface {
	Platform() string
	RequiredEnvVars() []string
	New(pc *PlatformConfig) any
	PostSetup(ctx context.Context, adapter, setupCtx any)
}

// SetupContext holds shared context for factory PostSetup.
type SetupContext struct {
	Manager        any // *AdapterManager
	Loader         any // *ConfigLoader
	Engine         any // *engine.Engine
	PermDir        string
	Logger         *slog.Logger
	WorkDirFn      func(sessionID string) string
	Platform       string
	PlatformConfig *PlatformConfig
}

// AdapterPluginRegistry manages adapter factory registration.
type AdapterPluginRegistry struct {
	mu        sync.RWMutex
	factories map[string]AdapterFactory
}

var (
	globalRegistry     *AdapterPluginRegistry
	globalRegistryOnce sync.Once
)

// GlobalAdapterRegistry returns the global adapter factory registry.
func GlobalAdapterRegistry() *AdapterPluginRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = &AdapterPluginRegistry{
			factories: make(map[string]AdapterFactory),
		}
	})
	return globalRegistry
}

// Register registers a factory.
func (r *AdapterPluginRegistry) Register(factory AdapterFactory) {
	platform := factory.Platform()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[platform]; exists {
		panic("adapter factory already registered for platform: " + platform)
	}
	r.factories[platform] = factory
}

// Get returns the factory for a platform.
func (r *AdapterPluginRegistry) Get(platform string) (AdapterFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[platform]
	return factory, ok
}

// List returns all registered platform names.
func (r *AdapterPluginRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// HasRequiredEnvVars checks if all required environment variables are set.
func HasRequiredEnvVars(required []string) bool {
	for _, envVar := range required {
		if os.Getenv(envVar) == "" {
			return false
		}
	}
	return true
}
