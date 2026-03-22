package cron

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestCronStore(t *testing.T) *CronStore {
	t.Helper()
	dir := t.TempDir()
	cs, err := NewCronStore(dir)
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	return cs
}

func makeTestJob(id string) *CronJob {
	return &CronJob{
		ID:          id,
		CronExpr:    "*/5 * * * *",
		Prompt:      "Run tests",
		WorkDir:     "/tmp",
		Type:        JobTypeLight,
		TimeoutMins: 30,
		Retries:     3,
		Enabled:     true,
		CreatedBy:   "tester",
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestCronStore_NewCronStore(t *testing.T) {
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
				// Use a temp dir instead of polluting home directory
				t.Setenv("HOME", t.TempDir())
			}
			cs, err := NewCronStore(tt.dataDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCronStore() error = %v, wantErr %v", err, tt.wantErr)
			}
			if cs != nil {
				_ = cs
			}
		})
	}
}

func TestCronStore_Add(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("job-1")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if job.ID != "job-1" {
		t.Errorf("ID = %q, want %q", job.ID, "job-1")
	}

	// Verify it's persisted
	reloaded, err := NewCronStore(filepath.Dir(cs.path))
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	got := reloaded.Get("job-1")
	if got == nil {
		t.Fatal("Get returned nil after reload")
	}
	if got.Prompt != "Run tests" {
		t.Errorf("Prompt = %q, want %q", got.Prompt, "Run tests")
	}
}

func TestCronStore_Add_GeneratesID(t *testing.T) {
	cs := newTestCronStore(t)

	job := &CronJob{CronExpr: "* * * * *", Prompt: "test"}
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if job.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

func TestCronStore_Add_GeneratesCreatedAt(t *testing.T) {
	cs := newTestCronStore(t)

	fixedTime := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	originalNow := nowFunc
	nowFunc = func() time.Time { return fixedTime }
	defer func() { nowFunc = originalNow }()

	job := &CronJob{CronExpr: "* * * * *", Prompt: "test"}
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !job.CreatedAt.Equal(fixedTime) {
		t.Errorf("CreatedAt = %v, want %v", job.CreatedAt, fixedTime)
	}
}

func TestCronStore_Add_PersistsToDisk(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("persist-test")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify file content
	data, err := os.ReadFile(cs.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var cf cronJobsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cf.Version != cronJobsSchemaVersion {
		t.Errorf("Version = %d, want %d", cf.Version, cronJobsSchemaVersion)
	}
	if len(cf.Jobs) != 1 {
		t.Fatalf("Jobs len = %d, want 1", len(cf.Jobs))
	}
	if cf.Jobs[0].ID != "persist-test" {
		t.Errorf("Job ID = %q, want %q", cf.Jobs[0].ID, "persist-test")
	}
}

func TestCronStore_Get(t *testing.T) {
	cs := newTestCronStore(t)

	// Non-existent job
	got := cs.Get("non-existent")
	if got != nil {
		t.Error("Get should return nil for non-existent job")
	}

	// Existing job
	job := makeTestJob("get-test")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got = cs.Get("get-test")
	if got == nil {
		t.Fatal("Get returned nil for existing job")
	}
	if got.CronExpr != "*/5 * * * *" {
		t.Errorf("CronExpr = %q, want %q", got.CronExpr, "*/5 * * * *")
	}
}

func TestCronStore_Update(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("update-test")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Update the job
	job.Prompt = "Updated prompt"
	job.Enabled = false
	if err := cs.Update(job); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := cs.Get("update-test")
	if got == nil {
		t.Fatal("Get returned nil after update")
	}
	if got.Prompt != "Updated prompt" {
		t.Errorf("Prompt = %q, want %q", got.Prompt, "Updated prompt")
	}
	if got.Enabled {
		t.Error("Enabled should be false")
	}
}

func TestCronStore_Update_NotFound(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("non-existent")
	err := cs.Update(job)
	if err == nil {
		t.Fatal("Update should return error for non-existent job")
	}
}

func TestCronStore_Delete(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("delete-test")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := cs.Delete("delete-test"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if got := cs.Get("delete-test"); got != nil {
		t.Error("Get should return nil after delete")
	}
}

func TestCronStore_Delete_NotFound(t *testing.T) {
	cs := newTestCronStore(t)

	err := cs.Delete("non-existent")
	if err == nil {
		t.Fatal("Delete should return error for non-existent job")
	}
}

func TestCronStore_List(t *testing.T) {
	cs := newTestCronStore(t)

	// Empty store
	jobs := cs.List()
	if len(jobs) != 0 {
		t.Errorf("List len = %d, want 0", len(jobs))
	}

	// Add multiple jobs
	for i := 0; i < 5; i++ {
		job := makeTestJob("list-job-" + string(rune('a'+i)))
		if err := cs.Add(job); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	jobs = cs.List()
	if len(jobs) != 5 {
		t.Errorf("List len = %d, want 5", len(jobs))
	}
}

func TestCronStore_List_ReturnsSnapshot(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("snapshot-test")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	jobs := cs.List()
	if len(jobs) != 1 {
		t.Fatalf("List len = %d, want 1", len(jobs))
	}

	// Appending to the returned slice should not affect the store
	jobs = append(jobs, &CronJob{ID: "extra"})
	if len(jobs) != 2 {
		t.Errorf("len should be 2, got %d", len(jobs))
	}
	jobs2 := cs.List()
	if len(jobs2) != 1 {
		t.Errorf("len after append = %d, want 1", len(jobs2))
	}
}

func TestCronStore_ReloadFromDisk(t *testing.T) {
	dir := t.TempDir()

	// Create first store, add a job
	cs1, err := NewCronStore(dir)
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	job := makeTestJob("reload-test")
	if err := cs1.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Create second store from same directory
	cs2, err := NewCronStore(dir)
	if err != nil {
		t.Fatalf("NewCronStore (2nd): %v", err)
	}

	got := cs2.Get("reload-test")
	if got == nil {
		t.Fatal("Get returned nil after reload")
	}
	if got.Prompt != "Run tests" {
		t.Errorf("Prompt = %q, want %q", got.Prompt, "Run tests")
	}
}

func TestCronStore_EmptyDir(t *testing.T) {
	cs := newTestCronStore(t)

	// Store created from empty dir should work
	jobs := cs.List()
	if len(jobs) != 0 {
		t.Errorf("List len = %d, want 0", len(jobs))
	}
}

func TestCronStore_AtomicWrite_Format(t *testing.T) {
	cs := newTestCronStore(t)

	job := makeTestJob("format-test")
	if err := cs.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, err := os.ReadFile(cs.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	// Verify the file is properly formatted JSON
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}

	// Verify no tmp file remains
	tmpPath := cs.path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tmp file should not exist after successful write")
	}
}

// ---------------------------------------------------------------------------
// CronStore - default directory path
// ---------------------------------------------------------------------------

func TestCronStore_DefaultDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cs, err := NewCronStore("")
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}

	// Verify the path is under HOME/.hotplex/cron
	expectedDir := filepath.Join(tmpHome, ".hotplex", "cron")
	if !strings.HasPrefix(cs.path, expectedDir) {
		t.Errorf("path = %q, want prefix %q", cs.path, expectedDir)
	}

	// Verify the directory was created
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("default directory should be created")
	}
}

// ---------------------------------------------------------------------------
// CronStore - corrupted JSON file
// ---------------------------------------------------------------------------

func TestCronStore_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")

	// Write corrupted JSON
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	_, err := NewCronStore(dir)
	if err == nil {
		t.Fatal("expected error for corrupted JSON file")
	}
}

// ---------------------------------------------------------------------------
// CronStore - MkdirAll failure
// ---------------------------------------------------------------------------

func TestCronStore_MkdirAllFailure(t *testing.T) {
	// Use a path that is guaranteed to fail on any OS
	invalidDir := "\x00/invalid/path"

	_, err := NewCronStore(invalidDir)
	if err == nil {
		t.Fatal("expected error for invalid directory path")
	}
}

// ---------------------------------------------------------------------------
// CronStore - path returns expected value
// ---------------------------------------------------------------------------

func TestCronStore_Path(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewCronStore(dir)
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}

	expectedPath := filepath.Join(dir, "jobs.json")
	if cs.path != expectedPath {
		t.Errorf("path = %q, want %q", cs.path, expectedPath)
	}
}
