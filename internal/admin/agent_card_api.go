package admin

import (
	"net/http"
	"time"

	"github.com/hrygo/hotplex/internal/adminapi"
	"github.com/hrygo/hotplex/internal/agent"
)

// getAgentCard handles GET /admin/agent-card.
func (h *Handler) getAgentCard(w http.ResponseWriter, r *http.Request) {
	card := h.agentRegistry.GetAgentCard()
	if card == nil {
		// Return a minimal default card if none is registered
		card = &agent.AgentCard{
			Name:      "hotplex",
			CreatedAt: time.Now(),
		}
	}
	adminapi.WriteJSON(w, http.StatusOK, card)
}
