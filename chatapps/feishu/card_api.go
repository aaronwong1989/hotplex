package feishu

import (
	"context"
	"encoding/json"
	"fmt"
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
	Code int `json:"code"`
	Msg  string `json:"msg"`
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
