package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	RootCmd.AddCommand(configCmd)
	configCmd.AddCommand(validateConfigCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
}

var validateConfigCmd = &cobra.Command{
	Use:   "validate <config-path>",
	Short: "Validate a configuration file",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidateConfig,
}

func runValidateConfig(cmd *cobra.Command, args []string) error {
	configPath := args[0]

	// Local validation first
	localErr := validateConfigLocally(configPath)
	if localErr != nil {
		fmt.Printf("Local validation failed: %v\n", localErr)
		return localErr
	}
	fmt.Println("Local validation passed.")

	// Remote validation via admin API
	body := strings.NewReader(fmt.Sprintf(`{"config_path": "%s"}`, configPath))
	resp, err := DoAdminAPI(cmd, http.MethodPost, "/admin/v1/config/validate", body, "Content-Type", "application/json")
	if err != nil {
		return fmt.Errorf("remote validation failed (server unreachable): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("remote validation failed (invalid response): %w", err)
	}

	fmt.Println("Remote validation result:")
	if result.Valid {
		fmt.Println("  Valid: true")
	} else {
		fmt.Println("  Valid: false")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("config validation failed")
	}

	return nil
}

func validateConfigLocally(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	// Parse as YAML and check required top-level keys
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	for _, field := range []string{"server", "engine"} {
		if _, ok := m[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}
