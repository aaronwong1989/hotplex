package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("HotPlex Daemon %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Build time: %s\n", BuildTime)
	},
}
