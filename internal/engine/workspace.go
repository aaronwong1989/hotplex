package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hrygo/hotplex/internal/security"
	"github.com/hrygo/hotplex/internal/security/audit"
)

// WorkspaceStatus represents the state of a workspace.
type WorkspaceStatus string

const (
	WorkspaceStatusActive  WorkspaceStatus = "active"
	WorkspaceStatusIdle    WorkspaceStatus = "idle"
	WorkspaceStatusBlocked WorkspaceStatus = "blocked"
	WorkspaceStatusDeleted WorkspaceStatus = "deleted"
)

// ResourceQuota defines resource limits for a workspace.
type ResourceQuota struct {
	// MemoryLimit is the maximum memory in bytes (0 = unlimited)
	MemoryLimit int64 `json:"memory_limit"`
	// CPUPercent is the maximum CPU percentage (0 = unlimited)
	CPUPercent int `json:"cpu_percent"`
	// MaxProcesses is the maximum number of processes (0 = unlimited)
	MaxProcesses int `json:"max_processes"`
	// DiskIOBytesPerSec is the maximum disk I/O bytes per second (0 = unlimited)
	DiskIOBytesPerSec int64 `json:"disk_io_bytes_per_sec"`
	// MaxSessions is the maximum concurrent sessions in this workspace
	MaxSessions int `json:"max_sessions"`
	// MaxWorkspaceSize is the maximum total size of files in the workspace (bytes)
	MaxWorkspaceSize int64 `json:"max_workspace_size"`
}

// DefaultResourceQuota returns a default quota with reasonable limits.
func DefaultResourceQuota() ResourceQuota {
	return ResourceQuota{
		MemoryLimit:       2 * 1024 * 1024 * 1024, // 2GB
		CPUPercent:        80,
		MaxProcesses:      50,
		DiskIOBytesPerSec: 100 * 1024 * 1024, // 100MB/s
		MaxSessions:       10,
		MaxWorkspaceSize:  10 * 1024 * 1024 * 1024, // 10GB
	}
}

// WorkspaceConfig contains configuration for a workspace.
type WorkspaceConfig struct {
	ID           string         // Unique workspace identifier
	Name         string        // Human-readable name
	RootPath     string        // Root directory for this workspace
	Quota        ResourceQuota  // Resource quota for this workspace
	AllowedPaths []string      // Additional paths allowed beyond RootPath
	Metadata     map[string]any // Additional metadata
	CreatedAt    time.Time
	CreatedBy    string // User who created the workspace
}

// Validate validates the workspace configuration.
func (wc *WorkspaceConfig) Validate() error {
	if wc.ID == "" {
		return errors.New("workspace: ID is required")
	}
	if wc.Name == "" {
		return errors.New("workspace: name is required")
	}
	if wc.RootPath == "" {
		return errors.New("workspace: root path is required")
	}
	if wc.CreatedBy == "" {
		return errors.New("workspace: created by is required")
	}

	// Validate root path is absolute
	if !filepath.IsAbs(wc.RootPath) {
		return errors.New("workspace: root path must be absolute")
	}

	return nil
}

// WorkspaceUsage represents current resource usage of a workspace.
type WorkspaceUsage struct {
	MemoryUsage     int64     `json:"memory_usage"`
	CPUPercent      int       `json:"cpu_percent"`
	ActiveProcesses int       `json:"active_processes"`
	ActiveSessions  int       `json:"active_sessions"`
	DiskUsage       int64     `json:"disk_usage"`
	LastActivity    time.Time `json:"last_activity"`
}

// Workspace represents an isolated execution environment with resource quotas.
type Workspace struct {
	Config    WorkspaceConfig
	Quota     ResourceQuota
	Status    WorkspaceStatus
	Usage     WorkspaceUsage
	mu        sync.RWMutex
	logger    *slog.Logger
	auditLog  security.AuditStore
	sessions  map[string]*Session
	createdAt time.Time
	lastActive time.Time
}

// WorkspaceManager defines the interface for managing workspaces.
type WorkspaceManager interface {
	// CreateWorkspace creates a new workspace with the given configuration
	CreateWorkspace(ctx context.Context, cfg WorkspaceConfig) (*Workspace, error)

	// GetWorkspace retrieves a workspace by ID
	GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, bool)

	// DeleteWorkspace removes a workspace and all its resources
	DeleteWorkspace(ctx context.Context, workspaceID string) error

	// ListWorkspaces returns all workspaces
	ListWorkspaces(ctx context.Context) []*Workspace

	// UpdateQuota updates the resource quota for a workspace
	UpdateQuota(ctx context.Context, workspaceID string, quota ResourceQuota) error

	// GetUsage returns current resource usage for a workspace
	GetUsage(ctx context.Context, workspaceID string) (WorkspaceUsage, error)

	// ValidatePath validates that a path is within the workspace boundaries
	ValidatePath(ctx context.Context, workspaceID string, path string) (bool, error)

	// AuditLog returns the audit store for a workspace
	AuditLog(ctx context.Context, workspaceID string) (security.AuditStore, error)

	// RegisterSession associates a session with a workspace
	RegisterSession(ctx context.Context, workspaceID string, session *Session) error

	// UnregisterSession removes a session from a workspace
	UnregisterSession(ctx context.Context, workspaceID string, sessionID string) error

	// Shutdown gracefully stops the workspace manager
	Shutdown()
}

// workspaceManager implements WorkspaceManager.
type workspaceManager struct {
	mu           sync.RWMutex
	workspaces   map[string]*Workspace
	logger       *slog.Logger
	auditDir     string
	defaultQuota ResourceQuota
	shutdownOnce sync.Once
	done         chan struct{}
}

// NewWorkspaceManager creates a new workspace manager.
func NewWorkspaceManager(logger *slog.Logger, auditDir string, defaultQuota ResourceQuota) WorkspaceManager {
	if logger == nil {
		logger = slog.Default()
	}
	if auditDir == "" {
		auditDir = filepath.Join(os.Getenv("HOME"), ".hotplex", "audit", "workspaces")
	}

	wm := &workspaceManager{
		workspaces:   make(map[string]*Workspace),
		logger:       logger,
		auditDir:     auditDir,
		defaultQuota: defaultQuota,
		done:         make(chan struct{}),
	}

	// Ensure audit directory exists
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		logger.Error("Failed to create workspace audit directory", "error", err)
	}

	return wm
}

// CreateWorkspace creates a new workspace.
func (wm *workspaceManager) CreateWorkspace(ctx context.Context, cfg WorkspaceConfig) (*Workspace, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid workspace config: %w", err)
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Check if workspace already exists
	if _, exists := wm.workspaces[cfg.ID]; exists {
		return nil, errors.New("workspace: already exists")
	}

	// Validate and create root directory
	if err := wm.validateAndCreateWorkspaceDir(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("workspace directory validation failed: %w", err)
	}

	// Use default quota if not specified
	quota := cfg.Quota
	if quota.MemoryLimit == 0 && quota.CPUPercent == 0 && quota.MaxProcesses == 0 {
		quota = wm.defaultQuota
	}

	// Create workspace audit store
	auditPath := filepath.Join(wm.auditDir, cfg.ID+".jsonl")
	var auditStore security.AuditStore
	store, err := audit.NewFileAuditStore(auditPath)
	if err != nil {
		wm.logger.Warn("Failed to create workspace audit store, using memory store", "error", err)
		auditStore = audit.NewMemoryAuditStore(1000)
	} else {
		auditStore = store
	}

	workspace := &Workspace{
		Config:     cfg,
		Quota:      quota,
		Status:     WorkspaceStatusActive,
		auditLog:   auditStore,
		sessions:   make(map[string]*Session),
		createdAt:  time.Now(),
		lastActive: time.Now(),
		logger:     wm.logger.With("workspace_id", cfg.ID, "workspace_name", cfg.Name),
	}

	wm.workspaces[cfg.ID] = workspace

	// Log workspace creation
	wm.logAudit(ctx, workspace, "workspace_created", "Workspace created", security.DangerLevelSafe)

	wm.logger.Info("Workspace created",
		"workspace_id", cfg.ID,
		"workspace_name", cfg.Name,
		"root_path", cfg.RootPath,
		"created_by", cfg.CreatedBy)

	return workspace, nil
}

// validateAndCreateWorkspaceDir validates and creates the workspace directory.
func (wm *workspaceManager) validateAndCreateWorkspaceDir(ctx context.Context, cfg *WorkspaceConfig) error {
	// Check if path tries to escape via ..
	cleanPath := filepath.Clean(cfg.RootPath)
	if cleanPath != cfg.RootPath {
		return errors.New("workspace: root path contains invalid components (../)")
	}

	// Check if it's within allowed base paths
	// Include common workspace directories
	homeDir := os.Getenv("HOME")
	basePaths := []string{
		filepath.Join(homeDir, ".hotplex", "workspaces"),
		"/tmp/hotplex",
		"/tmp", // Allow /tmp for testing and temporary workspaces
	}

	// Also add home directory itself for development
	if homeDir != "" {
		basePaths = append(basePaths, homeDir)
	}

	allowed := false
	for _, base := range basePaths {
		// Check exact prefix match or if base is a prefix of the path
		if strings.HasPrefix(cleanPath, base) || (base == "/tmp" && strings.HasPrefix(cleanPath, "/tmp/")) {
			allowed = true
			break
		}
	}

	// Also allow explicit base paths if configured
	if !allowed {
		// For now, only allow within base paths
		return fmt.Errorf("workspace: root path must be within allowed base directories, got: %s", cfg.RootPath)
	}

	// Create directory if not exists
	if err := os.MkdirAll(cleanPath, 0755); err != nil {
		return fmt.Errorf("workspace: failed to create directory: %w", err)
	}

	// Verify we have write permission
	if err := os.WriteFile(filepath.Join(cleanPath, ".workspace_marker"), []byte(cfg.ID), 0644); err != nil {
		return fmt.Errorf("workspace: no write permission: %w", err)
	}
	_ = os.Remove(filepath.Join(cleanPath, ".workspace_marker"))

	cfg.RootPath = cleanPath
	return nil
}

// GetWorkspace retrieves a workspace by ID.
func (wm *workspaceManager) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	ws, ok := wm.workspaces[workspaceID]
	return ws, ok
}

// DeleteWorkspace removes a workspace.
func (wm *workspaceManager) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace: not found")
	}

	// Check for active sessions
	if len(ws.sessions) > 0 {
		return errors.New("workspace: cannot delete workspace with active sessions")
	}

	ws.Status = WorkspaceStatusDeleted
	delete(wm.workspaces, workspaceID)

	// Log deletion
	wm.logAudit(ctx, ws, "workspace_deleted", "Workspace deleted", security.DangerLevelHigh)

	// Close audit store
	if ws.auditLog != nil {
		_ = ws.auditLog.Close()
	}

	wm.logger.Info("Workspace deleted",
		"workspace_id", workspaceID,
		"workspace_name", ws.Config.Name)

	return nil
}

// ListWorkspaces returns all workspaces.
func (wm *workspaceManager) ListWorkspaces(ctx context.Context) []*Workspace {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	list := make([]*Workspace, 0, len(wm.workspaces))
	for _, ws := range wm.workspaces {
		list = append(list, ws)
	}
	return list
}

// UpdateQuota updates the resource quota.
func (wm *workspaceManager) UpdateQuota(ctx context.Context, workspaceID string, quota ResourceQuota) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace: not found")
	}

	oldQuota := ws.Config.Quota
	ws.Config.Quota = quota

	wm.logAudit(ctx, ws, "quota_updated",
		fmt.Sprintf("Quota updated: memory %d->%d, cpu %d->%d",
			oldQuota.MemoryLimit, quota.MemoryLimit,
			oldQuota.CPUPercent, quota.CPUPercent),
		security.DangerLevelSafe)

	wm.logger.Info("Workspace quota updated",
		"workspace_id", workspaceID,
		"old_quota", oldQuota,
		"new_quota", quota)

	return nil
}

// GetUsage returns current resource usage.
func (wm *workspaceManager) GetUsage(ctx context.Context, workspaceID string) (WorkspaceUsage, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return WorkspaceUsage{}, errors.New("workspace: not found")
	}

	ws.mu.RLock()
	defer ws.mu.RUnlock()

	usage := WorkspaceUsage{
		ActiveSessions:  len(ws.sessions),
		ActiveProcesses: ws.Usage.ActiveProcesses,
		MemoryUsage:     ws.Usage.MemoryUsage,
		CPUPercent:      ws.Usage.CPUPercent,
		DiskUsage:       ws.Usage.DiskUsage,
		LastActivity:    ws.lastActive,
	}

	return usage, nil
}

// ValidatePath validates that a path is within workspace boundaries.
func (wm *workspaceManager) ValidatePath(ctx context.Context, workspaceID string, path string) (bool, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return false, errors.New("workspace: not found")
	}

	// Clean the path to resolve .. and .
	cleanPath := filepath.Clean(path)

	// Check if path is within workspace root
	if !strings.HasPrefix(cleanPath, ws.Config.RootPath) {
		wm.logAudit(ctx, ws, "path_traversal_attempt",
			fmt.Sprintf("Path traversal attempt: %s", path),
			security.DangerLevelHigh)
		return false, nil
	}

	// Check against allowed additional paths
	for _, allowed := range ws.Config.AllowedPaths {
		if strings.HasPrefix(cleanPath, filepath.Clean(allowed)) {
			return true, nil
		}
	}

	return true, nil
}

// AuditLog returns the audit store for a workspace.
func (wm *workspaceManager) AuditLog(ctx context.Context, workspaceID string) (security.AuditStore, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return nil, errors.New("workspace: not found")
	}

	return ws.auditLog, nil
}

// RegisterSession associates a session with a workspace.
func (wm *workspaceManager) RegisterSession(ctx context.Context, workspaceID string, session *Session) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace: not found")
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Check session limit
	if ws.Config.Quota.MaxSessions > 0 && len(ws.sessions) >= ws.Config.Quota.MaxSessions {
		return errors.New("workspace: session limit exceeded")
	}

	ws.sessions[session.ID] = session
	ws.lastActive = time.Now()
	ws.Status = WorkspaceStatusActive

	wm.logAudit(ctx, ws, "session_registered",
		fmt.Sprintf("Session registered: %s", session.ID),
		security.DangerLevelSafe)

	return nil
}

// UnregisterSession removes a session from a workspace.
func (wm *workspaceManager) UnregisterSession(ctx context.Context, workspaceID string, sessionID string) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	ws, ok := wm.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace: not found")
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	delete(ws.sessions, sessionID)

	if len(ws.sessions) == 0 {
		ws.Status = WorkspaceStatusIdle
	}

	wm.logAudit(ctx, ws, "session_unregistered",
		fmt.Sprintf("Session unregistered: %s", sessionID),
		security.DangerLevelSafe)

	return nil
}

// Shutdown gracefully stops the workspace manager.
func (wm *workspaceManager) Shutdown() {
	wm.shutdownOnce.Do(func() {
		close(wm.done)
	})

	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, ws := range wm.workspaces {
		if ws.auditLog != nil {
			_ = ws.auditLog.Close()
		}
	}

	wm.workspaces = make(map[string]*Workspace)
	wm.logger.Info("Workspace manager shutdown complete")
}

// logAudit logs an audit event for a workspace.
func (wm *workspaceManager) logAudit(ctx context.Context, ws *Workspace, operation, reason string, level security.DangerLevel) {
	if ws.auditLog == nil {
		return
	}

	event := &security.AuditEvent{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Operation: operation,
		Reason:    reason,
		Level:     level,
		Category:  "workspace",
		Action:    security.AuditActionApproved,
		Source:    "workspace_manager",
		Metadata: map[string]any{
			"workspace_id":   ws.Config.ID,
			"workspace_name": ws.Config.Name,
			"root_path":      ws.Config.RootPath,
		},
	}

	if err := ws.auditLog.Save(ctx, event); err != nil {
		wm.logger.Error("Failed to save audit event", "error", err)
	}
}
