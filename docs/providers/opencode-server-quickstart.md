# OpenCode Server 集成指南

## 📋 概述

OpenCode Server 是 HotPlex 的一种 provider 类型，通过 HTTP/SSE 连接到运行中的 `opencode serve` 实例，而不是启动 CLI 子进程。

**优势**：
- ⚡ **更快启动** - 无需 CLI spin-up 开销
- 🔄 **资源共享** - 多个 session 共享一个 server 进程
- 🎯 **集中管理** - 统一的 session 和配置管理
- 📊 **可观测性** - HTTP/SSE 流式传输，易于监控

---

## 🚀 快速开始

### 1. 本地开发模式

```bash
# 1. 生成密码（首次）
make opencode-password

# 2. 启动 OpenCode Server
make opencode-start

# 3. 检查状态
make opencode-status

# 4. 查看日志
make opencode-logs

# 5. 同时启动 HotPlex
make opencode-with-hotplex
```

### 2. 配置 HotPlex 使用 OpenCode Server

在 `~/.hotplex/configs/chatapps/slack.yaml` 或 `configs/admin/slack.yaml` 中：

```yaml
provider:
  type: opencode-server  # 使用 Server 模式而非 CLI 模式
  opencode:
    server_url: http://127.0.0.1:4096
    password: ${HOTPLEX_OPEN_CODE_PASSWORD}  # 从 .env 读取
    model: anthropic/claude-sonnet-4-20250514  # 可选
```

在 `.env` 文件中：

```bash
# OpenCode Server 配置
HOTPLEX_OPEN_CODE_PASSWORD=your-password-from-step-1
HOTPLEX_OPEN_CODE_SERVER_URL=http://127.0.0.1:4096
```

### 3. Docker Sidecar 模式

#### 步骤 1：更新 docker-compose.yml

```yaml
services:
  # OpenCode Server sidecar
  opencode:
    image: ${OPEN_CODE_IMAGE:-ghcr.io/anomalyco/opencode:latest}
    container_name: opencode-server
    environment:
      HOTPLEX_OPEN_CODE_PORT: 4096
      HOTPLEX_OPEN_CODE_PASSWORD: ${HOTPLEX_OPEN_CODE_PASSWORD:-}
    command: [ "serve", "--port", "4096", "--password", "${HOTPLEX_OPEN_CODE_PASSWORD:-}" ]
    healthcheck:
      test: [ "CMD-SHELL", "curl -sf http://localhost:4096/global/health | grep -q '\"healthy\":true'" ]
      interval: 10s
      timeout: 5s
      retries: 5


  # HotPlex 服务
  hotplex-01:
    image: ghcr.io/hrygo/hotplex:latest-go
    depends_on:
      opencode-server:
        condition: service_healthy
    environment:
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
      - HOTPLEX_OPEN_CODE_PASSWORD=${HOTPLEX_OPEN_CODE_PASSWORD}
    # ... 其他配置
```

#### 步骤 2：设置密码

```bash
# 生成密码
make opencode-password

# 添加到 .env
echo "HOTPLEX_OPEN_CODE_PASSWORD=$(cat ~/.hotplex/.opencode-password)" >> .env
```

#### 步骤 3：启动服务

```bash
# 启动所有服务（包括 OpenCode sidecar）
make docker-up
```

---

## 🛠️ Makefile 命令参考

### 基础管理命令

| 命令 | 说明 |
|------|------|
| `make opencode-config` | 显示当前配置（端口、日志、密码状态） |
| `make opencode-verify` | 验证依赖（二进制、端口可用性） |
| `make opencode-password` | 生成/更新密码 |
| `make opencode-start` | 启动 OpenCode Server |
| `make opencode-stop` | 停止 OpenCode Server |
| `make opencode-restart` | 重启 OpenCode Server |
| `make opencode-status` | 检查运行状态和健康检查 |
| `make opencode-logs` | 查看实时日志（Ctrl+C 停止） |

### 高级命令

| 命令 | 说明 |
|------|------|
| `make opencode-test` | 运行 Python 验证脚本 |
| `make opencode-logs-truncate` | 轮转日志（保留最近 1000 行） |
| `make opencode-with-hotplex` | 先启动 OpenCode，再启动 HotPlex |
| `make opencode-docker-integrate` | 显示 Docker 集成指南 |

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `OPENCODE_PORT` | 4096 | OpenCode Server 监听端口 |
| `OPENCODE_BINARY` | opencode | OpenCode 二进制文件名 |
| `OPENCODE_LOG_DIR` | ~/.hotplex/logs | 日志目录 |
| `OPENCODE_DEBUG` | false | 启用 debug 模式 |

---

## 🔧 配置详解

### 环境变量配置

在 `.env` 文件中：

```bash
# OpenCode Server 启用（Docker 模式）
HOTPLEX_OPEN_CODE_SERVER_ENABLED=true

# OpenCode Server 端口
HOTPLEX_OPEN_CODE_PORT=4096

# OpenCode Server 密码（Basic Auth）
# 生成命令：make opencode-password
HOTPLEX_OPEN_CODE_PASSWORD=your-secure-password

# OpenCode Server URL
# 本地开发：http://127.0.0.1:4096
# Docker：http://opencode-server:4096
# 远程：http://your-server:4096
HOTPLEX_OPEN_CODE_SERVER_URL=http://127.0.0.1:4096

# Provider 类型（设为 opencode-server）
HOTPLEX_PROVIDER_TYPE=opencode-server

# 模型选择（可选）
HOTPLEX_PROVIDER_MODEL=anthropic/claude-sonnet-4-20250514
```

### YAML 配置文件

在 `configs/chatapps/slack.yaml` 中：

```yaml
provider:
  type: opencode-server
  opencode:
    # 服务器地址（可选，默认 127.0.0.1:4096）
    server_url: ${HOTPLEX_OPEN_CODE_SERVER_URL}

    # 密码（可选，用于 Basic Auth）
    password: ${HOTPLEX_OPEN_CODE_PASSWORD}

    # 端口（可选，仅在未设置 server_url 时使用）
    port: 4096

    # 模型选择（可选）
    # 格式：providerID/modelID
    # 示例：
    #   - anthropic/claude-sonnet-4-20250514
    #   - openai/gpt-4-turbo
    #   - gemini/gemini-2.0-flash
    model: anthropic/claude-sonnet-4-20250514

    # Agent 配置（可选）
    agent: code-assistant
```

---

## 🧪 测试与验证

### 1. 基础验证

```bash
# 启动 OpenCode Server
make opencode-start

# 检查状态
make opencode-status

# 运行测试脚本
make opencode-test
```

### 2. 手动测试

```bash
# 获取密码
PASSWORD=$(cat ~/.hotplex/.opencode-password)

# 测试健康检查
curl -u admin:$PASSWORD http://127.0.0.1:4096/global/health

# 测试 chat endpoint
curl -u admin:$PASSWORD \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello","session_id":"test"}' \
  http://127.0.0.1:4096/v1/chat
```

### 3. SSE 流测试

```bash
# 使用 Python 脚本
python3 scripts/verify/verify_opencode_sse_events.py

# 或使用 curl
curl -N -u admin:$PASSWORD \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -d '{"message":"test","session_id":"test","stream":true}' \
  http://127.0.0.1:4096/v1/chat
```

---

## 🐛 故障排查

### 问题 1：端口被占用

```bash
# 检查端口占用
lsof -i :4096

# 使用不同端口
OPENCODE_PORT=4097 make opencode-start
```

### 问题 2：密码认证失败

```bash
# 检查密码文件
cat ~/.hotplex/.opencode-password

# 重新生成密码
make opencode-password

# 更新 .env
echo "HOTPLEX_OPEN_CODE_PASSWORD=$(cat ~/.hotplex/.opencode-password)" >> .env

# 重启服务
make opencode-restart
```

### 问题 3：日志文件过大

```bash
# 查看日志大小
du -h ~/.hotplex/logs/opencode-server.log

# 轮转日志
make opencode-logs-truncate
```

### 问题 4：Docker 模式连接失败

```bash
# 检查容器状态
docker ps | grep opencode

# 检查网络
docker network inspect hotplex-network

# 查看日志
docker logs hotplex-opencode-server-1

# 验证健康检查
docker inspect hotplex-opencode-server-1 | jq '.[0].State.Health'
```

---

## 📊 性能优化

### 1. 日志管理

```bash
# 设置日志轮转（crontab）
0 0 * * * cd /path/to/hotplex && make opencode-logs-truncate

# 或使用 logrotate
# /etc/logrotate.d/hotplex-opencode
~/.hotplex/logs/opencode-server.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    create 0640 $USER $USER
}
```

### 2. 资源限制

在 Docker Compose 中：

```yaml
opencode-server:
  # ... 其他配置
  deploy:
    resources:
      limits:
        cpus: '2'
        memory: 2G
      reservations:
        cpus: '0.5'
        memory: 512M
```

### 3. 连接池

HotPlex 自动管理到 OpenCode Server 的连接池，无需额外配置。

---

## 🔄 迁移指南

### 从 CLI 模式迁移到 Server 模式

**旧配置（CLI 模式）**：
```yaml
provider:
  type: opencode
  binary_path: /usr/local/bin/opencode
```

**新配置（Server 模式）**：
```yaml
provider:
  type: opencode-server
  opencode:
    server_url: http://127.0.0.1:4096
    password: ${HOTPLEX_OPEN_CODE_PASSWORD}
```

**迁移步骤**：
1. 生成密码：`make opencode-password`
2. 启动 server：`make opencode-start`
3. 更新 YAML 配置
4. 重启 HotPlex：`make restart`

---

## 📚 相关文档

- [OpenCode Provider 规范](./opencode-server-provider-spec.md)
- [OpenCode SSE 事件验证](../../scripts/verify/verify_opencode_sse_events.py)
- [配置示例](../examples/opencode-server-provider.yaml)
- [HotPlex 配置指南](../../docs/superpowers/specs/2026-03-16-unified-config-design.md)

---

## 🆘 获取帮助

- **文档问题**：提交 Issue 到 [GitHub](https://github.com/hrygo/hotplex)
- **功能请求**：在 GitHub Discussions 中讨论
- **Bug 报告**：使用 `make opencode-logs` 收集日志后提交 Issue
