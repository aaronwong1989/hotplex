package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/hrygo/hotplex/internal/server"
	"github.com/hrygo/hotplex/pkg/hotplex"
)

func main() {
	// Configure logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Info("Starting HotPlex Proxy Server...")

	// Initialize HotPlex Core Engine
	opts := hotplex.EngineOptions{
		Timeout: 30 * time.Minute,
		Logger:  logger,
	}

	engine, err := hotplex.NewEngine(opts)
	if err != nil {
		logger.Error("Failed to initialize HotPlex engine", "error", err)
		os.Exit(1)
	}
	defer engine.Close()

	// Initialize WebSocket handler
	wsHandler := server.NewWebSocketHandler(engine, logger)

	// Setup routes
	http.Handle("/ws/v1/agent", wsHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Listening on", "port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
