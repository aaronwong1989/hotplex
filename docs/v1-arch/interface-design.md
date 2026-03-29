# HotPlex v1.0.0 接口设计

> 版本：v1.0  
> 日期：2026-03-29  
> 状态：完整接口定义，可指导实际开发

---

## 1. 概述

本文档定义 HotPlex v1.0.0 所有核心接口。每个接口包含：
- 方法签名
- 核心数据结构
- 错误类型定义
- 实现注意事项

---

## 2. 包结构总览

```
pkg/
├── channel/          # 平台通道抽象
│   └── channel.go   # 接口 + 消息类型
├── brain/           # 原生智能层
│   └── brain.go     # 接口 + 编排器
├── worker/          # 执行器抽象
│   └── worker.go    # 接口 + 任务类型
├── session/          # 会话管理
│   └── session.go    # 接口 + 会话类型
├── supervisor/       # 进程守护
│   └── supervisor.go # 接口 + 策略
├── storage/          # 消息存储
│   └── storage.go   # 接口 + 记录类型
├── provider/         # LLM 提供商
│   └── provider.go  # 接口 + 请求响应
└── plugin/           # 插件系统
    └── plugin.go    # 注册机制
```

---

## 3. Channel 接口 (`pkg/channel/channel.go`)

### 3.1 核心消息结构

```go
package channel

import (
    "context"
    "time"
)

// Message 是跨平台统一消息格式
type Message struct {
    // 唯一标识
    ID string
    
    // 关联会话（由 SessionManager 生成）
    SessionID string
    
    // 渠道信息
    ChannelID   string // "feishu" | "slack" | "ws"
    ChannelName string // 友好名称
    
    // 发送者信息
    UserID   string
    UserName string
    
    // 消息内容
    Content  string    // 解析后的文本内容
    RawContent map[string]interface{} // 平台原生消息体（未解析）
    
    // 消息类型
    MsgType MessageType // "text" | "image" | "file" | "interactive"
    
    // 元数据
    Timestamp time.Time
    Metadata  map[string]string // 扩展字段
    
    // 回复上下文（用于 threading）
    ReplyTo   string    // 被回复的消息 ID
    ThreadID  string    // 话题 ID
}

// MessageType 消息类型枚举
type MessageType string

const (
    MessageTypeText        MessageType = "text"
    MessageTypeImage       MessageType = "image"
    MessageTypeFile        MessageType = "file"
    MessageTypeAudio      MessageType = "audio"
    MessageTypeVideo      MessageType = "video"
    MessageTypeInteractive MessageType = "interactive"
    MessageTypeUnknown     MessageType = "unknown"
)

// PlatformCapability 平台能力描述
type PlatformCapability struct {
    SupportsMarkdown   bool // 是否支持 Markdown 渲染
    SupportsStreaming  bool // 是否支持流式输出
    SupportsFileUpload bool // 是否支持文件上传
    SupportsMentions   bool // 是否支持 @ 提及
    SupportsThreads    bool // 是否支持话题/thread
    SupportsReactions  bool // 是否支持表情反应
    SupportsBlocks     bool // 是否支持 Block Kit / 卡片
    MaxMessageSize     int  // 最大消息字节数
    MaxFileSize        int  // 最大文件字节数
}

// Response 发送响应
type Response struct {
    MessageID string    // 平台消息 ID
    Timestamp time.Time
    Error     *SendError
}

// SendError 发送失败错误
type SendError struct {
    Code    string // "RATE_LIMIT" | "MSG_TOO_LONG" | "CHANNEL_ERROR"
    Message string
    Retryable bool // 是否可重试
}
```

### 3.2 Channel 接口定义

```go
// Channel 平台通道接口
type Channel interface {
    // 元信息
    Name() string // 唯一名称："feishu" | "slack" | "ws"
    Kind() string // 实现类型："feishu" | "slack" | "websocket"
    
    // 平台能力
    Capability() PlatformCapability
    
    // 生命周期
    Initialize(ctx context.Context, cfg *Config) error
    Start(ctx context.Context) error  // 启动消息监听
    Stop(ctx context.Context) error   // 优雅停止
    
    // 消息发送（必须线程安全）
    Send(ctx context.Context, resp Response) error
    SendRaw(ctx context.Context, raw []byte) error
    
    // 消息编辑/删除
    Update(ctx context.Context, msgID string, content string) error
    Delete(ctx context.Context, msgID string) error
    
    // 交互（可选）
    Reply(ctx context.Context, msgID string, content string) error
    React(ctx context.Context, msgID string, emoji string) error
    
    // 状态
    IsConnected() bool
    Health(ctx context.Context) *HealthStatus
}

// Config Channel 配置（平台特定）
type Config struct {
    Common struct {
        Timeout time.Duration
        Retry   RetryPolicy
    }
    // 平台特定配置（JSON）
    Platform map[string]interface{}
}

// RetryPolicy 重试策略
type RetryPolicy struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay    time.Duration
    BackoffMultiplier float64
}

// HealthStatus 健康状态
type HealthStatus struct {
    Status    string            // "healthy" | "degraded" | "down"
    Latency   time.Duration
    ErrorMsg  string
    Metadata  map[string]string
}
```

### 3.3 事件回调接口

```go
// EventHandler 消息事件处理
type EventHandler interface {
    OnMessage(ctx context.Context, msg Message) error
    OnError(err error, msg *Message) error
    OnConnect(ctx context.Context, channel Channel) error
    OnDisconnect(ctx context.Context, channel Channel, reason error) error
}

// MessageHandlerFunc 函数式处理器
type MessageHandlerFunc func(ctx context.Context, msg Message) error

func (f MessageHandlerFunc) OnMessage(ctx context.Context, msg Message) error {
    return f(ctx, msg)
}

// MiddlewareFunc Channel 中间件
type MiddlewareFunc func(next EventHandler) EventHandler

// 常用中间件
func WithLogging(next EventHandler) EventHandler
func WithMetrics(next EventHandler) EventHandler
func WithRateLimit(rps int) MiddlewareFunc
func WithAuth(verify func(msg Message) bool) MiddlewareFunc
```

---

## 4. Brain 接口 (`pkg/brain/brain.go`)

### 4.1 Brain 输入输出

```go
package brain

import (
    "context"
    "time"
)

// BrainInput Brain 处理输入
type BrainInput struct {
    // 原始消息
    Message *channel.Message
    
    // 会话上下文（用于上下文增强）
    SessionCtx *SessionContext
    
    // 请求级别配置
    Config *BrainConfig
}

// SessionContext 会话上下文
type SessionContext struct {
    SessionID    string
    UserID       string
    ChannelID    string
    
    // 历史消息（用于上下文补全）
    History []*channel.Message
    
    // 累积状态
    State map[string]interface{}
    
    // 元数据
    CreatedAt time.Time
    UpdatedAt time.Time
}

// BrainConfig Brain 配置
type BrainConfig struct {
    // 意图分类模型
    IntentModel string // "claude-sonnet-4" | "gpt-4o"
    
    // WAF 级别
    WAFLevel string // "strict" | "moderate" | "permissive"
    
    // 上下文窗口大小
    ContextWindow int // 最大历史消息条数
    
    // 超时
    Timeout time.Duration
}

// BrainOutput Brain 处理输出
type BrainOutput struct {
    // 意图识别结果
    Intent IntentResult
    
    // 安全检查结果
    Guard GuardResult
    
    // 路由决策
    RoutedTo string // Worker kind: "claude-code" | "open-code" | "builtin"
    
    // 增强后的消息（可选）
    Enhanced *channel.Message
    
    // 拦截决策
    Blocked      bool
    BlockReason  string
    BlockCode    string // 错误码
    
    // 执行指标
    Latency time.Duration
    Usage   *UsageStats // Token 消耗
}

// IntentResult 意图识别结果
type IntentResult struct {
    // 意图类型
    Kind IntentKind
    
    // 置信度 0.0-1.0
    Confidence float64
    
    // 提取的参数
    Params map[string]interface{}
    
    // 原始分类结果（调试用）
    RawOutput string
}

// IntentKind 意图类型枚举
type IntentKind string

const (
    IntentCodeGen   IntentKind = "code_gen"    // 代码生成/修改
    IntentChat      IntentKind = "chat"         // 问答对话
    IntentAdmin     IntentKind = "admin"        // 管理命令
    IntentCron      IntentKind = "cron"         // 定时任务
    IntentSystem    IntentKind = "system"       // 系统命令
    IntentUnknown   IntentKind = "unknown"      // 未知
)

// GuardResult 安全检查结果
type GuardResult struct {
    Passed      bool
    Level       GuardLevel    // "block" | "warn" | "pass"
    Violations  []Violation   // 违规项
    
    // 分类统计
    TotalChecks int
    PassedChecks int
}

// GuardLevel 安全级别
type GuardLevel string

const (
    GuardBlock GuardLevel = "block" // 直接拦截
    GuardWarn  GuardLevel = "warn"  // 警告但放行
    GuardPass  GuardLevel = "pass"  // 全部通过
)

// Violation 违规项
type Violation struct {
    RuleID   string // 规则 ID
    RuleName string // 规则名称
    Severity string // "critical" | "high" | "medium" | "low"
    Message  string // 违规描述
    Matched  string // 匹配到的内容
}

// UsageStats Token 消耗
type UsageStats struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
    CacheStats   *CacheStats // 缓存命中统计
}

// CacheStats 缓存统计
type CacheStats struct {
    CacheCreationTokens int
    CacheHitTokens       int
}
```

### 4.2 Brain 接口定义

```go
// NativeBrain 原生智能层接口
// Native Brain 不作为 Worker，是独立的智能处理层
type NativeBrain interface {
    // Kind 返回 Brain 类型
    Kind() string // "llm" | "rule" | "keyword"
    
    // Process 同步处理入口
    Process(ctx context.Context, input BrainInput) (BrainOutput, error)
    
    // Stream 流式处理（可选实现）
    Stream(ctx context.Context, input BrainInput, output chan<- BrainOutput) error
    
    // Initialize 初始化
    Initialize(ctx context.Context, cfg *BrainConfig) error
    
    // Health 健康检查
    Health(ctx context.Context) error
    
    // Close 清理资源
    Close() error
}

// BrainRegistry Brain 注册表
type BrainRegistry struct {
    brains map[string]NativeBrain
    mu     sync.RWMutex
}

func NewBrainRegistry() *BrainRegistry

func (r *BrainRegistry) Register(kind string, brain NativeBrain) error

func (r *BrainRegistry) Get(kind string) (NativeBrain, error)

func (r *BrainRegistry) List() []string

// BrainFactory Brain 工厂
type BrainFactory func(kind string, cfg *BrainConfig) (NativeBrain, error)

// 内置 Brain 实现
var BuiltinBrains = map[string]BrainFactory{
    "llm":     NewLLMBrain,
    "rule":    NewRuleBrain,
    "keyword": NewKeywordBrain,
}
```

### 4.3 LLM Brain 实现

```go
// LLMBrain 基于 LLM 的 Brain 实现
type LLMBrain struct {
    provider  provider.LLMProvider
    classifier *IntentClassifier
    waf       *WAFChecker
    enricher  *ContextEnricher
    
    cfg *BrainConfig
}

// IntentClassifier 意图分类器
type IntentClassifier struct {
    provider provider.LLMProvider
    model   string
    prompt  string // 分类提示词模板
}

// Classify 执行意图分类
func (c *IntentClassifier) Classify(ctx context.Context, msg string) (*IntentResult, error)

// WAFChecker WAF 检查器
type WAFChecker struct {
    rules    []WAFRule
    patterns []*regexp.Regexp
}

// Check 执行 WAF 检查
func (w *WAFChecker) Check(ctx context.Context, msg string) (*GuardResult, error)

// ContextEnricher 上下文增强器
type ContextEnricher struct {
    maxHistory int
    summarizer *MessageSummarizer
}

// Enrich 增强上下文
func (e *ContextEnricher) Enrich(ctx context.Context, history []*channel.Message) (*channel.Message, error)

// Process 实现 NativeBrain 接口
func (b *LLMBrain) Process(ctx context.Context, input BrainInput) (BrainOutput, error) {
    // 1. 意图分类
    intent, err := b.classifier.Classify(ctx, input.Message.Content)
    if err != nil {
        return BrainOutput{}, fmt.Errorf("intent classification failed: %w", err)
    }
    
    // 2. WAF 检查
    guard, err := b.waf.Check(ctx, input.Message.Content)
    if err != nil {
        return BrainOutput{}, fmt.Errorf("waf check failed: %w", err)
    }
    
    // 3. 路由决策
    routedTo := b.route(intent)
    
    // 4. 上下文增强（仅对需要长期记忆的意图）
    var enhanced *channel.Message
    if b.needsEnrichment(intent.Kind) {
        enhanced, err = b.enricher.Enrich(ctx, input.SessionCtx.History)
        if err != nil {
            // 增强失败不影响主流程
            enhanced = input.Message
        }
    }
    
    return BrainOutput{
        Intent:    *intent,
        Guard:     *guard,
        RoutedTo:  routedTo,
        Enhanced:  enhanced,
        Blocked:   guard.Level == GuardBlock,
        BlockReason: b.buildBlockReason(guard),
    }, nil
}

// route 根据意图返回路由目标
func (b *LLMBrain) route(intent *IntentResult) string {
    switch intent.Kind {
    case IntentCodeGen:
        return "claude-code"
    case IntentChat:
        return "open-code"
    case IntentAdmin:
        return "builtin"
    case IntentCron:
        return "claude-code"
    default:
        return "claude-code" // 默认走 Claude Code
    }
}
```

### 4.4 Rule Brain 实现

```go
// RuleBrain 基于规则的 Brain 实现
type RuleBrain struct {
    intentRules  []IntentRule
    wafRules     []WAFFRule
    routerules   []RouteRule
}

// IntentRule 意图规则
type IntentRule struct {
    Pattern *regexp.Regexp
    Intent  IntentKind
    Params  map[string]string // 提取参数映射
}

// WAFFRule WAF 规则
type WAFFRule struct {
    ID       string
    Pattern  *regexp.Regexp
    Level    GuardLevel
    Message  string
}

// RouteRule 路由规则
type RouteRule struct {
    Intent   IntentKind
    WorkerKind string
}

// Process 实现 NativeBrain 接口
func (b *RuleBrain) Process(ctx context.Context, input BrainInput) (BrainOutput, error) {
    // 1. 意图匹配
    var matchedIntent IntentResult
    for _, rule := range b.intentRules {
        if rule.Pattern.MatchString(input.Message.Content) {
            matchedIntent = IntentResult{
                Kind:       rule.Intent,
                Confidence: 1.0,
                Params:      b.extractParams(rule, input.Message.Content),
            }
            break
        }
    }
    if matchedIntent.Kind == "" {
        matchedIntent = IntentResult{Kind: IntentUnknown, Confidence: 0.0}
    }
    
    // 2. WAF 检查
    var violations []Violation
    for _, rule := range b.wafRules {
        if rule.Pattern.MatchString(input.Message.Content) {
            violations = append(violations, Violation{
                RuleID:   rule.ID,
                RuleName: rule.ID,
                Severity: "high",
                Message:  rule.Message,
                Matched:  rule.Pattern.FindString(input.Message.Content),
            })
        }
    }
    
    guard := GuardResult{Passed: len(violations) == 0}
    if len(violations) > 0 {
        guard.Level = GuardBlock
        guard.Violations = violations
    }
    
    // 3. 路由
    routedTo := "claude-code"
    for _, rule := range b.routeRules {
        if rule.Intent == matchedIntent.Kind {
            routedTo = rule.WorkerKind
            break
        }
    }
    
    return BrainOutput{
        Intent:     matchedIntent,
        Guard:      guard,
        RoutedTo:   routedTo,
        Blocked:    guard.Level == GuardBlock,
        BlockReason: b.buildBlockReason(guard),
    }, nil
}
```

---

## 5. Worker 接口 (`pkg/worker/worker.go`)

### 5.1 任务与结果

```go
package worker

import (
    "context"
    "time"
)

// Task 任务描述
type Task struct {
    // 任务标识
    ID string
    
    // 会话关联
    SessionID string
    
    // 消息内容
    Prompt      string
    SystemPrompt string
    
    // 模型配置
    Model     string // "claude-sonnet-4-20250514" | "gpt-4o"
    MaxTokens int    // 最大输出 Token 数
    
    // 执行配置
    Streaming bool           // 是否流式输出
    Timeout   time.Duration  // 超时时间
    Retry     RetryConfig    // 重试配置
    
    // 环境变量
    Env map[string]string
    
    // 工具配置
    Tools []ToolConfig
    
    // 附加文件上下文
    Attachments []Attachment
}

// RetryConfig 重试配置
type RetryConfig struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay    time.Duration
    RetryableErrors []string // 可重试的错误码
}

// ToolConfig 工具配置
type ToolConfig struct {
    Name    string
    Enabled bool
    Params  map[string]interface{}
}

// Attachment 附件
type Attachment struct {
    Type      string // "file" | "directory" | "url"
    Path      string // 文件路径或 URL
    Content   string // 文件内容（可选，避免重复读取）
    MimeType  string
}

// Result 任务执行结果
type Result struct {
    // 任务标识
    TaskID string
    
    // 执行状态
    Status ResultStatus
    
    // 输出内容
    Output    string
    StreamCh  chan string // 流式输出 channel
    
    // 错误信息
    Error     *TaskError
    
    // 执行统计
    ExitCode   int
    StartTime  time.Time
    EndTime    time.Time
    Duration   time.Duration
    
    // Token 消耗
    Usage *UsageStats
    
    // 执行指标
    Metrics MetricsMap
}

// ResultStatus 结果状态
type ResultStatus string

const (
    ResultStatusSuccess ResultStatus = "success"
    ResultStatusFailed  ResultStatus = "failed"
    ResultStatusTimeout ResultStatus = "timeout"
    ResultStatusAborted ResultStatus = "aborted"
)

// TaskError 任务错误
type TaskError struct {
    Code    string // "EXEC_FAILED" | "TIMEOUT" | "ABORTED" | "TOOL_ERROR"
    Message string
    Cause   error  // 原始错误
    Stack   string // 错误堆栈（调试用）
}

// UsageStats Token 消耗统计
type UsageStats struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
    
    // 缓存统计
    CacheCreationTokens int64
    CacheHitTokens      int64
    
    // 计费统计
    Cost float64 // 美元
}

// MetricsMap 执行指标
type MetricsMap map[string]interface{}

// 常用指标
var DefaultMetrics = []string{
    "first_token_latency",
    "tokens_per_second",
    "time_to_first_chunk",
    "total_chunks",
}
```

### 5.2 Worker 接口定义

```go
// Worker 执行器接口
type Worker interface {
    // 元信息
    Kind() string // "claude-code" | "open-code"
    Name() string // 友好名称
    
    // 生命周期
    Initialize(ctx context.Context, cfg *WorkerConfig) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // 任务执行
    Run(ctx context.Context, task Task) (Result, error)
    Stream(ctx context.Context, task Task) (<-chan Result, error) // 流式
    
    // 任务控制
    Abort(taskID string) error
    AbortAll() error
    
    // 状态查询
    Status() WorkerStatus
    Stats() WorkerStats
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
    // 并发控制
    MaxConcurrent int // 最大并发任务数
    MaxQueueSize int // 队列最大长度
    
    // 执行配置
    Timeout      time.Duration
    IdleTimeout  time.Duration // 空闲超时，触发进程回收
    
    // 重试配置
    Retry RetryConfig
    
    // 进程配置
    ProcessConfig *ProcessConfig
    
    // 环境变量
    Env map[string]string
    
    // 插件配置
    PluginDir string
}

// ProcessConfig 进程配置
type ProcessConfig struct {
    // CLI 路径
    BinPath string
    
    // CLI 参数
    Args []string
    
    // 工作目录
    WorkDir string
    
    // 用户/组（安全）
    UID int
    GID int
    
    // 资源限制
    MemoryLimitBytes int64
    CPULimitPercent  int
    DiskLimitBytes   int64
    
    // 安全配置
    ReadonlyFS     bool
    AllowedDirs    []string // 白名单目录
    BlockedEnvVars []string // 黑名单环境变量
}

// WorkerStatus Worker 状态
type WorkerStatus struct {
    State        WorkerState // "idle" | "busy" | "draining" | "down"
    RunningTasks int          // 当前运行任务数
    QueueLength  int          // 队列长度
    Uptime      time.Duration
    
    // 进程状态
    ProcessInfo []ProcessInfo
}

// WorkerState Worker 状态枚举
type WorkerState string

const (
    WorkerStateIdle     WorkerState = "idle"
    WorkerStateBusy     WorkerState = "busy"
    WorkerStateDraining WorkerState = "draining"
    WorkerStateDown     WorkerState = "down"
)

// ProcessInfo 进程信息
type ProcessInfo struct {
    PID       int
    SessionID string
    TaskID    string
    StartedAt time.Time
    CPUPercent float64
    MemoryMB   int64
}

// WorkerStats Worker 统计
type WorkerStats struct {
    TotalTasks   int64
    SuccessTasks int64
    FailedTasks  int64
    
    AvgLatency  time.Duration
    AvgTokens   int
    
    TotalTokens int64
    
    LastTaskAt time.Time
}
```

### 5.3 ClaudeCode Worker 实现

```go
// ClaudeCodeWorker Claude Code Worker 实现
type ClaudeCodeWorker struct {
    pool     *ProcessPool   // 进程池
    config   *WorkerConfig
    runner   *CLIRunner     // CLI 执行器
    parser   *OutputParser  // 输出解析
    mu       sync.RWMutex
    status   WorkerStatus
    stats    WorkerStats
}

// ProcessPool 进程池
type ProcessPool struct {
    processes chan *Process
    config    *PoolConfig
    factory   ProcessFactory
}

// Process CLI 进程
type Process struct {
    pid    int
    state  ProcessState
    task   *Task
    stdout io.Reader
    stderr io.Reader
    stdin  io.Writer
}

// ProcessFactory 进程工厂
type ProcessFactory func(cfg *WorkerConfig) (*Process, error)

// NewClaudeCodeWorker 创建 Claude Code Worker
func NewClaudeCodeWorker(cfg *WorkerConfig) (*ClaudeCodeWorker, error) {
    w := &ClaudeCodeWorker{
        config: cfg,
        runner: NewCLIRunner(cfg),
        parser: NewOutputParser(),
    }
    
    // 初始化进程池
    pool, err := NewProcessPool(&PoolConfig{
        MinProcesses: 1,
        MaxProcesses: cfg.MaxConcurrent,
        MaxIdleTime:  cfg.IdleTimeout,
        Factory:      w.createProcess,
    })
    if err != nil {
        return nil, err
    }
    w.pool = pool
    
    return w, nil
}

// Run 执行任务
func (w *ClaudeCodeWorker) Run(ctx context.Context, task Task) (Result, error) {
    // 获取空闲进程
    proc, err := w.pool.Acquire(ctx)
    if err != nil {
        return Result{Status: ResultStatusFailed, Error: &TaskError{
            Code:    "POOL_EXHAUSTED",
            Message: fmt.Sprintf("failed to acquire process: %v", err),
        }}, err
    }
    defer w.pool.Release(proc)
    
    // 执行任务
    return w.runner.Run(ctx, proc, task)
}

// Stream 流式执行
func (w *ClaudeCodeWorker) Stream(ctx context.Context, task Task) (<-chan Result, error) {
    outputCh := make(chan Result, 1)
    errCh := make(chan error, 1)
    
    go func() {
        defer close(outputCh)
        defer close(errCh)
        
        // 获取进程
        proc, err := w.pool.Acquire(ctx)
        if err != nil {
            errCh <- err
            return
        }
        defer w.pool.Release(proc)
        
        // 流式执行
        w.runner.RunStream(ctx, proc, task, outputCh)
    }()
    
    return outputCh, nil
}

// CLIRunner CLI 执行器
type CLIRunner struct {
    binaryPath string
    args       []string
    workDir    string
}

// Run 执行 CLI 命令
func (r *CLIRunner) Run(ctx context.Context, proc *Process, task Task) (Result, error) {
    // 构建命令
    cmd := r.buildCommand(task)
    
    // 启动进程
    if err := proc.Start(cmd); err != nil {
        return Result{Status: ResultStatusFailed, Error: &TaskError{
            Code:    "PROCESS_START_FAILED",
            Message: err.Error(),
        }}, err
    }
    
    // 等待完成
    done := make(chan struct{})
    go func() {
        proc.Wait()
        close(done)
    }()
    
    select {
    case <-ctx.Done():
        proc.Kill()
        return Result{Status: ResultStatusTimeout}, ctx.Err()
    case <-done:
        exitCode := proc.ExitCode()
        stdout, _ := io.ReadAll(proc.Stdout())
        stderr, _ := io.ReadAll(proc.Stderr())
        
        if exitCode != 0 {
            return Result{
                Status: ResultStatusFailed,
                Output: string(stdout),
                Error: &TaskError{
                    Code:    "EXEC_FAILED",
                    Message: string(stderr),
                },
                ExitCode: exitCode,
            }, nil
        }
        
        return Result{
            Status: ResultStatusSuccess,
            Output: string(stdout),
            ExitCode: 0,
        }, nil
    }
}

// buildCommand 构建 CLI 命令
func (r *CLIRunner) buildCommand(task Task) *exec.Cmd {
    args := []string{
        "--print",           // 输出模式
        "--output-format", "json",
        "--no-input",        // 无交互模式
    }
    
    // 添加模型
    if task.Model != "" {
        args = append(args, "--model", task.Model)
    }
    
    // 添加系统提示词
    if task.SystemPrompt != "" {
        args = append(args, "--system-prompt", task.SystemPrompt)
    }
    
    // 添加提示词
    args = append(args, task.Prompt)
    
    return exec.Command(r.binaryPath, args...)
}
```

---

## 6. Session 接口 (`pkg/session/session.go`)

### 6.1 会话结构

```go
package session

import (
    "context"
    "time"
)

// Session 会话
type Session struct {
    // 唯一标识
    ID string
    
    // 关联信息
    ChannelID   string // "feishu" | "slack" | "ws"
    ChannelName string
    UserID      string
    UserName    string
    
    // 状态
    State SessionState
    
    // 当前绑定的 Worker
    WorkerKind string // "claude-code" | "open-code"
    
    // Brain 上下文（用于中间结果传递）
    BrainCtx *BrainContext
    
    // 用户上下文（用户自定义数据）
    UserCtx map[string]interface{}
    
    // 时间
    CreatedAt time.Time
    UpdatedAt time.Time
    ExpiresAt time.Time // TTL 过期时间
    
    // 配置
    Config *SessionConfig
}

// SessionState 会话状态
type SessionState int

const (
    SessionStateActive   SessionState = iota // 活跃，可处理消息
    SessionStateWaiting                      // 等待 Worker 响应
    SessionStateDone                         // 已完成（正常结束）
    SessionStateExpired                      // 已过期
    SessionStateError                        // 错误状态
)

// String 实现 fmt.Stringer
func (s SessionState) String() string {
    switch s {
    case SessionStateActive:
        return "active"
    case SessionStateWaiting:
        return "waiting"
    case SessionStateDone:
        return "done"
    case SessionStateExpired:
        return "expired"
    case SessionStateError:
        return "error"
    default:
        return "unknown"
    }
}

// BrainContext Brain 处理上下文
type BrainContext struct {
    Intent      string            // 当前意图
    GuardPassed bool              // 安全检查是否通过
    Enhanced    string            // 增强后的内容
    HistorySummary string         // 历史消息摘要
    State       map[string]interface{} // 中间状态
}

// SessionConfig 会话配置
type SessionConfig struct {
    // TTL
    TTL time.Duration
    
    // 空闲超时
    MaxIdle time.Duration
    
    // 存储策略
    StorageKind string // "memory" | "sqlite" | "postgres"
    
    // Worker 配置
    DefaultWorker string
    MaxWorkers    int
    
    // 历史消息限制
    MaxHistoryMessages int
    MaxHistoryBytes    int64
}
```

### 6.2 SessionManager 接口

```go
// Manager 会话管理器
type Manager interface {
    // 创建会话
    Create(ctx context.Context, s *Session) error
    
    // 查询会话
    Get(ctx context.Context, id string) (*Session, error)
    GetByUser(ctx context.Context, userID string, channelID string) (*Session, error)
    
    // 更新会话
    Update(ctx context.Context, s *Session) error
    
    // 删除会话
    Delete(ctx context.Context, id string) error
    
    // 列表查询
    List(ctx context.Context, opts *ListOptions) ([]*Session, error)
    ListByChannel(ctx context.Context, channelID string) ([]*Session, error)
    ListByWorker(ctx context.Context, workerKind string) ([]*Session, error)
    
    // 状态操作
    SetState(ctx context.Context, id string, state SessionState) error
    Touch(ctx context.Context, id string) error // 更新 UpdatedAt
    
    // 批量操作
    DeleteExpired(ctx context.Context) (int, error)
    DeleteByUser(ctx context.Context, userID string) (int, error)
}

// ListOptions 列表查询选项
type ListOptions struct {
    State     SessionState
    WorkerKind string
    ChannelID string
    Limit     int
    Offset    int
    OrderBy   string // "created_at" | "updated_at"
    Ascending bool
}
```

### 6.3 Session 存储实现

```go
// MemoryStore 内存存储
type MemoryStore struct {
    sessions map[string]*Session
    index    *SessionIndex
    mu       sync.RWMutex
    lru      *lru.Cache[string, *Session]
}

// SessionIndex 会话索引
type SessionIndex struct {
    byUser    map[string]map[string]string // userID -> channelID -> sessionID
    byChannel map[string]map[string]string // channelID -> userID -> sessionID
    byWorker  map[string][]string         // workerKind -> []sessionID
    mu        sync.RWMutex
}

// NewMemoryStore 创建内存存储
func NewMemoryStore(maxSessions int) *MemoryStore {
    lruCache, _ := lru.New[string, *Session](maxSessions)
    return &MemoryStore{
        sessions: make(map[string]*Session),
        index:    NewSessionIndex(),
        lru:      lruCache,
    }
}

// Create 实现 Manager 接口
func (s *MemoryStore) Create(ctx context.Context, session *Session) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 生成 ID
    if session.ID == "" {
        session.ID = generateSessionID()
    }
    
    // 设置时间
    now := time.Now()
    session.CreatedAt = now
    session.UpdatedAt = now
    if session.ExpiresAt.IsZero() {
        session.ExpiresAt = now.Add(24 * time.Hour)
    }
    
    // 存储
    s.sessions[session.ID] = session
    s.lru.Add(session.ID, session)
    
    // 更新索引
    s.index.Add(session)
    
    return nil
}

// Get 实现 Manager 接口
func (s *MemoryStore) Get(ctx context.Context, id string) (*Session, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    session, ok := s.sessions[id]
    if !ok {
        return nil, ErrSessionNotFound
    }
    
    // LRU 更新
    s.lru.Get(id)
    
    return session, nil
}

// Update 实现 Manager 接口
func (s *MemoryStore) Update(ctx context.Context, session *Session) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, ok := s.sessions[session.ID]; !ok {
        return ErrSessionNotFound
    }
    
    session.UpdatedAt = time.Now()
    s.sessions[session.ID] = session
    s.lru.Add(session.ID, session)
    
    return nil
}

// DeleteExpired 删除过期会话
func (s *MemoryStore) DeleteExpired(ctx context.Context) (int, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    now := time.Now()
    var deleted []string
    
    for id, session := range s.sessions {
        if session.ExpiresAt.Before(now) {
            deleted = append(deleted, id)
            delete(s.sessions, id)
            s.lru.Remove(id)
            s.index.Remove(session)
        }
    }
    
    return len(deleted), nil
}
```

---

## 7. Supervisor 接口 (`pkg/supervisor/supervisor.go`)

### 7.1 Supervisor 结构

```go
package supervisor

// Supervisor 进程监督器
type Supervisor struct {
    workers map[string]WorkerHandle
    policy  RestartPolicy
    events  chan SupervisorEvent
    done    chan struct{}
    mu      sync.RWMutex
}

// WorkerHandle Worker 句柄
type WorkerHandle struct {
    Kind   string
    Worker worker.Worker
    state  WorkerHandleState
    restarts int
    lastStart time.Time
}

// WorkerHandleState Worker 状态
type WorkerHandleState int

const (
    HandleStateRunning WorkerHandleState = iota
    HandleStateRestarting
    HandleStateFailed
    HandleStateStopped
)

// RestartPolicy 重启策略
type RestartPolicy struct {
    Mode RestartMode
    // 指数退避
    MaxRestarts       int
    MaxRestartInterval time.Duration
    InitialInterval    time.Duration
}

// RestartMode 重启模式
type RestartMode string

const (
    RestartModeNever      RestartMode = "never"
    RestartModeOnFailure  RestartMode = "on-failure"
    RestartModeAlways     RestartMode = "always"
    RestartModeBackoff    RestartMode = "backoff"
)

// SupervisorEvent 监督事件
type SupervisorEvent struct {
    Type    EventType
    WorkerKind string
    PID     int
    Error   error
    Time    time.Time
}

// EventType 事件类型
type EventType string

const (
    EventWorkerStarted  EventType = "worker_started"
    EventWorkerStopped EventType = "worker_stopped"
    EventWorkerFailed  EventType = "worker_failed"
    EventWorkerRestart EventType = "worker_restart"
)
```

### 7.2 Supervisor 接口

```go
// Supervisor 监督器接口
type Supervisor interface {
    // 注册 Worker
    Register(kind string, w worker.Worker) error
    Unregister(kind string) error
    
    // 生命周期
    // 启动/停止
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // 状态
    State() SupervisorState
    ListWorkers() []string
    
    // 事件订阅
    Subscribe() (<-chan SupervisorEvent, func())
}

// SupervisorState 监督器状态
type SupervisorState struct {
    TotalWorkers int
    Running      int
    Failed       int
    Stopped      int
}
```

### 7.3 进程池

```go
// PoolConfig 进程池配置
type PoolConfig struct {
    MinProcesses int           // 最小进程数
    MaxProcesses int           // 最大进程数
    MaxIdleTime time.Duration  // 最大空闲时间
    AcquireTimeout time.Duration // 获取超时
}

// ProcessPool 进程池接口
type ProcessPool interface {
    Acquire(ctx context.Context) (*Process, error)
    Release(p *Process) error
    Stats() PoolStats
}

// PoolStats 进程池统计
type PoolStats struct {
    Active   int
    Idle     int
    Total    int
    Acquired int64 // 总获取次数
    Released int64 // 总释放次数
}
```

---

## 8. Storage 接口 (`pkg/storage/storage.go`)

### 8.1 存储记录

```go
package storage

import (
    "context"
    "time"
)

// MessageRecord 消息记录
type MessageRecord struct {
    // 标识
    ID        string
    SessionID string
    MessageID string // 平台消息 ID
    
    // 角色
    Role MessageRole // "user" | "assistant" | "system"
    
    // 内容
    Content   string
    RawContent map[string]interface{} // 原始消息
    
    // 元数据
    ChannelID  string
    UserID     string
    Timestamp  time.Time
    
    // 扩展
    Metadata map[string]string
    Tags     []string
}

// MessageRole 消息角色
type MessageRole string

const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
    RoleSystem    MessageRole = "system"
    RoleTool      MessageRole = "tool"
)

// MessagePair 请求-响应对
type MessagePair struct {
    Request  *MessageRecord
    Response *MessageRecord
    Latency  time.Duration
}

// SearchResult 搜索结果
type SearchResult struct {
    Records []*MessageRecord
    Total   int
    Score   float64 // 相关度分数
}
```

### 8.2 Storage 接口

```go
// Storage 消息存储接口
type Storage interface {
    // 写入
    Save(ctx context.Context, records ...*MessageRecord) error
    
    // 读取
    Get(ctx context.Context, id string) (*MessageRecord, error)
    GetBySession(ctx context.Context, sessionID string, limit int) ([]*MessageRecord, error)
    
    // 查询
    Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)
    List(ctx context.Context, opts *ListOptions) ([]*MessageRecord, error)
    
    // 更新
    Update(ctx context.Context, record *MessageRecord) error
    
    // 删除
    Delete(ctx context.Context, id string) error
    DeleteBySession(ctx context.Context, sessionID string) (int, error)
    
    // 统计
    Stats(ctx context.Context) (*StorageStats, error)
    
    // 生命周期
    Close() error
}

// SearchQuery 搜索查询
type SearchQuery struct {
    // 文本搜索
    Text string
    
    // 过滤条件
    SessionIDs []string
    UserIDs    []string
    ChannelIDs []string
    Roles      []MessageRole
    StartTime  time.Time
    EndTime    time.Time
    
    // 分页
    Limit  int
    Offset int
    
    // 排序
    OrderBy string // "timestamp" | "relevance"
    Ascending bool
    
    // 高级
    IncludeMetadata bool
    Tags []string
}

// ListOptions 列表选项
type ListOptions struct {
    SessionID string
    UserID    string
    ChannelID string
    Role      MessageRole
    Limit     int
    Offset    int
    StartTime time.Time
    EndTime   time.Time
}

// StorageStats 存储统计
type StorageStats struct {
    TotalRecords    int64
    TotalSessions   int64
    TotalUsers      int64
    TotalSizeBytes  int64
    OldestRecord    time.Time
    NewestRecord    time.Time
    ByChannel       map[string]int64
    ByRole          map[MessageRole]int64
}
```

### 8.3 SQLite 实现

```go
// SQLiteStorage SQLite 存储实现
type SQLiteStorage struct {
    db *sql.DB
    path string
}

// NewSQLiteStorage 创建 SQLite 存储
func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }
    
    // 初始化表
    if err := initSchema(db); err != nil {
        return nil, err
    }
    
    return &SQLiteStorage{db: db, path: path}, nil
}

// initSchema 初始化表结构
func initSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS messages (
        id TEXT PRIMARY KEY,
        session_id TEXT NOT NULL,
        message_id TEXT,
        role TEXT NOT NULL,
        content TEXT NOT NULL,
        raw_content TEXT,
        channel_id TEXT,
        user_id TEXT,
        timestamp DATETIME NOT NULL,
        metadata TEXT,
        tags TEXT
    );
    
    CREATE INDEX IF NOT EXISTS idx_session ON messages(session_id);
    CREATE INDEX IF NOT EXISTS idx_user ON messages(user_id);
    CREATE INDEX IF NOT EXISTS idx_timestamp ON messages(timestamp);
    CREATE INDEX IF NOT EXISTS idx_channel ON messages(channel_id);
    
    CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
        content, tags,
        content=messages,
        content_rowid=rowid
    );
    `
    _, err := db.Exec(schema)
    return err
}

// Save 实现 Storage 接口
func (s *SQLiteStorage) Save(ctx context.Context, records ...*MessageRecord) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO messages (id, session_id, message_id, role, content, raw_content,
                              channel_id, user_id, timestamp, metadata, tags)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, r := range records {
        rawJSON, _ := json.Marshal(r.RawContent)
        metaJSON, _ := json.Marshal(r.Metadata)
        tagsJSON, _ := json.Marshal(r.Tags)
        
        _, err := stmt.ExecContext(ctx,
            r.ID, r.SessionID, r.MessageID, r.Role, r.Content, rawJSON,
            r.ChannelID, r.UserID, r.Timestamp, metaJSON, tagsJSON,
        )
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// Search 实现 Storage 接口（使用 FTS5）
func (s *SQLiteStorage) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
    if query.Text != "" {
        return s.searchFTS(ctx, query)
    }
    return s.searchSQL(ctx, query)
}

func (s *SQLiteStorage) searchFTS(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
    sql := `
    SELECT m.*, fts.rank
    FROM messages m
    JOIN messages_fts fts ON m.rowid = fts.rowid
    WHERE messages_fts MATCH ?
    `
    args := []interface{}{query.Text}
    
    // 添加过滤条件
    sql, args = s.appendFilters(sql, query)
    
    sql += ` ORDER BY rank LIMIT ? OFFSET ?`
    args = append(args, query.Limit, query.Offset)
    
    rows, err := s.db.QueryContext(ctx, sql, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    return s.scanResults(rows)
}
```

---

## 9. Provider 接口 (`pkg/provider/provider.go`)

### 9.1 Provider 接口

```go
package provider

import (
    "context"
    "io"
)

// LLMProvider LLM 提供商接口
type LLMProvider interface {
    // 元信息
    Name() string // "anthropic" | "openai" | "siliconflow"
    Kind() string
    
    // 模型
    ListModels(ctx context.Context) ([]*Model, error)
    GetModel(modelID string) (*Model, error)
    
    // 生成
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Stream(ctx context.Context, req *GenerateRequest) (*StreamResponse, error)
    
    // 嵌入
    Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)
    
    // 生命周期
    Close() error
}

// Model 模型信息
type Model struct {
    ID          string
    Name        string
    Provider    string
    Kind        ModelKind // "chat" | "embedding" | "completion"
    
    // 限制
    MaxTokens    int
    ContextWindow int
    SupportsStreaming bool
    
    // 计费
    InputPrice  float64 // per 1M tokens
    OutputPrice float64
}

// ModelKind 模型类型
type ModelKind string

const (
    ModelKindChat       ModelKind = "chat"
    ModelKindEmbedding  ModelKind = "embedding"
    ModelKindCompletion ModelKind = "completion"
    ModelKindVision     ModelKind = "vision"
)

// GenerateRequest 生成请求
type GenerateRequest struct {
    Model string
    
    // 消息
    Messages []*Message
    
    // 参数
    Temperature float64
    TopP        float64
    MaxTokens   int
    Stop        []string
    
    // 流式
    Streaming bool
    
    // 系统
    SystemPrompt string
    
    // 工具
    Tools []ToolDef
    ToolChoice string
    
    // 扩展
    Extra map[string]interface{}
}

// Message 消息
type Message struct {
    Role    string // "user" | "assistant" | "system"
    Content string
    
    // 多模态
    MultiContent []ContentBlock
    
    // 工具调用
    ToolCalls []ToolCall
    ToolResult *ToolResult
}

// ContentBlock 内容块
type ContentBlock struct {
    Type string // "text" | "image" | "tool_result"
    
    Text string
    Image *ImageSource
    ToolResult *ToolResult
}

// ImageSource 图片源
type ImageSource struct {
    Type string // "url" | "base64"
    URL  string
    Data string // base64 data
}

// ToolDef 工具定义
type ToolDef struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
}

// ToolCall 工具调用
type ToolCall struct {
    ID   string
    Name string
    Args map[string]interface{}
}

// ToolResult 工具结果
type ToolResult struct {
    ToolID string
    Content string
    Error  error
}

// GenerateResponse 生成响应
type GenerateResponse struct {
    Message Message
    
    // 统计
    Usage   *Usage
    Metrics map[string]interface{}
    
    // 停止原因
    StopReason string
}

// Usage 使用量
type Usage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
    
    // 缓存
    CacheCreationInputTokens int64
    CacheHitInputTokens      int64
}

// StreamResponse 流式响应
type StreamResponse struct {
    Stream <-chan StreamChunk
    Usage  *Usage
}

// StreamChunk 流式块
type StreamChunk struct {
    Type string // "content_block" | "message_start" | "message_delta" | "message_stop"
    
    Content    string
    Delta      string
    Index      int
    
    Message    *Message
    Usage      *Usage
    StopReason string
}

// EmbedRequest 嵌入请求
type EmbedRequest struct {
    Model string
    Input []string
}

// EmbedResponse 嵌入响应
type EmbedResponse struct {
    Embeddings [][]float32
    Model      string
    Usage      *Usage
}
```

### 9.2 Anthropic Provider

```go
// AnthropicProvider Anthropic 提供商实现
type AnthropicProvider struct {
    apiKey   string
    baseURL  string
    client   *http.Client
    models   map[string]*Model
}

// NewAnthropicProvider 创建 Anthropic Provider
func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
    p := &AnthropicProvider{
        apiKey:  apiKey,
        baseURL: "https://api.anthropic.com",
        client: &http.Client{
            Timeout: 60 * time.Second,
        },
        models: map[string]*Model{
            "claude-sonnet-4-20250514": {
                ID:             "claude-sonnet-4-20250514",
                Name:           "Claude Sonnet 4",
                Provider:       "anthropic",
                Kind:           ModelKindChat,
                MaxTokens:      8192,
                ContextWindow:  200000,
                SupportsStreaming: true,
                InputPrice:     3.0,
                OutputPrice:    15.0,
            },
            "claude-opus-4-20250514": {
                ID:             "claude-opus-4-20250514",
                Name:           "Claude Opus 4",
                Provider:       "anthropic",
                Kind:           ModelKindChat,
                MaxTokens:      8192,
                ContextWindow:  200000,
                SupportsStreaming: true,
                InputPrice:     15.0,
                OutputPrice:    75.0,
            },
        },
    }
    return p, nil
}

// Generate 实现 LLMProvider 接口
func (p *AnthropicProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    // 构建请求
    apiReq := p.buildRequest(req)
    
    // 发送请求
    body, err := json.Marshal(apiReq)
    if err != nil {
        return nil, err
    }
    
    httpReq, _ := http.NewRequestWithContext(ctx, "POST",
        p.baseURL+"/v1/messages", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", p.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")
    
    httpResp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer httpResp.Body.Close()
    
    // 解析响应
    var apiResp anthropicResponse
    if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
        return nil, err
    }
    
    return p.parseResponse(&apiResp)
}

func (p *AnthropicProvider) buildRequest(req *GenerateRequest) map[string]interface{} {
    messages := make([]map[string]interface{}, len(req.Messages))
    for i, msg := range req.Messages {
        mc := map[string]interface{}{
            "role":    msg.Role,
            "content": msg.Content,
        }
        if len(msg.MultiContent) > 0 {
            mc["content"] = p.buildContentBlocks(msg.MultiContent)
        }
        messages[i] = mc
    }
    
    apiReq := map[string]interface{}{
        "model":           req.Model,
        "max_tokens":      req.MaxTokens,
        "messages":        messages,
        "temperature":     req.Temperature,
    }
    
    if req.SystemPrompt != "" {
        apiReq["system"] = req.SystemPrompt
    }
    
    if len(req.Tools) > 0 {
        apiReq["tools"] = req.Tools
    }
    
    return apiReq
}
```

---

## 10. 错误类型定义 (`pkg/errors/errors.go`)

### 10.1 错误码定义

```go
package errors

import (
    "errors"
    "fmt"
)

// 错误码
type Code string

const (
    // Channel 错误 (CH-*)
    ErrChannelNotFound      Code = "CH-001"
    ErrChannelDisconnected  Code = "CH-002"
    ErrChannelSendFailed    Code = "CH-003"
    ErrChannelAuthFailed    Code = "CH-004"
    ErrChannelRateLimited   Code = "CH-005"
    ErrChannelMsgTooLong    Code = "CH-006"
    
    // Brain 错误 (BR-*)
    ErrBrainInitFailed      Code = "BR-001"
    ErrBrainProcessFailed    Code = "BR-002"
    ErrBrainTimeout         Code = "BR-003"
    ErrBrainIntentFailed    Code = "BR-004"
    ErrBrainGuardBlocked    Code = "BR-005"
    
    // Worker 错误 (WK-*)
    ErrWorkerInitFailed     Code = "WK-001"
    ErrWorkerStartFailed    Code = "WK-002"
    ErrWorkerProcessFailed  Code = "WK-003"
    ErrWorkerTimeout       Code = "WK-004"
    ErrWorkerAbortFailed   Code = "WK-005"
    ErrWorkerPoolExhausted  Code = "WK-006"
    
    // Session 错误 (SE-*)
    ErrSessionNotFound      Code = "SE-001"
    ErrSessionCreateFailed  Code = "SE-002"
    ErrSessionUpdateFailed  Code = "SE-003"
    ErrSessionExpired       Code = "SE-004"
    ErrSessionFull         Code = "SE-005"
    
    // Storage 错误 (ST-*)
    ErrStorageInitFailed    Code = "ST-001"
    ErrStorageSaveFailed    Code = "ST-002"
    ErrStorageQueryFailed   Code = "ST-003"
    ErrStorageNotFound     Code = "ST-004"
    
    // Provider 错误 (PR-*)
    ErrProviderAuthFailed   Code = "PR-001"
    ErrProviderRateLimited  Code = "PR-002"
    ErrProviderQuotaExceeded Code = "PR-003"
    ErrProviderModelNotFound Code = "PR-004"
    ErrProviderRequestFailed Code = "PR-005"
    
    // 通用错误 (SY-*)
    ErrInvalidConfig        Code = "SY-001"
    ErrInvalidInput        Code = "SY-002"
    ErrInternalError       Code = "SY-003"
    ErrNotImplemented      Code = "SY-004"
    ErrTimeout             Code = "SY-005"
    ErrCancelled           Code = "SY-006"
)

// Error HotPlex 错误
type Error struct {
    Code    Code
    Message string
    Cause   error
    Details map[string]interface{}
    Stack   string
}

// Error 实现 error 接口
func (e *Error) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap
func (e *Error) Unwrap() error {
    return e.Cause
}

// WithCause 添加原因
func (e *Error) WithCause(err error) *Error {
    e.Cause = err
    e.Stack = string(debug.Stack())
    return e
}

// WithDetails 添加详情
func (e *Error) WithDetails(details map[string]interface{}) *Error {
    e.Details = details
    return e
}

// New 创建新错误
func New(code Code, msg string) *Error {
    return &Error{
        Code:    code,
        Message: msg,
        Stack:   string(debug.Stack()),
    }
}

// Wrap 包装错误
func Wrap(code Code, err error, msg string) *Error {
    return &Error{
        Code:    code,
        Message: msg,
        Cause:   err,
        Stack:   string(debug.Stack()),
    }
}

// Is 判断错误类型
func Is(err, target error) bool {
    return errors.Is(err, target)
}

// As 转换错误类型
func As(err error, target interface{}) bool {
    return errors.As(err, target)
}
```

---

## 11. 附录

### 11.1 接口依赖图

```
Channel
  ↓ (产生 Message)
Brain
  ↓ (产生 Task)
Session
  ↓ (管理状态)
Worker
  ↓ (调用 Provider)
Provider

Supervisor ← Worker (监督)
Storage ← 跨层 (存储消息)
```

### 11.2 线程安全要求

| 接口 | 线程安全 | 说明 |
|------|----------|------|
| Channel.Send | ✅ 必须 | 多 goroutine 并发调用 |
| SessionManager | ✅ 必须 | 多请求并发访问 |
| Worker.Run | ✅ 必须 | 并发任务执行 |
| Brain.Process | ✅ 必须 | 并发请求处理 |
| Storage | ✅ 必须 | 并发读写 |

### 11.3 上下文传播

所有接口的第一个参数都是 `context.Context`，用于：
- 超时控制
- 取消传播
- 链路追踪（traceID）
- 请求级别配置

```go
// 正确用法
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := worker.Run(ctx, task)

// 错误传递
if err != nil {
    return nil, errors.Wrap(ErrWorkerProcessFailed, err, "worker execution failed")
}
```

---

*文档版本：v1.0 | 最后更新：2026-03-29*
