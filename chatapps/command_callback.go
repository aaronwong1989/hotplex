package chatapps

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/provider"
)

// CommandCallback handles slash command progress events
type CommandCallback struct {
	ctx       context.Context
	platform  string
	sessionID string
	adapters  *AdapterManager
	logger    *slog.Logger
	metadata  map[string]any

	mu        sync.Mutex
	messageTS string
	channelID string
	title     string
}

// NewCommandCallback creates a new command callback
func NewCommandCallback(ctx context.Context, platform, sessionID string, adapters *AdapterManager, logger *slog.Logger, metadata map[string]any) *CommandCallback {
	return &CommandCallback{
		ctx:       ctx,
		platform:  platform,
		sessionID: sessionID,
		adapters:  adapters,
		logger:    logger,
		metadata:  metadata,
	}
}

// Handle implements event.Callback interface
func (c *CommandCallback) Handle(eventType string, data any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch provider.ProviderEventType(eventType) {
	case provider.EventTypeCommandProgress:
		return c.handleProgress(data)
	case provider.EventTypeCommandComplete:
		return c.handleComplete(data)
	default:
		c.logger.Debug("Unknown command event type", "type", eventType)
	}
	return nil
}

func (c *CommandCallback) handleProgress(data any) error {
	meta, ok := data.(*event.EventWithMeta)
	if !ok {
		return nil
	}

	// Extract title from event data
	title := meta.EventData
	if title == "" {
		title = "Processing..."
	}

	// Save title for final message
	c.title = title

	// Extract progress from metadata
	progress := meta.Meta.Progress
	totalSteps := meta.Meta.TotalSteps
	currentStep := meta.Meta.CurrentStep

	// Build step list from current state
	var steps []map[string]any
	if totalSteps > 0 {
		// Generate placeholder steps based on totalSteps
		stepNames := []string{"Finding session", "Deleting session file", "Deleting marker", "Terminating process"}
		for i := 0; i < int(totalSteps) && i < len(stepNames); i++ {
			status := "pending"
			if int(currentStep) > i {
				status = "success"
			} else if int(currentStep) == i+1 {
				status = "running"
			}
			steps = append(steps, map[string]any{
				"name":    stepNames[i],
				"message": stepNames[i],
				"status":  status,
			})
		}
	}

	c.logger.Debug("Command progress", "title", title, "progress", progress, "current_step", currentStep, "total_steps", totalSteps)

	// Format content as text
	content := fmt.Sprintf("*%s*\nProgress: %d%%", title, progress)

	// Send message with platform-agnostic MessageType
	msg := &base.ChatMessage{
		Type:    base.MessageTypeCommandProgress,
		Content: content,
		Metadata: map[string]any{
			"event_type":   string(provider.EventTypeCommandProgress),
			"progress":     progress,
			"current_step": currentStep,
			"total_steps":  totalSteps,
			"steps":        steps,
		},
	}
	msg.Metadata = c.mergeMetadata(msg.Metadata)

	return c.sendOrUpdate(msg)
}

func (c *CommandCallback) handleComplete(data any) error {
	meta, ok := data.(*event.EventWithMeta)
	if !ok {
		return nil
	}

	// Send completion message with platform-agnostic MessageType
	msg := &base.ChatMessage{
		Type:    base.MessageTypeCommandComplete,
		Content: meta.EventData,
		Metadata: map[string]any{
			"event_type": string(provider.EventTypeCommandComplete),
			"title":      c.title,
		},
	}
	msg.Metadata = c.mergeMetadata(msg.Metadata)

	return c.sendOrUpdate(msg)
}

func (c *CommandCallback) sendOrUpdate(msg *base.ChatMessage) error {
	// If we have a message TS, update; otherwise create new
	if c.messageTS != "" && c.channelID != "" {
		msg.Metadata["message_ts"] = c.messageTS
		msg.Metadata["channel_id"] = c.channelID
	}

	// Convert base.ChatMessage to ChatMessage for adapter
	chatMsg := &ChatMessage{
		Platform:    c.platform,
		SessionID:   c.sessionID,
		UserID:      msg.UserID,
		Content:     msg.Content,
		MessageID:   msg.MessageID,
		Timestamp:   msg.Timestamp,
		Metadata:    msg.Metadata,
		RichContent: msg.RichContent,
	}

	// Send message
	if err := c.adapters.SendMessage(c.ctx, c.platform, c.sessionID, chatMsg); err != nil {
		return fmt.Errorf("send command message: %w", err)
	}

	// Save TS for future updates
	if ts, ok := msg.Metadata["message_ts"].(string); ok && ts != "" {
		c.messageTS = ts
	}
	if ch, ok := msg.Metadata["channel_id"].(string); ok && ch != "" {
		c.channelID = ch
	}

	return nil
}

// mergeMetadata merges the callback's stored metadata with the provided metadata
func (c *CommandCallback) mergeMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	// Copy over important metadata from stored metadata
	maps.Copy(metadata, c.metadata)
	metadata["stream"] = true
	metadata["event_type"] = "command"
	return metadata
}
