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

	"github.com/hrygo/hotplex/chatapps/base"
)

func TestWireMessageRegister(t *testing.T) {
	wm := WireMessage{
		Type:         BridgeMsgTypeRegister,
		Platform:     "dingtalk",
		Token:        "secret",
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

// ---------------------------------------------------------------------------
// BridgePlatform handle* tests
// ---------------------------------------------------------------------------
func TestBridgePlatform_handleRegister_MissingPlatform(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("initial", nil)
	bp.server = bs

	err := bp.handleRegister(&WireMessage{Type: BridgeMsgTypeRegister})
	if err == nil {
		t.Error("handleRegister with empty platform: want error, got nil")
	}
}

func TestBridgePlatform_handleRegister_OK(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("initial", nil)
	bp.server = bs

	// Mock writeJSON to avoid WebSocket
	bp.testWriteJSON = func(wm *WireMessage) error { return nil }

	err := bp.handleRegister(&WireMessage{
		Type:         BridgeMsgTypeRegister,
		Platform:     "custom-bridge",
		Capabilities: []string{CapText, CapCard},
	})
	if err != nil {
		t.Errorf("handleRegister: unexpected error: %v", err)
	}
	if bp.platform != "custom-bridge" {
		t.Errorf("bp.platform = %q, want custom-bridge", bp.platform)
	}
}

func TestBridgePlatform_handleMessage_MissingSessionKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	err := bp.handleMessage(&WireMessage{Type: BridgeMsgTypeMessage})
	if err == nil {
		t.Error("handleMessage with empty session_key: want error, got nil")
	}
}

func TestBridgePlatform_handleMessage_OK(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	var receivedMsg *base.ChatMessage
	bp.handler = func(ctx context.Context, msg *base.ChatMessage) error {
		receivedMsg = msg
		return nil
	}

	err := bp.handleMessage(&WireMessage{
		Type:       BridgeMsgTypeMessage,
		SessionKey: "sess123",
		Content:    "hello",
		Metadata:   []byte(`{"user_id":"u1","room_id":"r1"}`),
	})
	if err != nil {
		t.Errorf("handleMessage: unexpected error: %v", err)
	}
	if receivedMsg == nil {
		t.Fatal("handler was not called")
	}
	if receivedMsg.Content != "hello" {
		t.Errorf("Content = %q, want hello", receivedMsg.Content)
	}
	if receivedMsg.SessionID != "sess123" {
		t.Errorf("SessionID = %q, want sess123", receivedMsg.SessionID)
	}
}

func TestBridgePlatform_handleMessage_NoHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)
	bp.handler = nil // no handler set

	err := bp.handleMessage(&WireMessage{
		Type:       BridgeMsgTypeMessage,
		SessionKey: "sess123",
		Content:    "hello",
	})
	if err != nil {
		t.Errorf("handleMessage with no handler: error = %v, want nil", err)
	}
}

func TestBridgePlatform_handleEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	err := bp.handleEvent(&WireMessage{
		Type:       BridgeMsgTypeEvent,
		Event:      "typing",
		SessionKey: "sess123",
	})
	if err != nil {
		t.Errorf("handleEvent: unexpected error: %v", err)
	}
}

func TestBridgePlatform_handleUnknown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	err := bp.handleUnknown(&WireMessage{Type: "unknown_type"})
	if err == nil {
		t.Error("handleUnknown: want error, got nil")
	}
}

// ---------------------------------------------------------------------------
// BridgePlatform writeJSON / writeError
// ---------------------------------------------------------------------------

func TestBridgePlatform_writeJSON_WithNilConnAndTestHook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	// When testWriteJSON is set, it is called instead of the real writeJSON.
	// This test verifies the hook mechanism works.
	called := false
	bp.testWriteJSON = func(wm *WireMessage) error {
		called = true
		return nil
	}

	err := bp.writeJSON(&WireMessage{Type: BridgeMsgTypeReply})
	if err != nil {
		t.Errorf("writeJSON with test hook: error = %v, want nil", err)
	}
	if !called {
		t.Error("testWriteJSON hook was not called")
	}
}

// ---------------------------------------------------------------------------
// BridgePlatform deliveryLoop
// ---------------------------------------------------------------------------

func TestBridgePlatform_deliveryLoop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	delivered := make(chan struct{}, 1)
	var captured *WireMessage
	bp.testWriteJSON = func(wm *WireMessage) error {
		captured = wm
		delivered <- struct{}{}
		return nil
	}

	go bp.deliveryLoop()

	msg := &base.ChatMessage{
		Platform:  "hotplex",
		SessionID: "sess123",
		Content:   "Hello from hotplex",
		Metadata: map[string]any{
			base.KeyRoomID:   "room1",
			base.KeyThreadID: "thread1",
			base.KeyUserID:   "user1",
		},
	}

	select {
	case bp.msgChan <- msg:
	default:
		t.Fatal("msgChan is full")
	}

	select {
	case <-delivered:
		// message delivered
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message delivery")
	}

	if captured == nil {
		t.Fatal("writeJSON was not called")
	}
	if captured.Type != BridgeMsgTypeMessage {
		t.Errorf("Type = %q, want %q", captured.Type, BridgeMsgTypeMessage)
	}
	if captured.Content != "Hello from hotplex" {
		t.Errorf("Content = %q, want Hello from hotplex", captured.Content)
	}

	bp.close()
}

func TestBridgePlatform_deliveryLoop_NilMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	called := false
	bp.testWriteJSON = func(wm *WireMessage) error {
		called = true
		return nil
	}

	go bp.deliveryLoop()

	select {
	case bp.msgChan <- nil:
	default:
		t.Fatal("msgChan is full")
	}

	// Wait for deliveryLoop to process and skip the nil message.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	<-ctx.Done()

	if called {
		t.Error("writeJSON should not be called for nil message")
	}
	bp.close()
}

func TestBridgePlatform_deliveryLoop_SessionKeyMapping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	delivered := make(chan struct{}, 1)
	var captured *WireMessage
	bp.testWriteJSON = func(wm *WireMessage) error {
		captured = wm
		delivered <- struct{}{}
		return nil
	}

	bp.mu.Lock()
	bp.sessionMap["hotplex-session-1"] = "bridge-session-key"
	bp.mu.Unlock()

	go bp.deliveryLoop()

	msg := &base.ChatMessage{
		Platform:  "hotplex",
		SessionID: "hotplex-session-1",
		Content:   "mapped",
	}

	select {
	case bp.msgChan <- msg:
	default:
		t.Fatal("msgChan is full")
	}

	select {
	case <-delivered:
		// message delivered
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message delivery")
	}

	if captured == nil {
		t.Fatal("writeJSON was not called")
	}
	if captured.SessionKey != "bridge-session-key" {
		t.Errorf("SessionKey = %q, want bridge-session-key", captured.SessionKey)
	}
	bp.close()
}

// ---------------------------------------------------------------------------
// BridgeServer lifecycle
// ---------------------------------------------------------------------------

func TestBridgeServer_Shutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	ctx := context.Background()
	if err := bs.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown(): error = %v, want nil", err)
	}
}

func TestBridgeServer_InjectAdapterManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	bs.InjectAdapterManager(nil) // nil should not panic

	if bs.getAdapterManager() != nil {
		t.Error("getAdapterManager() with nil injection: want nil")
	}
}

func TestBridgeServer_SetHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	handler := func(ctx context.Context, msg *base.ChatMessage) error { return nil }
	bs.SetHandler(handler)
	if bs.handler == nil {
		t.Error("SetHandler: handler is nil after set")
	}
}

func TestBridgePlatform_SetHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	handler := func(ctx context.Context, msg *base.ChatMessage) error { return nil }
	bp.SetHandler(handler)
	if bp.handler == nil {
		t.Error("SetHandler: handler is nil after set")
	}
}

func TestBridgePlatform_HandleMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	// HandleMessage should return nil (not used for bridge platforms)
	err := bp.HandleMessage(context.Background(), &base.ChatMessage{Content: "test"})
	if err != nil {
		t.Errorf("HandleMessage: error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// BridgeServer register/unregister
// ---------------------------------------------------------------------------

func TestBridgeServer_register_unregister(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	bp1 := bs.newBridgePlatform("dingtalk", nil)
	bp2 := bs.newBridgePlatform("wechat", nil)

	bs.register(bp1)
	bs.register(bp2)

	if platforms := bs.ListPlatforms(); len(platforms) != 2 {
		t.Errorf("ListPlatforms = %v, want 2 platforms", platforms)
	}

	if p := bs.GetPlatform("dingtalk"); p == nil {
		t.Error("GetPlatform(dingtalk) = nil, want platform")
	}

	bs.unregister("dingtalk")
	if platforms := bs.ListPlatforms(); len(platforms) != 1 {
		t.Errorf("After unregister, ListPlatforms = %v, want 1", platforms)
	}
}

func TestBridgePlatform_GetAdapterManager_NilServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	// server.adapterMgr is nil by default
	if bp.getAdapterManager() != nil {
		t.Error("getAdapterManager() with nil adapterMgr: want nil")
	}
}

func TestBridgePlatform_close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)
	bp := bs.newBridgePlatform("dingtalk", nil)

	// close should be safe to call multiple times
	bp.close()
	bp.close() // should not panic

	select {
	case <-bp.done:
		// done is closed as expected
	default:
		t.Error("done channel should be closed after close()")
	}
}

func TestBridgeServer_Close_WithPlatforms(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	bp := bs.newBridgePlatform("dingtalk", nil)
	bs.register(bp)

	bs.Close()

	if platforms := bs.ListPlatforms(); len(platforms) != 0 {
		t.Errorf("After Close, ListPlatforms = %v, want empty", platforms)
	}
}

func TestBridgeServer_GetPlatform_NotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	bs := NewBridgeServer(0, "", logger)

	if p := bs.GetPlatform("nonexistent"); p != nil {
		t.Errorf("GetPlatform(nonexistent) = %v, want nil", p)
	}
}
