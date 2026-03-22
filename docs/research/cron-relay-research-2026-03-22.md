# 调研报告：Cron 调度器 & Bot-to-Bot Relay — 竞品分析 & HotPlex 适配设计

**日期**: 2026-03-22
**基于**: Issue #325 + cc-connect 竞品源码 + OpenClaw AgentScope + A2A Protocol + CrewAI/Mastra/LangGraph 最佳实践
**目标**: 为 HotPlex 设计符合其架构风格的多 Agent 协作 + Cron + Relay 系统

---

## 1. 竞品概览

| 系统 | 架构语言 | Cron 实现 | Relay 实现 | 持久化 | 多 Agent 协作 |
|------|---------|----------|-----------|--------|--------------|
| **cc-connect** | Go (2300+ stars) | CronScheduler + CronStore | RelayManager + RelayExecutor | JSON 文件 | 无原生支持 |
| **OpenClaw AgentScope** | Python + FastAPI | CronManager 单例，6字段秒级 | HTTP Client → 目标实例 | cron_jobs.json | Session 级串行 |
| **Google A2A Protocol** | JSON-RPC 2.0 | 无内置 | 协议级 Agent 通信 | 外部 | 核心设计 |
| **CrewAI** | Python | Task async_execution + callback | Task delegation | 外部存储 | 角色分工 + hierarchical |
| **Mastra** | TypeScript | Scheduled Workflows | Agent tools | DB | Workflow 图编排 |
| **LangGraph** | Python | 外挂 + cron | Shared state + message passing | Checkpoint | 状态机 + 持久化 |
| **HotPlex** (现状) | Go | **无** | **无** | Session marker | Bridge 平台接入 |

---

## 1.5 多 Agent 协作最佳实践

### 1.5.1 A2A Protocol (Google) — 协议级设计

**核心理念**: Agent 作为服务，通过标准协议相互发现和通信。

```json
// Agent Card: 能力发现机制
{
  "name": "hotplex-coder",
  "provider": {"organization": "acme", "url": "https://..."},
  "capabilities": {
    "streaming": true,
    "pushNotifications": true,
    "extendedAgentCard": false
  },
  "skills": [
    {"id": "code-review", "name": "Code Review", "description": "..."}
  ],
  "securitySchemes": ["APIKey", "OAuth2"]
}
```

**任务状态机**（借鉴 A2A Protocol）:

| 状态 | 含义 |
|------|------|
| `working` | 处理中 |
| `input_required` | 需用户/其他 Agent 补充信息 |
| `completed` | 成功完成 |
| `failed` | 执行失败 |
| `canceled` | 被取消 |

**消息格式**（JSON-RPC 2.0）:
```json
{
  "jsonrpc": "2.0",
  "method": "tasks/send",
  "params": {
    "id": "task-uuid",
    "sessionId": "session-uuid",
    "acceptedOutputModes": ["text", "json"],
    "message": {
      "role": "user",
      "parts": [{"type": "text", "text": "帮我看看这段代码"}]
    }
  }
}
```

**HotPlex 适配**:
- 扩展 bridgewire WireMessage（`omitempty` 字段：`From`, `To`, `TaskID`, `Status`, `Response`, `Error`, `CreatedAt`）
- 引入 Agent Card 机制（配置中声明能力）
- 任务状态机支持 cancel/pause

### 1.5.2 CrewAI 多 Agent 协作模式

**Agent 定义三要素**:
```python
Agent(
    role="代码审查员",      # 角色/专业领域
    goal="发现代码问题",    # 个人目标
    backstory="10年经验",  # 背景/人格
    tools=[...],
    allow_delegation=True  # 是否允许委托任务
)
```

**Process Types**:

| 模式 | 特点 | 适用场景 |
|------|------|---------|
| Sequential | 预定义顺序，上游输出 → 下游输入 | 线性 Pipeline |
| Hierarchical | Manager Agent 负责任务分配和审核 | 需要质量把控 |
| Consensual | 民主协商（待实现） | 复杂决策 |

**Task Callback 机制**:
```python
def on_complete(output: TaskOutput):
    # 触发通知、更新状态、触发下游任务
    notify_user(output)

task = Task(
    description="审查代码",
    agent=reviewer,
    callback=on_complete,
    async_execution=True  # 不阻塞后续
)
```

**HotPlex 适配**:
- Cron Job 作为 Task，支持 callback 机制
- Hierarchical 模式：cron trigger → manager bot → delegate

### 1.5.3 LangGraph 状态持久化模式

**核心特性**:
- **Checkpoint**: 任意时刻可中断/恢复
- **Memory**: 短期工作记忆 + 长期持久化记忆
- **Human-in-the-loop**: 关键节点暂停等人工审批

```python
# LangGraph 风格的状态机
state = {"messages": [], "context": {}, "task_status": "working"}

# 可在任何节点中断
if should_interrupt(state):
    raise Interrupt("waiting_for_approval")

# Checkpoint 保存完整状态
checkpoint = {"state": state, "node": "analyze"}
```

**HotPlex 适配**:
- Cron Job 执行中间状态持久化（避免重复执行）
- 支持 pause/resume（借鉴 LangGraph interrupt）

### 1.5.4 Mastra Workflows 编排

```typescript
// Mastra 风格工作流
const workflow = createWorkflow({ trigger: cron("0 9 * * *") })
  .step(collectData)
  .then(analyzeData)
  .branch({
    if: (state) => state.anomaly,
    then: alertTeam,
    else: storeResults
  })
```

**HotPlex 适配**:
- Cron Trigger 作为 Workflow 入口
- 支持条件分支（anomaly detection → alert）

---

## 1.6 Agent Cron Job 最佳实践

### 1.6.1 任务分类 (CrewAI Style)

| 类型 | 特点 | 执行策略 |
|------|------|---------|
| `light` | 快速查询、分析 | 可并行，超时短 |
| `medium` | 报告生成、代码审查 | 串行，超时适中 |
| `resource-intensive` | 大规模重构、数据处理 | 资源感知调度 |

### 1.6.2 执行可靠性 (NexFlow Pulse)

| 机制 | 说明 |
|------|------|
| 自动重试 + 指数退避 | 失败后 1s → 2s → 4s 重试 |
| 执行历史记录 | HTTP 状态码、延迟、错误信息 |
| 测试运行 | 立即触发不影响原调度 |
| 按执行计费 | 暂停任务不收费 |

### 1.6.3 智能调度 (Arcron)

- **ML 驱动**: 学习历史负载模式，预测最优执行窗口
- **资源感知**: 避免与高峰时段冲突
- **Self-Healing**: 预测偏离实际时自动调整

**HotPlex 简化实现**:
- 基于 SessionPool 负载状态选择执行时机
- 避免在用户高峰期执行 heavy cron job

### 1.6.4 Session 隔离 (OpenClaw)

| Session 类型 | 用途 | 隔离策略 |
|-------------|------|---------|
| `user-*` | 用户交互 | 标准权限 |
| `cron-*` | 定时任务 | 独立 namespace，无打扰模式 |
| `relay-*` | Bot 间通信 | 专用上下文 |

### 1.6.5 输出格式化 (CrewAI Style)

```python
Task(
    output_json={"summary": str, "issues": list},
    output_pydantic=ReportModel  # Pydantic 模型校验
)
```

**HotPlex 适配**:
- Cron Job 支持 `output_format`: `text` | `json` | `structured`
- 执行结果自动解析并路由到目标平台

---

## 2. cc-connect cron.go 深度解析

### 2.1 核心数据结构

```go
// CronJob: 丰富的任务描述
type CronJob struct {
    ID          string    `json:"id"`
    Project     string    `json:"project"`         // 引擎/项目标识
    SessionKey  string    `json:"session_key"`     // 目标会话
    CronExpr    string    `json:"cron_expr"`       // cron 表达式
    Prompt      string    `json:"prompt"`          // 自然语言 prompt
    Exec        string    `json:"exec,omitempty"`  // 可选：直接 shell 命令
    WorkDir     string    `json:"work_dir,omitempty"`
    Description string    `json:"description"`
    Enabled     bool     `json:"enabled"`
    Silent      *bool    `json:"silent,omitempty"`  // 是否静默（不通知）
    Mute        bool     `json:"mute,omitempty"`
    SessionMode string   `json:"session_mode,omitempty"` // 专用 session
    TimeoutMins *int     `json:"timeout_mins,omitempty"`  // 超时控制
    CreatedAt   time.Time `json:"created_at"`
    LastRun     time.Time `json:"last_run,omitempty"`
    LastError   string    `json:"last_error,omitempty"`
}
```

### 2.2 CronStore: 持久化层

```go
type CronStore struct {
    path string
    mu   sync.Mutex
    jobs []*CronJob
}
// 核心方法：
// - NewCronStore(dataDir string)        // 创建目录 + 加载已有任务
// - Add(job *CronJob) error             // 原子写入 jobs.json
// - Remove(id string) bool
// - List() []*CronJob                   // 返回副本
// - Get(id string) *CronJob
// - MarkRun(id string, err error)       // 更新 LastRun + LastError
```

**关键设计**: `sync.Mutex` + 原子写入，避免并发写入损坏

### 2.3 CronScheduler: 调度循环

```go
type CronScheduler struct {
    store   *CronStore
    cron    *cron.Cron            // github.com/robfig/cron/v3
    engines map[string]*Engine    // project → Engine
    entries map[string]cron.EntryID  // jobID → EntryID
}

func (cs *CronScheduler) executeJob(jobID string) {
    // 1. 获取 job
    // 2. context.WithTimeout(ctx, *job.TimeoutMins)
    // 3. 调用 engine.ExecuteCronJob(job)
    // 4. 更新 LastRun / LastError
}
```

**关键设计**:
- 依赖 `robfig/cron/v3`（广泛使用的 cron 库）
- 5字段 cron（分 时 日 月 周），支持 `@every`、宏
- `executeJob` 在 goroutine 中执行，支持超时控制
- `entries map[jobID]cron.EntryID` 便于删除/禁用

### 2.4 Agent 工具集成

cc-connect 在 `AGENTS.md` system prompt 中注入工具：

```
## 定时任务
cc-connect cron add --cron "0 6 * * *" --prompt "..." --desc "...";
cc-connect cron list;
cc-connect cron del --id <id>;
```

**HotPlex 适配**: Brain 意图识别 → 构造成 AddCronJob/DeleteCronJob/SuspendCronJob 请求

---

## 3. cc-connect relay.go 深度解析

### 3.1 核心数据结构

```go
type RelayBinding struct {
    Platform  string            `json:"platform"`
    ChatID   string            `json:"chat_id"`
    Bots     map[string]string `json:"bots"` // project name → bot display name
}

type RelayManager struct {
    engines   map[string]*Engine   // project → Engine
    bindings  map[string]*RelayBinding // chatID → binding
    storePath string
    timeout   time.Duration
}

type RelayRequest struct {
    From        string `json:"from"`
    To          string `json:"to"`
    SessionKey  string `json:"session_key"`
    Message     string `json:"message"`
}

type RelayResponse struct {
    Response string `json:"response"`
}
```

### 3.2 Send 流程

```
User: "@bot2 帮我看看这个问题"
  → Bridge 识别 @bot2
  → RelayManager.Send(to="bot2", message="帮我看看这个问题")
    → 解析 SessionKey
    → 验证 binding 和目标 engine
    → Engine.HandleRelay(request)  // [C2] Phase 0 新增 RelayExecutor 接口
    → 目标 bot 执行任务
    → 返回响应
    → 在 group chat 发布 visibility 消息
```

**关键设计**:
- Binding 机制：Platform + ChatID 绑定一组 bots
- HTTP 无直接处理，依赖 RelayExecutor.HandleRelay（Phase 0 新增接口）
- 持久化 relay_bindings.json

---

## 4. OpenClaw AgentScope 框架实践

### 4.1 CronManager 设计

| 特性 | 实现 |
|------|------|
| **秒级精度** | 6字段 cron（秒 分 时 日 月 周） |
| **表达式示例** | `*/30 * * * * *` 每30秒；`0 */5 * * * *` 每5分钟 |
| **任务注册** | 内置工具：`add_cron`, `del_cron`, `list_crons` |
| **持久化** | `cron_jobs.json`，重启自动恢复 |
| **执行隔离** | 专用 `cronjob` session，串行执行 |
| **并发控制** | SessionManager 统一管理，任务不并发冲突 |

### 4.2 ReAct Agent 执行

```python
# cron job 触发时
agent = ReActAgent(config)
result = await agent.run(prompt)

# Agent 可主动调用 memory_search 等工具
# 内置 pre_reasoning hook 防幻觉
```

### 4.3 关键启示

1. **工具化注册**: Cron 作为内置工具，Agent 可以主动管理
2. **专用 Session**: `cronjob` session 与普通用户 session 完全隔离
3. **持久化**: JSON 文件，简单可靠，支持重启恢复

---

## 5. HotPlex 现有架构适配分析

### 5.1 可复用基础设施

| 组件 | 位置 | 复用价值 |
|------|------|---------|
| `SessionPool` | `internal/engine/pool.go` | Cron/Relay 任务的 session 管理 |
| `bridgewire` | `internal/bridgewire/wire.go` | WireMessage 格式可扩展支持 relay |
| `Engine` | `internal/engine/session.go` | Cron job 执行入口 |
| `safetyGuard` | `brain/guard.go` | Cron/Relay 意图识别扩展 |
| `hotplex.go` | `hotplex.go` | SDK 暴露 cron/relay 接口 |

### 5.2 架构差距

```
当前 HotPlex:
  Bridge ← Platform (Slack/TG/Ding)
              ↓
           Engine ← SessionPool
                        (无 cron、无 relay)

cc-connect:
  Bridge ← Platform
              ↓
  CronScheduler ← CronStore (JSON 持久化)
  RelayManager ← Engine
                    ↓
              CronJob 执行 / Relay Send
```

---

## 6. 设计方案：三阶段演进（含多 Agent 最佳实践）

### Phase 0: 架构基础（新增）

建立 cron + relay 的类型定义和存储层。

```
internal/cron/
├── job.go         # CronJob, CronRun, CronStore 类型
├── scheduler.go   # CronScheduler (调度循环)
├── executor.go   # ExecuteRequest → Engine
└── store.go      # 原子写入 JSON (Mutex + os.Rename)

internal/relay/
├── binding.go     # RelayBinding, RelayManager
├── sender.go     # HTTP Sender (A2A 风格)
├── agentcard.go  # Agent Card 能力发现
└── circuit.go   # CircuitBreaker (per target instance)

internal/agent/
├── card.go       # AgentCard 类型定义
└── registry.go  # 本地 Agent 注册表

internal/brain/
└── cron_intent.go # CronIntent（独立于 guard.go）
```

**Phase 0 必须完成的前置动作**:
- **[C4]** `hotplex.go` 公开 `EngineOptions.Namespace` 字段，使 cron/relay 可创建独立 Engine 实例
- **[C3]** `bridgewire/wire.go` 添加 relay 字段（`omitempty`）：`From`, `To`, `TaskID`, `Status`, `Response`, `Error`, `CreatedAt`
- **[C2]** `internal/engine/runner.go` 新增 `RelayExecutor` 接口（`HandleRelay(ctx, req) (*RelayResponse, error)`）

### Phase 1a: Cron 调度器（核心）

| 功能 | 实现方式 | 借鉴来源 |
|------|---------|---------|
| 表达式解析 | `github.com/robfig/cron/v3` | cc-connect |
| 持久化 | `cron_jobs.json`，原子写入 | cc-connect + NexFlow |
| 执行 | 专用 `cron-` session（复用 SessionPool） | OpenClaw |
| 意图解析 | Brain cron intent（`brain/cron_intent.go`） | cc-connect |
| 工具注入 | `add_cron`, `del_cron`, `list_crons` | OpenClaw |
| Session 隔离 | `Namespace="cron"` 独立于普通会话 | OpenClaw |
| 生命周期 | CronScheduler 随 Engine 启动/关闭 | — |

**关键决策**:
- 使用 `robfig/cron/v3`（Go 生态最成熟）
- Session 隔离：Cron job 使用 `namespace="cron"` 独立于普通会话
- CronScheduler 作为 `Engine` 的子组件，随 Engine 生命周期管理

### Phase 1b: Cron 增强功能

| 功能 | 实现方式 | 借鉴来源 |
|------|---------|---------|
| 工具扩展 | `pause_cron`, `resume_cron` | OpenClaw |
| 输出格式化 | `output_format`: `text` \| `json` \| `structured`；structured 使用 `map[string]any` + 字段枚举验证 | CrewAI |
| 执行历史 | `runs.json`（`CronRun` 类型） | NexFlow Pulse |
| 任务分类 | `type`: `light` \| `medium` \| `resource-intensive` | CrewAI |
| 重试机制 | 指数退避（1s → 2s → 4s，最多 N 次） | NexFlow Pulse |
| 并发限制 |  semaphore（默认 maxConcurrent=4） | — |
| Callback | `CronCallback` 接口（Webhook URL 或 in-process） | CrewAI |

### Phase 1.5: 多 Agent 协作（Relay 增强）

| 功能 | 实现方式 | 借鉴来源 |
|------|---------|---------|
| Agent Card | 配置中声明 capabilities + skills | A2A Protocol |
| 能力发现 | `AgentCard` HTTP endpoint | A2A Protocol |
| 任务状态机 | `working` → `completed`/`failed`/`canceled` | A2A Protocol |
| 消息格式 | 扩展 bridgewire WireMessage（含 relay 字段） | HotPlex 现有 |
| 传输协议 | JSON-RPC 2.0 over HTTP | A2A Protocol |
| 绑定管理 | RelayBinding: Platform + ChatID → Bots | cc-connect |
| Admin API | Cron/Relay 管理端点（`/admin/cron/*`, `/admin/relay/*`） | — |

**新增**: A2A 风格的 Agent Card 和能力发现机制。Phase 1.5 依赖 Phase 1b 的 `CronRun` 执行历史，作为状态追踪的基础。

### Phase 2: Bot-to-Bot Relay（完整实现）

| 功能 | 实现方式 | 借鉴来源 |
|------|---------|---------|
| 协议 | 扩展 `bridgewire` WireMessage（含 From/To/TaskID/Status） | HotPlex 现有 |
| 传输 | HTTP POST 到目标 HotPlex 实例 | cc-connect |
| 路由 | RelayBinding: Platform + ChatID → Bots | cc-connect |
| 安全 | API Key 认证 + WAF 检查 | A2A Protocol |
| 超时 | 可配置 timeout（默认 120s） | cc-connect |
| 重试 | 指数退避（1s → 2s → 4s，最多 3 次） | NexFlow Pulse |
| 熔断 | CircuitBreaker per target instance（closed/open/half-open） | — |
| 状态追踪 | 任务状态回调 + 执行历史（`CronRun`） | A2A + NexFlow |

**关键决策**:
- Relay 作为 Bridge 的扩展，而非独立系统
- 目标 HotPlex 实例通过配置注册（URL + API Key）
- **[C2]**: 通过 `RelayExecutor` 接口执行，不直接依赖 Engine 方法
- **[M3]**: 熔断器防止级联故障（target instance 不可用时快速失败）

---

## 6.5 增强的 CronJob 结构（融合最佳实践）

```go
type CronJob struct {
    // 核心字段
    ID          string    `json:"id"`           // UUID
    CronExpr    string    `json:"cron_expr"`   // 标准5字段cron
    Prompt      string    `json:"prompt"`       // 自然语言任务描述
    SessionKey  string    `json:"session_key"` // 目标会话
    WorkDir     string    `json:"work_dir"`    // 工作目录

    // 执行控制（借鉴 CrewAI）
    Type        JobType   `json:"type"`        // light | medium | resource-intensive
    TimeoutMins int       `json:"timeout_mins"`
    Retries     int           `json:"retries"`       // 重试次数（默认 3）
    RetryDelay  time.Duration `json:"retry_delay"`   // 初始重试延迟

    // 输出格式化（借鉴 CrewAI）
    OutputFormat OutputFormat `json:"output_format"` // text | json | structured
    OutputSchema string       `json:"output_schema,omitempty"` // JSON Schema (用于 text/structured 输出校验)

    // 通知控制
    Enabled     bool      `json:"enabled"`
    Silent      bool      `json:"silent"`       // 不通知
    NotifyOn    []Event   `json:"notify_on"`   // completed | failed | all

    // 元数据
    CreatedBy   string    `json:"created_by"`   // 用户标识
    CreatedAt   time.Time `json:"created_at"`
    LastRun     time.Time `json:"last_run,omitempty"`
    LastError   string    `json:"last_error,omitempty"`
    NextRun     time.Time `json:"next_run,omitempty"`
    RunCount    int       `json:"run_count"`    // 执行次数

    // Callback（借鉴 CrewAI）
    OnComplete  string    `json:"on_complete,omitempty"` // Webhook URL 或 in-process callback
    OnFail      string    `json:"on_fail,omitempty"`
}

// cron_jobs.json 文件头（用于 schema 版本管理）
// { "version": 1, "jobs": [...] }
const CronJobsSchemaVersion = 1

type JobType string
const (
    JobTypeLight             JobType = "light"
    JobTypeMedium            JobType = "medium"
    JobTypeResourceIntensive JobType = "resource-intensive"
)

type OutputFormat string
const (
    OutputFormatText       OutputFormat = "text"
    OutputFormatJSON        OutputFormat = "json"
    OutputFormatStructured  OutputFormat = "structured"
)

type Event string
const (
    EventCompleted Event = "completed"
    EventFailed    Event = "failed"
    EventCanceled  Event = "canceled"
)

// CronRun: runs.json 单条记录（NexFlow Pulse 风格）
type CronRun struct {
    ID         string        `json:"id"`          // Run ID (UUID)
    JobID      string        `json:"job_id"`      // 关联的 CronJob ID
    StartedAt  time.Time     `json:"started_at"`
    FinishedAt time.Time     `json:"finished_at,omitempty"`
    Duration   time.Duration `json:"duration"`    // FinishedAt - StartedAt
    Status     string        `json:"status"`      // success | failed | canceled | running
    Error      string        `json:"error,omitempty"`
    RetryCount int           `json:"retry_count"` // 本次 run 的重试次数
    Response   string        `json:"response,omitempty"` // 输出内容（截断）
}

// CronCallback: Phase 1b 支持两种 callback 模式
type CronCallback interface {
    OnComplete(run *CronRun) error
    OnFail(run *CronRun) error
}

// WebhookCallback 实现 CronCallback，支持 Bearer token 认证和超时控制
type WebhookCallback struct {
    URL     string
    Token   string // 可选 Bearer token
    Timeout time.Duration // 默认 10s，超时自动取消
    Retry   int           // 失败后重试次数，默认 2
}
```

---

## 6.6 增强的 Relay 结构（融合 A2A Protocol）

```go
// Agent Card（借鉴 A2A Protocol）
type AgentCard struct {
    Name        string       `json:"name"`
    Provider    Provider     `json:"provider"`
    URL         string       `json:"url"`          // Agent 服务端点
    Capabilities Capabilities `json:"capabilities"`
    Skills      []Skill      `json:"skills"`       // 可用技能列表
    Security    []Security   `json:"security"`
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

// Relay Binding（借鉴 cc-connect）
type RelayBinding struct {
    Platform string            `json:"platform"`  // slack, feishu, ding
    ChatID   string            `json:"chat_id"`   // Group Chat ID
    Bots     map[string]string `json:"bots"`     // project → display_name
}

// Relay Message（借鉴 A2A Protocol）
type RelayMessage struct {
    TaskID     string    `json:"task_id"`     // 任务唯一 ID
    From       string    `json:"from"`        // 发送方 Agent
    To         string    `json:"to"`          // 目标 Agent
    Content    string    `json:"content"`     // 消息内容
    SessionKey string    `json:"session_key"` // 可选：指定 session
    Metadata   string    `json:"metadata,omitempty"`
    Status     string    `json:"status"`      // working | completed | failed | canceled
    Response   string    `json:"response,omitempty"`
    Error      string    `json:"error,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}
```

---

## 7. 技术选型建议

| 决策点 | 推荐方案 | 理由 |
|--------|---------|------|
| Cron 库 | `robfig/cron/v3` | Go 生态标准，稳定可靠 |
| 持久化 | JSON 文件（原子写入） | 简单、无外部依赖、cc-connect 验证 |
| 执行隔离 | 独立 SessionPool (`cron-` 前缀) | HotPlex SessionPool 已支持多 namespace |
| 意图解析 | `brain/cron_intent.go`（独立于 guard.go） | 架构一致，关注分离 |
| Relay 协议 | 复用 bridgewire | 避免重复造轮子 |
| 自然语言注册 | 工具化（cc-connect 模式） | Agent 可主动管理 |

---

## 8. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| Cron 表达式解析错误 | 使用 `cron.ParseStandard()` + 验证 |
| 并发写入 jobs.json | `sync.Mutex` + 原子写入（os.Rename） |
| Cron job 执行阻塞 | context.WithTimeout + SessionGC |
| Relay 超时 | 可配置 timeout，优雅处理 |
| 安全风险 | WAF 检查 relay 消息内容 |
| **[C4]** Cron Session 被 IdleTimeout 回收 | Phase 0 公开 `Namespace`，cron Engine 使用 `IdleTimeout=0`（永不过期） |
| **[M3]** Relay 级联故障（目标实例宕机） | CircuitBreaker per target instance（失败 N 次后 open） |
| **[m4]** Schema 演进导致旧配置无法解析 | `cron_jobs.json` 包含 `version` 字段，读取时校验并迁移 |

---

## 9. 参考资料

- [cc-connect 源码 core/cron.go](https://github.com/chenhg5/cc-connect/blob/main/core/cron.go)
- [cc-connect 源码 core/relay.go](https://github.com/chenhg5/cc-connect/blob/main/core/relay.go)
- [OpenClaw CronManager](https://github.com/owenliang/openclaw)
- [robfig/cron/v3](https://pkg.go.dev/github.com/robfig/cron/v3)
- [Issue #325 - Cron调度器 + Bot-to-Bot Relay](https://github.com/hrygo/hotplex/issues/325)
