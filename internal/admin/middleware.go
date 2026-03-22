package admin

import (
	"log/slog"
	"net/http"

	"github.com/hrygo/hotplex/internal/adminapi"
)

// Middleware provides HTTP middleware functions for the admin API.
type Middleware struct {
	adminToken string
	logger     *slog.Logger
}

// NewMiddleware creates a new admin middleware.
// If logger is nil, slog.Default() is used.
func NewMiddleware(adminToken string, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &Middleware{
		adminToken: adminToken,
		logger:     logger,
	}
}

// AuthMiddleware returns an HTTP handler that validates the admin token.
// If adminToken is empty, authentication is disabled (dev mode).
// Uses Bearer token authentication (OAuth2 standard).
func (m *Middleware) AuthMiddleware(next http.Handler) http.Handler {
	auth := adminapi.AuthMiddleware(adminapi.AuthMiddlewareOptions{
		AdminKey: m.adminToken,
		Mode:     adminapi.AuthModeBearer,
		Logger:   m.logger,
	})
	return auth(next)
}
