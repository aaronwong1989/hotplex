package feishu

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hrygo/hotplex/chatapps/base"
)

// EventHandler provides common event handling logic (DRY principle)
type EventHandler struct {
	adapter *Adapter
	logger  *slog.Logger
}

// NewEventHandler creates a new event handler
func NewEventHandler(adapter *Adapter) *EventHandler {
	logger := adapter.Logger()
	if logger == nil {
		logger = slog.Default()
	}
	return &EventHandler{
		adapter: adapter,
		logger:  logger,
	}
}

// ProcessEvent is a common event processing pipeline
func (h *EventHandler) ProcessEvent(w http.ResponseWriter, r *http.Request, parseFunc func([]byte) (interface{}, error), handleFunc func(interface{}) error) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := base.ReadBody(r)
	if err != nil {
		h.logger.Error("Read body failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Verify signature
	if err := h.adapter.verifySignature(r, body); err != nil {
		h.logger.Warn("Invalid signature", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse event
	event, err := parseFunc(body)
	if err != nil {
		h.logger.Error("Parse event failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Handle event
	if err := handleFunc(event); err != nil {
		h.logger.Error("Handle event failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// HandleURLVerification handles URL verification for Feishu webhooks
func (h *EventHandler) HandleURLVerification(w http.ResponseWriter, token string, eventType string) bool {
	if eventType == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"challenge":"` + token + `"}`))
		return true
	}
	return false
}

// WriteJSONResponse writes a JSON response
func (h *EventHandler) WriteJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteOKResponse writes a simple OK response
func (h *EventHandler) WriteOKResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}
