# OpenClaw ACP 实践深度调研

**调研日期**: 2026-03-26
**目标**: 深入分析 OpenClaw 的 Agent Client Protocol (ACP) 实现，提炼可借鉴的实践经验

---

## 1. OpenClaw ACP 架构概览

### 1.1 核心设计理念

**定位**: **Gateway-backed ACP Bridge** (基于网关的 ACP 桥接器)

**关键洞察**:
- **不是完整 ACP 运行时**，而是协议转换层
- **复用现有基础设施**（Gateway + Session Store）
- **最小化实现范围**，聚焦核心场景

### 1.2 架构分层

```
┌─────────────────────────────────────────┐
│   IDE / ACP Client (Zed, VSCode, etc.)  │
└─────────────────┬───────────────────────┘
                  │ stdio (ACP Protocol)
                  ▼
┌─────────────────────────────────────────┐
│         openclaw acp (Bridge)            │
│  • ACP ↔ Gateway Protocol 翻译          │
│  • Session 映射 (ACP ID → Gateway Key)   │
│  • 事件流转换                           │
└─────────────────┬───────────────────────┘
                  │ WebSocket
                  ▼
┌─────────────────────────────────────────┐
│        OpenClaw Gateway (Control Plane)  │
│  • Session 生命周期管理                  │
│  • Agent 路由                           │
│  • MCP Server 集成                      │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│    Backend Agent (Claude, GPT, etc.)    │
└─────────────────────────────────────────┘
```

**核心职责划分**:
- **Bridge**: 协议翻译 + Session 映射
- **Gateway**: 会话持久化 + Agent 调度
- **Agent**: 实际 AI 能力

---

## 2. 核心实现细节

### 2.1 ACP 协议支持矩阵

| ACP 功能 | 实现状态 | 关键说明 |
|---------|---------|---------|
| **核心流程** | | |
| `initialize` | ✅ 完整 | 版本协商 |
| `newSession` | ✅ 完整 | 创建 ACP Session → 映射到 Gateway Key |
| `loadSession` | ⚠️ 部分 | 重放文本历史，但不重建工具调用 |
| `prompt` | ✅ 完整 | ACP Prompt → Gateway `chat.send` |
| `cancel` | ✅ 完整 | ACP Cancel → Gateway `chat.abort` |
| `listSessions` | ✅ 完整 | 列出 Gateway Sessions |
| **内容支持** | | |
| 文本输入 | ✅ 完整 | 直接传递 |
| 资源链接 | ✅ 完整 | 扁平化为文本 |
| 图片附件 | ✅ 完整 | 转换为 Gateway Attachments |
| **会话模式** | ⚠️ 部分 | 暴露部分 Gateway 控制项（思考级别、工具详细度等） |
| **事件流** | | |
| `message` | ✅ 完整 | 文本输出 |
| `tool_call` | ⚠️ 部分 | 原始 I/O + 文件路径（无终端/结构化 diff） |
| `session_info_update` | ⚠️ 部分 | 从 Gateway 快照推断（非实时） |
| `usage_update` | ⚠️ 部分 | 近似值，仅当 Gateway Token 数据标记为"新鲜"时发送 |
| **不支持的 ACP 功能** | | |
| Per-session MCP Servers | ❌ 拒绝 | 在 Gateway 层配置 MCP |
| Client 文件系统方法 | ❌ 不调用 | Bridge 不请求 Client 读写文件 |
| Client 终端方法 | ❌ 不调用 | Bridge 不创建 Client 终端 |
| 计划/思考流 | ❌ 不支持 | 仅输出文本 + 工具状态 |

### 2.2 Session 映射机制

**核心挑战**: ACP Session ID 与 Gateway Session Key 的映射

**默认策略**: 隔离模式
```
ACP Session ID: acp:<uuid>
Gateway Session Key: acp:<uuid>
```
- 每次连接创建新会话
- 多个连接之间完全隔离

**显式绑定策略**: 复用模式
```bash
# 直接指定 Gateway Key
openclaw acp --session agent:main:main

# 通过 Label 查找
openclaw acp --session-label "support inbox"

# 重置会话（保持 Key，清空历史）
openclaw acp --session agent:main:main --reset-session
```

**Metadata 覆盖**: ACP Client 可在请求中注入配置
```json
{
  "_meta": {
    "sessionKey": "agent:main:main",
    "sessionLabel": "support inbox",
    "resetSession": true,
    "requireExisting": false
  }
}
```

**关键设计**:
- **内存映射**: Bridge 进程生命周期内保持 ACP ID → Gateway Key 映射
- **持久化**: 由 Gateway 负责（Session Store）
- **冲突处理**: 多个 Client 共享同一 Gateway Key 时，事件路由是"尽力而为"（推荐使用隔离模式）

### 2.3 Prompt 翻译流程

**ACP Prompt → Gateway Chat.Send**

```typescript
// ACP 输入格式
{
  "prompt": {
    "parts": [
      { "type": "text", "text": "Refactor this function" },
      {
        "type": "resource_link",
        "uri": "file:///path/to/code.ts",
        "mimeType": "text/typescript"
      },
      {
        "type": "resource_link",
        "uri": "data:image/png;base64,...",
        "mimeType": "image/png"
      }
    ]
  }
}

// ↓ Bridge 转换 ↓

// Gateway chat.send
{
  "text": "Refactor this function\n\n[Resource: file:///path/to/code.ts]",
  "attachments": [
    { "mimeType": "image/png", "data": "base64,..." }
  ]
}
```

**工作目录注入**:
```bash
# 默认行为：在 Prompt 前添加 CWD
--prefix-cwd (默认开启)

# 禁用
--no-prefix-cwd
```

### 2.4 事件流转换

**Gateway 事件 → ACP 事件**

| Gateway 事件 | ACP 事件 | 映射逻辑 |
|-------------|---------|---------|
| `chat.message` | `message` | 文本块直接传递 |
| `tool.start` | `tool_call` | 初始化工具调用对象 |
| `tool.output` | `tool_call_update` | 追加原始 I/O + 文件路径（尽力而为） |
| `chat.done` (complete) | `done` (stop) | 正常完成 |
| `chat.done` (aborted) | `done` (cancel) | 用户取消 |
| `chat.done` (error) | `done` (error) | 错误终止 |

**已知限制**:
- **工具历史不可重建**: `loadSession` 只重放文本，不重放工具调用
- **终端不支持**: 无法暴露 ACP Client 终端
- **Usage 非实时**: 从 Gateway 快照推断，仅当 Token 数据标记为"新鲜"时发送

---

## 3. acpx: Headless CLI 客户端

### 3.1 设计哲学

**定位**: **最小的实用 ACP 客户端**

**核心原则**:
1. **轻量级**: 无需 PTY 抓取或适配器胶水代码
2. **Agent-to-Agent**: 主要用户是其他 Agent/Orchestrator（非人类）
3. **聚焦**: 只做 ACP 客户端，不做大型编排层
4. **稳定优先**: 减少约定（conventions），保持向后兼容

### 3.2 核心命令

**会话管理**:
```bash
# 创建新会话
acpx codex sessions new
acpx codex sessions new --name docs

# 列出会话
acpx codex sessions list

# 查看当前会话状态
acpx codex sessions show

# 切换会话
acpx codex -s docs "update CLI docs"
```

**提示词执行**:
```bash
# 一次性执行（默认 codex）
acpx codex 'fix the failing test'
acpx codex prompt 'rewrite AGENTS.md'
acpx codex exec 'summarize this repo'

# 指定工作目录
acpx codex --cwd /path/to/repo 'analyze code'

# JSON 输出
acpx --format json codex exec 'review changed files'
```

**会话控制**:
```bash
# 查看状态
acpx codex status

# 取消当前任务
acpx codex cancel
```

### 3.3 配置管理

**初始化**:
```bash
acpx config init
```

**查看配置**:
```bash
acpx config show
```

**自定义 Agent 配置** (`~/.acpx/config.json`):
```json
{
  "agents": {
    "openclaw": {
      "command": "env OPENCLAW_HIDE_BANNER=1 openclaw acp --url ws://127.0.0.1:18789 --token-file ~/.openclaw/gateway.token --session agent:main:main"
    }
  }
}
```

### 3.4 文档策略（重要经验）

**示例排序优先级**（强制规则）:
1. `pi`
2. `openclaw`
3. `codex`
4. `claude`
5. `gemini`
6. `cursor`
7. `copilot`

**主文档中立性原则**:
- `README.md` 和 `docs/CLI.md` **必须保持中立**
- **禁止**在主文档中推广特定 Harness
- 只有 `pi`, `openclaw`, `codex`, `claude` 可作为命名示例
- 其他 Harness 的文档必须放在 `agents/{Agent}.md`

**文档同步规则**:
- 任何修改 Built-in Agent 的 PR **必须同时更新**:
  1. `skills/acpx/SKILL.md`
  2. `agents/{Agent}.md`
- 文档不同步的 PR **禁止合并**

**启示**: 文档治理是长期维护成本的关键

---

## 4. 与 HotPlex 的架构对比

### 4.1 相似之处

| 维度 | OpenClaw ACP | HotPlex Provider |
|------|-------------|------------------|
| **桥接模式** | ACP ↔ Gateway | ChatApp ↔ Engine ↔ CLI |
| **会话管理** | Gateway Session Store | Session Pool + Marker |
| **协议转换** | ACP → WebSocket | WebSocket → stdin/stdout |
| **Agent 隔离** | PGID 无（依赖 Gateway） | PGID 进程组 |

### 4.2 差异点

| 维度 | OpenClaw ACP | HotPlex Provider |
|------|-------------|------------------|
| **协议层次** | Agent ↔ IDE (JSON-RPC) | Agent ↔ Engine (stdin/stdout) |
| **Agent 形态** | 外部服务（Gateway 托管） | 本地 CLI 进程（Engine 启动） |
| **标准化程度** | 高（ACP 开放协议） | 低（Provider 接口内部约定） |
| **扩展性** | 标准化 Agent 接入 | Plugin 系统 + Provider 接口 |
| **MCP 集成** | Gateway 层配置 | Engine 层管理（未来） |

### 4.3 可借鉴的设计

**1. Bridge 职责边界**
- **只做协议转换**，不实现复杂业务逻辑
- **复用现有基础设施**（Gateway/Session Store）
- **明确不支持的功能**（拒绝而非静默忽略）

**2. Session 映射策略**
- **默认隔离**（`acp:<uuid>`）
- **显式绑定**（`--session` / `--session-label`）
- **Metadata 覆盖**（`_meta` 字段）

**3. 文档治理**
- **中立性原则**：主文档不推广特定实现
- **同步规则**：代码 + 文档同步修改
- **示例排序**：明确的优先级约定

**4. 配置优先级**
```bash
# 1. CLI flags (最高优先级)
--url --token

# 2. 环境变量
OPENCLAW_GATEWAY_TOKEN

# 3. 配置文件
gateway.remote.*

# 4. 默认值 (最低)
```

**5. 调试工具**
- `openclaw acp client`: 内置 ACP 客户端用于测试
- `--verbose`: 日志输出到 stderr（保持 stdout 干净）

---

## 5. 实践经验总结

### 5.1 架构决策

**✅ 正确决策**:
1. **Bridge 模式** > 完整实现
   - 理由：复用现有 Gateway，快速落地
   - 代价：部分 ACP 功能受限（如工具历史重放）

2. **拒绝而非静默忽略**
   - 示例：Per-session MCP Servers 直接返回错误
   - 理由：避免隐藏问题，明确边界

3. **内存映射** + Gateway 持久化
   - 理由：Bridge 无状态，Gateway 统一存储
   - 代价：多 Client 共享 Key 时路由不严格

**⚠️ 已知限制**:
1. **工具历史不可重放** → `loadSession` 功能受限
2. **Usage 非实时** → 依赖 Gateway 快照标记
3. **终端不支持** → 无法暴露 ACP Client 终端

### 5.2 工程实践

**1. 测试策略**
- **单元测试**: `src/acp/session.test.ts` - Run ID 生命周期
- **集成测试**: 完整 Bridge 流程（需要 Gateway）
- **CI 验证**: `pnpm check` = format + typecheck + lint + build + test

**2. 版本管理**
- 使用 `@agentclientprotocol/sdk` 0.15.x
- 严格遵循 ACP spec（否则客户端不兼容）

**3. 安全考虑**
- `--token-file` 优于 `--token`（避免进程列表泄露）
- `OPENCLAW_SHELL=acp` 环境变量用于 Shell Profile 规则

**4. 性能优化**
- Session 映射在内存中（Bridge 生命周期）
- Gateway 负责重查询/压缩（不放在 Bridge）

### 5.3 避免的陷阱

**1. 过度实现**
- ❌ 在 Bridge 中实现完整 ACP 运行时
- ✅ 聚焦协议翻译，复用 Gateway

**2. 静默降级**
- ❌ 静默忽略不支持的 `mcpServers`
- ✅ 返回明确错误

**3. 文档膨胀**
- ❌ 在主文档中推广特定 Harness
- ✅ 保持中立，Harness 文档独立

**4. 配置混乱**
- ❌ 多个配置源无优先级
- ✅ 明确优先级：Flags > Env > Config > Defaults

---

## 6. 对 HotPlex 的启示

### 6.1 短期可借鉴

**1. ACP Provider 实现**
```go
type ACPProvider struct {
    ProviderBase
    endpoint string
    client   *http.Client
}

// 实现 Provider 接口
func (p *ACPProvider) BuildInputMessage(prompt, taskInstructions string) (map[string]any, error) {
    // 构造 ACP Prompt 格式
    return map[string]any{
        "prompt": map[string]any{
            "parts": []map[string]any{
                {"type": "text", "text": prompt},
            },
        },
    }, nil
}
```

**2. Session 映射**
- ACP Session ID → HotPlex Session ID
- 默认隔离 + 显式绑定（`--session` flag）
- Metadata 覆盖（`_meta` 字段）

**3. 配置优先级**
- Flags > Env > YAML Config > Defaults

### 6.2 中期规划

**1. acpx 风格的 CLI 工具**
```bash
# 管理远程 HotPlex Agent
hotplex-cli acp --url wss://hotplex-gateway:18789 --session agent:slack:main

# Agent-to-Agent 通信
hotplex-cli codex 'ask the slack bot for recent context'
```

**2. 文档治理**
- 主文档中立性
- Provider 文档独立（`docs/providers/{Claude,OpenCode}.md`）
- 示例排序约定

**3. 调试工具**
- `hotplex acp client`: 内置测试客户端
- `--verbose`: 日志到 stderr

### 6.3 长期演进

**1. ACP Server 模式**
- HotPlex 暴露 ACP 接口
- 接入 IDE 插件（VSCode/Cursor）

**2. MCP + ACP 共存**
- MCP: Agent ↔ Tools
- ACP: Agent ↔ IDE/Agent

**3. 多协议支持**
- Agent Communication Protocol (Agent 间协作)
- Agent Client Protocol (IDE 集成)
- MCP (工具访问)

---

## 7. 关键资源

### 7.1 官方资源

- **OpenClaw ACP 文档**: [docs.openclaw.ai/cli/acp](https://docs.openclaw.ai/cli/acp)
- **OpenClaw ACP Bridge 实现**: [github.com/openclaw/openclaw/blob/main/docs.acp.md](https://github.com/openclaw/openclaw/blob/main/docs.acp.md)
- **acpx CLI**: [github.com/openclaw/acpx](https://github.com/openclaw/acpx)
- **acpx AGENTS.md**: [github.com/openclaw/acpx/blob/main/AGENTS.md](https://github.com/openclaw/acpx/blob/main/AGENTS.md)

### 7.2 教程与指南

- **2026 Complete Guide**: [dev.to/czmilo/2026-complete-guide](https://dev.to/czmilo/2026-complete-guide-openclaw-acp-bridge-your-ide-to-ai-agents-3hl8)
- **阿里云部署教程**: [developer.aliyun.com/article/1717563](https://developer.aliyun.com/article/1717563)
- **三大 Agent 解析**: [aivi.fyi/aiagents/OpenClaw-Agent-Tutorial](https://www.aivi.fyi/aiagents/OpenClaw-Agent-Tutorial)

### 7.3 架构分析

- **Lessons from OpenClaw's Architecture**: [techwithibrahim.medium.com](https://techwithibrahim.medium.com/lessons-from-openclaws-architecture-for-agent-builders-243921dcbbad)
- **Protocol Gaps Analysis**: [shashikantjagtap.net](https://shashikantjagtap.net/openclaw-acp-what-coding-agent-users-need-to-know-about-protocol-gaps/)
- **Architecture Deep Dive**: [zhuanlan.zhihu.com](https://zhuanlan.zhihu.com/p/2006150745662181825)

---

## 8. 总结

**OpenClaw ACP 的核心价值**:
1. **标准化**：基于开放协议，接入任何 ACP 兼容客户端
2. **复用性**：Bridge 模式，复用现有 Gateway 基础设施
3. **实用性**：聚焦核心场景，明确边界
4. **可维护性**：清晰的文档治理 + 配置策略

**对 HotPlex 的启示**:
- **短期**：实现 ACP Provider，复用 Session 映射设计
- **中期**：开发 acpx 风格 CLI 工具，完善文档治理
- **长期**：支持 ACP Server 模式，接入 IDE 生态

**关键教训**:
1. **Bridge > 完整实现**：复用现有基础设施
2. **拒绝 > 静默**：明确边界，避免隐藏问题
3. **文档中立**：主文档保持中立，特定实现在独立文档
4. **配置优先级**：Flags > Env > Config > Defaults
