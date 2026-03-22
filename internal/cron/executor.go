package cron

import (
	"context"
	"fmt"
	"time"

	intengine "github.com/hrygo/hotplex/internal/engine"
)

// ExecuteRequest is the input for a cron job execution.
type ExecuteRequest struct {
	Job       *CronJob
	SessionID string
	WorkDir   string
	Prompt    string
}

// ExecuteResult is the output of a cron job execution.
type ExecuteResult struct {
	Response string
	Error    error
	Duration time.Duration
}

// Executor runs cron jobs via the SessionManager.
type Executor struct {
	manager intengine.SessionManager
}

// NewExecutor creates an Executor backed by a SessionManager.
func NewExecutor(manager intengine.SessionManager) *Executor {
	return &Executor{manager: manager}
}

// Execute runs the job prompt via the SessionManager.
func (e *Executor) Execute(ctx context.Context, req *ExecuteRequest) *ExecuteResult {
	start := time.Now()

	cfg := intengine.SessionConfig{
		WorkDir:     req.WorkDir,
		IdleTimeout: 0, // cron sessions never expire during execution
		Namespace:   "cron",
	}

	session, _, err := e.manager.GetOrCreateSession(ctx, req.SessionID, cfg, req.Prompt)
	if err != nil {
		return &ExecuteResult{Error: fmt.Errorf("get or create session: %w", err), Duration: time.Since(start)}
	}

	// TODO: stream result from session (Phase 1.5)
	_ = session
	return &ExecuteResult{Duration: time.Since(start)}
}
