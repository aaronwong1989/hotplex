package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version, Commit, BuildTime are set via ldflags
var (
	Version   = "v0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// RootCmd is the root command for hotplexd CLI.
var RootCmd = &cobra.Command{
	Use:   "hotplexd",
	Short: "HotPlex AI Agent Runtime Daemon",
	Long: `HotPlex is an AI Agent Runtime Engine that provides long-lived sessions
for Claude Code and OpenCode CLI tools.

Supports multiple chat platforms (Slack, Feishu, DingTalk, etc.)
and provides session management, diagnostics, and admin APIs.`,
	Version: Version,
}

// Execute runs the root command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().String("admin-token", "", "Admin API authentication token (also via HOTPLEX_ADMIN_TOKEN)")
	RootCmd.PersistentFlags().String("server-url", "http://localhost:8081", "Admin server URL")
}
