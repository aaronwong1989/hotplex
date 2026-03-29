package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// PermissionEvent represents a permission request event from the server.
type PermissionEvent struct {
	SessionID    string
	PermissionID string
	Type         string
	Title        string
	ChatID       string
}

// EventRelay manages SSE event subscription, filtering, and routing.
// It dispatches events to appropriate subscribers based on session ID
// and handles special events like permission requests.
type EventRelay struct {
	transport *HTTPTransport
	mu        sync.RWMutex
	subscribers map[string]chan *ProviderEvent // key: serverSessionID

	lastHeartbeat time.Time
	connected     bool

	permissionCallback func(PermissionEvent)

	logger *slog.Logger
}

// NewEventRelay creates a new EventRelay.
func NewEventRelay(transport *HTTPTransport, logger *slog.Logger) *EventRelay {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventRelay{
		transport:    transport,
		subscribers:  make(map[string]chan *ProviderEvent),
		logger:       logger.With("component", "event_relay"),
		lastHeartbeat: time.Now(),
	}
}

// Subscribe subscribes to events for a specific server session.
// Returns a channel that receives ProviderEvents and a cancel function.
// The subscriber buffer is 64, with 50ms timeout to prevent backpressure.
func (r *EventRelay) Subscribe(serverSessionID string) (<-chan *ProviderEvent, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan *ProviderEvent, 64)
	r.subscribers[serverSessionID] = ch

	r.logger.Debug("Event subscriber added",
		"server_session_id", serverSessionID,
		"total_subscribers", len(r.subscribers))

	cancel := func() {
		r.mu.Lock()
		delete(r.subscribers, serverSessionID)
		close(ch)
		r.mu.Unlock()
		r.logger.Debug("Event subscriber removed",
			"server_session_id", serverSessionID,
			"remaining_subscribers", len(r.subscribers))
	}

	return ch, cancel
}

// Dispatch processes a raw SSE event JSON string and dispatches to subscribers.
func (r *EventRelay) Dispatch(rawEvent string) {
	r.lastHeartbeat = time.Now()
	r.connected = true

	var sseEvt OCSSEEvent
	if err := json.Unmarshal([]byte(rawEvent), &sseEvt); err != nil {
		r.logger.Warn("Failed to parse SSE event", "error", err)
		return
	}

	// Handle permission.updated specially
	if sseEvt.Type == OCEventPermissionUpdated {
		var perm OCPermissionProps
		if err := json.Unmarshal(sseEvt.Properties, &perm); err != nil {
			r.logger.Warn("Failed to parse permission event", "error", err)
			return
		}

		permEvt := PermissionEvent{
			SessionID:    perm.SessionID,
			PermissionID: perm.ID,
			Type:         perm.Type,
			Title:        perm.Title,
		}

		r.mu.RLock()
		cb := r.permissionCallback
		r.mu.RUnlock()

		if cb != nil {
			cb(permEvt)
		}
		return
	}

	// Convert to ProviderEvent for routing
	providerEvents, err := r.mapEvent(sseEvt)
	if err != nil {
		r.logger.Warn("Failed to map event", "error", err)
		return
	}

	if len(providerEvents) == 0 {
		return
	}

	// Extract session ID for routing
	sessionID := r.extractSessionID(sseEvt)

	// Dispatch to all subscribers (fan-out)
	// If sessionID is empty, broadcast to all (global events)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sessionID == "" {
		// Broadcast to all subscribers
		for sid, ch := range r.subscribers {
			for _, evt := range providerEvents {
				evt.SessionID = sid
				if !r.sendToChannel(ch, evt) {
					r.logger.Warn("Failed to dispatch event to subscriber",
						"session_id", sid)
				}
			}
		}
		return
	}

	// Send to specific subscriber
	ch, ok := r.subscribers[sessionID]
	if !ok {
		// No subscriber for this session, discard
		r.logger.Debug("No subscriber for session, discarding event",
			"session_id", sessionID)
		return
	}

	for _, evt := range providerEvents {
		if !r.sendToChannel(ch, evt) {
			r.logger.Warn("Failed to dispatch event to subscriber",
				"session_id", sessionID)
		}
	}
}

// SetPermissionCallback sets the callback for permission events.
func (r *EventRelay) SetPermissionCallback(fn func(PermissionEvent)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.permissionCallback = fn
}

// IsConnected returns whether the relay is receiving events.
func (r *EventRelay) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connected
}

// LastHeartbeat returns the time of the last received event.
func (r *EventRelay) LastHeartbeat() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastHeartbeat
}

// sendToChannel sends an event to a subscriber channel with timeout.
// Returns false if the channel is blocked or closed.
func (r *EventRelay) sendToChannel(ch chan *ProviderEvent, evt *ProviderEvent) bool {
	select {
	case ch <- evt:
		return true
	case <-time.After(50 * time.Millisecond):
		r.logger.Warn("Subscriber buffer full, dropping event")
		return false
	}
}

// mapEvent converts an OpenCode SSE event to provider events.
func (r *EventRelay) mapEvent(evt OCSSEEvent) ([]*ProviderEvent, error) {
	switch evt.Type {
	case OCEventMessagePartUpdated:
		var props OCPartUpdateProps
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse part update: %w", err)
		}
		return r.mapPart(props.Part, props.Delta)

	case OCEventMessageUpdated:
		var props struct {
			Info OCAssistantMessage `json:"info"`
		}
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse message updated: %w", err)
		}
		if props.Info.Finish != "" {
			contextWindow := estimateContextWindow(props.Info.ModelID)
			return []*ProviderEvent{{
				Type:    EventTypeResult,
				RawType: evt.Type,
				Metadata: &ProviderEventMeta{
					InputTokens:   props.Info.Tokens.Input,
					OutputTokens:  props.Info.Tokens.Output,
					TotalCostUSD:  props.Info.Cost,
					Model:         props.Info.ModelID,
					ModelName:     props.Info.ModelID,
					ContextWindow: contextWindow,
				},
			}}, nil
		}
		return nil, nil

	case OCEventSessionIdle:
		return []*ProviderEvent{{Type: EventTypeResult, RawType: evt.Type}}, nil

	case OCEventSessionStatus:
		var props OCSessionStatusProps
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse session status: %w", err)
		}
		switch props.Status.Type {
		case "busy":
			return []*ProviderEvent{{Type: EventTypeSystem, Status: "running"}}, nil
		case "idle":
			return []*ProviderEvent{{Type: EventTypeResult, RawType: evt.Type}}, nil
		case "retry":
			msg := "Retrying"
			if props.Status.Attempt > 0 {
				msg = fmt.Sprintf("Retrying (attempt %d)", props.Status.Attempt)
			}
			return []*ProviderEvent{{
				Type:    EventTypeSystem,
				Status:  "retrying",
				Content: msg,
			}}, nil
		default:
			return nil, nil
		}

	case OCEventSessionError:
		var props OCSessionErrorProps
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse session error: %w", err)
		}
		msg := "unknown error"
		if props.Error.Name != "" {
			msg = props.Error.Name
		}
		errData, _ := json.Marshal(props.Error.Data)
		return []*ProviderEvent{{
			Type:    EventTypeError,
			Error:   msg,
			IsError: true,
			Content: string(errData),
		}}, nil

	case OCEventPermissionUpdated:
		var perm OCPermissionProps
		if err := json.Unmarshal(evt.Properties, &perm); err != nil {
			return nil, fmt.Errorf("parse permission: %w", err)
		}
		return []*ProviderEvent{{
			Type:     EventTypePermissionRequest,
			ToolName: perm.Title,
			ToolID:   perm.ID,
			Content:  fmt.Sprintf("[Permission] %s: %s", perm.Type, perm.Title),
		}}, nil

	default:
		return nil, nil
	}
}

// mapPart converts an OpenCode part to provider events.
func (r *EventRelay) mapPart(part OCPart, delta string) ([]*ProviderEvent, error) {
	switch part.Type {
	case OCPartText:
		c := delta
		if c == "" {
			c = part.Text
		}
		return []*ProviderEvent{{Type: EventTypeAnswer, Content: c}}, nil

	case OCPartReasoning:
		c := delta
		if c == "" {
			c = part.Text
		}
		meta := &ProviderEventMeta{}
		if part.Tokens != nil {
			meta.OutputTokens = part.Tokens.Output
		}
		return []*ProviderEvent{{Type: EventTypeThinking, Content: c, Metadata: meta}}, nil

	case OCPartTool:
		status := part.GetStatus()
		toolName := part.GetToolName()

		switch status {
		case "pending", "running":
			return []*ProviderEvent{{
				Type:      EventTypeToolUse,
				ToolName:  toolName,
				ToolID:    part.ID,
				ToolInput: part.Input,
				Status:    "running",
			}}, nil
		case "completed":
			return []*ProviderEvent{{
				Type:     EventTypeToolResult,
				ToolName: toolName,
				ToolID:   part.ID,
				Content:  part.Output,
				Status:   "success",
			}}, nil
		case "error":
			return []*ProviderEvent{{
				Type:     EventTypeToolResult,
				ToolName: toolName,
				ToolID:   part.ID,
				Error:    part.Error,
				IsError:  true,
				Status:   "error",
			}}, nil
		}

	case OCPartStepStart:
		return []*ProviderEvent{{Type: EventTypeStepStart}}, nil

	case OCPartStepFinish:
		meta := &ProviderEventMeta{}

		if part.Tokens != nil {
			meta.InputTokens = part.Tokens.Input
			meta.OutputTokens = part.Tokens.Output
			if part.Tokens.Cache != nil {
				meta.CacheReadTokens = part.Tokens.Cache.Read
				meta.CacheWriteTokens = part.Tokens.Cache.Write
			}
		}

		meta.CurrentStep = int32(part.StepNumber)
		meta.TotalSteps = int32(part.TotalSteps)

		content := finishReasonMessages[FinishReason(part.Reason)]

		return []*ProviderEvent{{
			Type:     EventTypeStepFinish,
			Content:  content,
			Status:   part.Reason,
			Metadata: meta,
		}}, nil
	}

	return nil, nil
}

// extractSessionID extracts the session ID from an SSE event.
func (r *EventRelay) extractSessionID(evt OCSSEEvent) string {
	switch evt.Type {
	case OCEventPermissionUpdated:
		var perm OCPermissionProps
		if json.Unmarshal(evt.Properties, &perm) == nil {
			return perm.SessionID
		}
	}
	return ""
}

// Start begins dispatching events from the transport to subscribers.
func (r *EventRelay) Start(ctx context.Context) {
	r.mu.Lock()
	r.connected = true
	r.mu.Unlock()

	events := r.transport.Subscribe()
	defer r.transport.Unsubscribe(events)

	for {
		select {
		case <-ctx.Done():
			r.mu.Lock()
			r.connected = false
			r.mu.Unlock()
			return
		case rawEvt, ok := <-events:
			if !ok {
				r.logger.Warn("Event channel closed")
				r.mu.Lock()
				r.connected = false
				r.mu.Unlock()
				return
			}
			r.Dispatch(rawEvt)
		}
	}
}
