---
name: Docker Container Operations
description: This skill should be used when the user asks to "manage Docker containers", "restart hotplex container", "check container status", "scale hotplex", "stop bot", "start bot". Provides container lifecycle management for hotplex deployment.
version: 0.1.0
---

# Docker Container Operations

Manage the lifecycle of hotplex containers running in Docker Compose deployment.

## Overview

This skill provides administrative operations for hotplex Docker containers. It interacts with the docker CLI and docker-compose to manage container lifecycle, scaling, and resource monitoring.

## Prerequisites

- Docker CLI installed on the host
- docker-compose or docker compose plugin available
- Permission to run docker commands
- HotPlex deployed via docker-compose

## Container Operations

### List All Containers

List all hotplex containers with their status:

```bash
docker compose ps
```

### Start a Container

Start a specific hotplex service:

```bash
docker compose up -d hotplex-01
docker compose up -d hotplex-02
```

### Stop a Container

Stop a specific hotplex service:

```bash
docker compose stop hotplex-01
```

### Restart a Container

Restart a specific hotplex service:

```bash
docker compose restart hotplex-01
```

### Remove a Container

Remove a stopped container:

```bash
docker compose rm hotplex-01
```

### View Container Logs

View real-time logs from a container:

```bash
docker compose logs -f hotplex-01
docker compose logs --tail=100 hotplex-02
```

### View Resource Usage

Check CPU and memory usage:

```bash
docker stats $(docker compose ps -q)
docker stats hotplex hotplex-02
```

## Multi-Bot Architecture

### Adding New Bots
To add a new bot, create a new `.env-NN` file and add service definition in docker-compose.yml.

### Important Constraints
- **One instance per bot**: Each bot MUST run as a single container
- **Unique bot_user_id**: Each bot must have a unique `SLACK_BOT_USER_ID` in its `.env` file
- **Reason**: Session ID = `platform:userID:botUserID:channelID:threadID`, duplicate bot_user_id causes session collision

## Configuration

The skill uses the docker-compose.yml file at `docker/matrix/`. All commands run in that directory.

### Container Naming Convention
- Format: `hotplex-{NN}` (e.g., hotplex-01, hotplex-02)
- Each container represents one bot instance
- **IMPORTANT**: Each bot MUST have only ONE running instance. Multiple instances of the same bot will cause Slack message routing confusion.

### Dynamic Container Discovery
When user mentions a specific bot, use the corresponding container name:
- Ask user which bot they want to manage if not specified
- Use `docker compose ps` to list available containers
- Replace `<BOT>` in commands below with actual container name

### Default Ports (Examples)
- hotplex-01: 18080 (example)
- hotplex-02: 18081 (example)
- Port is configured in docker-compose.yml

### Environment Files
- Format: `.env-{NN}` (e.g., .env-01, .env-02)
- Each bot has its own credentials file

> **Warning**: Never scale a bot to more than 1 instance. Slack消息路由依赖bot_user_id，单一bot多实例会导致消息错乱。

## Troubleshooting

- If container fails to start, check logs: `docker compose logs <BOT>`
- Verify container health: `docker inspect <BOT> --format='{{.State.Health}}'`
- Check container networking: `docker network ls`

> **Important**: Do NOT use `--scale` to run multiple instances of the same bot. This will cause Slack message routing issues.

## Additional Resources

### Reference Files

- **`docker/matrix/docker-compose.yml`** - Container deployment configuration
- **`docker/matrix/.env-01`** - Bot1 credentials
- **`docker/matrix/.env-02`** - Bot2 credentials
- **`docker/matrix/.env-03`** - Bot3 credentials
- **`references/docker-commands.md`** - Complete Docker CLI reference

### Related Skills

- **`hotplex-diagnostics`** - For log analysis and debugging
- **`hotplex-data-mgmt`** - For data and session management
