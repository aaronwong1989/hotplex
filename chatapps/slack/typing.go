// Package slack provides the Slack adapter implementation for the hotplex engine.
// TypingIndicator implements multi-stage emoji reaction-based typing feedback.
package slack

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

// TypingStage defines a stage in the multi-stage typing indicator.
type TypingStage struct {
	// After is the duration to wait before showing this stage
	After time.Duration
	// Emoji is the Slack emoji name (e.g., "eyes", "clock1")
	Emoji string
}

// DefaultStages is the multi-stage progression for emoji typing indicators.
// Stages progress from initial awareness to long-wait feedback.
var DefaultStages = []TypingStage{
	{0 * time.Second, "eyes"},                           // AI saw the message
	{2 * time.Minute, "clock1"},                          // Taking a while
	{7 * time.Minute, "hourglass_flowing_sand"},          // Long wait
	{12 * time.Minute, "gear"},                           // Processing complex task
	{17 * time.Minute, "hourglass_flowing_sand"},         // Still going...
}

// TypingIndicator manages emoji reactions for a single typing session.
// Each instance tracks reactions for one channel+message pair.
type TypingIndicator struct {
	adapter   *Adapter
	channelID string
	threadTS  string
	messageTS string // Message to react to (anchor)
	stages    []TypingStage

	mu    sync.Mutex
	done  bool
	added []string // Track added reactions for cleanup

	stopCh chan struct{}
}

// NewTypingIndicator creates a new typing indicator for a message.
func NewTypingIndicator(adapter *Adapter, channelID, threadTS, messageTS string) *TypingIndicator {
	return &TypingIndicator{
		adapter:   adapter,
		channelID: channelID,
		threadTS:  threadTS,
		messageTS: messageTS,
		stages:    DefaultStages,
		added:     make([]string, 0, len(DefaultStages)),
		stopCh:    make(chan struct{}),
	}
}

// NewTypingIndicatorWithStages creates a new typing indicator with custom stages.
func NewTypingIndicatorWithStages(adapter *Adapter, channelID, threadTS, messageTS string, stages []TypingStage) *TypingIndicator {
	return &TypingIndicator{
		adapter:   adapter,
		channelID: channelID,
		threadTS:  threadTS,
		messageTS: messageTS,
		stages:    stages,
		added:     make([]string, 0, len(stages)),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the multi-stage typing indicator.
// It adds the first emoji immediately, then schedules subsequent stages.
// Start is non-blocking; it runs the stage progression in a goroutine.
func (ti *TypingIndicator) Start(ctx context.Context) {
	// Always add eyes first
	ti.doAddReaction(ctx, ti.stages[0].Emoji)

	go ti.runStages(ctx)
}

// runStages handles the stage progression loop.
func (ti *TypingIndicator) runStages(ctx context.Context) {
	for i := 1; i < len(ti.stages); i++ {
		stage := ti.stages[i]
		timer := time.NewTimer(stage.After)
		select {
		case <-timer.C:
			ti.mu.Lock()
			if ti.done {
				ti.mu.Unlock()
				timer.Stop()
				return
			}
			emoji := stage.Emoji
			ti.mu.Unlock()
			ti.doAddReaction(ctx, emoji)
		case <-ti.stopCh:
			timer.Stop()
			return
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

// Stop stops the indicator and cleans up all reactions.
// It is safe to call Stop multiple times.
func (ti *TypingIndicator) Stop(ctx context.Context) {
	ti.mu.Lock()
	if ti.done {
		ti.mu.Unlock()
		return
	}
	ti.done = true
	close(ti.stopCh)
	// Only allocate if there are reactions to remove
	var added []string
	if len(ti.added) > 0 {
		added = make([]string, len(ti.added))
		copy(added, ti.added)
	}
	ti.mu.Unlock()

	for _, emoji := range added {
		ti.removeReaction(ctx, emoji)
	}
}

// Done returns a channel that's closed when the indicator has finished.
// Exported for testing.
func (ti *TypingIndicator) Done() <-chan struct{} {
	return ti.stopCh
}

// doAddReaction performs the actual API call and tracks the added emoji.
// Caller must not hold ti.mu when calling this.
func (ti *TypingIndicator) doAddReaction(ctx context.Context, emoji string) {
	if ti.adapter == nil || ti.adapter.client == nil {
		return
	}
	err := ti.adapter.client.AddReactionContext(ctx, emoji, slack.ItemRef{
		Channel:   ti.channelID,
		Timestamp: ti.messageTS,
	})
	if err != nil {
		ti.adapter.Logger().Debug("TypingIndicator: failed to add reaction",
			slog.String("emoji", emoji),
			slog.String("error", err.Error()))
		return
	}
	ti.mu.Lock()
	ti.added = append(ti.added, emoji)
	ti.mu.Unlock()
	ti.adapter.Logger().Debug("TypingIndicator: added reaction",
		slog.String("emoji", emoji),
		slog.String("channel", ti.channelID),
		slog.String("message_ts", ti.messageTS))
}

func (ti *TypingIndicator) removeReaction(ctx context.Context, emoji string) {
	if ti.adapter == nil || ti.adapter.client == nil {
		return
	}
	err := ti.adapter.client.RemoveReactionContext(ctx, emoji, slack.ItemRef{
		Channel:   ti.channelID,
		Timestamp: ti.messageTS,
	})
	if err != nil {
		ti.adapter.Logger().Debug("TypingIndicator: failed to remove reaction",
			slog.String("emoji", emoji),
			slog.String("error", err.Error()))
	}
}

// ActiveIndicators manages a collection of active TypingIndicators.
type ActiveIndicators struct {
	mu         sync.Mutex
	indicators map[string]*TypingIndicator // key: "channelID:messageTS"
}

// NewActiveIndicators creates a new ActiveIndicators manager.
func NewActiveIndicators() *ActiveIndicators {
	return &ActiveIndicators{
		indicators: make(map[string]*TypingIndicator),
	}
}

func (ai *ActiveIndicators) key(channelID, messageTS string) string {
	return fmt.Sprintf("%s:%s", channelID, messageTS)
}

// Start creates and starts a new typing indicator for the given channel+message.
func (ai *ActiveIndicators) Start(ctx context.Context, adapter *Adapter, channelID, threadTS, messageTS string) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	key := ai.key(channelID, messageTS)
	if _, exists := ai.indicators[key]; exists {
		return // Already exists
	}
	ti := NewTypingIndicator(adapter, channelID, threadTS, messageTS)
	ai.indicators[key] = ti
	ti.Start(ctx)
}

// Stop stops and removes the typing indicator for the given channel+message.
func (ai *ActiveIndicators) Stop(ctx context.Context, channelID, messageTS string) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	key := ai.key(channelID, messageTS)
	ti, exists := ai.indicators[key]
	if !exists {
		return
	}
	delete(ai.indicators, key)
	ti.Stop(ctx)
}

// Get returns the typing indicator for the given channel+message, or nil if not found.
func (ai *ActiveIndicators) Get(channelID, messageTS string) *TypingIndicator {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	return ai.indicators[ai.key(channelID, messageTS)]
}
