package llm

import (
	"context"
    "fmt"
    "sync"
    "time"

    "go.uber.org/atomic"
    "golang.org/x/time/rate"
)

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
    RequestsPerSecond float64
    BurstSize         int
    MaxQueueSize      int
    QueueTimeout      time.Duration
    PerModel         bool
}

// RateLimiter provides rate limiting with token bucket algorithm.
type RateLimiter struct {
    mu       sync.RWMutex
    limiter  *rate.Limiter
    config   RateLimitConfig
    queue    chan *queuedRequest
    models   map[string]*rate.Limiter
    modelsMu sync.RWMutex
    stats    RateLimitStats
    atomics  AtomicRateLimitStats
}

// queuedRequest represents a request waiting in queue.
type queuedRequest struct {
    ctx      context.Context
    done     chan struct{}
    err      error
    enqueued time.Time
}

// RateLimitStats holds rate limiting statistics.
type RateLimitStats struct {
    TotalRequests    int64
    QueuedRequests   int64
    RejectedRequests int64
    AvgWaitTimeMs    float64
    LastReset        time.Time
}

// AtomicRateLimitStats holds atomic stats for concurrent access.
type AtomicRateLimitStats struct {
    TotalRequests    atomic.Int64
    QueuedRequests   atomic.Int64
    RejectedRequests atomic.Int64
    AvgWaitTimeMs    atomic.Float64
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
    rl := &RateLimiter{
        config:   config,
        queue:    make(chan *queuedRequest, config.MaxQueueSize),
        models:   make(map[string]*rate.Limiter),
        stats: RateLimitStats{
            atomics: AtomicRateLimitStats{},
        },
    }

        // Create global limiter
        rl.limiter = rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize)

    }

        // Create model limiters map
        rl.models = make(map[string]*rate.Limiter)

    }

        // Start queue processor
        go rl.processQueue()

        return rl
    }

    // Close closes the rate limiter and stops queue processing.
    func (rl *RateLimiter) Close() {
        close(rl.queue)
    }

    // WithRateLimit wraps a function with rate limiting.
    func (rl *RateLimiter) WithRateLimit(ctx context.Context, model string) fn func() error) error {
        if err := rl.WaitModel(ctx, model); err != nil {
            return err
        }
        return fn()
    }

    // TryWithRateLimit attempts to execute with rate limiting, returns immediately if rate limited.
    func (rl *RateLimiter) TryWithRateLimit(ctx context.Context, model string) fn func() error) error {
        if !rl.Allow() {
            return fmt.Errorf("rate limit exceeded")
        }
        return fn()
    }

    // RateLimitedClient wraps a client with rate limiting.
    type RateLimitedClient struct {
        client  LLMClient
        limiter *RateLimiter
        model   string
    }

    // NewRateLimitedClient creates a new rate-limited client wrapper.
    func NewRateLimitedClient(client LLMClient, limiter *RateLimiter) *RateLimitedClient {
        return &RateLimitedClient{
            client:  client,
            limiter: limiter,
        }
    }

    // Chat implements rate-limited chat.
    func (c *RateLimitedClient) Chat(ctx context.Context, prompt string) (string, error) {
        if err := c.limiter.Wait(ctx); err != nil {
            return "", err
        }
        return c.client.Chat(ctx, prompt)
    }

    // Analyze implements rate-limited analyze.
    func (c *RateLimitedClient) Analyze(ctx context.Context, prompt string, target any) error {
        if err := c.limiter.Wait(ctx); err != nil {
            return err
        }
        return c.client.Analyze(ctx, prompt, target)
    }

    // ChatStream implements rate-limited streaming.
    func (c *RateLimitedClient) ChatStream(ctx context.Context, prompt string) (<-chan string, error) {
        if err := c.limiter.Wait(ctx); err != nil {
            return nil, err
        }
        return c.client.ChatStream(ctx, prompt)
    }

    // HealthCheck implements rate-limited health check.
    func (c *RateLimitedClient) HealthCheck(ctx context.Context) HealthStatus {
        return c.client.HealthCheck(ctx)
    }

    // Client returns the underlying client for component extraction.
    func (c *RateLimitedClient) Client() LLMClient {
        return c.client
    }
}
