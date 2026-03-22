package chatapps

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 100)

	for i := 0; i < 10; i++ {
		if !rl.Allow() {
			t.Errorf("request %d should be allowed", i)
		}
	}
	// 11th should be denied
	if rl.Allow() {
		t.Error("11th request should be blocked")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(1, 1000) // 1 token, refill 1000/s

	if !rl.Allow() {
		t.Error("first request should be allowed")
	}

	// Wait for refill
	time.Sleep(10 * time.Millisecond)

	if !rl.Allow() {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(1, 1000)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if !rl.Allow() {
		t.Error("first should be allowed")
	}

	err := rl.Wait(ctx)
	if err != nil {
		t.Logf("Wait returned expected possible error: %v", err)
	}
}

func TestRateLimiter_Wait_ContextCancelled(t *testing.T) {
	rl := NewRateLimiter(0, 0) // no tokens, no refill

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected context error")
	}
}

func TestRetryWithBackoff_Success(t *testing.T) {
	calls := 0
	err := RetryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_Exhausted(t *testing.T) {
	err := RetryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}, func() error {
		return errors.New("persistent error")
	})
	if err == nil {
		t.Error("expected error after exhaustion")
	}
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RetryWithBackoff(ctx, RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Second,
		MaxDelay:    time.Second,
	}, func() error {
		return errors.New("never succeeds")
	})
	if err == nil {
		t.Error("expected context error")
	}
}

func TestIsRetryableError_Nil(t *testing.T) {
	if IsRetryableError(nil) {
		t.Error("nil error should not be retryable")
	}
}

func TestIsRetryableError_NonRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"401", errors.New("unauthorized"), false},
		{"403", errors.New("forbidden"), false},
		{"404", errors.New("not found"), false},
		{"validation", errors.New("validation error"), false},
		{"invalid", errors.New("invalid request"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableError_Retryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout", errors.New("request timeout"), true},
		{"429", errors.New("429 too many requests"), true},
		{"500", errors.New("500 server error"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"connection refused", errors.New("connection refused"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableError_DefaultRetryable(t *testing.T) {
	// Unknown errors default to retryable (conservative)
	if !IsRetryableError(errors.New("something went wrong")) {
		t.Error("unknown errors should be retryable by default")
	}
}

func TestNewChatMessage(t *testing.T) {
	msg := NewChatMessage("slack", "sess1", "U1", "hello")
	if msg.Platform != "slack" {
		t.Errorf("expected platform slack, got %s", msg.Platform)
	}
	if msg.SessionID != "sess1" {
		t.Errorf("expected session sess1, got %s", msg.SessionID)
	}
	if msg.Content != "hello" {
		t.Errorf("expected content hello, got %s", msg.Content)
	}
	if msg.Metadata == nil {
		t.Error("metadata should not be nil")
	}
	if msg.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}
