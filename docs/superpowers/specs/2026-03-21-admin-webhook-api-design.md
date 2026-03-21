# HotPlex Admin Webhook API Design

**Date**: 2026-03-21
**Status**: Draft
**Type**: Feature Design
**Related Issues**: TBD

---

## 1. Motivation & Background

HotPlex is positioned as **Cli-as-a-Service** — a high-performance AI Agent control plane. While it exposes WebSocket (`/ws/v1/agent`) for interactive sessions, it currently lacks a programmatic management interface. External DevOps tooling (CI/CD pipelines, monitoring systems, internal operation dashboards) cannot programmatically:

- Query active sessions and system health
- Stop runaway or stuck sessions
- Observe and audit agent execution history
- Dynamically adjust runtime configuration

This design introduces an **Admin Webhook API** — a RESTful management interface gated by a dedicated admin API key, complementing the existing WebSocket gateway without overlapping in responsibility.

---

## 2. Design Principles

1. **Admin-only scope**: Separate from the existing `X-API-Key` used by client applications. A dedicated `HOTPLEX_ADMIN_API_KEY` gates all management endpoints.
2. **RESTful & minimal**: Standard HTTP methods, JSON payloads, predictable URLs.
3. **Reuse, don't rebuild**: Bridge to existing internal components (SessionPool, Engine, Config, DedupCache) rather than duplicating state.
4. **Zero breaking changes**: The new endpoints live under `/api/v1/admin/` and do not affect existing WebSocket or ChatApps routes.
5. **Audit-ready**: All write operations produce structured log entries with actor identity.

---

## 3. Scope

### 3.1 Endpoint Summary

All endpoints live under `/api/v1/admin/` and require `X-Admin-Key` header.

#### Session Management
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/admin/sessions` | List active sessions |
| `GET` | `/api/v1/admin/sessions/{id}` | Get session details |
| `GET` | `/api/v1/admin/sessions/{id}/stats` | Get session statistics |
| `POST` | `/api/v1/admin/sessions/{id}/stop` | Stop a running session |
| `POST` | `/api/v1/admin/sessions/batch/stop` | Stop multiple sessions |

#### System Health & Metrics
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/admin/health` | Full health check (all subsystems) |
| `GET` | `/api/v1/admin/metrics` | Prometheus-format metrics |
| `POST` | `/api/v1/admin/drain` | Enter drain mode |
| `DELETE` | `/api/v1/admin/drain` | Exit drain mode |
| `GET` | `/api/v1/admin/drain` | Get drain status |

#### Configuration (Read-Only)
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/admin/config` | Get current effective config |
| `GET` | `/api/v1/admin/config/allowed_tools` | Get allowed tools list |
| `GET` | `/api/v1/admin/config/disallowed_tools` | Get disallowed tools list |

#### Audit & History
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/admin/events` | Query execution events (with pagination) |
| `GET` | `/api/v1/admin/sessions/{id}/transcript` | Get full session transcript |

---

## 4. Data Models

### 4.1 Admin Session Object

**Status field mapping** (from `internal/engine/session.go` `SessionStatus` enum):

| `SessionStatus` | Admin API string |
|---|---|
| `SessionStatusStarting` | `"starting"` |
| `SessionStatusReady` | `"idle"` |
| `SessionStatusBusy` | `"running"` |
| `SessionStatusDead` | `"dead"` |

**Platform field**: `Session` in `internal/engine/` is platform-agnostic. Platform is populated from `hotplex.Config.SessionID` prefix or resolved via the platform registry. If unresolved, defaults to `"unknown"`.

**Tags field**: Derived from `Engine.GetOptions().Namespace` and other metadata. If no tags available, omit the field entirely (use `omitempty`).

```go
type AdminSession struct {
    SessionID     string            `json:"session_id"`
    Status        string            `json:"status"`            // "starting", "idle", "running", "dead"
    Platform      string            `json:"platform"`          // "websocket", "slack", "feishu", "admin", "unknown"
    CreatedAt     time.Time         `json:"created_at"`
    LastActiveAt  time.Time         `json:"last_active_at"`
    PromptCount   int64             `json:"prompt_count"`
    TotalTokens   int64             `json:"total_tokens"`
    AvgLatencyMs  int64             `json:"avg_latency_ms"`
    Tags          map[string]string `json:"tags,omitempty"`    // namespace, adapter, etc.
}
```

### 4.2 Session Statistics Object

Aligned with `engine/stats.go` (`engine.SessionStats`):

```go
type AdminSessionStats struct {
    SessionID             string     `json:"session_id"`
    TotalDurationMs       int64      `json:"total_duration_ms"`
    ThinkingDurationMs    int64      `json:"thinking_duration_ms"`
    ToolDurationMs        int64      `json:"tool_duration_ms"`
    GenerationDurationMs  int64      `json:"generation_duration_ms"`
    InputTokens           int64      `json:"input_tokens"`
    OutputTokens          int64      `json:"output_tokens"`
    CacheReadTokens       int64      `json:"cache_read_tokens"`
    CacheWriteTokens      int64      `json:"cache_write_tokens"`
    ToolCallCount         int64      `json:"tool_call_count"`
    ToolsUsed             []string   `json:"tools_used,omitempty"`
    FilesModified         int64      `json:"files_modified"`
    FilePaths             []string   `json:"file_paths,omitempty"`
    ErrorCount            int64      `json:"error_count"`
}
```

Note: `TurnCount`, `AvgLatencyMs`, `FirstTokenAt`, `LastTokenAt` are **not** tracked in `engine/stats.go` and are omitted from this model.

### 4.3 Admin Event Object

```go
type AdminEvent struct {
    EventID     string                 `json:"event_id"`
    Timestamp   time.Time              `json:"timestamp"`
    Type        string                 `json:"type"`          // "execution.start", "execution.end", "session.stop", "danger_block", "config.change"
    SessionID   string                 `json:"session_id,omitempty"`
    Actor       string                 `json:"actor,omitempty"` // "websocket", "slack", "admin:{key_prefix}"
    Payload     map[string]interface{}  `json:"payload,omitempty"`
}
```

### 4.4 Message Store Capability

The `/events` and `/sessions/{id}/transcript` endpoints require the message store plugin to be enabled. If the store is unavailable:

- Return `503 Service Unavailable` with `{"error": {"code": "STORE_UNAVAILABLE", "message": "Message store plugin is not enabled"}}`
- The store must implement the `MessageStore` interface from `plugins/storage/`.

### 4.5 Health Response

```go
type AdminHealth struct {
    Status     string                   `json:"status"`        // "ok", "degraded", "unhealthy"
    Version    string                   `json:"version"`        // hotplex.Version
    Uptime     string                   `json:"uptime"`
    Subsystems map[string]SubsystemHealth `json:"subsystems"`
}

type SubsystemHealth struct {
    Status  string `json:"status"`  // "ok", "error"
    Message string `json:"message,omitempty"`
    Latency string `json:"latency,omitempty"`
}
```

---

## 5. Endpoint Specifications

### 5.1 Authentication

All `/api/v1/admin/*` endpoints require:

```
X-Admin-Key: <HOTPLEX_ADMIN_API_KEY>
```

Environment variable: `HOTPLEX_ADMIN_API_KEY`.

- **If not set**: routes are not registered at all — requests hit `http.DefaultServeMux` with no match, resulting in `404 Not Found`.
- **If key mismatch**: returns `401 Unauthorized` with constant-time comparison (`subtle.ConstantTimeCompare`) to prevent timing attacks.

### 5.2 Session Management

#### GET /api/v1/admin/sessions

List active sessions.

**Query Parameters:**
- `status` (optional): Filter by status (`running`, `idle`, `stopping`)
- `platform` (optional): Filter by platform
- `limit` (optional, default 50, max 200)
- `offset` (optional, default 0)

**Response:**
```json
{
  "sessions": [AdminSession],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

#### GET /api/v1/admin/sessions/{id}

Get details for a specific session.

**Response:** `AdminSession` object or `404 Not Found`.

#### GET /api/v1/admin/sessions/{id}/stats

Get execution statistics for a session.

**Response:** `SessionStats` object or `404 Not Found`.

#### POST /api/v1/admin/sessions/{id}/stop

Stop a running session.

**Request:**
```json
{
  "reason": "Admin requested via webhook"
}
```

**Response:**
```json
{
  "session_id": "abc-123",
  "status": "stopping",
  "message": "Stop signal sent"
}
```

#### POST /api/v1/admin/sessions/batch/stop

Stop multiple sessions in one request. Max batch size: **100**. Requests exceeding this limit return `400 INVALID_REQUEST`.

**Request:**
```json
{
  "session_ids": ["abc-123", "def-456"],
  "reason": "Scheduled maintenance"
}
```

**Response:**
```json
{
  "stopped": ["abc-123"],
  "not_found": [],
  "failed": [
    {
      "session_id": "def-456",
      "error": "Session is already being terminated"
    }
  ]
}
```

### 5.3 System Health & Metrics

#### GET /api/v1/admin/health

Full health check across all subsystems.

**Response:** `AdminHealth` object.

Subsystems checked:
- `engine`: SessionPool health
- `security`: WAF status
- `storage`: Message store connectivity
- `config`: Config watcher status

#### GET /api/v1/admin/metrics

Prometheus-compatible metrics text format.

Metrics exposed:
- `hotplex_sessions_active{status, platform}` — Gauge
- `hotplex_sessions_total{platform}` — Counter
- `hotplex_executions_total{status}` — Counter
- `hotplex_execution_duration_seconds` — Histogram
- `hotplex_tokens_total{type}` — Counter
- `hotplex_danger_blocks_total` — Counter
- `hotplex_errors_total{type}` — Counter

#### POST /api/v1/admin/drain

Enter drain mode: stop accepting new WebSocket connections and new webhook executions. Existing sessions continue.

**Request:**
```json
{
  "message": "Scheduled maintenance at 02:00 UTC"
}
```

**Response:**
```json
{
  "status": "draining",
  "active_sessions": 3,
  "message": "Scheduled maintenance at 02:00 UTC"
}
```

#### DELETE /api/v1/admin/drain

Exit drain mode, resume accepting new requests.

#### GET /api/v1/admin/drain

Get current drain status.

### 5.4 Configuration (Read-Only)

#### GET /api/v1/admin/config

Returns sanitized effective config (secrets redacted).

Sensitive fields (`api_keys`, `tokens`, `secrets`) are replaced with `"[REDACTED]"`.

#### GET /api/v1/admin/config/allowed_tools

Returns the effective allowed tools list.

```json
{
  "allowed_tools": ["Read", "Write", "Bash"],
  "source": "config"
}
```

#### GET /api/v1/admin/config/disallowed_tools

Returns the effective disallowed tools list.

```json
{
  "disallowed_tools": ["Network"],
  "source": "config"
}
```

### 5.5 Audit & History

#### GET /api/v1/admin/events

Query recent execution events.

**Query Parameters:**
- `session_id` (optional)
- `type` (optional): Event type filter
- `actor` (optional): Filter by actor (e.g., `admin:xxx`)
- `since` (optional): ISO8601 timestamp
- `limit` (optional, default 100, max 500)
- `cursor` (optional): Pagination cursor

**Response:**
```json
{
  "events": [AdminEvent],
  "next_cursor": "abc123",
  "total": 1024
}
```

#### GET /api/v1/admin/sessions/{id}/transcript

Get the full message transcript for a session.

**Response:**
```json
{
  "session_id": "abc-123",
  "messages": [
    {
      "type": "user_input",
      "timestamp": "2026-03-21T10:00:00Z",
      "content": "Analyze this PR"
    },
    {
      "type": "final_response",
      "timestamp": "2026-03-21T10:00:05Z",
      "content": "The PR looks good overall..."
    }
  ]
}
```

---

## 6. Architecture

### 6.1 Component Location

```
internal/
  server/
    admin/
      admin.go            # Main router & handler registration
      admin_handler.go    # HTTP handler struct & ServeHTTP
      session_handler.go  # Session management endpoints
      health_handler.go   # Health & metrics endpoints
      config_handler.go   # Config read-only endpoints
      audit_handler.go    # Event & transcript endpoints
      auth.go             # Admin key middleware
      types.go            # Admin API data models
  engine/
    runner.go             # Engine.StopSession(), Engine.GetSessionStats()
  internal/
    config/
      server_config.go    # *ServerLoader (read-only config access)
```

Key interfaces used:
- `engine.manager.ListActiveSessions()` — not `engine.Sessions()`
- `engine.manager.GetSession(id)` — get single session
- `engine.StopSession(id, reason)` — terminate session
- `engine.GetSessionStats(id)` — return `*engine.SessionStats`
- `engine.GetOptions().AllowedTools` / `.DisallowedTools`
- `*ServerLoader.Get()` — read-only config access with `sync.RWMutex`
- `hotplex.Version` — version string (`hotplex.go:13`)

### 6.2 Auth Middleware

```go
func AdminAuthMiddleware(adminKey string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := r.Header.Get("X-Admin-Key")
        if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) != 1 {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Note: `adminKey` is guaranteed non-empty because the routes are only registered when `HOTPLEX_ADMIN_API_KEY` is set. If the env var is absent, the mux has no `/api/v1/admin/` routes at all.

### 6.3 Route Registration

In `cmd/hotplexd/main.go`:

```go
// Admin API (requires HOTPLEX_ADMIN_API_KEY)
adminKey := os.Getenv("HOTPLEX_ADMIN_API_KEY")
if adminKey != "" {
    adminRouter := mux.NewRouter()
    adminHandler := server.NewAdminHandler(engine, serverLoader, logger)
    adminRouter.Use(server.AdminAuthMiddleware(adminKey))
    adminHandler.RegisterRoutes(adminRouter)
    http.Handle("/api/v1/admin/", adminRouter)
}
```

Note: `server.NewAdminHandler` accepts `*ServerLoader` (from `internal/config/server_config.go`), not `configWatcher`.

### 6.4 Data Flow

```
External Tool (curl/SDK)
    │
    ▼
HTTP Request + X-Admin-Key
    │
    ▼
AdminAuthMiddleware (constant-time compare)
    │
    ▼
AdminHandler (route to sub-handlers)
    │
    ├─► SessionHandler ──► SessionPool (internal/engine/pool.go via manager)
    ├─► HealthHandler ──► Engine + Subsystems
    ├─► ConfigHandler ──► *ServerLoader (read-only, internal/config/server_config.go)
    └─► AuditHandler ──► MessageStore (plugins/storage/)
    │
    ▼
JSON Response
```

---

## 7. Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "SESSION_NOT_FOUND",
    "message": "Session abc-123 not found",
    "details": {}
  }
}
```

| HTTP Status | Error Code | Description |
|-------------|------------|-------------|
| 400 | `INVALID_REQUEST` | Malformed JSON or missing required fields |
| 401 | `UNAUTHORIZED` | Missing or invalid X-Admin-Key |
| 404 | `SESSION_NOT_FOUND` | Session does not exist |
| 409 | `SESSION_ALREADY_STOPPING` | Session is already being stopped |
| 503 | `STORE_UNAVAILABLE` | Message store plugin not enabled (events/transcript endpoints) |
| 503 | `DRAIN_MODE` | Service in drain mode |

---

## 8. Security Considerations

1. **Separate key**: `HOTPLEX_ADMIN_API_KEY` is independent from `X-API-Key` used by client applications.
2. **Constant-time comparison**: Prevents timing attacks on key validation.
3. **No secret exposure**: Config endpoint redacts all sensitive fields.
4. **Audit logging**: All write operations log actor identity and operation details.
5. **Read-only config**: No write operations on configuration via admin API (prevents configuration drift).

## 8.1 Drain Mode Implementation Requirements

Drain mode is **not currently implemented** in `internal/engine/pool.go`. Implementation requires:

1. **`Engine.SetDrainMode(enabled bool, message string)`** — thread-safe via `atomic.Bool`
2. **`Engine.IsDraining() bool`** — checked in `HotPlexWSHandler.ServeHTTP` before upgrade
3. **ChatApps integration** — drain signal propagated to `AdapterManager` to reject new sessions (requires adapter interface extension)
4. **ChatApps `/webhook/*` routes** — check `Engine.IsDraining()` before accepting new requests

**Scoped drain (WebSocket-only)** is achievable without ChatApps changes. Full drain (including ChatApps) requires additional interface work and is tracked as a follow-up.

---

## 9. What's NOT in Scope

- WebSocket endpoint creation or management (existing `/ws/v1/agent` is self-contained)
- Writing configuration changes (read-only config)
- Plugin management
- Multi-tenant isolation (admin key grants full admin access; no per-key permissions)

---

## 10. Testing Strategy

1. **Unit tests**: Each handler tested with mocked dependencies
2. **Integration tests**: Against a running hotplexd instance with test admin key
3. **Auth tests**: Verify 401 on missing/invalid key
4. **Drain mode tests**: Verify new connections rejected during drain
5. **Metrics tests**: Verify Prometheus format compliance

---

## 11. Open Questions

| # | Question | Recommendation |
|---|----------|----------------|
| 1 | Should drain mode affect ChatApps adapters? | **No for MVP** — implement WebSocket-only drain first; ChatApps drain is a follow-up |
| 2 | Rate limiting on admin endpoints? | **Deferred to v2** — current design focuses on correctness |
| 3 | Should admin events be persisted? | **In-memory ring buffer** for MVP (capacity: last 10,000 events); persistent storage via message store plugin in v2 |
| 4 | `actor` filter on events endpoint? | **Included** — added to query parameters in v1 |
