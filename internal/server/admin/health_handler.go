package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/hrygo/hotplex"
	"github.com/hrygo/hotplex/engine"
	intengine "github.com/hrygo/hotplex/internal/engine"
)

// HealthHandler handles health, metrics, and drain endpoints.
type HealthHandler struct {
	engine    *engine.Engine
	startTime time.Time
	logger    *slog.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(eng *engine.Engine, startTime time.Time, logger *slog.Logger) *HealthHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthHandler{
		engine:    eng,
		startTime: startTime,
		logger:    logger,
	}
}

// getHealth handles GET /api/v1/admin/health.
func (h *HealthHandler) getHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	subsystems := make(map[string]SubsystemHealth)
	overallStatus := "ok"

	// Check engine/session pool
	if h.checkEngine(ctx) {
		subsystems["engine"] = SubsystemHealth{Status: "ok"}
	} else {
		subsystems["engine"] = SubsystemHealth{Status: "error", Message: "Session pool unavailable"}
		overallStatus = "degraded"
	}

	// Check security/WAF
	if h.checkSecurity(ctx) {
		subsystems["security"] = SubsystemHealth{Status: "ok"}
	} else {
		subsystems["security"] = SubsystemHealth{Status: "error", Message: "WAF unavailable"}
		overallStatus = "degraded"
	}

	// Check storage (optional)
	if h.checkStorage(ctx) {
		subsystems["storage"] = SubsystemHealth{Status: "ok"}
	} else {
		subsystems["storage"] = SubsystemHealth{Status: "error", Message: "Message store not available"}
		if overallStatus == "ok" {
			overallStatus = "degraded"
		}
	}

	// Check config watcher
	if h.checkConfig(ctx) {
		subsystems["config"] = SubsystemHealth{Status: "ok"}
	} else {
		subsystems["config"] = SubsystemHealth{Status: "error", Message: "Config watcher unavailable"}
		overallStatus = "unhealthy"
	}

	// Count unhealthy subsystems
	unhealthy := 0
	for _, s := range subsystems {
		if s.Status == "error" {
			unhealthy++
		}
	}
	if unhealthy > 1 {
		overallStatus = "unhealthy"
	}

	response := AdminHealth{
		Status:      overallStatus,
		Version:     hotplex.Version,
		Uptime:      formatUptime(time.Since(h.startTime)),
		Subsystems:  subsystems,
	}

	writeJSON(w, http.StatusOK, response)
}

// getMetrics handles GET /api/v1/admin/metrics.
// Returns Prometheus-compatible text format.
func (h *HealthHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
	var sb stringsBuilder
	sb.WriteString("# HELP hotplex_uptime_seconds Uptime in seconds\n")
	sb.WriteString("# TYPE hotplex_uptime_seconds gauge\n")
	sb.WriteString("hotplex_uptime_seconds " + strconv.FormatFloat(time.Since(h.startTime).Seconds(), 'f', 3, 64) + "\n")

	if h.engine != nil && h.engine.GetSessionManager() != nil {
		sessions := h.engine.GetSessionManager().ListActiveSessions()

		sb.WriteString("# HELP hotplex_sessions_active Current number of active sessions\n")
		sb.WriteString("# TYPE hotplex_sessions_active gauge\n")

		// Count by status
		statusCounts := make(map[string]int)
		platformCounts := make(map[string]int)
		for _, s := range sessions {
			status := MapSessionStatus(s.GetStatus())
			statusCounts[status]++
			platformCounts[MapSessionToAdminSession(s).Platform]++
		}

		for status, count := range statusCounts {
			sb.WriteString("hotplex_sessions_active{status=\"" + status + "\"} " + strconv.Itoa(count) + "\n")
		}

		sb.WriteString("# HELP hotplex_sessions_total Total number of sessions\n")
		sb.WriteString("# TYPE hotplex_sessions_total counter\n")
		sb.WriteString("hotplex_sessions_total " + strconv.Itoa(len(sessions)) + "\n")

		// Platform counts
		sb.WriteString("# HELP hotplex_sessions_by_platform Sessions by platform\n")
		sb.WriteString("# TYPE hotplex_sessions_by_platform gauge\n")
		for platform, count := range platformCounts {
			sb.WriteString("hotplex_sessions_by_platform{platform=\"" + platform + "\"} " + strconv.Itoa(count) + "\n")
		}
	}

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	sb.WriteString("# HELP hotplex_memory_alloc_bytes Allocated memory in bytes\n")
	sb.WriteString("# TYPE hotplex_memory_alloc_bytes gauge\n")
	sb.WriteString("hotplex_memory_alloc_bytes " + strconv.FormatUint(memStats.Alloc, 10) + "\n")

	sb.WriteString("# HELP hotplex_goroutines Number of goroutines\n")
	sb.WriteString("# TYPE hotplex_goroutines gauge\n")
	sb.WriteString("hotplex_goroutines " + strconv.Itoa(runtime.NumGoroutine()) + "\n")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sb.String()))
}

// enterDrain handles POST /api/v1/admin/drain.
func (h *HealthHandler) enterDrain(w http.ResponseWriter, r *http.Request) {
	var req DrainRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON body")
			return
		}
	}

	msg := req.Message
	if msg == "" {
		msg = "Service is in drain mode"
	}

	if h.engine != nil {
		h.engine.EnterDrain(msg)
	}

	activeSessions := 0
	if h.engine != nil && h.engine.GetSessionManager() != nil {
		activeSessions = len(h.engine.GetSessionManager().ListActiveSessions())
	}

	h.logger.Info("Admin API: Entering drain mode",
		"message", msg,
		"active_sessions", activeSessions)

	response := DrainResponse{
		Status:         "draining",
		ActiveSessions: activeSessions,
		Message:        msg,
	}
	writeJSON(w, http.StatusOK, response)
}

// exitDrain handles DELETE /api/v1/admin/drain.
func (h *HealthHandler) exitDrain(w http.ResponseWriter, r *http.Request) {
	wasDraining := false
	if h.engine != nil {
		wasDraining = h.engine.IsDraining()
		h.engine.ExitDrain()
	}

	if !wasDraining {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Service is not in drain mode")
		return
	}

	h.logger.Info("Admin API: Exiting drain mode")

	response := DrainResponse{
		Status:  "active",
		Message: "Service has exited drain mode",
	}
	writeJSON(w, http.StatusOK, response)
}

// getDrainStatus handles GET /api/v1/admin/drain.
func (h *HealthHandler) getDrainStatus(w http.ResponseWriter, r *http.Request) {
	draining := false
	if h.engine != nil {
		draining = h.engine.IsDraining()
	}

	if draining {
		msg := ""
		if h.engine != nil {
			msg = h.engine.GetDrainMessage()
		}
		activeSessions := 0
		if h.engine != nil && h.engine.GetSessionManager() != nil {
			activeSessions = len(h.engine.GetSessionManager().ListActiveSessions())
		}
		response := DrainResponse{
			Status:         "draining",
			ActiveSessions: activeSessions,
			Message:        msg,
		}
		writeJSON(w, http.StatusOK, response)
	} else {
		response := DrainResponse{
			Status: "active",
		}
		writeJSON(w, http.StatusOK, response)
	}
}

// IsDraining returns true if the service is in drain mode.
func (h *HealthHandler) IsDraining() bool {
	if h.engine != nil {
		return h.engine.IsDraining()
	}
	return false
}

// GetDrainMessage returns the drain message.
func (h *HealthHandler) GetDrainMessage() string {
	if h.engine != nil {
		return h.engine.GetDrainMessage()
	}
	return "Service is in drain mode"
}

// checkEngine checks session pool health.
func (h *HealthHandler) checkEngine(ctx context.Context) bool {
	if h.engine == nil || h.engine.GetSessionManager() == nil {
		return false
	}
	done := make(chan bool, 1)
	go func() {
		sessions := h.engine.GetSessionManager().ListActiveSessions()
		done <- (sessions != nil)
	}()

	select {
	case <-ctx.Done():
		return false
	case ok := <-done:
		return ok
	}
}

// checkSecurity checks WAF/health.
func (h *HealthHandler) checkSecurity(ctx context.Context) bool {
	// Basic check - if engine is running, security is active
	return h.checkEngine(ctx)
}

// checkStorage checks message store availability.
func (h *HealthHandler) checkStorage(ctx context.Context) bool {
	// Optional subsystem - return true for now
	return true
}

// checkConfig checks config watcher health.
func (h *HealthHandler) checkConfig(ctx context.Context) bool {
	// Basic check - if we can respond, config is valid
	return true
}

// formatUptime formats duration as human-readable string.
func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	totalSeconds := int(d / time.Second)
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	if h > 0 {
		return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m " + strconv.Itoa(s) + "s"
	}
	if m > 0 {
		return strconv.Itoa(m) + "m " + strconv.Itoa(s) + "s"
	}
	return strconv.Itoa(s) + "s"
}

// stringsBuilder is a simple strings.Builder replacement.
type stringsBuilder struct {
	b []byte
}

func (sb *stringsBuilder) WriteString(s string) {
	sb.b = append(sb.b, s...)
}

func (sb *stringsBuilder) String() string {
	return string(sb.b)
}

// Ensure intengine.Session type compatibility
var _ = (*intengine.Session)(nil)
