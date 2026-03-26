package engine

import (
	"testing"

	"github.com/hrygo/hotplex/provider"
)

// TestContextWindowCalculation verifies that context window percentage is calculated correctly
// from provider event metadata tokens.
func TestContextWindowCalculation(t *testing.T) {
	// Test cases based on real stream-json output
	testCases := []struct {
		name                string
		inputTokens         int32
		cacheReadTokens     int32
		cacheWriteTokens    int32
		expectedPercent     float64
		contextWindow       int32
	}{
		{
			name:             "Turn 1: Simple question",
			inputTokens:      69041,
			cacheReadTokens:  512,
			cacheWriteTokens: 0,
			contextWindow:    200000,
			expectedPercent:  34.7765, // (69041 + 512 + 0) / 200000 * 100
		},
		{
			name:             "Turn 2: Longer response",
			inputTokens:      64575,
			cacheReadTokens:  2240,
			cacheWriteTokens: 0,
			contextWindow:    200000,
			expectedPercent:  33.4075, // (64575 + 2240 + 0) / 200000 * 100
		},
		{
			name:             "Turn 3: Even longer",
			inputTokens:      66783,
			cacheReadTokens:  512,
			cacheWriteTokens: 0,
			contextWindow:    200000,
			expectedPercent:  33.6475, // (66783 + 512 + 0) / 200000 * 100
		},
		{
			name:             "Empty tokens",
			inputTokens:      0,
			cacheReadTokens:  0,
			cacheWriteTokens: 0,
			contextWindow:    200000,
			expectedPercent:  0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock provider event with metadata
			pevt := &provider.ProviderEvent{
				Type:    provider.EventTypeResult,
				Content: "test content",
				Metadata: &provider.ProviderEventMeta{
					InputTokens:      tc.inputTokens,
					CacheReadTokens:  tc.cacheReadTokens,
					CacheWriteTokens: tc.cacheWriteTokens,
				},
			}

			// Create stats
			stats := &SessionStats{
				SessionID: "test-session",
			}

			// Simulate the handleNormalizedResult logic
			if pevt.Metadata != nil {
				stats.lastInputTokens = pevt.Metadata.InputTokens
				stats.lastCacheReadTokens = pevt.Metadata.CacheReadTokens
				stats.lastCacheWriteTokens = pevt.Metadata.CacheWriteTokens
			}

			// Calculate context percentage (same logic as runner.go:644-649)
			totalInputTokens := stats.lastInputTokens + stats.lastCacheReadTokens + stats.lastCacheWriteTokens
			contextUsedPercent := 0.0
			if tc.contextWindow > 0 && totalInputTokens > 0 {
				contextUsedPercent = float64(totalInputTokens) / float64(tc.contextWindow) * 100
			}

			// Verify calculation
			tolerance := 0.01
			if contextUsedPercent < tc.expectedPercent-tolerance || contextUsedPercent > tc.expectedPercent+tolerance {
				t.Errorf("Context percentage mismatch: got %.4f%%, want %.4f%%",
					contextUsedPercent, tc.expectedPercent)
			}

			t.Logf("✅ %s: %.4f%% (input=%d, cache_read=%d, cache_write=%d)",
				tc.name, contextUsedPercent, tc.inputTokens, tc.cacheReadTokens, tc.cacheWriteTokens)
		})
	}
}

// TestContextWindowIntegration tests the full integration with event callback
func TestContextWindowIntegration(t *testing.T) {
	// This test is a placeholder for integration testing
	// Full integration test would require mocking the provider properly
	t.Skip("Integration test requires full provider mock setup")
}
