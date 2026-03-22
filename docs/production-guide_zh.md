*查看其他语言: [English](production-guide.md), [简体中文](production-guide_zh.md).*

# 生产环境部署指南

## 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                     负载均衡器 (LB)                          │
│                  (nginx / 云厂商 LB)                         │
└─────────────────────────────────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ HotPlex  │       │ HotPlex  │       │ HotPlex  │
   │  节点 1  │       │  节点 2  │       │  节点 3  │
   └──────────┘       └──────────┘       └──────────┘
         │                  │                  │
         └──────────────────┴──────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ Prometheus│       │  Jaeger  │       │  Loki    │
   │  (指标)   │       │  (追踪)  │       │  (日志)  │
   └──────────┘       └──────────┘       └──────────┘
                            │
                            ▼
         ┌──────────────────┴──────────────────┐
         ▼                                     ▼
   ┌──────────┐                         ┌──────────┐
   │  Slack   │                         │  飞书    │
   │  告警    │                         │  告警    │
   └──────────┘                         └──────────┘
```

## 扩容建议

| 并发用户数 | 实例数量 | 单实例 CPU | 单实例内存 |
| ---------- | -------- | ---------- | ---------- |
| 1-100      | 1        | 0.5 核     | 512MB      |
| 100-500    | 2-3      | 1 核       | 1GB        |
| 500-2000   | 5-10     | 2 核       | 2GB        |
| 2000+      | 10+      | 2-4 核     | 2-4GB      |

## 配置说明

### 统一配置系统 (v0.33.0+)

HotPlex v0.33.0 引入了统一配置系统，具备：
- **实例隔离**: 每个机器人独立的配置目录
- **配置继承**: 基础模板配合 `inherits` 实现多环境配置
- **管理机器人支持**: 独立的管理机器人配置位于 `configs/admin/`

#### 配置目录结构

```
configs/
├── base/                    # SSOT 基础配置模板
│   ├── server.yaml         # 核心服务器配置
│   ├── slack.yaml          # Slack 适配器配置
│   ├── feishu.yaml        # 飞书适配器配置
│   └── slack_capabilities.yaml
├── admin/                   # 管理机器人配置
│   ├── server.yaml         # 继承自 base/server.yaml
│   └── slack.yaml
└── instances/              # 实例特定覆盖配置
    └── my-bot/
        ├── server.yaml     # 继承自 ../../base/server.yaml
        └── slack.yaml      # 继承自 ../../base/slack.yaml
```

#### 基础配置模板

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

#### 配置继承

使用 `inherits` 扩展基础配置：

```yaml
# configs/admin/server.yaml
inherits: ./base/server.yaml

server:
  log_level: debug
```

```yaml
# configs/instances/my-bot/slack.yaml
inherits: ../../base/slack.yaml

# 仅覆盖需要的内容
system_prompt: |
  你的自定义系统提示词...
```

### 多实例部署 (Docker Matrix)

对于运行多个机器人实例的生产部署，使用 Docker Matrix 模式：

#### 实例隔离结构

每个机器人在独立容器中运行，拥有隔离的配置：

| 实例 | 端口 | Bot ID | 配置路径 |
|:-----|:-----|:-------|:---------|
| hotplex-01 | 18080 | U0AHRCL1KCM | ~/.hotplex/instances/U0AHRCL1KCM/ |
| hotplex-02 | 18081 | U0AJVRH4YF6 | ~/.hotplex/instances/U0AJVRH4YF6/ |
| hotplex-03 | 18082 | U0AL7H8UU75 | ~/.hotplex/instances/U0AL7H8UU75/ |

#### Docker Compose 配置

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

#### 生产配置管理最佳实践

1. **使用环境变量管理敏感信息**
   ```bash
   # .env 文件
   HOTPLEX_SLACK_BOT_USER_ID=UXXXXXXXXXX
   HOTPLEX_SLACK_BOT_TOKEN=xoxb-...
   HOTPLEX_API_KEY=your-api-key
   ```

2. **每个实例使用唯一的 bot_user_id**
   - 每个机器人必须有唯一的 `bot_user_id` 以防止会话 ID 冲突
   - 通过环境变量配置：`${HOTPLEX_SLACK_BOT_USER_ID}`

3. **配置继承策略**
   - 将基础模板保存在 `configs/base/` (SSOT)
   - 在 `configs/instances/<bot-id>/` 创建实例特定覆盖
   - 使用 `inherits` 避免重复

4. **共享与隔离资源**
   - 共享: Claude 配置 (`~/.claude`)、Go 模块缓存
   - 隔离: 会话存储、项目目录、机器人特定配置

### 旧配置 (已废弃)

旧的 `configs/chatapps/` 目录已被废弃。迁移：

| 旧路径 | 新路径 |
|--------|--------|
| `configs/chatapps/slack.yaml` | `configs/base/slack.yaml` |
| `configs/chatapps/feishu.yaml` | `configs/base/feishu.yaml` |
| `configs/server.yaml` | `configs/base/server.yaml` |

### 环境变量

| 变量 | 描述 | 默认值 |
| :--- | :--- | :----- |
| `HOTPLEX_PORT` | HTTP 服务端口 | `8080` |
| `HOTPLEX_API_KEY` | 用于控制平面身份验证的主 API Key | - |
| `HOTPLEX_API_KEYS` | 多个 API Key（逗号分隔，优先于 HOTPLEX_API_KEY） | - |
| `HOTPLEX_LOG_LEVEL` | 日志级别 (debug/info/warn/error) | `info` |
| `HOTPLEX_ALLOWED_ORIGINS` | 允许的跨域来源（逗号分隔） | `localhost` |
| `HOTPLEX_CONFIG_DIR` | 配置目录 | `./configs` |
| `HOTPLEX_METRICS_PATH` | 指标端点路径 | `/metrics` |
| `HOTPLEX_SERVER_CONFIG` | 服务器配置文件路径 | - |
| `HOTPLEX_CHATAPPS_CONFIG_DIR` | ChatApps 配置目录 | - |
| `HOTPLEX_BOT_ID` | 机器人实例标识符（多实例） | - |
| `HOTPLEX_PROJECTS_DIR` | 项目工作目录 | `/tmp/hotplex` |

#### 机器人特定变量（多实例）

| 变量 | 描述 |
| :--- | :--- |
| `HOTPLEX_SLACK_BOT_USER_ID` | Slack 机器人用户 ID（每个实例必须唯一） |
| `HOTPLEX_SLACK_BOT_TOKEN` | Slack Bot Token |
| `HOTPLEX_SLACK_APP_TOKEN` | Slack App Token (Socket Mode) |
| `HOTPLEX_SLACK_PRIMARY_OWNER` | 主所有者的 Slack User ID |

## 监控配置

### Prometheus 配置

```yaml
scrape_configs:
  - job_name: 'hotplex'
    static_configs:
      - targets: ['hotplex:8080']
    metrics_path: /metrics
```

### 核心指标

#### 引擎指标
- `hotplex_engine_sessions_active`: 活跃会话数
- `hotplex_engine_sessions_total`: 创建的会话总数
- `hotplex_engine_executions_total`: 执行总数
- `hotplex_engine_execution_duration_seconds`: 执行耗时

#### ChatApps 指标
- `hotplex_chatapps_messages_received_total`: 接收的消息数
- `hotplex_chatapps_messages_sent_total`: 发送的消息数
- `hotplex_chatapps_processing_duration_seconds`: 消息处理耗时
- `hotplex_chatapps_errors_total`: 处理错误数

#### Provider 指标
- `hotplex_provider_tokens_total`: 消耗的 token 总数
- `hotplex_provider_cost_usd_total`: 总成本（美元）
- `hotplex_provider_tool_invocations_total`: 工具调用次数

### Grafana 仪表盘

关键面板：
- 活动会话数 (Active Sessions)
- 请求延迟 (p50, p95, p99)
- 各平台错误率
- Token 使用量与成本
- 工具调用频率

### 告警规则

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
      summary: 检测到高错误率

  - alert: SessionPoolExhausted
    expr: hotplex_engine_sessions_active > 800
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: 会话池即将耗尽

  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(hotplex_engine_execution_duration_seconds_bucket[5m])) > 60
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: 95 分位延迟超过 60 秒

  - alert: PlatformRateLimit
    expr: rate(hotplex_chatapps_rate_limits_total[5m]) > 0
    for: 1m
    labels:
      severity: warning
    annotations:
      summary: 触发平台速率限制
```

## 安全检查清单

- [x] 在负载均衡器启用 TLS 终止
- [x] 配置网络策略 (Network Policies)
- [x] 启用平台级频率限制
- [x] 启用签名验证 (Slack/飞书)
- [x] 配置资源限额 (Resource Limits)
- [x] 启用审计日志
- [x] 配置 WAF 正则规则
- [x] 设置 AllowedTools 白名单

#### 飞书
- 使用 SHA256 验证签名
- 验证请求时间戳

## 健康检查

### 端点
```
GET /health
```

响应：
```json
{
  "status": "healthy",
  "version": "v0.33.0",
  "uptime": "24h30m",
  "active_sessions": 42
}
```

### 就绪探针
```
GET /ready
```

返回 200 时表示可以接收流量。

## 备份与恢复

### 会话状态

会话是短暂的 (Hot-Multiplexing)，无需备份持久化状态。
对于关键会话，可使用 `internal/persistence/` 中的会话持久化标记。

### 配置信息

```bash
# 备份配置
kubectl get configmap hotplex-config -o yaml > hotplex-config-backup.yaml

# 恢复配置
kubectl apply -f hotplex-config-backup.yaml
```

## 故障分析排查

### 内存占用过高

```bash
# 检查堆内存分析
kubectl exec -it hotplex-xxx -- curl localhost:8080/debug/pprof/heap

# 检查活跃会话数
curl http://hotplex:8080/metrics | grep hotplex_engine_sessions_active
```

### 请求响应变慢

在 Jaeger 中检查追踪，找出瓶颈所在的 Spans。

### 会话泄漏

```bash
# 监控活跃会话
curl http://hotplex:8080/metrics | grep hotplex_engine_sessions_active

# 检查僵尸进程
ps aux | grep -E "(claude-code|opencode)" | grep defunct
```

#### 飞书
- 验证 AppID 和 AppSecret
- 检查回调 URL 可访问性
- 验证签名配置

## Docker 部署

### 单实例

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

### 多实例 (Matrix 模式)

对于需要运行多个机器人实例的生产部署，使用 Docker Matrix 模式：

```yaml
# docker-compose.yml
version: '3.8'
services:
  # 主机器人
  hotplex-01:
    image: hotplex:latest
    container_name: hotplex-01
    ports:
      - "127.0.0.1:18080:8080"
    env_file: [.env-01]
    volumes:
      - ~/.hotplex/instances/U0AHRCL1KCM:/home/hotplex/.hotplex:rw
      # Claude 配置文件 (只读)
      - ${HOME}/.claude/settings.json:/home/hotplex/.claude/settings.json:ro
      - ${HOME}/.claude/skills:/home/hotplex/.claude/skills:ro
      # Per-instance Claude 状态 (named volume)
      - hotplex-matrix-claude-01:/home/hotplex/.claude:rw
      # Per-instance Go build cache (named volume)
      - hotplex-matrix-go-build-01:/home/hotplex/.cache/go-build:rw
    environment:
      HOTPLEX_BOT_ID: U0AHRCL1KCM
      HOTPLEX_SERVER_CONFIG: /home/hotplex/.hotplex/configs/server.yaml
      HOTPLEX_CHATAPPS_CONFIG_DIR: /home/hotplex/.hotplex/configs

  # 从机器人
  hotplex-02:
    image: hotplex:latest
    container_name: hotplex-02
    ports:
      - "127.0.0.1:18081:8080"
    env_file: [.env-02]
    volumes:
      - ~/.hotplex/instances/U0AJVRH4YF6:/home/hotplex/.hotplex:rw
      # Claude 配置文件 (只读)
      - ${HOME}/.claude/settings.json:/home/hotplex/.claude/settings.json:ro
      - ${HOME}/.claude/skills:/home/hotplex/.claude/skills:ro
      # Per-instance Claude 状态 (named volume)
      - hotplex-matrix-claude-02:/home/hotplex/.claude:rw
      # Per-instance Go build cache (named volume)
      - hotplex-matrix-go-build-02:/home/hotplex/.cache/go-build:rw
    environment:
      HOTPLEX_BOT_ID: U0AJVRH4YF6
      HOTPLEX_SERVER_CONFIG: /home/hotplex/.hotplex/configs/server.yaml
      HOTPLEX_CHATAPPS_CONFIG_DIR: /home/hotplex/.hotplex/configs
```

#### Matrix 部署命令

```bash
# 启动所有实例
cd ~/hotplex/docker/matrix && docker compose up -d

# 重启特定实例
cd ~/hotplex/docker/matrix && docker compose restart hotplex-01

# 查看日志
cd ~/hotplex/docker/matrix && docker compose logs -f hotplex-01

# 查看状态
cd ~/hotplex/docker/matrix && docker compose ps
```

## Kubernetes 部署

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

*更多详情请参阅 [架构文档](architecture_zh.md) 和 [SDK 指南](sdk-guide_zh.md)。*
