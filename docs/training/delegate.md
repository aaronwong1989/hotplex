---
description: 委托 Agents Team 进行智能并行执行
argument-hint: <任务描述>
---

# 🚀 委托 Agents Team 并行执行

> **别名**: `/delegate`, `/parallel`

## 快速开始

```
/delegate <任务描述>
```

**示例**：
```
/delegate "创建 agent team 分析这个 CLI 工具：一个 teammate 做 UX 调研，一个做技术架构，一个做竞品分析"
```

---

## 核心决策：选择并行模式

| 模式 | 适用场景 | 特点 | Token 成本 |
|:---|:---|:---|:---|
| **Subagents** | 专注任务，只需结果回传 | 结果汇总给主 agent | 低 |
| **Agent Teams** | 复杂协作，需要讨论和协调 | Teammates 直接通信 | 高（约 15× 单会话） |

**选择标准**：
- Teammates 需要**互相讨论**？→ Agent Teams
- Teammates 需要**挑战彼此观点**？→ Agent Teams
- 只需**独立执行**并汇总结果？→ Subagents

---

## 执行协议（必须遵循）

### 1. 任务复杂度评估

| 复杂度 | Agent 数量 | 工具调用次数 | 示例 |
|:---|:---|:---|:---|
| **简单** | 1 | 3-10 | 查找文件、简单查询 |
| **中等** | 2-4 | 10-15 | 代码审查、对比分析 |
| **复杂** | 5-10 | 15+ | 全栈功能、大型重构 |

### 2. 角色定义模板

每个 teammate 需要明确的：
- **目标**：具体要完成什么
- **输出格式**：返回什么样的结果
- **工具/来源**：使用哪些工具和信息源
- **边界**：哪些不在职责范围内

```
Spawn a [角色名] teammate with the prompt:
"[目标描述]
Focus on: [具体关注点]
Output format: [期望的输出格式]
Constraints: [限制条件]
Context: [项目特定上下文]"
```

### 3. 并行执行原则

**DO**：
- 在**单个响应**中发起所有 Task 调用
- 为每个子任务提供完整上下文
- 明确文件边界，避免冲突

**DON'T**：
- 串行等待一个任务完成再启动下一个
- 让多个 teammates 修改同一文件
- 给出模糊指令（会导致重复工作）

---

## 任务类型匹配

### 调研与审查类（推荐起点）

| 任务 | Agent 配置 | 说明 |
|:---|:---|:---|
| **代码审查** | 3 teammates: 安全、性能、测试覆盖 | 不同视角并行审查 |
| **Bug 调查** | 3-5 teammates，各自调查不同假设 | 对抗性辩论找到根因 |
| **技术调研** | Explore + WebSearch + Context7 | 本地 + 远程并行 |
| **竞品分析** | 每个 competitor 一个 teammate | 全面覆盖 |

### 开发与实施类

| 任务 | Agent 配置 | 说明 |
|:---|:---|:---|
| **全栈功能** | Backend + Frontend + Test + Review | 跨层协调 |
| **大型重构** | 按模块分派，每人一个模块 | 独立文件边界 |
| **文档生成** | API 文档 + 用户指南 + 示例代码 | 并行产出 |

### 使用专业插件

| 插件 | Agent 数量 | 适用场景 |
|:---|:---|:---|
| `feature-dev` | 3 (explorer→architect→reviewer) | 功能开发流水线 |
| `pr-review-toolkit` | 6 (多维度审查) | PR 全面审查 |
| `code-reviewer` | 1 | 快速代码审查 |
| `frontend-design` | 1 | 高质量前端界面 |

---

## 工作隔离机制

| 场景 | 隔离方式 | 触发条件 |
|:---|:---|:---|
| **只读调研** | 无需隔离 | Explore Agent 默认只读 |
| **Plan 模式** | 无需隔离 | Plan Agent 只写计划文件 |
| **代码实施** | Git worktree | 需要修改代码时 |
| **多会话并行** | Git worktree | 完全隔离的会话环境 |

> 代码实施前执行 `/superpowers:using-git-worktrees` 创建隔离环境

---

## 高级模式

### 计划审批模式（复杂任务）

对于高风险或复杂任务，要求 teammates 先规划再实施：

```
Spawn an architect teammate to [任务描述].
Require plan approval before they make any changes.
```

**审批标准示例**：
- "只批准包含测试覆盖的计划"
- "拒绝修改数据库 schema 的计划"

### 委托模式（纯协调）

让 lead 只做协调，不参与实施：

1. 创建 team 后按 `Shift+Tab` 进入 delegate mode
2. Lead 只能：spawn、message、shutdown teammates、管理 tasks

### 对抗性调查（根因分析）

```
Spawn 5 agent teammates to investigate different hypotheses.
Have them talk to each other to try to disprove each other's theories,
like a scientific debate.
```

---

## 最佳实践

### 给 Teammates 足够上下文

Teammates **不继承** lead 的对话历史，需要完整的任务上下文：

```
Spawn a security reviewer teammate with the prompt:
"Review the authentication module at src/auth/ for security vulnerabilities.
Focus on token handling, session management, and input validation.
The app uses JWT tokens stored in httpOnly cookies.
Report any issues with severity ratings."
```

### 任务大小适中

- **太小**：协调开销超过收益
- **太大**：teammates 工作太久无检查点，风险浪费
- **刚好**：自包含单元，产出清晰可交付物

### 避免文件冲突

两个 teammates 修改同一文件 → 覆盖问题。解决方案：
- 按模块/目录分派任务
- 每个 teammate 拥有不同的文件集

### 监控与引导

- 定期检查 teammates 进度
- 重定向不正确的方法
- 及时综合发现
- 不要让 team 长时间无人监督

---

## 示例场景

### 1. 并行代码审查

```
Create an agent team to review PR #142. Spawn three reviewers:
- One focused on security implications
- One checking performance impact
- One validating test coverage
Have them each review and report findings.
```

### 2. 竞争假设调试

```
Users report the app exits after one message instead of staying connected.
Spawn 5 agent teammates to investigate different hypotheses.
Have them talk to each other to disprove each other's theories.
Update the findings doc with whatever consensus emerges.
```

### 3. 全栈功能开发

```
I need to build a user authentication system. Spawn separate agents:
1. Backend: Create Express.js routes for login, signup, token refresh
2. Frontend: Build React login/signup forms with validation
3. Testing: Write integration tests for all auth endpoints
4. Review: Review all code for security issues
```

### 4. 本地 + 远程混合调研

```
/delegate "调研 Next.js App Router 的最佳实践，并分析当前项目的实现差距"
→ 并行执行：
  - Context7 MCP 获取官方文档
  - WebSearch 搜索最佳实践
  - Explore Agent 分析本地代码
```

---

## 故障排查

| 问题 | 可能原因 | 解决方案 |
|:---|:---|:---|
| Teammates 未出现 | 任务不够复杂 | 明确要求创建 team |
| 权限提示过多 | Teammates 继承 lead 权限 | 预批准常见操作 |
| Teammates 出错停止 | 遇到错误未恢复 | 直接给指令或 spawn 替换 |
| Lead 提前结束 | 误判任务完成 | 告诉 lead 继续等待 |
| 孤立 tmux 会话 | 清理不完整 | `tmux kill-session -t <name>` |

---

## 执行检查清单

当收到委托任务时：

- [ ] 评估任务复杂度，确定 agent 数量（3-6 个）
- [ ] 为每个 agent 定义清晰角色和边界
- [ ] 确认文件边界，避免冲突
- [ ] 在**单个响应**中并行发起所有 Task 调用
- [ ] 等待所有子任务完成后聚合结果
- [ ] 如需代码修改，先创建 Git worktree

---

> **核心原则**：Multi-agent systems work because they help spend enough tokens to solve the problem. 让每个 teammate 专注于自己的领域，通过并行扩展 token 使用量来解决复杂问题。

**参考资料**：
- [Anthropic Agent Teams Docs](https://code.claude.com/docs/en/agent-teams)
- [Anthropic Engineering: Multi-agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system)
- [SitePoint: Claude Agent Teams Tutorial](https://www.sitepoint.com/anthropic-claude-code-agent-teams/)
