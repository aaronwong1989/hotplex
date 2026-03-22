package command

import (
	"errors"
	"testing"

	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/provider"
)

// TestDisconnectExecutor_CommandAndDescription tests basic getters
func TestDisconnectExecutor_CommandAndDescription(t *testing.T) {
	exec := NewDisconnectExecutor(nil)

	if cmd := exec.Command(); cmd != "/dc" {
		t.Errorf("Command() = %q, want /dc", cmd)
	}

	if desc := exec.Description(); desc == "" {
		t.Error("Description() should not be empty")
	}
}

// TestProgressEmitter_NewEmitter tests constructor
func TestProgressEmitter_NewEmitter(t *testing.T) {
	steps := []ProgressStep{
		{Name: "step1", Message: "First", Status: "pending"},
	}
	callback := func(eventType string, data any) error { return nil }

	emitter := NewProgressEmitter("/test", callback, steps)

	if emitter == nil {
		t.Fatal("NewProgressEmitter() returned nil")
	}
	gotSteps := emitter.GetSteps()
	if len(gotSteps) != 1 {
		t.Errorf("GetSteps() = %d steps, want 1", len(gotSteps))
	}
}

// TestProgressEmitter_UpdateStep tests step update
func TestProgressEmitter_UpdateStep(t *testing.T) {
	steps := []ProgressStep{
		{Name: "step1", Message: "First", Status: "pending"},
		{Name: "step2", Message: "Second", Status: "pending"},
	}
	callback := func(eventType string, data any) error { return nil }
	emitter := NewProgressEmitter("/test", callback, steps)

	// Valid update
	err := emitter.UpdateStep(0, "running", "In progress...")
	if err != nil {
		t.Errorf("UpdateStep() error: %v", err)
	}

	gotSteps := emitter.GetSteps()
	if gotSteps[0].Status != "running" {
		t.Errorf("Step status = %q, want running", gotSteps[0].Status)
	}

	// Invalid index
	err = emitter.UpdateStep(-1, "running", "")
	if err == nil {
		t.Error("UpdateStep(-1) should return error")
	}

	err = emitter.UpdateStep(10, "running", "")
	if err == nil {
		t.Error("UpdateStep(10) should return error")
	}
}

// TestProgressEmitter_StatusHelpers tests Running, Success, Error methods
func TestProgressEmitter_StatusHelpers(t *testing.T) {
	steps := []ProgressStep{
		{Name: "step1", Message: "", Status: "pending"},
	}
	callback := func(eventType string, data any) error { return nil }
	emitter := NewProgressEmitter("/test", callback, steps)

	// Running
	if err := emitter.Running(0); err != nil {
		t.Errorf("Running() error: %v", err)
	}
	if steps := emitter.GetSteps(); steps[0].Status != "running" {
		t.Errorf("After Running(), status = %q", steps[0].Status)
	}

	// Success
	if err := emitter.Success(0, "Done"); err != nil {
		t.Errorf("Success() error: %v", err)
	}
	if steps := emitter.GetSteps(); steps[0].Status != "success" {
		t.Errorf("After Success(), status = %q", steps[0].Status)
	}

	// Error
	if err := emitter.Error(0, "Failed"); err != nil {
		t.Errorf("Error() error: %v", err)
	}
	if steps := emitter.GetSteps(); steps[0].Status != "error" {
		t.Errorf("After Error(), status = %q", steps[0].Status)
	}
}

// TestProgressEmitter_Complete tests completion
func TestProgressEmitter_Complete(t *testing.T) {
	steps := []ProgressStep{
		{Name: "step1", Message: "", Status: "running"},
		{Name: "step2", Message: "", Status: "pending"},
	}
	var lastEventType string
	callback := func(eventType string, data any) error {
		lastEventType = eventType
		return nil
	}
	emitter := NewProgressEmitter("/test", callback, steps)

	err := emitter.Complete("All done")
	if err != nil {
		t.Errorf("Complete() error: %v", err)
	}

	if lastEventType != string(provider.EventTypeCommandComplete) {
		t.Errorf("Last event = %q, want %q", lastEventType, provider.EventTypeCommandComplete)
	}

	// Verify all steps marked as success
	gotSteps := emitter.GetSteps()
	for i, s := range gotSteps {
		if s.Status != "success" {
			t.Errorf("Step %d status = %q, want success", i, s.Status)
		}
	}
}

// TestProgressEmitter_GetSteps_Copy tests that GetSteps returns a copy
func TestProgressEmitter_GetSteps_Copy(t *testing.T) {
	steps := []ProgressStep{
		{Name: "step1", Message: "Original", Status: "pending"},
	}
	callback := func(eventType string, data any) error { return nil }
	emitter := NewProgressEmitter("/test", callback, steps)

	// Get steps and modify
	gotSteps := emitter.GetSteps()
	gotSteps[0].Status = "modified"

	// Get again - should still have original value
	gotSteps2 := emitter.GetSteps()
	if gotSteps2[0].Status == "modified" {
		t.Error("GetSteps() should return a copy, not reference")
	}
}

// TestProgressEmitter_Callback tests callback invocation
func TestProgressEmitter_Callback(t *testing.T) {
	var callbackInvoked bool
	callback := func(eventType string, data any) error {
		callbackInvoked = true
		return nil
	}

	steps := []ProgressStep{{Name: "step1"}}
	emitter := NewProgressEmitter("/test", callback, steps)

	// Start should invoke callback
	err := emitter.Start("Starting")
	if err != nil {
		t.Errorf("Start() error: %v", err)
	}
	if !callbackInvoked {
		t.Error("Start() should invoke callback")
	}
}

// TestProgressEmitter_CallbackError tests error propagation
func TestProgressEmitter_CallbackError(t *testing.T) {
	expectedErr := errors.New("callback failed")
	callback := func(eventType string, data any) error {
		return expectedErr
	}

	steps := []ProgressStep{{Name: "step1"}}
	emitter := NewProgressEmitter("/test", callback, steps)

	err := emitter.Start("Starting")
	if err == nil {
		t.Error("Start() should return callback error")
	}
	if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
		t.Errorf("Start() error = %v, want %v", err, expectedErr)
	}
}

// Compile-time check for ProgressEmitter interface behavior
var _ event.Callback = event.Callback(nil)
