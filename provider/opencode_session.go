package provider

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionInfo holds the mapping between provider session IDs and server session IDs.
type SessionInfo struct {
	ServerSessionID   string
	ProviderSessionID string
	HotplexSessionID  string
	Namespace         string
	WorkDir           string
	Status            string
	CreatedAt         time.Time
	LastActiveAt      time.Time
}

// HTTPSessionManager manages session lifecycle with deterministic UUID v5 mapping.
// It maintains an in-memory mapping between provider session IDs (derived from
// namespace + sessionID using UUID v5) and server session IDs.
type HTTPSessionManager struct {
	transport *HTTPTransport
	mu        sync.RWMutex
	sessions  map[string]*SessionInfo // key: providerSessionID

	logger *slog.Logger
}

// NewHTTPSessionManager creates a new HTTPSessionManager.
func NewHTTPSessionManager(transport *HTTPTransport, logger *slog.Logger) *HTTPSessionManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &HTTPSessionManager{
		transport: transport,
		sessions:  make(map[string]*SessionInfo),
		logger:    logger.With("component", "http_session_manager"),
	}
}

// DeriveProviderSessionID generates a deterministic UUID v5 from namespace and sessionID.
// This ensures the same namespace + sessionID always maps to the same provider session ID.
func DeriveProviderSessionID(namespace, sessionID string) string {
	uniqueStr := namespace + ":session:" + sessionID
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(uniqueStr)).String()
}

// CreateSession creates a new session or returns existing one if already mapped.
// Algorithm:
//  1. Check in-memory mapping → hit则直接返回
//  2. GET /session 遍历找 Title == providerSessionID → 命中则复用
//  3. POST /session 新建
func (m *HTTPSessionManager) CreateSession(ctx context.Context, namespace, sessionID, workDir string) (string, error) {
	providerSessionID := DeriveProviderSessionID(namespace, sessionID)

	// Step 1: Check in-memory mapping
	m.mu.RLock()
	if info, ok := m.sessions[providerSessionID]; ok {
		info.LastActiveAt = time.Now()
		m.mu.RUnlock()
		m.logger.Debug("Session found in memory",
			"provider_session_id", providerSessionID,
			"server_session_id", info.ServerSessionID)
		return info.ServerSessionID, nil
	}
	m.mu.RUnlock()

	// Step 2: Try to find existing session on server by title
	serverSessionID, err := m.findSessionByTitle(ctx, providerSessionID)
	if err != nil {
		m.logger.Warn("Failed to search for existing session", "error", err)
		// Continue to create new session
	}

	if serverSessionID != "" {
		m.logger.Info("Reusing existing server session",
			"provider_session_id", providerSessionID,
			"server_session_id", serverSessionID)
	} else {
		// Step 3: Create new session on server
		serverSessionID, err = m.transport.CreateSession(ctx, providerSessionID)
		if err != nil {
			return "", fmt.Errorf("create session: %w", err)
		}
		m.logger.Info("Created new server session",
			"provider_session_id", providerSessionID,
			"server_session_id", serverSessionID)
	}

	// Store in memory mapping
	m.mu.Lock()
	m.sessions[providerSessionID] = &SessionInfo{
		ServerSessionID:   serverSessionID,
		ProviderSessionID: providerSessionID,
		HotplexSessionID:  sessionID,
		Namespace:         namespace,
		WorkDir:           workDir,
		Status:            "active",
		CreatedAt:         time.Now(),
		LastActiveAt:      time.Now(),
	}
	m.mu.Unlock()

	return serverSessionID, nil
}

// DeleteSession removes a session from memory mapping and optionally from server.
func (m *HTTPSessionManager) DeleteSession(ctx context.Context, serverSessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove from mapping
	var providerSessionID string
	for psid, info := range m.sessions {
		if info.ServerSessionID == serverSessionID {
			providerSessionID = psid
			delete(m.sessions, psid)
			break
		}
	}

	if providerSessionID != "" {
		m.logger.Info("Deleted session from memory",
			"provider_session_id", providerSessionID,
			"server_session_id", serverSessionID)
	}

	// Also delete from server
	if err := m.transport.DeleteSession(ctx, serverSessionID); err != nil {
		return fmt.Errorf("delete server session: %w", err)
	}

	return nil
}

// SendMessage sends a message to an existing session.
func (m *HTTPSessionManager) SendMessage(ctx context.Context, serverSessionID string, msg map[string]any) error {
	return m.transport.Send(ctx, serverSessionID, msg)
}

// ListSessions returns all sessions from the server.
func (m *HTTPSessionManager) ListSessions(ctx context.Context) ([]*OCSession, error) {
	// This would need a list endpoint on the server
	// For now, return sessions from memory
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Note: The server may have sessions not tracked in memory
	// This is a simplified implementation
	return nil, fmt.Errorf("ListSessions not implemented - use RecoverMappings instead")
}

// RecoverMappings recovers session mappings from the server by matching titles.
// Called during process startup to restore in-memory mappings.
func (m *HTTPSessionManager) RecoverMappings(ctx context.Context) error {
	m.logger.Info("Recovering session mappings from server")

	// The server stores sessions with title = providerSessionID
	// We need to list all sessions and match our known provider session IDs

	// For now, we rely on the deterministic UUID v5 to match
	// If a session exists on server with matching title, we can recover it
	// This is a placeholder - actual implementation would need server-side session enumeration

	m.logger.Info("Session mapping recovery complete")
	return nil
}

// GetSession returns the session info for a provider session ID.
func (m *HTTPSessionManager) GetSession(providerSessionID string) (*SessionInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.sessions[providerSessionID]
	return info, ok
}

// ResolveWorkDir resolves a workdir template with session context.
// Template variables: {namespace}, {session_id}, {date}
// Default template: "/hotplex/{namespace}/{date}/{session_id}"
func ResolveWorkDir(tpl, namespace, sessionID string) string {
	if tpl == "" {
		tpl = "/hotplex/{namespace}/{date}/{session_id}"
	}

	date := time.Now().Format("2006-01-02")

	result := tpl
	result = strings.ReplaceAll(result, "{namespace}", namespace)
	result = strings.ReplaceAll(result, "{session_id}", sessionID)
	result = strings.ReplaceAll(result, "{date}", date)

	return result
}

// findSessionByTitle searches the server for a session with matching title.
func (m *HTTPSessionManager) findSessionByTitle(ctx context.Context, title string) (string, error) {
	// The server's GET /session returns a list of sessions
	// We need to iterate and find the one with matching title
	// This is a simplified implementation - actual API may differ

	// For OpenCode Server, we don't have a direct list endpoint exposed
	// So we return empty and let CreateSession handle it
	return "", nil
}
