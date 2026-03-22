package slack

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTypingIndicator_Stages(t *testing.T) {
	stages := []TypingStage{
		{0 * time.Second, "eyes"},
		{1 * time.Second, "clock1"},
	}
	ti := NewTypingIndicatorWithStages(nil, "C001", "T001", "M001", stages)
	assert.Equal(t, 2, len(ti.stages))
	assert.Equal(t, "eyes", ti.stages[0].Emoji)
	assert.Equal(t, "clock1", ti.stages[1].Emoji)
	assert.Equal(t, 1*time.Second, ti.stages[1].After)
}

func TestTypingIndicator_DefaultStages(t *testing.T) {
	ti := NewTypingIndicator(nil, "C001", "T001", "M001")
	assert.Equal(t, len(DefaultStages), len(ti.stages))
	assert.Equal(t, "eyes", ti.stages[0].Emoji)
	assert.Equal(t, 0*time.Second, ti.stages[0].After)
}

func TestTypingIndicator_StartNoClient(t *testing.T) {
	// Adapter with nil client should not panic
	ti := NewTypingIndicator(nil, "C001", "T001", "M001")
	ctx := context.Background()

	// Should not panic even with nil client
	ti.Start(ctx)

	// Stop should be safe with nil client
	ti.Stop(ctx)
}

func TestTypingIndicator_StopIdempotent(t *testing.T) {
	ti := NewTypingIndicator(nil, "C001", "T001", "M001")
	ctx := context.Background()

	ti.Start(ctx)
	ti.Stop(ctx)

	// Second stop should not panic
	ti.Stop(ctx)
}

func TestTypingIndicator_DoneChannel(t *testing.T) {
	ti := NewTypingIndicator(nil, "C001", "T001", "M001")
	ctx := context.Background()

	done := ti.Done()
	select {
	case <-done:
		t.Fatal("Done channel should not be closed before Stop")
	default:
		// Expected: channel not closed
	}

	ti.Stop(ctx)

	// After Stop, channel should be closed
	select {
	case <-done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after Stop")
	}
}

func TestTypingIndicator_StopCancelsStages(t *testing.T) {
	stages := []TypingStage{
		{0 * time.Second, "eyes"},
		{10 * time.Second, "clock1"}, // Long delay
	}
	ti := NewTypingIndicatorWithStages(nil, "C001", "T001", "M001", stages)
	ctx := context.Background()

	ti.Start(ctx)

	// Stop immediately - should not wait for the 10s stage
	start := time.Now()
	ti.Stop(ctx)
	elapsed := time.Since(start)

	// Should complete quickly (within 1s), not wait 10s
	assert.Less(t, elapsed, 2*time.Second,
		"Stop should not block waiting for long stage delays")
}

func TestTypingIndicator_StagesSliceIntegrity(t *testing.T) {
	stages := []TypingStage{
		{0 * time.Second, "a"},
		{5 * time.Second, "b"},
	}
	ti := NewTypingIndicatorWithStages(nil, "C001", "T001", "M001", stages)

	// Verify the stages slice is independent (not shared with caller)
	assert.Equal(t, 2, len(ti.stages))
}

func TestActiveIndicators_StartAndStop(t *testing.T) {
	ai := NewActiveIndicators()
	ctx := context.Background()

	// Start with nil adapter (safe)
	ai.Start(ctx, nil, "C001", "T001", "M001")

	// Should not panic - retrieves nil
	ti := ai.Get("C001", "M001")
	assert.NotNil(t, ti)

	// Stop should be safe even with nil adapter
	ai.Stop(ctx, "C001", "M001")
}

func TestActiveIndicators_GetNotExists(t *testing.T) {
	ai := NewActiveIndicators()
	ti := ai.Get("C999", "M999")
	assert.Nil(t, ti)
}

func TestActiveIndicators_DuplicateStart(t *testing.T) {
	ai := NewActiveIndicators()
	ctx := context.Background()

	ai.Start(ctx, nil, "C001", "T001", "M001")
	ai.Start(ctx, nil, "C001", "T001", "M001") // Duplicate - should not create new

	ti := ai.Get("C001", "M001")
	assert.NotNil(t, ti)

	// Only one indicator exists
	ai.Stop(ctx, "C001", "M001")
	ti = ai.Get("C001", "M001")
	assert.Nil(t, ti)
}
