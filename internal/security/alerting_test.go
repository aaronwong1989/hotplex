package security

import (
	"context"
	"testing"
	"time"
)

func TestAlert_SeverityOrdering(t *testing.T) {
	tests := []struct {
		name     string
		severity AlertSeverity
		minSev   AlertSeverity
		expected bool
	}{
		{"info meets info", AlertSeverityInfo, AlertSeverityInfo, true},
		{"warning meets warning", AlertSeverityWarning, AlertSeverityWarning, true},
		{"warning meets info", AlertSeverityWarning, AlertSeverityInfo, true},
		{"critical meets warning", AlertSeverityCritical, AlertSeverityWarning, true},
		{"info does not meet warning", AlertSeverityInfo, AlertSeverityWarning, false},
		{"warning does not meet critical", AlertSeverityWarning, AlertSeverityCritical, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := &Alert{Severity: tt.severity}
			result := alert.meetsMinSeverity(tt.minSev)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAlert_Resolve(t *testing.T) {
	alert := &Alert{
		ID:        "test-alert",
		Resolved:  false,
		ResolvedAt: nil,
	}

	alert.Resolve()

	if !alert.Resolved {
		t.Error("Expected alert to be resolved")
	}
	if alert.ResolvedAt == nil {
		t.Error("Expected resolved_at to be set")
	}
}

func TestAlert_Fingerprint(t *testing.T) {
	alert1 := &Alert{
		Category: AlertCategoryThreatDetection,
		Title:    "Test Alert",
		Source:   "test",
	}

	alert2 := &Alert{
		Category: AlertCategoryThreatDetection,
		Title:    "Test Alert",
		Source:   "test",
	}

	alert3 := &Alert{
		Category: AlertCategoryDangerBlock,
		Title:    "Test Alert",
		Source:   "test",
	}

	fp1 := alert1.fingerprint()
	fp2 := alert2.fingerprint()
	fp3 := alert3.fingerprint()

	if fp1 != fp2 {
		t.Errorf("Expected same fingerprint for identical alerts")
	}
	if fp1 == fp3 {
		t.Errorf("Expected different fingerprint for different categories")
	}
}

func TestNewThreatDetectedAlert(t *testing.T) {
	detection := &ThreatDetectionEvent{
		InputType: "shell_command",
		Category:  "command_injection",
		Score:     0.9,
		Blocked:   true,
		Verdict:   "malicious",
		SessionID: "session-123",
	}

	alert := NewThreatDetectedAlert(detection)

	if alert.Category != AlertCategoryThreatDetection {
		t.Errorf("Expected category %s, got %s", AlertCategoryThreatDetection, alert.Category)
	}
	if alert.Severity != AlertSeverityCritical {
		t.Errorf("Expected severity %s for high score, got %s", AlertSeverityCritical, alert.Severity)
	}
	if alert.SessionID != "session-123" {
		t.Errorf("Expected session ID, got %s", alert.SessionID)
	}
}

func TestNewThreatDetectedAlert_LowScore(t *testing.T) {
	detection := &ThreatDetectionEvent{
		Score:   0.3,
		Blocked: false,
	}

	alert := NewThreatDetectedAlert(detection)

	if alert.Severity != AlertSeverityWarning {
		t.Errorf("Expected severity %s for low score, got %s", AlertSeverityWarning, alert.Severity)
	}
}

func TestNewDangerBlockAlert(t *testing.T) {
	detection := &DangerDetectionEvent{
		Operation:      "rm -rf /",
		Reason:         "Recursive delete of root",
		PatternMatched: "rm.*-rf.*/",
		Level:          0, // Critical
		Category:       "destructive",
		BypassAllowed:  false,
		SessionID:      "session-456",
		UserID:         "user-789",
		WorkspaceID:    "ws-001",
	}

	alert := NewDangerBlockAlert(detection)

	if alert.Category != AlertCategoryDangerBlock {
		t.Errorf("Expected category %s, got %s", AlertCategoryDangerBlock, alert.Category)
	}
	if alert.Severity != AlertSeverityCritical {
		t.Errorf("Expected severity %s for critical level, got %s", AlertSeverityCritical, alert.Severity)
	}
	if alert.SessionID != "session-456" {
		t.Errorf("Expected session ID, got %s", alert.SessionID)
	}
	if alert.UserID != "user-789" {
		t.Errorf("Expected user ID, got %s", alert.UserID)
	}
}

func TestNewDangerBlockAlert_ModerateLevel(t *testing.T) {
	detection := &DangerDetectionEvent{
		Level: 2, // Moderate
	}

	alert := NewDangerBlockAlert(detection)

	if alert.Severity != AlertSeverityInfo {
		t.Errorf("Expected severity %s for moderate level, got %s", AlertSeverityInfo, alert.Severity)
	}
}

func TestNewBypassAttemptAlert(t *testing.T) {
	event := &BypassAttemptEvent{
		TargetRule:  "delete_rule",
		Success:     false,
		AttemptedBy: "user-123",
		SessionID:   "session-999",
	}

	alert := NewBypassAttemptAlert(event)

	if alert.Category != AlertCategoryBypassAttempt {
		t.Errorf("Expected category %s, got %s", AlertCategoryBypassAttempt, alert.Category)
	}
	if alert.Severity != AlertSeverityWarning {
		t.Errorf("Expected severity %s for failed bypass, got %s", AlertSeverityWarning, alert.Severity)
	}
}

func TestNewBypassAttemptAlert_Success(t *testing.T) {
	event := &BypassAttemptEvent{
		Success: true,
	}

	alert := NewBypassAttemptAlert(event)

	if alert.Severity != AlertSeverityCritical {
		t.Errorf("Expected severity %s for successful bypass, got %s", AlertSeverityCritical, alert.Severity)
	}
}

func TestNewAnomalyAlert(t *testing.T) {
	metadata := map[string]any{
		"metric": "request_rate",
		"value":  1000,
	}

	alert := NewAnomalyAlert("high_request_rate", "Request rate exceeded threshold", metadata)

	if alert.Category != AlertCategoryAnomaly {
		t.Errorf("Expected category %s, got %s", AlertCategoryAnomaly, alert.Category)
	}
	if alert.Title != "Anomaly Detected" {
		t.Errorf("Expected title 'Anomaly Detected', got %s", alert.Title)
	}
	if alert.Metadata["metric"] != "request_rate" {
		t.Errorf("Expected metadata to be preserved")
	}
}

func TestNewPermissionDeniedAlert(t *testing.T) {
	event := &PermissionDeniedEvent{
		Resource:   "file",
		Operation: "write",
		Reason:    "readonly_filesystem",
		SessionID: "session-111",
		UserID:    "user-222",
	}

	alert := NewPermissionDeniedAlert(event)

	if alert.Category != AlertCategoryPermission {
		t.Errorf("Expected category %s, got %s", AlertCategoryPermission, alert.Category)
	}
	if alert.Severity != AlertSeverityInfo {
		t.Errorf("Expected severity %s, got %s", AlertSeverityInfo, alert.Severity)
	}
}

func TestNewWorkspaceAccessDeniedAlert(t *testing.T) {
	event := &WorkspaceAccessEvent{
		WorkspaceID: "ws-001",
		Operation:  "read",
		Path:       "/other-workspace/file.txt",
		Allowed:    false,
		SessionID:  "session-333",
		UserID:     "user-444",
	}

	alert := NewWorkspaceAccessDeniedAlert(event)

	if alert.Category != AlertCategoryWorkspace {
		t.Errorf("Expected category %s, got %s", AlertCategoryWorkspace, alert.Category)
	}
	if alert.WorkspaceID != "ws-001" {
		t.Errorf("Expected workspace ID, got %s", alert.WorkspaceID)
	}
}

func TestNewLandlockViolationAlert(t *testing.T) {
	event := &LandlockEvent{
		EventType:  LandlockEventAccessDenied,
		Operation:  "write",
		Path:       "/forbidden/file.txt",
		Allowed:    false,
		AccessMask: []string{"write"},
	}

	alert := NewLandlockViolationAlert(event)

	if alert.Category != AlertCategoryLandlock {
		t.Errorf("Expected category %s, got %s", AlertCategoryLandlock, alert.Category)
	}
}

func TestAlertingEngine_Disabled(t *testing.T) {
	config := DefaultAlertConfig()
	config.Enabled = false

	engine := NewAlertingEngine(config)

	alert := &Alert{
		Title:   "Test Alert",
		Message: "This should be dropped",
	}

	engine.SendAlert(alert)
	// Should not panic, just drop the alert
}

func TestAlertingEngine_MinSeverity(t *testing.T) {
	config := DefaultAlertConfig()
	config.Enabled = true
	config.MinSeverity = AlertSeverityCritical
	config.BufferSize = 10
	config.Workers = 1

	engine := NewAlertingEngine(config)
	engine.Start(testContext(t))
	defer engine.Stop()

	// This should be filtered out
	alert := &Alert{
		Severity: AlertSeverityInfo,
		Title:    "Low Severity",
	}
	engine.SendAlert(alert)

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Check no alerts
	alerts := engine.GetActiveAlerts()
	if len(alerts) > 0 {
		t.Errorf("Expected no alerts (filtered by severity), got %d", len(alerts))
	}
}

func TestAlertingEngine_GetAlerts(t *testing.T) {
	config := DefaultAlertConfig()
	config.Enabled = false // Disable to avoid webhook calls

	engine := NewAlertingEngine(config)

	// Add some test alerts
	alert1 := &Alert{ID: "alert-1", Category: AlertCategoryThreatDetection, Severity: AlertSeverityCritical, Resolved: false}
	alert2 := &Alert{ID: "alert-2", Category: AlertCategoryThreatDetection, Severity: AlertSeverityWarning, Resolved: false}
	alert3 := &Alert{ID: "alert-3", Category: AlertCategoryDangerBlock, Severity: AlertSeverityCritical, Resolved: true}

	// Manually add to history for testing
	engine.mu.Lock()
	engine.history[alert1.fingerprint()] = alert1
	engine.history[alert2.fingerprint()] = alert2
	engine.history[alert3.fingerprint()] = alert3
	engine.mu.Unlock()

	// Test category filter
	filtered := engine.GetAlerts(AlertFilter{Category: AlertCategoryThreatDetection})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 alerts with category threat_detection, got %d", len(filtered))
	}

	// Test severity filter
	filtered = engine.GetAlerts(AlertFilter{Severity: AlertSeverityCritical})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 critical alerts, got %d", len(filtered))
	}

	// Test resolved filter
	filtered = engine.GetAlerts(AlertFilter{Resolved: false})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 unresolved alerts, got %d", len(filtered))
	}
}

func TestAlertingEngine_GetActiveAlerts(t *testing.T) {
	config := DefaultAlertConfig()
	config.Enabled = false

	engine := NewAlertingEngine(config)

	alert1 := &Alert{ID: "alert-1", Resolved: false}
	alert2 := &Alert{ID: "alert-2", Resolved: true}
	alert3 := &Alert{ID: "alert-3", Resolved: false}

	engine.mu.Lock()
	engine.history[alert1.fingerprint()] = alert1
	engine.history[alert2.fingerprint()] = alert2
	engine.history[alert3.fingerprint()] = alert3
	engine.mu.Unlock()

	active := engine.GetActiveAlerts()
	if len(active) != 2 {
		t.Errorf("Expected 2 active alerts, got %d", len(active))
	}
}

func TestAlertingEngine_GetCriticalAlerts(t *testing.T) {
	config := DefaultAlertConfig()
	config.Enabled = false

	engine := NewAlertingEngine(config)

	alert1 := &Alert{ID: "alert-1", Severity: AlertSeverityCritical, Resolved: false}
	alert2 := &Alert{ID: "alert-2", Severity: AlertSeverityCritical, Resolved: true}
	alert3 := &Alert{ID: "alert-3", Severity: AlertSeverityWarning, Resolved: false}

	engine.mu.Lock()
	engine.history[alert1.fingerprint()] = alert1
	engine.history[alert2.fingerprint()] = alert2
	engine.history[alert3.fingerprint()] = alert3
	engine.mu.Unlock()

	critical := engine.GetCriticalAlerts()
	if len(critical) != 1 {
		t.Errorf("Expected 1 critical alert, got %d", len(critical))
	}
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
