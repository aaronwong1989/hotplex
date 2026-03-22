package admin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/hrygo/hotplex/engine"
	intengine "github.com/hrygo/hotplex/internal/engine"
)

// AdminServer is the main admin API server that combines all handlers.
type AdminServer struct {
	engine         *engine.Engine
	logger         *slog.Logger
	adminKey       string
	startTime      time.Time
	sessionHandler *SessionHandler
	healthHandler  *HealthHandler
	configHandler  *ConfigHandler
	auditHandler   *AuditHandler
	eventBuffer    *EventBuffer
	mux            *http.ServeMux
}

// AdminServerOptions contains options for creating an AdminServer.
type AdminServerOptions struct {
	Engine          *engine.Engine
	AdminKey        string
	Logger          *slog.Logger
	EventBufferSize int
}

// NewAdminServer creates a new admin server with all handlers registered.
func NewAdminServer(opts AdminServerOptions) *AdminServer {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	startTime := time.Now()
	eventBuffer := NewEventBuffer(opts.EventBufferSize)
	if eventBuffer == nil || opts.EventBufferSize <= 0 {
		eventBuffer = NewEventBuffer(10000)
	}

	s := &AdminServer{
		engine:      opts.Engine,
		logger:      logger,
		adminKey:    opts.AdminKey,
		startTime:   startTime,
		eventBuffer: eventBuffer,
	}

	// Initialize handlers with proper pool adapter
	s.sessionHandler = s.newSessionHandler()
	s.healthHandler = NewHealthHandler(s.engine, s.startTime, s.logger)
	s.configHandler = NewConfigHandler(s.engine)
	s.auditHandler = NewAuditHandler(s.eventBuffer, s.engine)

	// Create router
	s.mux = http.NewServeMux()
	s.registerRoutes()

	return s
}

// newSessionHandler creates a session handler with pool adapter.
func (s *AdminServer) newSessionHandler() *SessionHandler {
	var pool SessionPoolInterface

	if s.engine != nil && s.engine.GetSessionManager() != nil {
		pool = &enginePoolAdapter{manager: s.engine.GetSessionManager()}
	} else {
		pool = &enginePoolAdapter{}
	}

	return NewSessionHandler(pool)
}

// enginePoolAdapter adapts the engine's SessionManager to SessionPoolInterface.
type enginePoolAdapter struct {
	manager intengine.SessionManager
}

func (a *enginePoolAdapter) ListActiveSessions() []*intengine.Session {
	if a.manager == nil {
		return []*intengine.Session{}
	}
	sessions := a.manager.ListActiveSessions()
	result := make([]*intengine.Session, len(sessions))
	for i, sess := range sessions {
		result[i] = sess
	}
	return result
}

func (a *enginePoolAdapter) GetSession(sessionID string) (*intengine.Session, bool) {
	if a.manager == nil {
		return nil, false
	}
	return a.manager.GetSession(sessionID)
}

func (a *enginePoolAdapter) TerminateSession(sessionID string) error {
	if a.manager == nil {
		return nil
	}
	return a.manager.TerminateSession(sessionID)
}

// registerRoutes registers all admin API routes under /api/v1/admin/.
// All routes share a single prefix strip and auth middleware to avoid
// routing conflicts between exact-match and prefix-match patterns.
func (s *AdminServer) registerRoutes() {
	authMiddleware := AdminAuthMiddleware(s.adminKey, s.logger)

	// Single combined router — avoids mux prefix/exact-match routing conflicts.
	combinedRoutes := http.NewServeMux()

	// Session routes
	combinedRoutes.HandleFunc("GET /sessions", s.sessionHandler.ListSessions)
	combinedRoutes.HandleFunc("GET /sessions/{id}", s.sessionHandler.GetSession)
	combinedRoutes.HandleFunc("POST /sessions/{id}/stop", s.sessionHandler.StopSession)
	combinedRoutes.HandleFunc("POST /sessions/batch-stop", s.sessionHandler.BatchStopSessions)
	combinedRoutes.HandleFunc("GET /sessions/{id}/transcript", s.auditHandler.getTranscript)

	// Audit routes
	combinedRoutes.HandleFunc("GET /events", s.auditHandler.getEvents)

	// Health & drain routes
	combinedRoutes.HandleFunc("GET /health", s.healthHandler.getHealth)
	combinedRoutes.HandleFunc("GET /metrics", s.healthHandler.getMetrics)
	combinedRoutes.HandleFunc("POST /drain", s.healthHandler.enterDrain)
	combinedRoutes.HandleFunc("DELETE /drain", s.healthHandler.exitDrain)
	combinedRoutes.HandleFunc("GET /drain", s.healthHandler.getDrainStatus)

	// Config routes
	combinedRoutes.HandleFunc("GET /config", s.configHandler.getConfig)
	combinedRoutes.HandleFunc("GET /config/allowed_tools", s.configHandler.getAllowedTools)
	combinedRoutes.HandleFunc("GET /config/disallowed_tools", s.configHandler.getDisallowedTools)

	// Register combined routes at /api/v1/admin/ with shared auth.
	s.mux.Handle("/api/v1/admin/", authMiddleware(http.StripPrefix("/api/v1/admin", combinedRoutes)))
}

// ServeHTTP implements http.Handler.
func (s *AdminServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// GetHealthHandler returns the health handler for drain integration.
func (s *AdminServer) GetHealthHandler() *HealthHandler {
	return s.healthHandler
}

// GetEventBuffer returns the event buffer for event recording.
func (s *AdminServer) GetEventBuffer() *EventBuffer {
	return s.eventBuffer
}
