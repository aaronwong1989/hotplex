package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show runtime status overview",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	resp, err := DoAdminAPI(cmd, http.MethodGet, "/admin/v1/stats")
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w (is the daemon running?)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var stats statsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println("HotPlex Daemon Status")
	fmt.Println("=====================")
	fmt.Printf("Total Sessions:   %d\n", stats.TotalSessions)
	fmt.Printf("Active Sessions:  %d\n", stats.ActiveSessions)
	fmt.Printf("Stopped Sessions: %d\n", stats.StoppedSessions)
	fmt.Printf("Uptime:           %s\n", stats.Uptime)
	fmt.Printf("Memory Usage:     %.2f MB\n", stats.MemoryUsageMB)
	fmt.Printf("CPU Usage:        %.2f%%\n", stats.CpuUsagePercent)

	return nil
}

type statsResponse struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	StoppedSessions int     `json:"stopped_sessions"`
	Uptime          string  `json:"uptime"`
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	CpuUsagePercent float64 `json:"cpu_usage_percent"`
}
