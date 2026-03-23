# 教学篇 07：无头模式与热多路复用 (Headless Mode & Hot Multiplexing)

在 AI 编程工具的进阶使用中，**无头模式 (Headless Mode)** 是实现自动化、集成化和高性能交互的核心。本篇将详细介绍无头模式的概念、价值，以及 HotPlex 如何利用这一特性实现毫秒级响应的 **热多路复用 (Hot Multiplexing)** 技术。

---

## 1. 什么是无头模式？

### 1.1 定义
无头模式是指在没有图形用户界面 (GUI) 或标准终端交互界面 (TTY) 的情况下运行程序。对于 Claude Code CLI 而言，这意味着它不再通过彩色的 TTY 输出、动画转轮或实时交互输入来与“人类”对话，而是通过**结构化的数据流**与“程序”对话。

### 1.2 核心价值
- **程序化调用 (Programmatic Access)**：允许将 Claude 的能力嵌入到 IDE 插件、Slack 机器人、CI/CD 流水线或自定义 Web 应用中。
- **自动化流 (Automation)**：无需人工干预即可完成复杂的代码审查、重构或测试任务。
- **结构化通信**：通过 JSON 等格式进行通信，避免了复杂的字符串解析和正则表达式匹配，极大提升了通信的可靠性。

### 1.3 通用使用方式
Claude Code CLI 提供了专门的参数来开启无头模式：

```bash
# 以 JSON 流模式启动
claude --output-format stream-json --input-format stream-json
```

在该模式下：
- **输出**：每一行都是一个独立的 JSON 对象，包含了 `thinking` (思考过程)、`tool_use` (工具调用)、`answer` (最终回答) 等事件。
- **输入**：通过标准输入推送 JSON 消息，例如 `{"type": "user", "message": "Fix this bug"}`。

### 1.4 非交互式快捷调用：使用 `-p` 标志
除了完全的 JSON 流模式，最常见的“无头”或“非交互式”用法是使用 `-p` (Prompt) 标志。这适用于脚本自动化或简单的命令行触发。

**实际用例 1：代码分析与解释**
如果你只想快速得到一个问题的答案，而不需要进入交互式会话：
```bash
claude -p "分析 main.go 的并发逻辑并指出潜在风险"
```

**实际用例 2：管道集成 (Pipeline)**
将其他命令的输出作为上下文传递给 Claude：
```bash
cat error.log | claude -p "根据以下错误日志，修复 main.go 中的相关 bug"
```

**实际用例 3：全自动执行任务**
结合 `--dangerously-skip-permissions`，Claude 可以完全自主地运行测试并修复问题：
```bash
claude -p "运行 go test ./...，如果失败则尝试修复代码直到测试通过" --dangerously-skip-permissions
```

> [!WARNING]
> 使用 `--dangerously-skip-permissions` 时请务必小心，因为它允许 Claude 在无需你确认的情况下执行任意 shell 命令。建议在受隔离的开发环境或 Docker 容器中使用。

---

## 2. 高级玩法：HotPlex 的热多路复用 (Hot Multiplexing)

在普通的无头模式调用中，每次请求都会面临 **“冷启动” (Cold Start)** 问题：Node.js 运行时启动、CLI 初始化、身份验证检查合起来通常需要 2-5 秒。这对于实时聊天或高频交互来说是不可接受的。

**HotPlex** 通过独创的“热多路复用”技术彻底解决了这一问题。

### 2.1 核心原理：保持“温热”的进程池
HotPlex 不会随用随启。它在内部维护了一个**进程池 (Process Pool)**：
1. **进程常驻**：HotPlex 在后台保持 warm 状态的 Claude Code 进程。
2. **状态感知**：HotPlex 追踪每个进程的活跃状态（Starting, Ready, Busy, Dead）。
3. **即时分发**：当用户的请求到来时，HotPlex 会在毫秒内将其路由到已经 Ready 的空闲进程中。

### 2.2 关键技术点

#### A. 会话持久化与 Marker Files
HotPlex 使用 **Marker Files (标记文件)** 来实现跨重启的会话恢复。
- 每个会话通过确定性的 SHA1 算法（基于 Namespace 和 Session ID）生成唯一的 `ProviderSessionID`。
- 即使 HotPlex 服务重启，它也能通过读取 Marker 文件定位到原有的 Claude 会话文件（通常位于 `~/.claude/projects/`），并使用 `--resume <session-id>` 实现无缝续接。

#### B. 全双工 JSON 事件流映射
HotPlex 将 Claude 复杂的原始输出实时映射为标准化的事件：
- **`thinking` 事件**：提取 Claude 的推理过程，实现“思考可见化”。
- **`tool_use` 事件**：在工具执行前拦截，应用安全规则（WAF）或自动授权模式（bypass-permissions）。
- **`result` 事件**：自动提取 Token 消耗（input/output tokens）和成本数据。

#### C. 环境隔离与安全
虽然多个进程在运行，但 HotPlex 确保了严格的隔离：
- **CWD 隔离**：每个进程运行在独立的 `WorkDir` (工作目录) 中。
- **PGID 管理**：确保当会话结束时，Claude 启动的所有子进程（如 dev server）都会被完整地清理，防止僵尸进程占用资源。

### 2.3 性能对比

| 指标 | 普通 CLI (冷启动) | HotPlex (热多路复用) | 提升 |
| :--- | :--- | :--- | :--- |
| **首字响应 (TTFT)** | 3s - 8s | **50ms - 200ms** | **~20x** |
| **进程开销** | 每次重建 | 进程复用 | 显著降低 IO |
| **会话恢复** | 手动操作 | 自动持久化 | 无缝体验 |

---

## 3. 总结

无头模式让 Claude 从一个“工具”变成了“引擎”，而 HotPlex 的热多路复用技术则是这个引擎的“增压加速器”。通过预热进程、状态管理和结构化流转，HotPlex 实现了像调用 API 一样快速、又像使用 CLI 一样全能的极致体验。

---

## 4. 相关资源

- **HotPlex 官方文档**：[https://hrygo.github.io/hotplex/](https://hrygo.github.io/hotplex/)
- **Claude Code 官方介绍**：[https://www.anthropic.com/claude/code](https://www.anthropic.com/claude/code)
- **项目源码**：
  - [internal/engine/pool.go](https://github.com/hrygo/hotplex/blob/main/internal/engine/pool.go)
  - [provider/claude_provider.go](https://github.com/hrygo/hotplex/blob/main/provider/claude_provider.go)
