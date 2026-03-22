package cron

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestRunsStore(t *testing.T) *RunsStore {
	t.Helper()
	dir := t.TempDir()
	rs, err := NewRunsStore(dir)
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}
	return rs
}

func makeTestRun(jobID string, t time.Time) *CronRun {
	return &CronRun{
		ID:        "run-" + jobID,
		JobID:     jobID,
		StartedAt: t,
		Duration:  5 * time.Second,
		Status:    string(EventCompleted),
	}
}

func TestRunsStore_NewRunsStore(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
		wantErr bool
	}{
		{"temp dir", t.TempDir(), false},
		{"empty dir creates default", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dataDir == "" {
				t.Setenv("HOME", t.TempDir())
			}
			rs, err := NewRunsStore(tt.dataDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRunsStore() error = %v, wantErr %v", err, tt.wantErr)
			}
			if rs != nil {
				_ = rs
			}
		})
	}
}

func TestRunsStore_AddRun(t *testing.T) {
	rs := newTestRunsStore(t)

	now := time.Now()
	run := makeTestRun("job-1", now)

	if err := rs.AddRun(run); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	runs := rs.GetRuns("job-1")
	if len(runs) != 1 {
		t.Fatalf("GetRuns len = %d, want 1", len(runs))
	}
	if runs[0].ID != "run-job-1" {
		t.Errorf("Run ID = %q, want %q", runs[0].ID, "run-job-1")
	}
}

func TestRunsStore_AddRun_PersistsToDisk(t *testing.T) {
	rs := newTestRunsStore(t)

	now := time.Now()
	run := makeTestRun("persist-job", now)
	if err := rs.AddRun(run); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	// Reload from disk
	rs2, err := NewRunsStore(filepath.Dir(rs.path))
	if err != nil {
		t.Fatalf("NewRunsStore (2nd): %v", err)
	}

	runs := rs2.GetRuns("persist-job")
	if len(runs) != 1 {
		t.Fatalf("GetRuns len = %d, want 1", len(runs))
	}
}

func TestRunsStore_AddRun_SortNewestFirst(t *testing.T) {
	rs := newTestRunsStore(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		run := makeTestRun("sort-job", base.Add(time.Duration(i)*time.Minute))
		run.ID = "run-" + string(rune('a'+i))
		if err := rs.AddRun(run); err != nil {
			t.Fatalf("AddRun: %v", err)
		}
	}

	runs := rs.GetRuns("sort-job")
	if len(runs) != 3 {
		t.Fatalf("GetRuns len = %d, want 3", len(runs))
	}

	// Newest first
	if !runs[0].StartedAt.After(runs[1].StartedAt) {
		t.Error("runs should be sorted newest first")
	}
	if !runs[1].StartedAt.After(runs[2].StartedAt) {
		t.Error("runs should be sorted newest first")
	}
}

func TestRunsStore_AddRun_TrimToLimit(t *testing.T) {
	rs := newTestRunsStore(t)

	// Add runs exceeding the default limit (100)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 110; i++ {
		run := makeTestRun("limit-job", base.Add(time.Duration(i)*time.Minute))
		run.ID = "run-" + string(rune('0'+i%10)) + "-" + string(rune('a'+i/10))
		if err := rs.AddRun(run); err != nil {
			t.Fatalf("AddRun %d: %v", i, err)
		}
	}

	runs := rs.GetRuns("limit-job")
	if len(runs) > defaultRunsLimit {
		t.Errorf("GetRuns len = %d, want <= %d", len(runs), defaultRunsLimit)
	}
}

func TestRunsStore_GetRuns_Empty(t *testing.T) {
	rs := newTestRunsStore(t)

	runs := rs.GetRuns("non-existent")
	if len(runs) != 0 {
		t.Errorf("GetRuns len = %d, want 0", len(runs))
	}
}

func TestRunsStore_GetRuns_ReturnsCopy(t *testing.T) {
	rs := newTestRunsStore(t)

	now := time.Now()
	run := makeTestRun("copy-job", now)
	if err := rs.AddRun(run); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	runs := rs.GetRuns("copy-job")
	if len(runs) != 1 {
		t.Fatalf("GetRuns len = %d, want 1", len(runs))
	}

	// Appending to the returned slice should not affect the store
	runs = append(runs, &CronRun{ID: "extra"})
	if len(runs) != 2 {
		t.Errorf("len should be 2, got %d", len(runs))
	}
	runs2 := rs.GetRuns("copy-job")
	if len(runs2) != 1 {
		t.Errorf("len after append = %d, want 1", len(runs2))
	}
}

func TestRunsStore_GetRuns_MultipleJobs(t *testing.T) {
	rs := newTestRunsStore(t)

	now := time.Now()
	if err := rs.AddRun(makeTestRun("job-a", now)); err != nil {
		t.Fatalf("AddRun job-a: %v", err)
	}
	if err := rs.AddRun(makeTestRun("job-a", now.Add(time.Minute))); err != nil {
		t.Fatalf("AddRun job-a-2: %v", err)
	}
	if err := rs.AddRun(makeTestRun("job-b", now)); err != nil {
		t.Fatalf("AddRun job-b: %v", err)
	}

	runsA := rs.GetRuns("job-a")
	if len(runsA) != 2 {
		t.Errorf("GetRuns job-a len = %d, want 2", len(runsA))
	}

	runsB := rs.GetRuns("job-b")
	if len(runsB) != 1 {
		t.Errorf("GetRuns job-b len = %d, want 1", len(runsB))
	}
}

func TestRunsStore_AtomicWrite_Format(t *testing.T) {
	rs := newTestRunsStore(t)

	now := time.Now()
	run := makeTestRun("format-job", now)
	if err := rs.AddRun(run); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	data, err := os.ReadFile(rs.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var rf runsFile
	if err := json.Unmarshal(data, &rf); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}

	if rf.Version != runsSchemaVersion {
		t.Errorf("Version = %d, want %d", rf.Version, runsSchemaVersion)
	}
	if len(rf.Runs) != 1 {
		t.Fatalf("Runs len = %d, want 1", len(rf.Runs))
	}
	if rf.Runs[0].JobID != "format-job" {
		t.Errorf("Run JobID = %q, want %q", rf.Runs[0].JobID, "format-job")
	}
}

func TestRunsStore_NoTmpFileRemains(t *testing.T) {
	rs := newTestRunsStore(t)

	now := time.Now()
	if err := rs.AddRun(makeTestRun("tmp-job", now)); err != nil {
		t.Fatalf("AddRun: %v", err)
	}

	tmpPath := rs.path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tmp file should not exist after successful write")
	}
}

// ---------------------------------------------------------------------------
// RunsStore - default directory path
// ---------------------------------------------------------------------------

func TestRunsStore_DefaultDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rs, err := NewRunsStore("")
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}

	// Verify the path is under HOME/.hotplex/cron
	expectedDir := filepath.Join(tmpHome, ".hotplex", "cron")
	if !strings.HasPrefix(rs.path, expectedDir) {
		t.Errorf("path = %q, want prefix %q", rs.path, expectedDir)
	}

	// Verify the directory was created
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("default directory should be created")
	}
}

// ---------------------------------------------------------------------------
// RunsStore - corrupted JSON file
// ---------------------------------------------------------------------------

func TestRunsStore_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.json")

	// Write corrupted JSON
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	_, err := NewRunsStore(dir)
	if err == nil {
		t.Fatal("expected error for corrupted JSON file")
	}
}

// ---------------------------------------------------------------------------
// RunsStore - MkdirAll failure
// ---------------------------------------------------------------------------

func TestRunsStore_MkdirAllFailure(t *testing.T) {
	invalidDir := "\x00/invalid/path"

	_, err := NewRunsStore(invalidDir)
	if err == nil {
		t.Fatal("expected error for invalid directory path")
	}
}

// ---------------------------------------------------------------------------
// RunsStore - multiple reloads preserve data
// ---------------------------------------------------------------------------

func TestRunsStore_MultipleReloads(t *testing.T) {
	dir := t.TempDir()

	// First store: add runs for two jobs
	rs1, err := NewRunsStore(dir)
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}

	now := time.Now()
	for i := 0; i < 3; i++ {
		run := makeTestRun("job-a", now.Add(time.Duration(i)*time.Minute))
		run.ID = "run-a-" + string(rune('0'+i))
		if err := rs1.AddRun(run); err != nil {
			t.Fatalf("AddRun job-a: %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		run := makeTestRun("job-b", now.Add(time.Duration(i)*time.Minute))
		run.ID = "run-b-" + string(rune('0'+i))
		if err := rs1.AddRun(run); err != nil {
			t.Fatalf("AddRun job-b: %v", err)
		}
	}

	// Second store: reload
	rs2, err := NewRunsStore(dir)
	if err != nil {
		t.Fatalf("NewRunsStore (2nd): %v", err)
	}

	runsA := rs2.GetRuns("job-a")
	if len(runsA) != 3 {
		t.Errorf("GetRuns job-a len = %d, want 3", len(runsA))
	}

	runsB := rs2.GetRuns("job-b")
	if len(runsB) != 2 {
		t.Errorf("GetRuns job-b len = %d, want 2", len(runsB))
	}

	runsC := rs2.GetRuns("non-existent")
	if len(runsC) != 0 {
		t.Errorf("GetRuns non-existent len = %d, want 0", len(runsC))
	}
}

// ---------------------------------------------------------------------------
// RunsStore - path field
// ---------------------------------------------------------------------------

func TestRunsStore_Path(t *testing.T) {
	dir := t.TempDir()
	rs, err := NewRunsStore(dir)
	if err != nil {
		t.Fatalf("NewRunsStore: %v", err)
	}

	expectedPath := filepath.Join(dir, "runs.json")
	if rs.path != expectedPath {
		t.Errorf("path = %q, want %q", rs.path, expectedPath)
	}
}
