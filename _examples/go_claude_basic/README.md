# Claude Basic 示例 (Go)

本示例演示如何使用 HotPlex 控制平面将 Claude Code CLI 集成到您的生产级 Go 应用中。

## 功能特点

- **基础用法**：展示 `HotPlexClient` 的基本使用方式
- **事件流处理**：通过回调函数接收流式事件
- **安全沙箱**：演示权限控制和工具白名单
- **会话管理**：展示如何使用 SessionID 进行进程复用

## 快速开始

```bash
cd _examples/go_claude_basic
go run main.go
```

## 代码结构

### 1. 初始化引擎

```go
opts := hotplex.EngineOptions{
    Timeout:   5 * time.Minute,    // 单次执行最大超时时间
    Logger:    logger,             // 日志记录器
    Namespace: "demo_app",         // 命名空间，用于 UUID 隔离

    // 安全配置
    PermissionMode: "bypass-permissions",  // 或 "default" 为交互模式
    AllowedTools:   []string{"Bash", "Edit"},  // 允许的工具列表
}

engine, err := hotplex.NewEngine(opts)
```

### 2. 配置执行参数

```go
cfg := &hotplex.Config{
    WorkDir:          "/tmp",              // 隔离的工作目录
    SessionID:        "conversation:44",   // 会话标识符
    TaskInstructions: "添加简短注释",        // 本次任务的特殊指令
}
```

### 3. 处理流式事件

```go
cb := func(eventType string, data any) error {
    switch eventType {
    case "thinking":
        // 模型正在思考
    case "tool_use":
        // 工具调用
    case "answer":
        // 流式输出
    case "session_stats":
        // 会话统计
    case "danger_block":
        // WAF 拦截
    }
    return nil
}
```

### 4. 执行任务

```go
err = engine.Execute(ctx, cfg, prompt, cb)
```

## 事件类型说明

| 事件类型 | 说明 |
|---------|------|
| `thinking` | 模型正在思考或规划 |
| `tool_use` | 调用工具（如 bash、read_file） |
| `answer` | 流式返回的文本响应 |
| `session_stats` | 会话完成后的统计数据 |
| `danger_block` | WAF 拦截了危险操作 |

## 配置选项

### EngineOptions

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `Timeout` | 单次执行超时时间 | 5 分钟 |
| `IdleTimeout` | 空闲会话存活时间 | 30 分钟 |
| `Namespace` | UUID 命名空间 | 空字符串 |
| `PermissionMode` | 权限模式 | "default" |
| `AllowedTools` | 允许的工具列表 | 全部允许 |
| `DisallowedTools` | 禁止的工具列表 | 无 |
| `BaseSystemPrompt` | 系统提示词（Engine 级别） | 无 |

### Config

| 参数 | 说明 |
|------|------|
| `WorkDir` | 隔离的工作目录（绝对路径） |
| `SessionID` | 会话标识符，用于进程池查找 |
| `TaskInstructions` | 任务指令，追加到用户提示前 |
| `InitialPrompt` | 会话建立时自动执行的任务 |
| `WAFApproved` | 是否跳过 WAF 检查 |

## 系统提示词注入

HotPlex 支持三种系统提示词注入方式：

### A) BaseSystemPrompt - Engine 级别

定义 AI 身份和输出风格，**会话全程生效**：

```go
opts := hotplex.EngineOptions{
    Namespace:        "demo",
    BaseSystemPrompt: "You are a Go expert. Provide concise answers.",
}
```

### B) TaskInstructions - Session 级别

每个 turn 追加的指令：

```go
cfg := &hotplex.Config{
    SessionID:        "my-session",
    TaskInstructions: "Always use Go 1.25+ features.",
}
```

### C) InitialPrompt - Session 级别

会话建立时自动执行的任务：

```go
cfg := &hotplex.Config{
    SessionID:    "my-session",
    InitialPrompt: "Show me current directory listing",
}
```

详细说明请参考 [完整生命周期示例](../go_claude_lifecycle)。

## 运行要求

- Go 1.25+
- Claude Code CLI 已安装并认证

## 扩展阅读

- [HotPlex 核心概念](../README_zh.md)
- [完整生命周期示例](../go_claude_lifecycle)
- [错误处理示例](../go_error_handling)
