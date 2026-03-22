# Slack 多级打字指示器 & Assistant API 降级策略

## 关联 Issue

- **Issue #324**: `[feat] 多级打字指示器: Slack 渐进式 emoji 反馈`

---

## 1. 调研结论：Slack Free Plan API 权限

### 1.1 Emoji Reactions (`reactions.add`)

| 维度 | 结论 |
|------|------|
| **可用性** | **Free Workspace 可用** |
| **所需 Scope** | `reactions:write` |
| **速率限制** | Tier 3 (50+ req/min) |
| **App Type** | Bot Token / User Token 均可 |

### 1.2 Assistant Status API (`assistant.threads.setStatus`)

| 维度 | 结论 |
|------|------|
| **可用性** | **仅限付费计划** (Pro/Business+/Enterprise Grid) |
| **所需 Scope** | `assistant:write` |
| **Free Plan 结果** | API 返回 `not_allowed` error |

### 1.3 Native Streaming (`chat.startStream`)

| 维度 | 结论 |
|------|------|
| **可用性** | **仅限付费计划** |
| **Free Plan 结果** | 不支持，fallback 到 `chat.postMessage` + `chat.update` |

### 1.4 降级决策矩阵

| 功能 | Paid Workspace | Free Workspace |
|------|----------------|----------------|
| Assistant Status (状态栏) | `assistant.threads.setStatus` | Emoji Reaction 替代 |
| Typing Indicator (打字指示) | `assistant.threads.setStatus` | Emoji Reaction 替代 |
| Native Streaming | `chat.startStream` | `postMessage` + `chat.update` |
| Ephemeral Message | ✅ 可用 | ✅ 可用 |
| Block Kit (Modal/Message) | ✅ 可用 | ✅ 可用 |

---

## 2. 架构设计

### 2.1 核心思路

采用**能力探测 + 策略注入**模式：

```
┌─────────────────────────────────────────────────────────┐
│                    Adapter.SetStatus                      │
│                    (统一入口)                              │
└─────────────────────────────────────────────────────────┘
                          │
            ┌─────────────┴──────────────┐
            ▼                             ▼
   IsAssistantCapable == true    IsAssistantCapable == false
            │                             │
            ▼                             ▼
   assistant.threads.setStatus     StatusIndicatorFallback
   (原生 API)                      (emoji reactions)
```

### 2.2 Config 新增字段

```go
// chatapps/slack/config.go

type Config struct {
    // ... existing fields ...

    // AssistantAPIEnabled controls whether to attempt native Assistant API first.
    // When true (default): probe capability at startup, fall back if not available.
    // When false: always use emoji reaction fallback.
    AssistantAPIEnabled bool

    // ForceAssistantFallback forces emoji reaction fallback regardless of plan.
    // Useful for development or when Assistant API has issues.
    ForceAssistantFallback bool
}
```

### 2.3 Adapter 新增字段

```go
// chatapps/slack/adapter.go

type Adapter struct {
    // ... existing fields ...

    // isAssistantCapable indicates if the workspace supports Assistant API (paid plan)
    isAssistantCapable     bool
    isAssistantCapableOnce sync.Once
    isAssistantCapableErr  error

    // activeIndicators tracks active emoji typing indicators per channel+thread
    activeIndicators sync.Map // map[string]*TypingIndicator
}
```

### 2.4 接口扩展

```go
// chatapps/base/types.go

// StatusProvider 扩展: 添加状态文本到 emoji reaction 的映射
const (
    // StatusEmojiMap maps StatusType to emoji name for fallback
    StatusEmojiMap = map[base.StatusType]string{
        base.StatusInitializing: "hourglass_flowing_sand",
        base.StatusThinking:     "brain",
        base.StatusToolUse:      "gear",
        base.StatusToolResult:   "wrench",
        base.StatusAnswering:    "pencil",
        base.StatusIdle:         "white_circle",
    }
)
```

---

## 3. 实现方案

### 3.1 能力探测 (Startup)

```go
// ProbeAssistantCapability attempts a lightweight Assistant API call to determine
// if the workspace supports it. Called once at startup.
func (a *Adapter) ProbeAssistantCapability(ctx context.Context) bool {
    if a.config.ForceAssistantFallback {
        return false
    }
    if !a.config.AssistantAPIEnabled {
        return false
    }

    // Try a no-op status set to check capability
    params := slack.AssistantThreadsSetStatusParameters{
        Status: "", // Clear any status
    }
    err := a.client.SetAssistantThreadsStatusContext(ctx, params)
    if err != nil {
        // Check if it's a paid-plan error
        if strings.Contains(err.Error(), "not_allowed") ||
           strings.Contains(err.Error(), "not_allowed_token_type") {
            a.Logger().Warn("Assistant API not available (free workspace?), falling back to emoji reactions")
            return false
        }
        // Other errors might be transient, still try native
        a.Logger().Warn("Assistant API probe failed, using fallback", "error", err)
        return false
    }
    return true
}
```

### 3.2 降级策略：Emoji Reaction TypingIndicator

```go
// chatapps/slack/typing.go

// TypingStage defines a stage in the multi-stage typing indicator
type TypingStage struct {
    After  time.Duration
    Emoji string
}

// DefaultStages is the multi-stage progression for emoji typing indicators
var DefaultStages = []TypingStage{
    {0 * time.Second, "eyes"},                          // AI saw the message
    {2 * time.Minute, "clock1"},                       // Taking a while
    {7 * time.Minute, "hourglass_flowing_sand"},        // Long wait
    {12 * time.Minute, "gear"},                        // Processing complex task
    {17 * time.Minute, "hourglass_flowing_sand"},      // Still going...
}

// TypingIndicator manages emoji reactions for a single typing session
type TypingIndicator struct {
    adapter   *Adapter
    channelID string
    threadTS  string
    messageTS string // Message to react to (anchor)
    stages    []TypingStage

    mu   sync.Mutex
    done bool
    added []string // Track added reactions for cleanup

    stopCh chan struct{}
}

// NewTypingIndicator creates a new typing indicator
func NewTypingIndicator(adapter *Adapter, channelID, threadTS, messageTS string) *TypingIndicator {
    return &TypingIndicator{
        adapter:   adapter,
        channelID: channelID,
        threadTS:  threadTS,
        messageTS: messageTS,
        stages:    DefaultStages,
        added:    make([]string, 0, len(DefaultStages)),
        stopCh:   make(chan struct{}),
    }
}

// Start begins the multi-stage typing indicator
func (ti *TypingIndicator) Start(ctx context.Context) {
    ti.addReaction(ctx, ti.stages[0].Emoji) // Always add eyes first

    for i := 1; i < len(ti.stages); i++ {
        stage := ti.stages[i]
        select {
        case <-time.After(stage.After):
            ti.mu.Lock()
            if ti.done {
                ti.mu.Unlock()
                return
            }
            ti.mu.Unlock()
            ti.addReaction(ctx, stage.Emoji)
        case <-ti.stopCh:
            return
        }
    }
}

// Stop stops the indicator and cleans up all reactions
func (ti *TypingIndicator) Stop(ctx context.Context) {
    ti.mu.Lock()
    if ti.done {
        ti.mu.Unlock()
        return
    }
    ti.done = true
    close(ti.stopCh)
    added := make([]string, len(ti.added))
    copy(added, ti.added)
    ti.mu.Unlock()

    // Remove all reactions (reverse order not required)
    for _, emoji := range added {
        ti.removeReaction(ctx, emoji)
    }
}

func (ti *TypingIndicator) addReaction(ctx context.Context, emoji string) {
    err := ti.adapter.client.AddReactionContext(ctx, emoji, slack.ItemRef{
        Channel:   ti.channelID,
        Timestamp: ti.messageTS,
    })
    if err != nil {
        ti.adapter.Logger().Debug("Failed to add reaction", "emoji", emoji, "error", err)
        return
    }
    ti.mu.Lock()
    ti.added = append(ti.added, emoji)
    ti.mu.Unlock()
    ti.adapter.Logger().Debug("Added typing indicator", "emoji", emoji)
}

func (ti *TypingIndicator) removeReaction(ctx context.Context, emoji string) {
    err := ti.adapter.client.RemoveReactionContext(ctx, emoji, slack.ItemRef{
        Channel:   ti.channelID,
        Timestamp: ti.messageTS,
    })
    if err != nil {
        ti.adapter.Logger().Debug("Failed to remove reaction", "emoji", emoji, "error", err)
    }
}
```

### 3.3 SetStatus 降级集成

```go
// SetStatus implements base.StatusProvider
// Tries native Assistant API first, falls back to emoji reactions
func (a *Adapter) SetStatus(ctx context.Context, channelID, threadTS string, status base.StatusType, text string) error {
    if a.client == nil {
        return fmt.Errorf("slack client not initialized")
    }

    // Check capability once
    if !a.isAssistantCapable {
        return a.setStatusWithEmojiFallback(ctx, channelID, threadTS, status)
    }

    // Try native Assistant API
    err := a.SetAssistantStatus(ctx, channelID, threadTS, text)
    if err != nil {
        // Fall back to emoji if native API fails
        if strings.Contains(err.Error(), "not_allowed") {
            a.isAssistantCapable = false // Don't retry
            return a.setStatusWithEmojiFallback(ctx, channelID, threadTS, status)
        }
        return err
    }
    return nil
}

// setStatusWithEmojiFallback uses emoji reactions as status indicator
func (a *Adapter) setStatusWithEmojiFallback(ctx context.Context, channelID, threadTS string, status base.StatusType) error {
    emoji, ok := base.StatusEmojiMap[status]
    if !ok || emoji == "" {
        return nil // No emoji for this status
    }

    // For status feedback, we need a message to react to
    // If threadTS is empty, we can't react - skip
    if threadTS == "" {
        a.Logger().Debug("No thread_ts for emoji fallback, skipping")
        return nil
    }

    return a.client.AddReactionContext(ctx, emoji, slack.ItemRef{
        Channel:   channelID,
        Timestamp: threadTS,
    })
}

// ClearStatus implements base.StatusProvider
func (a *Adapter) ClearStatus(ctx context.Context, channelID, threadTS string) error {
    if a.isAssistantCapable {
        err := a.SetAssistantStatus(ctx, channelID, threadTS, "")
        if err == nil || !strings.Contains(err.Error(), "not_allowed") {
            return err
        }
    }
    // Fallback: no-op (emoji reactions auto-cleanup on next SetStatus or timeout)
    return nil
}
```

### 3.4 能力探测触发时机

在 `NewAdapter` 末尾调用：

```go
// Probe assistant capability at startup (async, non-blocking)
go func() {
    probeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    a.isAssistantCapable = a.ProbeAssistantCapability(probeCtx)
}()
```

---

## 4. 影响文件

| 操作 | 文件 | 说明 |
|------|------|------|
| 新增 | `chatapps/slack/typing.go` | TypingIndicator 多阶段 emoji 实现 |
| 修改 | `chatapps/slack/config.go` | 新增 `AssistantAPIEnabled` / `ForceAssistantFallback` 配置 |
| 修改 | `chatapps/slack/adapter.go` | 集成能力探测 + 降级逻辑 |
| 修改 | `chatapps/slack/messages.go` | 重构 `SetStatus` / `ClearStatus` 降级 |
| 修改 | `chatapps/base/types.go` | 新增 `StatusEmojiMap` |
| 新增 | `chatapps/slack/typing_test.go` | TypingIndicator 单元测试 |

---

## 5. 配置示例

```yaml
# configs/chatapps/slack-dev.yaml
slack:
  assistant_api_enabled: true      # 启用 Assistant API 探测
  force_assistant_fallback: false # 不强制降级

# Free workspace (或开发环境):
# 设置 force_assistant_fallback: true 跳过探测，直接使用 emoji
```

---

## 6. 风险与限制

- **Rate Limit**: `reactions.add` Tier 3 (50+/min)，TypingIndicator 每个 session 每 2 分钟最多添加 1 个 emoji，风险低
- **Message TS**: Emoji reaction 需要锚定消息，`threadTS` 为空时降级为 no-op
- **清理时机**: Emoji reactions 不会自动消失，需要在 `StopStream` 或下一条消息时清理
- **状态覆盖**: 多个状态变更时，emoji 会叠加（eyes + brain 同时显示），这是预期行为
