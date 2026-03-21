package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
		{"Port Availability (8081)", checkPortAvailable("8081")},
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude-code", "--version")
	out, err := cmd.Output()
	if err != nil {
		return false, "claude-code not found in PATH"
	}

	version := strings.TrimSpace(string(out))
	return true, "Version: " + version
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
		if _, err := os.Stat(path); err == nil {
			// Try to parse YAML
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var m map[string]interface{}
			if err := yaml.Unmarshal(data, &m); err != nil {
				return false, fmt.Sprintf("invalid YAML in %s: %v", path, err)
			}

			return true, "Found: " + path
		}
	}

	return false, "no configuration file found"
}

func checkEnvVars() (bool, string) {
	required := []string{"HOTPLEX_PROJECTS_DIR"}
	missing := []string{}

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
			return true, "port " + port + " is available"
		}
		return true, "port " + port + " is in use (daemon running?)"
	}
}

func checkDatabase() (bool, string) {
	dbPath := os.Getenv("HOTPLEX_MESSAGE_STORE_SQLITE_PATH")
	if dbPath == "" {
		return true, "database not configured (optional)"
	}

	// Check if sqlite3 is available
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sqlite3", dbPath, "SELECT 1;")
	if err := cmd.Run(); err != nil {
		return false, "database check failed: " + err.Error()
	}

	return true, "database accessible"
}
