# Bot-to-Bot Relay (`internal/relay`)

The `relay` package enables secure, cross-platform communication between AI agents. It allows one HotPlex agent to send messages to another agent, even if they are on different chat platforms (e.g., Slack to DingTalk).

## Overview

Relay uses **Bindings** to map agent names to target URLs. It includes fault tolerance via circuit breakers and persistent routing configuration.

## Core Components

- **`RelayManager`**: Orchestrates message delivery and routing lookups.
- **`BindingStore`**: Manages persistent storage of relay bindings.
- **`RelaySender`**: Handles authenticated HTTP delivery of relay messages.
- **`RelayCircuitBreaker`**: Provides fault isolation and prevents cascading failures.

## Interaction Pattern

1.  **Lookup**: `RelayManager` finds the target URL associated with an agent name.
2.  **Circuit Check**: Verifies the target agent is healthy.
3.  **Delivery**: `RelaySender` POSTs a `RelayMessage` to the target.
4.  **Tracking**: Each relay is assigned a unique `TaskID` for async tracking.

## Usage

```go
// Add a binding
binding := &relay.RelayBinding{
    Platform: "slack",
    ChatID:   "C12345",
    Bots: map[string]string{
        "security-bot": "http://other-hotplex:8080/relay",
    },
}
manager.AddBinding(binding)

// Send a relay
resp, err := manager.Send(ctx, "security-bot", "Analyze this session: s_abc123")
```

## CLI Interface

Manage relay bindings via `hotplexd relay`:
- `hotplexd relay add_binding`: Map a local chat ID to remote bot URLs.
- `hotplexd relay list_bindings`: View all active relay routes.
- `hotplexd relay test_relay`: Send a test message to a remote agent.
