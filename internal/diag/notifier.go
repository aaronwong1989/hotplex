package diag

import (
	"context"
	"log/slog"
)

// Notifier forwards diagnostic results to user via chat platform.
type Notifier struct {
	logger *slog.Logger
}

// NewNotifier creates a new Notifier.
func NewNotifier(logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{logger: logger}
}

// NotifyResult represents the result from Provider analysis.
type NotifyResult struct {
	Analysis     string // Provider's analysis text
	IssueCreated bool   // Whether Provider created an issue
	IssueURL     string // URL of created issue (if any)
}

// Notify sends the diagnostic result to the user via chat platform.
// This is a placeholder - actual implementation would use ChatAdapter.
func (n *Notifier) Notify(ctx context.Context, platform, channelID, threadID string, result *NotifyResult) error {
	n.logger.Info("Sending diagnostic result",
		"platform", platform,
		"channel", channelID,
		"issue_created", result.IssueCreated)

	// TODO: Implement actual notification via ChatAdapter
	// For now, just log the result
	if result.IssueCreated && result.IssueURL != "" {
		n.logger.Info("Issue created", "url", result.IssueURL)
	}

	return nil
}
