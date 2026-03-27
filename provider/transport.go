package provider

import (
	"context"
)

// Transport defines the low-level communication interface with an agent backend.
// It is responsible for HTTP or other network protocol details, exposing a simple
// request/response interface and an SSE event stream to its callers.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Transport interface {
	// Connect establishes the connection to the agent server.
	// It is called once during transport initialization.
	Connect(ctx context.Context, cfg TransportConfig) error

	// Send delivers a message to an existing session.
	Send(ctx context.Context, sessionID string, message map[string]any) error

	// Events returns a channel that emits raw SSE event payloads (JSON strings).
	// The channel is closed when the transport is closed.
	// Multiple goroutines may read from the channel concurrently.
	Events() <-chan string

	// CreateSession creates a new session on the server and returns its ID.
	CreateSession(ctx context.Context, title string) (string, error)

	// DeleteSession terminates a session on the server.
	DeleteSession(ctx context.Context, sessionID string) error

	// RespondPermission sends a permission response to the server.
	RespondPermission(ctx context.Context, sessionID, permissionID, response string) error

	// Health checks if the server is reachable.
	Health(ctx context.Context) error

	// Close releases all resources held by the transport.
	// Close is idempotent and goroutine-safe.
	Close() error
}

// TransportConfig contains configuration for establishing a transport connection.
type TransportConfig struct {
	// Endpoint is the base URL of the agent server (e.g., "http://127.0.0.1:4096").
	Endpoint string

	// Env contains environment variables to pass to the server.
	Env map[string]string

	// WorkDir is the working directory for the session.
	WorkDir string

	// Password is the Basic Auth password for the server.
	Password string
}
