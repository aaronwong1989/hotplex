package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RelaySender sends relay messages to remote HotPlex instances.
type RelaySender struct {
	client *http.Client
	token  string
}

// NewRelaySender creates a new RelaySender with the given auth token.
func NewRelaySender(token string) *RelaySender {
	return &RelaySender{
		client: &http.Client{Timeout: 10 * time.Second},
		token:  token,
	}
}

// Send delivers a RelayMessage to a target HotPlex instance via HTTP POST.
// It retries up to 3 times with exponential backoff (1s -> 2s -> 4s).
func (s *RelaySender) Send(ctx context.Context, msg *RelayMessage, targetURL string) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal relay message: %w", err)
	}

	// Build request once; body is recreated per retry since Reader is consumed.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL+"/relay", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	const maxRetries = 3
	backoff := 1 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		// Recreate body reader for each attempt (bytes.Reader is consumed after first read).
		req.Body = io.NopCloser(bytes.NewReader(body))

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		// Read body for error context, then close.
		bodyBytes, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))

		// Do not retry on 4xx client errors (except 429).
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			break
		}
	}

	return lastErr
}
