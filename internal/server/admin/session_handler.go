package admin

import (
	"encoding/json"
	"net/http"

	"github.com/hrygo/hotplex/internal/adminapi"
	intengine "github.com/hrygo/hotplex/internal/engine"
)

// SessionPoolInterface defines the interface for session pool operations needed by admin handlers.
type SessionPoolInterface interface {
	ListActiveSessions() []*intengine.Session
	GetSession(sessionID string) (*intengine.Session, bool)
	TerminateSession(sessionID string) error
}

// SessionHandler handles session management endpoints.
type SessionHandler struct {
	pool SessionPoolInterface
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(pool SessionPoolInterface) *SessionHandler {
	return &SessionHandler{pool: pool}
}

// ListSessions handles GET /api/v1/sessions
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminapi.ParsePagination(r)

	sessions := h.pool.ListActiveSessions()
	total := len(sessions)

	// Apply pagination
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	adminSessions := make([]AdminSession, 0, end-start)
	for _, sess := range sessions[start:end] {
		adminSessions = append(adminSessions, MapSessionToAdminSession(sess))
	}

	resp := SessionsResponse{
		Sessions: adminSessions,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}

	adminapi.WriteJSON(w, http.StatusOK, resp)
}

// GetSession handles GET /api/v1/sessions/:id
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r)
	if sessionID == "" {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Missing session ID")
		return
	}

	sess, ok := h.pool.GetSession(sessionID)
	if !ok {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Session not found")
		return
	}

	adminSess := MapSessionToAdminSession(sess)
	adminapi.WriteJSON(w, http.StatusOK, adminSess)
}

// StopSession handles POST /api/v1/sessions/:id/stop
func (h *SessionHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r)
	if sessionID == "" {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Missing session ID")
		return
	}

	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "admin_stop"
	}

	_, ok := h.pool.GetSession(sessionID)
	if !ok {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Session not found")
		return
	}

	// Initiate termination asynchronously
	go func() {
		_ = h.pool.TerminateSession(sessionID)
	}()

	resp := StopResponse{
		SessionID: sessionID,
		Status:    "stopping",
		Message:   "Session termination initiated",
	}

	if req.Reason != "" {
		resp.Message = "Session termination initiated: " + req.Reason
	}

	adminapi.WriteJSON(w, http.StatusOK, resp)
}

// BatchStopSessions handles POST /api/v1/sessions/batch-stop
func (h *SessionHandler) BatchStopSessions(w http.ResponseWriter, r *http.Request) {
	var req BatchStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid request body")
		return
	}

	if len(req.SessionIDs) == 0 {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "No session IDs provided")
		return
	}

	resp := BatchStopResponse{
		Stopped:  make([]string, 0),
		NotFound: make([]string, 0),
		Failed:   make([]BatchStopFailed, 0),
	}

	for _, sessionID := range req.SessionIDs {
		_, ok := h.pool.GetSession(sessionID)
		if !ok {
			resp.NotFound = append(resp.NotFound, sessionID)
			continue
		}

		if err := h.pool.TerminateSession(sessionID); err != nil {
			resp.Failed = append(resp.Failed, BatchStopFailed{
				SessionID: sessionID,
				Error:     err.Error(),
			})
			continue
		}

		resp.Stopped = append(resp.Stopped, sessionID)
	}

	adminapi.WriteJSON(w, http.StatusOK, resp)
}

// extractSessionID extracts the session ID from the URL path.
func extractSessionID(r *http.Request) string {
	path := r.URL.Path
	// Find the last '/'
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			// Return segment after '/' if there's content
			if i < len(path)-1 {
				return path[i+1:]
			}
			// Empty segment after '/', return empty
			return ""
		}
	}
	// No '/' found, return the whole path
	return path
}
