package provider

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// GuardianState represents the state of the process guardian.
type GuardianState int

const (
	GuardianStarting GuardianState = iota
	GuardianRunning
	GuardianRestarting
	GuardianStopped
	GuardianDead
)

// String returns a human-readable string for the guardian state.
func (s GuardianState) String() string {
	switch s {
	case GuardianStarting:
		return "starting"
	case GuardianRunning:
		return "running"
	case GuardianRestarting:
		return "restarting"
	case GuardianStopped:
		return "stopped"
	case GuardianDead:
		return "dead"
	default:
		return "unknown"
	}
}

// FailureEntry records a single process failure event.
type FailureEntry struct {
	Time       time.Time
	Reason     string
	Attempt    int
	RestoredAt time.Time
}

// ProcessGuardian manages the lifecycle of the opencode serve subprocess.
// It monitors the process health and automatically restarts it on failure
// using exponential backoff.
type ProcessGuardian struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	state    GuardianState
	binary   string
	args     []string
	workDir  string
	password string

	healthInterval  time.Duration
	startupTimeout  time.Duration
	backoff        []time.Duration
	attempt        int
	maxFailBurst   int
	failures       []FailureEntry
	failureIndex   int // Ring buffer index

	onStateChange func(GuardianState)
	onFailure    func(FailureEntry)
	transport    *HTTPTransport

	logger *slog.Logger
}

// NewProcessGuardian creates a new ProcessGuardian for the opencode serve process.
func NewProcessGuardian(binary string, args []string, password string, workDir string, transport *HTTPTransport, logger *slog.Logger) *ProcessGuardian {
	if logger == nil {
		logger = slog.Default()
	}

	// Default backoff: 1s → 2s → 4s → 8s → 16s → 30s (max)
	defaultBackoff := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
	}

	return &ProcessGuardian{
		binary:          binary,
		args:            args,
		password:        password,
		workDir:         workDir,
		transport:       transport,
		logger:          logger.With("component", "process_guardian"),
		healthInterval:  10 * time.Second,
		startupTimeout:  60 * time.Second,
		backoff:         defaultBackoff,
		attempt:         0,
		maxFailBurst:    1000,
		failures:        make([]FailureEntry, 100), // Ring buffer of 100
		failureIndex:    0,
	}
}

// SetStateChangeCallback sets the callback for state changes.
func (g *ProcessGuardian) SetStateChangeCallback(fn func(GuardianState)) {
	g.onStateChange = fn
}

// SetFailureCallback sets the callback for failure events.
func (g *ProcessGuardian) SetFailureCallback(fn func(FailureEntry)) {
	g.onFailure = fn
}

// Start starts the opencode serve subprocess and begins health monitoring.
func (g *ProcessGuardian) Start(ctx context.Context) error {
	g.mu.Lock()
	if g.state == GuardianRunning || g.state == GuardianStarting {
		g.mu.Unlock()
		return nil
	}

	g.setState(GuardianStarting)
	g.mu.Unlock()

	if err := g.startProcess(ctx); err != nil {
		g.mu.Lock()
		g.setState(GuardianDead)
		g.mu.Unlock()
		return fmt.Errorf("start process: %w", err)
	}

	// Wait for the process to become healthy
	if err := g.waitForHealthy(ctx); err != nil {
		g.mu.Lock()
		g.killProcessLocked()
		g.setState(GuardianDead)
		g.mu.Unlock()
		return fmt.Errorf("wait for healthy: %w", err)
	}

	g.mu.Lock()
	g.setState(GuardianRunning)
	g.attempt = 0 // Reset attempt counter on successful start
	g.mu.Unlock()

	// Start health check loop in background
	go g.healthCheckLoop(context.Background())

	return nil
}

// Stop gracefully stops the opencode serve subprocess.
func (g *ProcessGuardian) Stop(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state == GuardianStopped || g.state == GuardianDead {
		return nil
	}

	g.setState(GuardianStopped)
	g.killProcessLocked()
	return nil
}

// PID returns the process ID of the managed subprocess, or 0 if not running.
func (g *ProcessGuardian) PID() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cmd != nil && g.cmd.Process != nil {
		return g.cmd.Process.Pid
	}
	return 0
}

// State returns the current guardian state.
func (g *ProcessGuardian) State() GuardianState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state
}

// Failures returns a copy of recent failure entries.
func (g *ProcessGuardian) Failures() []FailureEntry {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Count actual entries
	count := 0
	for i := range g.failures {
		if !g.failures[i].Time.IsZero() {
			count++
		}
	}

	result := make([]FailureEntry, 0, count)
	for i := range g.failures {
		if !g.failures[i].Time.IsZero() {
			result = append(result, g.failures[i])
		}
	}
	return result
}

// startProcess forks and execs the opencode serve subprocess.
func (g *ProcessGuardian) startProcess(ctx context.Context) error {
	g.mu.Lock()
	cmd := exec.CommandContext(ctx, g.binary, g.args...)
	g.cmd = cmd
	g.mu.Unlock()

	g.logger.Info("Starting opencode serve process",
		"binary", g.binary,
		"args", g.args,
		"workdir", g.workDir)

	if g.workDir != "" {
		cmd.Dir = g.workDir
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cmd start: %w", err)
	}

	g.logger.Info("opencode serve process started", "pid", cmd.Process.Pid)
	return nil
}

// waitForHealthy waits for the process to become healthy (HTTP health check passes).
func (g *ProcessGuardian) waitForHealthy(ctx context.Context) error {
	deadline := time.Now().Add(g.startupTimeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := g.transport.Health(ctx); err != nil {
			g.logger.Debug("Health check failed during startup", "error", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		g.logger.Info("opencode serve is healthy")
		return nil
	}

	return fmt.Errorf("startup timeout exceeded (%v)", g.startupTimeout)
}

// healthCheckLoop continuously monitors process health and restarts on failure.
func (g *ProcessGuardian) healthCheckLoop(ctx context.Context) {
	g.logger.Debug("Health check loop started")

	for {
		select {
		case <-ctx.Done():
			g.logger.Debug("Health check loop stopped")
			return
		default:
		}

		time.Sleep(g.healthInterval)

		if err := g.transport.Health(ctx); err != nil {
			g.logger.Warn("Health check failed", "error", err)
			g.handleUnhealthy(ctx, err)
		}
	}
}

// handleUnhealthy handles an unhealthy state by restarting the process.
func (g *ProcessGuardian) handleUnhealthy(ctx context.Context, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Record failure
	g.recordFailure(err.Error())

	// Check max fail burst
	g.attempt++
	if g.attempt > g.maxFailBurst {
		g.logger.Error("Max fail burst exceeded, entering dead state", "attempt", g.attempt)
		g.setState(GuardianDead)
		// Still attempt restart even in dead state
	}

	// If already stopped or dead and not restarting, skip
	if g.state == GuardianStopped {
		return
	}

	oldState := g.state
	g.setState(GuardianRestarting)

	// Kill existing process
	g.killProcessLocked()

	// Calculate backoff delay
	backoffIdx := min(g.attempt-1, len(g.backoff)-1)
	delay := g.backoff[backoffIdx]

	g.logger.Info("Restarting opencode serve",
		"attempt", g.attempt,
		"delay", delay,
		"reason", err.Error())

	// Release lock during backoff sleep
	g.mu.Unlock()

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	g.mu.Lock()

	// Start new process
	startCtx, cancel := context.WithTimeout(context.Background(), g.startupTimeout)
	err = g.startProcess(startCtx)
	cancel()

	if err != nil {
		g.logger.Error("Failed to restart process", "error", err)
		g.setState(GuardianDead)
		return
	}

	// Wait for healthy
	waitCtx, cancel := context.WithTimeout(context.Background(), g.startupTimeout)
	healthyErr := g.waitForHealthy(waitCtx)
	cancel()

	if healthyErr != nil {
		g.logger.Error("Process restarted but not healthy", "error", healthyErr)
		g.killProcessLocked()
		g.setState(GuardianDead)
		return
	}

	// Success - update restorted timestamp of last failure
	g.mu.Unlock()
	g.mu.Lock()
	if len(g.failures) > 0 {
		for i := range g.failures {
			if !g.failures[i].Time.IsZero() && g.failures[i].RestoredAt.IsZero() {
				g.failures[i].RestoredAt = time.Now()
			}
		}
	}

	g.setState(GuardianRunning)
	g.attempt = 0
	g.logger.Info("opencode serve restored", "previous_state", oldState.String())
}

// killProcessLocked kills the subprocess. Caller must hold the lock.
func (g *ProcessGuardian) killProcessLocked() {
	if g.cmd != nil && g.cmd.Process != nil {
		g.logger.Info("Killing opencode serve process", "pid", g.cmd.Process.Pid)
		_ = g.cmd.Process.Kill()
		_ = g.cmd.Wait() // Release resources
		g.cmd = nil
	}
}

// setState updates the guardian state and calls callback if set.
func (g *ProcessGuardian) setState(state GuardianState) {
	g.state = state
	if g.onStateChange != nil {
		g.mu.Unlock()
		g.onStateChange(state)
		g.mu.Lock()
	}
}

// recordFailure records a failure entry in the ring buffer.
func (g *ProcessGuardian) recordFailure(reason string) {
	entry := FailureEntry{
		Time:    time.Now(),
		Reason:  reason,
		Attempt: g.attempt,
	}
	g.failures[g.failureIndex] = entry
	g.failureIndex = (g.failureIndex + 1) % len(g.failures)
}
