# HotPlex Deep Dive Diagnostics

Enhanced diagnostic procedures specific to HotPlex AI Agent Control Plane.

## HotPlex Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    HotPlex Container                     │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │ HTTP Server │  │ WebSocket   │  │ ChatApp     │      │
│  │ (Port 8080) │  │ Gateway     │  │ Adapters    │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
│                         │                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │              Engine / Session Pool              │    │
│  │  • Session Management (PGID isolation)          │    │
│  │  • Process Lifecycle (spawn/kill/cleanup)       │    │
│  └─────────────────────────────────────────────────┘    │
│                         │                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │            AI CLI Providers (claude/opencode)   │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

## Diagnostic Endpoints

### Health Check

```bash
# Quick health (discover actual port first)
cd ~/hotplex/docker/matrix && docker compose ps  # Find port mapping
curl -s http://localhost:<discovered-port>/health | jq

# Expected response
{"status": "ok", "version": "0.30.4"}
```

### Metrics (if enabled)

```bash
curl -s http://localhost:<discovered-port>/metrics
```

## HotPlex-Specific Health Indicators

### 1. Socket Mode Connection Status

**Critical** for Slack connectivity. If disconnected, bot cannot receive messages.

```bash
# Check Socket Mode status (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  echo "=== $container ==="
  docker logs "${container}_1" --since 10m 2>&1 | grep -E "Socket Mode|Connected|Disconnected|invalid_auth"
done

# Good signs:
# - "Socket Mode connected"
# - "Connected to Slack"

# Bad signs:
# - "invalid_auth" - Token expired or invalid
# - "Disconnected" - Network or token issue
# - No Socket Mode logs - Never connected
```

### 2. Session Pool Health

Active sessions indicate CLI processes running.

```bash
# Count active sessions across all containers
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  active=$(docker exec "$instance" ps aux 2>/dev/null | grep -cE "claude|opencode" || echo "0")
  echo "$container: $active active sessions"
done

# Check for zombie sessions (processes without parent)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  zombies=$(docker exec "$instance" ps aux 2>/dev/null | awk '$8=="Z" {print}' || echo "")
  if [ -n "$zombies" ]; then
    echo "=== $container ==="
    echo "$zombies"
  fi
done

# Check PGID-based session tracking
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  echo "=== $container ==="
  docker exec "$instance" ps -eo pid,pgid,comm 2>/dev/null | grep -E "claude|opencode"
done
```

### 3. WAF (Web Application Firewall) Status

WAF blocks malicious inputs to CLI.

```bash
# Check for WAF blocks across all containers
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  echo "=== $container ==="
  docker logs "${container}_1" --since 1h 2>&1 | grep -i "waf\|blocked\|security" | head -5
done

# High WAF blocks may indicate:
# - Attack attempt
# - Malformed user input
# - Misconfigured client
```

### 4. Memory Pressure

HotPlex sessions consume memory for context.

```bash
# Check memory usage for all HotPlex containers
cd ~/hotplex/docker/matrix && \
docker stats --no-stream $(docker compose ps -q) --format "table {{.Name}}\t{{.MemUsage}}\t{{.MemPerc}}"

# High memory may indicate:
# - Long-running sessions
# - Memory leak in CLI
# - Too many concurrent sessions
```

### 5. Disk Usage

Sessions store state and logs.

```bash
# HotPlex data directory (all containers)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  echo "=== $container ==="
  docker exec "$instance" du -sh /home/hotplex/.hotplex 2>/dev/null || echo "  N/A"
  docker exec "$instance" du -sh /home/hotplex/.claude 2>/dev/null || echo "  N/A"
done

# Check for large log files
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  large_logs=$(docker exec "$instance" find /home/hotplex -name "*.log" -size +10M 2>/dev/null || echo "")
  if [ -n "$large_logs" ]; then
    echo "=== $container ==="
    echo "$large_logs"
  fi
done
```

## Common Issues & Solutions

### Issue: Bot Not Responding to Messages

**Diagnosis**:
```bash
# 1. Check container status
cd ~/hotplex/docker/matrix && docker compose ps

# 2. Discover port and check HTTP health (for any bot)
port=$(docker port $(docker compose ps -q | head -1) 8080 2>/dev/null | grep -oP ':\K\d+')
curl -s http://localhost:$port/health

# 3. Check Socket Mode (for any bot)
cd ~/hotplex/docker/matrix && \
docker compose logs --tail=50 2>&1 | grep -i "socket\|slack\|connect"

# 4. Check for errors (for any bot)
cd ~/hotplex/docker/matrix && \
docker compose logs --tail=100 2>&1 | grep -i "error\|fatal"
```

**Common Causes**:
| Symptom | Cause | Solution |
|:--------|:------|:---------|
| `invalid_auth` | Token expired | Regenerate Slack tokens |
| No Socket Mode logs | Never connected | Check SLACK_APP_TOKEN |
| Connection drops | Network issue | Check network stability |
| High CPU | Session overload | Reduce concurrent sessions |

### Issue: Session Not Starting

**Diagnosis**:
```bash
# 1. Check for CLI availability (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services | head -1); do
  instance="${container}_1"
  docker exec "$instance" which claude
  docker exec "$instance" which opencode
done

# 2. Check API keys (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services | head -1); do
  instance="${container}_1"
  docker exec "$instance" env | grep -i "API_KEY\|TOKEN" | head -5
done

# 3. Check work directory (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services | head -1); do
  instance="${container}_1"
  docker exec "$instance" ls -la /home/hotplex/projects 2>/dev/null
done

# 4. Check process limits (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services | head -1); do
  instance="${container}_1"
  docker exec "$instance" cat /proc/1/limits | grep -i "process\|file"
done
```

### Issue: Session Hang/Timeout

**Diagnosis**:
```bash
# 1. Find hanging session (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  echo "=== $container ==="
  docker exec "$instance" ps aux 2>/dev/null | grep -E "claude|opencode"
done

# 2. Check session logs (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs --since 10m 2>&1 | grep -i "session_id\|timeout\|hang"

# 3. Check for zombie processes (for any container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  zombies=$(docker exec "$instance" ps aux 2>/dev/null | awk '$8=="Z"' || echo "")
  if [ -n "$zombies" ]; then
    echo "=== $container ==="
    echo "$zombies"
  fi
done

# 4. Force kill session via Admin API (RECOMMENDED)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$admin_port" ]; then
    # List busy sessions
    busy_sessions=$(curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      http://localhost:$admin_port/admin/v1/sessions 2>/dev/null | \
      jq -r '.sessions[] | select(.status == "busy") | .id')
    if [ -n "$busy_sessions" ]; then
      echo "=== $container: Found busy sessions ==="
      echo "$busy_sessions"
      # Uncomment to terminate:
      # echo "$busy_sessions" | while read sid; do
      #   curl -X DELETE -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      #     http://localhost:$admin_port/admin/v1/sessions/"$sid"
      # done
    fi
  fi
done

# 5. Alternative: Force kill session by PGID (if Admin API unavailable)
# docker exec <container> kill -9 -<PGID>
```

## Admin API Diagnostics

### Quick Status Check (All Bots)

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$admin_port" ]; then
    echo "=== $container ==="
    curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      http://localhost:$admin_port/admin/v1/stats 2>/dev/null | jq || echo "  FAILED"
  fi
done
```

### Detailed Health Check

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$admin_port" ]; then
    echo "=== $container ==="
    curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      http://localhost:$admin_port/admin/v1/health/detailed 2>/dev/null | \
      jq '{status, cli_version: .details.cli_version, db_latency_ms: .details.database_latency_ms}' || \
      echo "  FAILED"
  fi
done
```

### List Active Sessions (All Bots)

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$admin_port" ]; then
    echo "=== $container ==="
    curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
      http://localhost:$admin_port/admin/v1/sessions 2>/dev/null | \
      jq -r '.sessions[] | "\(.id) - \(.status) - \(.last_active)"' || \
      echo "  No sessions or FAILED"
  fi
done
```

## Multi-Bot Cluster Diagnostics

### Check All Bots

```bash
cd ~/hotplex/docker/matrix

# Status
docker compose ps

# Health endpoints (discover ports dynamically)
echo "Main servers:"
for container in $(docker compose ps --services); do
  port=$(docker port "${container}_1" 8080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$port" ]; then
    status=$(curl -s http://localhost:$port/health 2>/dev/null || echo "FAILED")
    echo "$container (port $port): $status"
  fi
done

echo "Admin servers:"
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$admin_port" ]; then
    status=$(curl -s http://localhost:$admin_port/admin/v1/stats 2>/dev/null || echo "FAILED")
    echo "$container (port $admin_port): $status"
  fi
done

# Resource comparison
docker stats --no-stream $(docker compose ps -q) --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"
```

### Session Distribution

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  count=$(docker exec "$instance" ps aux 2>/dev/null | grep -cE "claude|opencode" || echo "0")
  echo "$container: $count active sessions"
done
```

### Bot User ID Collision Check

**Critical**: Each bot must have unique `bot_user_id`.

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  bot_id=$(docker exec "$instance" env 2>/dev/null | grep "HOTPLEX_SLACK_BOT_USER_ID" | cut -d= -f2)
  echo "$container: $bot_id"
done

# All should be DIFFERENT!
```

## Log Analysis Patterns

### Session Lifecycle

```bash
# Session started (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs 2>&1 | grep "session.*started"

# Session completed (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs 2>&1 | grep "session.*completed"

# Session errors (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs 2>&1 | grep -E "session.*error|session.*failed"
```

### Message Flow

```bash
# Incoming messages (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs 2>&1 | grep -E "message.*received|incoming.*message"

# Outgoing messages (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs 2>&1 | grep -E "message.*sent|outgoing"

# Message chunks (large messages split) (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs 2>&1 | grep -i "chunk\|split"
```

### Performance Metrics

```bash
# Execution time distribution (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs --since 1h 2>&1 | grep -oP "duration.*?ms" | sort | uniq -c

# Token usage (for any container)
cd ~/hotplex/docker/matrix && \
docker compose logs --since 1h 2>&1 | grep -i "token" | tail -20
```

## Proactive Monitoring

### Recommended Checks

| Frequency | Check | Command |
|:----------|:------|:--------|
| Every 5min | Health endpoint | `cd ~/hotplex/docker/matrix && for c in $(docker compose ps --services); do p=$(docker port "${c}_1" 8080 2>/dev/null | grep -oP ':\K\d+'); [ -n "$p" ] && curl -s localhost:$p/health; done` |
| Every 1min | Socket Mode | `cd ~/hotplex/docker/matrix && docker compose logs --tail=10 \| grep Socket` |
| Hourly | Error count | `cd ~/hotplex/docker/matrix && docker compose logs --since=1h \| grep -c error` |
| Daily | Disk usage | `cd ~/hotplex/docker/matrix && for c in $(docker compose ps --services); do docker exec "${c}_1" du -sh ~/.hotplex 2>/dev/null; done` |
| Daily | Restart count | `cd ~/hotplex/docker/matrix && for c in $(docker compose ps -q); do docker inspect $c --format='{{.Name}}: {{.RestartCount}}'; done` |

### Alert Thresholds

| Metric | Warning | Critical |
|:-------|:--------|:---------|
| Memory | >70% | >90% |
| CPU (sustained) | >60% for 5min | >80% for 5min |
| Error rate | >10/hour | >50/hour |
| Restarts | >1/hour | >3/hour |
| Disk | >70% | >90% |
