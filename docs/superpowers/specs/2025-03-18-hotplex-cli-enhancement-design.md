# HotPlex CLI 增强设计文档

**Date**: 2025-03-18
**Status**: Implemented
**Version**: 1.0

---

## 1. 概述

为 hotplexd 添加命令行管理能力，使其具备会话管理、诊断检查、配置验证等功能。

### 目标

- 提供统一的 CLI 界面管理 hotplexd daemon
- 支持远程管理（HTTP API）
- 提供诊断工具辅助运维

---

## 2. CLI 命令结构

采用 Cobra 框架实现子命令模式。

### 2.1 命令列表

```bash
# 启动 daemon（保持现有行为）
hotplexd start [--config=<path>] [--env-file=<path>]

# 会话管理
hotplexd session list                          # 列出所有会话
hotplexd session kill <session-id>            # 终止会话
hotplexd session logs <session-id>            # 查看会话日志

# 诊断命令
hotplexd doctor                               # 全面诊断检查
hotplexd config validate                      # 验证配置文件

# 状态命令
hotplexd status                               # 运行时状态概览
hotplexd version                              # 显示版本信息
```

### 2.2 全局 Flags

```bash
--admin-token=<token>    # Admin API 认证 token
--server-url=<url>       # Daemon 地址 (默认: http://localhost:8080)
```

---

## 3. Admin API 端点

基于 REST over HTTP 实现。

### 3.1 端点定义

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/admin/v1/sessions` | 列出所有会话 |
| GET | `/admin/v1/sessions/:id/logs` | 获取会话日志 |
| GET | `/admin/v1/sessions/:id` | 会话详情 |
| DELETE | `/admin/v1/sessions/:id` | 终止会话 |
| GET | `/admin/v1/stats` | 全局统计 |
| POST | `/admin/v1/config/validate` | 验证配置 |
| GET | `/admin/v1/health/detailed` | 详细健康检查 |

### 3.2 响应格式

**GET /admin/v1/sessions**

```json
{
  "sessions": [
    {
      "id": "sess_abc123",
      "status": "running",
      "created_at": "2025-01-15T10:30:00Z",
      "last_active": "2025-01-15T10:35:00Z",
      "provider": "claude-code",
      "cli_version": "1.0.12"
    }
  ],
  "total": 1
}
```

**GET /admin/v1/sessions/:id**

```json
{
  "id": "sess_abc123",
  "status": "running",
  "created_at": "2025-01-15T10:30:00Z",
  "last_active": "2025-01-15T10:35:00Z",
  "provider": "claude-code",
  "cli_version": "1.0.12",
  "config": {
    "provider": "claude-code",
    "workdir": "/tmp/hotplex/sess_abc123"
  },
  "stats": {
    "input_tokens": 1500,
    "output_tokens": 3200,
    "duration_seconds": 300
  }
}
```

**DELETE /admin/v1/sessions/:id**

```json
{
  "success": true,
  "message": "Session sess_abc123 terminated"
}
```

**GET /admin/v1/sessions/:id/logs**

```json
{
  "session_id": "sess_abc123",
  "log_path": "/var/log/hotplex/sessions/sess_abc123.log",
  "size_bytes": 102400,
  "last_modified": "2025-01-15T10:35:00Z"
}
```

**GET /admin/v1/stats**

```json
{
  "total_sessions": 5,
  "active_sessions": 2,
  "stopped_sessions": 3,
  "uptime": "24h30m",
  "memory_usage_mb": 128,
  "cpu_usage_percent": 12.5
}
```

**POST /admin/v1/config/validate**

Request:
```json
{
  "config_path": "/etc/hotplex/config.yaml"
}
```

Response:
```json
{
  "valid": true,
  "errors": []
}
```

**GET /admin/v1/health/detailed**

```json
{
  "status": "healthy",
  "checks": {
    "database": true,
    "config": true,
    "cli_available": true,
    "websocket_connections": 2
  },
  "details": {
    "database_latency_ms": 5,
    "cli_version": "1.0.12",
    "config_file": "/etc/hotplex/config.yaml"
  }
}
```

### 3.3 鉴权

- 环境变量：`HOTPLEX_ADMIN_TOKEN`
- HTTP Header：`Authorization: Bearer <token>`
- 403 Forbidden when token mismatch

### 3.4 错误响应格式

所有 API 错误统一使用以下格式：

```json
{
  "error": {
    "code": "AUTH_FAILED | FORBIDDEN | NOT_FOUND | INVALID_REQUEST | SERVER_ERROR",
    "message": "Human readable error message",
    "details": {}  // Optional: additional error context
  }
}
```

**HTTP 状态码映射**：

| Code | HTTP Status | Description |
|------|-------------|-------------|
| AUTH_FAILED | 401 | Token 缺失或无效 |
| FORBIDDEN | 403 | Token 无权限 |
| NOT_FOUND | 404 | 资源不存在 |
| INVALID_REQUEST | 400 | 请求参数错误 |
| SERVER_ERROR | 500 | 服务端错误 |

---

## 4. 架构设计

### 4.1 目录结构

```
cmd/hotplexd/
├── main.go              # Entry point + Cobra init
├── cmd/
│   ├── root.go          # Root command
│   ├── start.go         # Daemon start
│   ├── session/
│   │   ├── list.go
│   │   ├── kill.go
│   │   └── logs.go
│   ├── doctor.go        # Diagnostic
│   ├── config.go        # Config validate
│   └── status.go        # Runtime status

internal/
├── admin/
│   ├── handler.go       # HTTP handlers
│   ├── middleware.go    # Auth middleware
│   └── types.go         # Request/Response types
```

### 4.2 数据流

```
┌─────────────────────────────────────────────────────┐
│                    hotplexd (CLI)                    │
│  ┌─────────────────────────────────────────────┐    │
│  │              Cobra Commands                  │    │
│  │  start │ session ls/kill/logs │ doctor    │    │
│  └─────────────────────────────────────────────┘    │
│                         │                            │
│                         ▼                            │
│              ┌──────────────────┐                   │
│              │   HTTP Client     │                   │
│              └──────────────────┘                   │
└───────────────────────┬─────────────────────────────┘
                        │
                        ▼
┌───────────────────────────────────────────────────────┐
│                  hotplexd (Daemon)                    │
│  ┌───────────────────────────────────────────────┐  │
│  │          Admin API Handlers                    │  │
│  │  /admin/v1/sessions | /admin/v1/health       │  │
│  └───────────────────────────────────────────────┘  │
│                         │                            │
│                         ▼                            │
│              ┌──────────────────┐                   │
│              │  Engine (Pool)    │                   │
│              └──────────────────┘                   │
└───────────────────────────────────────────────────────┘
```

---

## 5. 实现计划

### Phase 1: CLI 框架集成

- 引入 Cobra 依赖
- 将现有 flag 迁移为 root command flags
- 实现 `hotplexd start` 子命令

### Phase 2: Admin API

- 添加 `/admin/v1/*` 端点
- 实现 Admin Token 鉴权
- 实现统一错误响应格式
- 复用现有 Engine 接口

### Phase 3: 会话管理命令

- 实现 `session list`
- 实现 `session kill`
- 实现 `session logs`

### Phase 4: 诊断工具

- 实现 `doctor` 命令
- 实现 `config validate`
- 实现 `status` 命令
- 实现 `version` 命令

---

## 6. 验收标准

- [ ] CLI 命令正确解析并执行
- [ ] Admin API 正确返回会话列表
- [ ] Admin Token 鉴权生效
- [ ] session kill 能正确终止会话
- [ ] doctor 命令能检测常见问题
- [ ] 向后兼容：现有启动方式不变

---

## 7. 相关资源

- Cobra: https://github.com/spf13/cobra
- HotPlex Engine 接口定义
- 现有 `/health` 端点实现参考
