package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex"
	"github.com/hrygo/hotplex/event"
)

// ExecutionController orchestrates engine executions for all protocol handlers.
// It resolves DRY violations by centralizing context timeouts, Config building,
// and engine invocation.
type ExecutionController struct {
	engine hotplex.HotPlexClient
	logger *slog.Logger
}

// NewExecutionController creates a new ExecutionController.
func NewExecutionController(engine hotplex.HotPlexClient, logger *slog.Logger) *ExecutionController {
	return &ExecutionController{
		engine: engine,
		logger: logger,
	}
}

// ExecutionRequest represents a protocol-agnostic execution request.
type ExecutionRequest struct {
	SessionID    string
	Prompt       string
	Instructions string
	WorkDir      string
	Timeout      time.Duration
}

// Execute orchestrates the engine execution.
func (c *ExecutionController) Execute(ctx context.Context, req ExecutionRequest, cb event.Callback) error {
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	workDir := req.WorkDir
	if workDir == "" {
		workDir = "/tmp/hotplex_sandbox" // Default fallback
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = 15 * time.Minute
	}

	taskCtx, taskCancel := context.WithTimeout(ctx, timeout)
	defer taskCancel()

	cfg := &hotplex.Config{
		SessionID:        sessionID,
		WorkDir:          workDir,
		TaskInstructions: req.Instructions,
	}

	c.logger.Info("Controller: starting engine execution", "session_id", sessionID)

	err := c.engine.Execute(taskCtx, cfg, req.Prompt, cb)
	if err != nil {
		if taskCtx.Err() == nil {
			c.logger.Error("Controller: execution failed", "session_id", sessionID, "error", err)
		} else {
			c.logger.Info("Controller: execution cancelled or timed out", "session_id", sessionID, "reason", taskCtx.Err())
		}
		return err
	}

	c.logger.Info("Controller: execution completed successfully", "session_id", sessionID)
	return nil
}
