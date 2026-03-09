package telemetry

import (
	"testing"
	"time"
)

func TestSecurityMetrics_IncThreatsDetected(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncThreatsDetected()
	m.IncThreatsDetected()

	snapshot := m.Snapshot()
	if snapshot.ThreatsDetected != 2 {
		t.Errorf("Expected 2 threats detected, got %d", snapshot.ThreatsDetected)
	}
}

func TestSecurityMetrics_IncThreatsBlocked(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncThreatsBlocked()

	snapshot := m.Snapshot()
	if snapshot.ThreatsBlocked != 1 {
		t.Errorf("Expected 1 threat blocked, got %d", snapshot.ThreatsBlocked)
	}
}

func TestSecurityMetrics_IncDangersBlocked(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncDangersBlocked()
	m.IncDangersBlocked()
	m.IncDangersBlocked()

	snapshot := m.Snapshot()
	if snapshot.DangersBlocked != 3 {
		t.Errorf("Expected 3 dangers blocked, got %d", snapshot.DangersBlocked)
	}
}

func TestSecurityMetrics_IncAttackType(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncAttackType("command_injection")
	m.IncAttackType("command_injection")
	m.IncAttackType("path_traversal")

	snapshot := m.Snapshot()
	if snapshot.AttackTypes["command_injection"] != 2 {
		t.Errorf("Expected 2 command_injection, got %d", snapshot.AttackTypes["command_injection"])
	}
	if snapshot.AttackTypes["path_traversal"] != 1 {
		t.Errorf("Expected 1 path_traversal, got %d", snapshot.AttackTypes["path_traversal"])
	}
}

func TestSecurityMetrics_WorkspaceMetrics(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncWorkspacesActive()
	m.IncWorkspacesActive()
	m.IncWorkspaceOpsTotal()
	m.IncWorkspaceOpsTotal()
	m.IncWorkspaceOpsTotal()
	m.IncWorkspaceOpsDenied()

	snapshot := m.Snapshot()
	if snapshot.WorkspacesActive != 2 {
		t.Errorf("Expected 2 active workspaces, got %d", snapshot.WorkspacesActive)
	}
	if snapshot.WorkspaceOpsTotal != 3 {
		t.Errorf("Expected 3 total ops, got %d", snapshot.WorkspaceOpsTotal)
	}
	if snapshot.WorkspaceOpsDenied != 1 {
		t.Errorf("Expected 1 denied ops, got %d", snapshot.WorkspaceOpsDenied)
	}

	m.DecWorkspacesActive()
	snapshot = m.Snapshot()
	if snapshot.WorkspacesActive != 1 {
		t.Errorf("Expected 1 active workspace after decrement, got %d", snapshot.WorkspacesActive)
	}
}

func TestSecurityMetrics_LandlockMetrics(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncLandlockAccessDenied()
	m.IncLandlockPathViolations()
	m.IncLandlockPathViolations()

	snapshot := m.Snapshot()
	if snapshot.LandlockAccessDenied != 1 {
		t.Errorf("Expected 1 landlock access denied, got %d", snapshot.LandlockAccessDenied)
	}
	if snapshot.LandlockPathViolations != 2 {
		t.Errorf("Expected 2 path violations, got %d", snapshot.LandlockPathViolations)
	}
}

func TestSecurityMetrics_PermissionMetrics(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncPermissionDenied()
	m.IncPermissionDenied()
	m.IncPermissionGranted()

	snapshot := m.Snapshot()
	if snapshot.PermissionDenied != 2 {
		t.Errorf("Expected 2 permission denied, got %d", snapshot.PermissionDenied)
	}
	if snapshot.PermissionGranted != 1 {
		t.Errorf("Expected 1 permission granted, got %d", snapshot.PermissionGranted)
	}
}

func TestSecurityMetrics_AiGuardMetrics(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncAiGuardBlocks()
	m.IncAiGuardAllows()
	m.IncAiGuardErrors()

	snapshot := m.Snapshot()
	if snapshot.AiGuardBlocks != 1 {
		t.Errorf("Expected 1 AI Guard block, got %d", snapshot.AiGuardBlocks)
	}
	if snapshot.AiGuardAllows != 1 {
		t.Errorf("Expected 1 AI Guard allow, got %d", snapshot.AiGuardAllows)
	}
	if snapshot.AiGuardErrors != 1 {
		t.Errorf("Expected 1 AI Guard error, got %d", snapshot.AiGuardErrors)
	}
}

func TestSecurityMetrics_PerformanceMetrics(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.RecordDetectionLatency(100 * time.Millisecond)
	m.RecordEnforcementLatency(50 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.DetectionLatency != 100*time.Millisecond {
		t.Errorf("Expected 100ms detection latency, got %v", snapshot.DetectionLatency)
	}
	if snapshot.EnforcementLatency != 50*time.Millisecond {
		t.Errorf("Expected 50ms enforcement latency, got %v", snapshot.EnforcementLatency)
	}
}

func TestSecurityMetrics_BypassMetrics(t *testing.T) {
	m := NewSecurityMetrics(nil)
	m.IncBypassAttempts()
	m.IncBypassAttempts()
	m.IncBypassSuccess()

	snapshot := m.Snapshot()
	if snapshot.BypassAttempts != 2 {
		t.Errorf("Expected 2 bypass attempts, got %d", snapshot.BypassAttempts)
	}
	if snapshot.BypassSuccess != 1 {
		t.Errorf("Expected 1 bypass success, got %d", snapshot.BypassSuccess)
	}
}

func TestSecurityMetrics_Snapshot(t *testing.T) {
	m := NewSecurityMetrics(nil)
	
	// Modify metrics
	m.IncThreatsDetected()
	m.IncThreatsBlocked()
	m.IncDangersBlocked()
	m.IncAttackType("test_attack")

	// Take snapshot
	snapshot1 := m.Snapshot()
	
	// Modify again
	m.IncThreatsDetected()
	
	// Take another snapshot
	snapshot2 := m.Snapshot()

	// Verify snapshot1 is a point-in-time copy
	if snapshot1.ThreatsDetected != 1 {
		t.Errorf("Snapshot 1: expected 1 threat detected, got %d", snapshot1.ThreatsDetected)
	}
	
	// Verify snapshot2 reflects new changes
	if snapshot2.ThreatsDetected != 2 {
		t.Errorf("Snapshot 2: expected 2 threats detected, got %d", snapshot2.ThreatsDetected)
	}
	
	// Verify deep copy for attack types
	if snapshot1.AttackTypes["test_attack"] != 1 {
		t.Errorf("Snapshot 1: expected attack type count 1, got %d", snapshot1.AttackTypes["test_attack"])
	}
}

func TestSecurityMetrics_Reset(t *testing.T) {
	m := NewSecurityMetrics(nil)
	
	m.IncThreatsDetected()
	m.IncDangersBlocked()
	m.IncAttackType("test")
	
	m.Reset()
	
	snapshot := m.Snapshot()
	if snapshot.ThreatsDetected != 0 {
		t.Errorf("Expected 0 after reset, got %d", snapshot.ThreatsDetected)
	}
	if snapshot.DangersBlocked != 0 {
		t.Errorf("Expected 0 after reset, got %d", snapshot.DangersBlocked)
	}
	if len(snapshot.AttackTypes) != 0 {
		t.Errorf("Expected empty attack types after reset, got %v", snapshot.AttackTypes)
	}
}

func TestSecurityMetrics_WindowEvents(t *testing.T) {
	m := NewSecurityMetrics(nil)
	
	// Trigger window events
	m.IncThreatsDetected()
	m.IncThreatsBlocked()
	m.IncDangersBlocked()
	
	events := m.WindowEvents()
	if events != 3 {
		t.Errorf("Expected 3 window events, got %d", events)
	}
}

func TestSecurityMetrics_GlobalInstance(t *testing.T) {
	// Test that GetSecurityMetrics returns a valid instance
	metrics := GetSecurityMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil global metrics")
	}
	
	// Test that multiple calls return the same instance
	metrics2 := GetSecurityMetrics()
	if metrics != metrics2 {
		t.Error("Expected same instance from GetSecurityMetrics")
	}
}
