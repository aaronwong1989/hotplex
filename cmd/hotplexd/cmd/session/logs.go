package session

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	cmdpkg "github.com/hrygo/hotplex/cmd/hotplexd/cmd"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <session-id>",
	Short: "Show session log information or content",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	SessionCmd.AddCommand(logsCmd)
	logsCmd.Flags().Bool("stream", false, "Stream log content to stdout")
}

func runLogs(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	stream, _ := cmd.Flags().GetBool("stream")

	resp, err := cmdpkg.DoAdminAPI(cmd, http.MethodGet, "/admin/v1/sessions/"+sessionID+"/logs", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("log file not found for session: %s", sessionID)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var result struct {
		SessionID    string `json:"session_id"`
		LogPath      string `json:"log_path"`
		SizeBytes   int64  `json:"size_bytes"`
		LastModified string `json:"last_modified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Session ID:     %s\n", result.SessionID)
	fmt.Printf("Log Path:       %s\n", result.LogPath)
	fmt.Printf("Size:           %d bytes\n", result.SizeBytes)
	fmt.Printf("Last Modified:  %s\n", result.LastModified)

	if stream {
		file, err := os.Open(result.LogPath)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer func() { _ = file.Close() }()

		fmt.Fprintf(os.Stderr, "\n--- Log Content ---\n")
		if _, err := io.Copy(os.Stdout, file); err != nil {
			return fmt.Errorf("failed to read log content: %w", err)
		}
	}

	return nil
}
