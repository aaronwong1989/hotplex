// Package server provides HTTP utilities shared across handlers.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		if logger != nil {
			logger.Error("failed to encode JSON response", "error", err)
		} else {
			slog.Error("failed to encode JSON response", "error", err)
		}
	}
}

// WriteError writes a JSON error response with status code and message.
func WriteError(w http.ResponseWriter, status int, message string, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]string{"error": message}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		if logger != nil {
			logger.Error("failed to encode error response", "error", err)
		} else {
			slog.Error("failed to encode error response", "error", err)
		}
	}
}
