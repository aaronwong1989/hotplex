# HotPlex 平台解耦实施规划

> 基于 cc-connect 竞品分析 Issue 调研 & HotPlex 现状分析
> 日期：2026-03-21 | 状态：Phase 1 ✅ 完成 (commit 4ae33b3)

---

## 1. 背景与收益分析

### 1.1 竞品分析来源 Issue 总览

| Issue | 类型 | 主题 | 状态 | 与本文关联 |
|-------|------|------|------|-----------|
| **#320** | feat | BridgeServer: 平台解耦 WebSocket 协议 | OPEN | 🔴 核心 |
| **#322** | feat | Platform/Agent 注册表: Plugin 化架构 | OPEN | 🔴 核心 |
| **#318** | feat | ProviderProxy: 第三方 API 协议兼容层 | OPEN | 🟡 相关 |
| **#319** | feat | 多Provider切换: ProviderManager 运行时切换 | OPEN | 🟡 相关 |
| **#327** | feat | Feishu 平台 DangerBlock 适配 | OPEN | 🟡 相关 |
| **#324** | feat | 多级打字指示器 | OPEN | 🟢 独立 |
| **#323** | feat | 会话历史读取 | OPEN | 🟢 独立 |
| **#321** | feat | Claude Code 权限协议原生支持 | OPEN | 🟢 独立 |
| #88 | refactor | ChatApps DRY SOLID 架构重构 | ✅ CLOSED | 已有基础 |
| #57 | refactor | 整洁架构重构 - Use Cases + Ports | ✅ CLOSED | 已有基础 |
| #51 | feat | Hotplex 架构优化方向调研报告 | ✅ CLOSED | 已有基础 |
| #329 | core | 通用 Human-in-the-Loop 框架 | ✅ CLOSED | 已有基础 |

### 1.2 当前耦合问题总结（现状分析）

| 耦合点 | 文件 | 严重度 | 说明 |
|--------|------|--------|------|
| Dual MessageType 系统 | `types/` vs `chatapps/base/` | 🔴 Critical | 两套重复定义，边界模糊 |
| Slack 常量泄漏到通用 Processor | `processor_chunk.go:40` | 🔴 Critical | `slack.SlackTextLimit` 硬编码 |
| Slack 平台判断泄漏 | `processor_format.go:61` | 🔴 Critical | `switch msg.Platform { case "slack" }` |
| FeishuEngineSupport 在 base 层 | `base/adapter.go:523` | 🟠 High | 以平台命名的接口污染基础层 |
| streamWriter 状态在 engine_handler | `engine_handler.go:828` | 🟠 High | Slack streaming 状态与通用回调混杂 |
| 元数据 key 使用 Slack 术语 | `engine_handler.go:1296` | 🟠 High | `channel_id`, `thread_ts` 是 Slack 概念 |
| RichContent 字段类型为 `any` | `base/types.go:88` | 🟡 Medium | 无编译期类型安全 |

### 1.3 收益量化分析

| 改进项 | 当前状态 | 改进后状态 | 收益 |
|--------|----------|------------|------|
| 新平台接入时间 | 3-5 天（需修改 core） | 2 小时（纯外部 Adapter） | **15x 提升** |
| Slack/Feishu 共用 Processor | 需两处修改 | 一处修改，全局生效 | **维护成本 -50%** |
| Provider 切换 | 需重启进程 | 运行时切换 | **停机时间 0** |
| 单元测试覆盖 | 无法 mock adapter | 可注入 mock | **测试覆盖率 +30%** |
| cc-connect 对齐 | 架构差距 2 代 | 架构差距 <0.5 代 | **竞争力提升** |

---

## 2. 实施架构设计

### 2.1 目标架构

```
┌─────────────────────────────────────────────────────────────┐
│                     External Platforms                       │
│         (Slack, Feishu, Telegram, Discord, Ding, ...)         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ WebSocket
┌─────────────────────────────────────────────────────────────┐
│                    BridgeServer (#320)                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Generic Wire Protocol: register / message / reply    │  │
│  │  Capability Declaration: text, image, buttons, card    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Platform Registry (#322)                    │
│  ┌────────────┐ ┌────────────┐ ┌──────────────────────┐  │
│  │  Slack     │ │  Feishu    │ │  External (Future)   │  │
│  │  Adapter   │ │  Adapter   │ │  via BridgeClient   │  │
│  └────────────┘ └────────────┘ └──────────────────────┘  │
│         │              │                    │                │
│         └──────────────┼────────────────────┘                │
│                        ▼                                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  base.ChatAdapter (Plugin Registration via init())     │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Core Engine (Platform-Agnostic)            │
│  SessionPool │ Provider │ Brain │ Security │ Persistence     │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 三阶段实施路径

```
Phase 1 (Foundation):  消除 processor 耦合 + 统一 MessageType
Phase 2 (Architecture): Plugin Registry + StreamStrategy 抽象
Phase 3 (Advanced):    BridgeServer + 外部平台生态
```

---

## 3. Phase 1: 消除 Processor 耦合 + 统一 MessageType

### 3.1 Issue 对应关系

- 对应 Issue: **#88**（已 CLOSED，但部分未落实）、**#57**（已 CLOSED）
- 核心目标：消除 processor 层 Slack 硬编码

### 3.2 任务 1F: 统一 MessageType 定义

**问题**：`types/message_type.go` 和 `chatapps/base/types.go` 各有一套 `MessageType` 定义。

**操作**：
1. 确定 `types/message_type.go` 为权威定义（storage 层依赖它）
2. `chatapps/base/types.go` 中的 `MessageType` 改为从 `types` 包导入
3. `chatapps/slack/messages.go` 中移除 `types.MessageType(msg.Type).IsStorable()` 的类型转换，直接用 `types.MessageType`

**文件变更**：
- `chatapps/base/types.go` — 删除重复的 `MessageType` 定义，改为 `import "github.com/hrygo/hotplex/types"`
- `chatapps/slack/messages.go` — 移除类型转换，直接用 `types.MessageType`
- `types/message_type.go` — 补充缺失的 `MessageTypeUserInput` / `MessageTypeFinalResponse` 到 base

### 3.3 任务 1G: 提取 `ContentConverter` 接口

**问题**：`processor_format.go:61` 的 `switch msg.Platform { case "slack" }` 硬编码。

**新增文件**：`chatapps/base/converter.go`

```go
// ContentConverter 将 Markdown 转换为平台特定格式
type ContentConverter interface {
    // ConvertMarkdownToPlatform 将 Markdown 文本转换为平台原生格式
    // parseMode 为 None 时返回原文
    ConvertMarkdownToPlatform(content string, parseMode ParseMode) string

    // EscapeSpecialChars 转义平台特殊字符
    EscapeSpecialChars(text string) string
}

// ContentConverterFunc 适配器：将函数转为接口
type ContentConverterFunc func(string, ParseMode) string

func (f ContentConverterFunc) ConvertMarkdownToPlatform(content string, parseMode ParseMode) string {
    return f(content, parseMode)
}
func (f ContentConverterFunc) EscapeSpecialChars(text string) string { return text }
```

**修改**：`processor_format.go`
- 注入 `converter ContentConverter` via `FormatProcessorOptions`
- 移除 `switch msg.Platform` 和 `escapeSlackChars`
- 每个 adapter 在初始化时注册自己的 converter

### 3.4 任务 1H: 提取 `Chunker` 接口

**问题**：`processor_chunk.go:9` 直接 import slack 包，`slack.SlackTextLimit` 硬编码。

**新增文件**：`chatapps/base/chunker.go`

```go
// Chunker 将消息分块以满足平台限制
type Chunker interface {
    // ChunkText 将文本分块，每块不超过平台限制
    ChunkText(text string, limit int) []string

    // MaxChars 返回平台单条消息的最大字符数
    MaxChars() int
}
```

**修改**：`processor_chunk.go`
- 移除 `import "github.com/hrygo/hotplex/chatapps/slack"`
- 注入 `chunker Chunker` via `ChunkProcessorOptions`
- 提供默认实现：`DefaultChunker{MaxChars: 4000}` 对应 Slack

### 3.5 任务 1I: 重命名 `FeishuEngineSupport` → `EngineSupportWithBotID`

**问题**：`base/adapter.go:523` 以平台命名接口，污染 base 层。

**操作**：
```go
// Before (base/adapter.go)
type FeishuEngineSupport interface {
    SetEngineWithBotID(eng *engine.Engine, botID string)
}

// After
// EngineSupportWithBotID is implemented by adapters that require
// both engine injection and bot identity for permission management.
type EngineSupportWithBotID interface {
    EngineSupport
    SetBotID(botID string)
}
```

**修改传播**：
- `chatapps/feishu/adapter.go` — 实现改名后的接口
- `chatapps/setup.go` — 更新类型断言

### 3.6 任务 1J: 统一元数据 Key

**问题**：`engine_handler.go:828` 使用 Slack 术语 `channel_id`, `thread_ts`。

**新增**：`chatapps/base/metadata.go`

```go
// Platform 元数据 key 定义（统一命名）
const (
    KeyRoomID   = "room_id"    // 通用：channel_id / chat_id / room_id
    KeyThreadID  = "thread_id"  // 通用：thread_ts / message_thread_id
    KeyUserID    = "user_id"    // 通用：user_id / from_id / open_id
    KeyBotUserID = "bot_user_id" // Bot 自身 ID
    KeyPlatform  = "platform"   // 平台标识：slack / feishu
)

// ToSlackMetadata 将统一元数据转换为 Slack 特定格式
func ToSlackMetadata(m map[string]any) (channelID, threadTS, userID string) {
    channelID, _ = m[KeyRoomID].(string)
    threadTS, _  = m[KeyThreadID].(string)
    userID, _    = m[KeyUserID].(string)
    return
}
```

**修改**：
- `engine_handler.go` — 使用统一 key，从 `channel_id` → `room_id`
- `chatapps/slack/adapter.go` — 适配转换函数
- `chatapps/feishu/adapter.go` — 适配转换函数
- 所有 `metadata["channel_id"]` → `metadata["room_id"]`

### 3.7 Phase 1 变更范围汇总

| 任务 | 操作类型 | 新增文件 | 修改文件 |
|------|----------|----------|----------|
| 1F MessageType 统一 | 重构 | - | 3 个 |
| 1G ContentConverter | 新增接口 | 1 个 | 2 个 |
| 1H Chunker | 新增接口 | 1 个 | 2 个 |
| 1I FeishuEngineSupport 重命名 | 重构 | - | 3 个 |
| 1J 元数据 Key 统一 | 重构 | 1 个 | ~8 个 |

---

## 4. Phase 2: Plugin Registry + StreamStrategy 抽象

### 4.1 Issue 对应关系

- 对应 Issue: **#322**（Platform/Agent 注册表 Plugin 化）
- 依赖 Phase 1 完成

### 4.2 任务 2A: Platform Registry（Plugin 自注册）

**Issue #322 核心**：`init()` 自注册替代硬编码 map。

**新增文件**：`chatapps/base/registry.go`

```go
package base

// PlatformRegistry holds all registered ChatAdapter factories.
type PlatformRegistry struct {
    mu       sync.RWMutex
    adapters map[string]AdapterFactory
}

type AdapterFactory func(cfg *PlatformConfig, log *slog.Logger) (ChatAdapter, error)

// globalRegistry is the package-level registry populated by init() functions.
var globalRegistry = &PlatformRegistry{adapters: make(map[string]AdapterFactory)}

// RegisterPlatform registers a platform adapter factory.
// Called from each adapter's init() via import side-effect.
func RegisterPlatform(name string, factory AdapterFactory) {
    globalRegistry.Register(name, factory)
}

func (r *PlatformRegistry) Register(name string, factory AdapterFactory) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.adapters[name]; ok {
        panic("duplicate platform registration: " + name)
    }
    r.adapters[name] = factory
}

func (r *PlatformRegistry) Get(name string) (AdapterFactory, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    f, ok := r.adapters[name]
    return f, ok
}

func (r *PlatformRegistry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]string, 0, len(r.adapters))
    for n := range r.adapters {
        names = append(names, n)
    }
    return names
}
```

**Adapter 端修改**（以 Slack 为例）：

```go
// chatapps/slack/adapter.go
func init() {
    base.RegisterPlatform("slack", newAdapter)
}
```

**Core 端修改**（`chatapps/setup.go`）：

```go
// Before: hardcoded adapter map
var adapters = map[string]func(...) base.ChatAdapter{
    "slack":  newSlackAdapter,
    "feishu": newFeishuAdapter,
}

// After: use registry
factory, ok := base.GlobalRegistry().Get(platformName)
if !ok {
    return nil, fmt.Errorf("unsupported platform: %s (available: %v)", platformName, base.GlobalRegistry().List())
}
adapter, err := factory(cfg, log)
```

### 4.3 任务 2B: StreamStrategy 接口抽象

**问题**：`engine_handler.go` 中的 `streamWriter` 状态和 Slack streaming 逻辑混杂在通用 `StreamCallback` 中。

**新增文件**：`chatapps/base/stream.go`

```go
// StreamStrategy 定义平台特定的流式输出策略
type StreamStrategy interface {
    // StartStreaming 初始化流式输出，返回 writer
    StartStreaming(ctx context.Context, msg *ChatMessage) (StreamingSession, error)

    // ShouldUseStreaming 根据消息内容和平台特性判断是否启用流式输出
    ShouldUseStreaming(msg *ChatMessage) bool
}

// StreamingSession 管理一次流式输出会话
type StreamingSession interface {
    // Write 向流式会话写入内容块
    Write(ctx context.Context, chunk []byte) error

    // Flush 推送累积内容到平台
    Flush(ctx context.Context) error

    // Close 结束流式会话
    Close(ctx context.Context) error
}

// DefaultStreamStrategy 默认策略：直接写入，无流式处理
type NoOpStreamStrategy struct{}

func (NoOpStreamStrategy) StartStreaming(ctx context.Context, msg *ChatMessage) (StreamingSession, error) {
    return NoOpStreamingSession{}, nil
}
func (NoOpStreamStrategy) ShouldUseStreaming(msg *ChatMessage) bool { return false }

type NoOpStreamingSession struct{}
func (NoOpStreamingSession) Write(ctx context.Context, chunk []byte) error { return nil }
func (NoOpStreamingSession) Flush(context.Context) error                  { return nil }
func (NoOpStreamingSession) Close(context.Context) error                  { return nil }
```

**修改**：`engine_handler.go` 的 `StreamCallback` 结构体
- 注入 `streamStrategy StreamStrategy` 替代直接持有 `streamWriter`
- Slack adapter 提供 `SlackStreamStrategy` 实现
- Feishu adapter 提供 `NoOpStreamStrategy`（当前无 native streaming）

### 4.4 Phase 2 变更范围汇总

| 任务 | 操作类型 | 新增文件 | 修改文件 |
|------|----------|----------|----------|
| 2A Platform Registry | 新增 | 1 个 | 4 个 |
| 2B StreamStrategy | 新增 | 1 个 | 3 个 |

---

## 5. Phase 3: BridgeServer + 外部平台生态

### 5.1 Issue 对应关系

- 对应 Issue: **#320**（BridgeServer WebSocket 协议）
- 这是 cc-connect 对齐的核心交付物

### 5.2 任务 3A: BridgeServer 协议

**Issue #320 核心**：实现通用 WebSocket 协议，让外部平台 Adapter 无需修改 core 代码即可接入。

**新增文件**：`internal/server/bridge.go`

```go
// BridgeServer provides a WebSocket gateway for external platform adapters.
// Adapters connect via WebSocket, declare capabilities, then communicate
// via the generic BridgePlatform protocol.
type BridgeServer struct {
    httpServer *http.Server
    upgrader   websocket.Upgrader
    registry   *base.PlatformRegistry  // external adapters register here
    engine     *engine.Engine
    log        *slog.Logger
}

// Bridge Wire Protocol (JSON over WebSocket):
//
// Registration:
//   {"type":"register","platform":"dingtalk","capabilities":["text","image","buttons","card","typing"]}
//
// Outbound (core → adapter):
//   {"type":"message","session_id":"...","content":"...","metadata":{"room_id":"...","user_id":"..."}}
//
// Inbound (adapter → core):
//   {"type":"reply","session_id":"...","user_input":"...","metadata":{}}
//
// Error:
//   {"type":"error","code":400,"message":"..."}
//
// BridgePlatform implements base.ChatAdapter and translates wire protocol
// to internal ChatMessage / SessionEvent types.
type BridgePlatform struct {
    conn       *websocket.Conn
    platform   string
    caps       []string
    msgChan    chan *base.ChatMessage
    eventChan  chan *event.Callback
    done       chan struct{}
}
```

**Capability 声明协议**：

```go
// PlatformCapability 声明平台支持的功能
type PlatformCapability string

const (
    CapText     PlatformCapability = "text"
    CapImage    PlatformCapability = "image"
    CapButtons  PlatformCapability = "buttons"
    CapCard     PlatformCapability = "card"
    CapTyping   PlatformCapability = "typing"
    CapEdit     PlatformCapability = "edit"      // 编辑已发送消息
    CapDelete   PlatformCapability = "delete"    // 删除消息
    CapReact    PlatformCapability = "react"      // 表情反应
    CapThread   PlatformCapability = "thread"    // 线程支持
)

// RegisterExternal 外部 BridgeClient 调用此方法接入
func (s *BridgeServer) RegisterExternal(conn *websocket.Conn, platform string, caps []PlatformCapability) {
    bridge := &BridgePlatform{
        conn:      conn,
        platform:  platform,
        caps:      caps,
        msgChan:   make(chan *base.ChatMessage, 100),
        eventChan: make(chan *event.Callback, 100),
        done:      make(chan struct{}),
    }
    s.registry.RegisterExternal(platform, bridge)
    go bridge.readLoop()
}
```

### 5.3 任务 3B: BridgeClient SDK（供外部平台使用）

**新增目录**：`cmd/bridge-client/`

提供 Go SDK 让外部平台 Adapter 开发者快速接入：

```go
// Example: DingTalk adapter using BridgeClient SDK
func main() {
    client := bridgeclient.New(
        bridgeclient.URL("wss://hotplex.internal:8080/bridge"),
        bridgeclient.Platform("dingtalk"),
        bridgeclient.Capabilities(bridgeclient.CapText, bridgeclient.CapCard),
        bridgeclient.AuthToken(os.Getenv("HOTPLEX_BRIDGE_TOKEN")),
    )

    client.OnMessage(func(msg *bridgeclient.Message) *bridgeclient.Reply {
        reply := hotplex.ProcessMessage(msg.Content, msg.SessionID)
        return &bridgeclient.Reply{
            Content:    reply.Text,
            SessionID:  msg.SessionID,
        }
    })

    client.Connect(context.Background())
}
```

### 5.4 Phase 3 里程碑与验收标准

| 里程碑 | 验收标准 |
|--------|----------|
| BridgeServer 协议完成 | WebSocket 连接建立，register 握手成功 |
| Slack 通过 BridgeServer 接入 | 现有 Slack adapter 重构为 BridgeClient，行为不变 |
| 外部 DingTalk adapter 示例 | 新平台接入无需修改 core，< 2 小时 |
| 性能回归测试 | 延迟增加 < 5ms，吞吐量无显著下降 |

---

## 6. 风险与依赖

### 6.1 实施风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Phase 1 元数据 key 重命名导致运行时崩溃 | 高 | 迁移期双向兼容：`room_id` 优先，回退 `channel_id` |
| Phase 2 Registry 破坏现有 adapter | 高 | 先加后删：先添加 Registry，保留旧 map，再删除旧代码 |
| Phase 3 BridgeServer 影响现有 WS 性能 | 中 | 在 setup gate 后再启用，不影响现有路径 |
| Feishu adapter 与 Slack 解耦后行为差异 | 中 | StreamStrategy 提供 NoOp fallback |

### 6.2 依赖关系

```
Phase 1 (Foundation) ✅ 全部完成
├── 1F MessageType 统一 ✅
├── 1G ContentConverter ✅
├── 1H Chunker ✅
├── 1I FeishuEngineSupport 重命名 ✅
└── 1J 元数据 Key 统一 ✅

Phase 2 (Architecture) ←── 依赖 Phase 1 全部完成 ✅
├── 2A Platform Registry ←── 依赖 1F, 1I
└── 2B StreamStrategy ←── 依赖 1J

Phase 3 (Advanced) ←── 依赖 Phase 2 完成
├── 3A BridgeServer
└── 3B BridgeClient SDK
```

---

## 7. 实施顺序建议

### 建议按以下顺序执行（考虑到风险最小化）：

**Step 1: 1F + 1I（最小侵入）**
- 统一 MessageType，消除 base 包重复定义
- 重命名 FeishuEngineSupport
- 风险：极低，不改变运行时行为

**Step 2: 1J（元数据 key 统一）**
- 引入统一 key，保留向后兼容回退
- 风险：低

**Step 3: 1G + 1H（Processor 接口提取）**
- 最关键的 Slack 耦合消除
- 风险：中（需要修改 processor 调用方）

**Step 4: 2B（StreamStrategy 抽象）**
- 将 Slack streaming 逻辑从 engine_handler 抽离
- 风险：中

**Step 5: 2A（Platform Registry）**
- Plugin 自注册系统
- 风险：低到中

**Step 6: 3A + 3B（BridgeServer）**
- 对齐 cc-connect 的核心架构
- 风险：中（新增功能，不破坏现有）

---

## 8. 与 cc-connect 竞品对标

| cc-connect 特性 | HotPlex 当前 | 目标状态 | 对应 Issue |
|-----------------|-------------|----------|-----------|
| `core/bridge.go` WebSocket 协议 | 无 | BridgeServer 实现 | #320 |
| `core/registry.go` Plugin 注册 | 硬编码 map | PlatformRegistry | #322 |
| `core/providerproxy.go` API 兼容层 | 无 | ProviderProxy | #318 |
| `core/interfaces.go` ProviderSwitcher | 单 Provider | ProviderManager | #319 |
| `platform/slack/slack.go` StartTyping | 无 | 多级打字指示器 | #324 |
| `agent/claudecode/session.go` GetSessionHistory | 基础 | 完整历史读取 | #323 |

---

## 9. 总结

本次规划基于 cc-connect 竞品分析的 12 个相关 Issue 和 HotPlex 代码库的全面现状分析，识别了 **7 个高严重度耦合点** 和 **6 个中严重度耦合点**。

**Phase 1**（Foundation）在不破坏现有功能的前提下，通过接口提取和类型统一，消除 processor 层的 Slack 硬编码，预计收益：
- 新平台接入时间从 3-5 天降至 1 天
- Processor 层维护成本降低 50%

**Phase 2**（Architecture）引入 Plugin Registry 和 StreamStrategy 抽象，实现 HotPlex 的架构现代化，对齐 cc-connect 的设计理念。

**Phase 3**（Advanced）实现 BridgeServer + BridgeClient SDK，将 HotPlex 从"内置平台"模式转变为"平台生态"模式，这是与 cc-connect 竞争的核心差异化能力。

**预计总工时**：Phase 1 约 2-3 周，Phase 2 约 2-3 周，Phase 3 约 3-4 周。三个阶段可独立验证，每阶段交付可用功能。
