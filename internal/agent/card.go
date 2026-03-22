// Package agent provides agent capability discovery via Agent Card.
package agent

import "time"

// AgentCard describes this HotPlex instance's capabilities and identity.
type AgentCard struct {
	Name         string       `json:"name"`
	Provider     Provider     `json:"provider"`
	URL          string       `json:"url"`
	CreatedAt    time.Time    `json:"created_at"`
	Capabilities Capabilities `json:"capabilities"`
	Skills       []Skill      `json:"skills"`
	Security     []Security   `json:"security"`
}

// Provider describes the AI provider backing this agent.
type Provider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

// Capabilities declares what features this agent supports.
type Capabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"push_notifications"`
}

// Skill represents a named capability this agent can perform.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Security declares the security mechanisms this agent requires.
type Security struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}
