# WebSocket Protocol Design Best Practices for Agent Gateway Platforms (HotPlex A1.0 AEP)

**Research Date**: 2026-03-30
**Focus**: Latest 2025-2026 patterns for event streaming, reconnection, state recovery, backpressure, security, and session persistence

**Based on**: Research of Discord Gateway, Slack RTM, OpenAI Realtime API, Ably best practices, Go patterns, and SQLite WAL documentation

---

## Executive Summary

This document provides **actionable design recommendations** for HotPlex's Agent Event Protocol (AEP) v1.0, focusing on patterns proven in production-scale real-time systems.

---

## 1. Event Streaming & Envelope Design
### 1.1 Versioned Envelope Pattern
**Recommendation**: Use a **versioned envelope with monotonically increasing sequence numbers.

```go
type AEPEnvelope struct {
    Version   int    `json:"version"`              // Protocol version (start with 1)
    Seq       int64  `json:"seq"`                 // Monotonically increasing sequence number
    Type      string `json:"type"`               // Event type (e.g., "message", "tool_use")
    Timestamp int64  `json:"timestamp"`           // Unix timestamp (milliseconds)
    ID        string `json:"id,omitempty"`        // Correlation ID (for request-response)
    Data      any    `json:"data"`                // Event payload
}
```

**Rationale**:
- **Version**: Enables protocol evolution without breaking clients (see Section 1.2)
- **Seq**: Critical for idempotency, replay, and ordering
- **ID**: Request-response correlation (like Slack Events API)
- **Timestamp**: Temporal ordering and debugging

**Industry Examples**:
- **Discord Gateway**: Uses `seq` for resume operations (Opcode 6)
- **Slack Events API**: Uses event IDs for deduplication
- **Ably**: Provides sequence numbers for exactly-once delivery

**Trade-offs**:
- **Seq-based replay**: Simple, but can replay large event ranges (slow reconnect)
- **Snapshot-based**: Complex, but faster recovery for long sessions (see Section 2)

### 1.2 Schema Evolution Strategy
**Recommendation**: **Additive-only changes** with version negotiation.

**Pattern**:
```go
// V1: Basic event
type ToolUseEventV1 struct {
    Type    string `json:"type"`
    ToolName string `json:"tool_name"`
    Input   any    `json:"input"`
}

// V2: Extended event (backward compatible)
type ToolUseEventV2 struct {
    Type        string `json:"type"`
    ToolName    string `json:"tool_name"`
    Input       any    `json:"input"`
    DurationMs  int64  `json:"duration_ms"` // New field
}
```

**Version Negotiation** (Handshake):
```go
type HandshakeRequest struct {
    Version          int    `json:"version"`
    SupportedVersions []int `json:"supported_versions"`
}

type HandshakeResponse struct {
    Version       int    `json:"version"`
    ServerVersion int    `json:"server_version"`
}
```

**Best Practices**:
- **Never remove required fields** (breaks clients)
- **New fields are optional** (backward compatible)
- **Deprecation timeline**: 6 months minimum (see Discord Gateway)
- **Validation**: Reject unknown event types with clear error messages

**Avoid**:
- **Renaming fields** (use new field instead)
- **Removing fields** (mark deprecated first)
- **Breaking changes** (use new event type instead)

---

## 2. Reconnection & State Recovery
### 2.1 Seq-Based Replay (Recommended)
**Use Case**: Short disconnections (< 5 minutes)

**Flow**:
```
Client                         Server
  │                              │
  │  1. Disconnect detected    │
  │                              │ 2. Reconnect with last_seq=42
  │                              │
  │  3. Replay events 43-50│ 3. Send events 43-50
  │                              │
  │  4. Resume streaming   └───────────────────────────
```

**Implementation**:
```go
type ReconnectRequest struct {
    SessionID string `json:"session_id"`
    LastSeq   int64  `json:"last_seq"`
}

type ReplayBatch struct {
    Events    []AEPEnvelope `json:"events"`
    FromSeq   int64          `json:"from_seq"`
    ToSeq     int64          `json:"to_seq"`
    Complete bool           `json:"complete"` // Last batch?
}
```

**Pros**:
- ✅ Simple implementation (no external storage)
- ✅ No schema migration needed
- ✅ Works well for short disconnections

**Cons**:
- ⚠️ Server buffers last N events (memory overhead)
- ⚠️ Large event ranges → slow reconnect (e.g., 10,000 events)

**Configuration**:
```go
const ReplayBufferSize = 1000 // Keep last 1000 events in memory
```

**Recommendation for HotPlex**:
- **Default**: Seq-based replay (simplest, works for AI agent sessions)
- **Buffer**: 1000 events (balance memory vs reconnect time)
- **Optional**: Snapshot-based for long sessions (> 10,000 events)

### 2.2 Snapshot + Incremental Log (Alternative)
**Use Case**: Long-running sessions (> 24 hours), large event history

**Architecture**:
```
┌─────────────┐      ┌──────────────┐      ┌──────────────┐
│   Client    │      │  WebSocket     │      │  SQLite WAL   │
│             │      │  Gateway        │      │  Event Store   │
└─────────────┘      └──────────────┘      └──────────────┘
         │                     │                       │
         │                     │                       │
         ▼                     ▼                       ▼
    Reconnect with session_id
         │
         ▼
    Load snapshot (every 1000 events)
    + replay from snapshot onwards
```

**SQLite WAL Mode**:
```go
// Enable WAL for concurrent reads + writes
db, err := sql.Open("file:hotplex.db?mode=WAL&cache=shared")
if err != nil {
    return err
}
defer db.Close()

// Durability settings
db.Exec("PRAGMA synchronous=NORMAL")  // Faster than FULL
db.Exec("PRAGMA busy_timeout=5000")  // 5 seconds
```

**Snapshot Creation**:
```go
// Create snapshot every N events
func createSnapshot(db *sql.DB, sessionID string, seq int64) error {
    tx, _ := db.Begin()
    defer tx.Rollback()

    // Serialize session state
    snapshot := serializeState(sessionID)

    // Store snapshot
    _, err := tx.Exec(
        "INSERT INTO snapshots (session_id, snapshot_seq, snapshot_data, created_at) VALUES (?, ?, ?, ?)",
        sessionID, seq, snapshot, time.Now().Unix(),
    )

    return err
}
```

**When to Use**:
- Long-running sessions (> 10,000 events)
- Crash recovery (persist snapshots on disk)
- Slow reconnect is acceptable (load snapshot first)

**Recommendation for HotPlex**:
- **Phase 1**: Start with seq-based replay only (simpler)
- **Phase 2**: Add snapshot support for long sessions (optional feature)
- **Default off**: Seq-based (faster for most use cases)

---

## 3. Backpressure & Flow Control
### 3.1 Go Channel Patterns
**Critical Rule**: Use **buffered channels** (size 100-1000) for burst tolerance.

**Pattern 1: Buffered Channel with Drop Strategy**
```go
type EventStream struct {
    events chan *AEPEnvelope // Buffered (1000)
    dropped int64
}

func NewEventStream() *EventStream {
    return &EventStream{
        events: make(chan *AEPEnvelope, 1000),
    }
}

// Non-blocking send
func (s *EventStream) Send(event *AEPEnvelope) bool {
    select {
    case s.events <- event:
        return true
    default:
        atomic.AddInt64(&s.dropped, 0)
        return false
    }
}
```

**When to Drop**:
- Non-critical events (heartbeats, status updates)
- Client is slow (queue depth > 800)
- Memory pressure detected

**Pattern 2: Blocking with Timeout** (For critical events)
```go
func (s *EventStream) SendCritical(event *AEPEnvelope) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    select {
    case s.events <- event:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("timeout sending critical event")
    }
}
```

**Buffer Size Guidelines**:
- **Low traffic** (< 10 events/sec): 100 buffer
- **Medium traffic** (10-100 events/sec): 500 buffer
- **High traffic** (> 100 events/sec): 1000 buffer

**Monitoring**:
```go
// Track queue depth for observability
func (s *EventStream) Metrics() map[string]interface{} {
    return map[string]interface{}{
        "queue_depth":     len(s.events),
        "queue_capacity": cap(s.events),
        "dropped_events": atomic.LoadInt64(&s.dropped),
    }
}
```

### 3.2 Backpressure Signals to Client
**Pattern**: Include `queue_depth` in heartbeat response.

```go
type HeartbeatResponse struct {
    ServerTime      int64 `json:"server_time"`
    ActiveSessions  int   `json:"active_sessions"`
    QueueDepth      int   `json:"queue_depth"`      // NEW: Client's queue depth
    EventsPerSecond int   `json:"events_per_second"`
}
```

**Client-Side Adaptation**:
```go
// Client adjusts consumption rate based on queue_depth
if resp.QueueDepth > 800 {
    // Client is overwhelmed, slow down
    client.SetRate(client.Rate() / 2)
}
```

**Rationale**:
- Transparent backpressure signaling
- Client can adapt consumption rate
- Prevents disconnects due to slow consumers

---

## 4. Heartbeat & Keepalive Design
### 4.1 Application-Level vs Transport-Level
**Recommendation**: Use **application-level heartbeats** with state hints.

**Why Application-Level?**:
- Browser WebSocket API doesn't expose ping/pong frames
- Can carry metadata (state hints)
- More control over timing and content

**Implementation**:
```go
type HeartbeatRequest struct {
    LastReceivedSeq int64 `json:"last_received_seq"` // Client's last received seq
    Timestamp       int64 `json:"timestamp"`
}

type HeartbeatResponse struct {
    ServerTime      int64 `json:"server_time"`
    ActiveSessions  int   `json:"active_sessions"`
    QueueDepth      int   `json:"queue_depth"`       // Backpressure signal
    EventsPerSecond int   `json:"events_per_second"`  // Throughput
}
```

### 4.2 Adaptive Interval (Based on RTT)
**Pattern**: Adjust interval based on network conditions.

**Implementation**:
```go
type AdaptiveHeartbeat struct {
    minInterval     time.Duration // 30s (healthy)
    maxInterval     time.Duration // 90s (degraded)
    currentInterval time.Duration
    lastPongTime    time.Time
}

func (h *AdaptiveHeartbeat) NextInterval() time.Duration {
    // If last pong was > 2x interval ago, increase interval
    if time.Since(h.lastPongTime) > h.currentInterval*2 {
        h.currentInterval = min(h.currentInterval*2, h.maxInterval)
    } else if time.Since(h.lastPongTime) < h.currentInterval/2 {
        // Network is healthy, decrease interval
        h.currentInterval = max(h.currentInterval/2, h.minInterval)
    }

    return h.currentInterval
}
```

**Configuration**:
- **Healthy network**: 30s interval
- **Degraded network**: 90s interval (3x increase)
- **Threshold**: If pong delay > 2x interval, increase

**Rationale**:
- Reduces unnecessary traffic on healthy networks
- Graceful degradation on slow networks
- Faster detection of disconnects

### 4.3 State Hints in Heartbeat
**Include**:
```go
type HeartbeatMeta struct {
    // Server state
    ActiveSessions  int   `json:"active_sessions"`
    QueueDepth      int   `json:"queue_depth"`
    EventsPerSecond int   `json:"events_per_second"`

    // Client state (sent by client)
    LastReceivedSeq int64 `json:"last_received_seq"`
    PendingAcks     int   `json:"pending_acks"`
}
```

**Use Cases**:
- **ActiveSessions**: Server load (client can back off if high)
- **QueueDepth**: Backpressure signal (client slows down)
- **EventsPerSecond**: Throughput monitoring
- **LastReceivedSeq**: Server knows client is behind (can trigger replay)

---

## 5. Session Persistence (SQLite WAL)
### 5.1 WAL Configuration
**Critical Settings**:
```go
// Open database with WAL mode
db, err := sql.Open("file:hotplex.db?mode=WAL&cache=shared&_journal_mode=WAL")
if err != nil {
    return err
}

// Durability settings
PRAGMA synchronous=NORMAL;   // Faster than FULL (acceptable for non-critical data)
PRAGMA busy_timeout=5000;    // 5 seconds
PRAGMA wal_autocheckpoint=1000; // Auto-checkpoint every 1000 frames
```

**Rationale**:
- **WAL mode**: Concurrent reads + writes (better performance)
- **synchronous=NORMAL**: Faster writes (acceptable for event logs)
- **wal_autocheckpoint=1000**: Recovery points for crash recovery

**Trade-off**:
- **synchronous=NORMAL**: Small window of data loss on crash (< 1 second)
- **synchronous=FULL**: Zero data loss, but slower writes

choose **NORMAL** for HotPlex (event logs are not critical)

### 5.2 Event Store Schema
**Recommendation**: Append-only log with optional snapshots.

**Schema**:
```sql
CREATE TABLE events (
    seq INTEGER PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    data TEXT NOT NULL,  -- JSON
    committed INTEGER DEFAULT 0,  -- WAL commit flag
    created_at INTEGER DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX idx_session_time ON events(session_id, timestamp);
CREATE INDEX idx_session_seq ON events(session_id, seq);

-- Snapshots (optional, for long sessions)
CREATE TABLE snapshots (
    session_id TEXT PRIMARY KEY,
    snapshot_seq INTEGER NOT NULL,
    snapshot_data TEXT NOT NULL,  -- Compressed JSON
    created_at INTEGER NOT NULL
);
```

**Write Pattern**:
```go
func AppendEvent(db *sql.DB, event *AEPEnvelope) error {
    tx, _ := db.Begin()
    defer tx.Rollback()

    // Insert event
    _, err := tx.Exec(
        "INSERT INTO events (seq, session_id, type, timestamp, data) VALUES (?, ?, ?, ?, ?)",
        event.Seq, event.SessionID, event.Type, event.Timestamp, event.Data,
    )

    // Auto-checkpoint every 1000 events
    if event.Seq % 1000 == 0 {
        createSnapshot(tx, event.SessionID, event.Seq)
    }

    return err
}
```

### 5.3 Crash Recovery Flow
```
Startup → Replay WAL → Rebuild state → Resume
```

**Implementation**:
```go
func RecoverFromCrash(dbPath string) (*sql.DB, error) {
    // 1. Open database
    db, err := sql.Open(fmt.Sprintf("file:%s?mode=WAL", dbPath))
    if err != nil {
        return nil, err
    }

    // 2. Check for uncommitted transactions
    var uncommitted []int64
    rows, _ := db.Query("SELECT seq FROM events WHERE committed = 0")
    defer rows.Close()
    for rows.Next() {
        var seq int64
        rows.Scan(&seq)
        uncommitted = append(uncommitted, seq)
    }

    // 3. Decision: rollback or replay
    if len(uncommitted) > 0 {
        log.Warn("Uncommitted events", "count", len(uncommitted))
        // For HotPlex: rollback (events are idempotent)
        db.Exec("DELETE FROM events WHERE seq IN (?)", uncommitted)
    }

    // 4. Rebuild in-memory state (load last snapshot)
    // (Implementation depends on HotPlex session structure)

    return db, nil
}
```

**Recommendation for HotPlex**:
- **Phase 1**: Event log only (no snapshots)
- **Phase 2**: Add snapshots for long sessions (optional)
- **WAL mode**: Required for crash recovery
- **Auto-checkpoint**: Every 1000 events

---

## 6. Security Patterns
### 6.1 Authentication: Handshake-Only
**Recommendation**: Authenticate **once** during WebSocket upgrade.

**Pattern**:
```go
func (h *Handler) Upgrade(w http.ResponseWriter, r *http.Request) {
    // 1. Extract token
    token := r.URL.Query().Get("token")
    if token == "" {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    // 2. Validate token (JWT or API key)
    userID, err := validateToken(token)
    if err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    // 3. Upgrade connection
    conn, err := h.upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    // 4. Store userID in connection context
    ctx := context.WithValue(r.Context(), "user_id", userID)

    // 5. Start event loop with authenticated context
    h.handleConnection(ctx, conn, userID)
}
```

**Why Handshake-Only?**:
- ✅ One-time validation (low overhead)
- ✅ Token reuse for entire session
- ✅ No per-message cost

**Avoid**:
- ❌ Per-message JWT validation (high overhead)
- ❌ Session-based auth (requires state store)

### 6.2 Token Refresh (Long Sessions)
**Pattern**: Send refresh token before expiry.

**Implementation**:
```go
type TokenRefreshMessage struct {
    Type         string `json:"type"`          // "token_refresh"
    RefreshToken string `json:"refresh_token"`
}

// Client sends refresh before token expires
func handleTokenRefresh(conn *websocket.Conn, msg TokenRefreshMessage) error {
    newToken, err := refreshJWT(msg.RefreshToken)
    if err != nil {
        return err
    }

    // Send new token
    return conn.WriteJSON(AEPEnvelope{
        Type: "token_refresh",
        Data: map[string]string{
            "token": newToken,
        },
    })
}
```

**When to Use**:
- Long sessions (> 1 hour)
- Token expiry < 24 hours
- Stateless server (no session store)

**Recommendation for HotPlex**:
- **Default**: Handshake auth only (simplest)
- **Optional**: Token refresh for long sessions
- **Avoid**: Per-message auth (high overhead)

### 6.3 Rate Limiting
**Pattern**: **Token bucket per session** (not per connection).

**Implementation**:
```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters sync.Map // session_id -> *rate.Limiter
}

func NewRateLimiter() *RateLimiter {
    return &RateLimiter{
        limiters: sync.Map{},
    }
}

func (rl *RateLimiter) Allow(sessionID string) bool {
    limiter, _ := rl.limiters.Load(sessionID)
    if limiter == nil {
        // 100 events per second per session
        limiter = rate.NewLimiter(100, 100)
        rl.limiters.Store(sessionID, limiter)
    }

    return limiter.Allow()
}
```

**Configuration**:
- **Rate**: 100 events/second per session (adjust based on use case)
- **Burst**: 100 tokens (allow short bursts)

**When to Limit**:
- High-frequency event sources (e.g., file watchers)
- Untrusted clients (prevent abuse)
- Shared infrastructure (multi-tenant)

**Recommendation for HotPlex**:
- **Default**: 100 events/sec per session
- **Optional**: Stricter limits for untrusted sources
- **Monitor**: Track rate limit rejections

---

## 7. Real-World Protocol Examples
### 7.1 Discord Gateway (Resume Pattern)
**Key Features**:
- **Opcode 6 Resume**: Sends `session_id`, `seq`, `token`
- **Resume URL**: Dedicated `resume_gateway_url` for reconnection
- **Sequence tracking**: Client tracks last `seq` number

**Protocol**:
```json
{
  "op": 6,
  "d": {
    "token": "bot_token",
    "session_id": "session_id_from_ready",
    "seq": 1337
  }
}
```

**Recovery Flow**:
1. Client disconnects
2. Client reconnects to `resume_gateway_url`
3. Client sends Opcode 6 Resume
4. Server validates session, replays events from `seq + 1`
5. Server sends Resumed event

**Applicable to HotPlex**:
- **Session resume**: Reconnect with session_id + last_seq
- **Dedicated endpoint**: Optional `/resume` endpoint (separate from `/ws`)
- **Validation**: Token + session_id + seq must to match

### 7.2 Slack Events API (Event IDs)
**Key Features**:
- **Event ID**: Unique ID for each event
- **Retry with ID**: Client retries with `event_id` header
- **Idempotency**: Server deduplicates based on event_id

**Protocol**:
```
POST /events
Authorization: Bearer xoxb-token
X-Event-ID: abc123
```

**Applicable to HotPlex**:
- **Event IDs**: Include `id` field in envelope (for deduplication)
- **Idempotent handlers**: Handle duplicate events gracefully
- **Retry header**: Client includes `Last-Event-ID` on reconnect

### 7.3 OpenAI Realtime API (Bidirectional Streaming)
**Key Features**:
- **Event types**: `session.update`, `input_audio_buffer.append`, `response.create`
- **Bidirectional**: Client sends audio, server sends responses
- **Function calling**: Supports tool use during conversation

**Protocol**:
```json
{
  "type": "session.update",
  "session": {
    "id": "sess_123",
    "model": "gpt-4o-realtime-preview"
  }
}
```

**Applicable to HotPlex**:
- **Bidirectional streaming**: Client sends inputs, server sends events
- **Event-driven**: Use `type` field for routing
- **Function calling**: Similar to tool_use events

---

## 8. Implementation Checklist
### 8.1 Phase 1: Core Protocol (MVP)
- [ ] **Envelope format**: Implement versioned envelope with `seq`, `type`, `id`
- [ ] **Handshake**: Version negotiation + authentication
- [ ] **Seq-based replay**: Buffer last 1000 events, replay on reconnect
- [ ] **Heartbeat**: Application-level with state hints (30s interval)
- [ ] **Backpressure**: Buffered channels (1000) with drop strategy
- [ ] **Authentication**: Handshake-only (JWT or API key)
- [ ] **Rate limiting**: Token bucket (100 events/sec)

### 8.2 Phase 2: Production Hardening
- [ ] **Schema evolution**: Additive-only changes, version negotiation
- [ ] **Adaptive heartbeats**: Adjust interval based on RTT (30-90s)
- [ ] **Snapshot support**: Optional for long sessions (> 10,000 events)
- [ ] **Token refresh**: Optional for long sessions (> 1 hour)
- [ ] **Crash recovery**: SQLite WAL mode, auto-checkpoint
- [ ] **Monitoring**: Queue depth, event rate, reconnect count

### 8.3 Phase 3: Advanced Features
- [ ] **Dedicated resume endpoint**: Separate `/resume` from `/ws`
- [ ] **Event deduplication**: Use `id` field for idempotency
- [ ] **Flow control**: Client-side rate adaptation based on `queue_depth`
- [ ] **Compression**: Optional gzip for large payloads
- [ ] **Observability**: Prometheus metrics, structured logs

---

## 9. Anti-Patterns (Avoid)
### 9.1 Unbounded Channels
```go
// ❌ BAD: Unbounded channel
eventChan := make(chan Event)
```
**Problem**: Slow consumer → memory leak (unbounded growth)

**Fix**: Use buffered channels (1000) with drop strategy

### 9.2 Per-Message Authentication
```go
// ❌ BAD: Validate on every message
func handleEvent(msg Event) error {
    if !validateToken(msg.Token) {
        return ErrUnauthorized
    }
    // ...
}
```
**Problem**: High overhead (validation on every message)

**Fix**: Authenticate on handshake, store user context in connection

### 9.3 Breaking Schema Changes
```go
// ❌ BAD: Rename required field
type EventV2 struct {
    EventType string `json:"event_type"` // Was "type"
}
```
**Problem**: Breaks existing clients

**Fix**: Additive changes only (new optional fields)

### 9.4 Ignoring Backpressure
```go
// ❌ BAD: Block indefinitely
eventChan <- event
```
**Problem**: Slow consumer → blocked producer → deadlock

**Fix**: Non-blocking send with drop strategy

### 9.5 Fixed Heartbeat Interval
```go
// ❌ BAD: Fixed 30s interval
ticker := time.NewTicker(30 * time.Second)
```
**Problem**: Unnecessary traffic on healthy network, slow detection on degraded network

**Fix**: Adaptive interval (30-90s based on RTT)

---

## 10. HotPlex-Specific Recommendations
### 10.1 Session Types
**Recommendation**: Use **different replay strategies** based on session type.

**Session Categories**:
- **Short sessions** (< 1000 events): Seq-based replay (in-memory buffer)
- **Long sessions** (> 1000 events): Snapshot + incremental replay (SQLite)
- **Critical sessions** (production): Snapshot + synchronous WAL (no data loss)

**Configuration**:
```go
type SessionConfig struct {
    ReplayBuffer    int   // In-memory buffer size (default: 1000)
    SnapshotInterval int   // Create snapshot every N events (default: 1000)
    WALMode         string // "NORMAL" or "FULL" (default: "NORMAL")
}
```

### 10.2 Event Priorities
**Recommendation**: Prioritize events for backpressure handling.

**Priority Levels**:
```go
const (
    PriorityCritical   = 0 // Never drop (errors, tool_results)
    PriorityHigh       = 1 // Rarely drop (messages, tool_use)
    PriorityNormal     = 2 // Can drop (heartbeats, status)
    PriorityLow        = 3 // Drop first (debug, metrics)
)
```

**Drop Strategy**:
```go
func (s *EventStream) Send(event *AEPEnvelope, priority int) bool {
    if priority == PriorityCritical {
        // Block until sent (timeout: 5s)
        return s.sendBlocking(event)
    }

    // Non-blocking for non-critical
    select {
    case s.events <- event:
        return true
    default:
        if priority >= PriorityNormal {
            atomic.AddInt64(&s.dropped, 1)
        }
        return false
    }
}
```

### 10.3 Client Hints
**Recommendation**: Include **client-side hints** in heartbeat.

**Hints from Client**:
```go
type ClientHeartbeat struct {
    LastReceivedSeq int64 `json:"last_received_seq"` // Client's last seq
    QueueDepth      int   `json:"queue_depth"`       // Client's buffer depth
    ProcessingTime  int64 `json:"processing_time"`  // Ms to process last event
}
```

**Server Adaptation**:
```go
// If client is behind, trigger replay
if client.LastReceivedSeq < server.LastSentSeq-100 {
    server.SendReplay(client.LastReceivedSeq+1, server.LastSentSeq)
}

// If client queue is full, slow down
if client.QueueDepth > 800 {
    server.ReduceEventRate(client.SessionID)
}
```

---

## 11. Migration Path (HotPlex v0.x → v1.0)
### 11.1 Phase 1: Foundation (Week 1-2)
**Goal**: Stable protocol with basic replay

**Tasks**:
- [ ] Implement versioned envelope format
- [ ] Add handshake with version negotiation
- [ ] Implement seq-based replay (1000 buffer)
- [ ] Add application-level heartbeats (30s fixed)
- [ ] Set up buffered channels (1000) with drop strategy
- [ ] Implement handshake authentication
- [ ] Add rate limiting (100 events/sec)

**Validation**:
- Reconnect works (events replayed correctly)
- No memory leaks (monitor queue depth)
- Authentication works (reject invalid tokens)
- Rate limiting works (reject > 100 events/sec)

### 11.2 Phase 2: Hardening (Week 3-4)
**Goal**: Production-ready protocol

**Tasks**:
- [ ] Add schema evolution support (additive-only)
- [ ] Implement adaptive heartbeats (30-90s)
- [ ] Enable SQLite WAL mode
- [ ] Add crash recovery logic
- [ ] Implement monitoring (queue depth, event rate)
- [ ] Add token refresh support (optional)

**Validation**:
- Schema changes don't break clients
- Crash recovery works (rebuild state after restart)
- Adaptive heartbeats reduce traffic
- Monitoring dashboards work

### 11.3 Phase 3: Optimization (Week 5-6)
**Goal**: Performance and scalability

**Tasks**:
- [ ] Add snapshot support (optional, for long sessions)
- [ ] Implement event deduplication (use `id` field)
- [ ] Add client-side hints (queue_depth, processing_time)
- [ ] Implement flow control (server adjusts rate based on client hints)
- [ ] Add compression support (optional gzip)
- [ ] Set up Prometheus metrics

**Validation**:
- Snapshots speed up long reconnects
- Deduplication prevents duplicate processing
- Flow control prevents slow consumers
- Compression reduces bandwidth

---

## 12. Monitoring & Observability
### 12.1 Key Metrics
**Recommendation**: Track these metrics for health monitoring.

**Server Metrics**:
```go
var ServerMetrics = struct {
    ActiveConnections    int64   // Currently connected clients
    TotalEventsSent      int64   // Total events sent since startup
    EventsPerSecond      float64 // Current event rate
    AvgQueueDepth        float64 // Average queue depth across all clients
    ReconnectCount       int64   // Total reconnect attempts
    ReplayCount          int64   // Total replay operations
    DropRate            float64 // Event drop rate (%)
    AvgHeartbeatLatency  float64 // Average heartbeat RTT (ms)
}
```

**Per-Session Metrics**:
```go
var SessionMetrics = struct {
    SessionID           string  // Session identifier
    EventsSent          int64   // Events sent to this session
    QueueDepth          int     // Current queue depth
    LastHeartbeat       int64   // Last heartbeat timestamp
    ReconnectCount      int     // Reconnect attempts
    EventsDropped       int64   // Events dropped (backpressure)
    AvgProcessingTime   float64 // Avg event processing time (ms)
}
```

### 12.2 Alerts
**Recommendation**: Set up alerts for critical conditions.

**Alert Rules**:
```go
// Alert if queue depth > 900 (90% capacity)
if metrics.QueueDepth > 900 {
    triggerAlert("Backpressure: Client queue nearly full")
}

// Alert if drop rate > 5%
if metrics.DropRate > 0.05 {
    triggerAlert("High event drop rate")
}

// Alert if no heartbeat > 90s (3x max interval)
if time.Since(session.LastHeartbeat) > 90*time.Second {
    triggerAlert("Client unresponsive")
}

// Alert if reconnect count > 10 in 5 minutes
if session.ReconnectCount > 10 {
    triggerAlert("Reconnect loop detected")
}
```

### 12.3 Logging
**Recommendation**: Structured logging with context.

**Pattern**:
```go
log.Info("Event sent",
    "session_id", sessionID,
    "seq", event.Seq,
    "type", event.Type,
    "queue_depth", len(eventChan),
    "elapsed_ms", time.Since(start).Milliseconds(),
)
```

**Key Events to Log**:
- Connection established/upgraded
- Authentication success/failure
- Session started/resumed
- Reconnect attempt (with last_seq)
- Replay triggered (from_seq, to_seq)
- Event dropped (queue full)
- Heartbeat sent/received
- Connection closed (with reason)

---

## 13. Testing Strategy
### 13.1 Unit Tests
**Recommendation**: Test each component in isolation.

**Test Cases**:
```go
func TestEnvelopeVersioning(t *testing.T) {
    // Test V1 client connects to V1 server
    // Test V1 client connects to V2 server
    // Test V2 client connects to V1 server (should negotiate V1)
}

func TestSeqBasedReplay(t *testing.T) {
    // Test reconnect with valid seq
    // Test reconnect with invalid seq (too old)
    // Test reconnect with future seq (error)
}

func TestBackpressure(t *testing.T) {
    // Test buffered channel fills up
    // Test drop strategy works
    // Test critical events block (don't drop)
}

func TestHeartbeat(t *testing.T) {
    // Test heartbeat sent on interval
    // Test adaptive interval adjusts
    // Test no heartbeat triggers disconnect
}
```

### 13.2 Integration Tests
**Recommendation**: Test full flows with real WebSocket connections.

**Test Cases**:
```go
func TestReconnectionFlow(t *testing.T) {
    // 1. Connect client
    // 2. Send 100 events
    // 3. Disconnect client
    // 4. Reconnect client with last_seq=50
    // 5. Verify events 51-100 are replayed
}

func TestSchemaEvolution(t *testing.T) {
    // 1. Start V1 server
    // 2. Connect V1 client
    // 3. Upgrade server to V2
    // 4. Verify V1 client still works
    // 5. Connect V2 client
    // 6. Verify V2 client gets V2 features
}

func TestCrashRecovery(t *testing.T) {
    // 1. Start server
    // 2. Send 1000 events
    // 3. Kill server (simulated crash)
    // 4. Restart server
    // 5. Reconnect client
    // 6. Verify state recovered (from WAL or snapshot)
}
```

### 13.3 Load Tests
**Recommendation**: Test with realistic production loads.

**Test Cases**:
```go
func TestHighConcurrency(t *testing.T) {
    // 1. Connect 1000 clients
    // 2. Send 100 events/sec to each client (100k events/sec total)
    // 3. Verify no memory leaks (monitor heap size)
    // 4. Verify drop rate < 5%
    // 5. Verify avg queue depth < 800
}

func TestSlowConsumers(t *testing.T) {
    // 1. Connect 100 clients
    // 2. Make 10 clients slow (process events slowly)
    // 3. Send high event rate
    // 4. Verify slow clients drop events (expected)
    // 5. Verify fast clients receive all events
}
```

---

## 14. Documentation Template
### 14.1 Protocol Specification
**Structure**:
```markdown
# AEP v1.0 Protocol Specification

## 1. Overview
- Purpose
- Design goals
- Compatibility

## 2. Message Format
### 2.1 Envelope
### 2.2 Event Types
### 2.3 Schema Evolution

## 3. Connection Lifecycle
### 3.1 Handshake
### 3.2 Authentication
### 3.3 Event Streaming
### 3.4 Reconnection
### 3.5 Termination

```

### 14.2 API Reference
**Structure**:
```markdown
# AEP API Reference

## Events

### message
- Type: `message`
- Data: `{ "content": "..." }`
- Example

### tool_use
- Type: `tool_use`
- Data: `{ "name": "bash", "input": {...} }`
- Example

## Control Messages

### handshake
- Request
- Response

### heartbeat
- Request
- Response
```

### 14.3 Client Implementation Guide
**Structure**:
```markdown
# Implementing AEP v1.0 Clients

## 1. Connection
- WebSocket endpoint
- Authentication
- Version negotiation

## 2. Event Handling
- Receiving events
- Sending events
- Backpressure handling

## 3. Reconnection
- Detecting disconnects
- Reconnect flow
- State recovery

## 4. Best Practices
- Buffer management
- Error handling
- Monitoring
```

---

## 15. References & Sources

**Primary Sources**:
1. [Ably: WebSocket Architecture Best Practices](https://ably.com/topic/websocket-architecture-best-practices) - Scalability, state management, backpressure
2. [ShadeCoder: WebSocket Protocol Guide 2025](https://www.shadecoder.com/topics/the-websocket-protocol-a-comprehensive-guide-for-2025) - Full-duplex communication, event-driven design
3. [OneUptime: SSE vs WebSockets (2026)](https://oneuptime.com/blog/post/2026-01-27-sse-vs-websockets/view) - Event streaming, reconnection patterns, best practices
4. [Discord Developer Docs: Gateway](https://docs.discord.com/developers/events/gateway) - Resume protocol (Opcode 6), sequence tracking
5. [Discord Userdoccers: Using Gateway](https://docs.discord.food/gateway/using-gateway) - Resume URL, reconnection flow
6. [Discord API Types: Gateway Opcodes](https://discord-api-types.dev/api/discord-api-types-v10/enum/GatewayOpcodes) - Opcode 6 Resume definition
7. [Slack Developer Docs: Events API](https://docs.slack.dev/apis/events-api/) - Event delivery, retry patterns
8. [ByteByteGo: How Slack Supports Billions](https://blog.bytebytego.com/p/how-slack-supports-billions-of-daily) - Message ordering, channel servers
9. [Medium: Event Sourcing Without Regret](https://medium.com/@Modexa/event-sourcing-without-regret-a-java-playbook-acc11f2b398c) - Schema evolution, production patterns
10. [youngju.dev: Event Sourcing Production Anti-Patterns](https://www.youngju.dev/blog/architecture/2026-03-07-architecture-event-sourcing-cqrs-production-patterns.en) - Schema evolution strategies, snapshotting, projection rebuilding

**Go-Specific Sources**:
11. [Reddit: Gorilla WebSocket Experience](https://www.reddit.com/r/golang/comments/1b28pd5/whats_being_your_experience_using_gorilla/) - Community feedback, debugging challenges
12. [websockets.readthedocs.io: Keepalive and Latency](https://websockets.readthedocs.io/en/stable/topics/keepalive.html) - Ping/pong frames, latency detection
13. [Gojek Blog: Adaptive Heartbeats](https://www.gojek.io/blog/adaptive-heartbeats-for-our-information-superhighway) - Adaptive keepalive intervals, network optimization
14. [Medium: Distributed System Event Delivery Pattern](https://medium.com/@bindubc/distributed-system-event-delivery-pattern-843a45048ac7) - Idempotency, exactly-once delivery
15. [Ably Blog: Exactly-Once Message Processing](https://ably.com/blog/achieving-exactly-once-message-processing-with-ably) - Delivery guarantees, idempotency
16. [CockroachDB: Idempotency and Ordering](https://www.cockroachlabs.com/blog/idempotency-and-ordering-in-event-driven-systems/) - Event ordering challenges, idempotency properties

**General Knowledge** (No specific sources):
- gRPC streaming: HTTP/2 flow control, window updates, credit-based flow control
- OpenAI Realtime API: Bidirectional streaming, event-driven protocol, function calling

---

**End of Document**
