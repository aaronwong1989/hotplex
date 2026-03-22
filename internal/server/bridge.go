// Package server provides the BridgeServer for external platform adapters.
//
// BridgeServer implements a WebSocket gateway that allows external platform adapters
// (e.g., DingTalk, WeChat) to connect to HotPlex without being compiled into the
// same binary. External adapters connect as WebSocket clients and communicate via
// the Bridge Wire Protocol.
//
// Architecture:
//
//	External Adapter (DingTalk) ──WebSocket──> BridgeServer ──ChatAdapter──> Engine
//	                                          <────────────── reply ───────────────
//
// BridgePlatform implements base.ChatAdapter, so HotPlex's engine handler is
// agnostic to whether the platform is built-in or external via BridgeServer.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/internal/bridgewire"
	"github.com/hrygo/hotplex/types"
)

// BridgeMsgType* aliases are kept for backward compatibility within the server package.
const (
	BridgeMsgTypeRegister = bridgewire.MsgTypeRegister
	BridgeMsgTypeMessage  = bridgewire.MsgTypeMessage
	BridgeMsgTypeReply    = bridgewire.MsgTypeReply
	BridgeMsgTypeEvent    = bridgewire.MsgTypeEvent
	BridgeMsgTypeError    = bridgewire.MsgTypeError
)

// Cap* aliases for internal use (mirrors bridgewire constants).
const (
	CapText    = bridgewire.CapText
	CapImage   = bridgewire.CapImage
	CapCard    = bridgewire.CapCard
	CapButtons = bridgewire.CapButtons
	CapTyping  = bridgewire.CapTyping
	CapEdit    = bridgewire.CapEdit
	CapDelete  = bridgewire.CapDelete
	CapReact   = bridgewire.CapReact
	CapThread  = bridgewire.CapThread
)

// AllCapabilities re-exports the shared list for internal use.
var allCapabilities = bridgewire.AllCapabilities

// WireMessage and WireMetadata are type aliases to the shared bridgewire package.
type WireMessage = bridgewire.WireMessage
type WireMetadata = bridgewire.WireMetadata

// =============================================================================
// BridgeServer
// =============================================================================

// BridgeServer is a WebSocket gateway for external platform adapters.
// It implements http.Handler and bridges the Bridge Wire Protocol to HotPlex's
// internal ChatMessage types.
type BridgeServer struct {
	port       int
	token      string
	platforms  map[string]*BridgePlatform
	upgrader   websocket.Upgrader
	logger     *slog.Logger
	handler    base.MessageHandler
	adapterMgr any // *chatapps.AdapterManager (any to avoid import cycle)
	mu         sync.RWMutex
	server     *http.Server
}

// NewBridgeServer creates a new BridgeServer.
func NewBridgeServer(port int, token string, logger *slog.Logger) *BridgeServer {
	if logger == nil {
		logger = slog.Default()
	}
	bs := &BridgeServer{
		port:      port,
		token:     token,
		platforms: make(map[string]*BridgePlatform),
		logger:    logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Bridge clients are authenticated via token
			},
		},
	}
	return bs
}

// SetHandler sets the MessageHandler that receives all messages from bridge adapters.
// This is called by main.go after the chatapps EngineMessageHandler is created.
func (s *BridgeServer) SetHandler(h base.MessageHandler) {
	s.handler = h
}

// InjectAdapterManager injects the chatapps AdapterManager for registering
// bridge platforms so they can receive events via the standard event pipeline.
// Pass nil to disable event routing for bridge platforms.
func (s *BridgeServer) InjectAdapterManager(adapterMgr any) {
	s.adapterMgr = adapterMgr
}

// GetPlatform returns the BridgePlatform for a registered platform, or nil.
func (s *BridgeServer) GetPlatform(name string) *BridgePlatform {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.platforms[name]
	if !ok {
		return nil
	}
	return p
}

// ListPlatforms returns all registered platform names.
func (s *BridgeServer) ListPlatforms() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.platforms))
	for n := range s.platforms {
		names = append(names, n)
	}
	return names
}

// ServeHTTP implements http.Handler for the /bridge/v1/ endpoint.
func (s *BridgeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract platform name from path: /bridge/v1/connect/{platform}
	path := strings.TrimPrefix(r.URL.Path, "/bridge/v1/")
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "platform name required", http.StatusBadRequest)
		return
	}
	platform := parts[0]

	// Authenticate via token
	token := r.URL.Query().Get("token")
	if s.token != "" && token != s.token {
		s.logger.Warn("Bridge auth failed", "platform", platform, "reason", "invalid_token")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Bridge upgrade failed", "platform", platform, "error", err)
		return
	}

	bp := s.newBridgePlatform(platform, conn)
	s.register(bp)
	defer s.unregister(platform)

	s.logger.Info("Bridge platform connected", "platform", platform, "addr", r.RemoteAddr)
	bp.readLoop()
}

// register adds a platform to the registry.
func (s *BridgeServer) register(bp *BridgePlatform) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.platforms[bp.platform] = bp
}

// unregister removes a platform from the registry and AdapterManager.
func (s *BridgeServer) unregister(platform string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.platforms, platform)
	// Also remove from AdapterManager so events stop routing to this platform
	if am := s.getAdapterManager(); am != nil {
		am.Unregister(platform)
	}
}

// getAdapterManager returns the injected AdapterManager, if available.
func (s *BridgeServer) getAdapterManager() adapterManagerInterface {
	if s.adapterMgr == nil {
		return nil
	}
	if ami, ok := s.adapterMgr.(adapterManagerInterface); ok {
		return ami
	}
	return nil
}

// adapterManagerInterface is the subset of *chatapps.AdapterManager needed by BridgeServer.
// Defined as interface to avoid import cycle.
type adapterManagerInterface interface {
	RegisterOrReplace(any) // ChatAdapter
	Unregister(platform string)
}

// Close gracefully shuts down all bridge connections.
func (s *BridgeServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, bp := range s.platforms {
		bp.close()
	}
	s.platforms = make(map[string]*BridgePlatform)
}

// =============================================================================
// BridgePlatform
// =============================================================================

// BridgePlatform implements base.ChatAdapter for an external platform connected
// via BridgeServer WebSocket. It translates between the Bridge Wire Protocol and
// HotPlex's internal ChatMessage type.
type BridgePlatform struct {
	server        *BridgeServer
	platform      string
	conn          *websocket.Conn
	caps          []string
	handler       base.MessageHandler
	msgChan       chan *base.ChatMessage
	eventChan     chan *WireMessage
	sessionMap    map[string]string // sessionKey → sessionID
	done          chan struct{}
	mu            sync.RWMutex
	logger        *slog.Logger
	testWriteJSON func(wm *WireMessage) error // test hook for writeJSON
}

var (
	_ base.ChatAdapter = (*BridgePlatform)(nil)
	_ http.Handler     = (*BridgeServer)(nil)
)

// newBridgePlatform creates a new BridgePlatform for a WebSocket connection.
func (s *BridgeServer) newBridgePlatform(platform string, conn *websocket.Conn) *BridgePlatform {
	return &BridgePlatform{
		server:     s,
		platform:   platform,
		conn:       conn,
		caps:       []string{CapText}, // defaults
		msgChan:    make(chan *base.ChatMessage, 100),
		eventChan:  make(chan *WireMessage, 100),
		sessionMap: make(map[string]string),
		done:       make(chan struct{}),
		logger:     s.logger,
	}
}

// readLoop processes incoming WebSocket messages from the external adapter.
func (bp *BridgePlatform) readLoop() {
	defer bp.close()

	for {
		msgType, p, err := bp.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				bp.logger.Debug("Bridge read error", "platform", bp.platform, "error", err)
			}
			return
		}

		if msgType != websocket.TextMessage {
			continue
		}

		var wm WireMessage
		if err := json.Unmarshal(p, &wm); err != nil {
			_ = bp.writeError(400, "invalid JSON: "+err.Error())
			continue
		}

		if err := bp.handleWireMessage(&wm); err != nil {
			bp.logger.Error("Bridge message handle error",
				"platform", bp.platform,
				"type", wm.Type,
				"error", err)
			_ = bp.writeError(500, err.Error())
		}
	}
}

// deliveryLoop drains msgChan and writes each ChatMessage to the WebSocket.
func (bp *BridgePlatform) deliveryLoop() {
	for {
		select {
		case <-bp.done:
			return
		case msg := <-bp.msgChan:
			if msg == nil {
				continue
			}
			wm, _ := bp.buildWireMessage(msg)
			if err := bp.writeJSON(wm); err != nil {
				bp.logger.Debug("Bridge delivery failed, message dropped",
					"platform", bp.platform,
					"session_id", msg.SessionID,
					"error", err)
			}
		}
	}
}
// handleWireMessage dispatches a WireMessage to the appropriate handler.
func (bp *BridgePlatform) handleWireMessage(wm *WireMessage) error {
	switch wm.Type {
	case BridgeMsgTypeRegister:
		return bp.handleRegister(wm)
	case BridgeMsgTypeMessage:
		return bp.handleMessage(wm)
	case BridgeMsgTypeEvent:
		return bp.handleEvent(wm)
	case BridgeMsgTypeReply:
		// Async reply acknowledgment — currently a no-op
		bp.logger.Debug("Bridge reply ack received", "platform", bp.platform, "session_key", wm.SessionKey)
		return nil
	default:
		return bp.handleUnknown(wm)
	}
}

// handleRegister processes the registration handshake from an external adapter.
func (bp *BridgePlatform) handleRegister(wm *WireMessage) error {
	if wm.Platform == "" {
		return errors.New("register: platform name required")
	}
	bp.mu.Lock()
	bp.platform = wm.Platform
	if len(wm.Capabilities) > 0 {
		bp.caps = wm.Capabilities
	}
	bp.mu.Unlock()

	// Update registration in server
	bp.server.mu.Lock()
	bp.server.platforms[wm.Platform] = bp
	bp.server.mu.Unlock()

	// Register with AdapterManager so events flow through the standard pipeline
	if am := bp.getAdapterManager(); am != nil {
		am.RegisterOrReplace(bp)
		bp.logger.Info("BridgePlatform registered with AdapterManager",
			"platform", wm.Platform,
			"capabilities", bp.caps)
	}

	bp.logger.Info("Bridge platform registered",
		"platform", wm.Platform,
		"capabilities", bp.caps)

	// Send ack
	return bp.writeJSON(&WireMessage{
		Type:     BridgeMsgTypeReply,
		Platform: "hotplex",
		Content:  "registered",
	})
}

// getAdapterManager returns the injected AdapterManager, if available.
func (bp *BridgePlatform) getAdapterManager() adapterManagerInterface {
	am := bp.server.adapterMgr
	if am == nil {
		return nil
	}
	if ami, ok := am.(adapterManagerInterface); ok {
		return ami
	}
	return nil
}

// handleMessage converts an inbound WireMessage to ChatMessage and forwards to handler.
func (bp *BridgePlatform) handleMessage(wm *WireMessage) error {
	if wm.SessionKey == "" {
		return errors.New("message: session_key required")
	}

	// Parse metadata
	var meta WireMetadata
	if len(wm.Metadata) > 0 {
		_ = json.Unmarshal(wm.Metadata, &meta)
	}

	msg := &base.ChatMessage{
		Platform:  bp.platform,
		SessionID: wm.SessionKey,
		Content:   wm.Content,
		Type:      types.MessageTypeUserInput,
		Metadata: map[string]any{
			base.KeyRoomID:   meta.RoomID,
			base.KeyThreadID: meta.ThreadID,
			base.KeyUserID:   meta.UserID,
			"platform":      bp.platform,
		},
		Timestamp: time.Now(),
	}

	// Store sessionKey → sessionID mapping
	bp.mu.Lock()
	bp.sessionMap[msg.SessionID] = wm.SessionKey
	bp.mu.Unlock()

	// Forward to HotPlex handler
	if bp.handler != nil {
		return bp.handler(context.Background(), msg)
	}
	return nil
}

// handleEvent forwards platform events to HotPlex (e.g., typing indicators).
func (bp *BridgePlatform) handleEvent(wm *WireMessage) error {
	bp.logger.Debug("Bridge event received",
		"platform", bp.platform,
		"event", wm.Event,
		"session_key", wm.SessionKey)
	// Events are informational — forward to handler if it cares
	return nil
}

// handleUnknown handles unrecognized message types.
func (bp *BridgePlatform) handleUnknown(wm *WireMessage) error {
	bp.logger.Warn("Unknown bridge message type", "platform", bp.platform, "type", wm.Type)
	return bp.writeError(400, "unknown message type: "+wm.Type)
}

// writeJSON sends a WireMessage to the WebSocket connection.
func (bp *BridgePlatform) writeJSON(wm *WireMessage) error {
	// Test hook for mocking WebSocket writes
	if bp.testWriteJSON != nil {
		return bp.testWriteJSON(wm)
	}

	bp.mu.RLock()
	conn := bp.conn
	bp.mu.RUnlock()

	if conn == nil {
		return errors.New("connection closed")
	}

	data, err := json.Marshal(wm)
	if err != nil {
		return err
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// writeError sends an error WireMessage.
func (bp *BridgePlatform) writeError(code int, message string) error {
	return bp.writeJSON(&WireMessage{
		Type:    BridgeMsgTypeError,
		Code:    code,
		Message: message,
	})
}

// close closes the WebSocket connection.
func (bp *BridgePlatform) close() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	select {
	case <-bp.done:
	default:
		close(bp.done)
	}

	if bp.conn != nil {
		_ = bp.conn.Close()
		bp.conn = nil
	}
}

// =============================================================================
// base.ChatAdapter Implementation
// =============================================================================

// Platform returns the platform name.
func (bp *BridgePlatform) Platform() string {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.platform
}

// SystemPrompt returns a default system prompt for bridge platforms.
func (bp *BridgePlatform) SystemPrompt() string {
	return "You are a helpful AI assistant."
}

// Start starts the bridge platform (no-op; connection is established externally).
func (bp *BridgePlatform) Start(ctx context.Context) error {
	return nil
}

// Stop stops the bridge platform.
func (bp *BridgePlatform) Stop() error {
	bp.close()
	return nil
}

// SendMessage sends a ChatMessage to the external platform via WebSocket.
func (bp *BridgePlatform) SendMessage(ctx context.Context, sessionID string, msg *base.ChatMessage) error {
	if msg == nil {
		return nil
	}
	wm, _ := bp.buildWireMessage(msg)
	return bp.writeJSON(wm)
}

// buildWireMessage builds a Bridge WireMessage from a ChatMessage.
func (bp *BridgePlatform) buildWireMessage(msg *base.ChatMessage) (*WireMessage, []byte) {
	meta := WireMetadata{Platform: "hotplex"}
	if msg.Metadata != nil {
		if v, ok := msg.Metadata[base.KeyRoomID].(string); ok {
			meta.RoomID = v
		}
		if v, ok := msg.Metadata[base.KeyThreadID].(string); ok {
			meta.ThreadID = v
		}
		if v, ok := msg.Metadata[base.KeyUserID].(string); ok {
			meta.UserID = v
		}
	}
	metaBytes, _ := json.Marshal(meta)

	sessionKey := msg.SessionID
	bp.mu.RLock()
	if sk, ok := bp.sessionMap[msg.SessionID]; ok {
		sessionKey = sk
	}
	bp.mu.RUnlock()

	return &WireMessage{
		Type:       BridgeMsgTypeMessage,
		Platform:   "hotplex",
		SessionKey: sessionKey,
		Content:    msg.Content,
		Metadata:   metaBytes,
	}, metaBytes
}

// HandleMessage is not used for bridge platforms (they receive via WebSocket).
func (bp *BridgePlatform) HandleMessage(ctx context.Context, msg *base.ChatMessage) error {
	return nil
}

// SetHandler sets the MessageHandler for incoming messages.
func (bp *BridgePlatform) SetHandler(h base.MessageHandler) {
	bp.handler = h
}

// =============================================================================
// Server Lifecycle
// =============================================================================

// ListenAndServe starts the BridgeServer HTTP listener.
// It blocks until the server is closed.
func (s *BridgeServer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/bridge/v1/", s)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	s.server = srv

	s.logger.Info("BridgeServer listening", "addr", addr)
	return srv.ListenAndServe()
}

// Shutdown gracefully shuts down the BridgeServer.
func (s *BridgeServer) Shutdown(ctx context.Context) error {
	s.Close()
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
