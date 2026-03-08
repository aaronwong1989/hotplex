# Thread Ownership Policy Design

> Issue: #241
> Status: Draft (Updated)

## Problem Statement

In channels with multiple HotPlex bots (container-isolated processes), when a user creates a Thread, multiple bots responding simultaneously creates noise and confusion.

### Architecture Context

```
Channel C1 contains:
├── Container 1 (BotA) ─── maintains owned_threads: Set<string>
├── Container 2 (BotB) ─── maintains owned_threads: Set<string>
└── Container 3 (BotC) ─── maintains owned_threads: Set<string>
```

**Key**: Each bot is an independent container process with its own state. No shared state between bots.

### Issues

1. **Noise Pollution** - All bots send "polite responses" in threads
2. **User Confusion** - User doesn't know which bot is "theirs"
3. **Context Fragmentation** - Conversation split across multiple bots

---

## Proposed Solution

### Thread Ownership Rules

Each bot maintains its **own set of owned threads** (`owned_threads: Set<thread_key>`).

| Rule | Description |
|------|-------------|
| **R1: Creator Claims** | Bot that first replies to a thread claims ownership |
| **R2: Respond If Owner** | Bot only responds to threads it owns |
| **R3: @ Updates Ownership** | Valid @mention message updates thread ownership |
| **R4: Multi-Owner** | `@BotA @BotB` → Both BotA and BotB own the thread |
| **R5: Auto-Release** | Bot sees @mention without itself → releases ownership |
| **R6: No @ No Change** | Message without @ doesn't change ownership, all owners respond |

### Decision Flow (Per Bot)

```
User sends message in Thread
           │
           ▼
┌─────────────────────┐
│ Extract thread_ts   │
│ Extract channel_id  │
│ thread_key = ch:ts  │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐    YES    ┌──────────────────────────┐
│ Has @mention?       ├──────────►│ Am I in @mention list?   │
└─────────┬───────────┘           │ - YES → ADD to owned     │
          │ NO                    │ - NO  → REMOVE from owned│
          ▼                       │ Then: respond if added   │
┌─────────────────────┐           └──────────────────────────┘
│ Do I own this thread│
│ (check owned_set)?  │
└─────────┬───────────┘
          │
    ┌─────┴─────┐
    ▼           ▼
   YES         NO
    │           │
    ▼           ▼
 Respond     Silent
```

### Example Scenarios

#### Scenario 1: New Thread

```
1. User creates Thread T1 in Channel C1
2. User: "Hello?" (no @)
   → BotA, BotB, BotC all silent (no owner yet)
3. User: "@BotA help me"
   → BotA: claims ownership, responds
   → BotB, BotC: silent
4. User: "Thanks" (no @)
   → BotA: responds (owns T1)
   → BotB, BotC: silent
```

#### Scenario 2: Ownership Transfer

```
1. Thread T1 owned by BotA
2. User: "@BotB take over"
   → BotA: releases ownership (sees @BotB, not self)
   → BotB: claims ownership, responds
3. User: "Continue" (no @)
   → BotB: responds (now owns T1)
   → BotA: silent
```

#### Scenario 3: Multi-Owner

```
1. Thread T1 owned by BotA
2. User: "@BotA @BotB collaborate on this"
   → BotA: keeps ownership
   → BotB: claims ownership
3. User: "What do you think?" (no @)
   → BotA: responds
   → BotB: responds
   (Two responses)
```

---

## Design Details

### 1. Data Structure

```go
// chatapps/slack/thread_ownership.go

// ThreadOwnership tracks threads owned by THIS bot
type ThreadOwnership struct {
    mu      sync.RWMutex
    owned   map[string]*OwnedThreadInfo // key: "channelID:threadTS"
    ttl     time.Duration
    logger  *slog.Logger

    // Optional: persistence via storage plugin
    store   storage.ThreadOwnershipStore
}

type OwnedThreadInfo struct {
    ClaimedAt   time.Time // When ownership was claimed
    LastActive  time.Time // Last activity in this thread
}
```

### 2. Core Methods

```go
// IsOwned checks if THIS bot owns the thread
func (t *ThreadOwnership) IsOwned(channelID, threadTS string) bool {
    key := t.key(channelID, threadTS)
    t.mu.RLock()
    defer t.mu.RUnlock()

    info, exists := t.owned[key]
    if !exists {
        return false
    }
    // Check TTL
    if time.Since(info.LastActive) > t.ttl {
        return false
    }
    return true
}

// Claim adds thread to owned set
func (t *ThreadOwnership) Claim(channelID, threadTS string) {
    key := t.key(channelID, threadTS)
    t.mu.Lock()
    defer t.mu.Unlock()

    t.owned[key] = &OwnedThreadInfo{
        ClaimedAt:  time.Now(),
        LastActive: time.Now(),
    }
    t.logger.Debug("Thread ownership claimed",
        "channel", channelID,
        "thread_ts", threadTS)

    // Persist if storage enabled
    if t.store != nil {
        ctx := context.Background()
        _ = t.store.Store(ctx, channelID, threadTS, true)
    }
}

// Release removes thread from owned set
func (t *ThreadOwnership) Release(channelID, threadTS string) {
    key := t.key(channelID, threadTS)
    t.mu.Lock()
    defer t.mu.Unlock()

    delete(t.owned, key)
    t.logger.Debug("Thread ownership released",
        "channel", channelID,
        "thread_ts", threadTS)

    // Persist if storage enabled
    if t.store != nil {
        ctx := context.Background()
        _ = t.store.Store(ctx, channelID, threadTS, false)
    }
}

func (t *ThreadOwnership) key(channelID, threadTS string) string {
    return channelID + ":" + threadTS
}
```

### 3. Decision Logic

```go
// shouldRespondInThread determines if bot should respond
// Returns: (shouldRespond, ownershipAction)
// ownershipAction: "claim" | "release" | "none"
func (a *Adapter) shouldRespondInThread(msgEvent MessageEvent) (respond bool, action string) {
    channelID := msgEvent.Channel
    threadTS := msgEvent.ThreadTS

    // Check for @mentions
    mentioned := ExtractMentionedUsers(msgEvent.Text)
    iAmMentioned := slices.Contains(mentioned, a.config.BotUserID)

    if len(mentioned) > 0 {
        // Has @mentions - update ownership
        if iAmMentioned {
            // I am mentioned - claim and respond
            return true, "claim"
        }
        // Others mentioned but not me - release ownership
        if a.ownership.IsOwned(channelID, threadTS) {
            return false, "release"
        }
        return false, "none"
    }

    // No @mentions - check if I own this thread
    if a.ownership.IsOwned(channelID, threadTS) {
        return true, "none"
    }

    // No @, not owner - stay silent
    return false, "none"
}
```

### 4. Event Handling Integration

```go
// In handleEventCallback (events.go)

// Thread-specific handling (only when ThreadPolicy != "broadcast")
if msgEvent.ThreadTS != "" && a.config.ThreadPolicy == "ownership" {
    respond, action := a.shouldRespondInThread(msgEvent)

    // Apply ownership action
    switch action {
    case "claim":
        a.ownership.Claim(msgEvent.Channel, msgEvent.ThreadTS)
    case "release":
        a.ownership.Release(msgEvent.Channel, msgEvent.ThreadTS)
    }

    if !respond {
        a.logger.Debug("Thread: not responding",
            "channel", msgEvent.Channel,
            "thread_ts", msgEvent.ThreadTS,
            "reason", "not_owner")
        return
    }
}
```

### 5. Config Extension

```go
type Config struct {
    // ...existing fields...

    // ThreadPolicy controls bot behavior in threads:
    // - "broadcast": Same as channel (polite response on no @) [default, backward compat]
    // - "ownership": Track thread ownership (recommended for multi-bot)
    ThreadPolicy string

    // ThreadOwnershipTTL: How long to remember owned threads
    // Default: 24h
    ThreadOwnershipTTL time.Duration
}
```

### 6. Persistence Integration

When Storage Plugin is enabled, persist ownership:

```go
// storage/thread_ownership.go

type ThreadOwnershipStore interface {
    // Store saves ownership status
    // owned=true means this bot owns the thread
    Store(ctx context.Context, channelID, threadTS string, owned bool) error

    // IsOwned checks ownership from persistent storage
    IsOwned(ctx context.Context, channelID, threadTS string) (bool, error)
}
```

**Integration Point**: Reuse existing `MessageStorePlugin` infrastructure.

---

## Initialization Flow

```go
// On bot startup:
func (a *Adapter) initThreadOwnership() {
    if a.config.ThreadPolicy != "ownership" {
        return
    }

    // If storage enabled, load ownership from persistent store
    if a.store != nil {
        ownedThreads, err := a.store.LoadOwnedThreads(context.Background())
        if err != nil {
            a.logger.Warn("Failed to load thread ownership", "error", err)
            return
        }
        for _, t := range ownedThreads {
            a.ownership.Claim(t.ChannelID, t.ThreadTS)
        }
    }
}
```

---

## Fallback: Local Storage Query

When ownership not in memory but storage is enabled:

```go
func (a *Adapter) checkOwnershipFromStorage(ctx context.Context, channelID, threadTS string) bool {
    if a.store == nil {
        return false
    }

    // Query local storage for bot's previous messages in this thread
    msgs, err := a.store.List(ctx, &storage.MessageQuery{
        ChannelID: channelID,
        ThreadID:  threadTS,
        FromBotID: a.config.BotUserID,
        Limit:     1,
    })
    if err != nil {
        return false
    }

    // If bot has responded before, it owns the thread
    return len(msgs) > 0
}
```

---

## Summary

| Aspect | Design |
|--------|--------|
| **State** | Per-bot `owned_threads: Set<string>` |
| **Claim** | First response OR @mention of self |
| **Release** | @mention of others (not self) |
| **Query** | Memory first, local storage fallback |
| **Persist** | Via Storage Plugin (if enabled) |
| **No API** | No Slack API calls |

---

## References

- [Slack AI Apps Best Practices](https://docs.slack.dev/ai/ai-apps-best-practices/)
- Issue: #241
