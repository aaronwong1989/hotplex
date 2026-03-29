# HotPlex v1.0.0 插件系统设计

> 版本：v1.0  
> 日期：2026-03-29  
> 状态：完整插件机制设计

---

## 1. 概述

### 1.1 设计目标

HotPlex v1.0.0 采用插件化架构，支持：
- **Channel 插件**：扩展新平台（飞书、Slack、WebSocket 已内置）
- **Worker 插件**：扩展新执行器（Claude Code、OpenCode 已内置）
- **Brain 插件**：扩展新智能层（LLM Brain、Rule Brain 已内置）
- **Storage 插件**：扩展新存储后端（SQLite、Postgres、Redis 已内置）
- **Provider 插件**：扩展新 LLM 提供商（Anthropic、OpenAI 已内置）

### 1.2 插件类型

| 类型 | 接口前缀 | 说明 |
|------|----------|------|
| Channel | `channel.` | 平台适配器 |
| Worker | `worker.` | 任务执行器 |
| Brain | `brain.` | 智能处理层 |
| Storage | `storage.` | 消息存储 |
| Provider | `provider.` | LLM 提供商 |
| Middleware | `middleware.` | 中间件 |

---

## 2. 插件接口 (`pkg/plugin/plugin.go`)

### 2.1 核心接口

```go
package plugin

import (
    "context"
    "io"
)

// Kind 插件类型
type Kind string

const (
    KindChannel   Kind = "channel"
    KindWorker    Kind = "worker"
    KindBrain     Kind = "brain"
    KindStorage   Kind = "storage"
    KindProvider  Kind = "provider"
    KindMiddleware Kind = "middleware"
)

// Base 插件基类
type Base struct {
    Name    string
    Version string
    Kind    Kind
}

// Plugin 插件接口
type Plugin interface {
    // 元信息
    Kind() Kind
    Name() string
    Version() string
    Description() string
    
    // 初始化
    Initialize(ctx context.Context, cfg map[string]interface{}) error
    
    // 关闭
    Close() error
}

// InitFunc 插件初始化函数
// 插件必须导出此符号，供动态加载
var InitFunc func(cfg map[string]interface{}) (Plugin, error)

// 示例：
// //go:build plugin
// //export InitFunc
// func InitFunc(cfg map[string]interface{}) (plugin.Plugin, error) {
//     return NewMyChannelPlugin(cfg)
// }
```

### 2.2 Channel 插件

```go
// ChannelPlugin Channel 插件接口
type ChannelPlugin interface {
    Plugin
    
    // 启动/停止
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // 发送消息
    Send(ctx context.Context, resp *Response) error
    
    // 事件处理
    SetEventHandler(handler EventHandler)
    
    // 健康检查
    Health(ctx context.Context) *HealthStatus
}

// Response 响应
type Response struct {
    MessageID  string
    ChannelID string
    UserID    string
    Content   string
    Format    ResponseFormat
}

// ResponseFormat 响应格式
type ResponseFormat string

const (
    FormatText        ResponseFormat = "text"
    FormatMarkdown    ResponseFormat = "markdown"
    FormatBlocks      ResponseFormat = "blocks"
    FormatInteractive ResponseFormat = "interactive"
)

// EventHandler 事件处理
type EventHandler interface {
    OnMessage(ctx context.Context, msg *Message) error
    OnError(err error, msg *Message)
    OnConnect()
    OnDisconnect(reason error)
}

// Message 消息
type Message struct {
    ID         string
    SessionID  string
    ChannelID  string
    UserID     string
    Content    string
    RawContent []byte
    Timestamp  int64
}
```

### 2.3 Worker 插件

```go
// WorkerPlugin Worker 插件接口
type WorkerPlugin interface {
    Plugin
    
    // 配置
    Configure(cfg *WorkerConfig) error
    
    // 生命周期
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // 执行
    Run(ctx context.Context, task *Task) (*Result, error)
    Stream(ctx context.Context, task *Task, output chan<- *Result) error
    
    // 控制
    Abort(taskID string) error
    
    // 状态
    Status() *WorkerStatus
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
    MaxConcurrent int
    Timeout       time.Duration
    Env           map[string]string
    PluginDir     string
}

// Task 任务
type Task struct {
    ID            string
    SessionID     string
    Prompt        string
    SystemPrompt  string
    Model         string
    MaxTokens     int
    Streaming     bool
    Timeout       time.Duration
}

// Result 结果
type Result struct {
    TaskID    string
    Status    ResultStatus
    Output    string
    Error     *TaskError
    ExitCode  int
    Usage     *Usage
    Metrics   map[string]interface{}
}

// ResultStatus 结果状态
type ResultStatus string

const (
    ResultSuccess  ResultStatus = "success"
    ResultFailed   ResultStatus = "failed"
    ResultTimeout  ResultStatus = "timeout"
    ResultAborted  ResultStatus = "aborted"
)

// WorkerStatus Worker 状态
type WorkerStatus struct {
    State       string
    Running     int
    MaxConcurrent int
}
```

### 2.4 Brain 插件

```go
// BrainPlugin Brain 插件接口
type BrainPlugin interface {
    Plugin
    
    // 初始化
    Setup(cfg *BrainConfig) error
    
    // 处理
    Process(ctx context.Context, input *BrainInput) (*BrainOutput, error)
    
    // 流式处理（可选）
    Stream(ctx context.Context, input *BrainInput, output chan<- *BrainOutput) error
}

// BrainConfig Brain 配置
type BrainConfig struct {
    Model          string
    WAFLevel       string
    ContextWindow  int
    Timeout        time.Duration
}

// BrainInput 输入
type BrainInput struct {
    Message   *Message
    History   []*Message
    SessionID string
    UserID    string
    Config    *BrainConfig
}

// BrainOutput 输出
type BrainOutput struct {
    Intent      *IntentResult
    Guard       *GuardResult
    RoutedTo    string
    Enhanced    *Message
    Blocked     bool
    BlockReason string
}

// IntentResult 意图结果
type IntentResult struct {
    Kind       string
    Confidence float64
    Params     map[string]interface{}
}

// GuardResult 安全结果
type GuardResult struct {
    Passed     bool
    Level      string
    Violations []Violation
}

// Violation 违规
type Violation struct {
    RuleID   string
    Message  string
    Severity string
    Matched  string
}
```

### 2.5 Storage 插件

```go
// StoragePlugin Storage 插件接口
type StoragePlugin interface {
    Plugin
    
    // 写入
    Save(ctx context.Context, records ...*Record) error
    
    // 读取
    Get(ctx context.Context, id string) (*Record, error)
    Query(ctx context.Context, q *Query) (*QueryResult, error)
    
    // 删除
    Delete(ctx context.Context, id string) error
    
    // 统计
    Stats(ctx context.Context) (*StorageStats, error)
    
    // 生命周期
    Close() error
}

// Record 记录
type Record struct {
    ID         string
    SessionID  string
    Role       string      // "user" | "assistant" | "system"
    Content    string
    Metadata   map[string]string
    CreatedAt  int64
}

// Query 查询
type Query struct {
    SessionIDs []string
    UserIDs    []string
    StartTime  int64
    EndTime    int64
    Limit      int
    Offset     int
    Text       string  // 全文搜索
}

// QueryResult 查询结果
type QueryResult struct {
    Records []*Record
    Total   int
}

// StorageStats 统计
type StorageStats struct {
    TotalRecords  int64
    TotalSessions int64
    TotalSizeBytes int64
}
```

### 2.6 Provider 插件

```go
// ProviderPlugin Provider 插件接口
type ProviderPlugin interface {
    Plugin
    
    // 模型
    ListModels(ctx context.Context) ([]*Model, error)
    GetModel(modelID string) (*Model, error)
    
    // 生成
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Stream(ctx context.Context, req *GenerateRequest) (<-chan *StreamChunk, error)
    
    // 关闭
    Close() error
}

// Model 模型
type Model struct {
    ID           string
    Name         string
    Kind         string  // "chat" | "embedding"
    MaxTokens    int
    ContextWindow int
}

// GenerateRequest 请求
type GenerateRequest struct {
    Model     string
    Messages  []*Message
    Temperature float64
    MaxTokens int
    Stream    bool
    Tools     []Tool
}

// Message 消息
type Message struct {
    Role    string
    Content string
}

// GenerateResponse 响应
type GenerateResponse struct {
    Message    *Message
    Usage      *Usage
    StopReason string
}

// StreamChunk 流式块
type StreamChunk struct {
    Type    string
    Content string
    Delta   string
    Index   int
}

// Usage 使用量
type Usage struct {
    InputTokens  int
    OutputTokens int
}

// Tool 工具
type Tool struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
}
```

---

## 3. 插件注册表 (`pkg/plugin/registry.go`)

### 3.1 注册表

```go
// Registry 插件注册表
type Registry struct {
    plugins map[Kind]map[string]PluginFactory
    mu      sync.RWMutex
}

// PluginFactory 插件工厂
type PluginFactory func(cfg map[string]interface{}) (Plugin, error)

// 全局注册表
var globalRegistry *Registry

func init() {
    globalRegistry = NewRegistry()
    
    // 注册内置插件
    RegisterBuiltinPlugins()
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
    return &Registry{
        plugins: make(map[Kind]map[string]PluginFactory),
    }
}

// Register 注册插件
func (r *Registry) Register(kind Kind, name string, factory PluginFactory) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if r.plugins[kind] == nil {
        r.plugins[kind] = make(map[string]PluginFactory)
    }
    
    if _, exists := r.plugins[kind][name]; exists {
        return fmt.Errorf("plugin %s.%s already registered", kind, name)
    }
    
    r.plugins[kind][name] = factory
    return nil
}

// Get 获取插件
func (r *Registry) Get(kind Kind, name string) (PluginFactory, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    factory, ok := r.plugins[kind][name]
    if !ok {
        return nil, fmt.Errorf("plugin %s.%s not found", kind, name)
    }
    
    return factory, nil
}

// List 列出插件
func (r *Registry) List(kind Kind) []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var names []string
    for name := range r.plugins[kind] {
        names = append(names, name)
    }
    return names
}

// Unregister 注销插件
func (r *Registry) Unregister(kind Kind, name string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, ok := r.plugins[kind][name]; !ok {
        return fmt.Errorf("plugin %s.%s not found", kind, name)
    }
    
    delete(r.plugins[kind], name)
    return nil
}
```

### 3.2 便捷函数

```go
// RegisterChannel 注册 Channel 插件
func RegisterChannel(name string, factory PluginFactory) {
    globalRegistry.Register(KindChannel, name, factory)
}

// RegisterWorker 注册 Worker 插件
func RegisterWorker(name string, factory PluginFactory) {
    globalRegistry.Register(KindWorker, name, factory)
}

// RegisterBrain 注册 Brain 插件
func RegisterBrain(name string, factory PluginFactory) {
    globalRegistry.Register(KindBrain, name, factory)
}

// RegisterStorage 注册 Storage 插件
func RegisterStorage(name string, factory PluginFactory) {
    globalRegistry.Register(KindStorage, name, factory)
}

// RegisterProvider 注册 Provider 插件
func RegisterProvider(name string, factory PluginFactory) {
    globalRegistry.Register(KindProvider, name, factory)
}

// GetChannel 获取 Channel 插件
func GetChannel(name string) (ChannelPlugin, error) {
    factory, err := globalRegistry.Get(KindChannel, name)
    if err != nil {
        return nil, err
    }
    return factory(nil).(ChannelPlugin), nil
}

// GetWorker 获取 Worker 插件
func GetWorker(name string) (WorkerPlugin, error) {
    factory, err := globalRegistry.Get(KindWorker, name)
    if err != nil {
        return nil, err
    }
    return factory(nil).(WorkerPlugin), nil
}
```

---

## 4. 动态加载 (`pkg/plugin/loader.go`)

### 4.1 插件加载器

```go
// Loader 插件加载器
type Loader struct {
    pluginDir string
    registry  *Registry
    loaded    map[string]Plugin
    mu        sync.Mutex
}

// NewLoader 创建加载器
func NewLoader(pluginDir string) *Loader {
    return &Loader{
        pluginDir: pluginDir,
        registry:  globalRegistry,
        loaded:    make(map[string]Plugin),
    }
}

// Load 加载插件
func (l *Loader) Load(ctx context.Context, kind Kind, name string, cfg map[string]interface{}) (Plugin, error) {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    key := fmt.Sprintf("%s.%s", kind, name)
    
    // 检查是否已加载
    if p, ok := l.loaded[key]; ok {
        return p, nil
    }
    
    // 尝试从注册表获取
    if factory, err := l.registry.Get(kind, name); err == nil {
        p, err := factory(cfg)
        if err != nil {
            return nil, fmt.Errorf("failed to create plugin %s: %w", key, err)
        }
        l.loaded[key] = p
        return p, nil
    }
    
    // 尝试动态加载 .so 文件
    pluginPath := filepath.Join(l.pluginDir, fmt.Sprintf("hotplex-%s-%s.so", kind, name))
    if _, err := os.Stat(pluginPath); err == nil {
        return l.loadSO(ctx, pluginPath, cfg)
    }
    
    return nil, fmt.Errorf("plugin %s not found", key)
}

// LoadSO 加载 .so 文件
func (l *Loader) LoadSO(ctx context.Context, path string, cfg map[string]interface{}) (Plugin, error) {
    // 打开 .so 文件
    pluginLib, err := plugin.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open plugin: %w", err)
    }
    
    // 查找 InitFunc
    sym, err := pluginLib.Lookup("InitFunc")
    if err != nil {
        return nil, fmt.Errorf("plugin missing InitFunc: %w", err)
    }
    
    initFn, ok := sym.(func(map[string]interface{}) (Plugin, error))
    if !ok {
        return nil, fmt.Errorf("InitFunc has wrong signature")
    }
    
    // 调用初始化
    p, err := initFn(cfg)
    if err != nil {
        return nil, fmt.Errorf("plugin init failed: %w", err)
    }
    
    return p, nil
}

// Unload 卸载插件
func (l *Loader) Unload(key string) error {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    p, ok := l.loaded[key]
    if !ok {
        return nil
    }
    
    if err := p.Close(); err != nil {
        return err
    }
    
    delete(l.loaded, key)
    return nil
}

// UnloadAll 卸载所有插件
func (l *Loader) UnloadAll() error {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    for key, p := range l.loaded {
        if err := p.Close(); err != nil {
            slog.Error("failed to close plugin", "key", key, "error", err)
        }
        delete(l.loaded, key)
    }
    
    return nil
}
```

### 4.2 插件发现

```go
// Discover 发现插件
func (l *Loader) Discover(ctx context.Context) ([]*DiscoveredPlugin, error) {
    var discovered []*DiscoveredPlugin
    
    // 扫描插件目录
    pattern := filepath.Join(l.pluginDir, "hotplex-*.so")
    files, err := filepath.Glob(pattern)
    if err != nil {
        return nil, err
    }
    
    for _, file := range files {
        p, err := l.loadSO(ctx, file, nil)
        if err != nil {
            slog.Warn("failed to load plugin", "path", file, "error", err)
            continue
        }
        
        discovered = append(discovered, &DiscoveredPlugin{
            Path:    file,
            Kind:    p.Kind(),
            Name:    p.Name(),
            Version: p.Version(),
        })
    }
    
    return discovered, nil
}

// DiscoveredPlugin 发现的插件
type DiscoveredPlugin struct {
    Path    string
    Kind    Kind
    Name    string
    Version string
}
```

---

## 5. 内置插件 vs 外部插件

### 5.1 内置插件

内置插件在主程序编译时链接：

```go
// 内置插件注册
func RegisterBuiltinPlugins() {
    // Channel
    plugin.RegisterChannel("feishu", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return feishu.NewChannel(cfg)
    })
    plugin.RegisterChannel("slack", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return slack.NewChannel(cfg)
    })
    plugin.RegisterChannel("ws", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return ws.NewChannel(cfg)
    })
    
    // Worker
    plugin.RegisterWorker("claude-code", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return worker.NewClaudeCode(cfg)
    })
    plugin.RegisterWorker("open-code", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return worker.NewOpenCode(cfg)
    })
    
    // Brain
    plugin.RegisterBrain("llm", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return brain.NewLLMBrain(cfg)
    })
    plugin.RegisterBrain("rule", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return brain.NewRuleBrain(cfg)
    })
    
    // Storage
    plugin.RegisterStorage("sqlite", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return storage.NewSQLite(cfg)
    })
    plugin.RegisterStorage("postgres", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return storage.NewPostgres(cfg)
    })
    plugin.RegisterStorage("redis", func(cfg map[string]interface{}) (plugin.Plugin, error) {
        return storage.NewRedis(cfg)
    })
}
```

### 5.2 外部插件

外部插件通过 `.so` 文件动态加载：

```
/etc/hotplex/plugins/
├── hotplex-channel-dingtalk.so     # 钉钉 Channel
├── hotplex-worker-copilot.so       # Copilot Worker
├── hotplex-brain-custom.so        # 自定义 Brain
├── hotplex-storage-s3.so          # S3 Storage
└── hotplex-provider-ollama.so     # Ollama Provider
```

**插件开发示例：**

```go
// hotplex-channel-dingtalk/main.go

package main

import (
    "context"
    "fmt"
)

type DingtalkChannel struct {
    appKey    string
    appSecret string
    handler   plugin.EventHandler
}

func NewDingtalkChannel(cfg map[string]interface{}) (plugin.Plugin, error) {
    return &DingtalkChannel{
        appKey:    cfg["app_key"].(string),
        appSecret: cfg["app_secret"].(string),
    }, nil
}

func (c *DingtalkChannel) Kind() plugin.Kind    { return plugin.KindChannel }
func (c *DingtalkChannel) Name() string        { return "dingtalk" }
func (c *DingtalkChannel) Version() string      { return "1.0.0" }
func (c *DingtalkChannel) Description() string { return "Dingtalk Channel Plugin" }

func (c *DingtalkChannel) Initialize(ctx context.Context, cfg map[string]interface{}) error {
    return nil
}

func (c *DingtalkChannel) Start(ctx context.Context) error {
    // 启动监听
    return nil
}

func (c *DingtalkChannel) Stop(ctx context.Context) error {
    return nil
}

func (c *DingtalkChannel) Send(ctx context.Context, resp *plugin.Response) error {
    return nil
}

func (c *DingtalkChannel) SetEventHandler(h plugin.EventHandler) {
    c.handler = h
}

func (c *DingtalkChannel) Health(ctx context.Context) *plugin.HealthStatus {
    return &plugin.HealthStatus{Status: "healthy"}
}

func (c *DingtalkChannel) Close() error {
    return nil
}

//go:build plugin
//export InitFunc
func InitFunc(cfg map[string]interface{}) (plugin.Plugin, error) {
    return NewDingtalkChannel(cfg)
}

func main() {}
```

**编译插件：**

```bash
# 编译插件
go build -buildmode=plugin -o hotplex-channel-dingtalk.so ./main.go

# 移动到插件目录
sudo mv hotplex-channel-dingtalk.so /etc/hotplex/plugins/
```

---

## 6. 插件配置

### 6.1 插件配置示例

```yaml
hotplex:
  plugins:
    enabled: true
    dir: "/etc/hotplex/plugins"
    
    # 内置插件
    builtins:
      feishu:
        enabled: true
      slack:
        enabled: false
    
    # 外部插件
    external:
      dingtalk:
        enabled: true
        config:
          app_key: "${DINGTALK_APP_KEY}"
          app_secret: "${DINGTALK_APP_SECRET}"
```

---

## 7. 插件生命周期

### 7.1 生命周期钩子

```go
// Lifecycle 生命周期接口
type Lifecycle interface {
    OnStart(ctx context.Context) error
    OnStop(ctx context.Context) error
    OnReload(cfg map[string]interface{}) error
}

// Manager 插件管理器
type Manager struct {
    loader   *Loader
    plugins  map[string]Plugin
    hooks    map[string][]Lifecycle
    mu       sync.RWMutex
}

// StartAll 启动所有插件
func (m *Manager) StartAll(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 按依赖顺序启动
    for _, name := range m.sortedNames() {
        p := m.plugins[name]
        if l, ok := p.(Lifecycle); ok {
            if err := l.OnStart(ctx); err != nil {
                return fmt.Errorf("plugin %s failed to start: %w", name, err)
            }
        }
    }
    
    return nil
}

// StopAll 停止所有插件
func (m *Manager) StopAll(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 逆序停止
    for i := len(m.sortedNames()) - 1; i >= 0; i-- {
        name := m.sortedNames()[i]
        p := m.plugins[name]
        if l, ok := p.(Lifecycle); ok {
            if err := l.OnStop(ctx); err != nil {
                slog.Error("plugin stop failed", "name", name, "error", err)
            }
        }
    }
    
    return nil
}
```

---

## 8. 插件安全

### 8.1 插件签名验证

```go
// Signature 插件签名
type Signature struct {
    Name    string
    Version string
    SHA256  string
    SignedBy string
}

// Verify 验证插件签名
func Verify(pluginPath string, sig *Signature) error {
    // 读取插件
    data, err := os.ReadFile(pluginPath)
    if err != nil {
        return err
    }
    
    // 校验哈希
    hash := sha256.Sum256(data)
    if hex.EncodeToString(hash[:]) != sig.SHA256 {
        return errors.New("plugin signature mismatch")
    }
    
    return nil
}
```

### 8.2 插件权限控制

```go
// Permissions 插件权限
type Permissions struct {
    AllowedDirs    []string
    AllowedNetwork []string
    AllowedEnvVars []string
}

// Sandbox 沙箱配置
type Sandbox struct {
    Enabled     bool
    Permissions Permissions
}

// ValidatePermissions 校验权限
func ValidatePermissions(p Plugin, perms *Permissions) error {
    switch v := p.(type) {
    case WorkerPlugin:
        if !isDirAllowed(v.Config().WorkDir, perms.AllowedDirs) {
            return fmt.Errorf("worker directory not allowed")
        }
    case ChannelPlugin:
        // Channel 可能需要网络权限
        if len(perms.AllowedNetwork) == 0 {
            return errors.New("channel plugin requires network permission")
        }
    }
    return nil
}
```

---

## 9. 附录：插件开发 SDK

### 9.1 Go Module 模板

```go
// go.mod
module github.com/your-org/hotplex-channel-dingtalk

go 1.22

require github.com/huangzhonghui/hotplex v1.0.0

replace github.com/huangzhonghui/hotplex => /path/to/hotplex
```

### 9.2 完整示例

参考 `docs/v1-arch/plugin-example/` 目录下的完整插件示例。

---

*文档版本：v1.0 | 最后更新：2026-03-29*
