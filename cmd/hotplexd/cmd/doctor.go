package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hrygo/hotplex/internal/sys"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	RootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostic checks for common issues",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Running HotPlex diagnostic checks...")
	fmt.Println()

	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"CLI Binary (claude-code)", checkCliBinary},
		{"Configuration Files", checkConfigFiles},
		{"Environment Variables", checkEnvVars},
		{"Port Availability (8080)", checkPortAvailable("8080")},
		{"Admin API Health (9080)", checkAdminAPIHealth},
		{"Database (SQLite)", checkDatabase},
	}

	allPassed := true
	for _, check := range checks {
		passed, detail := check.fn()
		status := "✓ PASS"
		if !passed {
			status = "✗ FAIL"
			allPassed = false
		}
		fmt.Printf("[%s] %s\n", status, check.name)
		if detail != "" {
			fmt.Printf("       %s\n", detail)
		}
		fmt.Println()
	}

	if allPassed {
		fmt.Println("All checks passed!")
		return nil
	}

	fmt.Println("Some checks failed. Please review the output above.")
	return fmt.Errorf("diagnostic checks failed")
}

func checkCliBinary() (bool, string) {
	result := sys.CheckCliAvailable()
	if !result.Available {
		return false, "claude-code not found in PATH"
	}
	return true, "Version: " + strings.TrimSpace(result.Version)
}

func checkConfigFiles() (bool, string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, "cannot determine home directory"
	}

	candidates := []string{
		filepath.Join(homeDir, ".hotplex", "config.yaml"),
		"configs/server.yaml",
		"./config.yaml",
	}

	for _, path := range candidates {
		// Try to read and parse YAML directly; skip if file inaccessible or invalid
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var m map[string]any
		if err := yaml.Unmarshal(data, &m); err != nil {
			return false, fmt.Sprintf("invalid YAML in %s: %v", path, err)
		}

		return true, "Found: " + path
	}

	return false, "no configuration file found"
}

func checkEnvVars() (bool, string) {
	required := []string{"HOTPLEX_PROJECTS_DIR"}
	var missing []string

	for _, env := range required {
		if os.Getenv(env) == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		return false, "missing: " + strings.Join(missing, ", ")
	}
	return true, "all required variables set"
}

func checkPortAvailable(port string) func() (bool, string) {
	return func() (bool, string) {
		// Simple check using netcat-like approach
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		cmd := exec.CommandContext(ctx, "nc", "-z", "localhost", port)
		if err := cmd.Run(); err != nil {
			// Connection refused → port is available
			return true, "port " + port + " is available"
		}
		// Connection succeeded → port is in use
		return false, "port " + port + " is in use (daemon running?)"
	}
}

func checkDatabase() (bool, string) {
	dbPath := os.Getenv("HOTPLEX_MESSAGE_STORE_SQLITE_PATH")
	if dbPath == "" {
		return true, "database not configured (optional)"
	}

	_, ok := sys.CheckDatabaseHealth(dbPath)
	if !ok {
		return false, "database check failed"
	}

	return true, "database accessible"
}

// checkAdminAPIHealth verifies the Admin API server is responding.
// Unlike checkPortAvailable which only tests port reachability, this makes
// an actual HTTP request to confirm the Admin API is operational.
func checkAdminAPIHealth() (bool, string) {
	adminPort := os.Getenv("HOTPLEX_ADMIN_PORT")
	if adminPort == "" {
		adminPort = "9080"
	}

	url := "http://localhost:" + adminPort + "/admin/v1/health"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, "failed to create request"
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Provide actionable diagnostics based on error type
		if strings.Contains(err.Error(), "connection refused") {
			return false, "connection refused — is the daemon running? (hotplexd start)"
		}
		if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "timeout") {
			return false, "timeout — daemon may still be starting, try again shortly"
		}
		return false, "request failed: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return false, fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return true, "Admin API is healthy"
}
