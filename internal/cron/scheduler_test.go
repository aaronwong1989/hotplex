package cron

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	intengine "github.com/hrygo/hotplex/internal/engine"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockSessionManager2 implements intengine.SessionManager for testing (always succeeds).
type mockSessionManager2 struct {
	intengine.SessionManager
}

func (m *mockSessionManager2) GetOrCreateSession(_ context.Context, _ string, _ intengine.SessionConfig, _ string) (*intengine.Session, bool, error) {
	return nil, true, nil
}

var _ intengine.SessionManager = (*mockSessionManager2)(nil)

// retryCountingManager wraps SessionManager to track retry counts.
type retryCountingManager struct {
	intengine.SessionManager
	fn func() error
}

func (r *retryCountingManager) GetOrCreateSession(_ context.Context, _ string, _ intengine.SessionConfig, _ string) (*intengine.Session, bool, error) {
	if r.fn != nil {
		if err := r.fn(); err != nil {
			return nil, false, err
		}
	}
	return nil, true, nil
}

var _ intengine.SessionManager = (*retryCountingManager)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestScheduler(t *testing.T) (*CronScheduler, *CronStore, *RunsStore) {
	t.Helper()
	dir := t.TempDir()
	cs, err := NewCronStore(dir)
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	rs, err := NewRunsStore(dir)
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}
	mgr := &mockSessionManager2{}
	exec := NewExecutor(mgr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)
	sched.SetRunsStore(rs)
	return sched, cs, rs
}

func makeSchedulerJob(id, cronExpr string, enabled bool) *CronJob {
	return &CronJob{
		ID:          id,
		CronExpr:    cronExpr,
		Prompt:      "test prompt",
		WorkDir:     "/tmp",
		Type:        JobTypeLight,
		TimeoutMins: 30,
		Retries:     3,
		Enabled:     enabled,
		CreatedBy:   "tester",
	}
}

// validCronExpr is a 6-field cron expression with seconds field (required by cron.WithSeconds()).
const validCronExpr = "0 * * * * *"

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewCronScheduler(t *testing.T) {
	tests := []struct {
		name          string
		logger        *slog.Logger
		maxConcurrent int
		wantMax       int
	}{
		{"nil logger defaults", nil, 0, 4},
		{"explicit max", slog.Default(), 8, 8},
		{"negative max defaults", slog.Default(), -1, 4},
		{"zero max defaults", slog.Default(), 0, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cs, _ := NewCronStore(dir)
			exec := NewExecutor(&mockSessionManager{})
			sched := NewCronScheduler(cs, exec, tt.logger, tt.maxConcurrent)
			if sched == nil {
				t.Fatal("NewCronScheduler returned nil")
			}
			if sched.maxConcurrent != tt.wantMax {
				t.Errorf("maxConcurrent = %d, want %d", sched.maxConcurrent, tt.wantMax)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Start / Stop
// ---------------------------------------------------------------------------

func TestCronScheduler_StartStop(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Starting again should be idempotent
	if err := sched.Start(); err != nil {
		t.Fatalf("Start (2nd): %v", err)
	}

	ctx := sched.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not complete in time")
	}

	// Stopping again should not panic
	ctx2 := sched.Stop()
	_ = ctx2
}

func TestCronScheduler_StartRegistersEnabledJobs(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	job := makeSchedulerJob("enabled-job", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	sched.mu.Lock()
	_, ok := sched.entryMap["enabled-job"]
	sched.mu.Unlock()

	if !ok {
		t.Error("enabled job should be registered in entryMap")
	}
}

func TestCronScheduler_StartSkipsDisabledJobs(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	job := makeSchedulerJob("disabled-job", validCronExpr, false)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	sched.mu.Lock()
	_, ok := sched.entryMap["disabled-job"]
	sched.mu.Unlock()

	if ok {
		t.Error("disabled job should NOT be registered in entryMap")
	}
}

// ---------------------------------------------------------------------------
// AddJob
// ---------------------------------------------------------------------------

func TestCronScheduler_AddJob(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	job := makeSchedulerJob("add-test", validCronExpr, true)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	got := sched.GetJob("add-test")
	if got == nil {
		t.Fatal("GetJob returned nil after AddJob")
	}

	sched.mu.Lock()
	_, ok := sched.entryMap["add-test"]
	sched.mu.Unlock()

	if !ok {
		t.Error("added job should be registered in entryMap")
	}
}

func TestCronScheduler_AddJob_Disabled(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	job := makeSchedulerJob("add-disabled", validCronExpr, false)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	got := sched.GetJob("add-disabled")
	if got == nil {
		t.Fatal("GetJob returned nil after AddJob")
	}

	sched.mu.Lock()
	_, ok := sched.entryMap["add-disabled"]
	sched.mu.Unlock()

	if ok {
		t.Error("disabled job should NOT be registered in entryMap")
	}
}

func TestCronScheduler_AddJob_InvalidCronExpr(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	job := makeSchedulerJob("bad-cron", "invalid-cron", true)
	err := sched.AddJob(job)
	if err == nil {
		t.Fatal("AddJob should return error for invalid cron expression")
	}
	if !strings.Contains(err.Error(), "invalid cron expr") {
		t.Errorf("error = %v, want invalid cron expr error", err)
	}
}

// ---------------------------------------------------------------------------
// RemoveJob
// ---------------------------------------------------------------------------

func TestCronScheduler_RemoveJob(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	job := makeSchedulerJob("remove-test", validCronExpr, true)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	if err := sched.RemoveJob("remove-test"); err != nil {
		t.Fatalf("RemoveJob: %v", err)
	}

	if got := sched.GetJob("remove-test"); got != nil {
		t.Error("GetJob should return nil after RemoveJob")
	}

	sched.mu.Lock()
	_, ok := sched.entryMap["remove-test"]
	sched.mu.Unlock()

	if ok {
		t.Error("removed job should NOT be in entryMap")
	}
}

func TestCronScheduler_RemoveJob_NotFound(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	err := sched.RemoveJob("non-existent")
	if err == nil {
		t.Fatal("RemoveJob should return error for non-existent job")
	}
}

// ---------------------------------------------------------------------------
// PauseJob / ResumeJob
// ---------------------------------------------------------------------------

func TestCronScheduler_PauseJob(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	job := makeSchedulerJob("pause-test", validCronExpr, true)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	if err := sched.PauseJob("pause-test"); err != nil {
		t.Fatalf("PauseJob: %v", err)
	}

	got := sched.GetJob("pause-test")
	if got == nil {
		t.Fatal("GetJob returned nil after PauseJob")
	}
	if got.Enabled {
		t.Error("job should be disabled after pause")
	}

	sched.mu.Lock()
	_, ok := sched.entryMap["pause-test"]
	sched.mu.Unlock()

	if ok {
		t.Error("paused job should NOT be in entryMap")
	}
}

func TestCronScheduler_PauseJob_NotFound(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	err := sched.PauseJob("non-existent")
	if err == nil {
		t.Fatal("PauseJob should return error for non-existent job")
	}
}

func TestCronScheduler_ResumeJob(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	job := makeSchedulerJob("resume-test", validCronExpr, false)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	if err := sched.ResumeJob("resume-test"); err != nil {
		t.Fatalf("ResumeJob: %v", err)
	}

	got := sched.GetJob("resume-test")
	if got == nil {
		t.Fatal("GetJob returned nil after ResumeJob")
	}
	if !got.Enabled {
		t.Error("job should be enabled after resume")
	}

	sched.mu.Lock()
	_, ok := sched.entryMap["resume-test"]
	sched.mu.Unlock()

	if !ok {
		t.Error("resumed job should be in entryMap")
	}
}

func TestCronScheduler_ResumeJob_NotFound(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	err := sched.ResumeJob("non-existent")
	if err == nil {
		t.Fatal("ResumeJob should return error for non-existent job")
	}
}

// ---------------------------------------------------------------------------
// ListJobs / GetJob
// ---------------------------------------------------------------------------

func TestCronScheduler_ListJobs(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	jobs := sched.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("ListJobs len = %d, want 0", len(jobs))
	}

	for i := 0; i < 3; i++ {
		job := makeSchedulerJob(fmt.Sprintf("list-job-%d", i), validCronExpr, true)
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	jobs = sched.ListJobs()
	if len(jobs) != 3 {
		t.Errorf("ListJobs len = %d, want 3", len(jobs))
	}
}

func TestCronScheduler_GetJob(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	if got := sched.GetJob("non-existent"); got != nil {
		t.Error("GetJob should return nil for non-existent job")
	}

	job := makeSchedulerJob("get-test", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got := sched.GetJob("get-test")
	if got == nil {
		t.Fatal("GetJob returned nil for existing job")
	}
	if got.Prompt != "test prompt" {
		t.Errorf("Prompt = %q, want %q", got.Prompt, "test prompt")
	}
}

// ---------------------------------------------------------------------------
// ListRuns
// ---------------------------------------------------------------------------

func TestCronScheduler_ListRuns_NoRunsStore(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	mgr := &mockSessionManager2{}
	exec := NewExecutor(mgr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)
	// Do NOT set RunsStore

	runs := sched.ListRuns("any-job")
	if len(runs) != 0 {
		t.Errorf("ListRuns len = %d, want 0 (no RunsStore)", len(runs))
	}
}

func TestCronScheduler_ListRuns_WithRuns(t *testing.T) {
	sched, cs, rs := newTestScheduler(t)

	job := makeSchedulerJob("runs-test", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	run := &CronRun{
		ID:        "run-1",
		JobID:     "runs-test",
		StartedAt: time.Now(),
		Duration:  1 * time.Second,
		Status:    string(EventCompleted),
	}
	if err := rs.AddRun(run); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	runs := sched.ListRuns("runs-test")
	if len(runs) != 1 {
		t.Fatalf("ListRuns len = %d, want 1", len(runs))
	}
	if runs[0].ID != "run-1" {
		t.Errorf("Run ID = %q, want %q", runs[0].ID, "run-1")
	}
}

func TestCronScheduler_ListRuns_NonExistentJob(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	runs := sched.ListRuns("non-existent")
	if len(runs) != 0 {
		t.Errorf("ListRuns len = %d, want 0", len(runs))
	}
}

// ---------------------------------------------------------------------------
// executeJob
// ---------------------------------------------------------------------------

func TestCronScheduler_ExecuteJob_Success(t *testing.T) {
	sched, cs, rs := newTestScheduler(t)

	job := makeSchedulerJob("exec-test", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	sched.executeJob(context.Background(), "exec-test")

	time.Sleep(100 * time.Millisecond)
	sched.Stop()

	runs := rs.GetRuns("exec-test")
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
	if runs[0].Status != string(EventCompleted) {
		t.Errorf("run status = %q, want %q", runs[0].Status, EventCompleted)
	}

	got := cs.Get("exec-test")
	if got.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", got.RunCount)
	}
}

func TestCronScheduler_ExecuteJob_NotFound(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Should not panic
	sched.executeJob(context.Background(), "non-existent")
	time.Sleep(50 * time.Millisecond)
}

func TestCronScheduler_ExecuteJob_WithRetries(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	rs, _ := NewRunsStore(dir)

	callCount := 0
	mgr := &retryCountingManager{
		fn: func() error {
			callCount++
			if callCount < 3 {
				return fmt.Errorf("simulated failure")
			}
			return nil
		},
	}
	exec := NewExecutor(mgr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)
	sched.SetRunsStore(rs)

	job := makeSchedulerJob("retry-test", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sched.executeJob(context.Background(), "retry-test")
	sched.Stop()

	runs := rs.GetRuns("retry-test")
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
	if runs[0].RetryCount == 0 {
		t.Error("expected some retries to have occurred")
	}
}

// ---------------------------------------------------------------------------
// nextRun
// ---------------------------------------------------------------------------

func TestCronScheduler_NextRun(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	exec := NewExecutor(&mockSessionManager{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)

	tests := []struct {
		name     string
		cronExpr string
		wantZero bool
	}{
		{"valid every minute", validCronExpr, false},
		{"valid every 5 min", "0 */5 * * * *", false},
		{"valid daily", "0 0 0 * * *", false},
		{"invalid", "bad-expr", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &CronJob{CronExpr: tt.cronExpr}
			next := sched.nextRun(job)
			if tt.wantZero && !next.IsZero() {
				t.Error("expected zero time for invalid expression")
			}
			if !tt.wantZero && next.IsZero() {
				t.Error("expected non-zero time for valid expression")
			}
			if !tt.wantZero && next.Before(time.Now()) {
				t.Error("next run should be in the future")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateOutput
// ---------------------------------------------------------------------------

func TestCronScheduler_TruncateOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	exec := NewExecutor(&mockSessionManager{})
	sched := NewCronScheduler(cs, exec, logger, 4)

	tests := []struct {
		name    string
		input   string
		wantLen int
	}{
		{"short", "hello", 5},
		{"exact limit", string(make([]byte, maxOutputBytes)), maxOutputBytes},
		{"over limit", string(make([]byte, maxOutputBytes+100)), maxOutputBytes},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sched.truncateOutput(tt.input)
			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// retryWithBackoff
// ---------------------------------------------------------------------------

func TestCronScheduler_RetryWithBackoff(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	exec := NewExecutor(&mockSessionManager{})
	sched := NewCronScheduler(cs, exec, logger, 4)

	t.Run("success on first attempt", func(t *testing.T) {
		calls := 0
		err := sched.retryWithBackoff(context.Background(), []time.Duration{10 * time.Millisecond}, func() error {
			calls++
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
	})

	t.Run("success after retry", func(t *testing.T) {
		calls := 0
		err := sched.retryWithBackoff(context.Background(), []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}, func() error {
			calls++
			if calls == 1 {
				return fmt.Errorf("first failure")
			}
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if calls != 2 {
			t.Errorf("calls = %d, want 2", calls)
		}
	})

	t.Run("all retries exhausted", func(t *testing.T) {
		calls := 0
		err := sched.retryWithBackoff(context.Background(), []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}, func() error {
			calls++
			return fmt.Errorf("always fails")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 3 { // 1 initial + 2 retries
			t.Errorf("calls = %d, want 3", calls)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var called atomic.Bool
		err := sched.retryWithBackoff(ctx, []time.Duration{5 * time.Second}, func() error {
			called.Store(true)
			return fmt.Errorf("fails")
		})
		if err == nil {
			t.Fatal("expected error from canceled context")
		}
		// First attempt should still execute
		if !called.Load() {
			t.Error("first attempt should execute even with cancelled context")
		}
	})

	t.Run("empty delays means no retries", func(t *testing.T) {
		calls := 0
		err := sched.retryWithBackoff(context.Background(), nil, func() error {
			calls++
			return fmt.Errorf("always fails")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1 (no retries)", calls)
		}
	})
}

// ---------------------------------------------------------------------------
// SetRunsStore
// ---------------------------------------------------------------------------

func TestCronScheduler_SetRunsStore(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	exec := NewExecutor(&mockSessionManager{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)

	rs, _ := NewRunsStore(dir)
	sched.SetRunsStore(rs)

	sched.mu.Lock()
	got := sched.runsStore
	sched.mu.Unlock()

	if got == nil {
		t.Error("runsStore should be set after SetRunsStore")
	}
}

// ---------------------------------------------------------------------------
// fireCallbacks
// ---------------------------------------------------------------------------

func TestCronScheduler_FireCallbacks(t *testing.T) {
	t.Run("on_complete fires webhook", func(t *testing.T) {
		var receivedStatus string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedStatus = r.Header.Get("X-CronEvent")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sched, cs, _ := newTestScheduler(t)

		job := makeSchedulerJob("cb-complete", validCronExpr, true)
		job.OnComplete = server.URL
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}

		run := &CronRun{
			ID:        "run-1",
			JobID:     "cb-complete",
			StartedAt: time.Now(),
			Status:    string(EventCompleted),
		}

		sched.fireCallbacks(run)
		if receivedStatus != "completed" {
			t.Errorf("status = %q, want %q", receivedStatus, "completed")
		}
	})

	t.Run("on_fail fires webhook", func(t *testing.T) {
		var receivedStatus string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedStatus = r.Header.Get("X-CronEvent")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sched, cs, _ := newTestScheduler(t)

		job := makeSchedulerJob("cb-fail", validCronExpr, true)
		job.OnFail = server.URL
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}

		run := &CronRun{
			ID:        "run-1",
			JobID:     "cb-fail",
			StartedAt: time.Now(),
			Status:    string(EventFailed),
		}

		sched.fireCallbacks(run)
		if receivedStatus != "failed" {
			t.Errorf("status = %q, want %q", receivedStatus, "failed")
		}
	})

	t.Run("no callback when URL is empty", func(t *testing.T) {
		sched, cs, _ := newTestScheduler(t)

		job := makeSchedulerJob("cb-empty", validCronExpr, true)
		job.OnComplete = ""
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}

		run := &CronRun{
			ID:        "run-1",
			JobID:     "cb-empty",
			StartedAt: time.Now(),
			Status:    string(EventCompleted),
		}

		// Should not panic
		sched.fireCallbacks(run)
	})

	t.Run("invalid URL does not panic", func(t *testing.T) {
		sched, cs, _ := newTestScheduler(t)

		job := makeSchedulerJob("cb-invalid", validCronExpr, true)
		job.OnComplete = "://not-a-valid-url"
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}

		run := &CronRun{
			ID:        "run-1",
			JobID:     "cb-invalid",
			StartedAt: time.Now(),
			Status:    string(EventCompleted),
		}

		// Should not panic
		sched.fireCallbacks(run)
	})

	t.Run("webhook server error does not panic", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		sched, cs, _ := newTestScheduler(t)

		job := makeSchedulerJob("cb-error", validCronExpr, true)
		job.OnComplete = server.URL
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}

		run := &CronRun{
			ID:        "run-1",
			JobID:     "cb-error",
			StartedAt: time.Now(),
			Status:    string(EventCompleted),
		}

		// Should not panic (webhook failure is logged, not propagated)
		sched.fireCallbacks(run)
	})

	t.Run("job not found for callback", func(t *testing.T) {
		sched, _, _ := newTestScheduler(t)

		run := &CronRun{
			ID:        "run-1",
			JobID:     "non-existent-job",
			StartedAt: time.Now(),
			Status:    string(EventCompleted),
		}

		// Should not panic
		sched.fireCallbacks(run)
	})
}

// ---------------------------------------------------------------------------
// Concurrency (semaphore size validation only - no concurrent executeJob
// because the source code has a known race on job field mutation)
// ---------------------------------------------------------------------------

func TestCronScheduler_SemaphoreCapacity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	exec := NewExecutor(&mockSessionManager2{})

	tests := []struct {
		name    string
		max     int
		wantCap int
	}{
		{"default", 0, 4},
		{"custom", 8, 8},
		{"single", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := NewCronScheduler(cs, exec, logger, tt.max)
			if cap(sched.sem) != tt.wantCap {
				t.Errorf("sem capacity = %d, want %d", cap(sched.sem), tt.wantCap)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Default retry delays validation
// ---------------------------------------------------------------------------

func TestDefaultRetryDelays(t *testing.T) {
	if len(defaultRetryDelays) != 3 {
		t.Errorf("defaultRetryDelays len = %d, want 3", len(defaultRetryDelays))
	}
	// Verify exponential growth
	if defaultRetryDelays[0] != 1*time.Second {
		t.Errorf("defaultRetryDelays[0] = %v, want 1s", defaultRetryDelays[0])
	}
	if defaultRetryDelays[1] != 2*time.Second {
		t.Errorf("defaultRetryDelays[1] = %v, want 2s", defaultRetryDelays[1])
	}
	if defaultRetryDelays[2] != 4*time.Second {
		t.Errorf("defaultRetryDelays[2] = %v, want 4s", defaultRetryDelays[2])
	}
}

// ---------------------------------------------------------------------------
// Start - registers invalid cron job (warning path)
// ---------------------------------------------------------------------------

func TestCronScheduler_Start_InvalidJob(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	job := makeSchedulerJob("invalid-cron", "bad-expr", true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Start should still succeed, logging a warning for the invalid job
	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Invalid job should NOT be in entryMap
	sched.mu.Lock()
	_, ok := sched.entryMap["invalid-cron"]
	sched.mu.Unlock()

	if ok {
		t.Error("invalid cron job should NOT be registered in entryMap")
	}
}

// ---------------------------------------------------------------------------
// RemoveJob - job not in entryMap (deleted from store but not scheduler)
// ---------------------------------------------------------------------------

func TestCronScheduler_RemoveJob_NotInEntryMap(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Add a disabled job - it goes into the store but NOT into entryMap
	job := makeSchedulerJob("no-entry", validCronExpr, false)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	// RemoveJob should fail because it's not in entryMap
	err := sched.RemoveJob("no-entry")
	if err == nil {
		t.Fatal("RemoveJob should return error when job not in entryMap")
	}
	if !strings.Contains(err.Error(), "not registered in scheduler") {
		t.Errorf("error = %v, want 'not registered in scheduler'", err)
	}
}

// ---------------------------------------------------------------------------
// PauseJob - job not in entryMap
// ---------------------------------------------------------------------------

func TestCronScheduler_PauseJob_NotInEntryMap(t *testing.T) {
	sched, _, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Add a disabled job
	job := makeSchedulerJob("pause-no-entry", validCronExpr, false)
	if err := sched.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	// PauseJob should fail because it's not in entryMap
	err := sched.PauseJob("pause-no-entry")
	if err == nil {
		t.Fatal("PauseJob should return error when job not in entryMap")
	}
	if !strings.Contains(err.Error(), "not registered in scheduler") {
		t.Errorf("error = %v, want 'not registered in scheduler'", err)
	}
}

// ---------------------------------------------------------------------------
// ResumeJob - invalid cron expression
// ---------------------------------------------------------------------------

func TestCronScheduler_ResumeJob_InvalidCronExpr(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Add a disabled job with invalid cron expression
	job := makeSchedulerJob("resume-bad-cron", "invalid", false)
	if err := cs.Add(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	// ResumeJob should fail because cron expression is invalid
	err := sched.ResumeJob("resume-bad-cron")
	if err == nil {
		t.Fatal("ResumeJob should return error for invalid cron expression")
	}
}

// ---------------------------------------------------------------------------
// executeJob - all retries exhausted
// ---------------------------------------------------------------------------

func TestCronScheduler_ExecuteJob_AllRetriesExhausted(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	rs, _ := NewRunsStore(dir)

	callCount := 0
	mgr := &retryCountingManager{
		fn: func() error {
			callCount++
			return fmt.Errorf("always fails")
		},
	}
	exec := NewExecutor(mgr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)
	sched.SetRunsStore(rs)

	job := makeSchedulerJob("all-retry-fail", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sched.executeJob(context.Background(), "all-retry-fail")
	sched.Stop()

	runs := rs.GetRuns("all-retry-fail")
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
	if runs[0].Status != string(EventFailed) {
		t.Errorf("run status = %q, want %q", runs[0].Status, EventFailed)
	}
	if runs[0].Error == "" {
		t.Error("run error should not be empty")
	}
}

// ---------------------------------------------------------------------------
// executeJob - context cancelled before slot acquisition
// ---------------------------------------------------------------------------

func TestCronScheduler_ExecuteJob_ContextCancelled(t *testing.T) {
	sched, cs, rs := newTestScheduler(t)

	job := makeSchedulerJob("ctx-cancel", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Fill the semaphore to capacity
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	sched.executeJob(ctx, "ctx-cancel")
	sched.Stop()

	// No run should have been recorded
	runs := rs.GetRuns("ctx-cancel")
	if len(runs) != 0 {
		t.Errorf("runs len = %d, want 0 (context cancelled)", len(runs))
	}
}

// ---------------------------------------------------------------------------
// addRun - no RunsStore set
// ---------------------------------------------------------------------------

func TestCronScheduler_AddRun_NoRunsStore(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewCronStore(dir)
	mgr := &mockSessionManager2{}
	exec := NewExecutor(mgr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sched := NewCronScheduler(cs, exec, logger, 4)
	// Do NOT set RunsStore

	job := makeSchedulerJob("no-runs-store", validCronExpr, true)
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sched.executeJob(context.Background(), "no-runs-store")
	sched.Stop()

	// Should not panic, job should still be updated
	got := cs.Get("no-runs-store")
	if got == nil {
		t.Fatal("job should still exist")
	}
	if got.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", got.RunCount)
	}
}

// ---------------------------------------------------------------------------
// executeJob - verifies job metadata updates
// ---------------------------------------------------------------------------

func TestCronScheduler_ExecuteJob_UpdatesJobMetadata(t *testing.T) {
	sched, cs, rs := newTestScheduler(t)

	job := makeSchedulerJob("metadata-test", validCronExpr, true)
	job.TimeoutMins = 1
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sched.executeJob(context.Background(), "metadata-test")
	sched.Stop()

	got := cs.Get("metadata-test")
	if got.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", got.RunCount)
	}
	if got.LastRun.IsZero() {
		t.Error("LastRun should not be zero")
	}
	if got.NextRun.IsZero() {
		t.Error("NextRun should not be zero")
	}

	runs := rs.GetRuns("metadata-test")
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
	if runs[0].FinishedAt.IsZero() {
		t.Error("run FinishedAt should not be zero")
	}
	if runs[0].Duration <= 0 {
		t.Error("run Duration should be positive")
	}
}

// ---------------------------------------------------------------------------
// executeJob - default timeout when TimeoutMins is 0
// ---------------------------------------------------------------------------

func TestCronScheduler_ExecuteJob_DefaultTimeout(t *testing.T) {
	sched, cs, rs := newTestScheduler(t)

	job := makeSchedulerJob("default-timeout", validCronExpr, true)
	job.TimeoutMins = 0 // Should default to 30 minutes
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sched.executeJob(context.Background(), "default-timeout")
	sched.Stop()

	runs := rs.GetRuns("default-timeout")
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
	if runs[0].Status != string(EventCompleted) {
		t.Errorf("run status = %q, want %q", runs[0].Status, EventCompleted)
	}
}

// ---------------------------------------------------------------------------
// fireCallbacks - no callback URL mismatch (completed with on_fail only)
// ---------------------------------------------------------------------------

func TestCronScheduler_FireCallbacks_NoMatchingCallback(t *testing.T) {
	sched, cs, _ := newTestScheduler(t)

	job := makeSchedulerJob("no-match-cb", validCronExpr, true)
	job.OnFail = "http://example.com/fail" // Only on_fail callback
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Completed run but only on_fail is set - should not fire
	run := &CronRun{
		ID:        "run-no-match",
		JobID:     "no-match-cb",
		StartedAt: time.Now(),
		Status:    string(EventCompleted),
	}

	// Should not panic or fire any callback
	sched.fireCallbacks(run)
}

// ---------------------------------------------------------------------------
// addRun - persists and fires callbacks
// ---------------------------------------------------------------------------

func TestCronScheduler_AddRun_WithCallbacks(t *testing.T) {
	var callbackFired bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackFired = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sched, cs, rs := newTestScheduler(t)

	job := makeSchedulerJob("cb-add-run", validCronExpr, true)
	job.OnComplete = server.URL
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sched.executeJob(context.Background(), "cb-add-run")
	sched.Stop()

	if !callbackFired {
		t.Error("callback should have been fired for completed run")
	}

	runs := rs.GetRuns("cb-add-run")
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
}
