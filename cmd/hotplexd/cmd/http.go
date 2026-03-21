package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// DoAdminAPI creates an authenticated HTTP request to the admin API.
// It reads --admin-token flag and HOTPLEX_ADMIN_TOKEN env var for auth,
// and --server-url flag for the base URL.
func DoAdminAPI(cmd *cobra.Command, method, path string) (*http.Response, error) {
	serverURL, err := cmd.Flags().GetString("server-url")
	if err != nil {
		return nil, fmt.Errorf("invalid --server-url flag: %w", err)
	}
	token, err := cmd.Flags().GetString("admin-token")
	if err != nil {
		return nil, fmt.Errorf("invalid --admin-token flag: %w", err)
	}
	if token == "" {
		token = os.Getenv("HOTPLEX_ADMIN_TOKEN")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(method, serverURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return resp, nil
}
