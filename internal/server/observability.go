package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hrygo/hotplex/telemetry"
)

// HealthHandler provides HTTP endpoints for health checking and metrics.
type HealthHandler struct {
	healthChecker *telemetry.HealthChecker
	metrics       *telemetry.Metrics
	startTime     time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		healthChecker: telemetry.GetHealthChecker(),
		metrics:       telemetry.GetMetrics(),
		startTime:     time.Now(),
	}
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    telemetry.HealthStatus `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Checks    map[string]bool        `json:"checks,omitempty"`
}

// ServeHTTP handles /health requests.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, checks := h.healthChecker.Check()

	resp := HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(h.startTime).String(),
		Checks:    checks,
	}

	w.Header().Set("Content-Type", "application/json")
	if status == telemetry.StatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else if status == telemetry.StatusDegraded {
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(resp)
}

// ReadyHandler handles /health/ready requests (Kubernetes readiness probe).
type ReadyHandler struct {
	engineReady func() bool
}

// NewReadyHandler creates a new ReadyHandler.
func NewReadyHandler(engineReady func() bool) *ReadyHandler {
	return &ReadyHandler{engineReady: engineReady}
}

func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.engineReady == nil || !h.engineReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
			"reason": "engine_not_initialized",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

// LiveHandler handles /health/live requests (Kubernetes liveness probe).
type LiveHandler struct {
	startTime time.Time
}

// NewLiveHandler creates a new LiveHandler.
func NewLiveHandler() *LiveHandler {
	return &LiveHandler{startTime: time.Now()}
}

func (h *LiveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "alive",
		"uptime":    time.Since(h.startTime).String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// MetricsHandler handles /metrics requests (Prometheus format).
type MetricsHandler struct {
	metrics *telemetry.Metrics
}

// NewMetricsHandler creates a new MetricsHandler.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{metrics: telemetry.GetMetrics()}
}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot := h.metrics.Snapshot()

	// Prometheus text format
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Session metrics
	fmt.Fprintf(w, "# HELP hotplex_sessions_active Number of currently active sessions\n")
	fmt.Fprintf(w, "# TYPE hotplex_sessions_active gauge\n")
	fmt.Fprintf(w, "hotplex_sessions_active %d\n", snapshot.SessionsActive)

	fmt.Fprintf(w, "# HELP hotplex_sessions_total Total number of sessions created\n")
	fmt.Fprintf(w, "# TYPE hotplex_sessions_total counter\n")
	fmt.Fprintf(w, "hotplex_sessions_total %d\n", snapshot.SessionsTotal)

	fmt.Fprintf(w, "# HELP hotplex_sessions_errors Total number of session errors\n")
	fmt.Fprintf(w, "# TYPE hotplex_sessions_errors counter\n")
	fmt.Fprintf(w, "hotplex_sessions_errors %d\n", snapshot.SessionsErrors)

	fmt.Fprintf(w, "# HELP hotplex_tools_invoked Total number of tool invocations\n")
	fmt.Fprintf(w, "# TYPE hotplex_tools_invoked counter\n")
	fmt.Fprintf(w, "hotplex_tools_invoked %d\n", snapshot.ToolsInvoked)

	fmt.Fprintf(w, "# HELP hotplex_dangers_blocked Total number of dangerous operations blocked\n")
	fmt.Fprintf(w, "# TYPE hotplex_dangers_blocked counter\n")
	fmt.Fprintf(w, "hotplex_dangers_blocked %d\n", snapshot.DangersBlocked)
}
