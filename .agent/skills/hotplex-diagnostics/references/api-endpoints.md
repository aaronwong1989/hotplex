# HotPlex API Endpoints

Complete API reference for hotplex WebSocket and HTTP endpoints.

## Base URLs

**Note**: The ports shown below are examples. Actual ports depend on bot configuration.
Use `cd ~/hotplex/docker/matrix && docker compose ps` to discover actual port mappings.

Port numbering convention:
- **Main Port (WebSocket/HTTP)**: `18080 + (BOT_INDEX - 1)`
- **Admin Port (Session Management)**: `19080 + (BOT_INDEX - 1)`

### Main Server Ports (WebSocket/HTTP)
- Bot 01: `http://localhost:18080`
- Bot 02: `http://localhost:18081`
- Bot 03: `http://localhost:18082`

### Admin API Ports (Session Management)
- Bot 01: `http://localhost:19080`
- Bot 02: `http://localhost:19081`
- Bot 03: `http://localhost:19082`

## WebSocket API

### Connection

**Example** (replace with actual port):
```
ws://localhost:<main-port>/ws
```

### Client Request Format

```json
{
  "request_id": 1,
  "type": "execute|stop|stats|version",
  "session_id": "session-123",
  "prompt": "Hello",
  "instructions": "Be helpful",
  "system_prompt": "Custom system prompt",
  "work_dir": "/home/hotplex/projects"
}
```

### Server Response Format

```json
{
  "request_id": 1,
  "event": "message|completed|error|stopped",
  "data": {}
}
```

## Request Types

### Execute

Start a new execution:

```json
{
  "type": "execute",
  "session_id": "my-session",
  "prompt": "List files in current directory",
  "instructions": "Use ls -la",
  "work_dir": "/home/hotplex/projects/myproject"
}
```

### Stop

Stop a running session:

```json
{
  "type": "stop",
  "session_id": "my-session",
  "reason": "user_requested"
}
```

### Stats

Get session statistics:

```json
{
  "type": "stats",
  "session_id": "my-session"
}
```

### Version

Get CLI version:

```json
{
  "type": "version"
}
```

## Response Events

### Message Event

```json
{
  "event": "message",
  "data": {
    "type": "content",
    "content": "Here are the files..."
  }
}
```

### Completed Event

```json
{
  "event": "completed",
  "data": {
    "session_id": "my-session",
    "stats": {
      "input_tokens": 1000,
      "output_tokens": 500
    }
  }
}
```

### Error Event

```json
{
  "event": "error",
  "data": {
    "message": "Execution failed: ..."
  }
}
```

### Stopped Event

```json
{
  "event": "stopped",
  "data": {
    "session_id": "my-session"
  }
}
```

## HTTP Endpoints (Main Server - Port 8080)

### Health Check

```
GET /health
```

Response:
```json
{
  "status": "ok",
  "version": "0.24.0"
}
```

### Metrics

```
GET /metrics
```

Returns Prometheus-format metrics.

---

## Admin API (Admin Server - Port 9080)

**Note**: Admin port is internal (9080), but mapped externally via Docker. Use dynamic discovery to find actual port.

### Port Discovery

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\\K\\d+')
  if [ -n "$admin_port" ]; then
    echo "$container admin port: $admin_port"
  fi
done
```

### Authentication

All Admin API requests require Bearer token authentication:

```bash
curl -H "Authorization: Bearer $HOTPLEX_ADMIN_TOKEN" http://localhost:$admin_port/admin/v1/stats
```

**Note**: `HOTPLEX_ADMIN_TOKEN` is set in `.env` file.

---

### Session Management

#### List All Sessions

```
GET /admin/v1/sessions
```

Response:
```json
{
  "sessions": [
    {
      "id": "slack:U123:BOT_U0AHRCL1KCM:C456:T789",
      "status": "ready",
      "created_at": "2024-01-15T10:30:00Z",
      "last_active": "2024-01-15T11:45:00Z"
    }
  ],
  "total": 1
}
```

**Example**:
```bash
# List sessions for specific bot
cd ~/hotplex/docker/matrix && \
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\\K\\d+') && \
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions | jq
```

#### Get Session Details

```
GET /admin/v1/sessions/:id
```

Response:
```json
{
  "id": "slack:U123:BOT_U0AHRCL1KCM:C456:T789",
  "status": "ready",
  "created_at": "2024-01-15T10:30:00Z",
  "last_active": "2024-01-15T11:45:00Z",
  "config": {
    "provider": "claude-code",
    "work_dir": "/home/hotplex/projects/myproject"
  },
  "stats": {
    "input_tokens": 5000,
    "output_tokens": 2500,
    "duration_secs": 120
  }
}
```

**Example**:
```bash
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions/slack:U123:BOT_U0AHRCL1KCM:C456:T789 | jq
```

#### Terminate Session

```
DELETE /admin/v1/sessions/:id
```

Response:
```json
{
  "success": true,
  "message": "Session slack:U123:BOT_U0AHRCL1KCM:C456:T789 terminated"
}
```

**Example** - Force kill hung session:
```bash
curl -X DELETE -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions/slack:U123:BOT_U0AHRCL1KCM:C456:T789
```

#### Get Session Logs Metadata

```
GET /admin/v1/sessions/:id/logs
```

Response:
```json
{
  "session_id": "slack:U123:BOT_U0AHRCL1KCM:C456:T789",
  "log_path": "/home/hotplex/.hotplex/logs/slack:U123:BOT_U0AHRCL1KCM:C456:T789.log",
  "size_bytes": 102400,
  "last_modified": "2024-01-15T11:45:00Z"
}
```

**Note**: This returns log metadata. To read actual logs:
```bash
docker exec <container> cat /home/hotplex/.hotplex/logs/<session-id>.log
```

---

### Statistics & Monitoring

#### Get Runtime Stats

```
GET /admin/v1/stats
```

Response:
```json
{
  "total_sessions": 5,
  "active_sessions": 3,
  "stopped_sessions": 2,
  "uptime": "2h30m15s",
  "memory_usage_mb": 128.5,
  "cpu_usage_percent": 5.2
}
```

**Example**:
```bash
# Quick status check
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\\K\\d+')
  if [ -n "$admin_port" ]; then
    echo "=== $container ==="
    curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      http://localhost:$admin_port/admin/v1/stats | jq
  fi
done
```

#### Detailed Health Check

```
GET /admin/v1/health/detailed
```

Response:
```json
{
  "status": "healthy",
  "checks": {
    "config": true,
    "cli_available": true,
    "database": true,
    "websocket_connections": 3
  },
  "details": {
    "database_latency_ms": 2,
    "cli_version": "claude-code v2.1.81"
  }
}
```

**Status Values**:
- `healthy` - All checks passed
- `degraded` - Non-critical checks failed (e.g., database not configured)

**Example**:
```bash
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\\K\\d+') && \
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/health/detailed | jq
```

---

### Configuration Management

#### Validate Configuration File

```
POST /admin/v1/config/validate
```

Request Body:
```json
{
  "config_path": "/home/hotplex/.hotplex/config.yaml"
}
```

Response:
```json
{
  "valid": true,
  "errors": []
}
```

**Validation Errors Example**:
```json
{
  "valid": false,
  "errors": [
    "missing required field: server",
    "missing required field: engine"
  ]
}
```

**Example**:
```bash
curl -X POST \
  -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"config_path": "/home/hotplex/.hotplex/config.yaml"}' \
  http://localhost:$admin_port/admin/v1/config/validate | jq
```

---

## Admin CLI Commands

### status - Runtime Overview

```bash
hotplexd status
```

Output:
```
HotPlex Daemon Status
=====================
Total Sessions:   5
Active Sessions:  3
Stopped Sessions: 2
Uptime:           2h30m15s
Memory Usage:     128.50 MB
CPU Usage:        5.20%
```

**Note**: `hotplexd status` calls `/admin/v1/stats` internally.

### doctor - Diagnostic Checks

```bash
hotplexd doctor
```

Output:
```
Running HotPlex diagnostic checks...

[✓ PASS] CLI Binary (claude-code)
       Version: claude-code v2.1.81

[✓ PASS] Configuration Files
       Found: /home/hotplex/.hotplex/config.yaml

[✓ PASS] Environment Variables
       all required variables set

[✓ PASS] Port Availability (8080)
       port 8080 is available

[✓ PASS] Port Availability (9080)
       port 9080 is available

[✓ PASS] Database (SQLite)
       database accessible

All checks passed!
```

**Checks**:
- CLI binary availability and version
- Configuration file validity
- Required environment variables
- Port availability (8080, 9080)
- Database connectivity (if configured)

### config validate - Validate Config File

```bash
hotplexd config validate /path/to/config.yaml
```

Output (Success):
```
Configuration is valid
```

Output (Errors):
```
Configuration errors:
  - missing required field: server
  - invalid YAML: line 15: mapping values are not allowed here
```

---

## Error Codes

| Code | Description |
|------|-------------|
| 4001 | Invalid request format |
| 4002 | Session not found |
| 4003 | Execution timeout |
| 4004 | Session already running |
| 5001 | Internal server error |

---

## Common Admin Operations

### Kill Hung Session

```bash
# 1. Find hung session
cd ~/hotplex/docker/matrix && \
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\\K\\d+') && \
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions | jq '.sessions[] | select(.status == "busy")'

# 2. Terminate via API
curl -X DELETE -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions/<session-id>
```

### Monitor All Bots Health

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\\K\\d+')
  if [ -n "$admin_port" ]; then
    echo "=== $container ==="
    curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      http://localhost:$admin_port/admin/v1/health/detailed | jq '{status, cli_version: .details.cli_version}'
  fi
done
```

## Session ID Format

Session IDs follow the format:
```
platform:userID:botUserID:channelID:threadID
```

Example:
```
slack:U123456:BOT_U0AHRCL1KCM:C123456:T123456
```

## Error Codes

| Code | Description |
|------|-------------|
| 4001 | Invalid request format |
| 4002 | Session not found |
| 4003 | Execution timeout |
| 4004 | Session already running |
| 5001 | Internal server error |
