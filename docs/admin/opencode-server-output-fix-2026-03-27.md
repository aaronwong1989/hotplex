# OpenCode Server 模式输出异常修复报告

**日期**: 2026-03-27
**问题**: OpenCode Server 模式输出到 Slack 时出现重复、混乱、格式错误

---

## 🔍 问题症状

用户反馈 OpenCode Server 模式输出到 Slack 时出现以下问题：

1. **重复输出** - 相同内容被输出 3+ 次
2. **内部指令泄露** - `[search-mode]`, `[analyze-mode]` 等内部指令被输出
3. **Step 信息混乱** - `Step 0/0: Starting step...` 等进度信息被发送到 channel
4. **Markdown 格式问题** - GitHub 风格 emoji (`:bar_chart:`) 不兼容 Slack

---

## 🎯 根本原因分析

### 1. Step 事件被错误地发送为 Slack 消息

**位置**: `chatapps/engine_handler.go:1287-1354`

**问题代码**:
```go
func (c *StreamCallback) handleStepStart(data any) error {
    // Update status indicator
    c.updateStatusMessage(base.MessageTypeStepStart, StatusStepStartLabel)

    // ❌ BUG: 调用 buildChatMessage 会发送消息到 Slack!
    return c.buildChatMessage(base.MessageTypeStepStart, content, metadata)
}
```

**原因**:
- Step 事件应该**只更新状态指示器**（Slack Assistant Status Bar）
- 但代码错误地调用了 `buildChatMessage`，导致每个 step 都发送一条 channel 消息
- 用户看到: `:arrow_forward: Step 0/0: Starting step...`, `:white_check_mark: Step 0 完成`

### 2. 内部指令泄露

**可能原因**:
- OpenCode 的 system prompt 包含 `[search-mode]`, `[analyze-mode]` 等指令
- 这些指令被 OpenCode 当作 "thinking" 内容输出
- HotPlex 的 `handleThinking` 方法将其发送到 Slack

**需要进一步排查**:
- OpenCode Server 的 system prompt 配置
- OpenCode 是否输出了不该输出的内容

### 3. 重复输出

**可能原因**:
- SSE 事件重复发送（OpenCode Server 端问题）
- Fallback 机制触发（native streaming 失败）
- `handleAnswer` 的累积内容机制问题

### 4. Markdown 格式问题

**原因**:
- 使用 GitHub 风格 emoji（`:bar_chart:`, `:warning:`）
- Slack 不支持这种格式，需要使用 Unicode emoji

---

## ✅ 修复方案

### Fix 1: Step 事件只更新状态，不发送消息

**修改文件**: `chatapps/engine_handler.go`

**修改内容**:
```go
// handleStepStart handles step start events (OpenCode specific)
// STATUS-ONLY: Updates status indicator, does NOT send channel message
func (c *StreamCallback) handleStepStart(data any) error {
    // Extract metadata for logging
    var step, total int32
    var toolName string

    if m, ok := data.(*event.EventWithMeta); ok {
        if m.Meta != nil {
            step = m.Meta.CurrentStep
            total = m.Meta.TotalSteps
            toolName = m.Meta.ToolName
        }
    }

    // Update status indicator (status-only, no channel message)
    if err := c.updateStatusMessage(base.MessageTypeStepStart, StatusStepStartLabel); err != nil {
        c.logger.Warn("Failed to update status for step_start", "error", err)
    }

    // Log step info for debugging
    c.logger.Debug("Step start", "step", step, "total", total, "tool_name", toolName)

    // ✅ FIXED: Return nil - do NOT send channel message
    return nil
}

// handleStepFinish - 同样修改
func (c *StreamCallback) handleStepFinish(data any) error {
    // ... 同样的模式，只更新状态，不发送消息
    return nil
}
```

**影响**:
- ✅ Step 进度信息不再发送到 channel
- ✅ 只更新 Slack Assistant Status Bar
- ✅ 减少 API 调用（每条消息都是一次 API 调用）

---

## 📊 验证结果

### 编译测试

```bash
$ go build ./chatapps/...
# 成功，无错误
```

---

## ⚠️ 剩余问题（需要进一步排查）

### 1. 内部指令泄露 (`[search-mode]`, `[analyze-mode]`)

**排查方向**:
1. 检查 OpenCode Server 的 system prompt 配置
2. 检查 OpenCode 是否将这些指令作为 "thinking" 内容输出
3. 可能需要在 `handleThinking` 中过滤这些指令

**临时解决方案**:
```go
func (c *StreamCallback) handleThinking(data any) error {
    var content string
    if m, ok := data.(*event.EventWithMeta); ok {
        content = m.EventData
    }

    // 🚫 Filter internal directives
    if strings.HasPrefix(content, "[search-mode]") ||
       strings.HasPrefix(content, "[analyze-mode]") {
        c.logger.Debug("Filtering internal directive", "content", content)
        return nil  // Silent drop
    }

    // ... rest of the code
}
```

### 2. 重复输出

**排查方向**:
1. 检查 OpenCode Server 的 SSE event 是否重复
2. 检查 `HTTPTransport.Subscribe()` 的 fan-out 机制
3. 检查 `handleAnswer` 的 fallback 逻辑

**日志调试**:
```go
// 在 HTTPSessionIO.StartReading 中添加
h.logger.Debug("StartReading received event",
    "event_number", eventCount,
    "event_length", len(line),
    "event_hash", sha256.Sum256([]byte(line)))  // 检测重复
```

### 3. Markdown 格式问题

**解决方案**:
- 替换 GitHub emoji 为 Unicode emoji
- `:bar_chart:` → `📊`
- `:warning:` → `⚠️`
- `:white_check_mark:` → `✅`

---

## 🎯 下一步行动

### 立即行动（已完成）
- [x] 修复 Step 事件发送消息的问题
- [x] 编译验证通过

### 短期行动（需要用户协助）
- [ ] 收集完整的日志输出（包括 OpenCode Server 日志）
- [ ] 检查 OpenCode Server 的 system prompt 配置
- [ ] 验证修复后的效果

### 长期优化
- [ ] 添加 internal directive 过滤机制
- [ ] 添加 SSE event deduplication
- [ ] 统一 emoji 格式（全部使用 Unicode）

---

## 📝 相关文件

- `chatapps/engine_handler.go` - 修复的文件
- `provider/opencode_server_provider.go` - OpenCode Server Provider
- `provider/transport_http.go` - HTTP Transport & SSE 处理
- `internal/engine/session_io.go` - Session I/O 处理

---

## 🔗 相关 Issue

- Issue #358 - OpenCode Server Provider 实现
