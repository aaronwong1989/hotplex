package security

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// LandlockConfig holds configuration for Landlock file system restrictions.
type LandlockConfig struct {
	// AllowedDirs defines directories where file operations are permitted.
	// All file operations outside these directories will be blocked.
	AllowedDirs []string

	// AllowRead defines if read operations are allowed.
	AllowRead bool

	// AllowWrite defines if write operations are allowed.
	AllowWrite bool

	// AllowExec defines if execute operations are allowed.
	AllowExec bool

	// DeniedExtensions file extensions that are always denied.
	DeniedExtensions []string

	// Logger for security events.
	Logger *slog.Logger
}

// LandlockEnforcer implements file system sandboxing using Linux Landlock LSM.
type LandlockEnforcer struct {
	config  LandlockConfig
	logger  *slog.Logger
	enabled bool
	mu      sync.RWMutex
	ruleset int
}

// LandlockAccess defines file system access rights.
type LandlockAccess uint64

const (
	// LandlockAccessReadFile - read file content
	LandlockAccessReadFile LandlockAccess = unix.LANDLOCK_ACCESS_FS_READ_FILE
	// LandlockAccessWriteFile - write to file
	LandlockAccessWriteFile LandlockAccess = unix.LANDLOCK_ACCESS_FS_WRITE_FILE
	// LandlockAccessExec - execute file
	LandlockAccessExec LandlockAccess = unix.LANDLOCK_ACCESS_FS_EXECUTE
	// LandlockAccessReadDir - read directory
	LandlockAccessReadDir LandlockAccess = unix.LANDLOCK_ACCESS_FS_READ_DIR
	// LandlockAccessRemoveDir - remove directory
	LandlockAccessRemoveDir LandlockAccess = unix.LANDLOCK_ACCESS_FS_REMOVE_DIR
	// LandlockAccessRemoveFile - remove file
	LandlockAccessRemoveFile LandlockAccess = unix.LANDLOCK_ACCESS_FS_REMOVE_FILE
	// LandlockAccessMkdir - create directory
	LandlockAccessMkdir LandlockAccess = unix.LANDLOCK_ACCESS_FS_MKDIR
	// LandlockAccessCreateFile - create file
	LandlockAccessCreateFile LandlockAccess = unix.LANDLOCK_ACCESS_FS_CREATE_FILE
	// LandlockAccessCreateDir - create directory
	LandlockAccessCreateDir LandlockAccess = unix.LANDLOCK_ACCESS_FS_CREATE_DIR
	// LandlockAccessLink - create hard link
	LandlockAccessLink LandlockAccess = unix.LANDLOCK_ACCESS_FS_LINK
	// LandlockAccessSymlink - create symbolic link
	LandlockAccessSymlink LandlockAccess = unix.LANDLOCK_ACCESS_FS_SYMLINK
)

// DefaultLandlockConfig returns a default safe configuration.
func DefaultLandlockConfig() LandlockConfig {
	return LandlockConfig{
		AllowRead:         true,
		AllowWrite:        true,
		AllowExec:         false,
		DeniedExtensions:  []string{".so", ".dll", ".dylib", ".exe"},
		AllowedDirs:       []string{},
		Logger:            slog.Default(),
	}
}

// NewLandlockEnforcer creates a new Landlock enforcer with the given configuration.
func NewLandlockEnforcer(config LandlockConfig) (*LandlockEnforcer, error) {
	le := &LandlockEnforcer{
		config: config,
		logger: config.Logger,
	}

	if le.logger == nil {
		le.logger = slog.Default()
	}

	// Validate and clean paths
	for i, dir := range config.AllowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed directory %q: %w", dir, err)
		}
		config.AllowedDirs[i] = absDir
	}

	// Check if Landlock is supported
	if !IsLandlockSupported() {
		le.logger.Warn("Landlock is not supported on this system - running in degraded mode")
		return le, nil
	}

	// Initialize ruleset
	if err := le.initializeRuleset(); err != nil {
		return nil, fmt.Errorf("failed to initialize Landlock ruleset: %w", err)
	}

	le.enabled = true
	le.logger.Info("Landlock enforcer initialized",
		"allowed_dirs", len(config.AllowedDirs),
		"allow_read", config.AllowRead,
		"allow_write", config.AllowWrite,
		"allow_exec", config.AllowExec)

	return le, nil
}

// IsLandlockSupported checks if the Linux kernel supports Landlock.
func IsLandlockSupported() bool {
	// Check if we can access Landlock syscall
	// Landlock was added in kernel 5.13
	var stat unix.Stat_t
	if err := unix.Stat("/proc/sys/kernel/uname", &stat); err != nil {
		return false
	}

	// Try to get Landlock ABI version
	// If this fails, Landlock is not supported
	abi, err := unix.LandlockGetAbi()
	if err != nil {
		return false
	}

	// ABI version 1 is the minimum supported
	return abi >= 1
}

// initializeRuleset creates and configures the Landlock ruleset.
func (le *LandlockEnforcer) initializeRuleset error {
	// Create ruleset
	le.ruleset, err := unix.LandlockCreateRuleset(&unix.LandlockRulesetAttr{
		HandledAccessFs: le.calculateAccessMask(),
	}, 0)
	if err != nil {
		return fmt.Errorf("failed to create ruleset: %w", err)
	}

	// Add directory restrictions
	if err := le.addDirectoryRestrictions(); err != nil {
		unix.Close(le.ruleset)
		return fmt.Errorf("failed to add directory restrictions: %w", err)
	}

	return nil
}

// calculateAccessMask calculates the Landlock access mask based on configuration.
func (le *LandlockEnforcer) calculateAccessMask() LandlockAccess {
	var mask LandlockAccess

	if le.config.AllowRead {
		mask |= LandlockAccessReadFile | LandlockAccessReadDir
	}
	if le.config.AllowWrite {
		mask |= LandlockAccessWriteFile | LandlockAccessMkdir |
			LandlockAccessCreateFile | LandlockAccessCreateDir |
			LandlockAccessRemoveDir | LandlockAccessRemoveFile |
			LandlockAccessLink | LandlockAccessSymlink
	}
	if le.config.AllowExec {
		mask |= LandlockAccessExec
	}

	// Always at least allow reading attributes
	if mask == 0 {
		mask = LandlockAccessReadFile
	}

	return mask
}

// addDirectoryRestrictions adds directory restrictions to the ruleset.
func (le *LandlockEnforcer) addDirectoryRestrictions() error {
	for _, dir := range le.config.AllowedDirs {
		// Get file descriptor for the directory
		fd, err := unix.Open(dir, unix.O_RDONLY, 0)
		if err != nil {
			le.logger.Warn("Failed to open directory for Landlock",
				"dir", dir, "error", err)
			continue
		}
		defer unix.Close(fd)

		// Add to ruleset
		err = unix.LandlockAddRule(le.ruleset, unix.LANDLOCK_RULE_PATH_BENEATH, &unix.LandlockPathBeneathAttr{
			FsAllowed: le.calculateAccessMask(),
			DirFd:     fd,
		}, 0)
		if err != nil {
			return fmt.Errorf("failed to add rule for %q: %w", dir, err)
		}

		le.logger.Debug("Added Landlock restriction", "dir", dir)
	}

	return nil
}

// Enforce applies the Landlock restrictions to the current process.
func (le *LandlockEnforcer) Enforce() error {
	le.mu.Lock()
	defer le.mu.Unlock()

	if !le.enabled {
		le.logger.Warn("Landlock is not enabled - cannot enforce")
		return fmt.Errorf("landlock not enabled")
	}

	if le.ruleset == 0 {
		return fmt.Errorf("ruleset not initialized")
	}

	// Restrict the current process
	if err := unix.LandlockRestrictSelf(le.ruleset, 0); err != nil {
		return fmt.Errorf("failed to restrict self: %w", err)
	}

	le.logger.Info("Landlock restrictions enforced successfully")
	return nil
}

// CheckPath checks if a path is allowed under current Landlock rules.
// This is a software-only check that doesn't require Landlock to be enforced.
func (le *LandlockEnforcer) CheckPath(path string) error {
	le.mu.RLock()
	defer le.mu.RUnlock()

	// Resolve to absolute path
	absPath, err := filepath.Abs(os.ExpandEnv(path))
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Normalize path
	absPath = filepath.Clean(absPath)

	// Check if we have any allowed directories
	if len(le.config.AllowedDirs) == 0 {
		return nil // No restrictions configured
	}

	// Check against allowed directories
	for _, allowed := range le.config.AllowedDirs {
		allowed = filepath.Clean(allowed)

		// Check if path is within allowed directory
		if absPath == allowed || filepath.HasPrefix(absPath, allowed+string(filepath.Separator)) {
			// Check extension restrictions
			if le.isExtensionDenied(absPath) {
				return fmt.Errorf("file extension denied: %s", filepath.Ext(absPath))
			}
			return nil
		}
	}

	return fmt.Errorf("path not in allowed directories: %s", path)
}

// isExtensionDenied checks if the file extension is denied.
func (le *LandlockEnforcer) isExtensionDenied(path string) bool {
	ext := filepath.Ext(path)
	for _, denied := range le.config.DeniedExtensions {
		if ext == denied || ext == "."+denied {
			return true
		}
	}
	return false
}

// UpdateAllowedDirs updates the list of allowed directories.
// Note: This requires re-creating the ruleset and re-enforcing.
func (le *LandlockEnforcer) UpdateAllowedDirs(dirs []string) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	// Clean paths
	cleaned := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("invalid directory %q: %w", dir, err)
		}
		cleaned = append(cleaned, absDir)
	}

	le.config.AllowedDirs = cleaned

	// Re-initialize ruleset if enabled
	if le.enabled {
		if le.ruleset != 0 {
			unix.Close(le.ruleset)
		}
		if err := le.initializeRuleset(); err != nil {
			return fmt.Errorf("failed to reinitialize ruleset: %w", err)
		}
	}

	le.logger.Info("Allowed directories updated", "count", len(cleaned))
	return nil
}

// IsEnabled returns whether Landlock is currently enabled.
func (le *LandlockEnforcer) IsEnabled() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.enabled
}

// Close releases Landlock resources.
func (le *LandlockEnforcer) Close() error {
	le.mu.Lock()
	defer le.mu.Unlock()

	if le.ruleset != 0 {
		err := unix.Close(le.ruleset)
		le.ruleset = 0
		return err
	}
	return nil
}

// PathTraversalPrevention provides path traversal attack prevention.
type PathTraversalPrevention struct {
	allowedRoots []string
	logger      *slog.Logger
}

// NewPathTraversalPrevention creates a new path traversal prevention checker.
func NewPathTraversalPrevention(allowedRoots []string, logger *slog.Logger) *PathTraversalPrevention {
	ptp := &PathTraversalPrevention{
		logger: logger,
	}

	if logger == nil {
		ptp.logger = slog.Default()
	}

	// Normalize and validate allowed roots
	for _, root := range allowedRoots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			ptp.logger.Warn("Invalid allowed root", "root", root, "error", err)
			continue
		}
		ptp.allowedRoots = append(ptp.allowedRoots, absRoot)
	}

	return ptp
}

// CheckPath validates that a path doesn't contain traversal attempts
// and stays within allowed directories.
func (ptp *PathTraversalPrevention) CheckPath(path string) error {
	// Expand environment variables
	path = os.ExpandEnv(path)

	// Check for path traversal patterns
	traversalPatterns := []string{
		"..",
		"%2e%2e",  // URL encoded
		"%252e",   // Double URL encoded
		"..\\",    // Windows-style
		"%2e%2e\\",
	}

	lowerPath := path
	for _, pattern := range traversalPatterns {
		if len(lowerPath) >= len(pattern)*2 {
			// Check for encoded patterns
			for i := 0; i <= len(lowerPath)-len(pattern)*2; i++ {
				sub := lowerPath[i : i+len(pattern)*2]
				if sub == pattern+pattern || sub == pattern+"/"+pattern {
					return fmt.Errorf("path traversal detected (encoded): %s", path)
				}
			}
		}

		// Direct traversal
		if contains(lowerPath, pattern) {
			return fmt.Errorf("path traversal detected: %s", path)
		}
	}

	// Resolve to absolute and check against allowed roots
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Clean the path (resolves .. and . segments)
	absPath = filepath.Clean(absPath)

	// Check against allowed roots
	if len(ptp.allowedRoots) > 0 {
		allowed := false
		for _, root := range ptp.allowedRoots {
			root = filepath.Clean(root)
			if absPath == root || filepath.HasPrefix(absPath, root+string(filepath.Separator)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path outside allowed roots: %s", path)
		}
	}

	return nil
}

// contains is a simple string contains check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

// findSubstring performs a simple substring search.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
