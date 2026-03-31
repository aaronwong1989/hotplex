---
name: hotplex-diagnostics
description: Use this skill when the user asks to "diagnose", "check health", "view logs", "debug", "check status", "get stats", "container logs", "check error", "troubleshoot", "what's wrong with", "diagnose docker", "check containers", "system health". Provides comprehensive diagnostics for all Docker containers with enhanced analysis for HotPlex services.
version: 0.3.0
---

# Docker & HotPlex Diagnostics

Comprehensive diagnostic system for Docker containers with enhanced capabilities for HotPlex services.

## Diagnostic Layers

| Layer | Scope | Depth |
|:------|:------|:------|
| **Layer 1** | All Docker containers | Basic health & status |
| **Layer 2** | All containers | Resource usage & logs |
| **Layer 3** | HotPlex containers | Deep application diagnostics |

## Quick Start

### Full System Scan (All Containers)

```bash
# Layer 1: Quick health of ALL containers
docker ps -a --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | head -20

# Layer 2: Resource usage of ALL containers
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"
```

### HotPlex-Only Quick Check

```bash
cd ~/hotplex/docker/matrix && docker compose ps
```

---

## Layer 1: Universal Container Health

### List All Running Containers

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
```

### List All Containers (Including Stopped)

```bash
docker ps -a --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
```

### Health Status Check (All Containers with Healthchecks)

```bash
docker ps --format "{{.Names}}" | while read c; do
  health=$(docker inspect "$c" --format='{{.State.Health.Status}}' 2>/dev/null || echo "n/a")
  printf "%-30s %s\n" "$c" "$health"
done
```

### Container State Summary

```bash
echo "=== Container State Summary ===" && \
docker ps -a --format "{{.State}}" | sort | uniq -c | sort -rn
```

### Recent Container Events (Restarts, Deaths)

```bash
docker events --since 1h --filter 'type=container' --format '{{.Action}} {{.Actor.Attributes.name}}' 2>/dev/null | tail -20
```

---

## Layer 2: Resource & Log Analysis

### Resource Usage (All Containers)

```bash
# CPU & Memory
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"

# Top CPU consumers
docker stats --no-stream --format "{{.Name}}\t{{.CPUPerc}}" | sort -t$'\t' -k2 -rn | head -5

# Top Memory consumers
docker stats --no-stream --format "{{.Name}}\t{{.MemPerc}}" | sort -t$'\t' -k2 -rn | head -5
```

### Network I/O Analysis

```bash
docker stats --no-stream --format "table {{.Name}}\t{{.NetIO}}\t{{.BlockIO}}"
```

### Error Scan (All Containers)

```bash
echo "=== Recent Errors (All Containers) ===" && \
for c in $(docker ps --format "{{.Names}}"); do
  errors=$(docker logs "$c" --since 1h 2>&1 | grep -iE "error|fatal|panic|exception" | head -3)
  if [ -n "$errors" ]; then
    echo "--- $c ---"
    echo "$errors"
  fi
done
```

### Container Restart History

```bash
for c in $(docker ps -a --format "{{.Names}}"); do
  restarts=$(docker inspect "$c" --format='{{.RestartCount}}' 2>/dev/null || echo "0")
  if [ "$restarts" -gt 0 ]; then
    echo "$c: $restarts restarts"
  fi
done
```

---

## Layer 3: HotPlex Enhanced Diagnostics

**Working Directory**: `~/hotplex/docker/matrix`

### Container Discovery

**IMPORTANT**: Do not hardcode container details. Always discover containers dynamically:

```bash
# Discover HotPlex containers
cd ~/hotplex/docker/matrix && docker compose ps

# Get container names and ports
docker ps --format "table {{.Names}}\t{{.Ports}}" | grep hotplex
```

### Port Mapping Convention

HotPlex follows a predictable port numbering pattern:

- **Main Port (WebSocket/HTTP)**: `18080 + (BOT_INDEX - 1)`
- **Admin Port (Session Management)**: `19080 + (BOT_INDEX - 1)`

Examples:
- Bot 01: Main=18080, Admin=19080
- Bot 02: Main=18081, Admin=19081
- Bot 03: Main=18082, Admin=19082

**However**, always verify actual ports using `docker compose ps` rather than assuming.

### HTTP Health Check

```bash
# Discover HotPlex containers and check health
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  port=$(docker port "${container}_1" 8080 2>/dev/null | grep -oP ':\K\d+' || echo "N/A")
  if [ "$port" != "N/A" ]; then
    status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$port/health 2>/dev/null || echo "000")
    echo "$container (port $port): HTTP $status"
  fi
done

# External services (example - only if running)
if docker ps --format '{{.Names}}' | grep -q openclaw-gateway; then
  echo "OpenClaw Gateway:"
  curl -s http://localhost:18789/health 2>/dev/null || echo "  FAILED or not running"
fi
```

### Socket Mode Status

```bash
cd ~/hotplex/docker/matrix && \
docker compose logs --tail=100 2>&1 | grep -E "Socket Mode|invalid_auth|Connected|Disconnected"
```

### Session Diagnostics

```bash
# Discover containers and check active sessions
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  active=$(docker exec "$instance" ps aux 2>/dev/null | grep -cE "claude|opencode" || echo "0")
  echo "$container: $active active sessions"
done

# Session count for specific container (example)
# docker logs hotplex-01 --since 1h 2>&1 | grep -c "session_id"
```

### HotPlex-Specific Errors

```bash
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  echo "=== $container ==="
  docker logs "$instance" --since 1h 2>&1 | grep -iE "error|waf|blocked|timeout" | head -5
done
```

### Disk Usage (HotPlex)

```bash
# Check disk usage for all HotPlex containers
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services); do
  instance="${container}_1"
  echo "=== $container ==="
  docker exec "$instance" du -sh /home/hotplex/.hotplex 2>/dev/null || echo "  N/A"
  docker exec "$instance" du -sh /home/hotplex/.claude 2>/dev/null || echo "  N/A"
done
```

### OpenClaw Gateway Diagnostics

**Note**: `openclaw-gateway` runs independently (not in docker-compose.yml).

```bash
# Health check
curl -s http://localhost:18789/health | jq

# Container status
docker ps --filter "name=openclaw-gateway" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# Recent logs
docker logs openclaw-gateway --tail=100 2>&1

# Error scan
docker logs openclaw-gateway --since 1h 2>&1 | grep -iE "error|fatal|panic|exception"

# Resource usage
docker stats --no-stream openclaw-gateway --format "CPU: {{.CPUPerc}}, Memory: {{.MemUsage}}"

# If container not found, check if it's supposed to be running
echo "Note: openclaw-gateway is not managed by docker-compose."
echo "It may be started by a separate compose file or manually."
```

---

## Admin CLI Commands

HotPlex provides admin CLI commands for quick diagnostics:

### Quick Status Check

```bash
# Check daemon status (from any bot container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services | head -1); do
  docker exec "${container}_1" hotplexd status
done
```

### Diagnostic Checks

```bash
# Run doctor diagnostic (from any bot container)
cd ~/hotplex/docker/matrix && \
for container in $(docker compose ps --services | head -1); do
  echo "=== Running doctor in $container ==="
  docker exec "${container}_1" hotplexd doctor
done
```

**Doctor checks**:
- CLI binary availability (claude-code)
- Configuration file validity
- Required environment variables
- Port availability (8080, 9080)
- Database connectivity (if configured)

### Config Validation

```bash
# Validate configuration file (from any bot container)
cd ~/hotplex/docker/matrix && \
container=$(docker compose ps --services | head -1) && \
docker exec "${container}_1" hotplexd config validate /home/hotplex/.hotplex/config.yaml
```

---

## Diagnostic Workflows

### Workflow 1: Full System Health Report

Use when user asks for general system health or "check everything":

```bash
echo "=== Docker System Health Report ===" && \
echo "" && \
echo ">>> Container States" && \
docker ps -a --format "{{.State}}" | sort | uniq -c && \
echo "" && \
echo ">>> Resource Usage (Top 10)" && \
docker stats --no-stream --format "{{.Name}}\t{{.CPUPerc}}\t{{.MemPerc}}" | head -11 && \
echo "" && \
echo ">>> Recent Errors" && \
docker ps --format "{{.Names}}" | head -5 | while read c; do
  e=$(docker logs "$c" --since 30m 2>&1 | grep -ciE "error|fatal")
  [ "$e" -gt 0 ] && echo "$c: $e errors"
done
```

### Workflow 2: Container-Specific Deep Dive

Use when user asks about a specific container:

```bash
CONTAINER="<container-name>"

echo "=== Deep Dive: $CONTAINER ===" && \
echo "" && \
echo ">>> Status" && \
docker inspect "$CONTAINER" --format='Status: {{.State.Status}}, Health: {{.State.Health.Status}}, Restarts: {{.RestartCount}}' 2>/dev/null && \
echo "" && \
echo ">>> Resources" && \
docker stats --no-stream "$CONTAINER" --format "CPU: {{.CPUPerc}}, Memory: {{.MemUsage}}" 2>/dev/null && \
echo "" && \
echo ">>> Recent Logs (Last 50 lines)" && \
docker logs "$CONTAINER" --tail=50 2>&1
```

### Workflow 3: HotPlex Cluster Health

Use when user asks about HotPlex specifically:

```bash
echo "=== HotPlex Cluster Health ===" && \
cd ~/hotplex/docker/matrix && \
echo "" && \
echo ">>> Container Status" && \
docker compose ps && \
echo "" && \
echo ">>> HTTP Health" && \
echo "Main servers:" && \
for container in $(docker compose ps --services); do
  port=$(docker port "${container}_1" 8080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$port" ]; then
    status=$(curl -s http://localhost:$port/health 2>/dev/null || echo "FAILED")
    echo "$container (port $port): $status"
  fi
done && \
echo "Admin servers:" && \
for container in $(docker compose ps --services); do
  admin_port=$(docker port "${container}_1" 9080 2>/dev/null | grep -oP ':\K\d+')
  if [ -n "$admin_port" ]; then
    status=$(curl -s http://localhost:$admin_port/admin/v1/stats 2>/dev/null || echo "FAILED")
    echo "$container (port $admin_port): $status"
  fi
done && \
echo "" && \
echo ">>> Resource Usage" && \
docker stats --no-stream $(docker compose ps -q) --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" 2>/dev/null
```

---

## Common Container Health Indicators

| Indicator | Healthy | Warning | Critical |
|:----------|:--------|:--------|:---------|
| Status | running | paused, restarting | exited, dead |
| Health | healthy | starting | unhealthy |
| Restarts | 0 | 1-3 | >3 |
| CPU | <50% | 50-80% | >80% |
| Memory | <70% | 70-90% | >90% |

## Reference Documentation

For deeper diagnostics, consult these reference files:

- `references/container-health.md` - Universal container health indicators and thresholds
- `references/hotplex-deep-dive.md` - HotPlex-specific diagnostic procedures
- `references/api-endpoints.md` - HotPlex WebSocket and HTTP API reference

## Diagnostic Script

For automated comprehensive diagnostics:

```bash
# Run full diagnostic scan
~/.agent/skills/hotplex-diagnostics/scripts/diagnose-all.sh all

# Specific layers
~/.agent/skills/hotplex-diagnostics/scripts/diagnose-all.sh 1     # Health only
~/.agent/skills/hotplex-diagnostics/scripts/diagnose-all.sh 2     # Resources only
~/.agent/skills/hotplex-diagnostics/scripts/diagnose-all.sh 3     # Logs only

# Deep dive on specific container
~/.agent/skills/hotplex-diagnostics/scripts/diagnose-all.sh <container-name>
```

## Related Skills

- `docker-container-ops` - Container lifecycle management
- `hotplex-data-mgmt` - Data and session management
