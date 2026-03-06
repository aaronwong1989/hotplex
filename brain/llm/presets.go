package llm

import "time"

// Preset configurations for common use cases.

// ProductionClient creates a production-ready LLM client with all capabilities enabled.
// Suitable for production workloads requiring reliability, observability, and protection.
func ProductionClient(apiKey, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithModel(model).
		WithMetrics().
		WithCache(DefaultCacheSize).
		WithCircuitBreaker().
		WithRateLimit(100).
		WithRetry(3).
		Build()
}

// ProductionClientWithEndpoint creates a production-ready client with custom endpoint.
func ProductionClientWithEndpoint(apiKey, endpoint, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithEndpoint(endpoint).
		WithModel(model).
		WithMetrics().
		WithCache(DefaultCacheSize).
		WithCircuitBreaker().
		WithRateLimit(100).
		WithRetry(3).
		Build()
}

// DevelopmentClient creates a development client with minimal overhead.
// Only metrics are enabled for debugging purposes.
func DevelopmentClient(apiKey, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithModel(model).
		WithMetrics(DefaultMetricsConfig()).
		Build()
}

// DevelopmentClientWithEndpoint creates a development client with custom endpoint.
func DevelopmentClientWithEndpoint(apiKey, endpoint, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithEndpoint(endpoint).
		WithModel(model).
		WithMetrics(DefaultMetricsConfig()).
		Build()
}

// TestingClient creates a client optimized for testing scenarios.
// Includes cache for deterministic responses and minimal rate limiting.
func TestingClient(apiKey, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithModel(model).
		WithCache(100).
		WithRateLimit(1000). // High limit for tests
		Build()
}

// HighThroughputClient creates a client optimized for high-throughput scenarios.
// Aggressive caching, high rate limits, and no retries for speed.
func HighThroughputClient(apiKey, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithModel(model).
		WithCache(10000).
		WithRateLimit(500).
		WithMetrics().
		Build()
}

// ReliableClient creates a client optimized for reliability.
// Aggressive retries, circuit breaker, and conservative rate limiting.
func ReliableClient(apiKey, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithModel(model).
		WithRetryConfig(5, 200, 10000).
		WithCircuitBreaker(CircuitBreakerConfig{
			Name:                "reliable",
			MaxFailures:         3,
			Interval:            30 * time.Second,
			Timeout:             60 * time.Second,
			HalfOpenMaxRequests: 1,
			SuccessThreshold:    3,
		}).
		WithRateLimit(50).
		WithMetrics().
		Build()
}

// MinimalClient creates a bare-bones client with no middleware.
// Useful for simple use cases or custom configurations.
func MinimalClient(apiKey, model string) (LLMClient, error) {
	return NewClientBuilder().
		WithAPIKey(apiKey).
		WithModel(model).
		Build()
}
