package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	feishuCardAPI = "/open-apis/card/v1/cards"
)

// CreateCardRequest represents a card creation request
type CreateCardRequest struct {
	Card *CardTemplate `json:"card"`
}

// CreateCardResponse represents a card creation response
type CreateCardResponse struct {
	Code int       `json:"code"`
	Msg  string    `json:"msg"`
	Data *CardData `json:"data"`
}

// CardData represents card data in response
type CardData struct {
	CardID string `json:"card_id"`
}

// UpdateCardRequest represents a card update request
type UpdateCardRequest struct {
	Card     *CardTemplate `json:"card"`
	Sequence int           `json:"sequence"`
}

// UpdateCardResponse represents a card update response
type UpdateCardResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// CreateCard creates a new card entity and returns card_id
// This is used for streaming: create card -> send card message -> update card
func (c *Client) CreateCard(ctx context.Context, token string, card *CardTemplate) (string, error) {
	url := feishuAPIBase + feishuCardAPI

	reqBody := CreateCardRequest{
		Card: card,
	}

	var cardResp CreateCardResponse
	if err := c.doRequest(ctx, "POST", url, reqBody, token, &cardResp); err != nil {
		return "", fmt.Errorf("create card failed: %w", err)
	}

	if cardResp.Code != 0 {
		c.logger.Error("Feishu create card API error", "code", cardResp.Code, "msg", cardResp.Msg)
		return "", &APIError{Code: cardResp.Code, Msg: cardResp.Msg}
	}

	if cardResp.Data == nil {
		return "", fmt.Errorf("create card response missing data")
	}

	return cardResp.Data.CardID, nil
}

// UpdateCard updates an existing card entity with new content
// IMPORTANT: sequence must be strictly incrementing for each card_id
func (c *Client) UpdateCard(ctx context.Context, token, cardID string, card *CardTemplate, sequence int) error {
	url := feishuAPIBase + feishuCardAPI + "/" + cardID

	reqBody := UpdateCardRequest{
		Card:     card,
		Sequence: sequence,
	}

	var cardResp UpdateCardResponse
	if err := c.doRequest(ctx, "PUT", url, reqBody, token, &cardResp); err != nil {
		return fmt.Errorf("update card failed: %w", err)
	}

	if cardResp.Code != 0 {
		c.logger.Error("Feishu update card API error",
			"code", cardResp.Code,
			"msg", cardResp.Msg,
			"card_id", cardID,
			"sequence", sequence)
		return &APIError{Code: cardResp.Code, Msg: cardResp.Msg}
	}

	return nil
}

// SendCardMessage sends a card message by card_id
// After creating a card entity, use this to send it to a chat
func (c *Client) SendCardMessage(ctx context.Context, token, chatID, cardID string) (string, error) {
	// Feishu card message format: {"type": "template", "data": {"template_id": cardID}}
	content := map[string]interface{}{
		"type": "template",
		"data": map[string]string{
			"template_id": cardID,
		},
	}

	contentBytes, err := json.Marshal(content)
	if err != nil {
		return "", err
	}

	reqBody := SendMessageRequest{
		ReceiveID: chatID,
		MsgType:   "interactive",
		Content:   string(contentBytes),
	}

	var msgResp SendMessageResponse
	if err := c.doRequest(ctx, "POST", feishuAPIBase+feishuMessageAPI+"?receive_id_type=chat_id", reqBody, token, &msgResp); err != nil {
		return "", fmt.Errorf("send card message failed: %w", err)
	}

	if msgResp.Code != 0 {
		c.logger.Error("Feishu send card message API error", "code", msgResp.Code, "msg", msgResp.Msg)
		return "", &APIError{Code: msgResp.Code, Msg: msgResp.Msg}
	}

	return msgResp.Data.MessageID, nil
}

// SendCardWithRetry sends a card message with exponential backoff retry
// Retry strategy:
//   - Initial delay: 100ms
//   - Max delay: 5s
//   - Max attempts: 3
//   - Retry on: network errors and 5xx server errors
//   - No retry on: 4xx client errors
func (c *Client) SendCardWithRetry(ctx context.Context, token, chatID, cardID string) (string, error) {
	const (
		maxAttempts    = 3
		initialDelay   = 100 * time.Millisecond
		maxDelay       = 5 * time.Second
		backoffFactor  = 2.0
	)

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		messageID, err := c.SendCardMessage(ctx, token, chatID, cardID)
		if err == nil {
			// Success
			return messageID, nil
		}

		lastErr = err

		// Check if error is retryable
		if !c.isRetryableError(err) {
			// Non-retryable error (e.g., 4xx client error)
			return "", err
		}

		// Don't sleep after last attempt
		if attempt < maxAttempts {
			delay := c.calculateBackoff(attempt, initialDelay, maxDelay, backoffFactor)
			c.logger.Warn("SendCardMessage failed, retrying",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"delay_ms", delay.Milliseconds(),
				"error", err)

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	// All retries exhausted
	return "", fmt.Errorf("send card message failed after %d attempts: %w", maxAttempts, lastErr)
}

// isRetryableError determines if an error is retryable
// Retryable: network errors, 5xx server errors
// Non-retryable: 4xx client errors
func (c *Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors (timeout, connection refused, etc.)
	if isNetworkError(err) {
		return true
	}

	// Check for API errors
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// 5xx server errors are retryable
		// 4xx client errors are not retryable
		return apiErr.Code >= 500 && apiErr.Code < 600
	}

	// Unknown error type - be conservative and don't retry
	return false
}

// isNetworkError checks if an error is a network-level error
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error (timeout, temporary errors)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for common network error types
	// This includes: connection refused, timeout, DNS errors, etc.
	return isNetOpError(err)
}

// isNetOpError checks if error is a net.OpError
func isNetOpError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}

// calculateBackoff calculates the backoff delay for a given attempt
func (c *Client) calculateBackoff(attempt int, initialDelay, maxDelay time.Duration, factor float64) time.Duration {
	delay := initialDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * factor)
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}
	return delay
}
