# OpenCode 生命周期示例 (Go)

本示例展示 OpenCode 会话的完整生命周期管理，包括冷启动、多轮交互、会话持久化和热启动恢复。

## 功能特点

- **冷启动**：初始化全新的会话
- **多轮交互**：在单个会话中进行连续对话
- **会话持久化**：捕获提供商特定的会话标识符
- **热启动恢复**：使用会话 ID 恢复之前的会话
- **进程恢复**：确保会话连续性

## 快速开始

```bash
cd _examples/go_opencode_lifecycle
go run main.go
```

## 生命周期阶段

### 阶段 1：初始冷启动

```go
cfg := &hotplex.Config{
    WorkDir:   "./lifecycle_demo",
    SessionID: "persistent-opencode-task",
}

err = engine.Execute(ctx, cfg,
    "你好！这是我们项目的开始。请创建一个 README.md 文件。",
    callback)
```

### 阶段 2：多轮交互（连续会话）

```go
// 使用相同的 SessionID 继续对话
err = engine.Execute(ctx, cfg,
    "现在添加一个名为'功能'的部分到 README.md。",
    callback)
```

### 阶段 3：模拟恢复（热启动）

```go
// 引擎可能被杀死，使用相同 SessionID 恢复
err = engine.Execute(ctx, cfg,
    "验证 README.md 的内容并告诉我总行数。",
    callback)
```

### 阶段 4：获取统计信息

```go
stats := engine.GetSessionStats(sessionID)
fmt.Printf("Duration: %dms, Tokens: %d/%d\n",
    stats.TotalDurationMs, stats.InputTokens, stats.OutputTokens)
```

## 关键概念

### SessionID vs ProviderSessionID

- **SessionID**：业务层面的会话标识符，用于 HotPlex 进程池查找
- **ProviderSessionID**：CLI 内部的会话标识符，由 OpenCode/Claude Code 管理

### 会话持久化机制

```
1. 执行任务时，HotPlex 创建/查找 SessionID 对应的进程
2. CLI 内部生成 providerSessionID
3. CLI 将 providerSessionID 保存到本地数据库
4. 下次使用相同 SessionID 时，HotPlex 传递 --resume 参数
5. CLI 检测到历史会话，自动恢复
```

### 与 Claude Code 的差异

| 方面 | Claude Code | OpenCode |
|------|------------|----------|
| 会话存储 | `~/.claude/sessions/` | 类似 |
| 恢复参数 | `--resume` | `--resume` |
| 模型选择 | 通过 API 密钥 | 通过配置 |

## 配置示例

```go
opencodePrv, err := hotplex.NewOpenCodeProvider(hotplex.ProviderConfig{
    Type:         hotplex.ProviderTypeOpenCode,
    DefaultModel: "zhipu/glm-5-code-plan",
    OpenCode: &hotplex.OpenCodeConfig{
        PlanMode: true,  // 安全模式
    },
}, logger)

engine, err := hotplex.NewEngine(hotplex.EngineOptions{
    Namespace:        "opencode_lifecycle",
    Provider:        opencodePrv,
    BaseSystemPrompt: "You are a helpful coding assistant.",
})
```

## 系统提示词注入

HotPlex 支持三种系统提示词注入方式，系统提示词通过 `--append-system-prompt`（BaseSystemPrompt）或 `--command`（InitialPrompt）传递给 OpenCode。

### A) BaseSystemPrompt - Engine 级别

```go
baseSystemPrompt := `You are an OpenCode expert.

## Guidelines
- Use Python as primary language
- Provide working code with tests`

opts := hotplex.EngineOptions{
    Namespace:        "opencode_demo",
    BaseSystemPrompt: baseSystemPrompt,
    Provider:         opencodePrv,
}
```

### B) TaskInstructions - Session 级别

```go
cfg := &hotplex.Config{
    SessionID:        "my-session",
    TaskInstructions: "Always write unit tests for your code.",
}
```

### C) InitialPrompt - Session 级别

```go
cfg := &hotplex.Config{
    SessionID:    "my-session",
    InitialPrompt: "Initialize a new Python project with pytest",
}
```

详细说明请参考 [Claude 生命周期示例](../go_claude_lifecycle)。

## 运行要求

- Go 1.25+
- OpenCode CLI 已安装并配置

## 扩展阅读

- [OpenCode 基础示例](../go_opencode_basic)
- [Claude 生命周期示例](../go_claude_lifecycle)
- [HotPlex 提供商文档](../../docs/providers/opencode_zh.md)
