# Cron 调度器 & Bot-to-Bot Relay

**Issue**: [#325](https://github.com/hrygo/hotplex/issues/325) | **版本**: v0.35+ | **状态**: Spec

为 HotPlex 添加定时任务调度和跨实例 Bot 通信能力，使 AI Agent 能够按计划执行任务并在多个实例间协作。

---

## 1. 概述

### 1.1 目标

- **Cron 调度器**：让 AI Agent 按 cron 表达式定时执行任务，支持重试、回调和执行历史。
- **Bot-to-Bot Relay**：让多个 HotPlex 实例通过 HTTP 相互通信，实现跨实例任务委托和响应聚合。
- **多 Agent 协作**：引入 Agent Card 能力发现和任务状态机，支持 Hierarchical 委托模式。

### 1.2 设计原则

- 复用现有基础设施（`SessionPool`、`bridgewire`、`Engine`），不引入新的进程池。
- 持久化使用原子 JSON 写入（`sync.Mutex` + `os.Rename`），无外部依赖。
- Cron/Relay session 与用户 session 完全隔离，使用独立 `namespace`。
- 意图解析独立于 `guard.go`，避免安全层与业务层耦合。

### 1.3 演进阶段

| 阶段 | 内容 | 里程碑 |
|------|------|---------|
| Phase 0 | 前置基础设施（类型、存储层、接口定义） | 所有类型定义就绪 |
| Phase 1a | Cron 核心（调度 + 执行 + 持久化） | `add_cron` 可用 |
| Phase 1b | Cron 增强（重试 + 并发限制 + Callback + 执行历史） | 执行历史可查 |
| Phase 1.5 | 多 Agent 协作（Agent Card + Admin API） | 能力发现可用 |
| Phase 2 | Bot-to-Bot Relay（HTTP 中继 + CircuitBreaker） | 跨实例通信可用 |

---

## 2. 架构

### 2.1 组件关系

```
Platform (Slack/TG/Ding)
        │
        ▼
    Bridge ──▶ Engine ──▶ SessionPool
        │              (namespace="cron")  (namespace="relay")
        │
        ▼
  CronScheduler      RelayManager
        │                    │
        ▼                    ▼
  CronStore          RelayBinding
  (jobs.json)        (bindings.json)
  (runs.json)
```

### 2.2 Session 隔离

| Namespace | 用途 | IdleTimeout |
|-----------|------|-------------|
| `user-*` | 用户交互 | 标准配置 |
| `cron-*` | 定时任务 | `0`（永不过期，避免执行中被回收） |
| `relay-*` | Bot 间通信 | 标准配置 |

### 2.3 包结构

```
internal/cron/
├── job.go       # CronJob, CronRun, JobType, OutputFormat, Event 类型
├── scheduler.go # CronScheduler（robfig/cron/v3 驱动）
├── executor.go  # ExecuteRequest → Engine 执行
└── store.go    # CronStore（Mutex + os.Rename 原子写入）

internal/relay/
├── binding.go    # RelayBinding, RelayManager
├── sender.go     # HTTP POST + 指数退避重试
├── circuit.go    # CircuitBreaker（per target instance）
└── agentcard.go  # AgentCard, Skills, Capabilities

internal/agent/
├── card.go      # AgentCard 类型（跨包共享）
└── registry.go  # 本地 Agent 注册表

internal/brain/
└── cron_intent.go  # CronIntent 类型（独立于 guard.go）

internal/engine/
└── relay.go  # RelayExecutor 接口 + HandleRelay 实现
```

---

## 3. 核心类型

### 3.1 CronJob

```go
type CronJob struct {
    ID          string    `json:"id"`            // UUID
    CronExpr    string    `json:"cron_expr"`    // 标准 5 字段 cron
    Prompt      string    `json:"prompt"`        // 自然语言任务描述
    SessionKey  string    `json:"session_key"`  // 目标会话（可选）
    WorkDir     string    `json:"work_dir"`     // 工作目录

    Type        JobType   `json:"type"`          // light | medium | resource-intensive
    TimeoutMins int       `json:"timeout_mins"`
    Retries     int       `json:"retries"`       // 默认 3
    RetryDelay  time.Duration `json:"retry_delay"`

    OutputFormat OutputFormat `json:"output_format"` // text | json | structured
    OutputSchema string      `json:"output_schema,omitempty"` // JSON Schema（字段枚举验证）

    Enabled   bool    `json:"enabled"`
    Silent    bool    `json:"silent"`
    NotifyOn  []Event `json:"notify_on"` // completed | failed | all

    CreatedBy string    `json:"created_by"`
    CreatedAt time.Time `json:"created_at"`
    LastRun   time.Time `json:"last_run,omitempty"`
    LastError string    `json:"last_error,omitempty"`
    NextRun   time.Time `json:"next_run,omitempty"`
    RunCount  int       `json:"run_count"`

    OnComplete string `json:"on_complete,omitempty"` // Webhook URL
    OnFail     string `json:"on_fail,omitempty"`
}

type JobType string

const (
    JobTypeLight             JobType = "light"
    JobTypeMedium            JobType = "medium"
    JobTypeResourceIntensive JobType = "resource-intensive"
)

type OutputFormat string

const (
    OutputFormatText       OutputFormat = "text"
    OutputFormatJSON       OutputFormat = "json"
    OutputFormatStructured OutputFormat = "structured"
)

type Event string

const (
    EventCompleted Event = "completed"
    EventFailed     Event = "failed"
    EventCanceled   Event = "canceled"
)
```

### 3.2 CronRun

每次执行的独立记录，不修改 CronJob 本身。

```go
type CronRun struct {
    ID          string        `json:"id"`           // Run UUID
    JobID       string        `json:"job_id"`       // 关联的 CronJob ID
    StartedAt   time.Time     `json:"started_at"`
    FinishedAt  time.Time     `json:"finished_at,omitempty"`
    Duration    time.Duration `json:"duration"`
    Status      string        `json:"status"`        // success | failed | canceled | running
    Error       string        `json:"error,omitempty"`
    RetryCount int           `json:"retry_count"`
    Response    string        `json:"response,omitempty"` // 输出内容（截断至 4KB）
}
```

### 3.3 CronCallback

```go
// CronCallback 支持两种模式：Webhook URL（远程回调）和 in-process 函数
type CronCallback interface {
    OnComplete(run *CronRun) error
    OnFail(run *CronRun) error
}

type WebhookCallback struct {
    URL     string
    Token   string        // 可选 Bearer token
    Timeout time.Duration // 默认 10s
    Retry   int           // 默认 2 次
}
```

### 3.4 AgentCard

用于跨实例能力发现（Phase 1.5）。

```go
type AgentCard struct {
    Name         string       `json:"name"`
    Provider     Provider     `json:"provider"`
    URL          string       `json:"url"`            // Agent 服务端点
    Capabilities Capabilities `json:"capabilities"`
    Skills       []Skill      `json:"skills"`
    Security     []Security   `json:"security"`
}

type Capabilities struct {
    Streaming          bool `json:"streaming"`
    PushNotifications bool `json:"push_notifications"`
}

type Skill struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}
```

### 3.5 RelayBinding

Platform + ChatID 绑定一组 Bot 实例。

```go
type RelayBinding struct {
    Platform string            `json:"platform"` // slack, feishu, ding
    ChatID   string            `json:"chat_id"`
    Bots     map[string]string `json:"bots"` // project name → display name
}
```

### 3.6 RelayMessage

扩展 `bridgewire.WireMessage`，添加 relay 专用字段（均 `omitempty`）。

```go
type RelayMessage struct {
    TaskID     string    `json:"task_id"`     // 任务唯一 ID
    From       string    `json:"from"`         // 发送方 Agent 名称
    To         string    `json:"to"`           // 目标 Agent 名称
    Content    string    `json:"content"`      // 消息正文
    SessionKey string    `json:"session_key,omitempty"`
    Metadata   string    `json:"metadata,omitempty"`
    Status     string    `json:"status"`      // working | completed | failed | canceled
    Response   string    `json:"response,omitempty"`
    Error      string    `json:"error,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}
```

---

## 4. Phase 0：前置基础设施

Phase 0 不实现任何调度逻辑，只建立类型系统和接口契约。

### 4.1 必须完成的前置动作

| 编号 | 动作 | 文件 |
|------|------|------|
| P0.1 | 公开 `EngineOptions.Namespace` 字段 | `hotplex.go` |
| P0.2 | `bridgewire.WireMessage` 添加 relay 字段（均 `omitempty`） | `internal/bridgewire/wire.go` |
| P0.3 | 新增 `RelayExecutor` 接口 + `HandleRelay` 实现 | `internal/engine/relay.go` |

```go
// P0.3: internal/engine/relay.go
type RelayExecutor interface {
    HandleRelay(ctx context.Context, req *RelayRequest) (*RelayResponse, error)
}

type RelayRequest struct {
    From       string `json:"from"`
    To         string `json:"to"`
    SessionKey string `json:"session_key"`
    Message    string `json:"message"`
}

type RelayResponse struct {
    Response string `json:"response"`
}
```

### 4.2 Schema 版本管理

所有持久化 JSON 文件头包含版本字段，读取时校验并迁移。

```go
// cron_jobs.json 结构
type cronJobsFile struct {
    Version int        `json:"version"`
    Jobs    []*CronJob `json:"jobs"`
}

const cronJobsSchemaVersion = 1  // 当前版本
```

---

## 5. Phase 1a：Cron 调度器核心

### 5.1 CronScheduler

使用 `github.com/robfig/cron/v3`，标准 5 字段（分 时 日 月 周）。

```go
type CronScheduler struct {
    store   *CronStore
    cron    *cron.Cron
    engine  *Engine
    sem     chan struct{}  // P1b：并发限制 semaphore
}

func (cs *CronScheduler) executeJob(jobID string) {
    job := cs.store.Get(jobID)
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(job.TimeoutMins)*time.Minute)
    defer cancel()

    // 1. 从 SessionPool 获取或创建 cron session（namespace="cron"）
    // 2. 调用 Engine 执行 prompt
    // 3. 记录 CronRun 到 runs.json
    // 4. 调用 CronCallback（如已配置）
    // 5. 更新 CronJob.LastRun / LastError
}
```

### 5.2 CronStore

```go
type CronStore struct {
    path string
    mu   sync.Mutex
    jobs map[string]*CronJob // id → job
}

func (cs *CronStore) atomicWrite() error {
    tmp := cs.path + ".tmp"
    if err := json.NewEncoder(f).Encode(...) ; err != nil {
        return err
    }
    return os.Rename(tmp, cs.path)  // 原子替换
}
```

### 5.3 Brain 意图解析

独立于 `guard.go`，在 `brain/cron_intent.go` 中定义：

```go
type AddCronJobIntent struct {
    CronExpr  string
    Prompt    string
    WorkDir   string
    Type      JobType
    TimeoutMins int
    Enabled   bool
}

type DeleteCronJobIntent struct{ JobID string }
type PauseCronJobIntent struct{ JobID string }
type ResumeCronJobIntent struct{ JobID string }
```

### 5.4 CLI 工具

| 工具 | 说明 |
|------|------|
| `add_cron` | 添加 cron job |
| `del_cron` | 删除 cron job |
| `list_crons` | 列出所有 cron job |

---

## 6. Phase 1b：Cron 增强功能

### 6.1 并发限制

`CronScheduler` 使用 `semaphore`（默认 `maxConcurrent=4`）限制同时执行的 job 数量，超出的进入 FIFO 队列。

```go
type CronScheduler struct {
    sem           chan struct{}
    maxConcurrent int
}

func NewScheduler(maxConcurrent int) *CronScheduler {
    if maxConcurrent <= 0 {
        maxConcurrent = 4
    }
    return &CronScheduler{
        sem:           make(chan struct{}, maxConcurrent),
        maxConcurrent: maxConcurrent,
    }
}
```

### 6.2 重试机制

失败后按指数退避重试，最多重 `Retries` 次。

```go
func retryWithBackoff(ctx context.Context, delays []time.Duration, fn func() error) error {
    var err error
    for i, delay := range delays {
        if err = fn(); err == nil {
            return nil
        }
        select {
        case <-time.After(delay):
        case <-ctx.Done():
            return ctx.Err()
        }
        delays[i] *= 2  // 指数退避
    }
    return fmt.Errorf("all retries failed: %w", err)
}
```

### 6.3 执行历史

每次执行追加到 `runs.json`（按 `JobID` + `StartedAt` 索引）。历史保留策略：每个 Job 最多保留最近 100 条（`runs.json` 写满时截断旧记录）。

### 6.4 Callback 触发时机

| 事件 | 触发条件 | 行为 |
|------|---------|------|
| `completed` | `Status == "success"` | `CronCallback.OnComplete()` |
| `failed` | `Status == "failed"` 且重试耗尽 | `CronCallback.OnFail()` |
| `canceled` | context 超时或显式取消 | `CronCallback.OnFail()` |

---

## 7. Phase 1.5：多 Agent 协作

### 7.1 Agent Card

每个 HotPlex 实例在启动时注册 Agent Card。能力发现通过 HTTP `GET /agent-card` 端点返回 JSON。

```go
// 配置示例 (hotplex.yaml)
agent_card:
  name: "hotplex-coder"
  url: "https://hotplex.example.com"
  provider:
    organization: "acme"
    url: "https://acme.example.com"
  capabilities:
    streaming: true
    push_notifications: false
  skills:
    - id: "code-review"
      name: "Code Review"
      description: "代码审查和质量分析"
    - id: "refactor"
      name: "Refactor"
      description: "代码重构和优化"
```

### 7.2 任务状态机

沿用 A2A Protocol 状态定义：

| 状态 | 含义 |
|------|------|
| `working` | 执行中 |
| `completed` | 成功完成 |
| `failed` | 执行失败 |
| `canceled` | 被取消 |
| `input_required` | 需其他 Agent 或用户补充信息 |

### 7.3 Admin API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/admin/cron/jobs` | GET | 列出所有 cron job |
| `/admin/cron/jobs` | POST | 创建 cron job |
| `/admin/cron/jobs/:id` | DELETE | 删除 cron job |
| `/admin/cron/jobs/:id/pause` | POST | 暂停 |
| `/admin/cron/jobs/:id/resume` | POST | 恢复 |
| `/admin/cron/jobs/:id/runs` | GET | 查看执行历史 |
| `/admin/relay/bindings` | GET | 列出 relay 绑定 |
| `/admin/relay/bindings` | POST | 创建 relay 绑定 |
| `/admin/agent-card` | GET | 查看本实例 Agent Card |

---

## 8. Phase 2：Bot-to-Bot Relay

### 8.1 RelayManager

```go
type RelayManager struct {
    engine      *Engine
    bindings    map[string]*RelayBinding // chatID → binding
    storePath   string
    timeout     time.Duration
    circuit     *CircuitBreaker          // per target instance
}

func (rm *RelayManager) Send(to, content string) (*RelayResponse, error) {
    // 1. 查找目标 binding
    // 2. CircuitBreaker 检查目标实例是否可用
    // 3. HTTP POST 到目标 /relay 端点
    // 4. 指数退避重试（1s → 2s → 4s，最多 3 次）
    // 5. CircuitBreaker 更新状态
}
```

### 8.2 CircuitBreaker

每个目标 HotPlex 实例独立熔断器，防止级联故障。

```
状态机: CLOSED → OPEN → HALF-OPEN → CLOSED
         │                   │
         ▼                   ▼
    连续失败 N 次       单次成功 → CLOSED
```

实现：`internal/relay/circuit.go`，使用 `go.uber.org/atomic` 管理状态。

### 8.3 安全

- 目标实例通过配置注册（URL + API Key）。
- 每条 relay 消息经过现有 WAF 检查（`detector.go`）。
- API Key 通过 HTTP Header 传递（`Authorization: Bearer <key>`）。

---

## 9. API 扩展（hotplex.go SDK）

```go
type HotPlexClient interface {
    // 现有接口 ...

    // Cron API
    AddCronJob(ctx context.Context, job *CronJob) error
    DeleteCronJob(ctx context.Context, id string) error
    PauseCronJob(ctx context.Context, id string) error
    ResumeCronJob(ctx context.Context, id string) error
    ListCronJobs(ctx context.Context) ([]*CronJob, error)
    GetCronRuns(ctx context.Context, jobID string) ([]*CronRun, error)

    // Relay API
    SendRelay(ctx context.Context, to, content string) (*RelayResponse, error)
    AddRelayBinding(ctx context.Context, binding *RelayBinding) error
    RemoveRelayBinding(ctx context.Context, chatID string) error
    ListRelayBindings(ctx context.Context) ([]*RelayBinding, error)
}
```

---

## 10. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| Cron 表达式解析错误 | `cron.ParseStandard()` + 启动时验证 |
| 并发写入 `jobs.json` | `sync.Mutex` + `os.Rename` 原子写入 |
| Cron Session 被 IdleTimeout 回收 | `IdleTimeout=0`（永不过期） |
| Relay 级联故障 | CircuitBreaker per target instance |
| Schema 演进导致旧配置无法解析 | `cron_jobs.json` 含 `version` 字段，读取时迁移 |
| Webhook callback 阻塞执行 | 独立 goroutine + 10s 超时 + 重试 2 次 |
| Relay 消息无 WAF 检查 | relay 消息复用现有 `detector.go` 检查 |

---

## 11. 参考资料

- [cc-connect cron.go](https://github.com/chenhg5/cc-connect/blob/main/core/cron.go)
- [cc-connect relay.go](https://github.com/chenhg5/cc-connect/blob/main/core/relay.go)
- [robfig/cron/v3](https://pkg.go.dev/github.com/robfig/cron/v3)
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/)
- [CrewAI Concepts](https://docs.crewai.com/en/concepts/)
- [Issue #325 — Cron调度器 + Bot-to-Bot Relay](https://github.com/hrygo/hotplex/issues/325)
