package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	feishuAPIBase    = "https://open.feishu.cn"
	feishuTokenAPI   = "/open-apis/auth/v3/app_access_token/internal"
	feishuMessageAPI = "/open-apis/im/v1/messages"
)

// doRequest is a helper function to reduce HTTP request boilerplate (DRY)
func (c *Client) doRequest(ctx context.Context, method, url string, reqBody interface{}, token string, respObj interface{}) error {
	var req *http.Request
	var err error

	if reqBody != nil {
		var bodyBytes []byte
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return err
		}
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if respObj != nil {
		if err := json.Unmarshal(body, respObj); err != nil {
			return err
		}
	}

	return nil
}

// FeishuAPIClient defines the interface for Feishu API operations (SOLID: Dependency Inversion)
type FeishuAPIClient interface {
	GetAppTokenWithContext(ctx context.Context) (string, int, error)
	SendMessage(ctx context.Context, token, chatID, msgType string, content map[string]string) (string, error)
	SendTextMessage(ctx context.Context, token, chatID, text string) (string, error)
	SendInteractiveMessage(ctx context.Context, token, chatID, cardJSON string) (string, error)

	// Card API methods for streaming support
	CreateCard(ctx context.Context, token string, card *CardTemplate) (string, error)
	UpdateCard(ctx context.Context, token, cardID string, card *CardTemplate, sequence int) error
	SendCardMessage(ctx context.Context, token, chatID, cardID string) (string, error)
}

// Client wraps the Feishu Open API
type Client struct {
	appID      string
	appSecret  string
	logger     *slog.Logger
	httpClient *http.Client
}

// Ensure Client implements FeishuAPIClient
var _ FeishuAPIClient = (*Client)(nil)

// NewClient creates a new Feishu API client
func NewClient(appID, appSecret string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		appID:     appID,
		appSecret: appSecret,
		logger:    logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TokenResponse represents the app access token response
type TokenResponse struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	AppAccessToken string `json:"app_access_token"`
	Expire         int    `json:"expire"`
}

// GetAppToken fetches a new app access token
func (c *Client) GetAppToken() (string, int, error) {
	ctx := context.Background()
	return c.GetAppTokenWithContext(ctx)
}

// GetAppTokenWithContext fetches a new app access token with context
func (c *Client) GetAppTokenWithContext(ctx context.Context) (string, int, error) {
	url := feishuAPIBase + feishuTokenAPI

	reqBody := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}

	var tokenResp TokenResponse
	if err := c.doRequest(ctx, "POST", url, reqBody, "", &tokenResp); err != nil {
		return "", 0, ErrTokenFetchFailed
	}

	if tokenResp.Code != 0 {
		c.logger.Error("Feishu token API error", "code", tokenResp.Code, "msg", tokenResp.Msg)
		return "", 0, &APIError{Code: tokenResp.Code, Msg: tokenResp.Msg}
	}

	return tokenResp.AppAccessToken, tokenResp.Expire, nil
}

// SendMessageRequest represents a message send request
type SendMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
}

// SendMessageResponse represents a message send response
type SendMessageResponse struct {
	Code int          `json:"code"`
	Msg  string       `json:"msg"`
	Data *MessageData `json:"data"`
}

// MessageData represents message data in response
type MessageData struct {
	MessageID string `json:"message_id"`
}

// SendTextMessage sends a text message
func (c *Client) SendTextMessage(ctx context.Context, token, chatID, text string) (string, error) {
	return c.SendMessage(ctx, token, chatID, "text", map[string]string{"text": text})
}

// SendMessage sends a message with specified type and content
// This is the generic message sending method for extensibility
func (c *Client) SendMessage(ctx context.Context, token, chatID, msgType string, content map[string]string) (string, error) {
	url := feishuAPIBase + feishuMessageAPI + "?receive_id_type=chat_id"

	contentBytes, err := json.Marshal(content)
	if err != nil {
		return "", err
	}

	reqBody := SendMessageRequest{
		ReceiveID: chatID,
		MsgType:   msgType,
		Content:   string(contentBytes),
	}

	var msgResp SendMessageResponse
	if err := c.doRequest(ctx, "POST", url, reqBody, token, &msgResp); err != nil {
		return "", ErrMessageSendFailed
	}

	if msgResp.Code != 0 {
		c.logger.Error("Feishu message API error", "code", msgResp.Code, "msg", msgResp.Msg)
		return "", &APIError{Code: msgResp.Code, Msg: msgResp.Msg}
	}

	return msgResp.Data.MessageID, nil
}

// SendInteractiveMessage sends an interactive card message
// This is a placeholder for Phase 2 implementation
func (c *Client) SendInteractiveMessage(ctx context.Context, token, chatID, cardJSON string) (string, error) {
	return c.SendMessage(ctx, token, chatID, "interactive", map[string]string{"config": cardJSON})
}
