# OpenCode SSE Event Exhaustion & ChatApp UI/UX Enhancement Design

**Date**: 2026-03-27
**Status**: Draft
**Author**: Claude Code
**Branch**: `feat/358-opencode-server-provider`

---

## 1. Problem Statement

The OpenCode Server Provider (`feat/358-opencode-server-provider`) implements SSE-based communication with the OpenCode `opencode serve` HTTP API. While the basic event-to-callback mapping is functional, there are three categories of gaps:

1. **Verification gap**: No exhaustive SSE event coverage validation exists — the event types and metadata fields are inferred from code, not empirically verified against real server output.
2. **Mapping gap**: Several high-value metadata fields from OpenCode SSE events are parsed but not forwarded downstream (ModelID, Error.Data, reasoning token usage).
3. **UI gap**: ChatApp presentation layer does not take full advantage of available event metadata to provide rich, contextual UI feedback.

---

## 2. Solution Overview

Four-phase incremental approach:

```
Phase 1: Python SSE exhaustive verification script
Phase 2: Go mapping enhancements (fill metadata gaps)
Phase 3: ChatApp UI/UX enhancements (leverage the metadata)
Phase 4: Verification loop (re-run script to confirm improvements)
```

---

## 3. Phase 1 — Python SSE Exhaustive Verification Script

### 3.1 Location
`scripts/verify/verify_opencode_sse_events.py`

### 3.2 Objectives
- Capture real SSE events from a live `opencode serve` instance
- Systematically trigger all OCPart types and SSE event types
- Produce a structured coverage report (event type catalog, delta examples, metadata field coverage)

### 3.3 Prompt Test Matrix

| Test ID | Prompt Type | Expected SSE Events Triggered |
|---------|-------------|------------------------------|
| T1 | Simple text (`"Hello"`) | `message.part.updated` (text) → `message.updated` |
| T2 | Reasoning (`"Explain why..."`) | `message.part.updated` (reasoning) → `message.part.updated` (text) |
| T3 | Tool call (`"List files"`) | `message.part.updated` (tool:pending/running) → `message.part.updated` (tool:completed) |
| T4 | Multi-step (`"Analyze this code"`) | `step-start` → reasoning/tool parts → `step-finish` |
| T5 | Error trigger (`"Run invalid command"`) | `session.error` or `message.part.updated` (tool:error) |
| T6 | Permission request (dangerous op) | `permission.updated` |
| T7 | Multi-turn (repeat T1) | `session.idle` → re-send → cycle |

### 3.4 Implementation

```
Session lifecycle per test case:
1. POST /session → get session_id
2. POST /session/{id}/message → trigger event stream
3. GET /event (streaming) → capture all events
4. DELETE /session/{id} → cleanup

SSE parsing:
- requests.get(stream=True) + iter_lines()
- Decode unicode, split on "data: " prefix
- json.loads() each data line → OCSSEEvent structure
- Categorize by evt.Type + OCPart.Type
- Archive raw JSON for reference

Output report:
- Total events captured per test case
- Event type coverage: expected vs. actual
- OCPart type distribution
- Metadata field population rate
- Delta vs. full-text examples (for text/reasoning parts)
- Missing fields / unexpected events
```

### 3.5 Expected Outputs
```
SSE Event Coverage Report
=========================
Total events captured: N
Event types seen: [list]
OCPart types seen: [list]
Metadata fields populated: [list]
Metadata fields missing: [list]
Delta vs. full text: [examples]

Test Case Results:
[T1] Simple text: PASS (2 events, text+updated)
[T2] Reasoning: PASS (3 events, reasoning+text+updated)
...
```

### 3.6 Error Handling
- Server not running → print warning + exit 1 (do not block Go changes)
- Session creation fails → skip test case, continue
- Timeout (30s) → close stream, archive captured events
- Malformed JSON → log and skip, continue

---

## 4. Phase 2 — Go Mapping + Runner Enhancements

### 4.1 Files Modified

| File | Changes |
|------|---------|
| `provider/opencode_server_provider.go` | mapEvent/mapPart: forward ModelID, Error.Data, reasoning tokens, step numbers, finish reason |
| `engine/runner.go` | Add dispatch cases for EventTypeStepStart/Finish; fix EventTypeError to use EventWithMeta; route ModelID to SessionStats |
| `event/events.go` | Add `Model string` field to `EventMeta` |
| `provider/opencode_types.go` | Add `ReasonEndTurn` to `finishReasonMessages` map |

### 4.2 Gap Analysis (Updated)

| Gap | Current Behavior | Desired Behavior |
|-----|-----------------|-----------------|
| `OCAssistantMessage.ModelID` | Not forwarded | → `ProviderEventMeta.Model` → `EventMeta.Model` |
| `OCError.Data` | Not forwarded | → `ProviderEvent.Content` (structured JSON) |
| `reasoning` usage | No token metadata | → `ProviderEventMeta.OutputTokens` |
| `OCPartStepFinish.StepNumber`/`TotalSteps` | Not forwarded | → `ProviderEventMeta.CurrentStep`/`TotalSteps` |
| `OCPartStepFinish.Reason` | Only hardcoded messages | Forward original value as `ProviderEvent.Status` |
| `EventMeta` struct | No `Model` field | Add `Model string` field |
| `dispatchNormalizedCallback` | No `step_start`/`step_finish` cases | Add explicit switch cases |
| `EventTypeError` dispatch | `callback("error", pevt.Error)` — loses metadata | `callback("error", event.NewEventWithMeta(..., pevt.Content))` |
| `finishReasonMessages` | Missing `end_turn` entry | Add `ReasonEndTurn: "✅ 回答生成完毕"` |
| `session.error` dispatch | Uses `pevt.Error` string only | Include `pevt.Content` (Error.Data JSON) via EventWithMeta |

### 4.3 `message.updated` — ModelID Forwarding

```go
// In mapEvent(), case OCEventMessageUpdated:
case OCEventMessageUpdated:
    var props struct {
        Info OCAssistantMessage `json:"info"`
    }
    if err := json.Unmarshal(evt.Properties, &props); err != nil {
        return nil, fmt.Errorf("parse message updated: %w", err)
    }
    if props.Info.Finish != "" {
        return []*ProviderEvent{{
            Type:    EventTypeResult,
            RawType: evt.Type,
            Metadata: &ProviderEventMeta{
                InputTokens:  props.Info.Tokens.InputTokens,
                OutputTokens: props.Info.Tokens.OutputTokens,
                TotalCostUSD: props.Info.Cost,
                Model:        props.Info.ModelID, // ← NEW
            },
        }}, nil
    }
    return nil, nil
```

### 4.4 `OCPartReasoning` — Add Usage Metadata

```go
// In mapPart(), case OCPartReasoning:
case OCPartReasoning:
    c := delta
    if c == "" {
        c = part.Text
    }
    meta := &ProviderEventMeta{}
    if part.Usage != nil {
        meta.OutputTokens = part.Usage.OutputTokens
    }
    return []*ProviderEvent{{
        Type:     EventTypeThinking,
        Content:  c,
        Metadata: meta,
    }}, nil
```

### 4.5 `session.error` — Forward Error.Data via EventWithMeta

```go
// In mapEvent(), case OCEventSessionError:
case OCEventSessionError:
    var props OCSessionErrorProps
    if err := json.Unmarshal(evt.Properties, &props); err != nil {
        return nil, fmt.Errorf("parse session error: %w", err)
    }
    msg := "unknown error"
    if props.Error.Name != "" {
        msg = props.Error.Name
    }
    errData, _ := json.Marshal(props.Error.Data) // Preserve structured error context
    return []*ProviderEvent{{
        Type:    EventTypeError,
        Error:   msg,
        IsError: true,
        Content: string(errData), // Forward structured error data
    }}, nil
```

### 4.6 `OCPartStepFinish` — Forward StepNumber, TotalSteps, and Original Reason

```go
// In mapPart(), case OCPartStepFinish:
case OCPartStepFinish:
    meta := &ProviderEventMeta{}

    if part.Usage != nil {
        meta.InputTokens = part.Usage.InputTokens
        meta.OutputTokens = part.Usage.OutputTokens
        if part.Usage.Cache != nil {
            meta.CacheReadTokens = part.Usage.Cache.Read
            meta.CacheWriteTokens = part.Usage.Cache.Write
        }
    }

    // NEW: forward step numbers for progress UI
    meta.CurrentStep = int32(part.StepNumber)
    meta.TotalSteps = int32(part.TotalSteps)

    // Map finish reason to user-friendly message, but preserve original for UI branching
    content := finishReasonMessages[FinishReason(part.Reason)]

    return []*ProviderEvent{{
        Type:     EventTypeStepFinish,
        Content:  content,
        Metadata: meta,
        Status:   string(part.Reason), // Forward original reason as Status
    }}, nil
```

### 4.7 `event/events.go` — Add `Model` Field to `EventMeta`

```go
// In EventMeta struct, add:
Model string `json:"model"` // Model ID used for this event (e.g., "claude-sonnet-4-20250514")
```

**Rationale**: `ProviderEventMeta.Model` is set by Phase 2's `mapEvent()` from `OCAssistantMessage.ModelID`. However, `engine/runner.go:dispatchNormalizedCallback()` constructs `EventWithMeta` via `ToEventWithMeta()` (`provider/event.go:186-205`), which only copies DurationMs, TotalDurationMs, and token fields — not `Model`. Adding `Model` to `EventMeta` closes this gap so the field survives the runner boundary.

### 4.8 `engine/runner.go` — Add Step Events + Fix Error Dispatch + Route ModelID

#### 4.8.1 Add `EventTypeStepStart` and `EventTypeStepFinish` cases to `dispatchNormalizedCallback`

**Current issue**: `EventTypeStepStart` and `EventTypeStepFinish` are not handled in the switch at `runner.go:744-890`. They fall through to `default`, which only fires when `pevt.Content != ""`. Since `step-start`/`step-finish` parts carry no meaningful `Content`, these events are silently swallowed — making all Phase 3 step-progress UI handlers unreachable.

**Fix** (add before the `default` case at `runner.go:881`):

```go
case provider.EventTypeStepStart:
    r.logger.Info("[RUNNER] Step start event received",
        "session_id", pevt.SessionID,
        "current_step", pevt.Metadata.CurrentStep,
        "total_steps", pevt.Metadata.TotalSteps)
    meta := &event.EventMeta{
        CurrentStep:      pevt.Metadata.CurrentStep,
        TotalSteps:       pevt.Metadata.TotalSteps,
        TotalDurationMs:   totalDur,
    }
    return callback("step_start", event.NewEventWithMeta("step_start", pevt.Content, meta))

case provider.EventTypeStepFinish:
    r.logger.Info("[RUNNER] Step finish event received",
        "session_id", pevt.SessionID,
        "current_step", pevt.Metadata.CurrentStep,
        "total_steps", pevt.Metadata.TotalSteps,
        "reason", pevt.Status)
    meta := &event.EventMeta{
        CurrentStep:       pevt.Metadata.CurrentStep,
        TotalSteps:        pevt.Metadata.TotalSteps,
        Status:            pevt.Status, // Original finish reason
        TotalDurationMs:   totalDur,
        Model:             pevt.Metadata.Model, // ← NEW: carry model through
    }
    return callback("step_finish", event.NewEventWithMeta("step_finish", pevt.Content, meta))
```

#### 4.8.2 Fix `EventTypeError` dispatch to use `EventWithMeta`

**Current issue** (`runner.go:818-819`):
```go
case provider.EventTypeError:
    return callback("error", pevt.Error)  // ← loses pevt.Content (Error.Data)
```

**Fix**:
```go
case provider.EventTypeError:
    meta := &event.EventMeta{
        ErrorMsg:        pevt.Error,
        TotalDurationMs: totalDur,
    }
    // pevt.Content carries OCError.Data JSON from provider
    return callback("error", event.NewEventWithMeta("error", pevt.Content, meta))
```

#### 4.8.3 Route `ModelID` to `SessionStats` via `EventTypeResult`

**Current issue** (`runner.go:704`): `ModelUsed: r.provider.Name()` sets the provider's display name ("OpenCode (Server)"), not the actual model ID.

**Fix**: In `EventTypeResult` case, extract `ProviderEventMeta.Model` and set `SessionStats.ModelUsed`:
```go
case provider.EventTypeResult:
    // ... existing result handling ...
    modelUsed := r.provider.Name()
    if pevt.Metadata != nil && pevt.Metadata.Model != "" {
        modelUsed = pevt.Metadata.Model // Prefer actual model ID from SSE
    }
    // ... in SessionStatsData construction:
    ModelUsed: modelUsed,
```

### 4.9 `provider/opencode_types.go` — Add Missing `finishReasonMessages` Entry

**Current issue**: `ReasonEndTurn` is defined (`opencode_types.go:143`) but has no entry in `finishReasonMessages`, causing empty content for normal turn completions.

```go
var finishReasonMessages = map[FinishReason]string{
    ReasonMaxTokens: "⚠️ Token 限制达到，建议增加配额",
    ReasonToolUse:   "🔧 需要执行工具调用",
    ReasonEndTurn:   "✅ 回答生成完毕", // ← NEW
}
```

### 4.10 `provider/opencode_server_provider.go` — Debug Log for Unknown Session Status

**Minor improvement**: Add debug log for unknown `session.status` types (currently silently returns nil):

```go
case OCEventSessionStatus:
    var props OCSessionStatusProps
    if err := json.Unmarshal(evt.Properties, &props); err != nil {
        return nil, fmt.Errorf("parse session status: %w", err)
    }
    switch props.Status.Type {
    case "busy":
        return []*ProviderEvent{{Type: EventTypeSystem, Status: "running"}}, nil
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
        p.logger.Debug("Unknown session status type", "status_type", props.Status.Type)
        return nil, nil
    }
```

---

## 5. Phase 3 — ChatApp UI/UX Enhancements

### 5.1 Scope
Files modified:
- `chatapps/engine_handler.go` (event handlers)
- `chatapps/slack/` (Slack-specific Block Kit rendering)
- `chatapps/base/` (common MessageBuilder)

### 5.2 Enhancement Matrix

#### 5.2.1 Reasoning Collapsible Block

**Trigger**: `EventTypeThinking` with non-empty content
**Goal**: Show thinking process without flooding the thread

```
🧠 深度推演中...
┌─ 👇 点击展开推演过程 ──────────────────┐
│ (thinking content here...)             │
└───────────────────────────────────────┘
```

**Implementation**:
- Slack: `section` block + `actions` block with button for expand/collapse
- Fallback: Plain message with 🧠 prefix if platform lacks collapsible support
- Platform detection via `base.SupportsCollapsible()`

**Behavior**:
- Show "展开" button when reasoning starts
- Replace with collapsed summary when reasoning ends
- Auto-cleanup after 10s to keep thread clean

#### 5.2.2 Tool Execution Status Cards

**Trigger**: `EventTypeToolUse` → `EventTypeToolResult`
**Goal**: Rich, contextual tool status with metadata

```
🔧 Bash 执行中: ls -la /tmp
✅ Bash 完成 (234ms) | 12行 | 3文件
```

**Metadata Used**:
- `Meta.DurationMs` → display as "(XXXms)"
- `Meta.LineCount` + `Meta.FilePath` → "N行, X文件"
- `Meta.OutputSummary` → first N chars of result
- `Meta.ErrorMsg` (on failure) → red error message

**Implementation**:
- `handleToolUse()`: Show category-emoji + tool name + truncated input
- `handleToolResult()`: Update to "✅/⚠️ 完成" with duration
- 3s sliding window deletion (existing mechanism)

#### 5.2.3 Step Progress Indicator

**Trigger**: `EventTypeStepStart` / `EventTypeStepFinish`
**Goal**: Visual progress feedback for multi-step tasks

```
🔍 正在分析执行轨迹... [Step 2/5]
✅ 当前任务阶段构建完成 [Step 5/5]
```

**Metadata Used**:
- `Meta.CurrentStep` → "Step N"
- `Meta.TotalSteps` → "/N"
- Combined: "Step N/N ████░░░░░░ 40%"

**Implementation**:
- `handleStepStart()`: Update status to `StatusStepStartLabel` + progress metadata
- `handleStepFinish()`: Update status to `StatusStepFinishLabel` + final step
- Use Slack reaction or `updateStatusMessage()` for in-place updates

#### 5.2.4 Permission Card Enhancement

**Trigger**: `EventTypePermissionRequest`
**Goal**: Richer permission context

**Current**: Shows tool name + basic message
**Enhanced**: Shows tool type label + detailed context

```
┌──────────────────────────────────────┐
│ 🛡️ 拦截到高危操作                    │
│ Type: Bash | Command: rm -rf /tmp/*  │
│ [ Allow Once ] [ Always ] [ Deny ]   │
└──────────────────────────────────────┘
```

**Metadata Used**:
- `OCPermissionProps.Type` → "Bash / Read / Write"
- `OCPermissionProps.Title` → command summary
- `OCError.Data` (if available) → detailed context

#### 5.2.5 Session Stats Card Enhancement

**Trigger**: `EventTypeResult` (session end)
**Goal**: Add model info and finish reason to session stats card

**Current metadata shown**: tokens, duration, tool count, provider name ("OpenCode (Server)")
**Enhanced metadata**:
- `EventMeta.Model` → "Model: claude-sonnet-4-20250514" (from SSE `OCAssistantMessage.ModelID`)
- Finish reason → "End reason: tool_use / end_turn / max_tokens" (from last `step_finish` event)

**Data flow for ModelID**:
```
OCAssistantMessage.ModelID
    → ProviderEventMeta.Model (Phase 2, opencode_server_provider.go)
    → EventMeta.Model (Phase 2, event/events.go + runner.go)
    → SessionStatsData.ModelUsed (Phase 2, runner.go: modelUsed variable)
    → handleSessionStats() → stats_card (Phase 3, engine_handler.go)
```

**Data flow for Finish Reason** (requires new struct field):
```
OCPartStepFinish.Reason
    → ProviderEvent.Status (Phase 2, opencode_server_provider.go)
    → Stored in engine session context or routed via EventWithMeta.Status
    → SessionStatsData.FinishReason (Phase 2: add field to event/events.go)
    → handleSessionStats() → displayed in stats card (Phase 3)
```

**Implementation**:
1. **Phase 2**: Add `FinishReason string` field to `SessionStatsData` in `event/events.go`
2. **Phase 2**: In `runner.go` result construction, track last `step_finish.Status` and set `FinishReason`
3. **Phase 3**: `engine_handler.go:handleSessionStats()` reads `stats.FinishReason` and includes in card metadata

### 5.2.6 Error Context Card Enhancement

**Trigger**: `EventTypeError` (Phase 2 fix)
**Goal**: Show structured error context from `OCError.Data`

**Current**: Shows only `pevt.Error` string (error name)
**Enhanced**:
- Error name from `OCError.Name` → card header
- Error data from `OCError.Data` (JSON) → structured context fields

**Implementation**:
- Phase 2 changes `EventTypeError` dispatch to use `EventWithMeta`
- `engine_handler.go:handleError()` already reads `v.Meta.ErrorMsg` from `EventWithMeta`
- Add parsing of `EventWithMeta.EventData` (OCError.Data JSON) to extract structured fields for display

### 5.3 Platform Abstraction
All UI enhancements use the existing `StatusManager` abstraction, so changes in `engine_handler.go` apply to all chatapp platforms (Slack, DingTalk, etc.). Slack-specific Block Kit rendering stays in `chatapps/slack/`.

### 5.4 Degradation Strategy
If a platform lacks support for rich features:
- Collapsible blocks → plain 🧠 prefixed message
- Progress indicators → text-only status update
- Permission cards → text-only permission card
- Never block on missing platform features

---

## 6. Phase 4 — Verification Loop

### 6.1 Strategy
Re-run the Phase 1 Python script after Phase 2+3 changes to verify:

1. **Coverage improvement**: All expected event types are captured
2. **Metadata completeness**: New fields (ModelID, Error.Data, reasoning tokens) are populated
3. **No regression**: Existing event handling still works correctly

### 6.2 `--verify` Flag
```bash
python3 scripts/verify/verify_opencode_sse_events.py --verify --baseline=report_v1.json --output=report_v2.json
```
- `--baseline`: Previous report for diff comparison
- `--output`: New report
- Exit code 0 if no new gaps, 1 if gaps remain

### 6.3 CI Integration
After Phase 2+3 changes, add the verification script to `scripts/verify/` and optionally add as a pre-commit check (non-blocking).

---

## 7. Data Flow Architecture

```
┌─────────────────────────────────────────────────────────┐
│  OpenCode Server (opencode serve)                        │
│  SSE endpoint: GET /event                                │
│  Message endpoint: POST /session/{id}/message            │
└──────────────────────────┬──────────────────────────────┘
                           │ HTTP
                           ▼
┌─────────────────────────────────────────────────────────┐
│  HTTPTransport (transport_http.go)                       │
│  - Parses "data: " prefix                               │
│  - Fan-out to subscribers via channel                   │
└──────────────────────────┬──────────────────────────────┘
                           │ raw JSON strings
                           ▼
┌─────────────────────────────────────────────────────────┐
│  HTTPSessionIO (session_io.go)                           │
│  - Subscribes to transport                               │
│  - Emits "raw_line" events                              │
└──────────────────────────┬──────────────────────────────┘
                           │ raw lines
                           ▼
┌─────────────────────────────────────────────────────────┐
│  Engine.handleStreamRawLine (runner.go)                  │
│  - Calls provider.ParseEvent()                          │
│  - Dispatches to provider.mapEvent()                    │
└──────────────────────────┬──────────────────────────────┘
                           │ []*ProviderEvent
                           ▼
┌─────────────────────────────────────────────────────────┐
│  opencode_server_provider.go (Phase 2 changes)           │
│  - mapEvent() → normalizes SSE → ProviderEvent           │
│  - mapPart() → OCPart → ProviderEvent                    │
│  NEW: forwards ModelID, Error.Data, reasoning tokens     │
└──────────────────────────┬──────────────────────────────┘
                           │ []*ProviderEvent with metadata
                           ▼
┌─────────────────────────────────────────────────────────┐
│  Engine.dispatchNormalizedCallback (runner.go)           │
│  - Wraps in event.EventWithMeta                         │
│  - Routes by type to stats tracking                     │
└──────────────────────────┬──────────────────────────────┘
                           │ event.Callback (eventType + data)
                           ▼
┌─────────────────────────────────────────────────────────┐
│  StreamCallback.Handle (engine_handler.go) (Phase 3)     │
│  - Routes by event type to handler methods               │
│  - Updates StatusManager + sends messages                │
│  NEW: richer status, collapsible reasoning, tool cards   │
└──────────────────────────┬──────────────────────────────┘
                           │ StatusManager / AdapterManager
                           ▼
┌─────────────────────────────────────────────────────────┐
│  ChatApp Platform (Slack / DingTalk / etc.)              │
│  - Renders blocks, reactions, status updates             │
│  - Phase 3 UI enhancements applied here                   │
└─────────────────────────────────────────────────────────┘
```

---

## 8. File Changes Summary

| File | Phase | Changes |
|------|-------|---------|
| `scripts/verify/verify_opencode_sse_events.py` | 1 | New file — exhaustive SSE capture + structured coverage report |
| `provider/opencode_server_provider.go` | 2 | mapEvent/mapPart: forward ModelID, Error.Data JSON, reasoning tokens, StepNumber/TotalSteps, finish reason; add debug log for unknown status |
| `engine/runner.go` | 2 | Add `EventTypeStepStart`/`StepFinish` switch cases (fixes silent swallow); fix `EventTypeError` to use `EventWithMeta`; route `ModelID` to `SessionStatsData.ModelUsed` |
| `event/events.go` | 2 | Add `Model string` field to `EventMeta` struct; add `FinishReason string` field to `SessionStatsData` struct |
| `provider/opencode_types.go` | 2 | Add `ReasonEndTurn` entry to `finishReasonMessages` map |
| `chatapps/engine_handler.go` | 3 | handleThinking: richer metadata; handleToolUse/ToolResult: enhanced context; handleStepStart/Finish: step progress UI; handleSessionStats: Model display, finish reason; handleError: parse EventData JSON for structured context |
| `chatapps/slack/` | 3 | Block Kit rendering for collapsible reasoning, tool cards (if needed) |
| `chatapps/base/` | 3 | CategoryStatusLabel improvements, degradation helpers (if needed) |

---

## 9. Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| OpenCode Server not running | Medium | Script warns + exits; does not block Go changes |
| New SSE event types discovered | Low | `mapEvent` default returns nil — no crash |
| Platform rendering differences | Medium | Degradation to plain text for unsupported features |
| Python script timeout on long sessions | Medium | 30s timeout per test case, graceful close |
| Duplicate events causing UI spam | Low | StatusManager deduplication + throttle (existing) |

---

## 10. Acceptance Criteria

- [ ] `verify_opencode_sse_events.py` captures all 7 test cases and produces a structured report
- [ ] All 6 SSE event types and 5 OCPart types are accounted for in the report
- [ ] `runner.go:dispatchNormalizedCallback` has explicit `EventTypeStepStart` and `EventTypeStepFinish` cases
- [ ] `EventTypeStepStart`/`StepFinish` events in `runner.go` forward `CurrentStep`/`TotalSteps` metadata
- [ ] `EventTypeError` dispatch in `runner.go` uses `event.NewEventWithMeta()` (not bare string)
- [ ] `EventMeta` in `event/events.go` has a `Model string` field
- [ ] `finishReasonMessages` in `opencode_types.go` has all three entries: `max_tokens`, `tool_use`, `end_turn`
- [ ] `ProviderEventMeta.Model` is populated from `OCAssistantMessage.ModelID` in `message.updated` events
- [ ] `ProviderEvent.Content` contains `OCError.Data` JSON (as string) for `session.error` events
- [ ] `ProviderEventMeta.OutputTokens` is populated for `reasoning` parts
- [ ] `ProviderEventMeta.CurrentStep`/`TotalSteps` are forwarded for `step-finish` parts
- [ ] `ProviderEvent.Status` carries original `OCPartStepFinish.Reason` value
- [ ] `SessionStatsData` in `event/events.go` has both `Model string` (in EventMeta) and `FinishReason string` fields
- [ ] `SessionStatsData.ModelUsed` reflects actual `ModelID` from SSE, not just provider name
- [ ] `SessionStatsData.FinishReason` is set from the last `step_finish` event's original reason
- [ ] ChatApp shows model name in session stats card
- [ ] ChatApp shows finish reason (end_turn / tool_use / max_tokens) in session stats card
- [ ] ChatApp shows step progress (N/M) in status indicator (verifiable via `handleStepStart`/`handleStepFinish` called)
- [ ] ChatApp shows structured error context for `session.error` events
- [ ] `go build ./...` and `go test ./...` pass after Phase 2 changes
- [ ] Re-run of verification script shows improved metadata coverage
