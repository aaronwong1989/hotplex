# OpenCode Basic 示例 (Go)

本示例演示如何使用 HotPlex 控制平面与 OpenCode 提供商集成。OpenCode 是支持多种 LLM 提供商和不同操作模式的 AI CLI 代理。

## 功能特点

- **提供商切换**：轻松切换底层 AI CLI 代理
- **Plan/Build 模式**：配置 OpenCode 特定的操作模式
- **模型配置**：覆盖提供商的默认模型

## 快速开始

```bash
cd _examples/go_opencode_basic
go run main.go
```

## 代码结构

### 1. 创建 OpenCode 提供商

```go
opencodePrv, err := hotplex.NewOpenCodeProvider(hotplex.ProviderConfig{
    Type:         hotplex.ProviderTypeOpenCode,
    DefaultModel: "zhipu/glm-5-code-plan",  // 使用 GLM-5 Code Plan
    OpenCode: &hotplex.OpenCodeConfig{
        PlanMode:   true,   // 规划模式
        UseHTTPAPI: false,  // 使用 CLI 模式
    },
}, logger)
```

### 2. 初始化引擎

```go
opts := hotplex.EngineOptions{
    Timeout:   5 * time.Minute,
    Logger:    logger,
    Namespace: "opencode_demo",
    Provider:  opencodePrv,  // 使用 OpenCode 而非默认的 Claude Code
}

engine, err := hotplex.NewEngine(opts)
```

### 3. 执行任务

```go
cfg := &hotplex.Config{
    WorkDir:   currentDir,
    SessionID: "opencode-session-1",
}

err = engine.Execute(ctx, cfg, prompt, callback)
```

## OpenCode 配置说明

### ProviderConfig

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `Type` | 提供商类型 | ClaudeCode |
| `DefaultModel` | 默认模型 | 提供商默认值 |
| `AllowedTools` | 允许的工具列表 | 全部允许 |

### OpenCodeConfig

| 参数 | 说明 |
|------|------|
| `PlanMode` | 是否使用规划模式（更安全） |
| `UseHTTPAPI` | 是否使用 HTTP API（默认 CLI） |
| `CustomEndpoints` | 自定义 API 端点 |

## 系统提示词注入

HotPlex 支持三种系统提示词注入方式，系统提示词通过 `--append-system-prompt`（BaseSystemPrompt）或 `--command`（InitialPrompt）传递给 OpenCode。

### A) BaseSystemPrompt - Engine 级别

```go
opts := hotplex.EngineOptions{
    Namespace:        "opencode_demo",
    BaseSystemPrompt: "You are a Python expert. Provide concise code examples.",
    Provider:         opencodePrv,
}
```

### B) TaskInstructions - Session 级别

```go
cfg := &hotplex.Config{
    SessionID:        "my-session",
    TaskInstructions: "Always use type hints in Python code.",
}
```

### C) InitialPrompt - Session 级别

```go
cfg := &hotplex.Config{
    SessionID:    "my-session",
    InitialPrompt: "List all files in current directory",
}
```

详细说明请参考 [Claude 生命周期示例](../go_claude_lifecycle)。

## 支持的模型

- `zhipu/glm-5-code-plan` - GLM-5 代码规划模型
- `zhipu/glm-4` - GLM-4 模型
- 其他 OpenCode 支持的模型

## 事件处理

与 Claude Code 示例相同，但 OpenCode 特有的事件：

```go
callback := func(eventType string, data any) error {
    switch eventType {
    case "thinking":
        // 模型思考中
    case "tool_use":
        // 工具调用
    case "answer":
        // 流式输出
    case "session_stats":
        // 统计信息（包含 ModelUsed 字段）
    }
    return nil
}
```

## 运行要求

- Go 1.25+
- OpenCode CLI 已安装

## 扩展阅读

- [OpenCode 生命周期示例](../go_opencode_lifecycle)
- [Claude 基础示例](../go_claude_basic)
- [HotPlex 提供商文档](../../docs/providers/opencode_zh.md)
