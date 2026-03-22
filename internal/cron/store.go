package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CronStore persists CronJobs to disk using atomic writes (Mutex + os.Rename).
type CronStore struct {
	path string
	mu   sync.Mutex
	jobs map[string]*CronJob
}

// cronJobsFile is the on-disk JSON format with a version header.
type cronJobsFile struct {
	Version int        `json:"version"`
	Jobs    []*CronJob `json:"jobs"`
}

const cronJobsSchemaVersion = 1

// NewCronStore loads or creates a CronStore at the given path.
func NewCronStore(dataDir string) (*CronStore, error) {
	if dataDir == "" {
		dataDir = filepath.Join(os.Getenv("HOME"), ".hotplex", "cron")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cron data dir: %w", err)
	}
	path := filepath.Join(dataDir, "jobs.json")
	cs := &CronStore{path: path, jobs: make(map[string]*CronJob)}
	if err := cs.load(); err != nil {
		return nil, fmt.Errorf("load cron jobs: %w", err)
	}
	return cs, nil
}

// Get returns a job by ID, or nil if not found.
func (cs *CronStore) Get(id string) *CronJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.jobs[id]
}

// Add creates and persists a new CronJob with a UUID v4 ID.
func (cs *CronStore) Add(job *CronJob) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = nowFunc()
	}
	cs.jobs[job.ID] = job
	return cs.atomicWriteLocked()
}

// Update persists changes to an existing job.
func (cs *CronStore) Update(job *CronJob) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if _, ok := cs.jobs[job.ID]; !ok {
		return fmt.Errorf("job %q not found", job.ID)
	}
	cs.jobs[job.ID] = job
	return cs.atomicWriteLocked()
}

// Delete removes a job by ID.
func (cs *CronStore) Delete(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if _, ok := cs.jobs[id]; !ok {
		return fmt.Errorf("job %q not found", id)
	}
	delete(cs.jobs, id)
	return cs.atomicWriteLocked()
}

// List returns a snapshot of all jobs.
func (cs *CronStore) List() []*CronJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	jobs := make([]*CronJob, 0, len(cs.jobs))
	for _, j := range cs.jobs {
		jobs = append(jobs, j.Clone())
	}
	return jobs
}

// atomicWriteLocked writes atomically using a temp file and Rename.
// Caller must hold cs.mu.
func (cs *CronStore) atomicWriteLocked() error {
	jobs := make([]*CronJob, 0, len(cs.jobs))
	for _, j := range cs.jobs {
		jobs = append(jobs, j)
	}
	data := cronJobsFile{Version: cronJobsSchemaVersion, Jobs: jobs}
	return cs.atomicWrite(data)
}

func (cs *CronStore) atomicWrite(data cronJobsFile) error {
	tmp := cs.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("encode json: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close tmp file: %w", err)
	}
	if err := os.Rename(tmp, cs.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

// load reads jobs.json, applying migrations if needed.
func (cs *CronStore) load() error {
	cs.jobs = make(map[string]*CronJob)
	f, err := os.Open(cs.path)
	if os.IsNotExist(err) {
		return nil // new file, no-op
	}
	if err != nil {
		return fmt.Errorf("open jobs.json: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	var cf cronJobsFile
	if err := json.NewDecoder(f).Decode(&cf); err != nil {
		return fmt.Errorf("decode jobs.json: %w", err)
	}
	// Schema migration: when cf.Version < cronJobsSchemaVersion, apply migrations here
	for _, j := range cf.Jobs {
		cs.jobs[j.ID] = j
	}
	return nil
}

// nowFunc is injectable for testing.
var nowFunc = func() time.Time { return time.Now() }
