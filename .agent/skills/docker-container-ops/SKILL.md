---
name: Docker Container Operations
description: Use this skill when the user asks to "manage Docker containers", "restart hotplex", "check container status", "scale hotplex", "stop bot", "start bot", "docker restart", "docker up", "docker down". Provides container lifecycle management for hotplex deployment.
version: 0.2.0
---

# Docker Container Operations

Manage the lifecycle of hotplex containers running in Docker Compose deployment.

## Critical: Working Directory

**All docker compose commands MUST be executed from the compose directory.**

```bash
COMPOSE_DIR="~/hotplex/docker/matrix"
```

**Pattern**: Always prefix docker compose commands with `cd $COMPOSE_DIR &&`:
```bash
cd ~/hotplex/docker/matrix && docker compose ps
```

## Container Discovery

**IMPORTANT**: Do not hardcode container details. Always discover containers dynamically:

```bash
# List all running containers with their ports
cd ~/hotplex/docker/matrix && docker compose ps

# Get container port mappings
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

## Quick Operations

### Check All Container Status

```bash
cd ~/hotplex/docker/matrix && docker compose ps
```

### Start All Containers

```bash
cd ~/hotplex/docker/matrix && docker compose up -d
```

### Stop All Containers

```bash
cd ~/hotplex/docker/matrix && docker compose down
```

### Restart All Containers

```bash
cd ~/hotplex/docker/matrix && docker compose restart
```

## Single Container Operations

### Start a Container

```bash
cd ~/hotplex/docker/matrix && docker compose up -d hotplex-01
```

### Stop a Container

```bash
cd ~/hotplex/docker/matrix && docker compose stop hotplex-01
```

### Restart a Container

```bash
cd ~/hotplex/docker/matrix && docker compose restart hotplex-01
```

### Recreate Container (Reload Env)

**Important**: Use `up -d` instead of `restart` to reload `.env` file changes:

```bash
cd ~/hotplex/docker/matrix && docker compose up -d hotplex-01
```

### View Container Logs

```bash
# Recent logs
cd ~/hotplex/docker/matrix && docker compose logs --tail=100 hotplex-01

# Follow logs
cd ~/hotplex/docker/matrix && docker compose logs -f hotplex-01
```

### View Resource Usage

```bash
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" \
  hotplex-01 hotplex-02 hotplex-03
```

## Multi-Container Operations

### Restart Multiple Containers

```bash
cd ~/hotplex/docker/matrix && docker compose restart hotplex-01 hotplex-02
```

### Recreate Multiple Containers

```bash
cd ~/hotplex/docker/matrix && docker compose up -d hotplex-01 hotplex-02
```

### Check Health of All Containers

```bash
for bot in hotplex-01 hotplex-02 hotplex-03; do
  status=$(docker inspect $bot --format='{{.State.Health.Status}}' 2>/dev/null || echo "not found")
  echo "$bot: $status"
done
```

## Configuration Management

### Environment Files

| File | Purpose |
|:-----|:--------|
| `.env` | Global image selection |
| `.env-01` | Bot 01 credentials |
| `.env-02` | Bot 02 credentials |
| `.env-03` | Bot 03 credentials |

### After Updating .env Files

**Must use `up -d` to reload environment variables**:

```bash
cd ~/hotplex/docker/matrix && docker compose up -d hotplex-01
```

`restart` will NOT reload `.env` file changes!

### Rebuild and Restart

```bash
cd ~/hotplex/docker/matrix && \
docker compose build hotplex-01 && \
docker compose up -d hotplex-01
```

## Adding New Bots

1. Create `.env-NN` file in `docker/matrix/`
2. Add service definition in `docker-compose.yml`
3. Create instance directory: `mkdir -p ~/.hotplex/instances/<BOT_ID>`
4. Start: `docker compose up -d hotplex-NN`

## Important Constraints

- **One instance per bot**: Each bot MUST run as a single container
- **Unique bot_user_id**: Each bot must have a unique `HOTPLEX_SLACK_BOT_USER_ID`
- **Session collision**: Duplicate bot_user_id causes session ID conflicts

> **Warning**: Never use `--scale` to run multiple instances of the same bot. Slack message routing depends on bot_user_id uniqueness.

## Troubleshooting

### Container Won't Start

```bash
# Check logs
cd ~/hotplex/docker/matrix && docker compose logs hotplex-01

# Check container status
docker inspect hotplex-01

# Check if port is in use
lsof -i :18080
```

### Container Health Check Failed

```bash
docker inspect hotplex-01 --format='{{json .State.Health}}' | jq
```

### Network Issues

```bash
docker network ls
docker network inspect hotplex_default
```

## Container Discovery

If user doesn't specify which bot:

```bash
cd ~/hotplex/docker/matrix && docker compose ps
```

Then use the container name (hotplex-01, hotplex-02, or hotplex-03) in subsequent commands.

## Session Management via Admin API

Each bot exposes an Admin API on port 9080 (internal) for session management and diagnostics.

### Quick Status

```bash
# Check runtime status for specific bot
cd ~/hotplex/docker/matrix && \
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\K\d+') && \
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/stats | jq
```

### List Active Sessions

```bash
cd ~/hotplex/docker/matrix && \
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\K\d+') && \
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions | jq
```

### Terminate Hung Session

```bash
cd ~/hotplex/docker/matrix && \
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\K\d+') && \
curl -X DELETE -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/sessions/<session-id>
```

**Note**: Replace `<session-id>` with actual session ID from the list endpoint.

### Health Check

```bash
cd ~/hotplex/docker/matrix && \
admin_port=$(docker port hotplex-01_1 9080 2>/dev/null | grep -oP ':\K\d+') && \
curl -s -H "Authorization: Bearer ${HOTPLEX_ADMIN_TOKEN}" \
  http://localhost:$admin_port/admin/v1/health/detailed | jq
```

**For complete Admin API reference, see**: `hotplex-diagnostics/references/api-endpoints.md`

## Additional Resources

### Reference Files

- `docker/matrix/docker-compose.yml` - Container deployment configuration
- `docker/matrix/common.yml` - Shared container configuration

### Related Skills

- `hotplex-diagnostics` - For log analysis and debugging
- `hotplex-data-mgmt` - For data and session management
