package bridgeclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsEchoServer is a test WebSocket server that echoes messages back and sends inbound messages.
func wsEchoServer(t *testing.T) (*httptest.Server, *sync.Mutex, map[string]int, chan wireMessage) {
	t.Helper()
	var mu sync.Mutex
	msgCounts := make(map[string]int)
	inbound := make(chan wireMessage, 10)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Send inbound messages
		go func() {
			for wm := range inbound {
				_ = conn.WriteJSON(wm)
			}
		}()

		for {
			var wm wireMessage
			if err := conn.ReadJSON(&wm); err != nil {
				return
			}
			mu.Lock()
			msgCounts[wm.Type]++
			mu.Unlock()

			// Echo back as reply
			switch wm.Type {
			case msgTypeRegister:
				_ = conn.WriteJSON(wireMessage{Type: msgTypeReply, Content: "registered"})
			case msgTypeMessage:
				_ = conn.WriteJSON(wireMessage{
					Type:       msgTypeMessage,
					SessionKey: wm.SessionKey,
					Content:    "echo: " + wm.Content,
				})
			}
		}
	}

	return httptest.NewServer(http.HandlerFunc(handler)), &mu, msgCounts, inbound
}

func TestClientConnectAndRegister(t *testing.T) {
	server, _, _, inbound := wsEchoServer(t)
	defer server.Close()
	defer close(inbound)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, err := New(
		URL(wsURL),
		Platform("test"),
		Capabilities(CapText),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// Give time for register to complete
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop readLoop
	cancel()
	time.Sleep(50 * time.Millisecond)

	_ = client.Close()
}

func TestClientSendMessage(t *testing.T) {
	server, mu, counts, inbound := wsEchoServer(t)
	defer server.Close()
	defer close(inbound)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, err := New(
		URL(wsURL),
		Platform("test"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	msg := &Message{
		SessionKey: "test-session",
		Content:    "hello",
		Metadata:   Metadata{UserID: "u1"},
	}
	if err := client.SendMessage(ctx, msg); err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := counts[msgTypeMessage]
	mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 message sent, got %d", count)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	_ = client.Close()
}

func TestClientOnMessageHandler(t *testing.T) {
	server, _, _, inbound := wsEchoServer(t)
	defer server.Close()
	defer close(inbound)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, err := New(URL(wsURL), Platform("test"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	received := make(chan *Message, 1)
	client.OnMessage(func(msg *Message) *Reply {
		received <- msg
		return &Reply{Content: "ack", SessionKey: msg.SessionKey}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Send an inbound message from server to client
	inbound <- wireMessage{
		Type:       msgTypeMessage,
		SessionKey: "test-session",
		Content:    "hello from server",
	}

	select {
	case msg := <-received:
		if msg.Content != "hello from server" {
			t.Errorf("expected 'hello from server', got %q", msg.Content)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for message")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	_ = client.Close()
}

func TestClientTyping(t *testing.T) {
	server, mu, counts, inbound := wsEchoServer(t)
	defer server.Close()
	defer close(inbound)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, err := New(URL(wsURL), Platform("test"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := client.Typing(ctx, "test-session"); err != nil {
		t.Errorf("Typing() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := counts[msgTypeEvent]
	mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 event sent, got %d", count)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	_ = client.Close()
}

func TestClientClose(t *testing.T) {
	server, _, _, inbound := wsEchoServer(t)
	defer server.Close()
	defer close(inbound)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, err := New(URL(wsURL), Platform("test"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Second close should be no-op
	if err := client.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestClientNotConnected(t *testing.T) {
	client, err := New(URL("ws://localhost:1"), Platform("test"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	msg := &Message{SessionKey: "s1", Content: "test"}

	err = client.SendMessage(ctx, msg)
	if err == nil {
		t.Error("SendMessage() on unconnected client: want error, got nil")
	}
}

func TestReplySerialization(t *testing.T) {
	reply := &Reply{
		Content:    "response",
		SessionKey: "sess-123",
		Metadata:   Metadata{ThreadID: "t1"},
	}

	metaBytes, _ := json.Marshal(metadata{
		Platform: "test",
		ThreadID: reply.Metadata.ThreadID,
	})
	wm := wireMessage{
		Type:       msgTypeReply,
		Platform:   "test",
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
}
