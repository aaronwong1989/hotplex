// Package brain provides intelligent orchestration capabilities for HotPlex.
// This file (config.go) defines configuration structures loaded from environment variables.
//
// # Configuration Hierarchy
//
//	Config (root)
//	├── Model (LLM backend settings)
//	├── Cache (response caching)
//	├── Retry (retry policy)
//	├── Metrics (observability)
//	├── Cost (cost tracking)
//	├── RateLimit (throttling)
//	├── Router (model routing)
//	├── CircuitBreaker (fault tolerance)
//	├── Failover (provider failover)
//	├── Budget (budget limits)
//	├── Priority (request prioritization)
//	├── IntentRouter (message classification)
//	├── Memory (context compression)
//	└── Guard (safety guardrails)
//
// # Environment Variables
//
// All config is loaded from environment variables with prefix HOTPLEX_BRAIN_.
// See LoadConfigFromEnv() for the full list of variables.
package brain

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hrygo/hotplex/brain/llm"
)

// === Model Configuration ===

// ModelConfig configures the LLM backend for Brain operations.
type ModelConfig struct {
	Provider string // LLM provider identifier (e.g., "openai", "anthropic", "siliconflow")
	Protocol string // Protocol to use: "openai" or "anthropic"
	APIKey   string // API Key for the provider
	Model    string // Model name: "gpt-4o", "claude-3-7-sonnet", etc.
	Endpoint string // Custom API endpoint (optional)
	TimeoutS int    // Request timeout in seconds
}

// === Cache Configuration ===

// CacheConfig configures response caching for repeated queries.
type CacheConfig struct {
	Enabled bool // Enable response caching
	Size    int  // Maximum cache entries
}

// === Retry Configuration ===

// RetryConfig configures retry behavior for transient failures.
type RetryConfig struct {
	Enabled     bool // Enable retry mechanism
	MaxAttempts int  // Maximum retry attempts
	MinWaitMs   int  // Minimum wait between retries (milliseconds)
	MaxWaitMs   int  // Maximum wait between retries (milliseconds)
}

// === Metrics Configuration ===

// MetricsConfig configures observability and metrics export.
type MetricsConfig struct {
	Enabled        bool          // Enable metrics collection
	ServiceName    string        // Service name for metrics identification
	Endpoint       string        // Metrics export endpoint (e.g., OTLP collector)
	ExportInterval time.Duration // Interval for periodic metric export
}

// === Cost Configuration ===

// CostConfig configures cost tracking for LLM API calls.
type CostConfig struct {
	Enabled      bool // Enable cost tracking
	EnableBudget bool // Enable budget enforcement
}

// === Rate Limit Configuration ===

// RateLimitConfig configures request rate limiting.
type RateLimitConfig struct {
	Enabled      bool          // Enable rate limiting
	RPS          float64       // Requests per second limit
	Burst        int           // Burst capacity (token bucket)
	QueueSize    int           // Queue size for waiting requests
	QueueTimeout time.Duration // Max wait time in queue
	PerModel     bool          // Apply limit per-model instead of global
}

// === Router Configuration ===

// RouterConfig configures intelligent model routing.
type RouterConfig struct {
	Enabled      bool              // Enable model routing
	DefaultStage string            // Default routing strategy: "cost_priority", "latency_priority"
	Models       []llm.ModelConfig // Available models with cost/latency info
}

// === Circuit Breaker Configuration ===

// CircuitBreakerConfig configures circuit breaker for fault tolerance.
type CircuitBreakerConfig struct {
	Enabled     bool          // Enable circuit breaker
	MaxFailures int           // Failures before opening circuit
	Timeout     time.Duration // Time before attempting to close circuit
	Interval    time.Duration // Interval for resetting failure count
}

// === Failover Configuration ===

// FailoverConfig configures provider failover behavior.
type FailoverConfig struct {
	Enabled        bool                 // Enable failover
	Providers      []llm.ProviderConfig // Backup providers
	EnableAuto     bool                 // Enable automatic failover
	EnableFailback bool                 // Enable automatic failback when primary recovers
	Cooldown       time.Duration        // Cooldown period before failback
}

// === Budget Configuration ===

// BudgetConfig configures budget limits for LLM costs.
type BudgetConfig struct {
	Enabled         bool      // Enable budget tracking
	Period          string    // Budget period: "daily", "weekly", "monthly"
	Limit           float64   // Budget limit in USD
	EnableHardLimit bool      // Block requests when budget exceeded
	AlertThresholds []float64 // Alert at these percentages (e.g., [0.5, 0.8, 0.9])
}

// === Priority Configuration ===

// PriorityConfig configures request prioritization.
type PriorityConfig struct {
	Enabled               bool // Enable priority queuing
	MaxQueueSize          int  // Maximum queued requests
	EnableLowPriorityDrop bool // Drop low-priority requests when queue full
	HighPriorityReserve   int  // Reserved slots for high-priority requests
}

// === Intent Router Configuration ===

// IntentRouterFeatureConfig configures intent routing features.
type IntentRouterFeatureConfig struct {
	Enabled             bool    `json:"enabled"`              // Enable intent routing
	ConfidenceThreshold float64 `json:"confidence_threshold"` // Minimum confidence for classification
	CacheSize           int     `json:"cache_size"`           // Cache size for classification results
}

// === Memory Compression Configuration ===

// MemoryCompressionConfig configures context compression.
type MemoryCompressionConfig struct {
	Enabled          bool    // Enable context compression
	TokenThreshold   int     // Trigger compression at this token count
	TargetTokenCount int     // Target tokens after compression
	PreserveTurns    int     // Recent turns to preserve during compression
	MaxSummaryTokens int     // Maximum tokens for summary
	CompressionRatio float64 // Target compression ratio (0.0-1.0)
	SessionTTL       string  // Session time-to-live (e.g., "24h")
}

// === Safety Guard Configuration ===

// SafetyGuardFeatureConfig configures safety guardrails.
type SafetyGuardFeatureConfig struct {
	Enabled            bool          // Enable safety guard
	InputGuardEnabled  bool          // Enable input validation
	OutputGuardEnabled bool          // Enable output sanitization
	Chat2ConfigEnabled bool          // Enable natural language config changes (security risk)
	MaxInputLength     int           // Maximum input length
	ScanDepth          int           // Depth for nested context scanning
	Sensitivity        string        // Detection sensitivity: "low", "medium", "high"
	AdminUsers         []string      // User IDs with admin privileges
	AdminChannels      []string      // Channel IDs with admin privileges
	ResponseTimeout    time.Duration // Timeout for Brain API calls
	RateLimitRPS       float64       // Requests per second per user (0 = disabled)
	RateLimitBurst     int           // Burst capacity per user
}

// === Main Config ===

// Config holds the configuration for the Global Brain.
// It aggregates all sub-configurations for the Brain system.
//
// # Auto-Enable Logic
//
// Config.Enabled is automatically set based on APIKey presence:
//   - HOTPLEX_BRAIN_API_KEY present → Enabled = true
//   - HOTPLEX_BRAIN_API_KEY absent → Enabled = false
//
// This allows graceful degradation when Brain is not configured.
type Config struct {
	// Enabled is automatically determined based on APIKey presence.
	Enabled bool
	// Model is the model configuration.
	Model ModelConfig
	// Cache is the cache configuration.
	Cache CacheConfig
	// Retry is the retry configuration.
	Retry RetryConfig
	// Metrics is the metrics configuration.
	Metrics MetricsConfig
	// Cost is the cost configuration.
	Cost CostConfig
	// RateLimit is the rate limit configuration.
	RateLimit RateLimitConfig
	// Router is the router configuration.
	Router RouterConfig
	// CircuitBreaker is the circuit breaker configuration.
	CircuitBreaker CircuitBreakerConfig
	// Failover is the failover configuration.
	Failover FailoverConfig
	// Budget is the budget configuration.
	Budget BudgetConfig
	// Priority is the priority configuration.
	Priority PriorityConfig
	// IntentRouter is the intent router feature configuration.
	IntentRouter IntentRouterFeatureConfig
	// Memory is the memory compression feature configuration.
	Memory MemoryCompressionConfig
	// Guard is the safety guard feature configuration.
	Guard SafetyGuardFeatureConfig
}

// LoadConfigFromEnv loads the brain configuration from environment variables.
func LoadConfigFromEnv() Config {
	// 1. Primary check: Direct brain configuration
	apiKey := os.Getenv("HOTPLEX_BRAIN_API_KEY")
	provider := getEnv("HOTPLEX_BRAIN_PROVIDER", "openai")
	protocol := getEnv("HOTPLEX_BRAIN_PROTOCOL", "openai")
	model := getEnv("HOTPLEX_BRAIN_MODEL", "gpt-4o") // 2026 Best Practice: Use full 4o/5 class for reasoning by default
	endpoint := os.Getenv("HOTPLEX_BRAIN_ENDPOINT")

	// 2. Secondary check: Link with global provider and CLI extraction
	if apiKey == "" {
		providerType := os.Getenv("HOTPLEX_PROVIDER_TYPE")
		if providerType == "" {
			providerType = provider // Default to openai
		}

		// Try CLI Extractor first (Priority 2)
		var extracted *ExtractedConfig
		switch providerType {
		case "claude-code", "anthropic":
			extracted, _ = NewClaudeCodeExtractor().Extract()
		}

		if extracted != nil {
			if extracted.APIKey != "" {
				apiKey = extracted.APIKey
				// Update provider/protocol based on extraction source
				if providerType == "claude-code" || providerType == "anthropic" {
					provider = "anthropic"
					protocol = "anthropic"
				}
			}
			if endpoint == "" && extracted.Endpoint != "" {
				endpoint = extracted.Endpoint
			}
			if (model == "" || model == "gpt-4o") && extracted.Model != "" {
				model = extracted.Model
			}
		}

		// Fallback to System Environment Variables (Priority 3)
		if apiKey == "" {
			switch providerType {
			case "claude-code", "anthropic":
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
				provider = "anthropic"
				protocol = "anthropic"
				if model == "gpt-4o" {
					model = getEnv("HOTPLEX_PROVIDER_MODEL", "claude-3-7-sonnet-latest")
				}
				if endpoint == "" {
					endpoint = os.Getenv("ANTHROPIC_BASE_URL")
				}
			case "openai":
				apiKey = os.Getenv("OPENAI_API_KEY")
				provider = "openai"
				protocol = "openai"
				if model == "gpt-4o" {
					model = getEnv("HOTPLEX_PROVIDER_MODEL", "gpt-4o")
				}
				if endpoint == "" {
					endpoint = os.Getenv("OPENAI_BASE_URL")
				}
			case "siliconflow":
				apiKey = os.Getenv("SILICONFLOW_API_KEY")
				provider = "openai"
				protocol = "openai"
				if model == "gpt-4o" {
					model = getEnv("HOTPLEX_PROVIDER_MODEL", "deepseek-ai/DeepSeek-V3")
				}
				if endpoint == "" {
					endpoint = getEnv("SILICONFLOW_BASE_URL", "https://api.siliconflow.cn/v1")
				}
			case "deepseek":
				apiKey = os.Getenv("DEEPSEEK_API_KEY")
				provider = "openai"
				protocol = "openai"
				if model == "gpt-4o" {
					model = getEnv("HOTPLEX_PROVIDER_MODEL", "deepseek-chat")
				}
				if endpoint == "" {
					endpoint = getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
				}
			}
		}
	}

	return Config{
		Enabled: apiKey != "",
		Model: ModelConfig{
			Provider: provider,
			Protocol: protocol,
			APIKey:   apiKey,
			Model:    model,
			Endpoint: endpoint,
			TimeoutS: getIntEnv("HOTPLEX_BRAIN_TIMEOUT_S", 30), // 2026 Best Practice: 30s to allow for deep reasoning models
		},
		Cache: CacheConfig{
			Enabled: true,
			Size:    getIntEnv("HOTPLEX_BRAIN_CACHE_SIZE", 1000),
		},
		Retry: RetryConfig{
			Enabled:     true,
			MaxAttempts: getIntEnv("HOTPLEX_BRAIN_MAX_RETRIES", 3),
			MinWaitMs:   getIntEnv("HOTPLEX_BRAIN_RETRY_MIN_WAIT_MS", 100),
			MaxWaitMs:   getIntEnv("HOTPLEX_BRAIN_RETRY_MAX_WAIT_MS", 5000),
		},
		Metrics: MetricsConfig{
			Enabled:        getBoolEnv("HOTPLEX_BRAIN_METRICS_ENABLED", true),
			ServiceName:    getEnv("HOTPLEX_BRAIN_METRICS_SERVICE_NAME", "hotplex-brain"),
			ExportInterval: getDurationEnv("HOTPLEX_BRAIN_METRICS_EXPORT_INTERVAL", 10*time.Second),
		},
		Cost: CostConfig{
			Enabled:      getBoolEnv("HOTPLEX_BRAIN_COST_TRACKING_ENABLED", true),
			EnableBudget: getBoolEnv("HOTPLEX_BRAIN_COST_ENABLE_BUDGET", false),
		},
		RateLimit: RateLimitConfig{
			Enabled:      getBoolEnv("HOTPLEX_BRAIN_RATE_LIMIT_ENABLED", false),
			RPS:          getFloatEnv("HOTPLEX_BRAIN_RATE_LIMIT_RPS", 10.0),
			Burst:        getIntEnv("HOTPLEX_BRAIN_RATE_LIMIT_BURST", 20),
			QueueSize:    getIntEnv("HOTPLEX_BRAIN_RATE_LIMIT_QUEUE_SIZE", 100),
			QueueTimeout: getDurationEnv("HOTPLEX_BRAIN_RATE_LIMIT_QUEUE_TIMEOUT", 30*time.Second),
			PerModel:     getBoolEnv("HOTPLEX_BRAIN_RATE_LIMIT_PER_MODEL", false),
		},
		Router: RouterConfig{
			Enabled:      getBoolEnv("HOTPLEX_BRAIN_ROUTER_ENABLED", false),
			DefaultStage: getEnv("HOTPLEX_BRAIN_ROUTER_STRATEGY", "cost_priority"),
			Models:       parseRouterModels(getEnv("HOTPLEX_BRAIN_ROUTER_MODELS", "")),
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:     getBoolEnv("HOTPLEX_BRAIN_CIRCUIT_BREAKER_ENABLED", false),
			MaxFailures: getIntEnv("HOTPLEX_BRAIN_CIRCUIT_BREAKER_MAX_FAILURES", 5),
			Timeout:     getDurationEnv("HOTPLEX_BRAIN_CIRCUIT_BREAKER_TIMEOUT", 30*time.Second),
			Interval:    getDurationEnv("HOTPLEX_BRAIN_CIRCUIT_BREAKER_INTERVAL", 60*time.Second),
		},
		Failover: FailoverConfig{
			Enabled:        getBoolEnv("HOTPLEX_BRAIN_FAILOVER_ENABLED", false),
			EnableAuto:     getBoolEnv("HOTPLEX_BRAIN_FAILOVER_ENABLE_AUTO", true),
			EnableFailback: getBoolEnv("HOTPLEX_BRAIN_FAILOVER_ENABLE_FAILBACK", true),
			Cooldown:       getDurationEnv("HOTPLEX_BRAIN_FAILOVER_COOLDOWN", 5*time.Minute),
		},
		Budget: BudgetConfig{
			Enabled:         getBoolEnv("HOTPLEX_BRAIN_BUDGET_ENABLED", false),
			Period:          getEnv("HOTPLEX_BRAIN_BUDGET_PERIOD", "daily"),
			Limit:           getFloatEnv("HOTPLEX_BRAIN_BUDGET_LIMIT", 10.0),
			EnableHardLimit: getBoolEnv("HOTPLEX_BRAIN_BUDGET_ENABLE_HARD_LIMIT", false),
		},
		Priority: PriorityConfig{
			Enabled:               getBoolEnv("HOTPLEX_BRAIN_PRIORITY_ENABLED", false),
			MaxQueueSize:          getIntEnv("HOTPLEX_BRAIN_PRIORITY_MAX_QUEUE_SIZE", 1000),
			EnableLowPriorityDrop: getBoolEnv("HOTPLEX_BRAIN_PRIORITY_ENABLE_LOW_PRIORITY_DROP", true),
			HighPriorityReserve:   getIntEnv("HOTPLEX_BRAIN_PRIORITY_HIGH_PRIORITY_RESERVE", 100),
		},
		IntentRouter: IntentRouterFeatureConfig{
			Enabled:             getBoolEnv("HOTPLEX_BRAIN_INTENT_ROUTER_ENABLED", true),
			ConfidenceThreshold: getFloatEnv("HOTPLEX_BRAIN_INTENT_ROUTER_CONFIDENCE", 0.7),
			CacheSize:           getIntEnv("HOTPLEX_BRAIN_INTENT_ROUTER_CACHE_SIZE", 1000),
		},
		Memory: MemoryCompressionConfig{
			Enabled:          getBoolEnv("HOTPLEX_BRAIN_MEMORY_ENABLED", true),
			TokenThreshold:   getIntEnv("HOTPLEX_BRAIN_MEMORY_TOKEN_THRESHOLD", 8000),
			TargetTokenCount: getIntEnv("HOTPLEX_BRAIN_MEMORY_TARGET_TOKENS", 2000),
			PreserveTurns:    getIntEnv("HOTPLEX_BRAIN_MEMORY_PRESERVE_TURNS", 5),
			MaxSummaryTokens: getIntEnv("HOTPLEX_BRAIN_MEMORY_MAX_SUMMARY_TOKENS", 500),
			CompressionRatio: getFloatEnv("HOTPLEX_BRAIN_MEMORY_COMPRESSION_RATIO", 0.25),
			SessionTTL:       getEnv("HOTPLEX_BRAIN_MEMORY_SESSION_TTL", "24h"),
		},
		Guard: SafetyGuardFeatureConfig{
			Enabled:            getBoolEnv("HOTPLEX_BRAIN_GUARD_ENABLED", true),
			InputGuardEnabled:  getBoolEnv("HOTPLEX_BRAIN_GUARD_INPUT_ENABLED", true),
			OutputGuardEnabled: getBoolEnv("HOTPLEX_BRAIN_GUARD_OUTPUT_ENABLED", true),
			Chat2ConfigEnabled: getBoolEnv("HOTPLEX_BRAIN_CHAT2CONFIG_ENABLED", false),
			MaxInputLength:     getIntEnv("HOTPLEX_BRAIN_GUARD_MAX_INPUT_LENGTH", 100000),
			ScanDepth:          getIntEnv("HOTPLEX_BRAIN_GUARD_SCAN_DEPTH", 3),
			Sensitivity:        getEnv("HOTPLEX_BRAIN_GUARD_SENSITIVITY", "medium"),
			AdminUsers:         parseStringList(getEnv("HOTPLEX_BRAIN_ADMIN_USERS", "")),
			AdminChannels:      parseStringList(getEnv("HOTPLEX_BRAIN_ADMIN_CHANNELS", "")),
			ResponseTimeout:    getDurationEnv("HOTPLEX_BRAIN_GUARD_RESPONSE_TIMEOUT", 10*time.Second),
			RateLimitRPS:       getFloatEnv("HOTPLEX_BRAIN_GUARD_RATE_LIMIT_RPS", 10.0),
			RateLimitBurst:     getIntEnv("HOTPLEX_BRAIN_GUARD_RATE_LIMIT_BURST", 20),
		},
	}
}

func parseRouterModels(s string) []llm.ModelConfig {
	if s == "" {
		return nil
	}

	var models []llm.ModelConfig
	parts := strings.Split(s, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Split(part, ":")
		if len(fields) < 5 {
			continue
		}

		costInput, _ := strconv.ParseFloat(fields[2], 64)
		costOutput, _ := strconv.ParseFloat(fields[3], 64)
		latency, _ := strconv.ParseInt(fields[4], 10, 64)

		models = append(models, llm.ModelConfig{
			Name:            fields[0],
			Provider:        fields[1],
			CostPer1KInput:  costInput,
			CostPer1KOutput: costOutput,
			AvgLatencyMs:    latency,
			Enabled:         true,
		})
	}

	return models
}

func parseStringList(s string) []string {
	if s == "" {
		return nil
	}

	var result []string
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// Helper functions for loading config from environment variables

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func getFloatEnv(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		// Try parsing as duration string (e.g., "30s", "1m")
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
		// Try parsing as seconds
		if n, err := strconv.Atoi(val); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}
