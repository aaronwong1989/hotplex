# HotPlex v1.0.0 架构设计方案

> 调研日期：2026-03-29  
> 定位：AI Agent Runtime Engine（放弃历史包袱，采用模块化插件化设计）

---

## 一、现状痛点

| 文件 | LoC | 问题 |
|------|------|------|
| `chatapps/engine_handler.go` | 2127 | 平台逻辑集中，单文件过大 |
| `brain/init_test.go` | 1490 | Brain 初始化过于复杂 |
| `engine/runner.go` | 1138 | I/O 复用、事件映射、超时控制混杂 |
| `internal/engine/session_starter.go` | 486 | CLI/HTTP 启动逻辑耦合 |

**三大根因：**
1. **Engine**: Session 和 I/O 传输紧耦合，新增 Provider 需改 runner
2. **Brain**: Intent/Guard/Memory 混杂，LLM 中间件分散
3. **ChatApps**: 平台适配器各自实现 Session 管理，无统一抽象

---

## 二、架构目标

```
┌─────────────────────────────────────────────────────────┐
│                      Channel Layer                       │
│   [Feishu Adapter]  [Slack Adapter]  [WS Adapter]       │
└──────────────────────────┬─────────────────────────────┘
                           │ 统一 Message + Session ID
                           ▼
┌─────────────────────────────────────────────────────────┐
│                   Native Brain (原生智能层)                │
│   Intent Classification → Guard/WAF → Routing             │
│   ⚠️ 不作为 Worker，是独立智能层                          │
└──────────────────────────┬─────────────────────────────┘
                           │ 标准化的 Task/Result
                           ▼
┌─────────────────────────────────────────────────────────┐
│                      Worker Layer                        │
│    [ClaudeCode Worker]  [OpenCode Worker]               │
└──────────────────────────┬─────────────────────────────┘
                           │ subprocess / API
                           ▼
┌─────────────────────────────────────────────────────────┐
│                      Provider Layer                      │
│        [Anthropic]  [OpenAI]  [SiliconFlow]            │
└─────────────────────────────────────────────────────────┘
```

### Native Brain 定位（核心澄清）

Native Brain **不是 Worker**，是 HotPlex 的原生智能层：

| 维度 | Native Brain | Worker |
|------|-------------|--------|
| 职责 | 意图路由、WAF、上下文补全、响应格式化 | 执行具体任务（代码生成、文件操作等）|
| 调用位置 | Channel 和 Worker **之间**（消息预处理） | Brain 之后 |
| 实现方式 | LLM API 调用 / 规则引擎 | Claude Code CLI / OpenCode CLI |
| 状态 | 无状态（每次请求独立） | 有状态（Session 绑定）|

---

## 三、核心接口设计

### 3.1 Channel 接口（平台抽象）

```go
// channel.go
type Message struct {
    ID          string                 // 平台消息ID
    SessionID   string                 // 统一会话ID
    ChannelID   string                 // 渠道标识 (feishu/slack/ws)
    UserID      string                 // 用户标识
    Content     string                 // 消息内容
    RawContent  map[string]interface{} // 平台原生消息体
    Timestamp   time.Time
    Metadata    map[string]string     // 扩展元数据
}

type Channel interface {
    Name() string                     // "feishu" | "slack" | "ws"
    Send(ctx context.Context, msg Message) error
    // 消息接收通过事件回调（避免阻塞）
    OnMessage(handler func(msg Message))
    // 平台特定能力
    PlatformCapability() PlatformCapability
}

type PlatformCapability struct {
    SupportsMarkdown   bool
    SupportsStreaming bool
    SupportsFileUpload bool
    SupportsMentions  bool
    MaxMessageSize    int
}
```

**插件机制：** 每个 Channel 是独立的 plugin package，通过 `plugin.Plugin` 接口注册。

### 3.2 Native Brain 接口（原生智能层）

```go
// brain.go
// Native Brain 是独立于 Worker 的智能层，不执行任务，只做消息预处理

type BrainInput struct {
    Message    Message
    SessionCtx *SessionContext  // 历史会话上下文
}

type BrainOutput struct {
    Intent     IntentResult       // 意图分类结果
    Guard     GuardResult        // WAF/安全检查结果
    RoutedTo  string             // 路由目标 worker kind
    Enhanced  *Message           // 增强后的消息（含上下文）
    Blocked   bool               // 是否被拦截
    BlockReason string           // 拦截原因
}

type IntentResult struct {
    Kind    string   // "code_gen" | "chat" | "admin" | "unknown"
    Conf    float64  // 置信度 0-1
    Params  map[string]interface{}  // 提取的参数
}

type GuardResult struct {
    Passed   bool
    Violations []string  // 违规项列表
    Level    string     // "block" | "warn" | "pass"
}

type NativeBrain interface {
    // Process 是同步入口，返回 BrainOutput
    Process(ctx context.Context, input BrainInput) (BrainOutput, error)
    // Stream 支持流式处理（可选）
    Stream(ctx context.Context, input BrainInput, ch chan<- BrainOutput) error
}

// 预置 Brain 实现
var BuiltinBrains = []NativeBrain{
    &LLMBrain{},        // LLM 驱动的意图识别 + WAF
    &RuleBrain{},       // 规则引擎（白名单/黑名单）
    &KeywordBrain{},     // 关键词路由
}
```

### 3.3 Worker 接口（执行抽象）

```go
// worker.go
type Task struct {
    ID          string
    SessionID   string
    Prompt      string
    Model       string
    MaxTokens   int
    Streaming   bool
    SystemPrompt string
    Env         map[string]string
}

type Result struct {
    TaskID      string
    Output      string
    Error       error
    ExitCode    int
    Usage       *UsageStats  // token 消耗
    Metrics     MetricsMap   // 执行指标
}

type Worker interface {
    Kind() string              // "claude-code" | "open-code"
    Run(ctx context.Context, task Task) (Result, error)
    Stream(ctx context.Context, task Task, ch chan<- Result) error
    Abort(taskID string) error
    Status() WorkerStatus
}

type WorkerStatus struct {
    Running   int      // 当前运行任务数
    MaxConcur int      // 最大并发
    Idle      bool     // 是否空闲
}
```

**子进程守护：**

```go
// supervisor.go
type Supervisor struct {
    workers    map[string]Worker
    policy     RestartPolicy
    maxRestarts int
    restartCooling time.Duration
}

type RestartPolicy int
const (
    RestartNever RestartPolicy = iota
    RestartOnFailure
    RestartAlways
    RestartBackoff  // 指数退避
)
```

### 3.4 Session 统一管理

```go
// session.go
type Session struct {
    ID          string
    ChannelID   string      // 来源渠道
    UserID      string      // 用户标识
    WorkerKind  string      // 当前绑定的 worker 类型
    State       SessionState
    BrainCtx    *BrainContext  // Brain 中间结果传递
    CreatedAt   time.Time
    UpdatedAt   time.Time
    TTL         time.Duration // 会话超时
}

type SessionState int
const (
    SessionActive  SessionState = iota
    SessionWaiting   // 等待 worker 响应
    SessionDone
    SessionExpired
)

type SessionManager interface {
    Create(s Session) error
    Get(id string) (*Session, error)
    Update(s *Session) error
    Delete(id string) error
    ListByUser(userID string) ([]*Session, error)
    // 统一超时控制
    ExpireAfter(d time.Duration)
}
```

### 3.5 Message Storage（插件化）

```go
// storage.go
type MessageStore interface {
    // 存储请求-响应对
    Save(req RequestRecord, resp ResponseRecord) error
    // 按 session 检索历史
    Query(sessions []string, limit int) ([]*MessagePair, error)
    // 全文搜索
    Search(query string, limit int) ([]*MessagePair, error)
}

// 内置实现：SQLite + 可选插件：PG / Redis / S3
type SQLiteStore struct { db *sql.DB }
type PostgresStore struct { conn *pgx.Conn }
type S3Store struct { bucket string }
```

**插件注册：**

```go
// plugin/plugin.go
type Plugin interface {
    Kind() string
    Init(cfg map[string]interface{}) error
}

func RegisterChannel(p Plugin)    { /* ... */ }
func RegisterWorker(p Plugin)     { /* ... */ }
func RegisterBrain(p Plugin)      { /* ... */ }
func RegisterStorage(p Plugin)    { /* ... */ }
```

---

## 四、消息流

```
用户消息 (Feishu/Slack/WS)
    │
    ▼
Channel Adapter (平台消息 → 统一 Message)
    │
    ▼
Native Brain (意图分类 → WAF → 路由决策)
    │  ├─ Intent: code_gen → Routing: claude-code
    │  ├─ Intent: chat     → Routing: open-code  
    │  └─ Intent: admin   → Routing: 内置命令
    │
    ▼
[Blocked?] ─Yes→ 返回拦截原因给用户
    │
    No
    │
    ▼
Session Manager (获取/创建 Session)
    │
    ▼
Worker (ClaudeCode / OpenCode)
    │
    ▼
Message Storage (存储请求-响应对)
    │
    ▼
Response → Channel Adapter → 用户
```

---

## 五、模块目录结构

```
hotplex/
├── cmd/
│   └── hotplexd/
│       └── main.go           # daemon 入口，依赖注入
├── internal/
│   ├── session/              # 统一 Session 管理
│   │   ├── manager.go
│   │   └── store.go
│   ├── supervisor/           # Worker 子进程守护
│   │   ├── pool.go
│   │   └── restart.go
│   └── config/               # 配置加载 (Viper/YAML)
├── pkg/
│   ├── channel/             # Channel 插件实现
│   │   ├── channel.go      # 接口定义
│   │   ├── feishu/
│   │   ├── slack/
│   │   └── ws/
│   ├── worker/              # Worker 插件实现
│   │   ├── worker.go       # 接口定义
│   │   ├── claude_code/
│   │   └── open_code/
│   ├── brain/               # Native Brain 实现
│   │   ├── brain.go        # 接口定义 + 编排器
│   │   ├── llm_brain.go   # LLM 驱动的 Brain
│   │   ├── rule_brain.go  # 规则引擎 Brain
│   │   └── keyword_brain.go
│   ├── storage/             # Storage 插件实现
│   │   ├── storage.go      # 接口定义
│   │   ├── sqlite/
│   │   ├── postgres/
│   │   └── redis/
│   ├── provider/            # LLM Provider 适配
│   │   ├── provider.go
│   │   ├── anthropic/
│   │   └── openai/
│   └── plugin/              # 插件注册表
│       └── registry.go
├── plugin/                  # 外部插件目录 (动态加载)
│   └── README.md            # 插件开发 SDK
├── server/                  # HTTP/WS API Server
│   └── server.go
└── docs/
    └── v1-arch/
        └── README.md
```

---

## 六、实现优先级

### Phase 1（v1.0.0 核心，MVP）

| 序号 | 模块 | 交付物 |
|------|------|--------|
| 1 | Channel 抽象层 | 接口定义 + Feishu 适配器（从现有代码迁移） |
| 2 | Native Brain 层 | LLM Brain（意图分类 + WAF）+ 规则 Brain |
| 3 | Worker 抽象层 | ClaudeCode Worker（现有 runner 逻辑重构） |
| 4 | Session Manager | 统一会话创建/查询/超时 |
| 5 | SQLite Storage | 请求-响应对的持久化 |
| 6 | 配置重构 | YAML 配置 + Viper，移除 flag 耦合 |

### Phase 2（v1.1）

- OpenCode Worker
- Slack / WS Channel 适配器
- Brain 中间件链（Context Enricher + Response Formatter）
- Postgres Storage 插件

### Phase 3（v1.2+）

- 多 Worker 协同（子任务拆分 + 结果聚合）
- 定时任务调度（Cron + 延时任务）
- Redis / S3 Storage 插件
- 动态插件加载（.so / go plugin）

---

## 七、关键设计决策

### 决策 1：Native Brain vs Worker 职责边界

```
Channel  →  Native Brain  →  Session Manager  →  Worker  →  Provider
              ↓ (预处理)                            ↓ (执行)
           拦截/路由                              输出
```

**结论：Native Brain 负责"思考"，Worker 负责"执行"。**  
Native Brain 不调用 Provider（除了 LLM Brain 做意图分类），Worker 调用 Provider 执行任务。

### 决策 2：子进程 vs API 调用？

```
ClaudeCode Worker  →  subprocess (本地进程，--print 模式)
OpenCode Worker   →  subprocess (本地进程)
Native Brain      →  LLM API (Anthropic/OpenAI) 做意图分类
```

**结论：Claude Code / OpenCode 必须通过子进程执行（它们的 tool use 能力依赖本地 CLI）。**  
Native Brain 的 LLM 调用走标准 Provider 接口。

### 决策 3：Session 存储在哪里？

**结论：Session 存储在 Memory（内存）+ SQLite（持久化）。**  
- 热数据：内存 Map（LRU cache）  
- 冷数据：SQLite（重启后可恢复）  
- 扩展：Postgres 插件（多实例部署）

### 决策 4：向后兼容

v1.0.0 breaking change，通过 `hotplex legacy` 子命令兼容旧版配置文件。

---

## 八、参考实现

- **OpenClaw**：Workspace-as-Kernel 理念，Agent 作为独立进程
- **LangChain Agent Runtime**：Provider → Tool → Agent 分层
- **Mastra**：Structured Agent + Task 抽象
- **Go Plugin System**：`plugin` 标准库 + `go-plugin` (HashiCorp)

---

*方案版本：v0.2 | 状态：草稿，基于黄飞虹反馈修正 Native Brain 定位*
