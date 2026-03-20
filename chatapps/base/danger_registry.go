package base

import (
	"sync"
	"time"
)

// ContextEntry wraps a stored value with its creation timestamp for TTL-based cleanup.
type ContextEntry struct {
	Value     string
	CreatedAt time.Time
}

// DangerApprovalRegistry manages pending danger block approvals.
// Used by chatapps to block on WAF interception until user approves/denies via Slack buttons.
type DangerApprovalRegistry struct {
	pending sync.Map // sessionID → chan bool
}

// GlobalDangerRegistry is the singleton registry for danger block approvals.
var GlobalDangerRegistry = &DangerApprovalRegistry{}

// Register creates a pending approval channel for the given sessionID.
// Returns the channel to block on. The caller should select on ctx.Done() and this channel.
func (r *DangerApprovalRegistry) Register(sessionID string) chan bool {
	ch := make(chan bool, 1)
	r.pending.Store(sessionID, ch)
	return ch
}

// Resolve resolves a pending approval for the given sessionID.
// Returns true if the sessionID was found and resolved, false otherwise.
func (r *DangerApprovalRegistry) Resolve(sessionID string, approved bool) bool {
	if val, ok := r.pending.LoadAndDelete(sessionID); ok {
		ch := val.(chan bool)
		ch <- approved
		return true
	}
	return false
}

// PermissionCardData holds the shared data for building permission cards.
// Defined in base so both Slack and Feishu adapters can use it.
type PermissionCardData struct {
	BotID, SessionID, MessageID, UserID, Tool, Command string
}

// PermissionApprovalRegistry manages pending permission approvals for the new
// permission card flow (Allow Once / Deny Once — no persistence).
// Mirrors DangerApprovalRegistry but for permission-specific decisions.
type PermissionApprovalRegistry struct {
	pending sync.Map // sessionID → chan PermissionDecision
}

// PermissionDecision represents the user's permission decision.
type PermissionDecision struct {
	Allow  bool
	Reason string // "allow_once", "allow_always", "deny_once", "deny_all"
}

// GlobalPermissionRegistry is the singleton registry for permission approvals.
var GlobalPermissionRegistry = &PermissionApprovalRegistry{}

// GlobalPermissionContext stores pending tool+command for Slack permission card callbacks.
// Key: actionID (perm_{action}:{sessionID}:{messageID}), Value: ContextEntry{Value, CreatedAt}
// Entries expire after contextTTL and are cleaned up by the background goroutine.
var GlobalPermissionContext sync.Map

// contextTTL is the time-to-live for permission context entries.
const contextTTL = 10 * time.Minute

// contextCleanupInterval is how often the cleanup goroutine runs.
const contextCleanupInterval = 2 * time.Minute

func init() {
	// Start background cleanup goroutine for orphaned context entries.
	// This prevents memory leaks when users abandon sessions without clicking a button.
	go func() {
		ticker := time.NewTicker(contextCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			GlobalPermissionContext.Range(func(key, value any) bool {
				if entry, ok := value.(ContextEntry); ok {
					if now.Sub(entry.CreatedAt) > contextTTL {
						GlobalPermissionContext.Delete(key)
					}
				}
				return true
			})
		}
	}()
}

// RegisterPermission creates a pending permission channel for the given sessionID.
func (r *PermissionApprovalRegistry) RegisterPermission(sessionID string) chan PermissionDecision {
	ch := make(chan PermissionDecision, 1)
	r.pending.Store(sessionID, ch)
	return ch
}

// ResolvePermission resolves a pending permission for the given sessionID.
func (r *PermissionApprovalRegistry) ResolvePermission(sessionID string, decision PermissionDecision) bool {
	if val, ok := r.pending.LoadAndDelete(sessionID); ok {
		ch := val.(chan PermissionDecision)
		ch <- decision
		return true
	}
	return false
}

// CancelPermission removes a pending permission without resolving it.
func (r *PermissionApprovalRegistry) CancelPermission(sessionID string) {
	r.pending.Delete(sessionID)
}

// StorePermissionContext stores context for an actionID with a creation timestamp.
func StorePermissionContext(actionID, toolCommand string) {
	GlobalPermissionContext.Store(actionID, ContextEntry{
		Value:     toolCommand,
		CreatedAt: time.Now(),
	})
}

// LoadPermissionContext retrieves and deletes context for an actionID.
// Returns empty string if not found.
func LoadPermissionContext(actionID string) (string, bool) {
	val, ok := GlobalPermissionContext.LoadAndDelete(actionID)
	if !ok {
		return "", false
	}
	if entry, ok := val.(ContextEntry); ok {
		return entry.Value, true
	}
	// Legacy: handle plain string (should not happen in new code)
	if s, ok := val.(string); ok {
		return s, true
	}
	return "", false
}

// Cancel removes a pending approval without resolving it (e.g. on context cancellation).
func (r *DangerApprovalRegistry) Cancel(sessionID string) {
	r.pending.Delete(sessionID)
}
