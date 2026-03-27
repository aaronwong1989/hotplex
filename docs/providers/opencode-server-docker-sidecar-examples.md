# OpenCode Server Docker Sidecar - 实战示例

## 📦 完整配置示例

### 最小化配置（5 分钟上手）

创建 `docker/matrix/docker-compose.yml`:

```yaml
version: '3.8'

services:
  # ============================================================================
  # OpenCode Server Sidecar (洗碗工)
  # ============================================================================
  opencode-server:
    image: ghcr.io/anomalyco/opencode:latest
    container_name: opencode-server
    command: >
      serve
      --port 4096
      --password ${HOTPLEX_OPEN_CODE_PASSWORD}

    # 仅暴露给其他容器，不暴露到宿主机
    expose:
      - "4096"

    networks:
      - hotplex-network

    healthcheck:
      test: ["CMD-SHELL", "curl -sf http://localhost:4096/global/health | grep -q '\"healthy\":true'"]
      interval: 10s
      timeout: 5s
      retries: 5

  # ============================================================================
  # HotPlex 服务 (主厨)
  # ============================================================================
  hotplex-01:
    image: ghcr.io/hrygo/hotplex:latest-go
    container_name: hotplex-bot-01

    # 等待 OpenCode Server 健康后再启动
    depends_on:
      opencode-server:
        condition: service_healthy

    networks:
      - hotplex-network

    environment:
      # 告诉 HotPlex 去哪里找 OpenCode Server
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
      - HOTPLEX_OPEN_CODE_PASSWORD=${HOTPLEX_OPEN_CODE_PASSWORD}
      - HOTPLEX_PROVIDER_TYPE=opencode-server

      # 其他 HotPlex 配置
      - HOTPLEX_BOT_ID=${HOTPLEX_BOT_ID}
      - HOTPLEX_PORT=8080
      - HOTPLEX_API_KEY=${HOTPLEX_API_KEY}

    ports:
      - "${HOTPLEX_PORT}:8080"

    volumes:
      - ~/.hotplex/instances/${HOTPLEX_BOT_ID}:/home/hotplex/.hotplex

networks:
  hotplex-network:
    driver: bridge
```

### 环境变量配置

创建 `docker/matrix/.env-01`:

```bash
# OpenCode Server 配置
HOTPLEX_OPEN_CODE_PASSWORD=your-password-here

# HotPlex 配置
HOTPLEX_BOT_ID=UXXXXXXXXXX
HOTPLEX_PORT=8080
HOTPLEX_API_KEY=your-api-key
```

### 启动命令

```bash
# 1. 生成密码
make opencode-password

# 2. 复制密码到 .env-01
# 编辑 docker/matrix/.env-01，设置 HOTPLEX_OPEN_CODE_PASSWORD

# 3. 启动服务
cd docker/matrix
docker compose up -d

# 4. 查看日志
docker compose logs -f

# 5. 检查状态
docker compose ps
```

---

## 🔍 配置详解（逐行解释）

### 1. 服务名称解析

```yaml
opencode-server:           # ← 这是服务名，用作 DNS 主机名
  container_name: opencode-server
```

**DNS 解析**：
```
HotPlex 容器内：
  ping opencode-server
  → 解析到 OpenCode 容器的 IP (如 172.18.0.2)
```

### 2. 端口配置差异

```yaml
opencode-server:
  expose:
    - "4096"              # 仅容器间访问，不暴露到宿主机

  # vs

  ports:
    - "4096:4096"         # 暴露到宿主机，可从外部访问
```

**最佳实践**：
- ✅ 使用 `expose` - 更安全，仅内部访问
- ⚠️ 使用 `ports` - 调试时才需要

### 3. 健康检查

```yaml
healthcheck:
  test: ["CMD-SHELL", "curl -sf http://localhost:4096/global/health | grep -q '\"healthy\":true'"]
  interval: 10s      # 每 10 秒检查一次
  timeout: 5s        # 5 秒超时
  retries: 5         # 连续失败 5 次才标记为 unhealthy
```

**启动流程**：
```
0s   → opencode-server 启动
0s   → hotplex-01 等待（因为 depends_on: condition: service_healthy）
5s   → 健康检查第 1 次（可能失败，OpenCode 还在启动）
10s  → 健康检查第 2 次（可能失败）
15s  → 健康检查第 3 次（成功！标记为 healthy）
15s  → hotplex-01 开始启动
```

### 4. 依赖管理

```yaml
hotplex-01:
  depends_on:
    opencode-server:
      condition: service_healthy  # ← 关键！等待健康检查通过
```

**对比**：

```yaml
# ❌ 错误：只等待容器启动，不等待服务就绪
depends_on:
  - opencode-server

# ✅ 正确：等待服务健康后才启动
depends_on:
  opencode-server:
    condition: service_healthy
```

---

## 🧪 测试示例

### 测试 1：验证网络连通性

```bash
# 从 HotPlex 容器内测试连接
docker exec hotplex-bot-01 curl http://opencode-server:4096/global/health

# 预期输出：
# {"healthy":true}
```

### 测试 2：验证密码认证

```bash
# 获取密码
PASSWORD=$(grep HOTPLEX_OPEN_CODE_PASSWORD docker/matrix/.env-01 | cut -d= -f2)

# 测试认证
docker exec hotplex-bot-01 \
  curl -u admin:$PASSWORD \
  http://opencode-server:4096/global/health

# 预期输出：
# {"healthy":true}
```

### 测试 3：模拟故障恢复

```bash
# 1. 停止 OpenCode Server
docker stop opencode-server

# 2. 观察 HotPlex 日志
docker logs -f hotplex-bot-01
# 应该看到连接失败的错误

# 3. 重启 OpenCode Server
docker start opencode-server

# 4. 等待健康检查
sleep 10

# 5. 验证 HotPlex 恢复
docker logs hotplex-bot-01 | tail -20
# 应该看到重新连接成功的日志
```

---

## 🎓 进阶示例

### 示例 1：多 HotPlex 共享一个 OpenCode Server

```yaml
services:
  opencode-server:
    image: ghcr.io/anomalyco/opencode:latest
    command: serve --port 4096 --password ${HOTPLEX_OPEN_CODE_PASSWORD}
    networks:
      - hotplex-network

  hotplex-01:
    image: ghcr.io/hrygo/hotplex:latest-go
    depends_on:
      opencode-server:
        condition: service_healthy
    environment:
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
    networks:
      - hotplex-network

  hotplex-02:
    image: ghcr.io/hrygo/hotplex:latest-go
    depends_on:
      opencode-server:
        condition: service_healthy
    environment:
      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
    networks:
      - hotplex-network
```

**架构图**：
```
                 opencode-server (1个)
                        ↓
        ┌───────────────┼───────────────┐
        ↓               ↓               ↓
   hotplex-01      hotplex-02      hotplex-03
```

### 示例 2：添加资源限制

```yaml
opencode-server:
  # ... 其他配置
  deploy:
    resources:
      limits:
        cpus: '2'
        memory: 2G
```

### 示例 3：持久化配置

```yaml
opencode-server:
  # ... 其他配置
  volumes:
    - opencode-data:/root/.opencode

volumes:
  opencode-data:
```

---

## 📊 对比：CLI 模式 vs Server 模式

### CLI 模式（旧）

```yaml
hotplex-01:
  image: ghcr.io/hrygo/hotplex:latest-go
  environment:
    - HOTPLEX_PROVIDER_TYPE=opencode
    - HOTPLEX_PROVIDER_BINARY=/usr/local/bin/opencode
  volumes:
    - /usr/local/bin/opencode:/usr/local/bin/opencode
```

**问题**：
- ❌ 每个 session 启动一个 opencode 进程
- ❌ 资源占用高
- ❌ 启动慢

### Server 模式（新）

```yaml
opencode-server:
  image: ghcr.io/anomalyco/opencode:latest
  command: serve --port 4096

hotplex-01:
  image: ghcr.io/hrygo/hotplex:latest-go
  depends_on:
    opencode-server:
      condition: service_healthy
  environment:
    - HOTPLEX_PROVIDER_TYPE=opencode-server
    - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096
```

**优势**：
- ✅ 所有 session 共享一个 opencode 进程
- ✅ 资源占用低
- ✅ 启动快

---

## 🎯 快速检查清单

启动前检查：

- [ ] 已生成密码：`make opencode-password`
- [ ] 已更新 `.env-01` 中的 `HOTPLEX_OPEN_CODE_PASSWORD`
- [ ] 已更新 `docker-compose.yml` 添加 `opencode-server` 服务
- [ ] 已在 `hotplex-01` 中添加 `depends_on` 配置
- [ ] 已在 `hotplex-01` 中设置 `HOTPLEX_OPEN_CODE_SERVER_URL`

启动后验证：

```bash
# 1. 检查容器状态
docker ps | grep -E 'opencode|hotplex'

# 2. 检查网络
docker network inspect hotplex-network

# 3. 测试连通性
docker exec hotplex-bot-01 curl http://opencode-server:4096/global/health

# 4. 查看日志
docker compose logs -f
```

---

## 🆘 常见问题

### Q1: 为什么要用 Sidecar 模式？

**A**: 资源隔离 + 独立管理。就像餐厅雇佣专业洗碗工，比主厨自己洗碗更高效。

### Q2: 服务名必须叫 `opencode-server` 吗？

**A**: 不必须，但建议保持一致。服务名就是 DNS 主机名，HotPlex 通过这个名字找到 OpenCode Server。

### Q3: 密码必须配置吗？

**A**: 不必须，但强烈推荐。没有密码，任何人都能访问你的 OpenCode Server。

### Q4: 可以在宿主机访问 OpenCode Server 吗？

**A**: 可以，将 `expose: - "4096"` 改为 `ports: - "4096:4096"`。

### Q5: 多个 HotPlex 会冲突吗？

**A**: 不会。OpenCode Server 通过 session_id 隔离不同 HotPlex 实例的会话。

---

## 📚 下一步

1. **阅读详细文档**: `docs/providers/opencode-server-docker-sidecar-deep-dive.md`
2. **测试配置**: `make docker-up`
3. **查看日志**: `make docker-logs`
4. **监控资源**: `docker stats`

---

**核心要点**：

- 🎯 Sidecar = 独立容器 + 共享网络
- 🔗 服务名 = DNS 主机名
- 💪 健康检查 = 确保依赖就绪
- 🔒 密码认证 = 安全保障
- 📦 资源隔离 = 独立管理
