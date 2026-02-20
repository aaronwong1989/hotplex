package hotplex

import "errors"

// Standard Sentinel Errors for the HotPlex SDK
var (
	// ErrDangerBlocked indicates that a command or input was intercepted and blocked by the WAF (Detector).
	ErrDangerBlocked = errors.New("danger event blocked: input matched forbidden patterns")

	// ErrSessionTerminated indicates that the underlying CLI process was killed or unexpectedly exited.
	ErrSessionTerminated = errors.New("underlying session process terminated")

	// ErrContextCancelled indicates that the request execution was cancelled by the provided context.
	ErrContextCancelled = errors.New("execution context cancelled")

	// ErrInvalidConfig indicates that the provided configuration is invalid or missing required fields.
	ErrInvalidConfig = errors.New("invalid execution configuration")
)
