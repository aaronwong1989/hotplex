package telemetry

import (
	"log/slog"
	"sync"
	"time"
)

// ========================================
// Security Metrics
// ========================================

// SecurityMetrics holds security-related metrics for observability.
type SecurityMetrics struct {
	logger *slog.Logger
	mu     sync.RWMutex

	// Threat Detection Metrics
	threatsDetected    int64
	threatsBlocked    int64
	threatsAllowed    int64

	// Danger Block Metrics
	dangersBlocked    int64
	bypassAttempts    int64
	bypassSuccess     int64

	// Attack Type Statistics (by category)
	attackTypes map[string]int64

	// Workspace Resource Metrics
	workspacesActive   int64
	workspaceOpsTotal  int64
	workspaceOpsDenied int64

	// Landlock Enforcement Metrics
	landlockAccessDenied   int64
	landlockPathViolations int64

	// Permission Metrics
	permissionDenied int64
	permissionGranted int64

	// AI Guard Metrics
	aiGuardBlocks    int64
	aiGuardAllows    int64
	aiGuardErrors    int64

	// Performance Metrics
	detectionLatency   time.Duration
	enforcementLatency time.Duration

	// Time window for rate limiting
	windowStart time.Time
	windowEvents int64
}

var (
	globalSecurityMetrics   *SecurityMetrics
	globalSecurityMetricsMu sync.Once
)

// NewSecurityMetrics creates a new security metrics instance.
func NewSecurityMetrics(logger *slog.Logger) *SecurityMetrics {
	if logger == nil {
		logger = slog.Default()
	}
	return &SecurityMetrics{
		logger:      logger,
		attackTypes: make(map[string]int64),
		windowStart: time.Now(),
	}
}

// Threat Detection Metrics

// IncThreatsDetected increments the threats detected counter.
func (m *SecurityMetrics) IncThreatsDetected() {
	m.mu.Lock()
	m.threatsDetected++
	m.incWindowEvent()
	m.mu.Unlock()
}

// IncThreatsBlocked increments the threats blocked counter.
func (m *SecurityMetrics) IncThreatsBlocked() {
	m.mu.Lock()
	m.threatsBlocked++
	m.incWindowEvent()
	m.mu.Unlock()
}

// IncThreatsAllowed increments the threats allowed counter.
func (m *SecurityMetrics) IncThreatsAllowed() {
	m.mu.Lock()
	m.threatsAllowed++
	m.mu.Unlock()
}

// Danger Block Metrics

// IncDangersBlocked increments the dangers blocked counter.
func (m *SecurityMetrics) IncDangersBlocked() {
	m.mu.Lock()
	m.dangersBlocked++
	m.incWindowEvent()
	m.mu.Unlock()
}

// IncBypassAttempts increments the bypass attempts counter.
func (m *SecurityMetrics) IncBypassAttempts() {
	m.mu.Lock()
	m.bypassAttempts++
	m.mu.Unlock()
}

// IncBypassSuccess increments the bypass success counter.
func (m *SecurityMetrics) IncBypassSuccess() {
	m.mu.Lock()
	m.bypassSuccess++
	m.mu.Unlock()
}

// Attack Type Statistics

// IncAttackType increments the counter for a specific attack type.
func (m *SecurityMetrics) IncAttackType(attackType string) {
	m.mu.Lock()
	m.attackTypes[attackType]++
	m.mu.Unlock()
}

// Workspace Resource Metrics

// IncWorkspacesActive increments the active workspaces counter.
func (m *SecurityMetrics) IncWorkspacesActive() {
	m.mu.Lock()
	m.workspacesActive++
	m.mu.Unlock()
}

// DecWorkspacesActive decrements the active workspaces counter.
func (m *SecurityMetrics) DecWorkspacesActive() {
	m.mu.Lock()
	if m.workspacesActive > 0 {
		m.workspacesActive--
	}
	m.mu.Unlock()
}

// IncWorkspaceOpsTotal increments total workspace operations.
func (m *SecurityMetrics) IncWorkspaceOpsTotal() {
	m.mu.Lock()
	m.workspaceOpsTotal++
	m.mu.Unlock()
}

// IncWorkspaceOpsDenied increments denied workspace operations.
func (m *SecurityMetrics) IncWorkspaceOpsDenied() {
	m.mu.Lock()
	m.workspaceOpsDenied++
	m.mu.Unlock()
}

// Landlock Enforcement Metrics

// IncLandlockAccessDenied increments Landlock access denied counter.
func (m *SecurityMetrics) IncLandlockAccessDenied() {
	m.mu.Lock()
	m.landlockAccessDenied++
	m.incWindowEvent()
	m.mu.Unlock()
}

// IncLandlockPathViolations increments Landlock path violations counter.
func (m *SecurityMetrics) IncLandlockPathViolations() {
	m.mu.Lock()
	m.landlockPathViolations++
	m.incWindowEvent()
	m.mu.Unlock()
}

// Permission Metrics

// IncPermissionDenied increments permission denied counter.
func (m *SecurityMetrics) IncPermissionDenied() {
	m.mu.Lock()
	m.permissionDenied++
	m.incWindowEvent()
	m.mu.Unlock()
}

// IncPermissionGranted increments permission granted counter.
func (m *SecurityMetrics) IncPermissionGranted() {
	m.mu.Lock()
	m.permissionGranted++
	m.mu.Unlock()
}

// AI Guard Metrics

// IncAiGuardBlocks increments AI Guard blocks counter.
func (m *SecurityMetrics) IncAiGuardBlocks() {
	m.mu.Lock()
	m.aiGuardBlocks++
	m.incWindowEvent()
	m.mu.Unlock()
}

// IncAiGuardAllows increments AI Guard allows counter.
func (m *SecurityMetrics) IncAiGuardAllows() {
	m.mu.Lock()
	m.aiGuardAllows++
	m.mu.Unlock()
}

// IncAiGuardErrors increments AI Guard errors counter.
func (m *SecurityMetrics) IncAiGuardErrors() {
	m.mu.Lock()
	m.aiGuardErrors++
	m.mu.Unlock()
}

// Performance Metrics

// RecordDetectionLatency records the detection latency.
func (m *SecurityMetrics) RecordDetectionLatency(d time.Duration) {
	m.mu.Lock()
	m.detectionLatency = d
	m.mu.Unlock()
}

// RecordEnforcementLatency records the enforcement latency.
func (m *SecurityMetrics) RecordEnforcementLatency(d time.Duration) {
	m.mu.Lock()
	m.enforcementLatency = d
	m.mu.Unlock()
}

// Window event management

func (m *SecurityMetrics) incWindowEvent() {
	m.windowEvents++
	// Reset window every minute
	if time.Since(m.windowStart) > time.Minute {
		m.windowEvents = 0
		m.windowStart = time.Now()
	}
}

// WindowEvents returns the number of events in the current window.
func (m *SecurityMetrics) WindowEvents() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.windowEvents
}

// WindowRate returns the events per second in the current window.
func (m *SecurityMetrics) WindowRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	elapsed := time.Since(m.windowStart)
	if elapsed == 0 {
		return 0
	}
	return float64(m.windowEvents) / elapsed.Seconds()
}

// SecurityMetricsSnapshot contains a point-in-time snapshot of security metrics.
type SecurityMetricsSnapshot struct {
	// Threat Detection
	ThreatsDetected int64
	ThreatsBlocked  int64
	ThreatsAllowed  int64

	// Danger Block
	DangersBlocked int64
	BypassAttempts int64
	BypassSuccess   int64

	// Attack Types
	AttackTypes map[string]int64

	// Workspace
	WorkspacesActive   int64
	WorkspaceOpsTotal  int64
	WorkspaceOpsDenied  int64

	// Landlock
	LandlockAccessDenied   int64
	LandlockPathViolations int64

	// Permission
	PermissionDenied  int64
	PermissionGranted int64

	// AI Guard
	AiGuardBlocks int64
	AiGuardAllows int64
	AiGuardErrors int64

	// Performance
	DetectionLatency   time.Duration
	EnforcementLatency time.Duration

	// Rate limiting
	WindowEvents int64
	WindowRate   float64
}

// Snapshot returns a point-in-time snapshot of the metrics.
func (m *SecurityMetrics) Snapshot() SecurityMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy attack types
	attackTypesCopy := make(map[string]int64, len(m.attackTypes))
	for k, v := range m.attackTypes {
		attackTypesCopy[k] = v
	}

	return SecurityMetricsSnapshot{
		ThreatsDetected:        m.threatsDetected,
		ThreatsBlocked:        m.threatsBlocked,
		ThreatsAllowed:         m.threatsAllowed,
		DangersBlocked:         m.dangersBlocked,
		BypassAttempts:        m.bypassAttempts,
		BypassSuccess:         m.bypassSuccess,
		AttackTypes:            attackTypesCopy,
		WorkspacesActive:      m.workspacesActive,
		WorkspaceOpsTotal:     m.workspaceOpsTotal,
		WorkspaceOpsDenied:    m.workspaceOpsDenied,
		LandlockAccessDenied:  m.landlockAccessDenied,
		LandlockPathViolations: m.landlockPathViolations,
		PermissionDenied:      m.permissionDenied,
		PermissionGranted:    m.permissionGranted,
		AiGuardBlocks:        m.aiGuardBlocks,
		AiGuardAllows:        m.aiGuardAllows,
		AiGuardErrors:        m.aiGuardErrors,
		DetectionLatency:      m.detectionLatency,
		EnforcementLatency:   m.enforcementLatency,
		WindowEvents:         m.windowEvents,
		WindowRate:           float64(m.windowEvents) / time.Since(m.windowStart).Seconds(),
	}
}

// Reset resets all metrics to zero.
func (m *SecurityMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.threatsDetected = 0
	m.threatsBlocked = 0
	m.threatsAllowed = 0
	m.dangersBlocked = 0
	m.bypassAttempts = 0
	m.bypassSuccess = 0
	m.attackTypes = make(map[string]int64)
	m.workspacesActive = 0
	m.workspaceOpsTotal = 0
	m.workspaceOpsDenied = 0
	m.landlockAccessDenied = 0
	m.landlockPathViolations = 0
	m.permissionDenied = 0
	m.permissionGranted = 0
	m.aiGuardBlocks = 0
	m.aiGuardAllows = 0
	m.aiGuardErrors = 0
	m.detectionLatency = 0
	m.enforcementLatency = 0
	m.windowStart = time.Now()
	m.windowEvents = 0
}

// InitSecurityMetrics initializes the global security metrics.
func InitSecurityMetrics(logger *slog.Logger) {
	globalSecurityMetricsMu.Do(func() {
		globalSecurityMetrics = NewSecurityMetrics(logger)
	})
}

// GetSecurityMetrics returns the global security metrics instance.
func GetSecurityMetrics() *SecurityMetrics {
	globalSecurityMetricsMu.Do(func() {
		if globalSecurityMetrics == nil {
			globalSecurityMetrics = NewSecurityMetrics(nil)
		}
	})
	return globalSecurityMetrics
}
