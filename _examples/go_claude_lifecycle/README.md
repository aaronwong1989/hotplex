# Claude 生命周期示例 (Go)

本示例展示 Claude Code 会话的完整生命周期管理，包括冷启动、热复用、进程恢复和手动终止。

## 功能特点

- **冷启动 (Cold Start)**：首次创建会话，初始化持久化进程
- **热复用 (Hot Multiplexing)**：复用已有进程，实现亚秒级延迟
- **进程恢复**：通过 Marker 文件实现崩溃后恢复
- **手动终止**：显式停止会话和底层进程组

## 快速开始

```bash
cd _examples/go_claude_lifecycle
go run main.go
```

## 生命周期阶段

### 阶段 1：冷启动与复用

```go
// 第一次执行 - 冷启动
err = engine.Execute(ctx, cfg, "记住密码是 ALBATROSS", silentCallback)

// 第二次执行 - 热复用（使用相同 SessionID）
// 观察日志：会复用已有进程 PID
err = engine.Execute(ctx, cfg, "密码是什么？", printingCallback)
```

### 阶段 2：进程恢复（持久化）

```go
// 模拟引擎重启
engine.Close()

// 重新创建引擎实例
engine, _ = hotplex.NewEngine(opts)

// Claude CLI 会自动使用 --resume 参数（因为 Marker 文件存在）
err = engine.Execute(ctx, cfg, "我刚重启了，密码是什么？", printingCallback)
```

### 阶段 3：手动终止

```go
err = engine.StopSession(sessionID, "任务完成")
```

### 阶段 4：获取统计信息

```go
stats := engine.GetSessionStats(sessionID)
fmt.Printf("Duration: %dms, Tokens: %d/%d\n",
    stats.TotalDurationMs, stats.InputTokens, stats.OutputTokens)
```

## 关键概念

### SessionID 的作用

- **唯一标识**：每个 SessionID 对应一个独立的 CLI 进程
- **进程复用**：相同 SessionID 的请求会路由到同一进程
- **状态保持**：进程内的对话历史会被保持

### Marker 文件

HotPlex 使用 Marker 文件实现会话持久化：
- 位置：`{WorkDir}/.claude/sessions/`
- 内容：包含 CLI 内部会话标识符
- 恢复：新进程启动时检测到 Marker 会自动恢复会话

### 进程组隔离

- 使用 `Setpgid: true` 创建进程组
- 终止时使用 `-PGID` 杀死整个进程组
- 确保子进程被正确清理

## 配置示例

```go
opts := hotplex.EngineOptions{
    Namespace:    "demo_lifecycle",
    Timeout:      5 * time.Minute,
    IdleTimeout:  10 * time.Second, // 演示时设置较短
    Logger:       logger,
    Provider:     provider,
    BaseSystemPrompt: "你是一个有用的助手。",
}
```

## 系统提示词注入

HotPlex 支持三种系统提示词注入方式，用于定义 AI 的身份、行为规范和任务指令。

### A) BaseSystemPrompt - Engine 级别

定义 AI 的核心身份和输出风格，**会话全程生效**。

```go
baseSystemPrompt := `You are HotPlex, a concise and practical AI coding assistant.

## Core Principles
- Think step by step before taking action
- Provide working code with minimal explanation

## Output Style
- Use bullet points for lists
- Code blocks must have language tags`

opts := hotplex.EngineOptions{
    Namespace:        "demo",
    BaseSystemPrompt: baseSystemPrompt,
}
```

### B) TaskInstructions - Session 级别

每个 turn 追加的指令，用于任务特定约束。

```go
taskInstructions := `## Task Rules
- Always respond in UPPERCASE for secret words
- Keep responses under 3 sentences
- End with an emoji`

cfg := &hotplex.Config{
    SessionID:        "my-session",
    TaskInstructions: taskInstructions,
}
```

### C) InitialPrompt - Session 级别

会话建立时**自动执行**的任务，无需用户发送消息。

```go
cfg := &hotplex.Config{
    SessionID:    "my-session",
    InitialPrompt: "Show me the git status", // 用户加入会话，AI 自动执行
}
```

### 对比总结

| 字段 | 级别 | 触发时机 | 传递方式 |
|------|------|----------|----------|
| `BaseSystemPrompt` | Engine | 会话开始 | `--append-system-prompt` |
| `TaskInstructions` | Session | 每个 turn | 追加到用户输入前 |
| `InitialPrompt` | Session | 会话建立时自动执行 | `--command` CLI 参数 |

## 统计信息说明

| 字段 | 说明 |
|------|------|
| `TotalDurationMs` | 总执行时间（毫秒） |
| `InputTokens` | 输入 Token 数 |
| `OutputTokens` | 输出 Token 数 |
| `TotalCostUSD` | 总费用（美元） |
| `ToolCallCount` | 工具调用次数 |
| `ToolsUsed` | 使用的工具列表 |

## 运行要求

- Go 1.25+
- Claude Code CLI 已安装并认证

## 扩展阅读

- [基础用法示例](../go_claude_basic)
- [OpenCode 生命周期示例](../go_opencode_lifecycle)
- [错误处理示例](../go_error_handling)
