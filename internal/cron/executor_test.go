package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	intengine "github.com/hrygo/hotplex/internal/engine"
)

// mockSessionManager implements intengine.SessionManager for testing.
type mockSessionManager struct {
	session *intengine.Session
	err     error
	called  bool
}

func (m *mockSessionManager) GetOrCreateSession(_ context.Context, _ string, _ intengine.SessionConfig, _ string) (*intengine.Session, bool, error) {
	m.called = true
	if m.err != nil {
		return nil, false, m.err
	}
	return m.session, m.err == nil, nil
}

func (m *mockSessionManager) GetSession(_ string) (*intengine.Session, bool) {
	return nil, false
}

func (m *mockSessionManager) TerminateSession(_ string) error {
	return nil
}

func (m *mockSessionManager) ListActiveSessions() []*intengine.Session {
	return nil
}

func (m *mockSessionManager) Shutdown() {}

var _ intengine.SessionManager = (*mockSessionManager)(nil)

func TestNewExecutor(t *testing.T) {
	mgr := &mockSessionManager{}
	exec := NewExecutor(mgr)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
}

func TestExecutor_Execute_Success(t *testing.T) {
	mgr := &mockSessionManager{}
	exec := NewExecutor(mgr)

	result := exec.Execute(context.Background(), &ExecuteRequest{
		Job:       &CronJob{ID: "job-1", Prompt: "test"},
		SessionID: "cron-job-1",
		WorkDir:   "/tmp",
		Prompt:    "run tests",
	})

	if result.Error != nil {
		t.Errorf("Error = %v, want nil", result.Error)
	}
	if !mgr.called {
		t.Error("GetOrCreateSession was not called")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestExecutor_Execute_SessionError(t *testing.T) {
	mgr := &mockSessionManager{
		err: errors.New("session creation failed"),
	}
	exec := NewExecutor(mgr)

	result := exec.Execute(context.Background(), &ExecuteRequest{
		Job:       &CronJob{ID: "job-1", Prompt: "test"},
		SessionID: "cron-job-1",
		WorkDir:   "/tmp",
		Prompt:    "run tests",
	})

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive even on error")
	}
	if !mgr.called {
		t.Error("GetOrCreateSession was not called")
	}
}

func TestExecutor_Execute_ContextCanceled(t *testing.T) {
	mgr := &mockSessionManager{
		err: context.Canceled,
	}
	exec := NewExecutor(mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := exec.Execute(ctx, &ExecuteRequest{
		Job:       &CronJob{ID: "job-1", Prompt: "test"},
		SessionID: "cron-job-1",
		WorkDir:   "/tmp",
		Prompt:    "run tests",
	})

	if result.Error == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestExecutor_Execute_Duration(t *testing.T) {
	mgr := &mockSessionManager{}
	exec := NewExecutor(mgr)

	start := time.Now()
	result := exec.Execute(context.Background(), &ExecuteRequest{
		Job:       &CronJob{ID: "job-1", Prompt: "test"},
		SessionID: "cron-job-1",
		WorkDir:   "/tmp",
		Prompt:    "run tests",
	})
	elapsed := time.Since(start)

	// Duration should reflect actual execution time
	if result.Duration > elapsed+time.Second {
		t.Errorf("Duration = %v, should be <= elapsed %v", result.Duration, elapsed)
	}
}
