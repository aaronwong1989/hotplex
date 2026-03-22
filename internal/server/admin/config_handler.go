package admin

import (
	"net/http"

	"github.com/hrygo/hotplex/engine"
	"github.com/hrygo/hotplex/internal/adminapi"
)

// ConfigHandler handles configuration endpoints.
type ConfigHandler struct {
	engine *engine.Engine
}

// NewConfigHandler creates a new config handler.
func NewConfigHandler(eng *engine.Engine) *ConfigHandler {
	return &ConfigHandler{engine: eng}
}

// getConfig handles GET /api/v1/admin/config.
func (h *ConfigHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeEngineNotInitialized, "Engine not initialized")
		return
	}

	opts := h.engine.GetOptions()

	// Create a safe config view (no secrets)
	config := map[string]interface{}{
		"namespace":        opts.Namespace,
		"timeout":          opts.Timeout.String(),
		"idle_timeout":     opts.IdleTimeout.String(),
		"permission_mode":  opts.PermissionMode,
		"skip_permissions": opts.DangerouslySkipPermissions,
		"allowed_tools":    opts.AllowedTools,
		"disallowed_tools": opts.DisallowedTools,
	}

	response := ConfigResponse{Config: config}
	adminapi.WriteJSON(w, http.StatusOK, response)
}

// getAllowedTools handles GET /api/v1/admin/config/allowed_tools.
func (h *ConfigHandler) getAllowedTools(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeEngineNotInitialized, "Engine not initialized")
		return
	}

	opts := h.engine.GetOptions()
	response := ToolsResponse{
		Tools:  opts.AllowedTools,
		Source: "engine_options",
	}
	adminapi.WriteJSON(w, http.StatusOK, response)
}

// getDisallowedTools handles GET /api/v1/admin/config/disallowed_tools.
func (h *ConfigHandler) getDisallowedTools(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeEngineNotInitialized, "Engine not initialized")
		return
	}

	opts := h.engine.GetOptions()
	response := ToolsResponse{
		Tools:  opts.DisallowedTools,
		Source: "engine_options",
	}
	adminapi.WriteJSON(w, http.StatusOK, response)
}
