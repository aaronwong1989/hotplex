package base

import (
	"testing"
)

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		count    int64
		expected string
	}{
		// < 1K: display as integer
		{0, "0"},
		{500, "500"},
		{999, "999"},

		// 1K to < 100K: display with 1 decimal
		{1000, "1.0K"},
		{50000, "50.0K"},
		{99949, "99.9K"},

		// 100K to < 1M: display as integer K
		{100000, "100K"},
		{500000, "500K"},
		{999499, "999K"},

		// >= 1M: display with 2 decimals
		{1000000, "1.00M"},
		{1234567, "1.23M"},
		{9999500, "10.00M"},

		// Edge case: 999.5K rounds to 1.00M
		{999500, "1.00M"},
		{999999, "1.00M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatTokenCount(tt.count)
			if result != tt.expected {
				t.Errorf("FormatTokenCount(%d) = %q, want %q", tt.count, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms       int64
		expected string
	}{
		{500, "500ms"},
		{1000, "1s"},
		{45000, "45s"},
		{60000, "1m 0s"},
		{198000, "3m 18s"},
		{3600000, "1h 0m"},
		{8100000, "2h 15m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.ms)
			if result != tt.expected {
				t.Errorf("FormatDuration(%d) = %q, want %q", tt.ms, result, tt.expected)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	// Test zero timestamp
	result := FormatTimestamp(0)
	if result != "" {
		t.Errorf("FormatTimestamp(0) = %q, want empty string", result)
	}

	// Test valid timestamp
	// Note: The actual formatted time depends on the local timezone
	result = FormatTimestamp(1705315800000)
	if result == "" {
		t.Error("FormatTimestamp(1705315800000) returned empty string")
	}
	// Verify format is correct (YYYY-MM-DD HH:MM:SS)
	if len(result) != 19 {
		t.Errorf("FormatTimestamp format incorrect: %q (length %d, expected 19)", result, len(result))
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0.0, "$0.000000"},
		{0.001234, "$0.001234"},
		{0.012345, "$0.0123"},
		{0.123456, "$0.1235"},
		{1.23, "$1.23"},
		{10.50, "$10.50"},
		{100.0, "$100.00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCost(tt.cost)
			if result != tt.expected {
				t.Errorf("FormatCost(%f) = %q, want %q", tt.cost, result, tt.expected)
			}
		})
	}
}
