# 🔥 HotPlex (Hot-Multiplexer)

*Read this in other languages: [English](README.md), [简体中文](README_zh.md).*

**HotPlex** 是一个高性能的**进程多路复用器 (Process Multiplexer)**，专为在长生命周期的服务器或 Web 环境中运行繁重的本地 AI CLI 代理（如 Claude Code、OpenCode、Aider）而设计。

它通过在后台保持繁重的 Node.js 或 Python CLI 进程常驻，并将并发的请求流（热重载多路复用）受控地路由到它们的标准输入/输出 (Stdin/Stdout) 管道中，从而彻底解决了“冷启动”问题。

## 🚀 为什么选用 HotPlex？

通常，从后端服务（如 Go API）运行本地 CLI 代理意味着必须为**每一次**交互都生成一个全新的操作系统进程。

*   **问题所在：** 像 `claude` (Claude Code) 这样的工具是重量级的 Node.js 应用。每次执行 `npx @anthropic-ai/claude-code` 都需要花费 **3-5 秒**，仅仅是为了启动 V8 引擎、读取文件系统上下文以及进行身份验证。对于实时的 Web UI 而言，这种延迟会让智能体感觉极其缓慢且不响应。
*   **解决方案：** HotPlex 为每个用户/会话仅仅启动一次 CLI 进程，将其保存在后台（被包裹在安全的进程组 `pgid` 中），并建立持久的双向管道。当用户发送新消息时，HotPlex 会立即通过 `Stdin` 注入消息，并通过 `Stdout` 将 JSON 响应流式传回。延迟从 **5000 毫秒锐减至 200 毫秒以内**。

## 💡 愿景与应用场景

创建 HotPlex 的原始驱动力是为了**赋能 AI 应用程序毫不费力地集成强大的 CLI 代理**（例如 Claude Code），作为其外部的“肌肉”。您的 AI 应用无需从零开始重复造轮子去构建编码、执行和文件操作能力，而是可以直接借用这些成熟 CLI 工具的强大功能。

关键应用场景包括：

- **Web 版 AI 智能体**: 构建全功能 Web 版的 Claude Code。用户通过流畅的浏览器 UI 进行交互，而 HotPlex 在安全沙盒化的后端环境中可靠地管理着持久的 Claude CLI 进程。
- **DevOps 工具链**: 将 AI 直接集成到您的 DevOps 工作流中。让智能体通过持久的 HotPlex 会话自动执行 Shell 脚本、读取 Kubernetes 日志并排查基础设施故障。
- **CI/CD 流水线**: 将智能代码审查、自动化测试和动态漏洞修复直接无缝嵌入您的 Jenkins、GitLab 或 GitHub Actions 流水线中，彻底免去每次重复启动笨重 Node.js 工具带来的延迟开销。
- **智能运维 (AIOps)**: 打造智能运维机器人 (ops-bots)，持续监控系统、分析事件报告，并通过受控的、多路复用的终端会话安全地自主执行恢复命令。

## 🛠 特性概览

- **极速热启动：** 首次启动后，后续请求瞬时响应。
- **会话池与垃圾回收 (GC)：** 自动追踪空闲进程，并在可配置的超时时间（默认 30 分钟）后终止它们以释放内存。
- **WebSocket 网关：** 包含一个开箱即用的独立服务器（`hotplexd`），将多路复用器原生暴露为 WebSocket 接口，供 React/Vue 前端或远程 Python/Node 脚本消费调用。
- **原生 Go SDK：** 导入 `github.com/hrygo/hotplex/pkg/hotplex` 即可将引擎原生且直接地嵌入到您的 Go 后端中。
- **正则安全防火墙：** 内置 `danger.go` 预检拦截器，在破坏性命令（例如 `rm -rf /`、Fork 炸弹、反向 Shell 等）抵达代理之前将其拦截。
- **上下文隔离：** 利用 UUID v5 确定性命名空间生成机制，保证沙盒化的会话隔离（例如隔离不同用户的工作空间）。

## 📦 架构设计

HotPlex 采用两层架构设计：

1.  **核心 SDK (`pkg/hotplex`)**: 引擎本体。包含 `Engine` 单例、`SessionPool`（会话池）以及 `Detector`（安全防火墙）。它负责接收来自 CLI 的 JSON 流，并对外发出强类型的 Go 事件。
2.  **独立服务器 (`cmd/hotplexd`)**: 基于核心 SDK 包装的轻量级程序，通过标准的 WebSockets 将能力暴露出去。

*注意：当前的 MVP 版本对 **Claude Code** (`--output-format stream-json`) 协议进行了深度优化支持，但在设计上保留了 `Provider` 接口抽象的演进路径，未来可无缝支持 OpenCode 和 Aider 等工具。*

## ⚡ 快速开始

### 1. 运行独立的 WebSocket 服务器

如果您只想运行服务端的守护进程，并从前端或 Python 脚本连接它：

```bash
# 确保全局已安装 Claude Code
npm install -g @anthropic-ai/claude-code

# 编译并运行服务端
cd cmd/hotplexd
go build -o hotplexd main.go
./hotplexd
```
服务器运行在 `ws://localhost:8080/ws/v1/agent`。可参考 `_examples/websocket_client/client.js` 查看集成示例。

### 2. 使用 Go SDK 原生集成

请查看 `_examples/basic_sdk/main.go` 获取完整示例。

```go
import "github.com/hrygo/hotplex/pkg/hotplex"

opts := hotplex.EngineOptions{
    Timeout: 5 * time.Minute,
    Logger:  logger,
    // InputCostPerMillion: 3.0, // 配置 Token 计费策略
}

engine, _ := hotplex.NewEngine(opts)
defer engine.Close()

cfg := &hotplex.Config{
    Mode:      "MVP",
    WorkDir:   "/tmp",
    SessionID: "user_123_session", // 确定性的热复用 ID
}

ctx := context.Background()

// 1. 发送提示词 (Prompt) 并处理流式回调
err := engine.Execute(ctx, cfg, "默默计算 20 乘 5", func(eventType string, data any) error {
    if eventType == "assistant" {
         fmt.Println("Agent generated text...")
    }
    return nil
})
```

## 🔒 核心安全姿态

HotPlex 将在您的机器上物理执行由大模型 (LLM) 生成的 Shell 命令。**请务必谨慎使用。**

我们通过以下几道防线降低风险：
1. **上下文目录限制 (WorkDirs)：** Agent 被沙盒化并限制在配置文件传入的 `WorkDir` 指定路径内操作。
2. **危险拦截器 (Danger Detector)：** 一种基于正则的“WAF”（Web 应用防火墙），它会在破坏性模式（例如 `mkfs`, `dd`, `rm -rf /*`, `chmod 000 /`）触及操作系统底线之前实施前置拦截。
3. **进程组连根拔起 (PGID)：** 当会话被外部调用终止时，HotPlex 会利用 `-pgid` 向整个底层系统进程组发送 `SIGKILL` 信号。这将保证不论是 CLI 主体、分离出去的 bash 脚本、还是衍生的子孙孙进程，都会被一瞬间彻底铲除，杜绝僵尸进程。

## 路线图 (Roadmap)
- [ ] 提取 Provider 接口 (增加对 `OpenCode` 的支持)
- [ ] 远程 Docker 沙盒执行能力 (取代本地操作系统执行)
- [ ] 提供 REST API 接口，用于会话自省管理和强制终止
