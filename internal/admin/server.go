package admin

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hrygo/hotplex/engine"
)

// Server is the admin HTTP server.
type Server struct {
	port      string
	token     string
	engine    *engine.Engine
	startTime time.Time
	logger    *slog.Logger
	server    *http.Server
	errCh     chan error // receives errors from the server goroutine
}

// NewServer creates a new admin server.
func NewServer(eng *engine.Engine, port, token string, startTime time.Time, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	handler := NewHandler(eng, startTime, logger)
	mw := NewMiddleware(token)

	router := mux.NewRouter()
	router.Use(mw.AuthMiddleware)

	api := router.PathPrefix("/admin/v1").Subrouter()

	api.HandleFunc("/sessions", handler.listSessions).Methods(http.MethodGet)
	api.HandleFunc("/sessions/{id}", handler.getSession).Methods(http.MethodGet)
	api.HandleFunc("/sessions/{id}", handler.deleteSession).Methods(http.MethodDelete)
	api.HandleFunc("/sessions/{id}/logs", handler.getSessionLogs).Methods(http.MethodGet)
	api.HandleFunc("/stats", handler.getStats).Methods(http.MethodGet)
	api.HandleFunc("/config/validate", handler.validateConfig).Methods(http.MethodPost)
	api.HandleFunc("/health/detailed", handler.getHealthDetailed).Methods(http.MethodGet)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		port:      port,
		token:     token,
		engine:    eng,
		startTime: startTime,
		logger:    logger,
		server:    server,
		errCh:     make(chan error, 1),
	}
}

// Start starts the admin server in a goroutine.
// The server reports startup errors (e.g., port already in use) via ErrCh.
// Callers should monitor ErrCh after Start() to detect startup failures.
func (s *Server) Start(errCh chan<- error) {
	go func() {
		s.logger.Info("Admin server starting", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Admin server failed", "error", err)
			s.errCh <- err
		}
	}()
}

// ErrChan returns the channel that receives server errors.
func (s *Server) ErrChan() <-chan error { return s.errCh }

// Stop gracefully stops the admin server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Admin server shutting down")
	return s.server.Shutdown(ctx)
}
