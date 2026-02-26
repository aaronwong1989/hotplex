package slack

import (
	"context"
	"strings"
	"time"
)

// RateLimitError represents a rate limit error with retry-after information
type RateLimitError struct {
	RetryAfter time.Duration // Duration to wait before retrying
}

func (e *RateLimitError) Error() string {
	return "rate limited: 429"
}

// ErrorClass classifies Slack API errors for handling decisions
type ErrorClass int

const (
	// Retryable - errors that should be retried with backoff
	Retryable ErrorClass = iota
	// Fatal - errors that should not be retried (auth, invalid, etc.)
	Fatal
	// UserError - errors that indicate user action needed (channel not found, etc.)
	UserError
	// Unknown - errors that are not recognized, default to retry
	Unknown
)

// classifySlackError classifies a Slack API error string into an ErrorClass
func classifySlackError(err string) ErrorClass {
	errLower := strings.ToLower(err)

	// Fatal errors - never retry
	fatalErrors := []string{
		"invalid_auth", "account_inactive", "token_revoked",
		"not_allowed_token_type", "missing_scope", "access_denied",
		"invalid_arguments", "invalid_arg_name", "invalid_array_json",
		"invalid_authz_context", "invalid_charset", "invalid_client_id",
		"invalid_cursor", "invalid_post_type", "invalid_request",
		"invalid_trigger", "missing_argument", "no_permission",
		"unauthorized", "forbidden", "401", "403",
	}
	for _, e := range fatalErrors {
		if strings.Contains(errLower, e) {
			return Fatal
		}
	}

	// User errors - indicate user action needed, don't retry
	userErrors := []string{
		"channel_not_found", "not_in_channel", "is_archived",
		"cannot_send_message_to_user", "user_not_found", "user_is_restricted",
		"message_not_found", "file_not_found", "comment_not_found",
		"group_not_found", "not_allowed_type", "read_only",
		"channel_is_archived", "user_group_not_found",
		"404", "validation_error", "validation",
		"422", "unprocessable", "invalid input", "malformed",
		"not found", "resource not found",
	}
	for _, e := range userErrors {
		if strings.Contains(errLower, e) {
			return UserError
		}
	}

	// Retryable errors - should retry with backoff
	retryableErrors := []string{
		"ratelimited", "rate_limited", "rate-limit", "429",
		"timeout", "temporary", "unavailable", "internal_error",
		"server_error", "fatal_error", "request_timeout",
		"connection_refused", "connection_reset", "i/o timeout",
		"500", "502", "503", "504",
	}
	for _, e := range retryableErrors {
		if strings.Contains(errLower, e) {
			return Retryable
		}
	}

	// Default: retry (conservative approach)
	return Unknown
}

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

	// Check if it's a RateLimitError - always retryable
	if _, ok := err.(*RateLimitError); ok {
		return true
	}

	errStr := strings.ToLower(err.Error())
	errClass := classifySlackError(errStr)

	// Only retry Retryable and Unknown errors
	return errClass == Retryable || errClass == Unknown
}

// GetErrorClass returns the ErrorClass for a given error
func GetErrorClass(err error) ErrorClass {
	if err == nil {
		return Unknown
	}
	return classifySlackError(strings.ToLower(err.Error()))
}
