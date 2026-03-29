# HotPlex v1.0.0 接口定义

> 版本：v1.0-final  
> 日期：2026-03-29  
> 状态：**已确认**

---

## 一、核心接口体系

### 1.1 接口全景图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HotPlex Runtime                                │
│                                                                             │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                    │
│  │   Channel   │────▶│    Brain    │────▶│  Supervisor │                    │
│  │  Interface  │     │  Interface  │     │  Interface  │                    │
│  └─────────────┘     └─────────────┘     └─────────────┘                    │
│         │                  │                  │                            │
│         │                  │                  │                            │
│         ▼                  ▼                  ▼                            │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌───────────┐ │
│  │   Worker    │     │   Session   │     │  Provider   │     │  Storage  │ │
│  │  Interface  │     │  Interface  │     │  Interface  │     │ Interface │ │
│  └─────────────┘     └─────────────┘     └─────────────┘     └───────────┘ │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Docker Plugin Interfaces                           │   │
│  │              ContainerPlugin  │  ContainerRegistry                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Observability Interfaces                          │   │
│  │                    Metrics  │  Tracing  │  Logging                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 二、Channel 接口

### 2.1 接口定义

```go
// pkg/channel/channel.go

package channel

import (
    "context"
    "time"
)

// Message 消息结构
type Message struct {
    // 消息唯一 ID
    ID string
    
    // 会话 ID
    SessionID string
    
    // 发送者
    From Sender
    
    // 接收者
    To Receiver
    
    // 消息类型: text/image/file/post
    Type string
    
    // 消息内容
    Content Content
    
    // 原始消息（通道特定格式）
    Raw interface{}
    
    // 时间戳
    Timestamp time.Time
    
    // 元数据
    Meta map[string]interface{}
}

// Sender 发送者
type Sender struct {
    // 用户 ID（通道特定）
    UserID string
    
    // 用户名
    Username string
    
    // 昵称
    Nickname string
    
    // 头像
    Avatar string
    
    // OpenID（统一）
    OpenID string
}

// Receiver 接收者
type Receiver struct {
    // 频道 ID
    ChannelID string
    
    // Bot ID
    BotID string
}

// Content 内容
type Content struct {
    // 文本内容
    Text string
    
    // 富文本（HTML/Markdown）
    RichText string
    
    // 附件列表
    Attachments []Attachment
    
    // 引用消息
    Quote *Message
}

// Attachment 附件
type Attachment struct {
    // 类型: image/audio/video/file
    Type string
    
    // URL 或文件路径
    URL string
    
    // 文件名
    Filename string
    
    // 大小（字节）
    Size int64
    
    // MIME 类型
    MimeType string
}

// Channel 通道接口
type Channel interface {
    // Name 返回通道名称
    Name() string
    
    // Send 发送消息
    Send(ctx context.Context, msg *Message) error
    
    // SendRaw 发送原始消息
    SendRaw(ctx context.Context, raw interface{}) error
    
    // Receive 接收消息（阻塞）
    Receive(ctx context.Context) (*Message, error)
    
    // Ack 确认消息
    Ack(ctx context.Context, msgID string) error
    
    // Close 关闭通道
    Close(ctx context.Context) error
}

// ChannelFactory 通道工厂
type ChannelFactory interface {
    // Create 创建通道
    Create(cfg *ChannelConfig) (Channel, error)
    
    // Type 返回通道类型
    Type() string
}

// ChannelConfig 通道配置
type ChannelConfig struct {
    // 通道类型
    Type string
    
    // 配置字典
    Config map[string]interface{}
    
    // 回调函数
    Callbacks ChannelCallbacks
}

// ChannelCallbacks 通道回调
type ChannelCallbacks struct {
    // 消息回调
    OnMessage func(*Message) error
    
    // 事件回调
    OnEvent func(event Event) error
    
    // 错误回调
    OnError func(err error)
}

// Event 事件
type Event struct {
    // 事件类型
    Type string
    
    // 事件数据
    Data interface{}
    
    // 时间戳
    Timestamp time.Time
}
```

### 2.2 内置通道实现

#### Feishu Channel

```go
// pkg/channel/feishu/feishu.go

package feishu

type FeishuChannel struct {
    config   *FeishuConfig
    client   *lark.Client
    botInfo  *BotInfo
    callbacks *ChannelCallbacks
}

type FeishuConfig struct {
    // App ID
    AppID string
    
    // App Secret
    AppSecret string
    
    // 加密密钥
    EncryptKey string
    
    // 验证 Token
    VerificationToken string
    
    // 消息回调地址
    CallbackURL string
    
    // 是否启用 Long Polling
    UseLongPolling bool
}

// 实现 Channel 接口
func (f *FeishuChannel) Name() string { return "feishu" }

func (f *FeishuChannel) Send(ctx context.Context, msg *Message) error {
    // 发送消息到飞书
}

func (f *FeishuChannel) Receive(ctx context.Context) (*Message, error) {
    // 接收飞书消息
}

func (f *FeishuChannel) Close(ctx context.Context) error {
    // 关闭连接
}
```

#### Slack Channel

```go
// pkg/channel/slack/slack.go

package slack

type SlackChannel struct {
    config   *SlackConfig
    client   *slack.Client
    callbacks *ChannelCallbacks
}

type SlackConfig struct {
    // Bot Token
    Token string
    
    // Signing Secret
    SigningSecret string
    
    // App Token（用于 Socket Mode）
    AppToken string
    
    // 工作空间 ID
    WorkspaceID string
    
    // 启用 Socket Mode
    UseSocketMode bool
}
```

---

## 三、Brain 接口

### 3.1 接口定义

```go
// pkg/brain/brain.go

package brain

import (
    "context"
    "time"
)

// Brain 原生智能层
// 注意：Brain 不是 Worker，是独立智能层
type Brain interface {
    // Process 处理消息（核心入口）
    Process(ctx context.Context, msg *Message) (*Result, error)
    
    // Name 返回 Brain 名称
    Name() string
    
    // Close 关闭
    Close(ctx context.Context) error
}

// Result 处理结果
type Result struct {
    // 是否需要路由到 Worker
    NeedWorker bool
    
    // 意图类型
    Intent Intent
    
    // 增强后的上下文
    EnhancedContext *Context
    
    // 拦截原因（如果 NeedWorker=false）
    BlockReason string
    
    // 直接回复（绕过 Worker）
    DirectReply *Message
    
    // 元数据
    Meta map[string]interface{}
}

// Intent 意图
type Intent struct {
    // 意图类型
    Type IntentType
    
    // 意图名称
    Name string
    
    // 置信度
    Confidence float64
    
    // 参数
    Params map[string]interface{}
}

// IntentType 意图类型
type IntentType string

const (
    IntentChat         IntentType = "chat"           // 闲聊
    IntentTask         IntentType = "task"           // 任务执行
    IntentQuery        IntentType = "query"          // 查询
    IntentCode         IntentType = "code"           // 代码相关
    IntentAdmin        IntentType = "admin"          // 管理命令
    IntentUnknown      IntentType = "unknown"        // 未知
)

// Context 上下文
type Context struct {
    // 会话 ID
    SessionID string
    
    // 用户信息
    User *User
    
    // 历史消息
    History []*Message
    
    // 长期记忆
    LongTermMemory []*Memory
    
    // 短期记忆
    ShortTermMemory map[string]interface{}
    
    // 扩展数据
    Extra map[string]interface{}
}

// User 用户
type User struct {
    // 用户 ID
    ID string
    
    // 用户名
    Username string
    
    // 角色
    Role UserRole
    
    // 权限列表
    Permissions []string
    
    // 偏好设置
    Preferences map[string]interface{}
}

// UserRole 用户角色
type UserRole string

const (
    RoleAdmin  UserRole = "admin"  // 管理员
    RoleUser   UserRole = "user"   // 普通用户
    RoleGuest  UserRole = "guest"   // 访客
)

// Memory 记忆
type Memory struct {
    // 记忆 ID
    ID string
    
    // 记忆内容
    Content string
    
    // 记忆类型
    Type MemoryType
    
    // 重要性
    Importance int
    
    // 创建时间
    CreatedAt time.Time
    
    // 访问时间
    AccessedAt time.Time
    
    // 元数据
    Meta map[string]interface{}
}

// MemoryType 记忆类型
type MemoryType string

const (
    MemoryConversation MemoryType = "conversation"  // 对话记忆
    MemoryFact         MemoryType = "fact"          // 事实
    MemoryPreference   MemoryType = "preference"     // 偏好
    MemorySkill       MemoryType = "skill"          // 技能
)
```

### 3.2 WAF 接口

```go
// pkg/brain/waf/waf.go

package waf

// WAF Web 应用防火墙
type WAF interface {
    // Check 检查消息
    Check(ctx context.Context, msg *Message) *WAFResult
    
    // Name 返回名称
    Name() string
}

// WAFResult 检查结果
type WAFResult struct {
    // 是否通过
    Pass bool
    
    // 违规类型
    ViolationType string
    
    // 违规详情
    Details string
    
    // 处置动作
    Action WAFAction
}

// WAFAction 处置动作
type WAFAction string

const (
    WAFActionAllow  WAFAction = "allow"   // 允许
    WAFActionBlock  WAFAction = "block"  // 拦截
    WAFActionWarn   WAFAction = "warn"   // 警告
    WAFActionAudit  WAFAction = "audit"  # 仅审计
)

// 内置 WAF 规则
var DefaultRules = []Rule{
    // 敏感信息检测
    {Type: "sensitive_data", Pattern: `password\s*=\s*.+`, Action: WAFActionBlock},
    {Type: "sensitive_data", Pattern: `api[_-]?key\s*=\s*.+`, Action: WAFActionBlock},
    {Type: "sensitive_data", Pattern: `secret\s*=\s*.+`, Action: WAFActionBlock},
    
    // SQL 注入
    {Type: "sql_injection", Pattern: `(?i)(union|select|insert|update|delete).*from`, Action: WAFActionBlock},
    
    // 命令注入
    {Type: "command_injection", Pattern: `[,;]\s*(rm|cat|ls|echo|wget|curl)`, Action: WAFActionBlock},
}
```

### 3.3 意图路由

```go
// pkg/brain/intent/router.go

package intent

// Router 意图路由器
type Router interface {
    // Route 路由意图
    Route(ctx context.Context, msg *Message) (*Intent, error)
    
    // Register 注册意图处理器
    Register(handler IntentHandler)
}

// IntentHandler 意图处理器
type IntentHandler interface {
    // Match 匹配条件
    Match(intent *Intent) bool
    
    // Handle 处理
    Handle(ctx context.Context, msg *Message, ctx *brain.Context) (*brain.Result, error)
}

// IntentHandlerFunc 函数类型处理器
type IntentHandlerFunc func(ctx context.Context, msg *Message, ctx *brain.Context) (*brain.Result, error)

func (f IntentHandlerFunc) Handle(ctx context.Context, msg *Message, ctx *brain.Context) (*brain.Result, error) {
    return f(ctx, msg, ctx)
}
```

---

## 四、Worker 接口

### 4.1 接口定义

```go
// pkg/worker/worker.go

package worker

import (
    "context"
    "time"
)

// Worker Worker 接口
// Worker 是 ClaudeCode 或 OpenCode，负责执行具体任务
type Worker interface {
    // ID 返回 Worker ID
    ID() string
    
    // Type 返回 Worker 类型
    Type() WorkerType
    
    // Execute 执行任务
    Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error)
    
    // Health 健康检查
    Health(ctx context.Context) error
    
    // Close 关闭 Worker
    Close(ctx context.Context) error
}

// WorkerType Worker 类型
type WorkerType string

const (
    WorkerClaudeCode WorkerType = "claude_code"  // Claude Code Worker
    WorkerOpenCode   WorkerType = "open_code"    // OpenCode Worker
)

// ExecuteRequest 执行请求
type ExecuteRequest struct {
    // 会话 ID
    SessionID string
    
    // 用户消息
    UserMessage string
    
    // 上下文
    Context map[string]interface{}
    
    // 超时时间
    Timeout time.Duration
    
    // 工作目录
    WorkDir string
    
    // 环境变量
    Env map[string]string
}

// ExecuteResult 执行结果
type ExecuteResult struct {
    // 是否成功
    Success bool
    
    // 输出内容
    Output string
    
    // 错误信息
    Error string
    
    // 退出码
    ExitCode int
    
    // 执行时间
    Duration time.Duration
    
    // Token 消耗
    TokenUsage *TokenUsage
    
    // 元数据
    Meta map[string]interface{}
}

// TokenUsage Token 消耗
type TokenUsage struct {
    // 输入 Token
    InputTokens int
    
    // 输出 Token
    OutputTokens int
    
    // 总计
    TotalTokens int
    
    // 成本
    Cost float64
}

// Supervisor 监管者接口
type Supervisor interface {
    // Register 注册 Worker
    Register(w Worker) error
    
    // Unregister 注销 Worker
    Unregister(workerID string) error
    
    // Get 获取 Worker
    Get(workerID string) (Worker, error)
    
    // List 列出所有 Worker
    List() []Worker
    
    // Acquire 获取可用 Worker
    Acquire(ctx context.Context, workerType WorkerType) (Worker, error)
    
    // Release 释放 Worker
    Release(workerID string) error
    
    // HealthCheck 健康检查
    HealthCheck(ctx context.Context) error
}
```

### 4.2 Worker 生命周期

```go
// pkg/worker/lifecycle.go

// Lifecycle 生命周期状态
type LifecycleState string

const (
    StatePending   LifecycleState = "pending"    // 等待中
    StateStarting  LifecycleState = "starting"   // 启动中
    StateRunning   LifecycleState = "running"     // 运行中
    StateBusy      LifecycleState = "busy"        // 忙碌中
    StateStopping  LifecycleState = "stopping"   // 停止中
    StateStopped   LifecycleState = "stopped"    // 已停止
    StateError     LifecycleState = "error"      // 错误
)

// StateChangedEvent 状态变更事件
type StateChangedEvent struct {
    WorkerID   string
    OldState   LifecycleState
    NewState   LifecycleState
    Reason     string
    Timestamp  time.Time
}

// StateCallback 状态回调
type StateCallback func(*StateChangedEvent)
```

---

## 五、Session 接口

### 5.1 接口定义

```go
// pkg/session/session.go

package session

import (
    "context"
    "time"
)

// Session 会话接口
type Session interface {
    // ID 返回会话 ID
    ID() string
    
    // Get 获取值
    Get(ctx context.Context, key string) (interface{}, error)
    
    // Set 设置值
    Set(ctx context.Context, key string, value interface{}) error
    
    // Delete 删除值
    Delete(ctx context.Context, key string) error
    
    // GetAll 获取所有值
    GetAll(ctx context.Context) (map[string]interface{}, error)
    
    // SetMetadata 设置元数据
    SetMetadata(ctx context.Context, meta map[string]interface{}) error
    
    // GetMetadata 获取元数据
    GetMetadata(ctx context.Context) (map[string]interface{}, error)
    
    // Touch 更新访问时间
    Touch(ctx context.Context) error
    
    // Delete 删除会话
    Delete(ctx context.Context) error
    
    // Close 关闭会话
    Close(ctx context.Context) error
}

// Manager 会话管理器
type Manager interface {
    // Create 创建会话
    Create(ctx context.Context, meta *SessionMeta) (Session, error)
    
    // Get 获取会话
    Get(ctx context.Context, sessionID string) (Session, error)
    
    // Delete 删除会话
    Delete(ctx context.Context, sessionID string) error
    
    // List 列出会话
    List(ctx context.Context, opts *ListOptions) ([]Session, error)
    
    // Count 计数
    Count(ctx context.Context) (int64, error)
    
    // Cleanup 清理过期会话
    Cleanup(ctx context.Context) error
}

// SessionMeta 会话元数据
type SessionMeta struct {
    // 会话 ID
    ID string
    
    // 用户 ID
    UserID string
    
    // 通道类型
    ChannelType string
    
    // 创建时间
    CreatedAt time.Time
    
    // 最后访问时间
    LastAccessedAt time.Time
    
    // 过期时间
    ExpiresAt time.Time
    
    // 元数据
    Metadata map[string]interface{}
}

// ListOptions 列表选项
type ListOptions struct {
    // 用户 ID
    UserID string
    
    // 通道类型
    ChannelType string
    
    // 过期过滤
    ExpiredOnly bool
    
    // 分页
    Offset int
    Limit int
}
```

### 5.2 分层存储

```go
// Memory LRU 缓存（短期）
type MemoryStore struct {
    lru     *lru.Cache
    ttl     time.Duration
    maxSize int
}

// SQLite 持久化（长期）
type SQLiteStore struct {
    db      *sql.DB
    path    string
}

// 分层策略
// 1. 读取：Memory LRU → SQLite
// 2. 写入：Memory LRU → SQLite（异步）
// 3. 过期：Memory 自动过期，SQLite 后台清理
```

---

## 六、Provider 接口

### 6.1 接口定义

```go
// pkg/provider/provider.go

package provider

import (
    "context"
)

// Model 模型
type Model struct {
    // 模型 ID
    ID string
    
    // 提供者
    Provider string
    
    // 名称
    Name string
    
    // 上下文窗口
    ContextWindow int
    
    // 输入成本（每 1M tokens）
    InputCost float64
    
    // 输出成本（每 1M tokens）
    OutputCost float64
    
    // 能力
    Capabilities []string
}

// Message 消息
type Message struct {
    // 角色: system/user/assistant
    Role string
    
    // 内容
    Content string
    
    // 名称（可选）
    Name string
}

// ChatRequest 聊天请求
type ChatRequest struct {
    // 模型
    Model string
    
    // 消息列表
    Messages []Message
    
    // 温度
    Temperature float64
    
    // 最大 Token
    MaxTokens int
    
    // 停止词
    Stop []string
    
    // 工具
    Tools []Tool
    
    // 流式回调
    StreamFunc func(chunk string)
}

// ChatResponse 聊天响应
type ChatResponse struct {
    // 内容
    Content string
    
    // Token 消耗
    Usage TokenUsage
    
    // 停止原因
    StopReason string
    
    // 元数据
    Meta map[string]interface{}
}

// TokenUsage Token 消耗
type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

// Tool 工具
type Tool struct {
    // 类型
    Type string
    
    // 函数定义
    Function *FunctionDefinition
}

// FunctionDefinition 函数定义
type FunctionDefinition struct {
    Name        string
    Description string
    Parameters  map[string]interface{}
}

// Provider 提供者接口
type Provider interface {
    // Name 返回提供者名称
    Name() string
    
    // Chat 聊天
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    
    // Embeddings 向量嵌入
    Embeddings(ctx context.Context, texts []string) ([]float32, error)
    
    // ListModels 列出模型
    ListModels(ctx context.Context) ([]*Model, error)
}
```

### 6.2 内置 Provider

```go
// Anthropic Provider
type AnthropicProvider struct {
    apiKey string
    baseURL string
    client  *http.Client
}

// OpenAI Provider
type OpenAIProvider struct {
    apiKey string
    baseURL string
    client  *http.Client
}

// SiliconFlow Provider
type SiliconFlowProvider struct {
    apiKey string
    baseURL string
    client  *http.Client
}
```

---

## 七、Storage 接口

### 7.1 接口定义

```go
// pkg/storage/storage.go

package storage

import (
    "context"
    "io"
    "time"
)

// File 文件存储
type File interface {
    // Upload 上传文件
    Upload(ctx context.Context, req *UploadRequest) (*UploadResult, error)
    
    // Download 下载文件
    Download(ctx context.Context, path string) (io.ReadCloser, error)
    
    // Delete 删除文件
    Delete(ctx context.Context, path string) error
    
    // GetURL 获取访问 URL
    GetURL(ctx context.Context, path string) (string, error)
    
    // Exists 检查是否存在
    Exists(ctx context.Context, path string) (bool, error)
}

// UploadRequest 上传请求
type UploadRequest struct {
    // 文件名
    Filename string
    
    // 内容
    Content io.Reader
    
    // 内容类型
    ContentType string
    
    // 大小
    Size int64
    
    // 目标路径
    Path string
}

// UploadResult 上传结果
type UploadResult struct {
    // 文件路径
    Path string
    
    // URL
    URL string
    
    // 大小
    Size int64
    
    // ETag
    ETag string
}

// ConfigStore 配置存储
type ConfigStore interface {
    // Get 获取配置
    Get(ctx context.Context, key string) ([]byte, error)
    
    // Set 设置配置
    Set(ctx context.Context, key string, value []byte) error
    
    // Delete 删除配置
    Delete(ctx context.Context, key string) error
    
    // List 列出配置
    List(ctx context.Context, prefix string) ([]string, error)
}
```

---

## 八、Docker 插件接口

### 8.1 ContainerPlugin 接口

```go
// pkg/plugin/container.go

package plugin

import (
    "context"
    "time"
)

// ContainerConfig 容器配置
type ContainerConfig struct {
    // 镜像
    Image string
    
    // 容器名称
    Name string
    
    // 环境变量
    Env map[string]string
    
    // 挂载点
    Mounts []Mount
    
    // 网络模式
    NetworkMode string  // "bridge" | "host" | "none"
    
    // 资源限制
    Resources ContainerResources
    
    // Linux capabilities
    Capabilities []string
    
    // 安全选项
    SecurityOpt []string
    
    // 只读根文件系统
    ReadOnlyRoot bool
    
    // 工作目录
    WorkDir string
}

// Mount 挂载
type Mount struct {
    Source string
    Target string
    Type   string  // "bind" | "volume"
    ReadOnly bool
}

// ContainerResources 资源限制
type ContainerResources struct {
    // CPU
    CPU int64  // 毫核
    CPUPeriod int64
    CPUQuota int64
    
    // 内存
    Memory int64  // 字节
    MemorySwap int64
    MemoryReservation int64
    
    // 磁盘
    DiskQuota int64  // 字节
    
    // PIDs 限制
    PidsLimit int64
}

// ContainerPlugin 容器插件接口
type ContainerPlugin interface {
    // Plugin 实现插件基类
    Plugin
    
    // Start 启动容器
    Start(ctx context.Context) error
    
    // Stop 停止容器
    Stop(ctx context.Context) error
    
    // Restart 重启容器
    Restart(ctx context.Context) error
    
    // Wait 等待容器退出
    Wait(ctx context.Context) (int, error)
    
    // Exec 在容器中执行命令
    Exec(ctx context.Context, cmd []string, opts *ExecOptions) (*ExecResult, error)
    
    // Logs 获取容器日志
    Logs(ctx context.Context, opts *LogsOptions) (io.ReadCloser, error)
    
    // Stats 获取容器统计
    Stats(ctx context.Context) (*ContainerStats, error)
    
    // Update 更新容器配置
    Update(ctx context.Context, cfg *ContainerConfig) error
    
    // Config 获取容器配置
    Config(ctx context.Context) (*ContainerInfo, error)
}

// ExecOptions 执行选项
type ExecOptions struct {
    // 工作目录
