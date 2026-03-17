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
# Quick health
curl -s http://localhost:18080/health | jq

# Expected response
{"status": "ok", "version": "0.30.4"}
```

### Metrics (if enabled)

```bash
curl -s http://localhost:18080/metrics
```

## HotPlex-Specific Health Indicators

### 1. Socket Mode Connection Status

**Critical** for Slack connectivity. If disconnected, bot cannot receive messages.

```bash
# Check Socket Mode status
docker logs hotplex-01 --since 10m 2>&1 | grep -E "Socket Mode|Connected|Disconnected|invalid_auth"

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
# Count active sessions
docker exec hotplex-01 ps aux | grep -cE "claude|opencode"

# Check for zombie sessions (processes without parent)
docker exec hotplex-01 ps aux | awk '$8=="Z" {print}'

# Check PGID-based session tracking
docker exec hotplex-01 ps -eo pid,pgid,comm | grep -E "claude|opencode"
```

### 3. WAF (Web Application Firewall) Status

WAF blocks malicious inputs to CLI.

```bash
# Check for WAF blocks
docker logs hotplex-01 --since 1h 2>&1 | grep -i "waf\|blocked\|security"

# High WAF blocks may indicate:
# - Attack attempt
# - Malformed user input
# - Misconfigured client
```

### 4. Memory Pressure

HotPlex sessions consume memory for context.

```bash
# Check memory usage
docker stats --no-stream hotplex-01 --format "Memory: {{.MemUsage}} ({{.MemPerc}})"

# High memory may indicate:
# - Long-running sessions
# - Memory leak in CLI
# - Too many concurrent sessions
```

### 5. Disk Usage

Sessions store state and logs.

```bash
# HotPlex data directory
docker exec hotplex-01 du -sh /home/hotplex/.hotplex

# Claude CLI cache
docker exec hotplex-01 du -sh /home/hotplex/.claude

# Check for large log files
docker exec hotplex-01 find /home/hotplex -name "*.log" -size +10M 2>/dev/null
```

## Common Issues & Solutions

### Issue: Bot Not Responding to Messages

**Diagnosis**:
```bash
# 1. Check container status
docker ps | grep hotplex

# 2. Check HTTP health
curl -s http://localhost:18080/health

# 3. Check Socket Mode
docker logs hotplex-01 --tail=50 | grep -i "socket\|slack\|connect"

# 4. Check for errors
docker logs hotplex-01 --tail=100 | grep -i "error\|fatal"
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
# 1. Check for CLI availability
docker exec hotplex-01 which claude
docker exec hotplex-01 which opencode

# 2. Check API keys
docker exec hotplex-01 env | grep -i "API_KEY\|TOKEN" | head -5

# 3. Check work directory
docker exec hotplex-01 ls -la /home/hotplex/projects 2>/dev/null

# 4. Check process limits
docker exec hotplex-01 cat /proc/1/limits | grep -i "process\|file"
```

### Issue: Session Hang/Timeout

**Diagnosis**:
```bash
# 1. Find hanging session
docker exec hotplex-01 ps aux | grep -E "claude|opencode"

# 2. Check session logs
docker logs hotplex-01 --since 10m | grep -i "session_id\|timeout\|hang"

# 3. Check for zombie processes
docker exec hotplex-01 ps aux | awk '$8=="Z"'

# 4. Force kill session by PGID
# docker exec hotplex-01 kill -9 -<PGID>
```

## Multi-Bot Cluster Diagnostics

### Check All Bots

```bash
cd ~/hotplex/docker/matrix

# Status
docker compose ps

# Health endpoints
for port in 18080 18081 18082; do
  echo "Port $port:"
  curl -s http://localhost:$port/health
  echo
done

# Resource comparison
docker stats --no-stream hotplex-01 hotplex-02 hotplex-03
```

### Session Distribution

```bash
for bot in hotplex-01 hotplex-02 hotplex-03; do
  count=$(docker exec $bot ps aux 2>/dev/null | grep -cE "claude|opencode" || echo "0")
  echo "$bot: $count active sessions"
done
```

### Bot User ID Collision Check

**Critical**: Each bot must have unique `bot_user_id`.

```bash
for bot in hotplex-01 hotplex-02 hotplex-03; do
  bot_id=$(docker exec $bot env | grep "HOTPLEX_SLACK_BOT_USER_ID" | cut -d= -f2)
  echo "$bot: $bot_id"
done

# All should be DIFFERENT!
```

## Log Analysis Patterns

### Session Lifecycle

```bash
# Session started
docker logs hotplex-01 | grep "session.*started"

# Session completed
docker logs hotplex-01 | grep "session.*completed"

# Session errors
docker logs hotplex-01 | grep -E "session.*error|session.*failed"
```

### Message Flow

```bash
# Incoming messages
docker logs hotplex-01 | grep -E "message.*received|incoming.*message"

# Outgoing messages
docker logs hotplex-01 | grep -E "message.*sent|outgoing"

# Message chunks (large messages split)
docker logs hotplex-01 | grep -i "chunk\|split"
```

### Performance Metrics

```bash
# Execution time distribution
docker logs hotplex-01 --since 1h | grep -oP "duration.*?ms" | sort | uniq -c

# Token usage
docker logs hotplex-01 --since 1h | grep -i "token" | tail -20
```

## Proactive Monitoring

### Recommended Checks

| Frequency | Check | Command |
|:----------|:------|:--------|
| Every 5min | Health endpoint | `curl localhost:18080/health` |
| Every 1min | Socket Mode | `docker logs --tail=10 \| grep Socket` |
| Hourly | Error count | `docker logs --since=1h \| grep -c error` |
| Daily | Disk usage | `docker exec ... du -sh ~/.hotplex` |
| Daily | Restart count | `docker inspect ... RestartCount` |

### Alert Thresholds

| Metric | Warning | Critical |
|:-------|:--------|:---------|
| Memory | >70% | >90% |
| CPU (sustained) | >60% for 5min | >80% for 5min |
| Error rate | >10/hour | >50/hour |
| Restarts | >1/hour | >3/hour |
| Disk | >70% | >90% |
