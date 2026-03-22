package admin

import (
	"crypto/subtle"
	"net/http"

	"github.com/hrygo/hotplex/internal/adminapi"
)

// Middleware provides HTTP middleware functions for the admin API.
type Middleware struct {
	adminToken string
}

// NewMiddleware creates a new admin middleware.
func NewMiddleware(adminToken string) *Middleware {
	return &Middleware{adminToken: adminToken}
}

// AuthMiddleware returns an HTTP handler that validates the admin token.
// If adminToken is empty, authentication is disabled (dev mode).
func (m *Middleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.adminToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			adminapi.WriteError(w, http.StatusUnauthorized, ErrCodeAuthFailed, "Authorization header required")
			return
		}

		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix)+len(m.adminToken) {
			adminapi.WriteError(w, http.StatusUnauthorized, ErrCodeAuthFailed, "Invalid authorization format")
			return
		}

		token := authHeader[len(bearerPrefix):]
		if subtle.ConstantTimeCompare([]byte(token), []byte(m.adminToken)) != 1 {
			adminapi.WriteError(w, http.StatusForbidden, ErrCodeForbidden, "Invalid admin token")
			return
		}

		next.ServeHTTP(w, r)
	})
}
