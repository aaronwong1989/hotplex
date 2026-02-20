package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hrygo/hotplex/pkg/hotplex"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// For MVP, allow all origins
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ClientRequest represents the JSON payload expected from the WebSocket client
type ClientRequest struct {
	Type      string `json:"type"`       // e.g. "execute"
	SessionID string `json:"session_id"` // Provide session_id to hot-multiplex
	Prompt    string `json:"prompt"`     // The user input
	WorkDir   string `json:"work_dir"`   // Working directory for CLI
}

// ServerResponse represents the JSON payload sent to the WebSocket client
type ServerResponse struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// WebSocketHandler manages a WebSocket connection to a HotPlex Engine.
type WebSocketHandler struct {
	engine hotplex.HotPlexClient
	logger *slog.Logger
}

// NewWebSocketHandler creates a new handler.
func NewWebSocketHandler(engine hotplex.HotPlexClient, logger *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		engine: engine,
		logger: logger,
	}
}

// ServeHTTP upgrades the HTTP connection and starts the read loop.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade websocket connection", "error", err)
		return
	}
	defer conn.Close()

	h.logger.Info("Client connected via WebSocket", "addr", r.RemoteAddr)

	for {
		// Read message from client
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("WebSocket closed unexpectedly", "error", err)
			} else {
				h.logger.Info("WebSocket closed normally", "addr", r.RemoteAddr)
			}
			return
		}

		if messageType != websocket.TextMessage {
			h.logger.Warn("Ignoring non-text message type", "type", messageType)
			continue
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			h.sendError(conn, "Invalid JSON payload: "+err.Error())
			continue
		}

		if req.Type == "execute" {
			h.handleExecute(conn, req)
		} else {
			h.sendError(conn, "Unknown request type: "+req.Type)
		}
	}
}

func (h *WebSocketHandler) handleExecute(conn *websocket.Conn, req ClientRequest) {
	if req.Prompt == "" {
		h.sendError(conn, "prompt cannot be empty")
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		// Auto-generate session ID if not provided
		sessionID = uuid.New().String()
		h.logger.Debug("Auto-generated session ID", "session_id", sessionID)
	}

	workDir := req.WorkDir
	if workDir == "" {
		workDir = "/tmp/hotplex_sandbox" // Fallback MVP directory
	}

	cfg := &hotplex.Config{
		WorkDir:   workDir,
		SessionID: sessionID,
		UserID:    1, // Default user
	}

	h.logger.Info("Handling execute request", "session_id", sessionID, "prompt_length", len(req.Prompt))

	// Define the callback that bridges HotPlex Engine events to WebSocket messages
	cb := func(eventType string, data any) error {
		resp := ServerResponse{
			Event: eventType,
			Data:  data,
		}

		val, err := json.Marshal(resp)
		if err != nil {
			h.logger.Error("Failed to marshal event response", "error", err)
			return nil // Keep executing even if JSON fails to marshal
		}

		err = conn.WriteMessage(websocket.TextMessage, val)
		if err != nil {
			h.logger.Error("Failed to write to websocket", "error", err)
			return err // If write fails, we should stop executing
		}
		return nil
	}

	// HotPlex execution
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err := h.engine.Execute(ctx, cfg, req.Prompt, cb)
	if err != nil {
		h.sendError(conn, "Execution failed: "+err.Error())
		return
	}

	// Send completion signal
	h.sendEvent(conn, "completed", map[string]string{"session_id": sessionID})
}

func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	h.sendEvent(conn, "error", map[string]string{"message": message})
}

func (h *WebSocketHandler) sendEvent(conn *websocket.Conn, event string, data any) {
	resp := ServerResponse{Event: event, Data: data}
	val, _ := json.Marshal(resp)
	_ = conn.WriteMessage(websocket.TextMessage, val)
}
