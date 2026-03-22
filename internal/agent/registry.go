package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AgentRegistry manages the local AgentCard and discovers remote agents.
type AgentRegistry struct {
	card  *AgentCard
	mu    sync.RWMutex
	cache map[string]*AgentCard // URL → card (with TTL)
}

// NewAgentRegistry creates a new registry with the given local card.
func NewAgentRegistry(card *AgentCard) *AgentRegistry {
	return &AgentRegistry{
		card:  card,
		cache: make(map[string]*AgentCard),
	}
}

// GetAgentCard returns the local agent card.
func (r *AgentRegistry) GetAgentCard() *AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.card
}

// Register replaces the local agent card.
func (r *AgentRegistry) Register(card *AgentCard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.card = card
}

// Discover fetches and caches a remote agent card.
func (r *AgentRegistry) Discover(remoteURL string) (*AgentCard, error) {
	r.mu.RLock()
	cached, ok := r.cache[remoteURL]
	r.mu.RUnlock()
	if ok && time.Since(cached.CreatedAt) < 10*time.Minute {
		return cached, nil
	}

	resp, err := http.Get(remoteURL + "/admin/agent-card")
	if err != nil {
		return nil, fmt.Errorf("discover agent: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Printf("warning: failed to close response body: %v\n", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover agent: unexpected status %d", resp.StatusCode)
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decode agent card: %w", err)
	}

	r.mu.Lock()
	r.cache[remoteURL] = &card
	r.mu.Unlock()

	return &card, nil
}
