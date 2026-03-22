package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hrygo/hotplex"
	"github.com/hrygo/hotplex/engine"
	"github.com/hrygo/hotplex/internal/security"
)

// RelayHandler handles incoming relay messages from remote agents.
type RelayHandler struct {
	engine      hotplex.HotPlexClient
	logger      *slog.Logger
	wafDetector *security.Detector
}

// NewRelayHandler creates a new RelayHandler.
func NewRelayHandler(engine hotplex.HotPlexClient, logger *slog.Logger, wafDetector *security.Detector) *RelayHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if wafDetector == nil {
		wafDetector = security.NewDetector(logger)
	}
	return &RelayHandler{
		engine:      engine,
		logger:      logger,
		wafDetector: wafDetector,
	}
}

// ServeHTTP handles POST /relay requests.
// Note: Method validation is handled by mux router (Methods(http.MethodPost)).
func (h *RelayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req engine.RelayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("relay: invalid JSON payload", "error", err)
		WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), h.logger)
		return
	}

	// WAF security check on relay content
	if h.wafDetector.CheckInput(req.Message) != nil {
		h.logger.Warn("relay: message blocked by WAF",
			"from", req.From,
			"to", req.To)
		WriteError(w, http.StatusForbidden, "message blocked by security policy", h.logger)
		return
	}

	// Delegate to engine via RelayExecutor interface
	executor, ok := h.engine.(engine.RelayExecutor)
	if !ok {
		h.logger.Error("relay: engine does not implement RelayExecutor")
		WriteError(w, http.StatusNotImplemented, "relay not supported", h.logger)
		return
	}

	resp, err := executor.HandleRelay(r.Context(), &req)
	if err != nil {
		h.logger.Error("relay: HandleRelay failed", "error", err)
		WriteError(w, http.StatusInternalServerError, err.Error(), h.logger)
		return
	}

	WriteJSON(w, http.StatusOK, resp, h.logger)
}

// RegisterRoutes registers the relay handler on the given router.
func (h *RelayHandler) RegisterRoutes(r *mux.Router) {
	r.Handle("/relay", h).Methods(http.MethodPost)
}
