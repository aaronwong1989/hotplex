package relay

import (
	"context"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// RelayCircuitBreaker manages per-destination circuit breakers for relay requests.
// Each unique target (identified by name, typically the bot or instance name) gets
// its own circuit breaker that prevents cascading failures.
type RelayCircuitBreaker struct {
	breakers map[string]*gobreaker.CircuitBreaker
	mu       sync.RWMutex
	settings gobreaker.Settings
}

// NewRelayCircuitBreaker creates a new RelayCircuitBreaker with sensible defaults:
// - MaxRequests: 1 (half-open allows 1 probe request)
// - Interval: 0 (no periodic reset; only trip-based reset)
// - Timeout: 30s (how long the breaker stays open before transitioning to half-open)
// - ReadyToTrip: opens after 5 consecutive failures
func NewRelayCircuitBreaker() *RelayCircuitBreaker {
	return &RelayCircuitBreaker{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		settings: gobreaker.Settings{
			Name: "relay",
			ReadyToTrip: func(count gobreaker.Counts) bool {
				return count.ConsecutiveFailures >= 5
			},
			Timeout: 30 * time.Second,
		},
	}
}

// Get returns the circuit breaker for the given name, creating one if it doesn't exist.
// Thread-safe: uses double-checked locking.
func (rcb *RelayCircuitBreaker) Get(name string) *gobreaker.CircuitBreaker {
	rcb.mu.RLock()
	cb, ok := rcb.breakers[name]
	rcb.mu.RUnlock()
	if ok {
		return cb
	}

	rcb.mu.Lock()
	defer rcb.mu.Unlock()
	// Double-check after acquiring write lock.
	if cb, ok = rcb.breakers[name]; ok {
		return cb
	}

	settings := rcb.settings
	settings.Name = name
	cb = gobreaker.NewCircuitBreaker(settings)
	rcb.breakers[name] = cb
	return cb
}

// Call executes fn through the circuit breaker identified by name.
// Returns gobreaker.ErrOpenState if the circuit is open.
// The state change (closed → open → half-open → closed) is managed by gobreaker.
func (rcb *RelayCircuitBreaker) Call(ctx context.Context, name string, fn func() (any, error)) (any, error) {
	cb := rcb.Get(name)
	return cb.Execute(func() (any, error) {
		return fn()
	})
}
