# Cron Relay 实施计划

**关联 Issue**: [#325](https://github.com/hrygo/hotplex/issues/325) | **基于规格**: `docs/superpowers/specs/2026-03-22-cron-relay-design.md`

---

## 实施概览

| 阶段 | 主要交付物 | 预估工作 |
|------|-----------|---------|
| Phase 0 | 类型系统 + 接口契约 | 基础层，无调度逻辑 |
| Phase 1a | CronScheduler + CronStore + CLI | 最小可用 Cron |
| Phase 1b | 并发限制 + 重试 + 历史 + Webhook | 健壮性增强 |
| Phase 1.5 | Agent Card + Admin API | 多 Agent 协作基础 |
| Phase 2 | RelayManager + CircuitBreaker | 跨实例通信 |

**依赖关系**：
```
Phase 0 ──► Phase 1a ──► Phase 1b ──► Phase 1.5
                              │
                              └──► Phase 2（独立）
```

---

## 阶段 0：前置基础设施

### 0.1 新增依赖

```bash
go get github.com/robfig/cron/v3@v3.0.1
go get github.com/sony/gobreaker@v2
```

> `github.com/google/uuid` 已在 go.mod 中（v1.6.0），复用于 UUID v4 生成。

### 0.2 `internal/cron/job.go` — 核心类型

**文件**: `internal/cron/job.go`

- [ ] `CronJob` struct（所有字段，`ID` 由 `uuid.New().String()` 生成，UUID v4）
- [ ] `JobType` 常量 (`light`, `medium`, `resource-intensive`)
- [ ] `OutputFormat` 常量 (`text`, `json`, `structured`)
- [ ] `Event` 常量 (`completed`, `failed`, `canceled`)
- [ ] `CronRun` struct
- [ ] `CronCallback` 接口
- [ ] `WebhookCallback` 实现
- [ ] `cronJobsFile` 结构（含 `Version` 字段）
- [ ] `const cronJobsSchemaVersion = 1`

**文件**: `internal/relay/binding.go`

- [ ] `RelayBinding` struct
- [ ] `RelayMessage` struct（嵌入 `bridgewire.WireMessage` 扩展字段）

### 0.3 `internal/agent/card.go` — Agent Card 类型

- [ ] `AgentCard` struct
- [ ] `Capabilities` struct
- [ ] `Skill` struct
- [ ] `Security` struct
- [ ] `Provider` struct

### 0.4 `internal/brain/cron_intent.go` — 意图解析类型

- [ ] `AddCronJobIntent` struct
- [ ] `DeleteCronJobIntent` struct
- [ ] `PauseCronJobIntent` struct
- [ ] `ResumeCronJobIntent` struct
- [ ] 意图检测正则表达式（复用 `cron`, `每`, `定时` 等关键词）

### 0.5 `internal/engine/relay.go` — Relay 执行器接口

- [ ] `RelayExecutor` 接口定义
- [ ] `RelayRequest` / `RelayResponse` struct
- [ ] `HandleRelay(ctx, req) → (*RelayResponse, error)` 实现
- [ ] 编译时接口验证: `var _ RelayExecutor = (*Engine)(nil)`

### 0.6 `internal/bridgewire/wire.go` — WireMessage 扩展

- [ ] 在 `WireMessage` 中添加 relay 字段（均 `omitempty`）:
  - `From`, `To`, `TaskID`, `Status`, `Response`, `Error`, `CreatedAt`
- [ ] 添加前先读取现有实现，确认兼容现有 JSON 序列化

### 0.7 Schema 版本管理

**文件**: `internal/cron/store.go`

- [ ] `ReadWithMigration(path)` — 读取 JSON 时检测 version 字段，执行必要迁移
- [ ] 当前 version=1 时为初始状态，直接加载

### 验证标准

```bash
go build ./internal/cron/...
go build ./internal/relay/...
go build ./internal/brain/...
go build ./internal/engine/...
```

---

## 阶段 1a：Cron 调度器核心

### 1a.1 `internal/cron/store.go` — 持久化存储

- [ ] `CronStore` struct（含 `sync.Mutex` 和 `map[string]*CronJob`）
- [ ] `NewCronStore(path string) (*CronStore, error)` — 启动时加载 `jobs.json`，无则创建
- [ ] `Get(id string) *CronJob`
- [ ] `Add(job *CronJob) error` — 创建时用 `uuid.New().String()` 生成 ID（UUID v4，与 engine/chatapps 一致），原子写入
- [ ] `Update(job *CronJob) error` — 原子写入
- [ ] `Delete(id string) error`
- [ ] `List() []*CronJob`
- [ ] `atomicWrite(data any) error` — tmp 文件 + Rename
- [ ] 编译时验证: `var _ Store = (*CronStore)(nil)`

### 1a.2 `internal/cron/scheduler.go` — 调度器

- [ ] `CronScheduler` struct
- [ ] `NewCronScheduler(store *CronStore, engine *Engine) *CronScheduler`
- [ ] `Start()` — 遍历所有 `Enabled=true` 的 job，用 `robfig/cron` 注册
- [ ] `Stop()` — 优雅停止所有 cron 条目
- [ ] `AddJob(job *CronJob) error` — 动态添加（不重启调度器）
- [ ] `RemoveJob(id string) error` — 动态移除
- [ ] `PauseJob(id string)` / `ResumeJob(id string)` — 暂停/恢复
- [ ] `executeJob(jobID string)` — 执行逻辑：
  1. 获取 job
  2. `context.WithTimeout`
  3. 获取/创建 namespace="cron" 的 session
  4. 调用 Engine 执行
  5. 记录 CronRun
  6. 调用 Callback（如配置）
  7. 更新 `LastRun` / `LastError` / `NextRun`

### 1a.3 `internal/cron/executor.go` — 执行请求

- [ ] `ExecuteRequest` struct
- [ ] `Execute(ctx, req) (*ExecuteResult, error)`
- [ ] `ExecuteResult` struct（含 `Response`, `Error`, `Duration`）

### 1a.4 `internal/brain/cron_intent.go` — 意图检测实现

- [ ] `DetectCronIntent(userMsg string) (Intent, bool)` — 返回 intent 和是否检测到
- [ ] 意图到 `AddCronJobIntent` 的转换（含 cron 表达式解析）
- [ ] `ParseCronExpr(expr string) (string, error)` — 验证并规范化
- [ ] 测试用例覆盖常见表达（"每5分钟", "每天早上9点", "每周一"）

### 1a.5 CLI 工具

**文件**: `cmd/hotplexd/cron_cmd.go` 或独立子命令

| 命令 | 参数 | 说明 |
|------|------|------|
| `add_cron` | `--cron`, `--prompt`, `[--work-dir]`, `[--type]`, `[--timeout]` | 添加 job |
| `del_cron` | `--id` | 删除 job |
| `list_crons` | — | 列出所有 job |
| `pause_cron` | `--id` | 暂停 job |
| `resume_cron` | `--id` | 恢复 job |

- [ ] 所有子命令接入现有 `hotplexd` CLI
- [ ] 输出格式: 表格（可用 `tabby` 或纯文本）

### 1a.6 Engine 集成

**文件**: `internal/engine/pool.go` 或 `session.go`

- [ ] 确保 `GetOrCreateSession` 支持 `namespace="cron"` 参数
- [ ] `namespace="cron"` 时设置 `IdleTimeout=0`（永不过期）
- [ ] 添加单元测试覆盖 namespace 隔离

### 验证标准

```bash
go test ./internal/cron/... -v
go test ./internal/brain/... -v
hotplexd add_cron --cron "*/5 * * * *" --prompt "检查服务状态" --type light
hotplexd list_crons
```

---

## 阶段 1b：Cron 增强功能

### 1b.1 并发限制

**文件**: `internal/cron/scheduler.go`

- [ ] `CronScheduler` 添加 `sem chan struct{}` + `maxConcurrent int`
- [ ] `NewScheduler` 初始化 semaphore（默认 max=4）
- [ ] `executeJob` 入口处获取 semaphore，超限进入 FIFO 等待队列
- [ ] defer 释放 semaphore
- [ ] 配置项: `cron.max_concurrent_jobs`（YAML 配置）

### 1b.2 重试机制

**文件**: `internal/cron/executor.go`

- [ ] `retryWithBackoff(ctx, delays []time.Duration, fn func() error) error`
- [ ] 指数退避: `delays[i] *= 2`（默认: 1s → 2s → 4s）
- [ ] context 取消传播
- [ ] `executeJob` 调用重试逻辑，`Retries` 字段控制次数
- [ ] 记录 `RetryCount` 到 CronRun

### 1b.3 执行历史

**文件**: `internal/cron/runs.go`（新增）

- [ ] `RunsStore` struct（独立文件 `runs.json`，按 JobID 索引）
- [ ] `AddRun(run *CronRun)` — 追加，超限截断。`run.ID` 用 `uuid.New().String()` 生成（UUID v4）
- [ ] `GetRuns(jobID string) []*CronRun` — 获取指定 job 的历史
- [ ] `runs.json` 结构: `{ "version": 1, "runs": [...] }`
- [ ] 历史保留: 每个 job 最多 100 条（旧记录优先删除）

### 1b.4 Webhook Callback

**文件**: `internal/cron/callback.go`

- [ ] `WebhookCallback.Send(run *CronRun, url string) error`
- [ ] 支持 `Authorization: Bearer <token>` Header
- [ ] 10s 超时，2 次重试（指数退避）
- [ ] 独立 goroutine 执行，不阻塞主执行流
- [ ] 回调触发时机（Phase 1b 表格已在 spec Section 6.4）

### 1b.5 `list_runs` CLI

- [ ] `list_runs --job-id <id>` — 查看执行历史
- [ ] `list_runs --job-id <id> --last 10` — 最近 10 条

### 1b.6 输出格式验证

- [ ] `OutputFormat=json` 时对输出做 JSON 验证
- [ ] `OutputFormat=structured` 时按 `OutputSchema` 校验字段
- [ ] 输出截断至 4KB（`maxOutputBytes = 4096`）

### 验证标准

```bash
# 并发限制
cron.max_concurrent_jobs=2 hotplexd start
# → 第5个并发 job 应等待

# 重试
# → 日志显示 1s → 2s → 4s 重试序列

# 历史
hotplexd list_runs --job-id <id>
```

---

## 阶段 1.5：多 Agent 协作

### 1.5.1 Agent Card 注册

**文件**: `internal/agent/registry.go`

- [ ] `AgentRegistry` struct
- [ ] `Register(card *AgentCard)` — 启动时注册
- [ ] `GetAgentCard() *AgentCard` — 返回本实例 card
- [ ] `Discover(remoteURL string) (*AgentCard, error)` — HTTP GET 能力发现
- [ ] 本地内存缓存（避免频繁 HTTP 请求）

### 1.5.2 Agent Card 配置

**文件**: `internal/config/`

- [ ] `AgentCardConfig` 结构
- [ ] YAML 配置支持（`agent_card:` 节）
- [ ] 启动时验证必要字段（name, url）

### 1.5.3 Admin API 端点

**文件**: `internal/admin/cron_api.go`（新增）

- [ ] `GET /admin/cron/jobs` — 列表
- [ ] `POST /admin/cron/jobs` — 创建
- [ ] `DELETE /admin/cron/jobs/:id` — 删除
- [ ] `POST /admin/cron/jobs/:id/pause`
- [ ] `POST /admin/cron/jobs/:id/resume`
- [ ] `GET /admin/cron/jobs/:id/runs` — 执行历史

**文件**: `internal/admin/relay_api.go`（新增）

- [ ] `GET /admin/relay/bindings`
- [ ] `POST /admin/relay/bindings`

**文件**: `internal/admin/agent_card_api.go`（新增）

- [ ] `GET /admin/agent-card`

### 1.5.4 Admin API 路由注册

**文件**: `internal/admin/server.go` 或新文件

- [ ] 在现有 Admin Server 上注册新路由组 `/admin/cron`, `/admin/relay`
- [ ] 中间件: 认证（已有 `AdminToken` 检查则复用）

### 1.5.5 Brain Intent → Admin API 联动

- [ ] 用户说"每天早上9点提醒我 check PR" → `AddCronJobIntent` → `POST /admin/cron/jobs`
- [ ] 实现于 `brain/` 处理链路（现有 `brain/` 集成点调研）

### 验证标准

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:9080/admin/cron/jobs
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:9080/admin/agent-card
```

---

## 阶段 2：Bot-to-Bot Relay

### 2.1 `internal/relay/sender.go` — HTTP 发送

- [ ] `RelaySender` struct
- [ ] `Send(ctx, msg *RelayMessage, targetURL string) error`
- [ ] HTTP POST + JSON body
- [ ] `Authorization: Bearer <api_key>` Header
- [ ] 10s 超时
- [ ] 指数退避重试（1s → 2s → 4s，最多 3 次）
- [ ] 编译时验证: `var _ RelaySender = (*RelaySenderImpl)(nil)`

### 2.2 `internal/relay/circuit.go` — CircuitBreaker（gobreaker SDK）

使用 [sony/gobreaker](https://github.com/sony/gobreaker)，不重复造轮子。

- [ ] `RelayCircuitBreaker` struct（封装 `gobreaker.CircuitBreaker`）
- [ ] `NewRelayCircuitBreaker(name string, settings gobreaker.Settings) *RelayCircuitBreaker`
- [ ] 默认配置: `FailureThreshold=5`（连续失败次数）, `OpenDuration=30s`
- [ ] `Call(ctx, fn func() (any, error)) (any, error)` — 执行 + 状态转换
- [ ] 每个目标实例独立 CircuitBreaker（按 instance name 缓存）
- [ ] `circuitBreakers map[string]*gobreaker.CircuitBreaker`（sync.Map 或 RWMutex）
- [ ] 编译时验证: `var _ CircuitBreaker = (*RelayCircuitBreaker)(nil)`

### 2.3 `internal/relay/binding.go` — RelayManager

- [ ] `RelayManager` struct（含 CircuitBreaker map）
- [ ] `NewRelayManager(engine *Engine, config *RelayConfig) *RelayManager`
- [ ] `AddBinding(binding *RelayBinding)` — 注册 platform+chatID 路由
- [ ] `RemoveBinding(chatID string)`
- [ ] `ListBindings() []*RelayBinding`
- [ ] `Send(toAgent, content string) (*RelayResponse, error)`
  1. 查找 binding
  2. CircuitBreaker 检查
  3. HTTP POST
  4. 更新 CircuitBreaker
- [ ] `PersistBindings()` — 启动时加载 `bindings.json`，变更时原子写入
- [ ] `bindings.json` 持久化（含 version 字段）

### 2.4 Engine HandleRelay 实现

**文件**: `internal/engine/relay.go`

- [ ] `Engine.HandleRelay` 实现
  1. 解析 `RelayRequest`
  2. 路由到目标 session（`namespace="relay"`）
  3. 调用 Engine 执行消息内容
  4. 返回 `RelayResponse`
- [ ] 编译时验证: `var _ RelayExecutor = (*Engine)(nil)`

### 2.5 Relay 接收端点

**文件**: `internal/server/` 或 `internal/admin/`

- [ ] `POST /relay` — 接收 relay 消息
  - 验证 `Authorization` Header
  - 复用 WAF 检查（`detector.go`）
  - 调用 `HandleRelay`
  - 返回 JSON response

### 2.6 Relay CLI

| 命令 | 参数 | 说明 |
|------|------|------|
| `list_bindings` | — | 列出所有 relay 绑定 |
| `add_binding` | `--platform`, `--chat-id`, `--bot` | 添加绑定 |
| `del_binding` | `--chat-id` | 删除绑定 |
| `test_relay` | `--to <agent>` | 测试 relay 连通性 |

### 2.7 SDK 扩展

**文件**: `hotplex.go` 或 `client.go`

- [ ] `HotPlexClient.AddCronJob(ctx, job)`
- [ ] `HotPlexClient.DeleteCronJob(ctx, id)`
- [ ] `HotPlexClient.PauseCronJob(ctx, id)`
- [ ] `HotPlexClient.ResumeCronJob(ctx, id)`
- [ ] `HotPlexClient.ListCronJobs(ctx)`
- [ ] `HotPlexClient.GetCronRuns(ctx, jobID)`
- [ ] `HotPlexClient.SendRelay(ctx, to, content)`
- [ ] `HotPlexClient.AddRelayBinding(ctx, binding)`
- [ ] `HotPlexClient.RemoveRelayBinding(ctx, chatID)`
- [ ] `HotPlexClient.ListRelayBindings(ctx)`

### 验证标准

```bash
# 端到端测试（需两个 HotPlex 实例）
Instance A: hotplexd start --config instance-a.yaml
Instance B: hotplexd start --config instance-b.yaml

Instance A: hotplexd add_binding --platform slack --chat-id C001 --bot hotplex-b
Instance A: hotplexd test_relay --to hotplex-b
# → 应收到 Instance B 的响应

curl -X POST http://instance-b:9080/relay \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"from":"a","to":"b","content":"ping"}'
```

---

## 跨阶段集成测试计划

### T0: Phase 0 后
- 所有新增类型可正常编译
- Schema version=1 可正常解析

### T1: Phase 1a 后
```bash
# 基本 CRUD
hotplexd add_cron --cron "*/10 * * * *" --prompt "echo hello" --type light
hotplexd list_crons
# → 应显示新 job

# 等待触发（最快 10 分钟，或改用测试用 cron "*/1 * * * *"）
# 观察日志和 runs.json

hotplexd del_cron --id <job-id>
hotplexd list_crons
# → 应为空
```

### T2: Phase 1b 后
```bash
# 并发限制
# 启动时设 max_concurrent_jobs=1，同时添加 3 个同时触发的 job
# → 应串行执行

# 重试
# 添加一个总是失败的 job（无效 prompt），Retries=3
# → 日志应显示 3 次重试

# Webhook
# 添加带 OnComplete webhook 的 job
# → 触发后外部服务应收到 POST
```

### T3: Phase 1.5 后
```bash
curl -s http://localhost:9080/admin/agent-card | jq .
# → 应返回 AgentCard JSON
```

### T4: Phase 2 后
- 双实例 relay 连通性测试
- CircuitBreaker OPEN → HALF_OPEN 状态转换测试

---

## 配置项汇总

```yaml
# hotplex.yaml 新增配置节

cron:
  max_concurrent_jobs: 4        # Phase 1b
  default_timeout_mins: 30
  default_retries: 3
  default_retry_delay: 1s

relay:
  timeout: 10s                  # Phase 2
  retry_delays: [1s, 2s, 4s]   # Phase 2
  max_retries: 3
  circuit_failure_threshold: 5  # Phase 2 (gobreaker FailureThreshold)
  circuit_open_duration: 30s   # Phase 2 (gobreaker OpenDuration)

agent_card:                     # Phase 1.5
  name: "hotplex-default"
  url: "http://localhost:9080"
  provider:
    organization: "local"
  capabilities:
    streaming: false
    push_notifications: false
  skills: []
```

---

## 文件清单

| 文件 | 新建/修改 | 阶段 |
|------|----------|------|
| `go.mod` (新增依赖) | 修改 | 0 |
| `internal/cron/job.go` | 新建 | 0 |
| `internal/cron/store.go` | 新建 | 0, 1a |
| `internal/cron/scheduler.go` | 新建 | 1a |
| `internal/cron/executor.go` | 新建 | 1a |
| `internal/cron/runs.go` | 新建 | 1b |
| `internal/cron/callback.go` | 新建 | 1b |
| `internal/relay/binding.go` | 新建 | 0, 2 |
| `internal/relay/sender.go` | 新建 | 2 |
| `internal/relay/circuit.go` | 新建 | 2 |
| `internal/agent/card.go` | 新建 | 0 |
| `internal/agent/registry.go` | 新建 | 1.5 |
| `internal/brain/cron_intent.go` | 新建 | 0, 1a |
| `internal/engine/relay.go` | 新建 | 0, 2 |
| `internal/bridgewire/wire.go` | 修改 | 0 |
| `internal/admin/cron_api.go` | 新建 | 1.5 |
| `internal/admin/relay_api.go` | 新建 | 1.5 |
| `internal/admin/agent_card_api.go` | 新建 | 1.5 |
| `cmd/hotplexd/cron.go` | 新建 | 1a, 1b |
| `cmd/hotplexd/relay.go` | 新建 | 2 |
| `hotplex.go` 或 `client.go` | 修改 | 2 |
| `configs/hotplex.yaml` | 修改 | all |

---

## 风险与缓解

| 风险 | 缓解 | 验证方式 |
|------|------|---------|
| Phase 0 字段变更破坏现有 WireMessage 序列化 | 添加 `omitempty`，所有 relay 字段可选 | JSON roundtrip 测试 |
| cron session 泄露（IdleTimeout=0） | 独立 namespace，调度器 Stop 时清理所有 cron session | 长时间运行测试 |
| 并发写入 runs.json | 使用独立 mutex，与 jobs.json 分离 | 并发压测 |
| CircuitBreaker 状态机复杂易出错 | 使用 sony/gobreaker 成熟 SDK | gobreaker 单元测试覆盖 |
| Schema 版本迁移遗漏 | version 字段强制校验，无 version 拒绝加载 | 旧版本文件测试 |
