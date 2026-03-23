package cron

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// CronScheduler manages scheduled jobs using robfig/cron/v3.
type CronScheduler struct {
	store         *CronStore
	runsStore     *RunsStore
	executor      *Executor
	cron          *cron.Cron
	logger        *slog.Logger
	mu            sync.Mutex
	running       bool
	entryMap      map[string]cron.EntryID
	sem           chan struct{}
	maxConcurrent int
}

// SetRunsStore injects the RunsStore after construction.
func (cs *CronScheduler) SetRunsStore(rs *RunsStore) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.runsStore = rs
}

// NewCronScheduler creates a scheduler backed by a store and executor.
// maxConcurrent limits how many jobs run simultaneously (default 4).
func NewCronScheduler(store *CronStore, executor *Executor, logger *slog.Logger, maxConcurrent int) *CronScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &CronScheduler{
		store:         store,
		executor:      executor,
		cron:          cron.New(cron.WithSeconds()),
		logger:        logger,
		entryMap:      make(map[string]cron.EntryID),
		sem:           make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// Start registers all enabled jobs and begins scheduling.
func (cs *CronScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.running {
		return nil
	}
	for _, job := range cs.store.List() {
		if !job.Enabled {
			continue
		}
		if err := cs.addJobLocked(job); err != nil {
			cs.logger.Warn("failed to register job on start", "job_id", job.ID, "error", err)
		}
	}
	cs.running = true
	cs.cron.Start()
	return nil
}

// Stop gracefully stops the scheduler.
func (cs *CronScheduler) Stop() context.Context {
	cs.mu.Lock()
	if !cs.running {
		cs.mu.Unlock()
		return context.Background()
	}
	cs.running = false
	cs.mu.Unlock()
	return cs.cron.Stop()
}

// AddJob registers a new job and starts scheduling it.
func (cs *CronScheduler) AddJob(job *CronJob) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if err := cs.store.Add(job); err != nil {
		return fmt.Errorf("store add: %w", err)
	}
	if job.Enabled {
		return cs.addJobLocked(job)
	}
	return nil
}

// RemoveJob stops and deletes a job.
func (cs *CronScheduler) RemoveJob(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if err := cs.store.Delete(id); err != nil {
		return fmt.Errorf("store delete: %w", err)
	}
	entryID, ok := cs.entryMap[id]
	if !ok {
		return fmt.Errorf("job %q not registered in scheduler", id)
	}
	cs.cron.Remove(entryID)
	delete(cs.entryMap, id)
	return nil
}

// PauseJob disables a job without removing it.
func (cs *CronScheduler) PauseJob(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	job := cs.store.Get(id)
	if job == nil {
		return fmt.Errorf("job %q not found", id)
	}
	job.Enabled = false
	if err := cs.store.Update(job); err != nil {
		return fmt.Errorf("store update: %w", err)
	}
	entryID, ok := cs.entryMap[id]
	if !ok {
		return fmt.Errorf("job %q not registered in scheduler", id)
	}
	cs.cron.Remove(entryID)
	delete(cs.entryMap, id)
	return nil
}

// ResumeJob re-enables a paused job.
func (cs *CronScheduler) ResumeJob(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	job := cs.store.Get(id)
	if job == nil {
		return fmt.Errorf("job %q not found", id)
	}
	job.Enabled = true
	if err := cs.store.Update(job); err != nil {
		return fmt.Errorf("store update: %w", err)
	}
	return cs.addJobLocked(job)
}

// addJobLocked registers a job with the cron parser. Caller holds cs.mu.
func (cs *CronScheduler) addJobLocked(job *CronJob) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	spec, err := parser.Parse(job.CronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expr %q: %w", job.CronExpr, err)
	}
	job.NextRun = spec.Next(time.Now())

	entryID, err := cs.cron.AddFunc(job.CronExpr, func() {
		cs.executeJob(context.Background(), job.ID)
	})
	if err != nil {
		return fmt.Errorf("add cron func: %w", err)
	}
	cs.entryMap[job.ID] = entryID
	cs.logger.Info("cron job registered",
		"job_id", job.ID,
		"cron_expr", job.CronExpr,
		"next_run", job.NextRun)
	return nil
}

// executeJob runs a cron job and records the result.
func (cs *CronScheduler) executeJob(ctx context.Context, jobID string) {
	// Check context before acquiring slot
	if err := ctx.Err(); err != nil {
		cs.logger.Warn("executeJob: context already cancelled", "job_id", jobID, "error", err)
		return
	}

	// Acquire concurrency slot; release when done.
	select {
	case cs.sem <- struct{}{}:
		defer func() { <-cs.sem }()
	case <-ctx.Done():
		cs.logger.Warn("executeJob: context cancelled during slot acquisition", "job_id", jobID)
		return
	}

	job := cs.store.Get(jobID)
	if job == nil {
		cs.logger.Warn("executeJob: job not found", "job_id", jobID)
		return
	}

	sessionID := fmt.Sprintf("cron-%s", jobID)
	timeout := time.Duration(job.TimeoutMins) * time.Minute
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	run := &CronRun{
		ID:        uuid.New().String(),
		JobID:     jobID,
		StartedAt: time.Now(),
	}

	// Wrap executor call with retry using exponential backoff.
	var result *ExecuteResult
	retryCount := 0
	execFn := func() error {
		r := cs.executor.Execute(ctx, &ExecuteRequest{
			Job:       job,
			SessionID: sessionID,
			WorkDir:   job.WorkDir,
			Prompt:    job.Prompt,
		})
		result = r
		if r.Error != nil {
			retryCount++
		}
		return r.Error
	}

	if err := cs.retryWithBackoff(ctx, defaultRetryDelays, execFn); err != nil {
		cs.logger.Warn("all retry attempts exhausted", "job_id", jobID, "error", err)
	}
	run.RetryCount = retryCount

	run.Duration = time.Since(run.StartedAt)
	if result != nil && result.Error != nil {
		run.Status = string(EventFailed)
		run.Error = result.Error.Error()
	} else if result != nil {
		run.Status = string(EventCompleted)
		run.Response = cs.truncateOutput(result.Response)
	} else {
		run.Status = string(EventFailed)
		run.Error = "execution produced no result"
	}

	// Lock to protect job field updates (job is a shared pointer from store.Get)
	cs.mu.Lock()
	job.RunCount++
	job.LastRun = run.StartedAt
	if run.Error != "" {
		job.LastError = run.Error
	}
	if next := cs.nextRun(job); !next.IsZero() {
		job.NextRun = next
	}

	if err := cs.store.Update(job); err != nil {
		cs.logger.Warn("update job after execution", "job_id", jobID, "error", err)
	}
	cs.mu.Unlock()

	cs.addRun(run)
	cs.logger.Info("cron job completed",
		"job_id", jobID,
		"status", run.Status,
		"duration", run.Duration)
}

func (cs *CronScheduler) nextRun(job *CronJob) time.Time {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	spec, err := parser.Parse(job.CronExpr)
	if err != nil {
		return time.Time{}
	}
	return spec.Next(time.Now())
}

// defaultRetryDelays are the default exponential backoff intervals: 1s → 2s → 4s.
var defaultRetryDelays = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

const maxOutputBytes = 4096

// retryWithBackoff executes fn immediately, then retries on failure with exponential
// backoff. It respects context cancellation and retries up to len(delays) times.
func (cs *CronScheduler) retryWithBackoff(ctx context.Context, delays []time.Duration, fn func() error) error {
	// First attempt: no delay
	if err := fn(); err != nil {
		for i, delay := range delays {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			if serr := fn(); serr != nil {
				cs.logger.Warn("retry attempt failed", "attempt", i+2, "error", serr)
				continue
			}
			return nil
		}
		return fmt.Errorf("all %d retry attempts exhausted", len(delays))
	}
	return nil
}

// truncateOutput truncates response to maxOutputBytes and logs if truncation occurred.
func (cs *CronScheduler) truncateOutput(response string) string {
	if len(response) <= maxOutputBytes {
		return response
	}
	cs.logger.Warn("job output truncated", "original_bytes", len(response), "max_bytes", maxOutputBytes)
	return response[:maxOutputBytes]
}

// addRun persists the run and fires webhook callbacks based on run status.
func (cs *CronScheduler) addRun(run *CronRun) {
	run.FinishedAt = time.Now()

	// Persist to RunsStore
	cs.mu.Lock()
	rs := cs.runsStore
	cs.mu.Unlock()
	if rs != nil {
		if err := rs.AddRun(run); err != nil {
			cs.logger.Warn("failed to persist run", "run_id", run.ID, "error", err)
		}
	}

	// Fire webhook callbacks
	cs.fireCallbacks(run)
}

// fireCallbacks invokes webhook callbacks based on run status and job config.
func (cs *CronScheduler) fireCallbacks(run *CronRun) {
	job := cs.store.Get(run.JobID)
	if job == nil {
		return
	}

	fire := func(urlStr string, run *CronRun) {
		if urlStr == "" {
			return
		}
		u, err := url.Parse(urlStr)
		if err != nil {
			cs.logger.Warn("invalid callback URL", "url", urlStr, "error", err)
			return
		}
		wc := &WebhookCallback{URL: u.String()}
		if err := wc.send(run); err != nil {
			cs.logger.Warn("callback failed", "url", urlStr, "run_id", run.ID, "error", err)
		} else {
			cs.logger.Info("callback fired", "url", urlStr, "run_id", run.ID, "status", run.Status)
		}
	}

	if run.Status == string(EventCompleted) && job.OnComplete != "" {
		fire(job.OnComplete, run)
	}
	if run.Status == string(EventFailed) && job.OnFail != "" {
		fire(job.OnFail, run)
	}
}

// ListJobs returns all jobs from the store.
func (cs *CronScheduler) ListJobs() []*CronJob {
	return cs.store.List()
}

// GetJob returns a job by ID, or nil if not found.
func (cs *CronScheduler) GetJob(id string) *CronJob {
	return cs.store.Get(id)
}

// ListRuns returns all runs for a job from the RunsStore.
func (cs *CronScheduler) ListRuns(jobID string) []*CronRun {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.runsStore == nil {
		return nil
	}
	return cs.runsStore.GetRuns(jobID)
}
