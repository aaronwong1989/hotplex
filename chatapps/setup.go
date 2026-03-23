package chatapps

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/engine"
	"github.com/hrygo/hotplex/internal/permission"
	"github.com/hrygo/hotplex/internal/sys"
	"github.com/hrygo/hotplex/provider"
)

// IsEnabled returns true if ChatApps should be activated based on environment variables or flags.
// It returns true if any of the following is true:
// 1. HOTPLEX_CHATAPPS_ENABLED environment variable is "true"
// 2. configDir parameter is not empty (explicitly set via --config flag)
// 3. HOTPLEX_CHATAPPS_CONFIG_DIR environment variable is not empty
func IsEnabled(configDir string) bool {
	if os.Getenv("HOTPLEX_CHATAPPS_ENABLED") == "true" {
		return true
	}
	if configDir != "" {
		return true
	}
	if os.Getenv("HOTPLEX_CHATAPPS_CONFIG_DIR") != "" {
		return true
	}
	return false
}

// Setup initializes all enabled ChatApps and their dedicated Engines.
// It returns an http.Handler that handles all webhook routes.
// The configDir parameter takes priority over HOTPLEX_CHATAPPS_CONFIG_DIR environment variable.
func Setup(ctx context.Context, logger *slog.Logger, configDir ...string) (http.Handler, *AdapterManager, error) {
	// Config directory search priority:
	// 1. configDir parameter (--config flag, highest)
	// 2. HOTPLEX_CHATAPPS_CONFIG_DIR environment variable
	// 3. ~/.hotplex/configs (user config)
	// 4. ./configs/admin (default, for admin bot)
	dir := ""

	// 1. configDir parameter (highest priority)
	if len(configDir) > 0 && configDir[0] != "" {
		dir = configDir[0]
	}

	// 2. HOTPLEX_CHATAPPS_CONFIG_DIR env var
	if dir == "" {
		dir = os.Getenv("HOTPLEX_CHATAPPS_CONFIG_DIR")
	}

	// 3. User config directory
	if dir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Debug("Could not determine user home directory", "cause", err)
		} else {
			userConfigDir := filepath.Join(homeDir, ".hotplex", "configs")
			if _, err := os.Stat(userConfigDir); err != nil {
				logger.Debug("User config directory does not exist", "path", userConfigDir, "cause", err)
			} else {
				dir = userConfigDir
				logger.Info("Using user config directory", "path", dir)
			}
		}
	}

	// 4. Default config directory (admin bot)
	if dir == "" {
		dir = "configs/admin"
		// Check if default config directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			logger.Info("Default config directory not found, skipping config loading", "path", dir)
			dir = ""
		}
	}

	var loader *ConfigLoader
	var err error
	if dir != "" {
		loader, err = NewConfigLoader(dir, logger)
		if err != nil {
			logger.Info("Could not load configuration from directory", "path", dir, "cause", err)
			// Don't fail completely, try to continue with env-based config
		}
	}

	manager := NewAdapterManager(logger)

	// Auto-discover platforms from the global registry
	registry := base.GlobalAdapterRegistry()
	for _, platform := range registry.List() {
		factory, _ := registry.Get(platform)
		setupPlatform(ctx, factory, loader, manager, logger)
	}

	if err := manager.StartAll(ctx); err != nil {
		return nil, nil, fmt.Errorf("start all adapters: %w", err)
	}

	if len(manager.ListPlatforms()) == 0 {
		logger.Error("No ChatApp platforms were successfully initialized. Please check your configuration.")
	} else {
		logger.Info("ChatApps setup completed", "platforms", manager.ListPlatforms())
	}

	return manager.Handler(), manager, nil
}

// WorkDirResult holds the resolved work directory information
type WorkDirResult struct {
	ResolvedPath  string // Expanded absolute path (empty if not configured or expansion failed)
	PlatformName  string // Platform name for default path construction
	DefaultReason string // Reason for using default directory (for logging)
}

// resolveWorkDir processes the work directory configuration once at setup time.
// This avoids repeated path expansion and log noise on every message (Issue #294).
func resolveWorkDir(configuredPath, platform string, logger *slog.Logger) WorkDirResult {
	result := WorkDirResult{
		PlatformName:  platform,
		DefaultReason: "no work_dir configured",
	}

	if configuredPath == "" {
		return result
	}

	// Attempt path expansion
	resolved := sys.ExpandPath(configuredPath)

	// CRITICAL: Validate path expansion succeeded (Code Review Finding #1)
	if resolved == "" {
		logger.Warn("Work directory path expansion failed - using default temp directory",
			"platform", platform,
			"configured_path", configuredPath,
			"reason", "path expansion returned empty string (possible unset environment variable)")
		result.DefaultReason = "work_dir configured but path expansion failed"
		return result
	}

	// Success - log and return resolved path
	logger.Info("Work directory initialized",
		"platform", platform,
		"config", configuredPath,
		"path", resolved)

	result.ResolvedPath = resolved
	result.DefaultReason = "" // Not using default
	return result
}

func setupPlatform(
	_ context.Context,
	factory base.AdapterFactory,
	loader *ConfigLoader,
	manager *AdapterManager,
	logger *slog.Logger,
) {
	platform := factory.Platform()

	// Early exit if required environment variables are not set
	if required := factory.RequiredEnvVars(); len(required) > 0 && !base.HasRequiredEnvVars(required) {
		logger.Info("Platform skipped (missing required env vars)", "platform", platform, "required", required)
		return
	}

	var pc *PlatformConfig
	if loader != nil {
		pc = loader.GetConfig(platform)
	}
	if pc == nil {
		pc = &PlatformConfig{Platform: platform}
	}

	// 1. Create dedicated Engine for this platform
	eng, err := createEngineForPlatform(pc, logger)
	if err != nil {
		logger.Error("Failed to create engine for platform", "platform", platform, "error", err)
		return
	}
	manager.RegisterEngine(eng)

	// 2. Resolve workDir first (needed for permissions directory)
	workDirResult := resolveWorkDir(pc.Engine.WorkDir, platform, logger)
	workDir := workDirResult.ResolvedPath
	if workDir == "" {
		workDir = filepath.Join("/tmp/hotplex-chatapps", platform)
	}

	// 3. Create PermissionMatcher in workDir/permissions and and inject into engine
	permDir := filepath.Join(workDir, "permissions")
	permissionMatcher := permission.NewPermissionMatcher(permDir)
	eng.SetPermissionMatcher(permissionMatcher)

	// 3. Create Adapter via factory
	rawAdapter := factory.New(pc)
	if rawAdapter == nil {
		logger.Info("Platform not initialized (likely missing credentials)", "platform", platform)
		return
	}
	adapter, ok := rawAdapter.(ChatAdapter)
	if !ok {
		logger.Error("Factory returned non-ChatAdapter", "platform", platform)
		return
	}

	// 4. Wire up Engine for slash command support (platform-agnostic via interface)
	if engineSupport, ok := adapter.(base.EngineSupport); ok {
		engineSupport.SetEngine(eng)
		logger.Debug("Engine injected", "platform", platform)
	}

	// 5. For adapters that need botID for permission management
	if botIDSupport, ok := adapter.(base.EngineSupportWithBotID); ok {
		botIDSupport.SetBotID(pc.Security.Permission.BotUserID)
		logger.Debug("BotID injected for permission management", "platform", platform, "bot_id", pc.Security.Permission.BotUserID)
	}

	// 6. Create EngineMessageHandler
	wrappedEng := &engineWrapper{eng: eng}

	msgHandler := NewEngineMessageHandler(wrappedEng, manager,
		WithConfigLoader(loader),
		WithLogger(logger),
		WithWorkDirFn(func(sessionID string) string {
			if workDirResult.ResolvedPath != "" {
				return workDirResult.ResolvedPath
			}
			defaultDir := filepath.Join("/tmp/hotplex-chatapps", workDirResult.PlatformName, sessionID)
			logger.Debug("Using default temp work_dir",
				"platform", workDirResult.PlatformName,
				"session_id", sessionID,
				"default_path", defaultDir,
				"reason", workDirResult.DefaultReason)
			return defaultDir
		}),
	)

	// 7. Link everything
	adapter.SetHandler(msgHandler.Handle)

	// 8. Register adapter
	if err := manager.Register(adapter); err != nil {
		logger.Error("Failed to register adapter", "platform", platform, "error", err)
		return
	}

	// 9. Call factory PostSetup hook (e.g., AppHome for Slack)
	setupCtx := &base.SetupContext{
		Manager:        manager,
		Loader:         loader,
		Engine:         eng,
		PermDir:        permDir,
		Logger:         logger,
		Platform:       platform,
		PlatformConfig: pc,
		WorkDirFn: func(sessionID string) string {
			if workDirResult.ResolvedPath != "" {
				return workDirResult.ResolvedPath
			}
			return filepath.Join("/tmp/hotplex-chatapps", platform, sessionID)
		},
	}
	factory.PostSetup(context.Background(), adapter, setupCtx)

	if pc.SourceFile != "" {
		logger.Info("Platform successfully initialized from configuration file", "platform", platform, "file", pc.SourceFile)
	} else {
		logger.Info("Platform successfully initialized from environment variables", "platform", platform)
	}
}

func createEngineForPlatform(pc *PlatformConfig, logger *slog.Logger) (*engine.Engine, error) {
	// Initialize Provider
	pCfg := pc.Provider
	if pCfg.Type == "" {
		pCfg.Type = provider.ProviderTypeClaudeCode
	}
	if pCfg.Enabled == nil {
		enabled := true
		pCfg.Enabled = &enabled
	}

	prv, err := provider.CreateProvider(pCfg)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	// Engine options with defaults
	timeout := pc.Engine.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	idleTimeout := pc.Engine.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 30 * time.Minute
	}

	// Tool Filtering Logic: Provider-level takes precedence over Engine-level
	allowedTools := pc.Provider.AllowedTools
	if len(allowedTools) == 0 {
		allowedTools = pc.Engine.AllowedTools
	}
	disallowedTools := pc.Provider.DisallowedTools
	if len(disallowedTools) == 0 {
		disallowedTools = pc.Engine.DisallowedTools
	}

	opts := engine.EngineOptions{
		Timeout:          timeout,
		IdleTimeout:      idleTimeout,
		Logger:           logger,
		Namespace:        pc.Platform,
		BaseSystemPrompt: pc.SystemPrompt,
		Provider:         prv,
		// Pass permission settings from YAML config
		PermissionMode:             pc.Provider.DefaultPermissionMode,
		DangerouslySkipPermissions: BoolValue(pc.Provider.DangerouslySkipPermissions, true),
		AllowedTools:               allowedTools,
		DisallowedTools:            disallowedTools,
	}

	return engine.NewEngine(opts)
}

// ExpandPath expands ~ to the user's home directory and cleans the path.
// Supports both ~ and ~/path formats.
// Returns an empty string if the path contains traversal attacks.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	expanded := sys.ExpandPath(path)
	if expanded == "" {
		return ""
	}
	if strings.HasPrefix(expanded, "/") && isSensitivePath(expanded) {
		return "" // Block access to sensitive paths
	}
	return filepath.Clean(expanded)
}

// isSensitivePath checks if a path points to a sensitive system location
func isSensitivePath(path string) bool {
	// List of sensitive directories to block
	sensitivePrefixes := []string{
		"/etc/",
		"/etc",
		"/var/",
		"/var",
		"/usr/",
		"/usr",
		"/bin",
		"/sbin",
		"/root",
		"/proc/",
		"/proc",
		"/sys/",
		"/sys",
		"/boot",
		"/dev/",
		"/dev",
	}

	lowerPath := strings.ToLower(path)
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(lowerPath, prefix) {
			return true
		}
	}
	return false
}
