# cc-connect 竞品深度调研：HotPlex 对比与改进提案

> 来源：[chenhg5/cc-connect](https://github.com/chenhg5/cc-connect) v0.31.7+
> 调研分支：`research/cc-connect-deep-dive`
> HotPlex 版本：v0.31.7

---

## 架构对比总览

| 维度 | HotPlex | cc-connect |
|------|---------|------------|
| **架构模型** | SDK + Daemon + 事件驱动 WebSocket | 协议桥接（Platform ↔ Agent） |
| **Provider** | 单 Provider，env 注入，CLI 进程调用 | 多 Provider + 本地反向代理（ProviderProxy） |
| **Session** | 池化管理，长驻进程，PGID 隔离 | 每消息启动/恢复，`--resume` |
| **权限** | WAF + AllowedTools，YAML 配置 | `control_response` 原生协议，`--permission-prompt-tool stdio` |
| **多项目** | 单一 Engine，支持多 Session | 多项目独立 Engine + BridgeServer |
| **平台集成** | Platform Adapter 嵌入编译 | BridgeServer WebSocket 解耦 |
| **配置** | 多层（YAML/ENV/继承） | TOML + 行级原子更新 |
| **CLI 协议** | 基础 stdio pipe | `stream-json` + 完整事件解析 |

---

## Issue #1: ProviderProxy — 第三方 API 协议兼容层

**优先级**: 高

### cc-connect 方案

`core/providerproxy.go` 启动本地反向代理，在 127.0.0.1 随机端口监听：

```go
// 所有 /messages POST 请求经过时重写 thinking.type
if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages") {
    rewriteThinkingInRequest(r, override)  // adaptive → disabled/enabled
}
proxy.ServeHTTP(w, r)
```

解决了 SiliconFlow 等 Provider 不支持 `thinking.type=adaptive` 的兼容问题。

### HotPlex 现状

- `provider/claude_provider.go`：纯 CLI 进程调用，无 HTTP 层
- `BuildCLIArgs()` 设置 `ANTHROPIC_BASE_URL` env，但无协议转换能力
- 第三方 Provider（如 SiliconFlow、OpenRouter）可能发送不兼容的 API 参数

### 改进方案

```go
// internal/provider/proxy.go
type ProviderProxy struct {
    targetURL        string
    thinkingOverride string  // "disabled" | "enabled" | "adaptive"
    listener         net.Listener
    server           *http.Server
}

// 透明重写请求体中的 thinking.type 字段
func rewriteThinkingInRequest(r *http.Request, override string)
// Agent 层调用：proxyURL := NewProviderProxy(targetURL, thinking)
// CLI env: ANTHROPIC_BASE_URL=proxyURL + ANTHROPIC_AUTH_TOKEN=apiKey
```

**新增文件**：`internal/provider/proxy.go`
**影响范围**：`provider/claude_provider.go` - 添加 ProviderProxy 生命周期管理

---

## Issue #2: 多 Provider 运行时切换

**优先级**: 高

### cc-connect 方案

每个 Agent 配置多个 Provider，运行时切换：

```go
type ProviderConfig struct {
    Name     string   // e.g. "siliconflow", "openrouter"
    APIKey   string
    BaseURL  string
    Model    string
    Models   []ModelOption  // 预定义模型列表
    Thinking string         // thinking type 重写
    Env      map[string]string
}

// ProviderSwitcher 接口
func (a *Agent) SetActiveProvider(name string) bool
func (a *Agent) GetActiveProvider() *ProviderConfig
func (a *Agent) ListProviders() []ProviderConfig
```

`core/provider.go` 提供 Provider 模型列表查询：
```go
func GetProviderModels(providers []ProviderConfig, activeIdx int) []ModelOption
```

### HotPlex 现状

- `provider.ProviderConfig`：单 Provider 配置，无多 Provider 概念
- 配置通过 `provider.New()` 在启动时注入，无法运行时切换
- 无 `ModelSwitcher` 接口，模型切换需重建 Session

### 改进方案

```go
// types/provider.go
type ProviderConfig struct {
    Name     string
    APIKey   string
    BaseURL  string
    Model    string
    Models   []ModelOption  // 预定义可切换模型
    Thinking string          // thinking type override
    Env      map[string]string
}

// internal/provider/manager.go
type ProviderManager struct {
    providers  []ProviderConfig
    activeIdx int
    mu        sync.RWMutex
}

func (m *ProviderManager) SetActive(name string) bool
func (m *ProviderManager) GetActive() *ProviderConfig
func (m *ProviderManager) List() []ProviderConfig
func (m *ProviderManager) EnvForActive() []string  // 导出 env vars
```

**新增文件**：`types/provider.go`、`internal/provider/manager.go`
**扩展**：`provider/claude_provider.go` - 实现 `ProviderSwitcher` 接口

---

## Issue #3: BridgeServer — 平台解耦 WebSocket 协议

**优先级**: 高

### cc-connect 方案

`core/bridge.go` 实现了完整的 BridgeServer：

```go
// 适配器连接时注册能力和元数据
{"type":"register","platform":"telegram","capabilities":["text","image","buttons","card","typing"]}

// 消息协议
{"type":"message","msg_id":"...","session_key":"...","user_id":"...","content":"...","reply_ctx":"..."}

// 回复/发送
{"type":"reply","session_key":"...","content":"...","format":"text"}

// 富卡片
{"type":"card","session_key":"...","card":{...}}

// 打字指示
{"type":"typing_start"},{"type":"typing_stop"}

// 能力发现：fallback 机制
if !capabilities["card"] { fallback to plain text }
```

BridgePlatform 实现所有可选接口，fallback 到纯文本：

```go
func (bp *BridgePlatform) SendCard(ctx, replyCtx, card) error {
    if !capabilities["card"] {
        return bp.Reply(ctx, replyCtx, card.RenderText())  // fallback
    }
}
```

### HotPlex 现状

- `chatapps/` 下每个平台独立实现 adapter（slack、telegram 等）
- 无通用桥接协议，新增平台需修改核心代码
- `internal/server/hotplex_ws.go` 仅处理 SDK 客户端，不支持外部适配器

### 改进方案

```go
// internal/server/bridge.go
type BridgeServer struct {
    port    int
    token   string  // 认证 token
    adapters map[string]*bridgeAdapter
    engines  map[string]*BridgePlatform  // project → platform
}

type BridgePlatform struct {
    server      *BridgeServer
    project     string
    handler     base.MessageHandler
    navHandler  CardNavigationHandler
}

// WebSocket 路由 /bridge/ws
// 注册 → 声明能力 → 消息收发 → 心跳
```

**新增文件**：`internal/server/bridge.go`
**扩展**：`chatapps/` - 部分 adapter 可迁移为 Bridge 客户端

---

## Issue #4: Claude Code 原生协议适配

**优先级**: 高

### cc-connect 方案

cc-connect 完整解析了 Claude Code 的 `stream-json` 输出：

```go
// 事件类型
case "system":      // session_id 更新
case "assistant":   // tool_use, thinking, text content blocks
case "user":        // tool_result
case "result":      // 最终结果
case "control_request":    // 权限请求 ← 关键
case "control_cancel_request":
```

**权限协议**（cc-connect 核心创新）：

```go
// 启动参数
args := []string{
    "--output-format", "stream-json",
    "--input-format", "stream-json",
    "--permission-prompt-tool", "stdio",  // 关键！
}

// 权限请求事件
{"type":"control_request","request_id":"xxx","request":{
    "subtype":"can_use_tool",
    "tool_name":"Bash",
    "input":{...}
}}

// 响应
{"type":"control_response","response":{
    "subtype":"success","request_id":"xxx",
    "response":{"behavior":"allow","updatedInput":{...}}
}}
```

**自动模式**：`mode == "bypassPermissions"` 时自动允许所有权限请求。

### HotPlex 现状

- `provider/claude_provider.go`：已使用 `--output-format stream-json --input-format stream-json`
- `session.go`：解析 `assistant`、`tool_use`、`result` 事件
- **缺失**：`--permission-prompt-tool stdio`，权限通过 YAML AllowedTools 静态配置
- **缺失**：`control_request` 事件解析，无运行时权限询问机制
- `types.StreamMessage`：事件类型不完整，缺少 `thinking`、`control_request`、`control_cancel`

### 改进方案

```go
// event/events.go 新增事件类型
const (
    EventControlRequest       = "control_request"
    EventControlCancelRequest = "control_cancel_request"
)

// types/message.go 新增消息类型
type ControlRequest struct {
    RequestID string `json:"request_id"`
    Request   struct {
        Subtype string         `json:"subtype"`
        ToolName string        `json:"tool_name"`
        Input   map[string]any `json:"input"`
    } `json:"request"`
}

// provider/claude_provider.go 改进 BuildCLIArgs
if cfg.PermissionMode == "ask" {
    args = append(args, "--permission-prompt-tool", "stdio")
}

// session.go 改进事件解析
case "control_request":
    emit EventControlRequest with RequestID, ToolName, Input
    // 同步等待 callback 返回 PermissionResult，写入 stdin
```

**影响文件**：
- `event/events.go` - 新增事件类型
- `types/message.go` - 新增消息类型
- `provider/claude_provider.go` - 新增 `permission-mode=ask` 选项
- `internal/engine/session.go` - 新增 `control_request` 处理分支
- `internal/server/controller.go` - 新增权限响应 API

---

## Issue #5: 可选接口注册表模式

**优先级**: 中

### cc-connect 方案

```go
// core/registry.go
type PlatformFactory func(opts map[string]any) (Platform, error)
type AgentFactory func(opts map[string]any) (Agent, error)

var platformFactories = make(map[string]PlatformFactory)
var agentFactories = make(map[string]AgentFactory)

func RegisterPlatform(name string, factory PlatformFactory)
func RegisterAgent(name string, factory AgentFactory)
func CreatePlatform(name string, opts map[string]any) (Platform, error)

// agent/claudecode/claudecode.go
func init() {
    core.RegisterAgent("claudecode", New)
}
```

Platform/Agent 按需实现可选接口，核心层通过类型断言安全检查：

```go
if engineSupport, ok := adapter.(base.EngineSupport); ok {
    engineSupport.SetEngine(eng)
} else {
    logger.Warn("Adapter does not implement EngineSupport", ...)
}
```

### HotPlex 现状

- `chatapps/slack/adapter.go`：手动编译时检查
- `base/` 下定义接口，但无统一注册表
- 新增 Agent/Platform 类型需在 `setup.go` 或 `manager.go` 手动注册

### 改进方案

```go
// internal/registry/registry.go
type PlatformFactory func(opts map[string]any) (base.ChatAdapter, error)
type AgentFactory func(opts map[string]any) (base.Agent, error)

var platformFactories = make(map[string]PlatformFactory)
var agentFactories = make(map[string]AgentFactory)

func RegisterPlatform(name string, f PlatformFactory)
func RegisterAgent(name string, f AgentFactory)
func CreatePlatform(name string, opts map[string]any) (base.ChatAdapter, error)
func CreateAgent(name string, opts map[string]any) (base.Agent, error)
func AvailablePlatforms() []string
func AvailableAgents() []string

// chatapps/slack/adapter.go
func init() {
    registry.RegisterPlatform("slack", NewAdapter)
}
```

**新增文件**：`internal/registry/registry.go`
**迁移**：`chatapps/` 下所有 adapter 改用 `init()` 自注册

---

## Issue #6: 会话历史读取

**优先级**: 中

### cc-connect 方案

```go
// agent/claudecode/claudecode.go
func (a *Agent) GetSessionHistory(ctx, sessionID, limit) ([]HistoryEntry, error) {
    // 读取 ~/.claude/projects/{projectKey}/{sessionID}.jsonl
    // 解析每个 JSONL 条目，提取 role + text content
    // 返回 []HistoryEntry{Role, Content, Timestamp}
}
```

历史条目按 `role`（user/assistant）和时间戳组织，支持 limit 截断。

### HotPlex 现状

- `types/message_type.go`：仅 `user_input` 和 `final_response` 持久化
- 中间事件（thinking、tool_use）不存储
- `brain/memory.go` 的 `SessionHistory` 仅保存内存中的压缩摘要
- 无从 `.jsonl` 读取原始会话记录的能力

### 改进方案

```go
// internal/engine/session_history.go
type HistoryEntry struct {
    Role      string    // "user" | "assistant" | "system"
    Content   string    // 原始文本或 tool_use 摘要
    Timestamp time.Time
    Type      string    // "text" | "tool_use" | "thinking" | "tool_result"
}

func ReadSessionHistory(sessionID string, limit int) ([]HistoryEntry, error) {
    // 读取 ~/.claude/projects/{projectKey}/{sessionID}.jsonl
    // 解析 JSONL，过滤 user/assistant 条目
    // 返回最后 limit 条（或全部）
}

func GetSessionSummary(sessionID string) (string, int, error) {
    // 返回第一条 user 消息前 40 字符 + 总消息数
}
```

**新增文件**：`internal/engine/session_history.go`
**扩展 API**：`internal/server/hotplex_ws.go` - 新增 `/history` 端点

---

## Issue #7: 多级打字指示器

**优先级**: 中

### cc-connect 方案

Slack 适配器实现渐进式 emoji 反馈：

```go
// platform/slack/slack.go
// 立即：👀 (eyes)
// 2 分钟后：🕐 (clock)
// 每 5 分钟：额外 emoji（⏳🔧💡🚀🧠💎🔬🛰️🔥✨）
// stop()：移除所有反应
```

通过 Slack emoji reaction 实现，无需修改消息内容。

### HotPlex 现状

- `chatapps/slack/adapter.go`：无打字指示器
- `internal/server/hotplex_ws.go`：无 streaming 进度反馈
- WebSocket 推送 `tool_use` 等事件，但无"正在处理"指示

### 改进方案

```go
// chatapps/slack/typing.go
type TypingIndicator struct {
    platform *Platform
    replyCtx replyContext
    emojis   []string
    stopCh   chan struct{}
}

func (p *Platform) StartTyping(ctx, replyCtx) (stop func()) {
    // goroutine: 每阶段添加 emoji reaction
    // goroutine: stopCh 关闭时清理所有 emoji
}

// 阶段
type TypingStage struct {
    After time.Duration
    Emoji string
}
stages := []TypingStage{
    {0, "eyes"},
    {2*time.Minute, "clock1"},
    {7*time.Minute, "hourglass_flowing_sand"},
    // ...
}
```

**新增文件**：`chatapps/slack/typing.go`
**接口**：`base.TypingIndicator` 接口已在 HotPlex 中定义（chatapps/base/interfaces.go），确认实现

---

## Issue #8: 自然语言 Cron + Bot-to-Bot Relay

**优先级**: 低

### cc-connect 方案

System prompt 注入工具命令，AI 自行解析日程：

```go
// cc-connect 在 --append-system-prompt 中注入：
`
## 定时任务
cc-connect cron add --cron "0 6 * * *" --prompt "..." --desc "..."

## Bot 间通信
cc-connect relay send --to <target_project> "message"
`
```

AI 理解意图后调用对应命令，实现"说一句话就设置好定时任务"。

### HotPlex 现状

- 无 Cron 调度系统
- 无 bot-to-bot 通信机制
- `brain/` 有 LLM 调用能力，但无定时触发器

### 改进方案

**Phase 1: Cron 调度**

```go
// internal/cron/scheduler.go
type Scheduler struct {
    engine *Engine
    jobs   map[string]*CronJob
    mu     sync.RWMutex
}

type CronJob struct {
    ID          string
    CronExpr    string      // "0 6 * * *"
    Prompt      string      // 自然语言描述
    SessionKey  string      // 目标会话
    NextRun     time.Time
}

// cron add --cron "0 6 * * *" --prompt "总结 GitHub trending"
// → 解析 cron 表达式，存储 Job，定时触发 hotplex.Execute()
```

**Phase 2: Bot-to-Bot Relay**

```go
// internal/relay/relay.go
type Relay struct {
    sessions map[string]string  // project → session key
}

func (r *Relay) Send(toProject, message string) (string, error) {
    // 发送消息到目标 bot，返回响应
    // 使用 BridgeServer 或内部 session
}
```

**新增文件**：
- `internal/cron/scheduler.go`
- `internal/relay/relay.go`
- `brain/guard.go` 扩展，识别 cron/relay 意图

---

## 实施路线图

| 阶段 | Issue | 工作项 | 复杂度 |
|------|-------|--------|--------|
| P0 | #1 ProviderProxy | 协议兼容层 | 中 |
| P0 | #4 权限协议 | control_request/response | 高 |
| P1 | #2 多Provider | ProviderManager | 中 |
| P1 | #3 BridgeServer | WebSocket 桥接协议 | 高 |
| P2 | #5 注册表 | Plugin 化 | 低 |
| P2 | #6 历史读取 | JSONL 解析 | 中 |
| P2 | #7 打字指示 | 渐进式 emoji | 低 |
| P3 | #8 Cron+Relay | 调度器和Relay | 高 |
