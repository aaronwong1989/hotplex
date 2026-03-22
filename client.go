package hotplex

import (
	"context"

	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/internal/cron"
	"github.com/hrygo/hotplex/internal/relay"
	"github.com/hrygo/hotplex/types"
)

// HotPlexClient defines the comprehensive public API for the HotPlex engine.
// It integrates execution, session management, safety configuration, cron scheduling,
// and bot-to-bot relay.
type HotPlexClient interface {
	Executor
	SessionController
	SafetyManager
	CronManager
	RelayManager

	// Close gracefully terminates all managed sessions and releases resources.
	Close() error
}

// Executor handles the core execution logic and configuration validation.
type Executor interface {
	// Execute runs a command or prompt and streams normalized events.
	Execute(ctx context.Context, cfg *types.Config, prompt string, callback event.Callback) error

	// ValidateConfig checks if the session configuration is secure and valid.
	ValidateConfig(cfg *types.Config) error
}

// SessionController provides administrative control over persistent sessions.
type SessionController interface {
	// GetSessionStats returns telemetry and token usage for the given sessionID.
	// Note: Use the business-side sessionID provided during execution, not the internal
	// CLI-level session identifier. This sessionID maps to a specific background process.
	GetSessionStats(sessionID string) *SessionStats

	// StopSession forcibly terminates a persistent session and its underlying OS process group.
	// Note: Use the business-side sessionID (provided by the user) to identify which
	// specific agent instance to terminate.
	StopSession(sessionID string, reason string) error

	// GetCLIVersion returns the version string of the underlying AI CLI tool.
	GetCLIVersion() (string, error)
}

// SafetyManager controls the security boundaries and WAF settings.
type SafetyManager interface {
	// SetDangerAllowPaths configures the whitelist of safe directories for file I/O.
	SetDangerAllowPaths(paths []string)

	// SetDangerBypassEnabled toggles the regex WAF (requires valid admin token).
	SetDangerBypassEnabled(token string, enabled bool) error
}

// CronManager defines the cron scheduling API for the HotPlex engine.
type CronManager interface {
	// AddCronJob registers a new cron job.
	AddCronJob(ctx context.Context, job *CronJob) error
	// DeleteCronJob removes a cron job by ID.
	DeleteCronJob(ctx context.Context, id string) error
	// PauseCronJob suspends a cron job without removing it.
	PauseCronJob(ctx context.Context, id string) error
	// ResumeCronJob re-activates a paused cron job.
	ResumeCronJob(ctx context.Context, id string) error
	// ListCronJobs returns all registered cron jobs.
	ListCronJobs(ctx context.Context) ([]*CronJob, error)
	// GetCronRuns returns the run history for a specific job.
	GetCronRuns(ctx context.Context, jobID string) ([]*CronRun, error)
}

// RelayManager defines the bot-to-bot relay API for the HotPlex engine.
type RelayManager interface {
	// SendRelay delivers a message to a named agent via the relay network.
	SendRelay(ctx context.Context, to string, content string) (*RelayResponse, error)
	// AddRelayBinding registers a chat-to-bots binding for relay routing.
	AddRelayBinding(ctx context.Context, binding *RelayBinding) error
	// RemoveRelayBinding deletes a relay binding by ChatID.
	RemoveRelayBinding(ctx context.Context, chatID string) error
	// ListRelayBindings returns all registered relay bindings.
	ListRelayBindings(ctx context.Context) ([]*RelayBinding, error)
}

// CronJob is an alias for the internal cron job type.
type CronJob = cron.CronJob

// CronRun is an alias for the internal cron run record type.
type CronRun = cron.CronRun

// RelayBinding is an alias for the relay binding type.
type RelayBinding = relay.RelayBinding

// RelayResponse is an alias for the relay response type.
type RelayResponse = relay.RelayResponse
