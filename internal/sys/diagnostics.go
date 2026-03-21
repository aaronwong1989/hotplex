package sys

import (
	"context"
	"os/exec"
	"time"
)

// CheckCliResult holds the result of a CLI availability check.
type CheckCliResult struct {
	Available bool
	Version   string
}

// CheckCliAvailable checks if the Claude Code CLI is available and returns its version.
// It uses a 5-second timeout to avoid blocking.
func CheckCliAvailable() CheckCliResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude-code", "--version")
	out, err := cmd.Output()
	if err != nil {
		return CheckCliResult{Available: false, Version: "unknown"}
	}
	return CheckCliResult{Available: true, Version: string(out)}
}

// CheckDatabaseHealth checks if a SQLite database is accessible and returns latency.
// It uses a 5-second timeout to avoid blocking.
func CheckDatabaseHealth(dbPath string) (latencyMs int, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "sqlite3", dbPath, "SELECT 1;")
	if err := cmd.Run(); err != nil {
		return 0, false
	}

	return int(time.Since(start).Milliseconds()), true
}
