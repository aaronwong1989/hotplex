// Package adminapi provides shared authentication middleware for admin APIs.
// It supports two authentication modes following industry best practices:
//
// 1. Bearer Token (OAuth2 standard) - Authorization: Bearer <token>
// 2. API Key (simplified) - X-API-Key: <key>
//
// Both modes use constant-time comparison to prevent timing attacks.
package adminapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
)

// AuthMode defines the authentication mode for admin APIs.
type AuthMode int

const (
	// AuthModeBearer uses OAuth2-style Authorization: Bearer <token>
	// Recommended for external integrations and webhook APIs.
	AuthModeBearer AuthMode = iota

	// AuthModeAPIKey uses X-API-Key header
	// Recommended for internal tooling and simple integrations.
	AuthModeAPIKey
)

// AuthMiddlewareOptions configures the authentication middleware.
type AuthMiddlewareOptions struct {
	// AdminKey is the secret key/token for authentication.
	// If empty, authentication is bypassed (dev/unsecured mode).
	AdminKey string

	// Mode specifies the authentication mode (Bearer or API Key).
	Mode AuthMode

	// Logger for structured logging. Defaults to slog.Default().
	Logger *slog.Logger
}

// AuthMiddleware creates an HTTP middleware that validates authentication.
// It uses constant-time comparison to prevent timing attacks.
//
// If adminKey is empty, authentication is bypassed (useful for dev/testing).
//
// Example (Bearer mode):
//
//	auth := AuthMiddleware(AuthMiddlewareOptions{
//	    AdminKey: "secret-token",
//	    Mode:     AuthModeBearer,
//	    Logger:   logger,
//	})
//	handler := auth(myHandler)
//
// Example (API Key mode):
//
//	auth := AuthMiddleware(AuthMiddlewareOptions{
//	    AdminKey: "api-key-123",
//	    Mode:     AuthModeAPIKey,
//	    Logger:   logger,
//	})
//	handler := auth(myHandler)
func AuthMiddleware(opts AuthMiddlewareOptions) func(http.Handler) http.Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no admin key is configured (dev/unsecured mode)
			if opts.AdminKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			var key string
			var errCode ErrorCode
			var errMsg string

			// Extract key based on authentication mode
			switch opts.Mode {
			case AuthModeBearer:
				key, errCode, errMsg = extractBearerToken(r)
			case AuthModeAPIKey:
				key, errCode, errMsg = extractAPIKey(r)
			default:
				WriteError(w, http.StatusInternalServerError, ErrCodeServerError, "Invalid authentication mode")
				return
			}

			// Check if key was extracted
			if key == "" {
				logger.Warn("admin_auth: authentication failed",
					"mode", modeString(opts.Mode),
					"reason", errMsg,
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
				)
				WriteError(w, http.StatusUnauthorized, errCode, errMsg)
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(key), []byte(opts.AdminKey)) != 1 {
				logger.Warn("admin_auth: invalid credentials",
					"mode", modeString(opts.Mode),
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
					"key_prefix", keyPrefix(key),
				)
				WriteError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "Invalid credentials")
				return
			}

			// Authentication successful
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken extracts the token from Authorization: Bearer <token> header.
func extractBearerToken(r *http.Request) (token string, errCode ErrorCode, errMsg string) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrCodeUnauthorized, "Authorization header required"
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) {
		return "", ErrCodeUnauthorized, "Invalid authorization format"
	}

	token = authHeader[len(bearerPrefix):]
	if token == "" {
		return "", ErrCodeUnauthorized, "Bearer token is empty"
	}

	return token, "", ""
}

// extractAPIKey extracts the key from X-API-Key header.
func extractAPIKey(r *http.Request) (key string, errCode ErrorCode, errMsg string) {
	key = r.Header.Get("X-API-Key")
	if key == "" {
		return "", ErrCodeUnauthorized, "X-API-Key header required"
	}
	return key, "", ""
}

// keyPrefix returns the first 4 characters of the key for logging.
// This allows identifying which key was used without exposing the full key.
func keyPrefix(key string) string {
	if len(key) < 4 {
		return "****"
	}
	return key[:4] + "****"
}

// modeString returns a human-readable string for the authentication mode.
func modeString(mode AuthMode) string {
	switch mode {
	case AuthModeBearer:
		return "bearer"
	case AuthModeAPIKey:
		return "api_key"
	default:
		return "unknown"
	}
}
