# Claude Code Agents Team 最佳实践 (v2.1.32)

在 2026 年的 Claude Code (v2.1.32+) 中，**多代理协作 (Multi-Agent Orchestration)** 已成为处理复杂、跨模块工程任务的标准配置。本文将介绍两种核心协作模式及其开启方式。


## 1. Subagents vs. Agent Teams

| 特性 | Subagents (子代理) | Agent Teams (代理团队) |
| :--- | :--- | :--- |
| **设计模式** | "火完即忘" (Fire-and-forget) | "协同作战" (Collaborative) |
| **上下文管理** | 每个子代理拥有相互隔离的上下文 | 共享全局任务列表，团队成员可相互通信 |
| **通信流** | 仅垂直通信 (父 -> 子 -> 父) | 支持水平通信 (Teammate <-> Teammate) |
| **适用场景** | 独立调研、代码路径探索、文档查阅 | 跨层开发 (BFF+Web)、复杂 Debug、大重构 |
| **资源效率** | 极高 (由于上下文隔离，减少了 Token 消耗) | 较低 (由于协调开销，Token 消耗显著增加) |

## 2. 如何开启与使用

### 2.1 启用 Agent Team
Agent Team 为实验性功能 (Experimental)，开启需要满足以下条件：

1. **版本要求**：Claude Code v2.1.32 或更高版本。
2. **环境变量**：需要设置 `export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`。
3. **配置文件**：也可以在 `~/.claude/settings.json` 中配置：
   ```json
   {
     "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1",
     "teammateMode": "auto"
   }
   ```

- **快速召唤团队**：
  在会话中输入：`claude "召唤一个代理团队来设计并实现身份验证层。"`
- **指定角色**：
  `claude agents create --team --roles "lead,backend,qa"`

> [!TIP]
> **视觉增强**：建议在 `tmux` 或 `iTerm2` 环境下使用。设置 `teammateMode: "tmux"` 后，每个 Teammate 会在独立的窗格 (Pane) 中运行，方便实时观察协作过程。

### 2.2 启动 Subagent
通常用于临时性的、不需要持久上下文的任务。
- **命令行调用**：
  `claude agents create --subagent "调研 API 网关最近的变更。"`


## 3. 2026 编排最佳实践

### 3.1 意图显式化 (Explicit Intent)
在召唤 Agent Team 时，务必明确 **Team Lead** 的职责：
> *“启动一个代理团队，由 Team Lead 负责任务拆解和最后的质量评估。”*

### 3.2 下游任务隔离与权限
- **Subagents**：利用 Subagents 进行重复性的扫描或探索任务，避免主进程的上下文窗口被冗余信息填满。
- **权限继承**：Teammates 默认继承 Team Lead 的权限设置。如果 Lead 开启了 `--dangerously-skip-permissions`，团队成员也将自动获得该授权。

---
*注：本指南内容基于 2026 年 3 月发布的 v2.1.32 版本，部分编排指令可能因安装的 Marketplace 插件而异。*

