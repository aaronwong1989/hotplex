package croncmd

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hrygo/hotplex/internal/cron"
)

// ---------------------------------------------------------------------------
// add_cron
// ---------------------------------------------------------------------------

func TestAddCronCmd_MissingBothFlags(t *testing.T) {
	SessionCmd.SetArgs([]string{"add_cron"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error when neither --cron nor --prompt provided")
	}
}

func TestAddCronCmd_MissingPromptFlag(t *testing.T) {
	SessionCmd.SetArgs([]string{"add_cron", "--cron", "*/5 * * * *"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error when --prompt is missing")
	}
}

func TestAddCronCmd_MissingCronFlag(t *testing.T) {
	// Reset --cron flag from previous tests to avoid cobra flag persistence
	if err := addCronCmd.Flags().Set("cron", ""); err != nil {
		t.Fatalf("reset --cron flag: %v", err)
	}
	SessionCmd.SetArgs([]string{"add_cron", "--prompt", "test"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error when --cron is missing")
	}
}

func TestAddCronCmd_Valid(t *testing.T) {
	SessionCmd.SetArgs([]string{"add_cron", "--cron", "*/5 * * * *", "--prompt", "Run tests"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddCronCmd_WithOptionalFlags(t *testing.T) {
	SessionCmd.SetArgs([]string{
		"add_cron",
		"--cron", "0 0 * * *",
		"--prompt", "Daily task",
		"--work-dir", "/tmp",
		"--type", "medium",
		"--timeout", "60",
		"--retries", "5",
		"--disabled",
	})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// del_cron
// ---------------------------------------------------------------------------

func TestDelCronCmd_MissingID(t *testing.T) {
	SessionCmd.SetArgs([]string{"del_cron"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error for missing --id flag")
	}
}

func TestDelCronCmd_NonExistent(t *testing.T) {
	SessionCmd.SetArgs([]string{"del_cron", "--id", "non-existent-id"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent job ID")
	}
}

// ---------------------------------------------------------------------------
// list_crons
// ---------------------------------------------------------------------------

func TestListCronsCmd_Empty(t *testing.T) {
	SessionCmd.SetArgs([]string{"list_crons"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCronsCmd_WithJobs(t *testing.T) {
	// Add a job first
	SessionCmd.SetArgs([]string{"add_cron", "--cron", "* * * * *", "--prompt", "test list"})
	t.Setenv("HOME", t.TempDir())
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("add_cron: %v", err)
	}

	// Then list
	SessionCmd.SetArgs([]string{"list_crons"})
	err := SessionCmd.Execute()
	if err != nil {
		t.Fatalf("list_crons: %v", err)
	}
}

// ---------------------------------------------------------------------------
// pause_cron / resume_cron
// ---------------------------------------------------------------------------

func TestPauseCronCmd_MissingID(t *testing.T) {
	SessionCmd.SetArgs([]string{"pause_cron"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error for missing --id flag")
	}
}

func TestResumeCronCmd_MissingID(t *testing.T) {
	SessionCmd.SetArgs([]string{"resume_cron"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error for missing --id flag")
	}
}

func TestPauseResumeCronCmd_NonExistent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	SessionCmd.SetArgs([]string{"pause_cron", "--id", "non-existent"})
	if err := SessionCmd.Execute(); err == nil {
		t.Error("expected error for non-existent job (pause)")
	}

	SessionCmd.SetArgs([]string{"resume_cron", "--id", "non-existent"})
	if err := SessionCmd.Execute(); err == nil {
		t.Error("expected error for non-existent job (resume)")
	}
}

// ---------------------------------------------------------------------------
// list_runs
// ---------------------------------------------------------------------------

func TestListRunsCmd_MissingJobID(t *testing.T) {
	SessionCmd.SetArgs([]string{"list_runs"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error for missing --job-id flag")
	}
}

func TestListRunsCmd_NoRuns(t *testing.T) {
	SessionCmd.SetArgs([]string{"list_runs", "--job-id", "non-existent"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRunsCmd_WithLastN(t *testing.T) {
	// --last 0 with non-existent job should still work
	SessionCmd.SetArgs([]string{"list_runs", "--job-id", "some-job", "--last", "0"})
	t.Setenv("HOME", t.TempDir())

	err := SessionCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pause/Resume with existing job (setJobEnabled success path)
// ---------------------------------------------------------------------------

func TestPauseCronCmd_ExistingJob(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a job using the same path the command will use
	store, err := cron.NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	job := &cron.CronJob{
		ID:       "known-job-id",
		CronExpr: "* * * * *",
		Prompt:   "test",
		Enabled:  true,
	}
	if err := store.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Now pause it using the command
	SessionCmd.SetArgs([]string{"pause_cron", "--id", "known-job-id"})
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("pause_cron: %v", err)
	}

	// Verify it's paused - reload from disk
	store2, err := cron.NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore (reload): %v", err)
	}
	got := store2.Get("known-job-id")
	if got == nil {
		t.Fatal("job should exist after pause")
	}
	if got.Enabled {
		t.Error("job should be disabled after pause")
	}
}

func TestResumeCronCmd_ExistingJob(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a disabled job using the same path the command will use
	store, err := cron.NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	job := &cron.CronJob{
		ID:       "known-job-id-2",
		CronExpr: "* * * * *",
		Prompt:   "test",
		Enabled:  false,
	}
	if err := store.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Now resume it using the command
	SessionCmd.SetArgs([]string{"resume_cron", "--id", "known-job-id-2"})
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("resume_cron: %v", err)
	}

	// Verify it's resumed - reload from disk
	store2, err := cron.NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore (reload): %v", err)
	}
	got := store2.Get("known-job-id-2")
	if got == nil {
		t.Fatal("job should exist after resume")
	}
	if !got.Enabled {
		t.Error("job should be enabled after resume")
	}
}

// ---------------------------------------------------------------------------
// SessionCmd structure
// ---------------------------------------------------------------------------

func TestSessionCmd_HasExpectedSubcommands(t *testing.T) {
	expected := map[string]bool{
		"add_cron":    false,
		"del_cron":    false,
		"list_crons":  false,
		"pause_cron":  false,
		"resume_cron": false,
		"list_runs":   false,
	}

	for _, cmd := range SessionCmd.Commands() {
		if _, ok := expected[cmd.Name()]; ok {
			expected[cmd.Name()] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestSessionCmd_Use(t *testing.T) {
	if SessionCmd.Use != "cron" {
		t.Errorf("Use = %q, want %q", SessionCmd.Use, "cron")
	}
	if SessionCmd.Short != "Cron job management commands" {
		t.Errorf("Short = %q, want %q", SessionCmd.Short, "Cron job management commands")
	}
}

// ---------------------------------------------------------------------------
// del_cron - success path
// ---------------------------------------------------------------------------

func TestDelCronCmd_Success(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-create a job
	store, err := cron.NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	job := &cron.CronJob{
		ID:       "delete-me",
		CronExpr: "* * * * *",
		Prompt:   "to be deleted",
	}
	if err := store.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	SessionCmd.SetArgs([]string{"del_cron", "--id", "delete-me"})
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("del_cron: %v", err)
	}

	// Verify deleted
	store2, err := cron.NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore (reload): %v", err)
	}
	if got := store2.Get("delete-me"); got != nil {
		t.Error("job should be deleted")
	}
}

// ---------------------------------------------------------------------------
// list_runs - with runs and --last flag
// ---------------------------------------------------------------------------

func TestListRunsCmd_WithRuns(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-create some runs
	rs, err := cron.NewRunsStore("")
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}
	now := time.Now()
	for i := 0; i < 5; i++ {
		run := &cron.CronRun{
			ID:        fmt.Sprintf("run-%d", i),
			JobID:     "runs-job",
			StartedAt: now.Add(time.Duration(i) * time.Minute),
			Duration:  time.Duration(i+1) * time.Second,
			Status:    string(cron.EventCompleted),
		}
		if err := rs.AddRun(run); err != nil {
			t.Fatalf("AddRun: %v", err)
		}
	}

	// List all runs
	SessionCmd.SetArgs([]string{"list_runs", "--job-id", "runs-job"})
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("list_runs: %v", err)
	}

	// List with --last 2
	SessionCmd.SetArgs([]string{"list_runs", "--job-id", "runs-job", "--last", "2"})
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("list_runs --last: %v", err)
	}
}

// ---------------------------------------------------------------------------
// list_runs - failed run with error message
// ---------------------------------------------------------------------------

func TestListRunsCmd_WithFailedRun(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rs, err := cron.NewRunsStore("")
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}

	run := &cron.CronRun{
		ID:        "run-fail",
		JobID:     "fail-job",
		StartedAt: time.Now(),
		Duration:  3 * time.Second,
		Status:    string(cron.EventFailed),
		Error:     "something went wrong",
	}
	if err := rs.AddRun(run); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	// Failed run with empty error should show "(none)"
	run2 := &cron.CronRun{
		ID:        "run-fail-no-err",
		JobID:     "fail-job",
		StartedAt: time.Now().Add(time.Minute),
		Duration:  1 * time.Second,
		Status:    string(cron.EventFailed),
		Error:     "",
	}
	if err := rs.AddRun(run2); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	SessionCmd.SetArgs([]string{"list_runs", "--job-id", "fail-job"})
	if err := SessionCmd.Execute(); err != nil {
		t.Fatalf("list_runs: %v", err)
	}
}

// ---------------------------------------------------------------------------
// setJobEnabled - job not found
// ---------------------------------------------------------------------------

func TestSetJobEnabled_NotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Pause non-existent job
	SessionCmd.SetArgs([]string{"pause_cron", "--id", "ghost-job"})
	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent job")
	}
}

// ---------------------------------------------------------------------------
// add_cron - store creation error (read-only directory)
// ---------------------------------------------------------------------------

func TestAddCronCmd_StoreError(t *testing.T) {
	// Use a read-only directory to force MkdirAll failure
	readOnlyDir := t.TempDir() + "/readonly"
	if err := os.Mkdir(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0o755) }() // restore for cleanup

	// Point HOME to the read-only parent so the store creation fails
	t.Setenv("HOME", readOnlyDir)

	SessionCmd.SetArgs([]string{"add_cron", "--cron", "*/5 * * * *", "--prompt", "test"})
	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error when store creation fails")
	}
}

// ---------------------------------------------------------------------------
// list_crons - store creation error (read-only directory)
// ---------------------------------------------------------------------------

func TestListCronsCmd_StoreError(t *testing.T) {
	readOnlyDir := t.TempDir() + "/readonly"
	if err := os.Mkdir(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0o755) }()

	t.Setenv("HOME", readOnlyDir)

	SessionCmd.SetArgs([]string{"list_crons"})
	err := SessionCmd.Execute()
	if err == nil {
		t.Error("expected error when store creation fails")
	}
}
