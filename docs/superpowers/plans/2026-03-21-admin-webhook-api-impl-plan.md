# HotPlex Admin Webhook API — Implementation Plan

**Design**: `docs/superpowers/specs/2026-03-21-admin-webhook-api-design.md`
**Version**: v1 (MVP)

---

## Phase 1: Foundation (Types + Auth)

### 1.1 `internal/server/admin/types.go` — Data Models

Define all admin-facing structs.

- `AdminSession` — aligned with `engine.Session`, status mapped via `SessionStatus` enum
- `AdminSessionStats` — aligned with `engine.SessionStats`
- `AdminEvent` — in-memory ring buffer entry
- `AdminHealth` / `SubsystemHealth`
- `BatchStopRequest` / `BatchStopResponse` — with per-session error details
- `AdminError` — uniform error envelope `{error: {code, message, details}}`

**File**: `internal/server/admin/types.go`

### 1.2 `internal/server/admin/auth.go` — Auth Middleware

- `AdminAuthMiddleware(adminKey string) func(http.Handler) http.Handler`
- Reads `X-Admin-Key` header
- Constant-time compare via `subtle.ConstantTimeCompare`
- Logs failed attempts with key prefix (never logs the full key)

**Test**: `internal/server/admin/auth_test.go`

---

## Phase 2: Session Management

### 2.1 `internal/server/admin/session_handler.go`

Implements all `/api/v1/admin/sessions/*` routes.

**`GET /sessions`**
- `manager.ListActiveSessions()` → filter by `status` query param → map to `AdminSession[]`
- Pagination: `limit` (default 50, max 200), `offset`
- Platform field: resolve from `SessionID` prefix or registry (fallback `"unknown"`)

**`GET /sessions/{id}`**
- `manager.GetSession(id)` → map to `AdminSession`
- Returns `404` if not found

**`GET /sessions/{id}/stats`**
- `engine.GetSessionStats(id)` → map to `AdminSessionStats`
- Returns `404` if not found

**`POST /sessions/{id}/stop`**
- `engine.StopSession(id, reason)` — reason from request body
- Returns `409` if already stopping

**`POST /sessions/batch/stop`**
- Max 100 IDs — return `400` if exceeded
- Parallel stop, collect results
- `failed[]` entries include per-session error messages

**File**: `internal/server/admin/session_handler.go`
**Test**: `internal/server/admin/session_handler_test.go`

---

## Phase 3: Health & Metrics

### 3.1 `internal/server/admin/health_handler.go`

**`GET /health`**
- Check subsystems: engine (pool), security (WAF), storage (ping), config
- Map `hotplex.Version` into response
- Overall `status`: `"ok"` if all subsystems ok, `"degraded"` if one failed, `"unhealthy"` if multiple failed

**`GET /metrics`**
- Reuse or extend existing Prometheus handler in `internal/server/observability.go`
- Add new gauges: `hotplex_sessions_active{status, platform}`, counters from engine

**`POST /drain`** / **`DELETE /drain`** / **`GET /drain`**
- Requires `Engine.SetDrainMode()` (see Phase 6)
- MVP: WebSocket-only drain (scope-limited)

**File**: `internal/server/admin/health_handler.go`
**Test**: `internal/server/admin/health_handler_test.go`

---

## Phase 4: Configuration (Read-Only)

### 4.1 `internal/server/admin/config_handler.go`

**`GET /config`**
- `serverLoader.Get()` → deep-copy → redact sensitive fields
- Replace `api_keys`, `tokens`, `secrets`, `passwords` with `"[REDACTED]"`

**`GET /config/allowed_tools`**
- `engine.GetOptions().AllowedTools`

**`GET /config/disallowed_tools`**
- `engine.GetOptions().DisallowedTools`

**File**: `internal/server/admin/config_handler.go`
**Test**: `internal/server/admin/config_handler_test.go`

---

## Phase 5: Audit & History

### 5.1 `internal/server/admin/event_buffer.go` — Ring Buffer

- In-memory ring buffer: last 10,000 admin events
- Thread-safe via `sync.RWMutex`
- `Push(event)`, `Query(filter)` methods
- Event types: `"admin.session.stop"`, `"admin.drain.enter"`, `"admin.drain.exit"`, `"admin.health.check"`

### 5.2 `internal/server/admin/audit_handler.go`

**`GET /events`**
- Query ring buffer with filters: `session_id`, `type`, `actor`, `since`, `limit`, `cursor`
- Return `503 STORE_UNAVAILABLE` if message store unavailable

**`GET /sessions/{id}/transcript`**
- Delegate to message store plugin (`plugins/storage/`)
- Must implement `MessageStore.GetTranscript(sessionID)` interface
- Return `503 STORE_UNAVAILABLE` if store not enabled

**File**: `internal/server/admin/audit_handler.go`
**Test**: `internal/server/admin/audit_handler_test.go`

---

## Phase 6: Drain Mode (Scoped)

### 6.1 `internal/engine/drain.go`

```go
type DrainManager struct {
    enabled   atomic.Bool
    message   atomic.Value // string
    mu        sync.Mutex
}

func (d *DrainManager) SetDrainMode(enabled bool, msg string)
func (d *DrainManager) IsDraining() bool
func (d *DrainManager) DrainMessage() string
```

### 6.2 Integrate into Engine

- Embed `*DrainManager` in `Engine` struct
- Expose `SetDrainMode()`, `IsDraining()` methods
- Register drain endpoints in `health_handler.go`

### 6.3 Gate WebSocket Handler

In `internal/server/hotplex_ws.go` `ServeHTTP`:
- Before upgrade: check `engine.IsDraining()` → if true, return `503` with drain message

**Note**: ChatApps drain integration is out of scope for MVP.

---

## Phase 7: Integration & Registration

### 7.1 `internal/server/admin/admin.go` — Router

```go
type AdminHandler struct {
    sessionHandler *SessionHandler
    healthHandler *HealthHandler
    configHandler *ConfigHandler
    auditHandler  *AuditHandler
}

func NewAdminHandler(engine hotplex.HotPlexClient, serverLoader *ServerLoader, ...) *AdminHandler
func (h *AdminHandler) RegisterRoutes(r *mux.Router)
```

### 7.2 `cmd/hotplexd/main.go`

```go
adminKey := os.Getenv("HOTPLEX_ADMIN_API_KEY")
if adminKey != "" {
    adminRouter := mux.NewRouter()
    adminHandler := server.NewAdminHandler(engine, serverLoader, logger)
    adminRouter.Use(server.AdminAuthMiddleware(adminKey))
    adminHandler.RegisterRoutes(adminRouter)
    http.Handle("/api/v1/admin/", adminRouter)
    logger.Info("Admin API enabled")
} else {
    logger.Info("Admin API disabled (HOTPLEX_ADMIN_API_KEY not set)")
}
```

---

## Phase 8: End-to-End Tests

- `internal/server/admin/admin_test.go` — integration tests against mocked engine
- Verify `401` on missing/invalid key
- Verify `404` when admin API disabled
- Verify drain mode rejects new WebSocket connections
- Verify batch stop with mixed success/failure results

---

## File Manifest

```
internal/server/admin/
  types.go              # Data models
  auth.go               # Middleware
  event_buffer.go       # Ring buffer
  session_handler.go    # Session endpoints
  health_handler.go     # Health + metrics + drain
  config_handler.go     # Config read-only
  audit_handler.go      # Events + transcript
  admin.go              # Router + registration
  auth_test.go
  session_handler_test.go
  health_handler_test.go
  config_handler_test.go
  audit_handler_test.go
  admin_test.go

internal/engine/
  drain.go              # DrainManager (new file)

cmd/hotplexd/
  main.go               # Route registration (modify)
```

---

## Dependencies & Prerequisites

1. `gorilla/mux` — already used for route registration
2. `subtle.ConstantTimeCompare` — standard library
3. `engine.manager` field must be accessible — verify `internal/engine/pool.go` exports `ListActiveSessions()` and `GetSession()`
4. `engine.StopSession()` must accept a reason string — verify signature in `engine/runner.go:780`

---

## Rollout Order

1. Types + Auth (no dependencies)
2. Session Handler (depends on 1)
3. Health + Metrics (depends on 1)
4. Config Handler (depends on 1)
5. Audit Handler (depends on 1)
6. Drain Manager (depends on 4)
7. Gate WebSocket with drain (depends on 6)
8. Integration + Registration (depends on all above)
9. E2E Tests (depends on all above)

---

## Open Questions (Implementation-Time Decisions)

| # | Question | Decision Needed |
|---|----------|----------------|
| 1 | `AdminEvent` ring buffer — what triggers writes? | Wrap `engine.StopSession()`, `POST /drain` to write events |
| 2 | Transcript via message store — which interface method? | Define `Store.GetTranscript(sessionID)` in `plugins/storage/` |
| 3 | Prometheus metrics — extend existing or new handler? | Extend `internal/server/observability.go` to avoid duplication |
| 4 | Health subsystem checks — sync or async? | Sequential with timeout (3s per subsystem max) |
