package chatapps

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// RateLimitProcessor implements non-blocking rate limiting for message sending.
// Instead of blocking the caller with time.After(), it drops messages that arrive
// too fast and lets the next one through after the minimum interval.
// This prevents stalling the engine callback goroutine during rapid event bursts.
type RateLimitProcessor struct {
	logger *slog.Logger

	// Per-session rate limiting
	sessionLimits map[string]time.Time
	mu            sync.Mutex

	// Configuration
	minInterval time.Duration
	maxBurst    int
	burstWindow time.Duration
}

// RateLimitProcessorOptions configures the rate limit processor
type RateLimitProcessorOptions struct {
	MinInterval time.Duration // Minimum interval between messages
	MaxBurst    int           // Maximum messages in burst window
	BurstWindow time.Duration // Time window for burst calculation
}

// NewRateLimitProcessor creates a new RateLimitProcessor
func NewRateLimitProcessor(logger *slog.Logger, opts RateLimitProcessorOptions) *RateLimitProcessor {
	if logger == nil {
		logger = slog.Default()
	}

	// Set defaults
	if opts.MinInterval == 0 {
		opts.MinInterval = 100 * time.Millisecond
	}
	if opts.MaxBurst == 0 {
		opts.MaxBurst = 5
	}
	if opts.BurstWindow == 0 {
		opts.BurstWindow = time.Second
	}

	return &RateLimitProcessor{
		logger:        logger,
		sessionLimits: make(map[string]time.Time),
		minInterval:   opts.MinInterval,
		maxBurst:      opts.MaxBurst,
		burstWindow:   opts.BurstWindow,
	}
}

// Name returns the processor name
func (p *RateLimitProcessor) Name() string {
	return "RateLimitProcessor"
}

// Order returns the processor order (should run first)
func (p *RateLimitProcessor) Order() int {
	return int(OrderRateLimit)
}

// Process applies non-blocking rate limiting to the message.
// If a message arrives before the minimum interval has elapsed,
// it is dropped (returns nil) rather than blocking the caller.
// Messages with Immediate flag in aggregator config are always passed through.
func (p *RateLimitProcessor) Process(ctx context.Context, msg *base.ChatMessage) (*base.ChatMessage, error) {
	if msg == nil {
		return nil, nil
	}

	// Create session key
	sessionKey := msg.Platform + ":" + msg.SessionID

	p.mu.Lock()
	lastSend := p.sessionLimits[sessionKey]
	now := time.Now()

	elapsed := now.Sub(lastSend)
	if elapsed < p.minInterval {
		p.mu.Unlock()

		// Non-blocking: drop the message instead of waiting
		p.logger.Debug("Rate limit - dropping message (non-blocking)",
			"session_key", sessionKey,
			"elapsed_ms", elapsed.Milliseconds(),
			"min_interval_ms", p.minInterval.Milliseconds())

		RateLimitDroppedTotal.Inc()
		return nil, nil
	}

	// Update last send time
	p.sessionLimits[sessionKey] = now
	p.mu.Unlock()

	p.logger.Debug("Rate limit check passed",
		"session_key", sessionKey,
		"platform", msg.Platform)

	return msg, nil
}

// Cleanup removes old session rate limits
func (p *RateLimitProcessor) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-p.burstWindow)

	for key, lastTime := range p.sessionLimits {
		if lastTime.Before(cutoff) {
			delete(p.sessionLimits, key)
		}
	}

	p.logger.Debug("Rate limit cleanup completed",
		"remaining_sessions", len(p.sessionLimits))
}

// GetSessionStats returns rate limit stats for a session
func (p *RateLimitProcessor) GetSessionStats(platform, sessionID string) (lastSend time.Time, exists bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sessionKey := platform + ":" + sessionID
	lastTime, ok := p.sessionLimits[sessionKey]
	if !ok {
		return time.Time{}, false
	}
	return lastTime, true
}

// Verify RateLimitProcessor implements MessageProcessor at compile time
var _ MessageProcessor = (*RateLimitProcessor)(nil)
