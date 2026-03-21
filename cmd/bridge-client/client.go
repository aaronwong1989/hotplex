// Package bridgeclient provides a Go SDK for connecting external platform adapters
// to HotPlex via the BridgeServer WebSocket gateway.
//
// Usage:
//
//	client := bridgeclient.New(
//		bridgeclient.URL("wss://hotplex.internal:8080/bridge"),
//		bridgeclient.Platform("dingtalk"),
//		bridgeclient.Capabilities(bridgeclient.CapText, bridgeclient.CapCard),
//		bridgeclient.AuthToken(os.Getenv("HOTPLEX_BRIDGE_TOKEN")),
//	)
//
//	client.OnMessage(func(msg *bridgeclient.Message) *bridgeclient.Reply {
//		// Process msg.Content, msg.SessionKey, msg.Metadata
//		return &bridgeclient.Reply{
//			Content:    "Hello from DingTalk!",
//			SessionKey: msg.SessionKey,
//		}
//	})
//
//	ctx := context.Background()
//	if err := client.Connect(ctx); err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Block until disconnected
//	<-ctx.Done()
package bridgeclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// =============================================================================
// Capability Constants
// =============================================================================

// Capabilities that an adapter can declare when registering with BridgeServer.
const (
	CapText    = "text"
	CapImage   = "image"
	CapCard    = "card"
	CapButtons = "buttons"
	CapTyping  = "typing"
	CapEdit    = "edit"
	CapDelete  = "delete"
	CapReact   = "react"
	CapThread  = "thread"
)

// =============================================================================
// Wire Protocol Types
// =============================================================================

// msgType constants (matches internal/server/bridge.go).
const (
	msgTypeRegister = "register"
	msgTypeMessage  = "message"
	msgTypeReply    = "reply"
	msgTypeEvent    = "event"
	msgTypeError    = "error"
)

// wireMessage is the bidirectional JSON envelope for Bridge Wire Protocol.
type wireMessage struct {
	Type         string          `json:"type"`
	Platform     string          `json:"platform,omitempty"`
	Token        string          `json:"token,omitempty"`
	SessionKey   string          `json:"session_key,omitempty"`
	Content      string          `json:"content,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Event        string          `json:"event,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	Code         int             `json:"code,omitempty"`
	Message      string          `json:"message,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
}

// metadata carries user/room/thread identity inside a WireMessage.
type metadata struct {
	UserID   string `json:"user_id,omitempty"`
	RoomID   string `json:"room_id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	Platform string `json:"platform,omitempty"`
}

// =============================================================================
// Public Types
// =============================================================================

// Message represents an inbound message from HotPlex (server → client).
type Message struct {
	SessionKey string
	Content    string
	Metadata   Metadata
	raw        *wireMessage
}

// Metadata holds message identity fields.
type Metadata struct {
	UserID   string
	RoomID   string
	ThreadID string
	Platform string
}

// Reply is returned by a MessageHandler to send a response back to HotPlex.
type Reply struct {
	Content    string
	SessionKey string
	Metadata   Metadata
}

// Event represents an inbound event from HotPlex (server → client).
// Common events: "stream_start", "stream_chunk", "stream_end", "typing_start", "typing_end".
type Event struct {
	Event   string
	Data    json.RawMessage
	raw     *wireMessage
}

// Error represents an error message from HotPlex (server → client).
type Error struct {
	Code    int
	Message string
}

// MessageHandler processes inbound messages from HotPlex and optionally returns a reply.
type MessageHandler func(msg *Message) *Reply

// EventHandler processes inbound events from HotPlex.
type EventHandler func(evt *Event)

// =============================================================================
// Client
// =============================================================================

// Client is a BridgeClient that connects to HotPlex BridgeServer as a WebSocket client.
type Client struct {
	url        string
	platform   string
	token      string
	caps       []string
	logger     *slog.Logger
	httpClient *http.Client

	msgHandler  MessageHandler
	eventHandler EventHandler

	conn   *websocket.Conn
	done   chan struct{}
	closed bool
	mu     sync.RWMutex

	// for dialing
	dialer websocket.Dialer
}

// New creates a new BridgeClient with the given options.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		logger: slog.Default(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		dialer: websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
		caps: []string{CapText}, // default capability
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.url == "" {
		return nil, errors.New("bridgeclient: URL is required")
	}
	if c.platform == "" {
		return nil, errors.New("bridgeclient: platform name is required")
	}
	return c, nil
}

// Connect establishes a WebSocket connection to BridgeServer, performs the
// register handshake, and starts background goroutines to handle incoming
// messages and events. It blocks until the context is cancelled or a fatal
// error occurs.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.dial(ctx); err != nil {
		return fmt.Errorf("bridgeclient dial: %w", err)
	}

	if err := c.register(); err != nil {
		c.closeConn()
		return fmt.Errorf("bridgeclient register: %w", err)
	}

	go c.readLoop()
	return nil
}

// SendMessage sends a message from the external platform to HotPlex.
// This is used for inbound messages (e.g., a user sends a DM on DingTalk).
func (c *Client) SendMessage(ctx context.Context, msg *Message) error {
	meta := metadata{
		UserID:   msg.Metadata.UserID,
		RoomID:   msg.Metadata.RoomID,
		ThreadID: msg.Metadata.ThreadID,
		Platform: c.platform,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	wm := wireMessage{
		Type:       msgTypeMessage,
		Platform:   c.platform,
		SessionKey: msg.SessionKey,
		Content:    msg.Content,
		Metadata:   metaBytes,
	}
	return c.writeJSON(ctx, wm)
}

// Typing sends a typing indicator to HotPlex.
func (c *Client) Typing(ctx context.Context, sessionKey string) error {
	metaBytes, _ := json.Marshal(metadata{Platform: c.platform})
	evt := wireMessage{
		Type:     msgTypeEvent,
		Platform: c.platform,
		Event:    "typing",
		SessionKey: sessionKey,
		Data:     metaBytes,
	}
	return c.writeJSON(ctx, evt)
}

// Close gracefully shuts down the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.done)
	c.mu.Unlock()

	return c.closeConn()
}

// OnMessage registers a handler for inbound messages from HotPlex.
// Only one handler can be registered; subsequent calls replace the previous one.
func (c *Client) OnMessage(h MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgHandler = h
}

// OnEvent registers a handler for inbound events from HotPlex.
// Only one handler can be registered; subsequent calls replace the previous one.
func (c *Client) OnEvent(h EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = h
}

// =============================================================================
// Internal
// =============================================================================

func (c *Client) dial(ctx context.Context) error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	if c.token != "" {
		q.Set("token", c.token)
	}
	u.RawQuery = q.Encode()

	conn, _, err := c.dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", u.Redacted(), err)
	}
	c.conn = conn
	return nil
}

func (c *Client) register() error {
	c.mu.RLock()
	caps := c.caps
	c.mu.RUnlock()

	wm := wireMessage{
		Type:         msgTypeRegister,
		Platform:     c.platform,
		Capabilities: caps,
	}
	if c.token != "" {
		wm.Token = c.token
	}
	return c.writeJSON(context.Background(), wm)
}

func (c *Client) writeJSON(ctx context.Context, wm wireMessage) error {
	c.mu.RLock()
	conn := c.conn
	closed := c.closed
	c.mu.RUnlock()

	if conn == nil || closed {
		return errors.New("bridgeclient: not connected")
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- conn.WriteJSON(wm)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("write json: %w", err)
		}
	}
	return nil
}

func (c *Client) readLoop() {
	defer c.cleanup()

	for {
		var wm wireMessage
		err := c.conn.ReadJSON(&wm)
		if err != nil {
			if !c.isClosed() {
				c.logger.Debug("bridge read error", "error", err)
			}
			return
		}
		c.handleMessage(&wm)
	}
}

func (c *Client) handleMessage(wm *wireMessage) {
	switch wm.Type {
	case msgTypeMessage:
		c.handleInboundMessage(wm)
	case msgTypeEvent:
		c.handleInboundEvent(wm)
	case msgTypeError:
		c.handleError(wm)
	default:
		c.logger.Debug("unknown wire message type", "type", wm.Type)
	}
}

func (c *Client) handleInboundMessage(wm *wireMessage) {
	meta := Metadata{Platform: "hotplex"}
	if len(wm.Metadata) > 0 {
		var m metadata
		if err := json.Unmarshal(wm.Metadata, &m); err != nil {
			c.logger.Debug("unmarshal metadata", "error", err)
		} else {
			meta = Metadata{
				UserID:   m.UserID,
				RoomID:   m.RoomID,
				ThreadID: m.ThreadID,
				Platform: m.Platform,
			}
		}
	}

	msg := &Message{
		SessionKey: wm.SessionKey,
		Content:    wm.Content,
		Metadata:   meta,
		raw:        wm,
	}

	c.mu.RLock()
	handler := c.msgHandler
	c.mu.RUnlock()

	if handler == nil {
		return
	}
	reply := handler(msg)
	if reply == nil {
		return
	}

	c.sendReply(reply)
}

func (c *Client) handleInboundEvent(wm *wireMessage) {
	evt := &Event{
		Event: wm.Event,
		Data:  wm.Data,
		raw:   wm,
	}

	c.mu.RLock()
	handler := c.eventHandler
	c.mu.RUnlock()

	if handler != nil {
		handler(evt)
	}
}

func (c *Client) handleError(wm *wireMessage) {
	c.logger.Error("bridge error",
		"code", wm.Code,
		"message", wm.Message,
	)
}

func (c *Client) sendReply(reply *Reply) {
	metaBytes, _ := json.Marshal(metadata{
		Platform: c.platform,
		UserID:   reply.Metadata.UserID,
		RoomID:   reply.Metadata.RoomID,
		ThreadID: reply.Metadata.ThreadID,
	})
	wm := wireMessage{
		Type:       msgTypeReply,
		Platform:   c.platform,
		SessionKey: reply.SessionKey,
		Content:    reply.Content,
		Metadata:   metaBytes,
	}
	// Best-effort; don't block on reply failure
	go func() {
		if err := c.writeJSON(context.Background(), wm); err != nil {
			c.logger.Debug("send reply failed", "error", err)
		}
	}()
}

func (c *Client) cleanup() {
	c.Close()
}

func (c *Client) closeConn() error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil
	}

	// Send close frame; don't block
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	return conn.Close()
}

func (c *Client) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}
