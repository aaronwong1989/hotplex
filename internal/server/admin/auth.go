package admin

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
)

// AdminAuthMiddleware creates an HTTP middleware that validates the X-Admin-Key header.
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

			key := r.Header.Get("X-Admin-Key")

			// Empty key check
			if key == "" {
				logger.Warn("admin_auth: missing key",
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
				)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing X-Admin-Key header")
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) != 1 {
				logger.Warn("admin_auth: invalid key",
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
					"key_prefix", keyPrefix(key),
				)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid X-Admin-Key")
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

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := NewAdminError(code, message)
	// Simple JSON encoding without importing encoding/json
	w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`))
	_ = resp // Just for documentation
}
