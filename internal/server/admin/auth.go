package admin

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/hrygo/hotplex/internal/adminapi"
)

// AdminAuthMiddleware creates an HTTP middleware that validates the X-API-Key header.
// It uses constant-time comparison to prevent timing attacks.
// If adminKey is empty, authentication is bypassed (no auth required).
func AdminAuthMiddleware(adminKey string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no admin key is configured
			if adminKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("X-API-Key")

			// Empty key check
			if key == "" {
				logger.Warn("admin_auth: missing key",
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
				)
				adminapi.WriteError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "Missing X-API-Key header")
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) != 1 {
				logger.Warn("admin_auth: invalid key",
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
					"key_prefix", keyPrefix(key),
				)
				adminapi.WriteError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "Invalid X-API-Key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// keyPrefix returns the first 4 characters of the key for logging.
// This allows identifying which key was used without logging the full key.
func keyPrefix(key string) string {
	if len(key) < 4 {
		return "****"
	}
	return key[:4] + "****"
}
