package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// adminClient is shared across all admin API calls for connection pooling.
var adminClient = &http.Client{Timeout: 10 * time.Second}

// DoAdminAPI creates an authenticated HTTP request to the admin API.
// It reads --server-url and --admin-token flags, falling back to HOTPLEX_ADMIN_TOKEN env var.
// The body parameter supports POST requests; pass nil for GET.
// headers are optional extra headers (e.g., Content-Type).
func DoAdminAPI(cmd *cobra.Command, method, path string, body io.Reader, headers ...string) (*http.Response, error) {
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

	req, err := http.NewRequest(method, serverURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := adminClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return resp, nil
}
