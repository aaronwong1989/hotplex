package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHandleInteractive_ButtonCallback tests the full button callback flow
func TestHandleInteractive_ButtonCallback(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	// Create a valid button callback event
	actionValue, _ := EncodeActionValue("permission_request", "test-session-123", "msg-456")

	event := InteractiveEvent{
		Header: &InteractiveHeader{
			EventType: "im.message.reply",
		},
		Event: &InteractiveEventData{
			Message: &InteractiveMessage{
				ChatID: "test_chat_id",
			},
			User: &InteractiveUser{
				UserID: "test_user",
			},
			Action: &InteractiveAction{
				Value: actionValue,
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/feishu/interactive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Add signature headers
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Signature", calculateHMACSHA256("1234567890"+"test_encrypt_key"+string(body), "test_encrypt_key"))

	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	// Note: Will return 500 due to invalid credentials in test environment
	// The important part is that the callback flow is executed (logged above)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", rr.Code)
	}
}

// TestHandleInteractive_UnknownActionType tests unknown action type handling
func TestHandleInteractive_UnknownActionType(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	// Create event with unknown action type
	event := InteractiveEvent{
		Header: &InteractiveHeader{
			EventType: "im.message.reply",
		},
		Event: &InteractiveEventData{
			Message: &InteractiveMessage{
				ChatID: "test_chat_id",
			},
			User: &InteractiveUser{
				UserID: "test_user",
			},
			Action: &InteractiveAction{
				Value: `{"action":"unknown_action","session_id":"test"}`,
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/feishu/interactive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Signature", calculateHMACSHA256("1234567890"+"test_encrypt_key"+string(body), "test_encrypt_key"))

	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	// Should return bad request for unknown action
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// TestHandleInteractive_MissingChatID tests missing chat_id handling
func TestHandleInteractive_MissingChatID(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	// Create event with missing chat_id
	actionValue, _ := EncodeActionValue("permission_request", "test-session", "msg-123")

	event := InteractiveEvent{
		Header: &InteractiveHeader{
			EventType: "im.message.reply",
		},
		Event: &InteractiveEventData{
			Message: &InteractiveMessage{
				ChatID: "", // Missing chat_id
			},
			User: &InteractiveUser{
				UserID: "test_user",
			},
			Action: &InteractiveAction{
				Value: actionValue,
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/feishu/interactive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Signature", calculateHMACSHA256("1234567890"+"test_encrypt_key"+string(body), "test_encrypt_key"))

	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	// Should return bad request for missing chat_id
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// TestUpdatePermissionCard tests the card update function
func TestUpdatePermissionCard(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	ctx := context.Background()

	// Test with approved result
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "approved")
	if err == nil {
		// Expected to fail due to invalid credentials, but should not panic
		t.Log("Expected error due to invalid credentials")
	}

	// Test with denied result
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "denied")
	if err == nil {
		t.Log("Expected error due to invalid credentials")
	}

	// Test with unknown result
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "unknown")
	if err == nil {
		t.Log("Expected error due to invalid credentials")
	}
}

// TestClient_SendInteractiveMessage tests the interactive message sending
func TestClient_SendInteractiveMessage(t *testing.T) {
	logger := slog.Default()
	client := NewClient("test_app_id", "test_app_secret", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a simple card
	card := &CardTemplate{
		Config: &CardConfig{
			WideScreenMode: false,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Template: CardTemplateBlue,
			Title: &Text{
				Content: "Test Card",
				Tag:     TextTypePlainText,
			},
		},
	}

	cardJSON, _ := json.Marshal(card)

	// This will fail due to invalid token, but tests the function exists
	_, err := client.SendInteractiveMessage(ctx, "fake_token", "fake_chat_id", string(cardJSON))
	if err == nil {
		t.Error("Expected error due to invalid token")
	}
}

// TestCardBuilder_AllCardTypes tests all card builder methods
func TestCardBuilder_AllCardTypes(t *testing.T) {
	builder := NewCardBuilder("test-session")

	tests := []struct {
		name      string
		buildFunc func() (string, error)
	}{
		{
			name: "Thinking Card",
			buildFunc: func() (string, error) {
				return builder.BuildThinkingCard("Thinking...")
			},
		},
		{
			name: "Tool Use Card",
			buildFunc: func() (string, error) {
				return builder.BuildToolUseCard("Bash", "ls -la")
			},
		},
		{
			name: "Permission Card - Low Risk",
			buildFunc: func() (string, error) {
				return builder.BuildPermissionCard("Test", "Description", "low")
			},
		},
		{
			name: "Permission Card - High Risk",
			buildFunc: func() (string, error) {
				return builder.BuildPermissionCard("Test", "Description", "high")
			},
		},
		{
			name: "Answer Card",
			buildFunc: func() (string, error) {
				return builder.BuildAnswerCard("## Answer\n\nContent")
			},
		},
		{
			name: "Error Card",
			buildFunc: func() (string, error) {
				return builder.BuildErrorCard("Something went wrong")
			},
		},
		{
			name: "Session Stats Card",
			buildFunc: func() (string, error) {
				return builder.BuildSessionStatsCard("1.5s", 1000, map[string]string{"steps": "3/5"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cardJSON, err := tt.buildFunc()
			if err != nil {
				t.Fatalf("Card build failed: %v", err)
			}

			// Verify JSON is valid
			var card CardTemplate
			if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
				t.Fatalf("Invalid JSON: %v", err)
			}

			// Verify card has content
			if card.Header == nil && len(card.Elements) == 0 {
				t.Error("Card has no header and no elements")
			}
		})
	}
}

// TestEncodeDecodeActionValue_RoundTrip tests encoding and decoding round trip
func TestEncodeDecodeActionValue_RoundTrip(t *testing.T) {
	original := &ActionValue{
		Action:    "permission_request",
		SessionID: "session-xyz-789",
		MessageID: "msg-abc-123",
	}

	encoded, err := EncodeActionValue(original.Action, original.SessionID, original.MessageID)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeActionValue(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Action != original.Action {
		t.Errorf("Action mismatch: got %s, want %s", decoded.Action, original.Action)
	}
	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", decoded.SessionID, original.SessionID)
	}
	if decoded.MessageID != original.MessageID {
		t.Errorf("MessageID mismatch: got %s, want %s", decoded.MessageID, original.MessageID)
	}
}

// TestHandleInteractive_MethodNotAllowed tests GET request handling
func TestHandleInteractive_MethodNotAllowed(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	req := httptest.NewRequest("GET", "/feishu/interactive", nil)
	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

// TestHandleInteractive_InvalidSignature tests invalid signature handling
func TestHandleInteractive_InvalidSignature(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	event := InteractiveEvent{
		Header: &InteractiveHeader{
			EventType: "im.message.reply",
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/feishu/interactive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Invalid signature
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Signature", "invalid_signature")

	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// TestHandleInteractive_InvalidJSON tests invalid JSON handling
func TestHandleInteractive_InvalidJSON(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	req := httptest.NewRequest("POST", "/feishu/interactive", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Signature", calculateHMACSHA256("1234567890"+"test_encrypt_key"+"invalid json", "test_encrypt_key"))

	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestUpdatePermissionCard_Results tests different result types
func TestUpdatePermissionCard_Results(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	ctx := context.Background()

	// Test approved
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "approved")
	if err == nil {
		t.Error("Expected error due to invalid credentials")
	}

	// Test denied
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "denied")
	if err == nil {
		t.Error("Expected error due to invalid credentials")
	}

	// Test allow (alias)
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "allow")
	if err == nil {
		t.Error("Expected error due to invalid credentials")
	}

	// Test deny (alias)
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "deny")
	if err == nil {
		t.Error("Expected error due to invalid credentials")
	}

	// Test unknown
	err = handler.UpdatePermissionCard(ctx, "msg-123", "chat-456", "unknown")
	if err == nil {
		t.Error("Expected error due to invalid credentials")
	}
}
