# OpenCode Server Docker Sidecar 集成详解

## 📖 什么是 Sidecar 模式？

**Sidecar 模式**是一种容器化架构模式，将辅助服务（如 OpenCode Server）作为独立容器运行在主应用（HotPlex）旁边，通过共享网络进行通信。

```
┌─────────────────────────────────────────────────────────┐
│                    Docker Compose                        │
│                                                          │
│  ┌──────────────────┐         ┌──────────────────┐      │
│  │  HotPlex 容器     │         │ OpenCode 容器    │      │
│  │                  │         │                  │      │
│  │  ┌────────────┐  │         │  ┌────────────┐  │      │
│  │  │ WebSocket  │  │         │  │ HTTP/SSE   │  │      │
│  │  │ Gateway    │  │         │  │ Server     │  │      │
│  │  └────────────┘  │         │  └────────────┘  │      │
│  │  ┌────────────┐  │  HTTP   │  ┌────────────┐  │      │
│  │  │ Provider   │──┼─────────┼──│ OpenCode   │  │      │
│  │  │ Client     │  │  :4096  │  │ CLI        │  │      │
│  │  └────────────┘  │         │  └────────────┘  │      │
│  └──────────────────┘         └──────────────────┘      │
│         │                              │                 │
│         │                              │                 │
│         └──────────────────────────────┘                 │
│              共享 Docker 网络                             │
└─────────────────────────────────────────────────────────┘
```

**优势**：
- ✅ **资源隔离** - OpenCode Server 独立运行，不影响 HotPlex
- ✅ **独立扩展** - 可以单独扩容 OpenCode Server
- ✅ **故障隔离** - 一个服务崩溃不影响另一个
- ✅ **配置解耦** - 各自独立的配置和环境变量
- ✅ **易于监控** - 独立的日志和指标

---

## 🎯 为什么需要 Sidecar 模式？

### 问题：CLI 模式的局限性

在 CLI 模式下，HotPlex 每次创建 session 都会启动一个新的 `opencode` 子进程：

```
HotPlex (1个进程)
├── Session 1 → opencode process 1
├── Session 2 → opencode process 2
└── Session 3 → opencode process 3
```

**问题**：
- ❌ 每个进程独立，资源浪费
- ❌ 启动开销大（spin-up time）
- ❌ 配置分散，难以管理
- ❌ 日志分散，难以监控

### 解决方案：Server 模式 + Sidecar

在 Server 模式下，所有 session 共享一个 OpenCode Server 进程：

```
HotPlex (1个容器)
└── OpenCode Server (1个容器)
    ├── Session 1
    ├── Session 2
    └── Session 3
```

**优势**：
- ✅ 单一进程，资源高效
- ✅ 快速启动（无 spin-up）
- ✅ 集中管理
- ✅ 统一监控

---

## 🛠️ 配置步骤详解

### 步骤 1：生成密码

```bash
# 生成密码
make opencode-password

# 输出示例：
# ╭─ 🔐 Managing OpenCode Password ────────────────────────────
#   → Generating secure password...
# ✓ Password saved to: /Users/you/.hotplex/.opencode-password
#   → Password (copy this):
# K8j2nF9xM3pL7qR4vT1wY6zA5bC0dE
#
# ⚠ Add to .env:
# HOTPLEX_OPEN_CODE_PASSWORD=K8j2nF9xM3pL7qR4vT1wY6zA5bC0dE
```

**密码存储位置**：
- 本地：`~/.hotplex/.opencode-password`
- 权限：`600`（仅所有者可读写）

### 步骤 2：更新 docker-compose.yml

打开 `docker/matrix/docker-compose.yml`，添加以下配置：

```yaml
version: '3.8'

services:
  # ============================================================================
  # OpenCode Server Sidecar
  # ============================================================================
  opencode-server:
    image: opencode/opencode:latest
    container_name: hotplex-opencode-server

    # 启动命令
    command: >
      serve
      --port 4096
      --password ${HOTPLEX_OPEN_CODE_PASSWORD}

    # 端口映射
    ports:
      - "4096:4096"

    # 网络配置
    networks:
      - hotplex-network

    # 环境变量
    environment:
      - OPEN_CODE_PASSWORD=${HOTPLEX_OPEN_CODE_PASSWORD}
      - LOG_LEVEL=${HOTPLEX_LOG_LEVEL:-info}

    # 健康检查
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4096/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

    # 资源限制
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M

    # 重启策略
    restart: unless-stopped

    # 日志配置
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  # ============================================================================
  # HotPlex 服务（依赖 OpenCode Server）
  # ============================================================================
  hotplex-01:
    image: ghcr.io/hrygo/hotplex:latest-go
    container_name: hotplex-bot-01

    # 依赖配置
    depends_on:
      opencode-server:
        condition: service_healthy  # 等待 OpenCode 健康后启动

    # 网络配置
    networks:
      - hotplex-network

    # 环境变量
    environment:
      # OpenCode Server 连接配置
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
      - HOTPLEX_OPEN_CODE_PASSWORD=${HOTPLEX_OPEN_CODE_PASSWORD}

      # Provider 类型
      - HOTPLEX_PROVIDER_TYPE=opencode-server

      # 其他 HotPlex 配置
      - HOTPLEX_BOT_ID=${HOTPLEX_BOT_ID}
      - HOTPLEX_PORT=${HOTPLEX_PORT}
      - HOTPLEX_API_KEY=${HOTPLEX_API_KEY}

    # 卷挂载
    volumes:
      - ~/.hotplex/instances/${HOTPLEX_BOT_ID}:/home/hotplex/.hotplex

    # 端口映射
    ports:
      - "${HOTPLEX_PORT}:8080"
      - "${HOTPLEX_ADMIN_PORT:-9080}:9080"

    # 重启策略
    restart: unless-stopped

networks:
  hotplex-network:
    driver: bridge
```

### 步骤 3：配置环境变量

在 `docker/matrix/.env-01` 中添加：

```bash
# OpenCode Server 配置
HOTPLEX_OPEN_CODE_PASSWORD=K8j2nF9xM3pL7qR4vT1wY6zA5bC0dE
HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
HOTPLEX_PROVIDER_TYPE=opencode-server

# 其他配置
HOTPLEX_BOT_ID=UXXXXXXXXXX
HOTPLEX_PORT=8080
# ...
```

### 步骤 4：启动服务

```bash
# 启动所有服务
make docker-up

# 查看日志
make docker-logs

# 检查服务状态
docker ps
```

---

## 🔧 配置详解

### 1. 网络配置

**为什么需要共享网络？**

在 Docker Compose 中，服务默认通过服务名（service name）进行通信：

```yaml
# HotPlex 配置
HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
#                                      ↑
#                                   服务名
```

**DNS 解析**：
```
opencode-server → 容器 IP (如 172.18.0.2)
hotplex-01      → 容器 IP (如 172.18.0.3)
```

**网络隔离**：
```
┌─────────────────────────────────────────┐
│         hotplex-network (bridge)        │
│                                          │
│  opencode-server (172.18.0.2)           │
│  hotplex-01 (172.18.0.3)                │
│                                          │
│  ✓ 可以互相通信                          │
│  ✗ 外部无法直接访问（除非端口映射）       │
└─────────────────────────────────────────┘
```

### 2. 依赖管理（depends_on）

**condition: service_healthy 的作用**：

```yaml
depends_on:
  opencode-server:
    condition: service_healthy
```

这确保 HotPlex 在 OpenCode Server **健康后**才启动：

```
时间线：
0s   → opencode-server 启动
0s   → hotplex-01 等待
10s  → opencode-server 健康检查通过
10s  → hotplex-01 开始启动
```

**健康检查流程**：

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:4096/"]
  interval: 30s      # 每 30 秒检查一次
  timeout: 10s       # 10 秒超时
  retries: 3         # 连续 3 次失败才标记为 unhealthy
  start_period: 10s  # 启动后 10 秒才开始检查
```

### 3. 资源限制

**为什么需要资源限制？**

防止 OpenCode Server 占用过多资源，影响其他服务：

```yaml
deploy:
  resources:
    limits:           # 最大资源限制
      cpus: '2'       # 最多 2 个 CPU
      memory: 2G      # 最多 2GB 内存
    reservations:     # 保留资源
      cpus: '0.5'     # 至少 0.5 个 CPU
      memory: 512M    # 至少 512MB 内存
```

**监控资源使用**：

```bash
# 查看容器资源使用
docker stats hotplex-opencode-server

# 输出示例：
# CONTAINER ID   NAME                      CPU %   MEM USAGE / LIMIT
# abc123         hotplex-opencode-server   15.2%   512MiB / 2GiB
```

### 4. 日志配置

**日志轮转**：

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"  # 单个日志文件最大 10MB
    max-file: "3"    # 保留最近 3 个日志文件
```

**日志总大小**：10MB × 3 = 30MB

**查看日志**：

```bash
# 查看实时日志
docker logs -f hotplex-opencode-server

# 查看最近 100 行
docker logs --tail 100 hotplex-opencode-server

# 查看带时间戳的日志
docker logs -t hotplex-opencode-server
```

---

## 🚀 高级配置

### 1. 多 OpenCode Server 实例（负载均衡）

如果有多个 bot 实例，可以共享一个 OpenCode Server：

```yaml
services:
  opencode-server:
    # ... 配置同上

  hotplex-01:
    depends_on:
      - opencode-server
    environment:
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096

  hotplex-02:
    depends_on:
      - opencode-server
    environment:
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
```

### 2. OpenCode Server 集群（高可用）

```yaml
services:
  opencode-server-1:
    image: opencode/opencode:latest
    command: serve --port 4096
    networks:
      - hotplex-network

  opencode-server-2:
    image: opencode/opencode:latest
    command: serve --port 4096
    networks:
      - hotplex-network

  # 负载均衡器
  nginx-lb:
    image: nginx:alpine
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    ports:
      - "4096:4096"
    networks:
      - hotplex-network
    depends_on:
      - opencode-server-1
      - opencode-server-2
```

### 3. 持久化 OpenCode 配置

```yaml
services:
  opencode-server:
    volumes:
      - opencode-config:/root/.opencode
      - opencode-sessions:/root/.opencode/sessions

volumes:
  opencode-config:
  opencode-sessions:
```

### 4. 自定义 OpenCode 镜像

创建 `docker/Dockerfile.opencode`:

```dockerfile
FROM opencode/opencode:latest

# 安装额外工具
RUN apt-get update && apt-get install -y \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

# 复制配置
COPY opencode-config.yaml /root/.opencode/config.yaml

# 设置环境变量
ENV LOG_LEVEL=info
```

使用自定义镜像：

```yaml
services:
  opencode-server:
    build:
      context: .
      dockerfile: docker/Dockerfile.opencode
```

---

## 🧪 测试与验证

### 1. 测试网络连通性

```bash
# 从 HotPlex 容器测试连接
docker exec hotplex-bot-01 curl -f http://opencode-server:4096/

# 从 OpenCode 容器测试
docker exec hotplex-opencode-server curl -f http://localhost:4096/
```

### 2. 测试健康检查

```bash
# 查看健康状态
docker inspect hotplex-opencode-server | jq '.[0].State.Health'

# 输出示例：
# {
#   "Status": "healthy",
#   "FailingStreak": 0,
#   "Log": [
#     {
#       "Start": "2026-03-27T10:00:00Z",
#       "End": "2026-03-27T10:00:01Z",
#       "ExitCode": 0,
#       "Output": ""
#     }
#   ]
# }
```

### 3. 测试密码认证

```bash
# 获取密码
PASSWORD=$(grep HOTPLEX_OPEN_CODE_PASSWORD docker/matrix/.env-01 | cut -d= -f2)

# 测试认证
curl -u admin:$PASSWORD http://localhost:4096/
```

### 4. 测试服务依赖

```bash
# 停止 OpenCode Server
docker stop hotplex-opencode-server

# HotPlex 应该自动重启（因为 depends_on）
docker ps | grep hotplex-bot-01

# 启动 OpenCode Server
docker start hotplex-opencode-server

# 等待健康检查通过
sleep 10

# 验证 HotPlex 连接
docker logs hotplex-bot-01 | grep -i opencode
```

---

## 🐛 故障排查

### 问题 1：连接被拒绝

**症状**：
```
HotPlex: connection refused: http://opencode-server:4096
```

**排查步骤**：

```bash
# 1. 检查 OpenCode Server 是否运行
docker ps | grep opencode-server

# 2. 检查网络
docker network inspect hotplex-network

# 3. 检查 DNS 解析
docker exec hotplex-bot-01 nslookup opencode-server

# 4. 检查端口
docker exec hotplex-opencode-server netstat -tlnp | grep 4096
```

**解决方案**：
- 确保 OpenCode Server 正在运行
- 确保两个容器在同一个网络
- 检查防火墙规则

### 问题 2：认证失败

**症状**：
```
HotPlex: 401 Unauthorized
```

**排查步骤**：

```bash
# 1. 检查密码是否正确
docker exec hotplex-bot-01 env | grep HOTPLEX_OPEN_CODE_PASSWORD
docker exec hotplex-opencode-server env | grep OPEN_CODE_PASSWORD

# 2. 测试密码
PASSWORD=$(docker exec hotplex-bot-01 env | grep HOTPLEX_OPEN_CODE_PASSWORD | cut -d= -f2)
curl -u admin:$PASSWORD http://localhost:4096/
```

**解决方案**：
- 确保 `.env` 文件中密码一致
- 重新生成密码并更新所有配置

### 问题 3：健康检查失败

**症状**：
```
HotPlex: waiting for opencode-server to be healthy
```

**排查步骤**：

```bash
# 1. 查看 OpenCode Server 日志
docker logs hotplex-opencode-server

# 2. 手动测试健康检查
docker exec hotplex-opencode-server curl -f http://localhost:4096/

# 3. 查看健康状态
docker inspect hotplex-opencode-server | jq '.[0].State.Health'
```

**解决方案**：
- 增加 `start_period`
- 检查 OpenCode Server 启动日志
- 确保 curl 已安装

---

## 📊 性能优化

### 1. 连接池配置

HotPlex 自动管理连接池，无需手动配置。

### 2. 资源调优

根据负载调整资源限制：

```yaml
# 轻负载
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 1G

# 重负载
deploy:
  resources:
    limits:
      cpus: '4'
      memory: 4G
```

### 3. 日志级别

```yaml
environment:
  - LOG_LEVEL=warn  # 减少日志输出
```

---

## 📚 相关文档

- [OpenCode Server 快速入门](./opencode-server-quickstart.md)
- [Docker Compose 官方文档](https://docs.docker.com/compose/)
- [Sidecar 模式详解](https://docs.microsoft.com/azure/architecture/patterns/sidecar)

---

## 🎉 总结

**Sidecar 模式核心要点**：

1. **独立容器** - OpenCode Server 作为独立服务运行
2. **共享网络** - 通过服务名进行通信
3. **健康检查** - 确保依赖服务就绪
4. **资源隔离** - 独立的资源限制和监控
5. **配置解耦** - 各自独立的环境变量和配置

**最佳实践**：

- ✅ 使用健康检查确保启动顺序
- ✅ 设置合理的资源限制
- ✅ 配置日志轮转
- ✅ 使用环境变量管理密码
- ✅ 定期监控资源使用

**下一步**：

```bash
# 查看集成指南
make opencode-docker-integrate

# 启动服务
make docker-up

# 查看日志
make docker-logs
```
