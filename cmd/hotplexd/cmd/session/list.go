package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	cmdpkg "github.com/hrygo/hotplex/cmd/hotplexd/cmd"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active sessions",
	RunE:  runList,
}

func init() {
	SessionCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	resp, err := cmdpkg.DoAdminAPI(cmd, http.MethodGet, "/admin/v1/sessions")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var result struct {
		Sessions []struct {
			ID         string    `json:"id"`
			Status     string    `json:"status"`
			CreatedAt  time.Time `json:"created_at"`
			LastActive time.Time `json:"last_active"`
		} `json:"sessions"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "SESSION ID\tSTATUS\tCREATED\tLAST ACTIVE\n")
	for _, s := range result.Sessions {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			s.ID, s.Status,
			s.CreatedAt.Format("2006-01-02 15:04"),
			s.LastActive.Format("2006-01-02 15:04"))
	}
	_ = w.Flush()
	fmt.Fprintf(os.Stderr, "\nTotal: %d sessions\n", result.Total)

	return nil
}
