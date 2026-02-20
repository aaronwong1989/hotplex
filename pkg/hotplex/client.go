package hotplex

import "context"

// HotPlexClient defines the public API for the HotPlex engine.
// It abstracts the underlying process management and provides a clean interface for callers.
type HotPlexClient interface {
	// Execute runs a command or prompt within the HotPlex sandbox and streams events back via the Callback.
	Execute(ctx context.Context, cfg *Config, prompt string, callback Callback) error

	// Close gracefully terminates all managed sessions and releases system resources.
	Close() error

	// ConversationIDToSessionID converts a deterministic conversation ID to a unique Session UUID.
	ConversationIDToSessionID(conversationID int64) string
}
