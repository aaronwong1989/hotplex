# Claude Code Provider 集成指南

*查看其他语言: [English](claudecode.md), [简体中文](claudecode_zh.md).*

本文档介绍了如何在 HotPlex 生态系统中集成和使用 **Claude Code**。

## 概述

**Claude Code** 是 HotPlex 的默认驱动程序，也是功能最强大的 Provider。它针对交互式、多轮编码任务进行了高度优化，并与 HotPlex 的 **热复用 (Hot-Multiplexing)** 技术无缝集成，提供毫秒级的响应速度。

> **注：** HotPlex 已全面兼容及测试 **Claude Code v2.1.x** 及以上版本，并完整支持其底层最新的基于 `modelUsage` 的 Token 消耗统计机制。

## 安装

必须通过 npm 安装 Claude Code：

```bash
npm install -g @anthropic-ai/claude-code
```

验证安装：
```bash
claude --version
```

## 认证

在使用 HotPlex 调用 Claude Code 之前，请确保已完成登录认证：
```bash
claude auth login
```

HotPlex 将自动继承当前机器的认证状态。

## 配置

可以通过 `hotplex.ClaudeCodeProvider` 进行配置。

### Provider 配置参数

| 参数                    | 类型       | 说明                                                    |
| :---------------------- | :--------- | :------------------------------------------------------ |
| `DefaultPermissionMode` | `string`   | 设置为 `"bypassPermissions"` 可对工具调用进行自动授权。 |
| `AllowedTools`          | `[]string` | 工具白名单（如 `["Bash", "Read", "Edit"]`）。           |
| `DisallowedTools`       | `[]string` | 工具黑名单，显式禁止使用的工具。                        |
| `Model`                 | `string`   | 覆盖默认模型（如 `claude-3-5-sonnet-20241022`）。       |

## 核心特性

### 1. 热复用 (Hot-Multiplexing)
HotPlex 维护着一个处于活跃状态的 Claude Code 进程池。当请求到达时，它会立即被分发到空闲进程，从而消除了 Node.js CLI 数秒之久的冷启动延迟。

### 2. 会话持久化
HotPlex 使用 **Marker Files (标记文件)** 来跟踪会话状态。如果 HotPlex 引擎重启，它可以通过向 CLI 传递 `--resume` 标志来自动恢复之前的会话。

### 3. 全双工流式传输
HotPlex 将 Claude 复杂的终端输出（包括进度条、加载动画等）标准化为清晰、结构化的 JSON 事件：
- `thinking`: 从 Claude 的推理块中捕获。
- `tool_use`: 当 Claude 调用本地 Shell 或编辑器工具时捕获。
- `answer`: 最终生成的文本回复。

## Go 语言集成示例

```go
package main

import (
	"context"
	"fmt"
	"github.com/hrygo/hotplex"
)

func main() {
	// 1. 初始化 Claude Code Provider (默认方式)
	claudePrv, _ := hotplex.NewClaudeCodeProvider(hotplex.ProviderConfig{
		DefaultPermissionMode: "bypassPermissions",
		AllowedTools:          []string{"Bash", "Read", "Edit"},
	}, nil)

	// 2. 注入引擎
	engine, _ := hotplex.NewEngine(hotplex.EngineOptions{
		Provider: claudePrv,
	})
	defer engine.Close()

	// 3. 执行任务
	ctx := context.Background()
	engine.Execute(ctx, &hotplex.Config{SessionID: "debug-session"}, "修复 main.go 中的 bug", 
		func(eventType string, data any) error {
			if eventType == "answer" {
				fmt.Print(data.(*hotplex.EventWithMeta).EventData)
			}
			return nil
		})
}
```

## 安全与隔离

- **PGID 隔离**: HotPlex 确保每个 Claude 进程及其子进程（例如由 Claude 启动的开发服务器）都能被正确跟踪并在会话结束时终止。
- **指令级 WAF**: HotPlex 会在流传输过程中进行审计，拦截并阻止高风险指令执行。
