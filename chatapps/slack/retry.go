package slack

import (
	"context"
	"strings"
	"time"
)

// RetryConfig configures the retry behavior
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// retryWithBackoff retries a function with exponential backoff
func retryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			// Check if error is retryable
			if !isRetryableError(err) {
				return err
			}
			delay := config.BaseDelay * time.Duration(1<<attempt)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return nil
	}
	return lastErr
}

// isRetryableError classifies errors as retryable or non-retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())

	// Non-retryable: auth, validation, client errors
	nonRetryable := []string{"401", "403", "404", "422", "unauthorized", "forbidden", "not found", "validation", "invalid", "malformed"}
	for _, n := range nonRetryable {
		if strings.Contains(errStr, n) {
			return false
		}
	}

	// Retryable: timeouts, rate limits, server errors
	retryable := []string{"timeout", "temporary", "unavailable", "429", "500", "502", "503", "504", "rate limit", "too many requests", "server error", "connection refused", "connection reset", "i/o timeout"}
	for _, r := range retryable {
		if strings.Contains(errStr, r) {
			return true
		}
	}

	// Default: retry (conservative)
	return true
}
