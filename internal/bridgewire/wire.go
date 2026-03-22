// Package bridgewire defines the shared Bridge Wire Protocol types and constants
// used by both BridgeServer (internal/server) and BridgeClient (cmd/bridge-client).
//
// The wire protocol is a bidirectional JSON envelope. Both parties must agree on
// the exact field names and JSON tags documented here.
package bridgewire

import "encoding/json"

// =============================================================================
// Message Types
// =============================================================================

// MsgType* are the valid Type field values in a WireMessage.
const (
	MsgTypeRegister = "register"
	MsgTypeMessage  = "message"
	MsgTypeReply    = "reply"
	MsgTypeEvent    = "event"
	MsgTypeError    = "error"
)

// =============================================================================
// Capabilities
// =============================================================================

// Capability constants that an adapter can declare when registering.
const (
	CapText    = "text"
	CapImage   = "image"
	CapCard    = "card"
	CapButtons = "buttons"
	CapTyping  = "typing"
	CapEdit    = "edit"
	CapDelete  = "delete"
	CapReact   = "react"
	CapThread  = "thread"
)

// AllCapabilities is the full set of supported capabilities.
var AllCapabilities = []string{
	CapText, CapImage, CapCard, CapButtons,
	CapTyping, CapEdit, CapDelete, CapReact, CapThread,
}

// =============================================================================
// Wire Protocol Types
// =============================================================================

// WireMessage is the bidirectional JSON envelope for the Bridge Wire Protocol.
// All fields use snake_case JSON tags to match the wire format.
//
// Direction: both client→server and server→client share the same struct,
// but only the relevant fields are populated for each message type.
type WireMessage struct {
	Type         string          `json:"type"`
	Platform     string          `json:"platform,omitempty"`
	Token        string          `json:"token,omitempty"`
	SessionKey   string          `json:"session_key,omitempty"`
	Content      string          `json:"content,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Event        string          `json:"event,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	Code         int             `json:"code,omitempty"`
	Message      string          `json:"message,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`

	// Relay fields (omitempty for backward compatibility)
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Response  string `json:"response,omitempty"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// WireMetadata is the metadata carried inside WireMessage.Metadata.
// It describes the source room, thread, and user identity.
type WireMetadata struct {
	UserID   string `json:"user_id,omitempty"`
	RoomID   string `json:"room_id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	Platform string `json:"platform,omitempty"`
}
