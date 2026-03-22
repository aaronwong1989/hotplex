# Event System

> **Note**: HotPlex does not have a "Hooks" system. This page previously described a non-existent feature (`hotplexd hook register`, `engine.OnInput()`). The correct extensibility mechanism is the **Event System** documented below.

## Event-Driven Architecture

HotPlex uses an event-driven architecture to provide visibility into agent execution. Rather than injecting custom logic at lifecycle points (which is not supported), you can **subscribe to events** emitted by the engine during session execution.

---

### Event Types

All events share a common structure:

```go
type EventMeta struct {
    SessionID  string    // Unique session identifier
    Timestamp time.Time // When the event occurred
    TraceID  string    // Distributed trace ID for correlation
}

type EventWithMeta struct {
    EventType string    // e.g., "tool_use", "thinking", "error"
    Data      string    // Human-readable event description
    Meta      *EventMeta // Always non-nil when created via NewEventWithMeta
}

type Callback func(eventType string, data any) error
```

### Built-in Events

The engine emits the following event types:

| Event Type | Description | Data |
|-----------|-------------|------|
| `thinking` | AI is reasoning | Reasoning text |
| `tool_use` | Agent called a tool | Tool name + arguments |
| `tool_result` | Tool execution completed | Result or error |
| `error` | An error occurred | Error message |
| `session_stats` | Session completed | `SessionStatsData` struct |

---

### Subscribing to Events

Pass a `Callback` function when starting a session:

```go
callback := func(eventType string, data any) error {
    switch eventType {
    case "tool_use":
        fmt.Printf("[TOOL] %v\n", data)
    case "error":
        fmt.Printf("[ERROR] %v\n", data)
    }
    return nil
}

// Wrap with safe logging
safeCallback := event.WrapSafe(logger, callback)

// Engine sends events to your callback
engine := New(callback)
```

### OpenTelemetry Integration

For production observability, export events to your telemetry infrastructure:

```bash
# Start hotplexd with OTLP endpoint
hotplexd start --otel-endpoint "otel.internal.yourcorp.com:4317"
```

Events are exported as OpenTelemetry spans with the following attributes:
- `session.id`: Session identifier
- `event.type`: Event type name
- `event.data`: Event payload

See [Observability Guide](./observability) for full telemetry setup.
