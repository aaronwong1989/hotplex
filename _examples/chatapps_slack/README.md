# Slack Chat Adapter Example

This example demonstrates how to use the HotPlex Slack adapter to create a chat bot that integrates with the HotPlex AI Engine.

## Prerequisites

1. **Slack App Configuration**:
   - Create a Slack App at https://api.slack.com/apps
   - Enable the following OAuth scopes:
     - `chat:write`
     - `channels:read`
     - `groups:read`
     - `im:read`
     - `mpim:read`
     - `app_mentions:read`
   - For Socket Mode (optional):
     - Enable Socket Mode in your app settings
     - Generate an App Token (`xapp-...`)

2. **Environment Variables**:
   ```bash
   export HOTPLEX_SLACK_BOT_TOKEN=xoxb-...  # Required
   export HOTPLEX_SLACK_APP_TOKEN=xapp-...   # Required for Socket Mode
   export HOTPLEX_SLACK_SIGNING_SECRET=...   # Required for HTTP Mode
   export HOTPLEX_SLACK_SERVER_ADDR=:8080    # Optional, default: :8080
   export HOTPLEX_SLACK_MODE=http            # Optional: "http" or "socket"
   export HOTPLEX_SLACK_BOT_USER_ID=U...     # Optional: Your bot's user ID
   ```

## Running the Example

```bash
cd _examples/chatapps_slack
go run main.go
```

## Mode Options

### HTTP Mode (Default)
- Uses webhooks for receiving events
- Requires `SigningSecret` to be configured
- Events arrive at `/events` endpoint

### Socket Mode
- Uses WebSocket connection for real-time events
- Requires `AppToken` to be configured
- More efficient for high-volume applications

## Permission Policies

You can configure permission policies in the config:

```go
config := &slack.Config{
    // Direct Message policy: "allow", "pairing", "block"
    DMPolicy: "allow",

    // Group message policy: "allow", "mention", "multibot", "block"
    GroupPolicy: "allow",

    // Whitelist specific users
    AllowedUsers: []string{"U1234567890"},

    // Blacklist specific users
    BlockedUsers: []string{"U0987654321"},
}
```

## Integrating with HotPlex Engine

To integrate with the HotPlex Engine, modify the message handler:

```go
adapter.SetHandler(func(ctx context.Context, msg *chatapps.ChatMessage) error {
    // Create HotPlex Engine
    engine, err := hotplex.NewEngine(hotplex.EngineOptions{
        Namespace: "slack_bot",
        // ... other options
    })
    if err != nil {
        return err
    }
    defer engine.Close()

    // Execute prompt through HotPlex
    cfg := &hotplex.Config{
        SessionID: msg.SessionID,
        WorkDir:   "/tmp",
    }

    return engine.Execute(ctx, cfg, msg.Content, func(eventType string, data any) error {
        // Send streaming events back to Slack
        // ...
        return nil
    })
})
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/events` | Slack Events API webhook |
| `/interactive` | Interactive component callbacks |
| `/slack` | Slash command handler |
| `/health` | Health check endpoint |
