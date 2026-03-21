package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/hrygo/hotplex/engine"
	intengine "github.com/hrygo/hotplex/internal/engine"
	"github.com/hrygo/hotplex/internal/telemetry"
	"gopkg.in/yaml.v3"
)

// Compile-time interface compliance: *slog.Logger satisfies Logger.
var _ Logger = (*slog.Logger)(nil)

// Handler handles admin API requests.
type Handler struct {
	engine    *engine.Engine
	startTime time.Time
	logger    Logger
}

// Logger interface for logging.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NewHandler creates a new admin handler.
func NewHandler(eng *engine.Engine, startTime time.Time, logger Logger) *Handler {
	return &Handler{
		engine:    eng,
		startTime: startTime,
		logger:    logger,
	}
}

// listSessions handles GET /admin/v1/sessions.
func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Engine not initialized")
		return
	}

	manager := h.engine.GetSessionManager()
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Session manager not available")
		return
	}

	sessions := manager.ListActiveSessions()
	infos := make([]*SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		infos = append(infos, &SessionInfo{
			ID:         sess.ID,
			Status:     string(sess.Status),
			CreatedAt:  sess.CreatedAt,
			LastActive: sess.GetLastActive(),
		})
	}

	resp := SessionListResponse{
		Sessions: infos,
		Total:    len(infos),
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// getSession handles GET /admin/v1/sessions/:id.
func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Engine not initialized")
		return
	}

	manager := h.engine.GetSessionManager()
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Session manager not available")
		return
	}

	sess, ok := manager.GetSession(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "Session not found: "+sessionID)
		return
	}

	resp := SessionDetailResponse{
		ID:         sess.ID,
		Status:     string(sess.Status),
		CreatedAt:  sess.CreatedAt,
		LastActive: sess.GetLastActive(),
		Config: SessionConfig{
			// Provider is not exposed on individual sessions; report engine-level provider.
			Provider: "claude-code",
			WorkDir:  sess.Config.WorkDir,
		},
	}

	if stats := h.engine.GetSessionStats(sessionID); stats != nil {
		resp.Stats = SessionStats{
			InputTokens:  int64(stats.InputTokens),
			OutputTokens: int64(stats.OutputTokens),
			DurationSecs: stats.TotalDurationMs / 1000,
		}
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// deleteSession handles DELETE /admin/v1/sessions/:id.
func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Engine not initialized")
		return
	}

	manager := h.engine.GetSessionManager()
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Session manager not available")
		return
	}

	if _, ok := manager.GetSession(sessionID); !ok {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "Session not found: "+sessionID)
		return
	}

	if err := h.engine.StopSession(sessionID, "admin-terminated"); err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeServerError, "Failed to terminate session: "+err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, SessionDeleteResponse{
		Success: true,
		Message: "Session " + sessionID + " terminated",
	})
}

// getSessionLogs handles GET /admin/v1/sessions/:id/logs.
func (h *Handler) getSessionLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeServerError, "Failed to determine home directory")
		return
	}

	logDir := filepath.Join(homeDir, ".hotplex", "logs")
	logPath := filepath.Join(logDir, sessionID+".log")

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "Log file not found for session: "+sessionID)
			return
		}
		writeError(w, http.StatusInternalServerError, ErrCodeServerError, "Failed to read log file")
		return
	}

	h.writeJSON(w, http.StatusOK, SessionLogsResponse{
		SessionID:    sessionID,
		LogPath:      logPath,
		SizeBytes:    info.Size(),
		LastModified: info.ModTime(),
	})
}

// getStats handles GET /admin/v1/stats.
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	var total, active, stopped int

	if h.engine != nil {
		manager := h.engine.GetSessionManager()
		if manager != nil {
			sessions := manager.ListActiveSessions()
			total = len(sessions)
			for _, sess := range sessions {
				if sess.Status == intengine.SessionStatusStarting ||
					sess.Status == intengine.SessionStatusReady ||
					sess.Status == intengine.SessionStatusBusy {
					active++
				}
			}
			stopped = total - active
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	resp := StatsResponse{
		TotalSessions:   total,
		ActiveSessions:  active,
		StoppedSessions: stopped,
		Uptime:          time.Since(h.startTime).String(),
		MemoryUsageMB:   float64(memStats.Alloc) / 1024 / 1024,
		CpuUsagePercent: 0,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// validateConfig handles POST /admin/v1/config/validate.
func (h *Handler) validateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON body")
		return
	}

	errors := validateConfigFile(req.ConfigPath)
	resp := ConfigValidateResponse{
		Valid:  len(errors) == 0,
		Errors: errors,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// getHealthDetailed handles GET /admin/v1/health/detailed.
func (h *Handler) getHealthDetailed(w http.ResponseWriter, r *http.Request) {
	cliVersion := getCliVersion()
	checks := HealthChecks{
		Config:               true,
		CliAvailable:         cliVersion != "unknown",
		WebsocketConnections: countWebsocketConnections(),
	}
	details := HealthDetails{
		DatabaseLatencyMs: 0,
		CliVersion:        cliVersion,
	}

	dbPath := os.Getenv("HOTPLEX_MESSAGE_STORE_SQLITE_PATH")
	if dbPath != "" {
		latency, ok := checkDatabaseHealth(dbPath)
		if ok {
			checks.Database = true
			details.DatabaseLatencyMs = latency
		}
	}

	status := "healthy"
	if !checks.Config || !checks.CliAvailable {
		status = "degraded"
	}

	h.writeJSON(w, http.StatusOK, HealthDetailedResponse{
		Status:  status,
		Checks:  checks,
		Details: details,
	})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

func checkCliAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude-code", "--version")
	return cmd.Run() == nil
}

func getCliVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude-code", "--version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func countWebsocketConnections() int {
	metrics := telemetry.GetMetrics()
	snapshot := metrics.Snapshot()
	return int(snapshot.SessionsActive)
}

func checkDatabaseHealth(dbPath string) (int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()

	cmd := exec.CommandContext(ctx, "sqlite3", dbPath, "SELECT 1;")
	if err := cmd.Run(); err != nil {
		return 0, false
	}

	return int(time.Since(start).Milliseconds()), true
}

func validateConfigFile(path string) []string {
	if path == "" {
		return []string{"config_path is required"}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"config file not found: " + path}
		}
		return []string{"failed to read config file: " + err.Error()}
	}

	if len(data) == 0 {
		return []string{"config file is empty"}
	}

	// Parse YAML to check structure
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return []string{"invalid YAML: " + err.Error()}
	}

	// Check required top-level fields
	requiredFields := []string{"server", "engine"}
	var errors []string
	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			errors = append(errors, "missing required field: "+field)
		}
	}

	return errors
}
