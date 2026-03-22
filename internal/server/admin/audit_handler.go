package admin

import (
	"net/http"
	"strconv"

	"github.com/hrygo/hotplex/internal/adminapi"
	intengine "github.com/hrygo/hotplex/internal/engine"
)

// AuditHandler handles audit and history endpoints.
type AuditHandler struct {
	eventBuffer   *EventBuffer
	sessionPool   interface {
		GetSession(sessionID string) (*intengine.Session, bool)
	}
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(eventBuffer *EventBuffer, sessionPool interface {
	GetSession(sessionID string) (*intengine.Session, bool)
}) *AuditHandler {
	return &AuditHandler{
		eventBuffer: eventBuffer,
		sessionPool: sessionPool,
	}
}

// getEvents handles GET /api/v1/admin/events.
func (h *AuditHandler) getEvents(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	cursorStr := r.URL.Query().Get("cursor")

	limit := 100
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	// cursor takes precedence over offset (cursor-based pagination)
	if cursorStr != "" {
		if c, err := strconv.Atoi(cursorStr); err == nil && c >= 0 {
			offset = c
		}
	} else if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var events []AdminEvent
	var total int64

	if h.eventBuffer != nil {
		events, total = h.eventBuffer.GetEventsPaginated(limit, offset)
	} else {
		events = []AdminEvent{}
		total = 0
	}

	// Calculate next cursor
	var nextCursor string
	if offset+limit < int(total) {
		nextCursor = strconv.Itoa(offset + limit)
	}

	response := EventsResponse{
		Events:     events,
		NextCursor: nextCursor,
		Total:      total,
	}
	adminapi.WriteJSON(w, http.StatusOK, response)
}

// getTranscript handles GET /api/v1/admin/sessions/:id/transcript.
// Note: Full transcript retrieval requires MessageStore integration.
// This is a placeholder that returns session metadata.
func (h *AuditHandler) getTranscript(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r)
	if sessionID == "" {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Missing session ID")
		return
	}

	sess, ok := h.sessionPool.GetSession(sessionID)
	if !ok {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Session not found")
		return
	}

	// Build basic transcript from session data
	// Full message history requires MessageStore integration
	messages := []TranscriptMsg{
		{
			Type:      "session_start",
			Timestamp: sess.CreatedAt,
			Content:   "Session started",
		},
	}

	response := TranscriptResponse{
		SessionID: sessionID,
		Messages:  messages,
	}
	adminapi.WriteJSON(w, http.StatusOK, response)
}

// PushEvent is a helper to add an event to the buffer.
func (h *AuditHandler) PushEvent(event AdminEvent) {
	if h.eventBuffer != nil {
		h.eventBuffer.Push(event)
	}
}
