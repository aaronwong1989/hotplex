package session

import (
	"github.com/spf13/cobra"
)

// SessionCmd is the parent command for session subcommands.
var SessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session management commands",
}
