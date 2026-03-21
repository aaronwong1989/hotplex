package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	cmdpkg "github.com/hrygo/hotplex/cmd/hotplexd/cmd"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill <session-id>",
	Short: "Terminate an active session",
	Args:  cobra.ExactArgs(1),
	RunE:  runKill,
}

func init() {
	SessionCmd.AddCommand(killCmd)
}

func runKill(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	resp, err := cmdpkg.DoAdminAPI(cmd, http.MethodDelete, "/admin/v1/sessions/"+sessionID, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if resp.StatusCode != http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, result.Message)
		}
		return fmt.Errorf("server error (%d)", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Success {
		fmt.Println(result.Message)
	} else {
		return errors.New(result.Message)
	}

	return nil
}
