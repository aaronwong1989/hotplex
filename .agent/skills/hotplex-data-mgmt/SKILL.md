---
name: hotplex-data-mgmt
description: Use this skill when the user asks to "manage data", "clean sessions", "cleanup markers", "view messages", "export data", "delete session", "message database", "session cleanup". Provides data and session management for hotplex persistence layer.
version: 0.2.0
---

# HotPlex Data Management

Manage persistent data including session markers, messages, and temporary files.

## Critical: Working Directory

**All docker compose commands MUST be executed from the compose directory.**

```bash
COMPOSE_DIR="~/hotplex/docker/matrix"
```

**Pattern**: Always prefix docker compose commands with `cd $COMPOSE_DIR &&`:
```bash
cd ~/hotplex/docker/matrix && docker compose ps
```

## Container Reference

| Container | Bot ID | Instance Path |
|:----------|:-------|:--------------|
| hotplex-01 | U0AHRCL1KCM | ~/.hotplex/instances/U0AHRCL1KCM |
| hotplex-02 | U0AJVRH4YF6 | ~/.hotplex/instances/U0AJVRH4YF6 |
| hotplex-03 | U0AL7H8UU75 | ~/.hotplex/instances/U0AL7H8UU75 |

## Session Markers

### List Session Markers

```bash
# Host side (direct access)
ls -la ~/.hotplex/instances/U0AHRCL1KCM/markers/

# Container side
docker exec hotplex-01 ls -la /home/hotplex/.hotplex/markers/
```

### Delete a Session Marker

```bash
# Host side
rm ~/.hotplex/instances/U0AHRCL1KCM/markers/<provider-session-id>

# Container side
docker exec hotplex-01 rm /home/hotplex/.hotplex/markers/<provider-session-id>
```

### Delete All Markers for a Bot

```bash
docker exec hotplex-01 rm -rf /home/hotplex/.hotplex/markers/*
```

### Clean Old Markers (24h+)

```bash
docker exec hotplex-01 find /home/hotplex/.hotplex/markers -type f -mtime +1 -delete
```

## Message Storage

### Check Database Size

```bash
docker exec hotplex-01 du -sh /home/hotplex/.hotplex/*.db
docker exec hotplex-01 ls -lh /home/hotplex/.hotplex/
```

### Query Messages (SQLite)

```bash
# 按会话统计消息数
docker exec hotplex-01 sqlite3 /home/hotplex/.hotplex/chatapp_messages.db \
  "SELECT session_id, COUNT(*) as msg_count FROM messages GROUP BY session_id ORDER BY msg_count DESC LIMIT 10"

# 按日期统计
docker exec hotplex-01 sqlite3 /home/hotplex/.hotplex/chatapp_messages.db \
  "SELECT date(created_at) as date, COUNT(*) as count FROM messages GROUP BY date ORDER BY date DESC LIMIT 7"

# 查看表结构
docker exec hotplex-01 sqlite3 /home/hotplex/.hotplex/chatapp_messages.db ".schema"
```

### Export Messages

```bash
# Export to CSV
docker exec hotplex-01 sqlite3 /home/hotplex/.hotplex/chatapp_messages.db \
  -csv -header "SELECT * FROM messages LIMIT 100" > /tmp/messages_export.csv

# Export to host
docker cp hotplex-01:/home/hotplex/.hotplex/chatapp_messages.db /tmp/messages_backup.db
```

## Session Cleanup

### Force Stop All CLI Processes in Container

```bash
docker exec hotplex-01 pkill -f "claude\|opencode"
```

### Clean Zombie Sessions

```bash
# Find zombie markers (older than 1 hour)
docker exec hotplex-01 find /home/hotplex/.hotplex/markers -type f -mmin +60

# Remove markers older than 24 hours
docker exec hotplex-01 find /home/hotplex/.hotplex/markers -type f -mtime +1 -delete
```

## Temporary Files

### Clean Claude Cache

```bash
docker exec hotplex-01 rm -rf /home/hotplex/.claude/sessions/*
```

### Clean Temp Directories

```bash
docker exec hotplex-01 rm -rf /tmp/hotplex_*
```

## Backup and Restore

### Backup Single Instance

```bash
# Backup from host
tar -czf hotplex-backup-U0AHRCL1KCM-$(date +%Y%m%d).tar.gz \
  -C ~/.hotplex/instances U0AHRCL1KCM

# Backup from container
docker exec hotplex-01 tar -czf /tmp/backup.tar.gz -C /home/hotplex .hotplex
docker cp hotplex-01:/tmp/backup.tar.gz ./hotplex-backup-$(date +%Y%m%d).tar.gz
```

### Restore to Instance

```bash
# Restore to host
tar -xzf hotplex-backup-U0AHRCL1KCM-20240101.tar.gz -C ~/.hotplex/instances/

# Restart container to pick up changes
cd ~/hotplex/docker/matrix && docker compose restart hotplex-01
```

## All-Bot Operations

### Clean All Markers (All Bots)

```bash
for bot in hotplex-01 hotplex-02 hotplex-03; do
  echo "Cleaning $bot..."
  docker exec $bot rm -rf /home/hotplex/.hotplex/markers/*
done
```

### Backup All Instances

```bash
tar -czf hotplex-all-backup-$(date +%Y%m%d).tar.gz \
  -C ~/.hotplex/instances .
```

### Check All Database Sizes

```bash
for bot in hotplex-01 hotplex-02 hotplex-03; do
  echo "=== $bot ==="
  docker exec $bot du -sh /home/hotplex/.hotplex/*.db 2>/dev/null || echo "No database"
done
```

## Troubleshooting

### Disk Full

```bash
df -h ~
docker system df
docker exec hotplex-01 df -h /home/hotplex
```

### Permission Issues

```bash
sudo chown -R $(id -u):$(id -g) ~/.hotplex/instances/U0AHRCL1KCM
```

## Container Discovery

If user doesn't specify which bot:

```bash
cd ~/hotplex/docker/matrix && docker compose ps
```

Then use the container name (hotplex-01, hotplex-02, or hotplex-03) in subsequent commands.

## Additional Resources

### Reference Files

- `internal/persistence/marker.go` - Marker store implementation
- `plugins/storage/` - Message storage backends

### Related Skills

- `docker-container-ops` - For container lifecycle management
- `hotplex-diagnostics` - For monitoring and debugging
