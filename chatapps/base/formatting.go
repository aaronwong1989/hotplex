// Package base provides shared utilities for chat application adapters.
package base

import (
	"fmt"
	"time"
)

// FormatTokenCount formats token count in compact form (1.2K, 1.00M).
// Uses proper threshold: K for < 1M, M for >= 1M.
//
// Formatting rules:
// - < 1K: display as integer (e.g., 500)
// - 1K to < 100K: display with 1 decimal (e.g., 1.5K, 99.9K)
// - 100K to < 1M: display as integer K (e.g., 100K, 999K)
// - >= 1M: display with 2 decimals (e.g., 1.00M, 1.23M)
//
// Special handling for 999.5K edge case: rounds to 1.00M to avoid confusion
func FormatTokenCount(count int64) string {
	if count >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(count)/1000000)
	}
	if count >= 1000 {
		kValue := float64(count) / 1000
		// If k value >= 999.5, show M to avoid rounding issues (999.9K -> 1000.0K)
		if kValue >= 999.5 {
			return fmt.Sprintf("%.2fM", float64(count)/1000000)
		}
		// Use integer for >= 100K
		if kValue >= 100 {
			return fmt.Sprintf("%.0fK", kValue)
		}
		return fmt.Sprintf("%.1fK", kValue)
	}
	return fmt.Sprintf("%d", count)
}

// FormatDuration formats milliseconds into human-readable duration.
//
// Formatting rules:
// - < 1s: display in milliseconds (e.g., "500ms")
// - < 1m: display in seconds (e.g., "45s")
// - < 1h: display in minutes and seconds (e.g., "3m 18s")
// - >= 1h: display in hours and minutes (e.g., "2h 15m")
//
// Examples:
//   - FormatDuration(500) = "500ms"
//   - FormatDuration(45000) = "45s"
//   - FormatDuration(198000) = "3m 18s"
//   - FormatDuration(8100000) = "2h 15m"
func FormatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}

	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
}

// FormatTimestamp formats Unix timestamp (milliseconds) into human-readable string.
// Returns empty string if timestamp is 0.
func FormatTimestamp(timestampMs int64) string {
	if timestampMs == 0 {
		return ""
	}
	return time.Unix(timestampMs/1000, (timestampMs%1000)*1e6).Format("2006-01-02 15:04:05")
}

// FormatCost formats USD cost with appropriate precision.
//
// Formatting rules:
// - < $0.01: display with 6 decimals (e.g., "$0.001234")
// - < $1: display with 4 decimals (e.g., "$0.1234")
// - >= $1: display with 2 decimals (e.g., "$1.23")
func FormatCost(costUSD float64) string {
	if costUSD >= 1.0 {
		return fmt.Sprintf("$%.2f", costUSD)
	}
	if costUSD >= 0.01 {
		return fmt.Sprintf("$%.4f", costUSD)
	}
	return fmt.Sprintf("$%.6f", costUSD)
}

// ExtractInt64 extracts int64 value from metadata map, supporting both int32 and int64 types.
// Returns 0 if key doesn't exist or type doesn't match.
func ExtractInt64(metadata map[string]any, key string) int64 {
	if v, ok := metadata[key].(int64); ok {
		return v
	}
	if v, ok := metadata[key].(int32); ok {
		return int64(v)
	}
	return 0
}

// ExtractFloat64 extracts float64 value from metadata map.
// Returns 0 if key doesn't exist or type doesn't match.
func ExtractFloat64(metadata map[string]any, key string) float64 {
	if v, ok := metadata[key].(float64); ok {
		return v
	}
	return 0
}
