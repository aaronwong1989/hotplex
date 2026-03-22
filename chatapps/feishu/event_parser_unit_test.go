package feishu

import (
	"log/slog"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

func TestParseEvent_ValidMessage(t *testing.T) {
	jsonBody := `{
		"schema": "2.0",
		"header": {
			"event_type": "im.message.receive_v1",
			"event_id": "evt-123",
			"create_time": 1234567890,
			"tenant_key": "tenant-1",
			"app_id": "app-1"
		},
		"event": {
			"message": {
				"message_id": "msg-1",
				"sender_id": "user-1",
				"chat_id": "chat-1",
				"content": {"type": "text", "text": "hello"},
				"create_time": 1234567890000,
				"tenant_key": "tenant-1"
			}
		}
	}`

	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, slog.Default()),
	}
	event, err := adapter.parseEvent([]byte(jsonBody))
	if err != nil {
		t.Fatalf("parseEvent failed: %v", err)
	}
	if event.Header.EventType != "im.message.receive_v1" {
		t.Errorf("expected event_type im.message.receive_v1, got %s", event.Header.EventType)
	}
	if event.Event == nil || event.Event.Message == nil {
		t.Fatal("expected event message to be non-nil")
	}
	if event.Event.Message.MessageID != "msg-1" {
		t.Errorf("expected message_id msg-1, got %s", event.Event.Message.MessageID)
	}
	if event.Event.Message.Content == nil || event.Event.Message.Content.Text != "hello" {
		t.Error("expected content text 'hello'")
	}
}

func TestParseEvent_URLVerification(t *testing.T) {
	jsonBody := `{
		"type": "url_verification",
		"challenge": "test-challenge"
	}`

	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, slog.Default()),
	}
	event, err := adapter.parseEvent([]byte(jsonBody))
	if err != nil {
		t.Fatalf("parseEvent failed: %v", err)
	}
	if event.Type != "url_verification" {
		t.Errorf("expected type url_verification, got %s", event.Type)
	}
	if event.Challenge != "test-challenge" {
		t.Errorf("expected challenge test-challenge, got %s", event.Challenge)
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, slog.Default()),
	}
	_, err := adapter.parseEvent([]byte("not json"))
	if err != ErrEventParseFailed {
		t.Errorf("expected ErrEventParseFailed, got %v", err)
	}
}

func TestParseEvent_EmptyJSON(t *testing.T) {
	adapter := &Adapter{
		Adapter: base.NewAdapter("feishu", base.Config{}, slog.Default()),
	}
	_, err := adapter.parseEvent([]byte("{}"))
	if err != nil {
		t.Fatalf("parseEvent empty JSON should succeed: %v", err)
	}
}
