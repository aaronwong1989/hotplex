package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// RunsStore persists CronRun records to runs.json with a per-job retention limit.
type RunsStore struct {
	path  string
	mu    sync.Mutex
	runs  map[string][]*CronRun // jobID → sorted runs (newest first)
	limit int
}

// runsFile is the on-disk JSON format with a version header.
type runsFile struct {
	Version int        `json:"version"`
	Runs    []*CronRun `json:"runs"` // flat list; JobID field identifies the job
}

const runsSchemaVersion = 1

const defaultRunsLimit = 100

// NewRunsStore loads or creates a RunsStore at dataDir/runs.json.
func NewRunsStore(dataDir string) (*RunsStore, error) {
	if dataDir == "" {
		dataDir = filepath.Join(os.Getenv("HOME"), ".hotplex", "cron")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cron data dir: %w", err)
	}
	path := filepath.Join(dataDir, "runs.json")
	rs := &RunsStore{path: path, runs: make(map[string][]*CronRun), limit: defaultRunsLimit}
	if err := rs.load(); err != nil {
		return nil, fmt.Errorf("load runs: %w", err)
	}
	return rs, nil
}

// AddRun appends a run to the job's history, trims to the per-job limit, and persists.
func (rs *RunsStore) AddRun(run *CronRun) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	jobRuns := rs.runs[run.JobID]
	if jobRuns == nil {
		jobRuns = make([]*CronRun, 0)
	}
	jobRuns = append(jobRuns, run)

	// Trim to limit (keep newest)
	if len(jobRuns) > rs.limit {
		jobRuns = jobRuns[len(jobRuns)-rs.limit:]
	}

	// Sort newest first
	sort.Slice(jobRuns, func(i, j int) bool {
		return jobRuns[i].StartedAt.After(jobRuns[j].StartedAt)
	})

	rs.runs[run.JobID] = jobRuns
	return rs.atomicWriteLocked()
}

// GetRuns returns all runs for a job, newest first.
func (rs *RunsStore) GetRuns(jobID string) []*CronRun {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	runs := rs.runs[jobID]
	result := make([]*CronRun, len(runs))
	copy(result, runs)
	return result
}

// atomicWriteLocked writes atomically using a temp file and Rename.
// Caller must hold rs.mu.
func (rs *RunsStore) atomicWriteLocked() error {
	var allRuns []*CronRun
	for _, jobRuns := range rs.runs {
		allRuns = append(allRuns, jobRuns...)
	}
	data := runsFile{Version: runsSchemaVersion, Runs: allRuns}
	return rs.atomicWrite(data)
}

func (rs *RunsStore) atomicWrite(data runsFile) error {
	tmp := rs.path + ".tmp"
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
	if err := os.Rename(tmp, rs.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

// load reads runs.json and indexes entries by JobID.
func (rs *RunsStore) load() error {
	rs.runs = make(map[string][]*CronRun)
	f, err := os.Open(rs.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open runs.json: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	var rf runsFile
	if err := json.NewDecoder(f).Decode(&rf); err != nil {
		return fmt.Errorf("decode runs.json: %w", err)
	}
	// Schema migration: when rf.Version < runsSchemaVersion, apply migrations here

	for _, run := range rf.Runs {
		rs.runs[run.JobID] = append(rs.runs[run.JobID], run)
	}

	// Sort each job's runs newest first
	for _, jobRuns := range rs.runs {
		sort.Slice(jobRuns, func(i, j int) bool {
			return jobRuns[i].StartedAt.After(jobRuns[j].StartedAt)
		})
	}
	return nil
}
