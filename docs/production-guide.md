*Read this in other languages: [English](production-guide.md), [简体中文](production-guide_zh.md).*

# Production Deployment Guide

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Load Balancer                           │
│                  (nginx / cloud LB)                         │
└─────────────────────────────────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ HotPlex  │       │ HotPlex  │       │ HotPlex  │
   │  Node 1  │       │  Node 2  │       │  Node 3  │
   └──────────┘       └──────────┘       └──────────┘
         │                  │                  │
         └──────────────────┴──────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ Prometheus│       │  Jaeger  │       │  Loki    │
   │ (metrics)│       │ (traces) │       │  (logs)  │
   └──────────┘       └──────────┘       └──────────┘
                            │
                            ▼
         ┌──────────────────┴──────────────────┐
         ▼                                     ▼
   ┌──────────┐                         ┌──────────┐
   │  Slack   │                         │  Feishu  │
   │  Alerts  │                         │  Alerts  │
   └──────────┘                         └──────────┘
```

## Scaling Guidelines

| Concurrent Users | Instances | CPU/Instance | Memory/Instance |
| ---------------- | --------- | ------------ | --------------- |
| 1-100            | 1         | 0.5 core     | 512MB           |
| 100-500          | 2-3       | 1 core       | 1GB             |
| 500-2000         | 5-10      | 2 core       | 2GB             |
| 2000+            | 10+       | 2-4 core     | 2-4GB           |

## Configuration

### Unified Configuration System (v0.30.0+)

HotPlex v0.30.0 introduces a unified configuration system with:
- **Instance Isolation**: Per-bot config directories for multi-bot deployments
- **Configuration Inheritance**: Base templates with `inherits` for multi-environment setups
- **Admin Bot Support**: Separate admin configurations in `configs/admin/`

#### Configuration Directory Structure

```
configs/
├── base/                    # SSOT base configuration templates
│   ├── server.yaml         # Core server config
│   ├── slack.yaml          # Slack adapter config
│   ├── feishu.yaml        # Feishu adapter config
│   └── slack_capabilities.yaml
├── admin/                   # Admin bot configurations
│   ├── server.yaml         # Inherits from base/server.yaml
│   └── slack.yaml
└── instances/              # Instance-specific overrides
    └── my-bot/
        ├── server.yaml     # Inherits from ../../base/server.yaml
        └── slack.yaml      # Inherits from ../../base/slack.yaml
```

#### Base Configuration Templates

```yaml
# configs/base/server.yaml
server:
  port: 8080
  log_level: info
engine:
  timeout: 30m
  idle_timeout: 1h
  work_dir: /tmp/hotplex_sandbox
security:
  api_key: "${HOTPLEX_API_KEY}"
```

```yaml
# configs/base/slack.yaml
platform: slack
mode: socket
provider:
  type: claude-code
  default_model: sonnet
engine:
  work_dir: ${HOTPLEX_PROJECTS_DIR}/hotplex
  timeout: 30m
security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

#### Configuration Inheritance

Use `inherits` to extend base configurations:

```yaml
# configs/admin/server.yaml
inherits: ./base/server.yaml

server:
  log_level: debug
```

```yaml
# configs/instances/my-bot/slack.yaml
inherits: ../../base/slack.yaml

# Override only what you need
system_prompt: |
  Your custom system prompt here...
```

### Multi-Instance Deployment (Docker Matrix)

For production deployments running multiple bot instances, use the Docker Matrix pattern:

#### Instance Isolation Structure

Each bot runs in its own container with isolated configuration:

| Instance | Port | Bot ID | Config Path |
|:---------|:-----|:-------|:------------|
| hotplex-01 | 18080 | U0AHRCL1KCM | ~/.hotplex/instances/U0AHRCL1KCM/ |
| hotplex-02 | 18081 | U0AJVRH4YF6 | ~/.hotplex/instances/U0AJVRH4YF6/ |
| hotplex-03 | 18082 | U0AL7H8UU75 | ~/.hotplex/instances/U0AL7H8UU75/ |

#### Docker Compose Configuration

```yaml
# docker-compose.yml (Matrix)
services:
  hotplex-01:
    image: hotplex:latest
    ports: ["127.0.0.1:18080:8080"]
    env_file: [.env-01]
    volumes:
      - ~/.hotplex/instances/U0AHRCL1KCM:/home/hotplex/.hotplex:rw
    environment:
      HOTPLEX_BOT_ID: U0AHRCL1KCM
      HOTPLEX_SERVER_CONFIG: /home/hotplex/.hotplex/configs/server.yaml
      HOTPLEX_CHATAPPS_CONFIG_DIR: /home/hotplex/.hotplex/configs
```

#### Best Practices for Production Config Management

1. **Use Environment Variables for Secrets**
   ```bash
   # .env file
   HOTPLEX_SLACK_BOT_USER_ID=UXXXXXXXXXX
   HOTPLEX_SLACK_BOT_TOKEN=xoxb-...
   HOTPLEX_API_KEY=your-api-key
   ```

2. **Unique bot_user_id per Instance**
   - Each bot MUST have a unique `bot_user_id` to prevent session ID collisions
   - Configure via environment variable: `${HOTPLEX_SLACK_BOT_USER_ID}`

3. **Configuration Inheritance Strategy**
   - Keep base templates in `configs/base/` (SSOT)
   - Create instance-specific overrides in `configs/instances/<bot-id>/`
   - Use `inherits` to avoid duplication

4. **Shared vs Isolated Resources**
   - Shared: Claude configuration (`~/.claude`), Go module cache
   - Isolated: Session storage, project directories, bot-specific configs

### Legacy Configuration (Deprecated)

The old `configs/chatapps/` directory has been deprecated. Migration:

| Old Path | New Path |
|----------|----------|
| `configs/chatapps/slack.yaml` | `configs/base/slack.yaml` |
| `configs/chatapps/feishu.yaml` | `configs/base/feishu.yaml` |
| `configs/server.yaml` | `configs/base/server.yaml` |

### Environment Variables

| Variable | Description | Default |
| :------- | :---------- | :------ |
| `HOTPLEX_PORT` | HTTP server port | `8080` |
| `HOTPLEX_API_KEY` | Primary API Key for control plane authentication | - |
| `HOTPLEX_API_KEYS` | Multiple API Keys (comma-separated, takes precedence) | - |
| `HOTPLEX_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `HOTPLEX_ALLOWED_ORIGINS` | Comma-separated allowed origins for CORS | `localhost` |
| `HOTPLEX_CONFIG_DIR` | Configuration directory | `./configs` |
| `HOTPLEX_METRICS_PATH` | Metrics endpoint path | `/metrics` |
| `HOTPLEX_SERVER_CONFIG` | Server config file path | - |
| `HOTPLEX_CHATAPPS_CONFIG_DIR` | ChatApps config directory | - |
| `HOTPLEX_BOT_ID` | Bot instance identifier (multi-instance) | - |
| `HOTPLEX_PROJECTS_DIR` | Working directory for projects | `/tmp/hotplex` |

#### Bot-Specific Variables (Multi-Instance)

| Variable | Description |
| :------- | :---------- |
| `HOTPLEX_SLACK_BOT_USER_ID` | Slack Bot User ID (must be unique per instance) |
| `HOTPLEX_SLACK_BOT_TOKEN` | Slack Bot Token |
| `HOTPLEX_SLACK_APP_TOKEN` | Slack App Token (Socket Mode) |
| `HOTPLEX_SLACK_PRIMARY_OWNER` | Primary owner's Slack User ID |

## Monitoring

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'hotplex'
    static_configs:
      - targets: ['hotplex:8080']
    metrics_path: /metrics
```

### Key Metrics

#### Engine Metrics
- `hotplex_engine_sessions_active`: Number of active sessions
- `hotplex_engine_sessions_total`: Total sessions created
- `hotplex_engine_executions_total`: Total executions
- `hotplex_engine_execution_duration_seconds`: Execution duration

#### ChatApps Metrics
- `hotplex_chatapps_messages_received_total`: Messages received by platform
- `hotplex_chatapps_messages_sent_total`: Messages sent to platform
- `hotplex_chatapps_processing_duration_seconds`: Message processing time
- `hotplex_chatapps_errors_total`: Processing errors

#### Provider Metrics
- `hotplex_provider_tokens_total`: Total tokens used
- `hotplex_provider_cost_usd_total`: Total cost in USD
- `hotplex_provider_tool_invocations_total`: Tool invocation count

### Grafana Dashboard

Key panels:
- Active Sessions
- Request Latency (p50, p95, p99)
- Error Rate by Platform
- Token Usage & Cost
- Tool Invocation Rate

### Alerting Rules

```yaml
groups:
- name: hotplex
  rules:
  - alert: HighErrorRate
    expr: rate(hotplex_engine_errors_total[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: High error rate detected

  - alert: SessionPoolExhausted
    expr: hotplex_engine_sessions_active > 800
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: Session pool nearly exhausted

  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(hotplex_engine_execution_duration_seconds_bucket[5m])) > 60
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: 95th percentile latency exceeds 60s

  - alert: PlatformRateLimit
    expr: rate(hotplex_chatapps_rate_limits_total[5m]) > 0
    for: 1m
    labels:
      severity: warning
    annotations:
      summary: Platform rate limit triggered
```

## Security Checklist

- [x] Enable TLS termination at LB
- [x] Configure network policies
- [x] Enable rate limiting per platform
- [x] Enable signature verification (Slack/Feishu)
- [x] Set resource limits
- [x] Enable audit logging
- [x] Configure WAF patterns
- [x] Set AllowedTools whitelist

#### Feishu
- Verify signature with SHA256
- Validate request timestamp

## Health Checks

### Endpoint
```
GET /health
```

Response:
```json
{
  "status": "healthy",
  "version": "v0.30.0",
  "uptime": "24h30m",
  "active_sessions": 42
}
```

### Readiness Probe
```
GET /ready
```

Returns 200 when ready to accept traffic.

## Backup & Recovery

### Session State

Sessions are ephemeral (Hot-Multiplexing). No persistent state to backup.
For critical sessions, use session persistence markers in `internal/persistence/`.

### Configuration

```bash
# Backup configs
kubectl get configmap hotplex-config -o yaml > hotplex-config-backup.yaml

# Restore configs
kubectl apply -f hotplex-config-backup.yaml
```

## Troubleshooting

### High Memory Usage

```bash
# Check heap profile
kubectl exec -it hotplex-xxx -- curl localhost:8080/debug/pprof/heap

# Check active sessions
curl http://hotplex:8080/metrics | grep hotplex_engine_sessions_active
```

### Slow Requests

Check traces in Jaeger for bottleneck spans.

### Session Leaks

```bash
# Monitor active sessions
curl http://hotplex:8080/metrics | grep hotplex_engine_sessions_active

# Check for zombie processes
ps aux | grep -E "(claude-code|opencode)" | grep defunct
```

#### Feishu
- Validate AppID and AppSecret
- Check callback URL accessibility
- Verify signature configuration

## Docker Deployment

### Single Instance

```yaml
# docker-compose.yaml
version: '3.8'
services:
  hotplex:
    image: hotplex:latest
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs
      - /tmp/hotplex:/tmp/hotplex
    environment:
      - HOTPLEX_LOG_LEVEL=info
      - HOTPLEX_CONFIG_DIR=/app/configs
    restart: unless-stopped
    resources:
      limits:
        cpu: "2"
        memory: 2Gi
```

### Multi-Instance (Matrix Pattern)

For production deployments with multiple bot instances, use the Docker Matrix pattern:

```yaml
# docker-compose.yml
version: '3.8'
services:
  # Primary Bot
  hotplex-01:
    image: hotplex:latest
    container_name: hotplex-01
    ports:
      - "127.0.0.1:18080:8080"
    env_file: [.env-01]
    volumes:
      - ~/.hotplex/instances/U0AHRCL1KCM:/home/hotplex/.hotplex:rw
      - ~/.claude:/home/hotplex/.claude_seed:ro
    environment:
      HOTPLEX_BOT_ID: U0AHRCL1KCM
      HOTPLEX_SERVER_CONFIG: /home/hotplex/.hotplex/configs/server.yaml
      HOTPLEX_CHATAPPS_CONFIG_DIR: /home/hotplex/.hotplex/configs

  # Secondary Bot
  hotplex-02:
    image: hotplex:latest
    container_name: hotplex-02
    ports:
      - "127.0.0.1:18081:8080"
    env_file: [.env-02]
    volumes:
      - ~/.hotplex/instances/U0AJVRH4YF6:/home/hotplex/.hotplex:rw
      - ~/.claude:/home/hotplex/.claude_seed:ro
    environment:
      HOTPLEX_BOT_ID: U0AJVRH4YF6
      HOTPLEX_SERVER_CONFIG: /home/hotplex/.hotplex/configs/server.yaml
      HOTPLEX_CHATAPPS_CONFIG_DIR: /home/hotplex/.hotplex/configs
```

#### Matrix Deployment Commands

```bash
# Start all instances
cd ~/hotplex/docker/matrix && docker compose up -d

# Restart specific instance
cd ~/hotplex/docker/matrix && docker compose restart hotplex-01

# View logs
cd ~/hotplex/docker/matrix && docker compose logs -f hotplex-01

# Check status
cd ~/hotplex/docker/matrix && docker compose ps
```

## Kubernetes Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hotplex
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hotplex
  template:
    metadata:
      labels:
        app: hotplex
    spec:
      containers:
      - name: hotplex
        image: hotplex:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: "1"
            memory: 1Gi
          limits:
            cpu: "2"
            memory: 2Gi
        env:
        - name: HOTPLEX_LOG_LEVEL
          value: "info"
        volumeMounts:
        - name: config
          mountPath: /app/configs
      volumes:
      - name: config
        configMap:
          name: hotplex-config
```

---

*For more details, see [Architecture Documentation](architecture.md) and [SDK Guide](sdk-guide.md).*
