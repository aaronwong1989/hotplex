package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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
	serverURL, _ := cmd.Flags().GetString("server-url")
	token, _ := cmd.Flags().GetString("admin-token")
	if token == "" {
		token = os.Getenv("HOTPLEX_ADMIN_TOKEN")
	}

	// Local validation first
	if err := validateConfigLocally(configPath); err != nil {
		fmt.Printf("Local validation failed: %v\n", err)
	} else {
		fmt.Println("Local validation passed.")
	}

	// Remote validation via admin API
	client := &http.Client{Timeout: 10 * time.Second}
	url := serverURL + "/admin/v1/config/validate"

	body := strings.NewReader(fmt.Sprintf(`{"config_path": "%s"}`, configPath))
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Remote validation skipped: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Remote validation skipped: failed to parse response\n")
		return nil
	}

	fmt.Println("Remote validation result:")
	if result.Valid {
		fmt.Println("  Valid: true")
	} else {
		fmt.Println("  Valid: false")
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	return nil
}

func validateConfigLocally(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	// Try to parse as YAML
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	return nil
}
