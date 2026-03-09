package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hrygo/hotplex/internal/security"
)

// WorkspaceAuditFilter extends AuditFilter with workspace-specific fields.
type WorkspaceAuditFilter struct {
	security.AuditFilter
	WorkspaceID string // Filter by workspace ID
}

// NewWorkspaceAuditStore creates a new workspace-scoped audit store.
type WorkspaceAuditStore struct {
	mu            sync.RWMutex
	workspaceID   string
	auditDir      string
	stores        map[string]security.AuditStore // sessionID -> audit store
	defaultStore  security.AuditStore
}

// NewWorkspaceAuditStore creates a new workspace audit store manager.
// It manages separate audit stores per session within a workspace.
func NewWorkspaceAuditStore(workspaceID, auditDir string) (*WorkspaceAuditStore, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace audit: workspace ID is required")
	}

	if auditDir == "" {
		auditDir = filepath.Join(os.Getenv("HOME"), ".hotplex", "audit", "workspaces", workspaceID)
	}

	ws := &WorkspaceAuditStore{
		workspaceID:  workspaceID,
		auditDir:     auditDir,
		stores:       make(map[string]security.AuditStore),
	}

	// Create default store for workspace-level events
	defaultStore, err := NewFileAuditStore(filepath.Join(auditDir, "workspace.jsonl"))
	if err != nil {
		// Fall back to memory store
		ws.defaultStore = NewMemoryAuditStore(1000)
	} else {
		ws.defaultStore = defaultStore
	}

	return ws, nil
}

// GetSessionStore returns or creates an audit store for a specific session.
func (ws *WorkspaceAuditStore) GetSessionStore(sessionID string) (security.AuditStore, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("workspace audit: session ID is required")
	}

	ws.mu.RLock()
	store, ok := ws.stores[sessionID]
	ws.mu.RUnlock()

	if ok {
		return store, nil
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Double check after acquiring write lock
	if store, ok := ws.stores[sessionID]; ok {
		return store, nil
	}

	// Create new session store
	sessionPath := filepath.Join(ws.auditDir, "sessions", sessionID+".jsonl")
	store, err := NewFileAuditStore(sessionPath)
	if err != nil {
		// Fall back to memory store
		store = NewMemoryAuditStore(500)
	}

	ws.stores[sessionID] = store
	return store, nil
}

// DefaultStore returns the workspace-level default audit store.
func (ws *WorkspaceAuditStore) DefaultStore() security.AuditStore {
	return ws.defaultStore
}

// Save saves an audit event to the workspace default store.
func (ws *WorkspaceAuditStore) Save(ctx context.Context, event *security.AuditEvent) error {
	if event == nil {
		return fmt.Errorf("workspace audit: event cannot be nil")
	}

	// Add workspace ID to metadata if not present
	if event.Metadata == nil {
		event.Metadata = make(map[string]any)
	}
	event.Metadata["workspace_id"] = ws.workspaceID

	return ws.defaultStore.Save(ctx, event)
}

// SaveSessionEvent saves an event to a session-specific store.
func (ws *WorkspaceAuditStore) SaveSessionEvent(ctx context.Context, sessionID string, event *security.AuditEvent) error {
	store, err := ws.GetSessionStore(sessionID)
	if err != nil {
		return err
	}

	if event.Metadata == nil {
		event.Metadata = make(map[string]any)
	}
	event.Metadata["workspace_id"] = ws.workspaceID

	return store.Save(ctx, event)
}

// Query retrieves audit events from the default store.
func (ws *WorkspaceAuditStore) Query(ctx context.Context, filter security.AuditFilter) ([]security.AuditEvent, error) {
	return ws.defaultStore.Query(ctx, filter)
}

// QuerySession retrieves audit events for a specific session.
func (ws *WorkspaceAuditStore) QuerySession(ctx context.Context, sessionID string, filter security.AuditFilter) ([]security.AuditEvent, error) {
	store, err := ws.GetSessionStore(sessionID)
	if err != nil {
		return nil, err
	}
	return store.Query(ctx, filter)
}

// QueryAllSessions retrieves audit events from all sessions in the workspace.
func (ws *WorkspaceAuditStore) QueryAllSessions(ctx context.Context, filter security.AuditFilter, limit int) ([]security.AuditEvent, error) {
	ws.mu.RLock()
	sessionIDs := make([]string, 0, len(ws.stores))
	for id := range ws.stores {
		sessionIDs = append(sessionIDs, id)
	}
	ws.mu.RUnlock()

	var allEvents []security.AuditEvent
	remaining := limit

	for _, sessionID := range sessionIDs {
		if remaining <= 0 {
			break
		}

		store, err := ws.GetSessionStore(sessionID)
		if err != nil {
			continue
		}

		filterCopy := filter
		if remaining > 0 {
			filterCopy.Limit = remaining
		}

		events, err := store.Query(ctx, filterCopy)
		if err != nil {
			continue
		}

		allEvents = append(allEvents, events...)
		remaining -= len(events)
	}

	return allEvents, nil
}

// Stats returns aggregated statistics for the workspace.
func (ws *WorkspaceAuditStore) Stats(ctx context.Context) (security.AuditStats, error) {
	return ws.defaultStore.Stats(ctx)
}

// Close closes all session stores and the default store.
func (ws *WorkspaceAuditStore) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	var lastErr error

	for _, store := range ws.stores {
		if err := store.Close(); err != nil {
			lastErr = err
		}
	}
	ws.stores = make(map[string]security.AuditStore)

	if ws.defaultStore != nil {
		if err := ws.defaultStore.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
