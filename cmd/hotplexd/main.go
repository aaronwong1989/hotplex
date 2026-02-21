package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hrygo/hotplex"
	"github.com/hrygo/hotplex/internal/server"
)

func main() {
	// Configure logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Info("Starting HotPlex Proxy Server...")

	// 1. Initialize HotPlex Core Engine
	idleTimeout := 30 * time.Minute
	if val := os.Getenv("IDLE_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			idleTimeout = d
		}
	}

	opts := hotplex.EngineOptions{
		Timeout:     30 * time.Minute,
		IdleTimeout: idleTimeout,
		Logger:      logger,
	}

	engine, err := hotplex.NewEngine(opts)
	if err != nil {
		logger.Error("Failed to initialize HotPlex engine", "error", err)
		os.Exit(1)
	}

	// 2. Initialize CORS configuration and WebSocket handler
	corsConfig := server.NewCORSConfig(logger)
	wsHandler := server.NewWebSocketHandler(engine, logger, corsConfig)
	http.Handle("/ws/v1/agent", wsHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 3. Listen for OS signals to ensure graceful shutdown of engine and child processes
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: http.DefaultServeMux,
	}

	go func() {
		logger.Info("Listening on", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("Shutting down gracefully...")

	// Create a timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
	}

	// engine.Close() will sweep all active process groups
	if err := engine.Close(); err != nil {
		logger.Error("Engine shutdown failed", "error", err)
	}

	logger.Info("HotPlex Proxy Server stopped")
}
