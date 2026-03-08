# Thread Ownership Policy Design

> Issue: #241
> Status: Draft

## Problem Statement

In channels with multiple HotPlex bots (container-isolated processes), when a user creates a Thread, multiple bots responding simultaneously creates noise and confusion.

### Current Behavior

| Scenario | Behavior |
|----------|----------|
| Channel + @mention | Bot responds |
| Channel + no @ | Polite broadcast response |
| Thread + @mention | Bot responds |
| Thread + no @ | Polite broadcast response (same as channel) |

### Issues

1. **Noise Pollution** - All bots send "polite responses" in threads
2. **User Confusion** - User doesn't know which bot is "theirs"
3. **Context Fragmentation** - Conversation split across multiple bots

---

## Proposed Solution

### Thread Ownership Policy

**Core Concept**: First bot mentioned in a Thread "owns" that thread.

- Owner responds to all subsequent messages in that thread
- Other bots stay silent unless explicitly @mentioned
- Ownership transfers on explicit @mention of another bot

---

## Design Details

### 1. Data Structure

```go
// chatapps/slack/thread_ownership.go

// ThreadOwnership tracks which bot owns a specific thread
type ThreadOwnership struct {
    mu       sync.RWMutex
    threads  map[string]*OwnerInfo // key: "channelID:threadTS"
    ttl      time.Duration
    logger   *slog.Logger
}

type OwnerInfo struct {
    BotUserID   string    // Owning bot's user ID
    ClaimedAt   time.Time // When ownership was claimed
    LastActive  time.Time // Last activity timestamp
}

func NewThreadOwnership(ttl time.Duration, logger *slog.Logger) *ThreadOwnership {
    if ttl == 0 {
        ttl = 24 * time.Hour
    }
    return &ThreadOwnership{
        threads: make(map[string]*OwnerInfo),
        ttl:     ttl,
        logger:  logger,
    }
}
```

### 2. Core Methods

```go
// Get retrieves the owner of a thread
func (t *ThreadOwnership) Get(channelID, threadTS string) string {
    key := t.key(channelID, threadTS)
    t.mu.RLock()
    defer t.mu.RUnlock()

    info, exists := t.threads[key]
    if !exists || time.Since(info.LastActive) > t.ttl {
        return ""
    }
    return info.BotUserID
}

// Set assigns ownership of a thread
func (t *ThreadOwnership) Set(channelID, threadTS, botUserID string) {
    key := t.key(channelID, threadTS)
    t.mu.Lock()
    defer t.mu.Unlock()

    t.threads[key] = &OwnerInfo{
        BotUserID:  botUserID,
        ClaimedAt:  time.Now(),
        LastActive: time.Now(),
    }
    t.logger.Debug("Thread ownership set",
        "channel", channelID,
        "thread_ts", threadTS,
        "owner", botUserID)
}

// key generates a unique key for thread lookup
func (t *ThreadOwnership) key(channelID, threadTS string) string {
    return channelID + ":" + threadTS
}

// Cleanup removes expired entries
func (t *ThreadOwnership) Cleanup() {
    t.mu.Lock()
    defer t.mu.Unlock()

    now := time.Now()
    for key, info := range t.threads {
        if now.Sub(info.LastActive) > t.ttl {
            delete(t.threads, key)
        }
    }
}
```

### 3. Decision Flow

```
User sends message in Thread
           │
           ▼
┌─────────────────────┐
│ Extract thread_ts   │
│ Extract channel_id  │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐    YES    ┌─────────────────────┐
│ Has @mention?       ├──────────►│ Check who is @'d    │
└─────────┬───────────┘           │ - @self → respond   │
          │ NO                    │ - @other → transfer │
          ▼                       └─────────────────────┘
┌─────────────────────┐
│ Query thread owner  │
│ (cache or API)      │
└─────────┬───────────┘
          │
    ┌─────┴─────┐
    ▼           ▼
 Has owner    No owner
    │             │
    ▼             ▼
┌───────┐   ┌─────────────────┐
│Is self│   │ Check thread    │
│?      │   │ history via API │
└───┬───┘   └────────┬────────┘
    │                │
    ▼                ▼
 YES → respond    Found bot reply?
 NO  → silent        │
                    ▼
              YES → claim & respond
              NO  → silent
```

### 4. API Integration

```go
// getThreadOwnerFromAPI fetches thread history to determine owner
func (a *Adapter) getThreadOwnerFromAPI(ctx context.Context, channelID, threadTS string) (string, error) {
    msgs, _, err := a.client.GetConversationRepliesContext(ctx,
        &slack.GetConversationRepliesParameters{
            ChannelID: channelID,
            Timestamp: threadTS,
            Limit:     100,
        },
    )
    if err != nil {
        return "", fmt.Errorf("get conversation replies: %w", err)
    }

    // Find first bot reply (they own the thread)
    for _, msg := range msgs {
        if msg.BotID != "" {
            return msg.BotID, nil
        }
    }

    return "", nil // No bot has replied yet
}
```

### 5. Config Extension

```go
type Config struct {
    // ...existing fields...

    // ThreadPolicy controls bot behavior in threads:
    // - "broadcast": Same as channel (polite response on no @) [default for backward compat]
    // - "silent": No response without @mention
    // - "ownership": First responder owns the thread
    ThreadPolicy string

    // ThreadOwnershipTTL: How long to remember thread ownership
    // Default: 24h
    ThreadOwnershipTTL time.Duration
}
```

### 6. Event Handling Integration

```go
// In handleEventCallback (events.go)

// Thread-specific handling
if msgEvent.ThreadTS != "" && a.config.ThreadPolicy != "broadcast" {
    shouldRespond, shouldClaim := a.shouldRespondInThread(ctx, msgEvent)
    if !shouldRespond {
        return
    }
    if shouldClaim {
        a.ownership.Set(msgEvent.Channel, msgEvent.ThreadTS, a.config.BotUserID)
    }
}

func (a *Adapter) shouldRespondInThread(ctx context.Context, msgEvent MessageEvent) (respond bool, claim bool) {
    channelID := msgEvent.Channel
    threadTS := msgEvent.ThreadTS

    // Check for explicit @mentions
    mentioned := ExtractMentionedUsers(msgEvent.Text)
    if len(mentioned) > 0 {
        if slices.Contains(mentioned, a.config.BotUserID) {
            // Explicitly @mentioned - respond and claim
            return true, true
        }
        // Other bot mentioned - stay silent
        return false, false
    }

    // No @mention - check ownership
    switch a.config.ThreadPolicy {
    case "silent":
        // Silent mode: no @ = no response
        return false, false

    case "ownership":
        owner := a.ownership.Get(channelID, threadTS)
        if owner == "" {
            // No cached owner - check API
            apiOwner, err := a.getThreadOwnerFromAPI(ctx, channelID, threadTS)
            if err != nil {
                a.logger.Warn("Failed to get thread owner", "error", err)
                return false, false
            }
            if apiOwner == "" {
                // No bot has replied - stay silent, wait for explicit @
                return false, false
            }
            owner = apiOwner
            a.ownership.Set(channelID, threadTS, owner)
        }

        // Only respond if we own this thread
        return owner == a.config.BotUserID, false

    default: // "broadcast"
        return true, false
    }
}
```

---

## Slack API Reference

### conversations.replies

- **Purpose**: Retrieve thread message history
- **Auth**: `bot` token required
- **Scopes**: `conversations:history` or `channels:history` + `groups:history`
- **Rate Limit**: Tier 3 (50+ requests/minute)

```go
// SDK usage
msgs, hasMore, nextCursor, err := client.GetConversationRepliesContext(ctx,
    &slack.GetConversationRepliesParameters{
        ChannelID: "C1234567890",
        Timestamp: "1234567890.123456", // thread_ts
        Limit:     100,
        Cursor:    "", // for pagination
    },
)
```

### Best Practices (from Slack Docs)

1. **Keep chat titles updated** - Use `assistant.threads.setTitle`
2. **Status updates** - Use `assistant.threads.setStatus` for "thinking" state
3. **Context continuity** - Call `conversations.replies` to maintain context

---

## Migration Path

### Phase 1: Add Config (Non-breaking)

- Add `ThreadPolicy` with default `"broadcast"`
- Existing behavior unchanged

### Phase 2: Implement Ownership

- Add `thread_ownership.go`
- Implement `ThreadOwnership` struct
- Add API integration

### Phase 3: Update Event Handling

- Modify `handleEventCallback` to use thread policy
- Add ownership checks

### Phase 4: Documentation

- Update README with new config options
- Add migration guide

---

## Recommended Defaults

| Environment | ThreadPolicy | Rationale |
|-------------|-------------|-----------|
| Production | `silent` | Minimal noise |
| Multi-bot | `ownership` | Clear ownership |
| Legacy | `broadcast` | Backward compat |

---

## Open Questions

1. **Ownership persistence** - Should we persist ownership across restarts?
2. **Cross-channel threads** - How to handle thread_broadcast subtype?
3. **Bot identity sync** - How to know all bot user IDs in workspace?

---

## References

- [Slack AI Apps Best Practices](https://docs.slack.dev/ai/ai-apps-best-practices/)
- [conversations.replies API](https://docs.slack.dev/reference/methods/conversations.replies)
- Issue: #241
