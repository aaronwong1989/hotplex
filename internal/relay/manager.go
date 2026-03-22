package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RelayManager manages relay bindings and orchestrates message delivery.
type RelayManager struct {
	sender  *RelaySender
	cb      *RelayCircuitBreaker
	store   *BindingStore
	mu      sync.RWMutex      // protects runtime routing lookups only; store is self-locking
	byAgent map[string]string // agentName -> bindingKey for fast lookup
}

// NewRelayManager creates a new RelayManager backed by a BindingStore.
func NewRelayManager(sender *RelaySender) *RelayManager {
	store := NewBindingStore("")
	rm := &RelayManager{
		sender:  sender,
		cb:      NewRelayCircuitBreaker(),
		store:   store,
		byAgent: make(map[string]string),
	}
	// Index existing bindings by agent for fast lookup.
	for _, b := range rm.store.List() {
		for agent := range b.Bots {
			rm.byAgent[agent] = b.ChatID
		}
	}
	return rm
}

// AddBinding registers a new relay binding and persists it to disk.
func (rm *RelayManager) AddBinding(binding *RelayBinding) error {
	if binding == nil {
		return fmt.Errorf("binding is nil")
	}
	if binding.ChatID == "" {
		return fmt.Errorf("binding chat_id is required")
	}
	if binding.Bots == nil {
		binding.Bots = make(map[string]string)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if err := rm.store.Add(binding); err != nil {
		return err
	}
	for agent := range binding.Bots {
		rm.byAgent[agent] = binding.ChatID
	}
	return nil
}

// RemoveBinding removes a relay binding by platform and chatID.
func (rm *RelayManager) RemoveBinding(platform, chatID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Find and remove the binding from the store.
	for _, b := range rm.store.List() {
		if b.Platform == platform && b.ChatID == chatID {
			if err := rm.store.Delete(b.ChatID); err != nil {
				return err
			}
			for agent := range b.Bots {
				delete(rm.byAgent, agent)
			}
			return nil
		}
	}
	return fmt.Errorf("binding not found for %s:%s", platform, chatID)
}

// ListBindings returns a snapshot of all registered bindings.
func (rm *RelayManager) ListBindings() []*RelayBinding {
	return rm.store.List()
}

// Send delivers a relay message to a target agent identified by toAgent.
// It uses the circuit breaker for fault tolerance and the sender for HTTP delivery.
func (rm *RelayManager) Send(ctx context.Context, toAgent, content string) (*RelayResponse, error) {
	if toAgent == "" {
		return nil, fmt.Errorf("toAgent is required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Fast lookup: find the binding key for this agent.
	rm.mu.RLock()
	chatID, ok := rm.byAgent[toAgent]
	rm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no binding found for agent %q", toAgent)
	}

	// Find the binding in the store.
	var binding *RelayBinding
	for _, b := range rm.store.List() {
		if b.ChatID == chatID {
			binding = b
			break
		}
	}
	if binding == nil {
		return nil, fmt.Errorf("binding not found for agent %q", toAgent)
	}

	targetURL, ok := binding.Bots[toAgent]
	if !ok || targetURL == "" {
		return nil, fmt.Errorf("agent %q has no target URL in binding", toAgent)
	}

	taskID := uuid.NewString()
	msg := &RelayMessage{
		TaskID:    taskID,
		To:        toAgent,
		Content:   content,
		Status:    TaskStatusWorking,
		CreatedAt: time.Now(),
	}

	// Execute via circuit breaker.
	result, err := rm.cb.Call(ctx, toAgent, func() (any, error) {
		if sendErr := rm.sender.Send(ctx, msg, targetURL); sendErr != nil {
			return nil, sendErr
		}
		return &RelayResponse{TaskID: taskID, Status: "sent", Timestamp: time.Now()}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("relay send failed for %q: %w", toAgent, err)
	}

	resp, ok := result.(*RelayResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
	return resp, nil
}
