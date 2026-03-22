# Event Protocol

The `event` package defines the high-performance communication protocol for HotPlex. It provides the callback mechanisms and metadata structures required for real-time AI interaction.

## 📡 Event Models

- **`EventWithMeta`**: The envelope for all system events, containing the type, data, and metadata.
- **`EventMeta`**: Detailed contextual information including durations, tool usage, tokens, and file system impacts.
- **`SessionStatsData`**: Final summary of a session (Total tokens, cost, duration, total files modified).
- **`Callback`**: A unified function signature: `func(eventType string, data any) error`.

## 🔄 Interaction Pattern

HotPlex events follow a **Streamed Observer** pattern:

1. **Dispatch**: The Engine generates events (tokens, status updates, security blocks).
2. **Metadata Injection**: The event is wrapped with session-specific metadata.
3. **Execution**: Registered callbacks (CLI, WebSocket, or ChatApps) process the event in real-time.

## 🛠 Practical Usage

```go
// Register a simple observer
engine.Execute(ctx, cfg, prompt, func(eventType string, data any) error {
    switch eventType {
    case "token":
        processToken(data.(string))
    case "tool_result":
        meta := data.(*event.EventMeta)
        logToolSuccess(meta.ToolName, meta.DurationMs)
    case "session_stats":
        stats := data.(*event.SessionStatsData)
        reportCost(stats.TotalCostUSD)
    }
    return nil
})
```
