package bridgeclient

import (
	"encoding/json"
	"testing"
)

func TestCapabilities(t *testing.T) {
	tests := []struct {
		name  string
		caps  []string
		check func([]string) bool
	}{
		{"CapText", []string{CapText}, func(c []string) bool { return len(c) == 1 && c[0] == "text" }},
		{"CapImage", []string{CapImage}, func(c []string) bool { return len(c) == 1 && c[0] == "image" }},
		{"CapCard", []string{CapCard}, func(c []string) bool { return len(c) == 1 && c[0] == "card" }},
		{"AllCaps", []string{CapText, CapImage, CapCard, CapButtons, CapTyping, CapEdit, CapDelete, CapReact, CapThread},
			func(c []string) bool { return len(c) == 9 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.caps) {
				t.Errorf("capabilities = %v", tt.caps)
			}
		})
	}
}

func TestMessageRoundTrip(t *testing.T) {
	meta := Metadata{UserID: "u123", RoomID: "r456", ThreadID: "t789", Platform: "dingtalk"}
	msg := &Message{
		SessionKey: "sess-abc",
		Content:    "Hello from DingTalk!",
		Metadata:   meta,
	}

	// Simulate what SendMessage does: marshal metadata
	metaBytes, err := json.Marshal(metadata{
		UserID:   msg.Metadata.UserID,
		RoomID:   msg.Metadata.RoomID,
		ThreadID: msg.Metadata.ThreadID,
		Platform: "dingtalk",
	})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	wm := wireMessage{
		Type:       msgTypeMessage,
		Platform:   "dingtalk",
		SessionKey: msg.SessionKey,
		Content:    msg.Content,
		Metadata:   metaBytes,
	}

	// Marshal the wire message (as it would travel over WebSocket)
	bytes, err := json.Marshal(wm)
	if err != nil {
		t.Fatalf("marshal wire message: %v", err)
	}

	// Unmarshal to verify round-trip
	var got wireMessage
	if err := json.Unmarshal(bytes, &got); err != nil {
		t.Fatalf("unmarshal wire message: %v", err)
	}

	if got.Type != msgTypeMessage {
		t.Errorf("Type = %q, want %q", got.Type, msgTypeMessage)
	}
	if got.SessionKey != msg.SessionKey {
		t.Errorf("SessionKey = %q, want %q", got.SessionKey, msg.SessionKey)
	}
	if got.Content != msg.Content {
		t.Errorf("Content = %q, want %q", got.Content, msg.Content)
	}
	if got.Platform != "dingtalk" {
		t.Errorf("Platform = %q, want %q", got.Platform, "dingtalk")
	}

	// Unmarshal metadata
	var gotMeta metadata
	if err := json.Unmarshal(got.Metadata, &gotMeta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if gotMeta.UserID != meta.UserID {
		t.Errorf("metadata.UserID = %q, want %q", gotMeta.UserID, meta.UserID)
	}
}

func TestReplyRoundTrip(t *testing.T) {
	reply := &Reply{
		Content:    "AI response text",
		SessionKey: "sess-abc",
		Metadata:   Metadata{UserID: "u123"},
	}

	metaBytes, _ := json.Marshal(metadata{
		Platform: "dingtalk",
		UserID:   reply.Metadata.UserID,
	})
	wm := wireMessage{
		Type:       msgTypeReply,
		Platform:   "dingtalk",
		SessionKey: reply.SessionKey,
		Content:    reply.Content,
		Metadata:   metaBytes,
	}

	bytes, err := json.Marshal(wm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got wireMessage
	if err := json.Unmarshal(bytes, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != msgTypeReply {
		t.Errorf("Type = %q, want %q", got.Type, msgTypeReply)
	}
	if got.SessionKey != reply.SessionKey {
		t.Errorf("SessionKey = %q, want %q", got.SessionKey, reply.SessionKey)
	}
	if got.Content != reply.Content {
		t.Errorf("Content = %q, want %q", got.Content, reply.Content)
	}
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name   string
		opts   []Option
		wantErr string
	}{
		{
			name:   "missing URL",
			opts:   []Option{Platform("dingtalk")},
			wantErr: "bridgeclient: URL is required",
		},
		{
			name:   "missing platform",
			opts:   []Option{URL("ws://localhost:8080/bridge")},
			wantErr: "bridgeclient: platform name is required",
		},
		{
			name:   "valid",
			opts:   []Option{URL("ws://localhost:8080/bridge"), Platform("dingtalk")},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.opts...)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("New() error = %v, want nil", err)
				}
			} else {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("New() error = %v, want %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestOptionOrder(t *testing.T) {
	// Options should be applied in order; last one wins for duplicates
	c, err := New(
		Platform("dingtalk"),
		URL("ws://localhost:8080/bridge"),
		Capabilities(CapText, CapCard),
		Capabilities(CapText), // override: should only have CapText
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(c.caps) != 1 || c.caps[0] != CapText {
		t.Errorf("caps = %v, want [%q]", c.caps, CapText)
	}
}

func TestMetadata(t *testing.T) {
	m := Metadata{
		UserID:   "openid-123",
		RoomID:   "chatid-456",
		ThreadID: "thread-789",
		Platform: "dingtalk",
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	var got Metadata
	if err := json.Unmarshal(bytes, &got); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if got != m {
		t.Errorf("metadata round-trip: got %+v, want %+v", got, m)
	}
}
