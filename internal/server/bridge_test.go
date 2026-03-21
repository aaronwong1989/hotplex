package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hrygo/hotplex/chatapps/base"
)

func TestWireMessageRegister(t *testing.T) {
	wm := WireMessage{
		Type:         BridgeMsgTypeRegister,
		Platform:    "dingtalk",
		Token:       "secret",
		Capabilities: []string{CapText, CapCard},
	}
	data, err := json.Marshal(wm)
	if err != nil {
		t.Fatalf("marshal register: %v", err)
	}

	var got WireMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal register: %v", err)
	}

	if got.Type != BridgeMsgTypeRegister {
		t.Errorf("Type=%s, want %s", got.Type, BridgeMsgTypeRegister)
	}
	if got.Platform != "dingtalk" {
		t.Errorf("Platform=%s, want dingtalk", got.Platform)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("Capabilities len=%d, want 2", len(got.Capabilities))
	}
}

func TestWireMessageMessage(t *testing.T) {
	meta := WireMetadata{
		UserID:   "u123",
		RoomID:   "r456",
		ThreadID: "t789",
	}
	metaBytes, _ := json.Marshal(meta)

	wm := WireMessage{
		Type:       BridgeMsgTypeMessage,
		SessionKey: "sess-abc",
		Content:    "Hello world",
		Metadata:   metaBytes,
	}
	data, err := json.Marshal(wm)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	var got WireMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}

	if got.Type != BridgeMsgTypeMessage {
		t.Errorf("Type=%s, want %s", got.Type, BridgeMsgTypeMessage)
	}
	if got.SessionKey != "sess-abc" {
		t.Errorf("SessionKey=%s, want sess-abc", got.SessionKey)
	}

	var gotMeta WireMetadata
	if err := json.Unmarshal(got.Metadata, &gotMeta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if gotMeta.UserID != "u123" {
		t.Errorf("UserID=%s, want u123", gotMeta.UserID)
	}
	if gotMeta.RoomID != "r456" {
		t.Errorf("RoomID=%s, want r456", gotMeta.RoomID)
	}
}

func TestWireMessageError(t *testing.T) {
	wm := WireMessage{
		Type:    BridgeMsgTypeError,
		Code:    400,
		Message: "invalid request",
	}
	data, err := json.Marshal(wm)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got WireMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got.Code != 400 {
		t.Errorf("Code=%d, want 400", got.Code)
	}
	if got.Message != "invalid request" {
		t.Errorf("Message=%s, want 'invalid request'", got.Message)
	}
}

func TestBridgeServer_GetSetPlatforms(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	if platforms := bs.ListPlatforms(); len(platforms) != 0 {
		t.Errorf("ListPlatforms initial = %v, want empty", platforms)
	}

	p := bs.GetPlatform("slack")
	if p != nil {
		t.Errorf("GetPlatform(slack) = %v, want nil (not registered)", p)
	}
}

func TestBridgeServer_ServeHTTP_MissingPlatform(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	req := httptest.NewRequest("GET", "/bridge/v1/", nil)
	rec := httptest.NewRecorder()
	bs.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestBridgeServer_ServeHTTP_UnauthorizedToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "expected-token", logger)

	req := httptest.NewRequest("GET", "/bridge/v1/dingtalk?token=wrong-token", nil)
	rec := httptest.NewRecorder()
	bs.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBridgeServer_ServeHTTP_Upgrade(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	req := httptest.NewRequest("GET", "/bridge/v1/dingtalk", nil)
	rec := httptest.NewRecorder()

	// Run in goroutine since Upgrade blocks
	done := make(chan struct{})
	go func() {
		bs.ServeHTTP(rec, req)
		close(done)
	}()

	// Give it a moment to set up
	time.Sleep(10 * time.Millisecond)

	select {
	case <-done:
		// Server handled request normally
	default:
		// Expected: upgrade would have been called on a real WebSocket
	}
}

func TestBridgePlatform_ChatAdapterInterface(t *testing.T) {
	// Verify BridgePlatform implements base.ChatAdapter at compile time
	var _ base.ChatAdapter = (*BridgePlatform)(nil)
}

func TestBridgePlatform_Platform(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	if bp.Platform() != "dingtalk" {
		t.Errorf("Platform()=%s, want dingtalk", bp.Platform())
	}
}

func TestBridgePlatform_SystemPrompt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	if bp.SystemPrompt() == "" {
		t.Error("SystemPrompt() should not be empty")
	}
}

func TestBridgePlatform_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	// Start should be no-op
	if err := bp.Start(context.Background()); err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	// Stop should be safe
	if err := bp.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
}

func TestBridgePlatform_SendMessage_NilMsg(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	// Sending nil message should not panic
	if err := bp.SendMessage(context.Background(), "sess1", nil); err != nil {
		t.Errorf("SendMessage(nil) error = %v, want nil", err)
	}
}

func TestBridgeServer_Close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	// Close should be safe with no platforms
	bs.Close()

	if platforms := bs.ListPlatforms(); len(platforms) != 0 {
		t.Errorf("After Close, ListPlatforms = %v, want empty", platforms)
	}
}

func TestAllCapabilities(t *testing.T) {
	want := []string{CapText, CapImage, CapCard, CapButtons, CapTyping, CapEdit, CapDelete, CapReact, CapThread}
	if len(allCapabilities) != len(want) {
		t.Errorf("allCapabilities len=%d, want %d", len(allCapabilities), len(want))
	}
	for _, c := range want {
		found := false
		for _, a := range allCapabilities {
			if a == c {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("capability %q not found in allCapabilities", c)
		}
	}
}

func TestBridgeMsgTypes(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{BridgeMsgTypeRegister, "register"},
		{BridgeMsgTypeMessage, "message"},
		{BridgeMsgTypeReply, "reply"},
		{BridgeMsgTypeEvent, "event"},
		{BridgeMsgTypeError, "error"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("BridgeMsgType=%s, want %s", tt.got, tt.want)
		}
	}
}

// wsRecorder wraps httptest.ResponseRecorder to satisfy gorilla/websocket Upgrader.
type wsRecorder struct{ *httptest.ResponseRecorder }

func (wsRecorder) Hijack() (interface{}, *websocket.Conn, error) {
	return nil, nil, nil
}
