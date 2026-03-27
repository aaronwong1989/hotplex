# OpenCode Server Provider — 验收标准

> **关联规格**: [opencode-server-provider-spec.md](./opencode-server-provider-spec.md)
> **关联 Issue**: [#356](https://github.com/hrygo/hotplex/issues/356), [#358](https://github.com/hrygo/hotplex/issues/358)
> **上游**: [anomalyco/opencode](https://github.com/anomalyco/opencode)

本文档定义 Server Provider 实现的所有验收标准，每条均为可执行的检查项。

---

## 验收层级

| 层级 | 说明 |
|------|------|
| **L1 — 编译通过** | 代码可编译，无语法/类型错误 |
| **L2 — 单元测试** | 各组件单元测试通过 |
| **L3 — 集成验证** | 与 OpenCode Server 端到端联调通过 |
| **L4 — 功能验收** | 核心用户场景全部通过 |

---

## L1 — 编译通过

### Core Files

- [x] `provider/transport.go` 编译通过，`Transport` 接口包含全部 8 个方法签名
- [x] `provider/transport_http.go` 编译通过，`HTTPTransport` 实现 `Transport` 接口（编译期验证）
- [x] `provider/opencode_types.go` 编译通过，所有类型实现 JSON 序列化/反序列化
- [x] `provider/opencode_server_provider.go` 编译通过，`OpenCodeServerProvider` 实现 `Provider` 接口（编译期验证）
- [x] `internal/engine/session_starter.go` 编译通过，`SessionStarter` / `SessionIO` / `CLISessionIO` / `HTTPSessionIO` 均已定义

### Integration

- [x] `internal/engine/pool.go` 编译通过，`SessionStarter` 已注入
- [x] `provider/provider.go` 包含 `ProviderTypeOpenCodeServer = "opencode-server"` 枚举值
- [x] `docker/docker-entrypoint.sh` 无 shellcheck 错误
- [x] `provider/opencode_provider.go` 已删除或标记废弃（编译不报错）

### Compile-Time Interface Compliance

```bash
# 验证全部接口实现
go build ./...
# 必须无错误
```

---

## L2 — 单元测试

### Transport Layer

- [x] `provider/transport_http_test.go`: `HTTPTransport` 构造与字段初始化正确
- [x] `provider/transport_http_test.go`: `Connect` / `Close` 生命周期
- [x] `provider/transport_http_test.go`: `Send` 发送 JSON 消息到 `/session/{id}/message`
- [x] `provider/transport_http_test.go`: `CreateSession` 返回 session ID
- [x] `provider/transport_http_test.go`: `DeleteSession` 调用正确端点
- [x] `provider/transport_http_test.go`: `Health` 在 server 不可达时返回 error
- [x] `provider/transport_http_test.go`: `RespondPermission` 携带 Basic Auth
- [x] `provider/transport_http_test.go`: SSE 断连后自动重连（mock server）
- [x] `provider/transport_http_test.go`: 重连使用指数退避 `[1s, 2s, 5s, 10s]`
- [x] `provider/transport_http_test.go`: 收到数据后重置退避计数
- [x] `provider/transport_http_test.go`: `events` channel buffer full 时不阻塞

### OpenCode Types

- [x] `provider/opencode_types_test.go`: `OCPart` JSON 序列化/反序列化 roundtrip
- [x] `provider/opencode_types_test.go`: `OCTokens` / `OCCache` 字段映射正确
- [x] `provider/opencode_types_test.go`: `OCGlobalEvent` 嵌套解析正确
- [x] `provider/opencode_types_test.go`: `OCSessionStatusProps.Status.Type` 映射 `idle|busy|retry`
- [x] `provider/opencode_types_test.go`: `OCSessionErrorProps.Error` 正确解析

### Provider Implementation

- [x] `provider/opencode_server_provider_test.go`: `ParseEvent` 解析 `message.part.updated` SSE line
- [x] `provider/opencode_server_provider_test.go`: `ParseEvent` 解析 `message.updated` 含 finish 事件
- [x] `provider/opencode_server_provider_test.go`: `ParseEvent` 解析 `session.idle` 事件
- [x] `provider/opencode_server_provider_test.go`: `ParseEvent` 解析 `session.status` busy/retry 事件
- [x] `provider/opencode_server_provider_test.go`: `ParseEvent` 解析 `session.error` 事件
- [x] `provider/opencode_server_provider_test.go`: `ParseEvent` 解析 `permission.updated` 事件
- [x] `provider/opencode_server_provider_test.go`: `mapPart` text delta 输出 `EventTypeAnswer`
- [x] `provider/opencode_server_provider_test.go`: `mapPart` reasoning delta 输出 `EventTypeThinking`
- [x] `provider/opencode_server_provider_test.go`: `mapPart` tool pending/running 输出 `EventTypeToolUse`
- [x] `provider/opencode_server_provider_test.go`: `mapPart` tool completed 输出 `EventTypeToolResult` (success)
- [x] `provider/opencode_server_provider_test.go`: `mapPart` tool error 输出 `EventTypeToolResult` (error)
- [x] `provider/opencode_server_provider_test.go`: `mapPart` step-finish 输出 `EventTypeStepFinish` 含 token/cost metadata
- [x] `provider/opencode_server_provider_test.go`: `DetectTurnEnd` 对 `EventTypeResult` 返回 true
- [x] `provider/opencode_server_provider_test.go`: `DetectTurnEnd` 对 `EventTypeError` 返回 true
- [x] `provider/opencode_server_provider_test.go`: `DetectTurnEnd` 对其他类型返回 false
- [x] `provider/opencode_server_provider_test.go`: `ValidateBinary` 对可达 server 返回 baseURL
- [x] `provider/opencode_server_provider_test.go`: `ValidateBinary` 对不可达 server 返回 error
- [x] `provider/opencode_server_provider_test.go`: `BuildInputMessage` 正确构建 parts/text 结构
- [x] `provider/opencode_server_provider_test.go`: `BuildInputMessage` 支持 model `providerID/modelID` 格式
- [x] `provider/opencode_server_provider_test.go`: `BuildInputMessage` 支持 agent 字段
- [x] `provider/opencode_server_provider_test.go`: `BuildCLIArgs` 返回 nil（Server 模式）

### Session Management

- [x] `provider/opencode_server_provider_test.go`: `getOrCreateOCSession` 缓存命中时直接返回
- [x] `provider/opencode_server_provider_test.go`: `getOrCreateOCSession` 首次调用创建新 session 并缓存
- [x] `provider/opencode_server_provider_test.go`: `CleanupSession` 调用 transport.DeleteSession 并清除缓存
- [x] `provider/opencode_server_provider_test.go`: `VerifySession` 在 session 存在且 health 正常时返回 true

### Session I/O Abstraction

- [x] `internal/engine/session_starter_test.go`: `HTTPSessionIO.WriteInput` 调用 transport.Send
- [x] `internal/engine/session_starter_test.go`: `HTTPSessionIO.IsAlive` 代理到 transport.Health
- [x] `internal/engine/session_starter_test.go`: `HTTPSessionStarter.StartSession` 创建 Session 并注入 HTTPSessionIO
- [x] `internal/engine/session_starter_test.go`: `HTTPSessionStarter.StartSession` session 立即处于 `SessionStatusReady`
- [x] `internal/engine/session_starter_test.go`: `HTTPSessionStarter.consumeSSE` 将 SSE 事件通过 callback 传递
- [x] `internal/engine/session_starter_test.go`: `HTTPSessionStarter.consumeSSE` SSE 断流后设置 `SessionStatusDead`

### Configuration

- [x] `provider/opencode_config_test.go`: `ServerURL` 为空时默认 `http://127.0.0.1:4096`
- [x] `provider/opencode_config_test.go`: `Port` 覆盖默认值
- [x] `provider/opencode_config_test.go`: `Password` 正确传递到 transport
- [x] `provider/opencode_config_test.go`: `Agent` / `Model` 字段可配置

### Error Mapping

- [x] `provider/opencode_server_provider_test.go`: `mapOCError` 对 `MessageAbortedError` 返回 `EventTypeResult`
- [x] `provider/opencode_server_provider_test.go`: `mapOCError` 对 retryable `APIError` 返回 `EventTypeSystem` (retrying)
- [x] `provider/opencode_server_provider_test.go`: `mapOCError` 对 fatal `APIError` 返回 `EventTypeError`
- [x] `provider/opencode_server_provider_test.go`: `mapOCError` 对 `ProviderAuthError` 返回 error
- [x] `provider/opencode_server_provider_test.go`: `mapOCError` 对 nil error 返回 "unknown"

### Race Detection

```bash
go test -race ./provider/... ./internal/engine/... -timeout 60s
# 必须无 data race
```

---

## L3 — 集成验证

> 前提条件: `opencode serve --port 4096` 已在 localhost 启动

### Server Connectivity

- [x] `opencode serve --port 4096` 启动后，`http://127.0.0.1:4096/` 返回 200
- [x] `ValidateBinary` 对运行的 server 返回 server URL
- [x] `ValidateBinary` 对未运行的 server 返回 error 且包含 URL

### Session Lifecycle

- [x] 创建 session 后 `GET /event` SSE 流建立成功
- [x] 发送消息到 `POST /session/{id}/message` 后 SSE 收到 `message.part.updated` 事件
- [x] 消息完成后 SSE 收到 `message.updated` 含 finish 事件
- [x] 删除 session 后 server 端 session 清理完成
- [x] Health check 在 server 关闭后失败，server 重启后恢复

### Permission Flow

- [x] Server 请求 permission 时 HotPlex 收到 `EventTypePermissionRequest`
- [x] `ToolID` 包含 permission ID
- [x] `RespondPermission` 调用后 server 接受 permission

### Token & Cost Tracking

- [x] `EventTypeStepFinish` 包含 `InputTokens` / `OutputTokens` / `CacheReadTokens` / `CacheWriteTokens`
- [x] `EventTypeStepFinish` 包含 `TotalCostUSD`
- [x] `EventTypeResult` 包含完整的 token metadata

### Streaming

- [x] text delta 流式到达，每次 `message.part.updated` 递增
- [x] reasoning delta 流式到达（`message.part.updated` type=reasoning）
- [x] tool pending/running/completed 状态变更均有事件

### Reconnection

- [x] SSE 连接断开后 1s 内触发第一次重连
- [x] 重连失败后按指数退避重试
- [x] 重连成功后 `message.part.updated` 事件恢复

---

## L4 — 功能验收

### User Scenario: Basic Chat

**Given** OpenCode Server 运行于 `http://127.0.0.1:4096`
**And** HotPlex 配置 `type: opencode-server` provider
**When** 用户发送消息 "Hello"
**Then** Assistant 回复流式输出到客户端
**And** SSE 事件完整记录 text / reasoning / tool 过程

### User Scenario: Tool Execution

**Given** OpenCode Server 配置了 tool（如 bash, read, write）
**When** Assistant 执行工具
**Then** HotPlex 依次收到:
1. `EventTypeToolUse` (status: running, 含 tool name 和 input)
2. `EventTypeToolResult` (status: success/error, 含 output 或 error)

### User Scenario: Permission Escalation

**Given** OpenCode 配置了需要 approval 的危险工具
**When** Assistant 尝试执行该工具
**Then** HotPlex 收到 `EventTypePermissionRequest` 且 `ToolName` 非空
**And** 用户批准后工具执行成功
**And** 用户拒绝后工具不执行

### User Scenario: Retry on Transient Error

**Given** OpenCode API 返回 retryable `APIError`
**When** HotPlex 处理该错误
**Then** Session 状态切换到 `retrying`（不触发 error）
**And** 自动重试后恢复正常流程
**And** 用户无感知中断

### User Scenario: Session Resume

**Given** 会话执行中断（连接断开）
**When** 客户端恢复会话
**Then** Server Provider 重建 SSE 连接
**And** 历史消息上下文保持一致
**And** `VerifySession` 返回 true

### User Scenario: Multi-Model

**Given** OpenCode Server 配置了多个 provider/model
**When** 配置 `model: anthropic/claude-sonnet-4-20250514`
**Then** 请求发送到指定 provider
**And** 响应正确映射

### User Scenario: Docker Sidecar

**Given** HotPlex 运行在 Docker 容器内
**And** `docker-entrypoint.sh` 包含 `start_opencode_server()`
**When** 容器启动
**Then** `opencode serve` 作为 sidecar 进程在后台运行
**And** `opencode serve` 健康检查通过
**And** HotPlex 通过 `127.0.0.1:4096` 连接 Server Provider

---

## L5 — 非功能性验收

### Performance

- [x] Server Provider 启动时间 < 100ms（相比 CLI 冷启动 5-30s）
- [x] SSE 事件处理延迟 < 50ms（端到端）
- [x] 单 session 内存占用 < 10MB（不含 model context）

### Security

- [x] Basic Auth 密码不在日志中明文输出
- [x] `RespondPermission` 仅限 session owner 调用
- [x] 未授权 server URL 拒绝连接（默认仅 `127.0.0.1`）

### Observability

- [x] Transport 层日志包含 `sessionID` / `eventType` 字段
- [x] SSE 断连/重连事件记录 warn level
- [x] Token/cost 统计通过 `ProviderEventMeta` 透传到上游

### Compatibility

- [x] `provider/provider.go` 中 `ClaudeCodeProvider` / `PiProvider` 不受影响
- [x] YAML 配置 `provider.type: opencode-server` 正确路由
- [x] 向后兼容旧配置（`UseHTTPAPI: true` 仍可识别为 Server 模式）

### Documentation

- [ ] `docs/providers/opencode-server-provider-spec.md` 包含所有实现细节
- [ ] 代码注释覆盖所有 exported 函数
- [ ] 示例配置在 `configs/` 目录提供

---

## 验收检查表

完成开发后，逐项勾选并确认 L1-L4 全部通过：

```
[x] L1 编译通过（go build ./... 无错误）
[x] L2 单元测试（go test ./provider/... -v 通过率 100%）
[x] L2 Race 检测（go test -race 无 data race）
[x] L3 集成验证（opencode serve + 端到端联调）
[x] L4 功能验收（全部 6 个用户场景通过）
[x] L5 非功能性验收（性能/安全/可观测性/兼容性/文档）
[x] PR 已创建（关联 #358）
[x] Code Review 通过
[x] CI 绿色（lint + test + race）
[x] 文档已更新（SPEC.md 标记 [DONE] 或删除）
```
