// Package main is the entry point for the hotplexd daemon.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/hrygo/hotplex"
	"github.com/hrygo/hotplex/brain"
	"github.com/hrygo/hotplex/chatapps"
	"github.com/hrygo/hotplex/cmd/hotplexd/cmd"
	croncmd "github.com/hrygo/hotplex/cmd/hotplexd/cmd/cron"
	relaycmd "github.com/hrygo/hotplex/cmd/hotplexd/cmd/relay"
	"github.com/hrygo/hotplex/cmd/hotplexd/cmd/session"
	"github.com/hrygo/hotplex/internal/admin"
	"github.com/hrygo/hotplex/internal/config"
	"github.com/hrygo/hotplex/internal/security"
	"github.com/hrygo/hotplex/internal/server"
	adminwebhook "github.com/hrygo/hotplex/internal/server/admin"
	"github.com/hrygo/hotplex/internal/sys"
	"github.com/hrygo/hotplex/provider"
	"github.com/joho/godotenv"
)

var (
	version = "v0.0.0-dev"
	commit  = "unknown"
	builtBy = "source"
)

func main() {
	// Set version info for cobra commands
	cmd.Version = version
	cmd.Commit = commit

	// Register session subcommands
	cmd.RootCmd.AddCommand(session.SessionCmd)
	cmd.RootCmd.AddCommand(croncmd.SessionCmd)
	cmd.RootCmd.AddCommand(relaycmd.SessionCmd)

	// Handle start command specially
	if len(os.Args) > 1 && os.Args[1] == "start" {
		runDaemon()
		return
	}

	// For other commands, use Cobra
	cmd.Execute()
}

func runDaemon() {
	// Parse command line flags
	serverConfig := flag.String("config", "", "Server config YAML file")
	envFileFlag := flag.String("env-file", "", "Path to .env file")
	adminPort := flag.String("admin-port", "", "Admin API server port (default: 9080)")
	flag.Parse()

	// 0. Ensure HOME environment variable is set
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			homeDir = h
			_ = os.Setenv("HOME", homeDir)
		}
	}

	// Resolve admin port: CLI flag > env var > default
	resolvedAdminPort := *adminPort
	if resolvedAdminPort == "" {
		if envPort := os.Getenv("HOTPLEX_ADMIN_PORT"); envPort != "" {
			resolvedAdminPort = envPort
		} else {
			resolvedAdminPort = "9080" // default
		}
	}

	// 1. Load .env file
	loadEnvFile(envFileFlag)

	// Expand tilde in path environment variables
	expandPathEnvVars()

	// 2. Load server configuration
	serverConfigPath := config.ResolveConfigPath(*serverConfig)
	serverCfg, cfgLogLevel, cfgLogFormat := loadServerConfig(serverConfigPath)

	// Apply precedence: env vars > config file > defaults
	if envLogLevel := strings.ToUpper(os.Getenv("HOTPLEX_LOG_LEVEL")); envLogLevel != "" {
		switch envLogLevel {
		case "DEBUG":
			cfgLogLevel = slog.LevelDebug
		case "WARN":
			cfgLogLevel = slog.LevelWarn
		case "ERROR":
			cfgLogLevel = slog.LevelError
		}
	}
	if envLogFormat := os.Getenv("HOTPLEX_LOG_FORMAT"); envLogFormat == "json" {
		cfgLogFormat = "json"
	}

	// 4. Initialize logger
	logger := initLogger(cfgLogLevel, cfgLogFormat)
	slog.SetDefault(logger)

	logger.Info("🔥 HotPlex Daemon starting...",
		"version", version,
		"commit", commit,
		"built_by", builtBy)

	// 5. Initialize Native Brain
	if err := brain.Init(logger); err != nil {
		logger.Warn("Native Brain initialization error (fail-open)", "error", err)
	}

	// 6. Create Engine
	engine, adminToken := createEngine(logger, serverCfg)

	// 7. Setup HTTP handlers
	mainRouter, chatappsMgr := setupHTTPHandlers(engine, adminToken, logger, serverCfg)

	// 7.1 Initialize BridgeServer for external platform adapters
	var bridgeServer *server.BridgeServer
	bridgePort := ""
	if serverCfg != nil {
		bridgePort = serverCfg.GetBridgePort()
	}
	if bridgePort != "" {
		bridgeToken := ""
		if serverCfg != nil {
			bridgeToken = serverCfg.GetBridgeToken()
		}
		bridgeServer = server.NewBridgeServer(0, bridgeToken, logger)
		if chatappsMgr != nil {
			bridgeServer.InjectAdapterManager(chatappsMgr)
		}
		bridgeAddr := ":" + bridgePort
		bridgeMux := http.NewServeMux()
		bridgeMux.Handle("/bridge/v1/", bridgeServer)
		bridgeHTTPSrv := &http.Server{
			Addr:    bridgeAddr,
			Handler: bridgeMux,
		}
		go func() {
			logger.Info("BridgeServer listening", "port", bridgePort)
			if err := bridgeHTTPSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("BridgeServer failed", "error", err)
			}
		}()
		logger.Info("BridgeServer initialized", "port", bridgePort)
	}

	// 7.2 Cleanup safety net (deferred before servers start)
	defer func() {
		logger.Info("Executing final cleanup safety net...")
		if bridgeServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := bridgeServer.Shutdown(ctx); err != nil {
				logger.Error("BridgeServer shutdown failed", "error", err)
			}
			cancel()
		}
		if chatappsMgr != nil {
			if err := chatappsMgr.StopAll(); err != nil {
				logger.Error("ChatApps cleanup failed", "error", err)
			}
		}
		if engine != nil {
			if err := engine.Close(); err != nil {
				logger.Error("Core engine cleanup failed", "error", err)
			}
		}
	}()

	// 8. Start Admin Server (independent port)
	adminServer := admin.NewServer(engine, nil, nil, resolvedAdminPort, adminToken, time.Now(), logger)
	adminServer.Start()

	// Monitor admin server startup
	select {
	case err := <-adminServer.ErrChan():
		logger.Error("Admin server failed to start", "error", err)
		os.Exit(1)
	default:
	}

	// 9. Start Main HTTP Server
	port := "8080"
	if serverCfg != nil {
		port = serverCfg.GetPort()
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mainRouter,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Main server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
			stop <- syscall.SIGTERM
		}
	}()

	<-stop
	logger.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := adminServer.Stop(shutdownCtx); err != nil {
		logger.Error("Admin server shutdown failed", "error", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
	}
	if err := engine.Close(); err != nil {
		logger.Error("Engine cleanup failed", "error", err)
	}
	if chatappsMgr != nil {
		if err := chatappsMgr.StopAll(); err != nil {
			logger.Error("ChatApps cleanup failed", "error", err)
		}
	}
}

// Helper functions

func loadEnvFile(envFileFlag *string) {
	envPath := *envFileFlag
	if envPath == "" {
		envPath = os.Getenv("ENV_FILE")
	}

	if envPath != "" {
		if err := godotenv.Load(envPath); err != nil {
			slog.Warn("Failed to load specified env file", "path", envPath, "error", err)
		} else {
			_ = os.Setenv("ENV_FILE", envPath)
		}
	} else {
		homeDir, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(homeDir, ".hotplex", ".env"),
			".env",
			filepath.Join(sys.ConfigDir(), ".env"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				if err := godotenv.Load(c); err == nil {
					_ = os.Setenv("ENV_FILE", c)
					break
				}
			}
		}
	}
}

func expandPathEnvVars() {
	pathEnvVars := []string{
		"HOTPLEX_PROJECTS_DIR",
		"HOTPLEX_DATA_ROOT",
		"HOTPLEX_MESSAGE_STORE_SQLITE_PATH",
		"HOTPLEX_CHATAPPS_CONFIG_DIR",
	}
	for _, envVar := range pathEnvVars {
		if val := os.Getenv(envVar); val != "" {
			_ = os.Setenv(envVar, sys.ExpandPath(val))
		}
	}
}

// loadServerConfig loads the server configuration from the given path.
func loadServerConfig(configPath string) (*config.ServerLoader, slog.Level, string) {
	if configPath == "" {
		configPath = config.ResolveConfigPath("")
	}
	if configPath == "" {
		return nil, slog.LevelInfo, "text"
	}

	serverCfg, err := config.NewServerLoader(configPath, nil)
	if err != nil {
		slog.Warn("Failed to load server config", "error", err)
		return nil, slog.LevelInfo, "text"
	}

	cfg := serverCfg.Get()
	logLevel := slog.LevelInfo
	switch strings.ToUpper(cfg.Server.LogLevel) {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	}
	logFormat := strings.ToLower(cfg.Server.LogFormat)

	return serverCfg, logLevel, logFormat
}

func initLogger(logLevel slog.Level, logFormat string) *slog.Logger {
	logOpts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					file := source.File
					file = strings.TrimPrefix(file, "github.com/hrygo/hotplex/")
					file = strings.TrimPrefix(file, "./")
					return slog.String("source", file)
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, logOpts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, logOpts)
	}

	return slog.New(handler)
}

func createEngine(logger *slog.Logger, serverCfg *config.ServerLoader) (*hotplex.Engine, string) {
	idleTimeout := 1 * time.Hour
	executionTimeout := 30 * time.Minute
	var baseSystemPrompt string

	if serverCfg != nil {
		idleTimeout = serverCfg.GetIdleTimeout()
		executionTimeout = serverCfg.GetTimeout()
		baseSystemPrompt = serverCfg.GetSystemPrompt()
	}

	providerType := provider.ProviderType(os.Getenv("HOTPLEX_PROVIDER_TYPE"))
	if providerType == "" {
		providerType = provider.ProviderTypeClaudeCode
	}

	providerBinary := os.Getenv("HOTPLEX_PROVIDER_BINARY")
	providerModel := os.Getenv("HOTPLEX_PROVIDER_MODEL")

	prv, err := provider.CreateProvider(provider.ProviderConfig{
		Type:         providerType,
		Enabled:      provider.PtrBool(true),
		BinaryPath:   providerBinary,
		DefaultModel: providerModel,
	})
	if err != nil {
		logger.Error("Failed to create provider", "type", providerType, "error", err)
		os.Exit(1)
	}

	var adminToken string
	if serverCfg != nil {
		adminToken = serverCfg.Get().Security.APIKey
	}

	if adminToken == "" {
		logger.Warn("SECURITY WARNING: No admin token configured. " +
			"Set HOTPLEX_API_KEY or HOTPLEX_API_KEYS for production use.")
	}

	opts := hotplex.EngineOptions{
		Timeout:          executionTimeout,
		IdleTimeout:      idleTimeout,
		Logger:           logger,
		AdminToken:       adminToken,
		Provider:         prv,
		BaseSystemPrompt: baseSystemPrompt,
	}

	engine, err := hotplex.NewEngine(opts)
	if err != nil {
		logger.Error("Failed to initialize HotPlex engine", "error", err)
		os.Exit(1)
	}

	return engine, adminToken
}

func setupHTTPHandlers(engine *hotplex.Engine, adminToken string, logger *slog.Logger, serverCfg *config.ServerLoader) (*mux.Router, *chatapps.AdapterManager) {
	r := mux.NewRouter()

	// WebSocket handler
	var securityKeys []string
	if serverCfg != nil {
		securityKeys = append(securityKeys, serverCfg.Get().Security.APIKey)
	}
	corsConfig := server.NewSecurityConfig(logger, securityKeys...)
	wsHandler := server.NewHotPlexWSHandler(engine, logger, corsConfig)
	r.Handle("/ws/v1/agent", wsHandler)

	// WAF detector for HTTP handlers
	wafDetector := security.NewDetector(logger)

	// Relay handler for bot-to-bot communication
	relayHandler := server.NewRelayHandler(engine, logger, wafDetector)
	relayHandler.RegisterRoutes(r)

	// OpenCode compatibility
	if os.Getenv("HOTPLEX_OPENCODE_COMPAT_ENABLED") != "false" {
		openCodeSrv := server.NewOpenCodeHTTPHandler(engine, logger, corsConfig)
		ocRouter := mux.NewRouter()
		openCodeSrv.RegisterRoutes(ocRouter)
		r.Handle("/global/", ocRouter)
		r.Handle("/session", ocRouter)
		r.Handle("/session/", ocRouter)
		r.Handle("/config", ocRouter)
		logger.Info("OpenCode compatibility server initialized")
	}

	// Observability handlers
	healthHandler := server.NewHealthHandler()
	metricsHandler := server.NewMetricsHandler()
	readyHandler := server.NewReadyHandler(func() bool { return engine != nil })
	liveHandler := server.NewLiveHandler()

	r.Handle("/health", healthHandler)
	r.Handle("/health/ready", readyHandler)
	r.Handle("/health/live", liveHandler)
	r.Handle("/metrics", metricsHandler)

	// Enhanced Admin Webhook API (internal/server/admin) at /api/v1/admin/
	adminServer := adminwebhook.NewAdminServer(adminwebhook.AdminServerOptions{
		Engine:   engine,
		AdminKey: adminToken,
		Logger:   logger,
	})
	r.Handle("/api/v1/admin/", adminServer)

	// ChatApps adapters
	configDir := os.Getenv("HOTPLEX_CHATAPPS_CONFIG_DIR")
	var chatappsMgr *chatapps.AdapterManager
	if chatapps.IsEnabled(configDir) {
		chatappsHandler, mgr, err := chatapps.Setup(context.Background(), logger, configDir)
		if err != nil {
			logger.Error("Failed to setup chatapps", "error", err)
		} else {
			r.Handle("/webhook/", chatappsHandler)
			logger.Info("ChatApps adapters initialized")
		}
		chatappsMgr = mgr
	}

	return r, chatappsMgr
}
