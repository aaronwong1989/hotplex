# Slack UX Optimization Research Report

## Overview

This document analyzes the complete flow from Engine Event to Slack message, identifies current UX pain points, and provides optimization recommendations to surpass OpenClaw.

---

## 1. Event Flow Architecture

### 1.1 Complete Event Lifecycle

```
CLI Output (stream-json)
        ↓
provider/claude_provider.go:ParseEvent()
        ↓
┌──────────────────────────────────────────────────────────────┐
│  Event Type Mapping (provider/event.go:18-77)               │
├──────────────────────────────────────────────────────────────┤
│  EventTypeThinking        → thinking event                  │
│  EventTypeToolUse         → tool invocation started          │
│  EventTypeToolResult      → tool execution result            │
│  EventTypeAnswer          → final answer                    │
│  EventTypePermissionRequest→ permission request               │
│  EventTypePlanMode        → plan mode                      │
│  EventTypeExitPlanMode   → exit plan mode                  │
│  EventTypeAskUserQuestion → user question                   │
│  EventTypeError           → error                           │
│  EventTypeResult          → session complete                │
└──────────────────────────────────────────────────────────────┘
        ↓
chatapps/engine_handler.go (Bridge Layer)
        ↓
┌──────────────────────────────────────────────────────────────┐
│  Message Processing Chain (processor_*.go)                   │
│  1. RateLimit    - Rate limiting (100ms interval)           │
│  2. Thread       - Thread management                        │
│  3. Aggregation  - Message aggregation (500ms window)       │
│  4. RichContent  - Rich content processing                  │
│  5. FormatConversion - Markdown → Mrkdwn                    │
│  6. Chunk        - Chunking (4000 char limit)              │
└──────────────────────────────────────────────────────────────┘
        ↓
chatapps/slack/block_builder.go → Slack Block Kit
        ↓
Slack API (chat.postMessage / chat.update)
```

### 1.2 Event Type Definitions

#### ProviderEventType Enumeration

| Event Type | Description | Slack Message Type |
|------------|-------------|-------------------|
| `thinking` | AI thinking process | Context Block |
| `tool_use` | Tool invocation started | Section Block |
| `tool_result` | Tool execution result | Section + Context |
| `answer` | AI generated content | Section Block (stream-update) |
| `error` | Error message | Danger Section |
| `permission_request` | Permission approval | Actions Block (buttons) |
| `plan_mode` | Plan generation | Header + Section |
| `result` | Session complete | Stats Block |

---

## 2. Current UX Pain Points

### 2.1 Critical Issues

| Issue | Location | Problem | Impact |
|-------|----------|---------|--------|
| **Thinking State Too Simple** | `block_builder.go:340` | Only shows "Thinking..." static text | Users don't know what AI is doing |
| **Message Chunking Poor** | `chunker.go:59` | Simple char split, code blocks may break | Long responses hard to read |
| **Error Feedback Unfriendly** | `adapter.go:1097` | Exposes raw Slack API error codes | Confuses regular users |

### 2.2 Medium Issues

| Issue | Problem | Improvement Direction |
|-------|---------|----------------------|
| **No Tool Execution Progress** | Only shows tool name | Real-time progress + ETA |
| **Permission Confirmation Popup** | Click every time | Smart remember authorization |
| **No Response Delay Feedback** | Users wait blindly | Show progress percentage |

### 2.3 Current Code Analysis

#### Thinking Status Block (block_builder.go:333)

```go
func (b *BlockBuilder) BuildStatusBlock(statusType StatusType, content string) []map[string]any {
    // Current: Only displays static emoji + text
    // Issue: No progress indication, no detailed thinking content
}
```

#### Message Chunking (chunker.go:68)

```go
func ChunkMessageMarkdown(text string, limit int) []string {
    // Current: Simple character-based splitting
    // Issue: May break code blocks mid-way
}
```

---

## 3. OpenClaw Competitive Analysis

### 3.1 OpenClaw Core Advantages

| Feature | OpenClaw Implementation | HotPlex Current | Gap |
|---------|------------------------|-----------------|-----|
| **Streaming Output** | `chat.startStream` / `appendStream` / `stopStream` native Slack API | `chat.update` polling | 🔴 |
| **Streaming Modes** | `partial` (replace) / `block` (append) / `progress` (status) | Only `partial` | 🔴 |
| **Ack Reaction** | Shows 👀 emoji while processing | None | 🟡 |
| **Thinking Levels** | off/minimal/low/medium/high/xhigh fine control | Single thinking | 🟡 |
| **Message Chunking** | 4000 char + paragraph-first mode (`chunkMode: "newline"`) | Simple char split | 🟡 |

### 3.2 OpenClaw Key Code Reference

#### Streaming API Usage (src/slack/streaming.ts)

```typescript
// OpenClaw uses Slack native streaming API
const streamer = client.chatStream({
  channel,
  thread_ts: threadTs,
  recipient_team_id: teamId,
  recipient_user_id: userId,
});

// Three-stage streaming handling
await startSlackStream({...})
await appendSlackStream({text: chunk, ...})
await stopSlackStream({...})
```

#### Message Chunking (src/slack/send.ts)

```typescript
// OpenClaw supports paragraph-first chunking
chunkMarkdownTextWithMode(text, {
  mode: 'newline',  // paragraph priority
  limit: 4000,
})
```

#### Slack Configuration (docs/channels/slack.md)

```yaml
channels:
  slack:
    streaming: partial    # off/partial/block/progress
    nativeStreaming: true # Use Slack native streaming API
    chunkMode: newline   # paragraph-first splitting
    textChunkLimit: 4000
```

---

## 4. Slack API Best Practices

### 4.1 Streaming Message Throttling Strategy

```go
// Recommended: Implement message update throttler
type ThrottledUpdater struct {
    minInterval  time.Duration  // Minimum 600ms
    minCharDelta int             // Minimum 50 chars
    lastUpdate  time.Time
    lastLen     int
}

func (u *ThrottledUpdater) ShouldUpdate(newLen int) bool {
    return (newLen - u.lastLen) >= u.minCharDelta &&
           time.Since(u.lastUpdate) >= u.minInterval
}
```

### 4.2 Message Hierarchy Design

```
┌─────────────────────────────────────────┐
│ 🧠 Thinking: Analyzing code structure...│ ← Context Block
├─────────────────────────────────────────┤
│ 🔧 Using tool: Bash                     │ ← Section + accessory
│    └─ git status                       │
├─────────────────────────────────────────┤
│ ✅ Bash completed (150ms)               │ ← Section
│    On branch main...                    │
├─────────────────────────────────────────┤
│ 📊 Session Stats                        │ ← Header + Fields
│    ⏱️ 2.5s  │  📝 500/300 tokens      │
│    💰 $0.002 │  🔧 3 tools             │
└─────────────────────────────────────────┘
```

### 4.3 New Markdown Block (12000 chars)

```go
// Use new markdown block instead of section
func BuildMarkdownBlock(text string) []map[string]any {
    return []map[string]any{
        {
            "type": "markdown",
            "text": text,
        },
    }
}
```

---

## 5. Optimization Recommendations

### 5.1 Phase 1: Quick Improvements (1-2 days)

| Optimization | Implementation Location | Expected Effect |
|--------------|------------------------|-----------------|
| Error message friendly | `block_builder.go:525` | `rate_limited` → "Server busy, please retry later" |
| Thinking content display | `block_builder.go:333` | Show actual thinking content |
| Ack reaction | `adapter.go` | Add 👀 reaction when sending |

### 5.2 Phase 2: Core Experience (1 week)

| Optimization | Implementation Location | Expected Effect |
|--------------|------------------------|-----------------|
| Streaming message throttling | New `throttled_updater.go` | Avoid rate limit, improve fluency |
| Smart message chunking | `chunker.go` | Keep code blocks intact |
| Tool execution progress | `block_builder.go:378` | Show execution stage |

### 5.3 Phase 3: Surpass OpenClaw (2+ weeks)

| Optimization | Implementation Location | Advantage |
|--------------|------------------------|-----------|
| Native Slack Streaming API | `adapter.go` | Smoother than OpenClaw |
| Thinking level control | New config | Finer than OpenClaw |
| App Home Tab | New `home_tab.go` | Persistent UI beyond messages |

---

## 6. Key File Reference

| File | Lines | Function |
|------|-------|----------|
| `provider/claude_provider.go` | 152-450 | Event parsing |
| `chatapps/engine_handler.go` | 36-100 | Event → Message conversion |
| `chatapps/processor_aggregator.go` | 29-461 | Message aggregation |
| `chatapps/slack/block_builder.go` | 324-550 | Block Kit building |
| `chatapps/slack/chunker.go` | 68-180 | Message chunking |
| `chatapps/slack/adapter.go` | 790-1400 | Message sending & interaction |

---

## 7. Competitive Summary

### OpenClaw Points to Learn:

1. **Slack Native Streaming API** - Use `chat.startStream` for true streaming
2. **Streaming Mode Configuration** - `partial/block/progress` three modes
3. **Ack Reaction Mechanism** - Show feedback while processing
4. **Thinking Levels** - Fine control over thinking process display
5. **Paragraph-first Chunking** - Keep semantic integrity

### HotPlex Surpass Opportunities:

1. **Faster Response** - Optimize event processing latency
2. **Richer Event Display** - Show more metadata in tool_result
3. **App Home Tab** - Persistent session management & stats
4. **Better Error Recovery** - Smart retry + user-friendly hints

---

## Appendix: Event Field Details

### Metadata Fields

```go
type EventMeta struct {
    // Time related
    DurationMs      int64  `json:"duration_ms"`
    TotalDurationMs int64  `json:"total_duration_ms"`

    // Tool information
    ToolName    string `json:"tool_name"`
    ToolID      string `json:"tool_id"`
    Status      string `json:"status"`

    // Token stats
    InputTokens     int32 `json:"input_tokens"`
    OutputTokens    int32 `json:"output_tokens"`
    CacheWriteTokens int32 `json:"cache_write_tokens"`
    CacheReadTokens  int32 `json:"cache_read_tokens"`

    // File operations
    FilePath  string `json:"file_path"`
    LineCount int32  `json:"line_count"`

    // Progress tracking
    Progress    int32 `json:"progress"`
    TotalSteps int32 `json:"total_steps"`
    CurrentStep int32 `json:"current_step"`
}
```

---

*Generated: 2026-02-27*
*Purpose: Slack UX optimization research for HotPlex*
