package admin

import (
	"encoding/json"
	"net/http"

	"github.com/hrygo/hotplex/internal/adminapi"
)

// listRelayBindings handles GET /admin/relay/bindings.
func (h *Handler) listRelayBindings(w http.ResponseWriter, r *http.Request) {
	bindings := h.relayBindings
	if bindings == nil {
		bindings = []*RelayBindingResponse{}
	}
	adminapi.WriteJSON(w, http.StatusOK, RelayBindingsResponse{Bindings: bindings, Total: len(bindings)})
}

// createRelayBinding handles POST /admin/relay/bindings.
func (h *Handler) createRelayBinding(w http.ResponseWriter, r *http.Request) {
	var req RelayBindingCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON body")
		return
	}

	binding := &RelayBindingResponse{
		Platform: req.Platform,
		ChatID:   req.ChatID,
		Bots:     req.Bots,
	}

	h.relayBindings = append(h.relayBindings, binding)
	adminapi.WriteJSON(w, http.StatusCreated, binding)
}
