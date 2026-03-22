package admin

import (
	"log/slog"
	"net/http"

	"github.com/hrygo/hotplex/internal/adminapi"
)

// AdminAuthMiddleware creates an HTTP middleware that validates the X-API-Key header.
// It uses constant-time comparison to prevent timing attacks.
// If adminKey is empty, authentication is bypassed (no auth required).
//
// Deprecated: Use adminapi.AuthMiddleware directly with AuthModeAPIKey.
// This function is kept for backward compatibility.
func AdminAuthMiddleware(adminKey string, logger *slog.Logger) func(http.Handler) http.Handler {
	return adminapi.AuthMiddleware(adminapi.AuthMiddlewareOptions{
		AdminKey: adminKey,
		Mode:     adminapi.AuthModeAPIKey,
		Logger:   logger,
	})
}
