# Context Window Calculation Fix

## 问题描述

HotPlex 的上下文窗口使用百分比计算存在问题：

### 根本原因

1. **硬编码的上下文窗口大小**
   - `engine/runner.go` 硬编码了 `contextWindowTokens = 200000`
   - 不同模型有不同的上下文窗口大小，   - 未来模型可能有更大的上下文窗口

2. **类型定义缺失字段**
   - `provider/types.go:ModelUsageStats` 缺少 `contextWindow` 和 `maxOutputTokens` 字段
   - `provider/event.go:ProviderEventMeta` 缺少 `contextWindow` 字段
   - Claude Code CLI 实际提供这些字段，但 HotPlex 未解析

3. **动态数据未传递**
   - Provider 层解析了 `modelUsage` 但未提取 `contextWindow`
   - Engine 层无法获取动态的上下文窗口大小

## 验证结果

通过 Python 脚本验证 Claude Code stream-json 输出，确认 `modelUsage` 提供以下完整字段：

```json
{
  "modelUsage": {
    "claude-sonnet-4-6": {
      "inputTokens": 68581,
      "outputTokens": 1112,
      "cacheReadInputTokens": 896,
      "cacheCreationInputTokens": 0,
      "contextWindow": 200000,  // ✅ 动态提供
      "maxOutputTokens": 32000,       // ✅ 动态提供
      "costUSD": 0.222692,
      "webSearchRequests": 0
    }
  }
}
```

## 修复方案

### 1. 更新类型定义

**provider/types.go**
```go
type ModelUsageStats struct {
    InputTokens              int32
    OutputTokens             int32
    CacheReadInputTokens     int32
    CacheCreationInputTokens int32
    CostUSD                  float64
    ContextWindow            int32   // ✅ 新增
    MaxOutputTokens          int32   // ✅ 新增
}
```

**provider/event.go**
```go
type ProviderEventMeta struct {
    // ... 其他字段 ...
    ContextWindow    int32 `json:"context_window,omitempty"` // ✅ 新增
}
```

### 2. Provider 层提取 contextWindow

**provider/claude_provider.go**
```go
var contextWindow int32
var maxOutputTokens int32

if len(msg.ModelUsage) > 0 {
    for _, mUsage := range msg.ModelUsage {
        // ... 累加 tokens ...

        // ✅ 提取模型限制
        if mUsage.ContextWindow > 0 {
            contextWindow = mUsage.ContextWindow
        }
        if mUsage.MaxOutputTokens > 0 {
            maxOutputTokens = mUsage.MaxOutputTokens
        }
    }
}

// ✅ 设置到 event metadata
if hasModelUsage {
    // ... 其他字段 ...
    if contextWindow > 0 {
        event.Metadata.ContextWindow = contextWindow
    }
}
```

### 3. Engine 层使用动态值

**engine/runner.go**
```go
// ✅ 使用动态值，contextWindowTokens := int32(200000) // Default 200K
if pevt.Metadata != nil && pevt.Metadata.ContextWindow > 0 {
    contextWindowTokens = pevt.Metadata.ContextWindow
}
```

## 计算公式验证

正确的计算公式：
```
total_input = input_tokens + cache_read_tokens + cache_write_tokens
context_used_percent = (total_input / context_window_size) * 100
```

示例验证：
- Input tokens: 68,581
- Cache read: 896
- Cache write: 0
- Context window: 200,000
- **Result**: (68581 + 896 + 0) / 200000 * 100 = **34.74%**

## 测试结果

```bash
=== RUN   TestContextWindowCalculation
=== RUN   TestContextWindowCalculation/Turn_1:_Simple_question
    ✅ Turn 1: Simple question: 34.7765%
=== RUN   TestContextWindowCalculation/Turn_2:_Longer_response
    ✅ Turn 2: Longer response: 33.4075%
=== RUN   TestContextWindowCalculation/Turn_3:_Even_longer
    ✅ Turn 3: Even longer: 33.6475%
--- PASS: TestContextWindowCalculation
```

## 修改的文件

1. `provider/types.go` - 添加 `ContextWindow` 和 `MaxOutputTokens` 字段
2. `provider/event.go` - 添加 `ContextWindow` 字段到 `ProviderEventMeta`
3. `provider/claude_provider.go` - 提取并传递 `contextWindow` 值
4. `engine/runner.go` - 使用动态的 `contextWindow` 值
5. `engine/context_window_test.go` - 添加单元测试验证计算逻辑

## 向后兼容性

- 如果 provider 未提供 `contextWindow`，默认使用 200K
- 如果 `modelUsage` 为空，回退到 `usage` 字段
- 保持所有现有字段和功能

## 未来扩展

现在支持：
- 不同模型的不同上下文窗口大小
- 未来模型可能有更大的上下文窗口（如 1M tokens）
- 动态适应不同 provider 的特性
