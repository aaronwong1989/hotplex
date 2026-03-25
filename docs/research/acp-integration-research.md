# ACP 协议集成 HotPlex Provider 技术调研

**调研日期**: 2026-03-25
**调研目标**: 分析如何将 ACP (Agent Communication Protocol) 接入 HotPlex Provider 层，实现多 Agent 协作能力

---

## 1. ACP 协议概述

### 1.1 什么是 ACP？

**Agent Communication Protocol (ACP)** 是由 IBM 开发的开放协议，用于解决 AI Agent 间互操作性问题。

**核心特性**：
- **RESTful API**: 标准 HTTP 端点，无需特殊协议
- **多模态支持**: 文本、图像、音频、视频等（基于 MimeType）
- **同步/异步通信**: 支持长时间运行任务
- **流式交互**: 支持实时数据流
- **无 SDK 依赖**: 可用 curl/Postman 直接调用（但也提供 Python/TypeScript SDK）
- **Offline Discovery**: 支持未激活 Agent 的发现

### 1.2 ACP vs MCP 对比

| 维度 | MCP (Anthropic) | ACP (IBM) |
|------|----------------|-----------|
| **架构** | Client-Server | Peer-to-Peer |
| **主要用途** | Agent ↔ Tools 连接 | Agent ↔ Agent 通信 |
| **通信方式** | 单个 Agent 访问多个工具 | 多个 Agent 相互协作 |
| **任务委托** | 有限 | 强大的任务委托机制 |
| **发起方** | 仅客户端发起 | 任意 Agent 可发起 |

**关键洞察**：
- MCP 专注于"工具访问"（Agent 使用外部 API/DB）
- ACP 专注于"Agent 协作"（多个 Agent 共同完成任务）
- 两者可以共存：MCP 提供工具，ACP 编排 Agent

---

## 2. HotPlex Provider 架构分析

### 2.1 当前 Provider 接口

```go
type Provider interface {
    Metadata() ProviderMeta
    BuildCLIArgs(providerSessionID string, opts *ProviderSessionOptions) []string
    BuildInputMessage(prompt string, taskInstructions string) (map[string]any, error)
    ParseEvent(line string) ([]*ProviderEvent, error)
    DetectTurnEnd(event *ProviderEvent) bool
    ValidateBinary() (string, error)
    CleanupSession(providerSessionID string, workDir string) error
    VerifySession(providerSessionID string, workDir string) bool
    Name() string
}
```

**设计特点**：
- 基于 CLI 进程管理（stdin/stdout 通信）
- 事件驱动解析（`ParseEvent`）
- 会话隔离（`ProviderSessionID`）

### 2.2 集成挑战

**问题 1: 协议差异**
- 当前 Provider: CLI stdin/stdout (本地进程)
- ACP: HTTP REST API (网络服务)

**问题 2: 生命周期管理**
- CLI Provider: HotPlex 启动进程（PGID 隔离）
- ACP Provider: 外部服务，需 HTTP 客户端

**问题 3: 事件流转换**
- CLI Provider: 实时 stdout 流（JSON lines）
- ACP Provider: HTTP 响应流（SSE 或 chunked）

---

## 3. 集成方案头脑风暴

### 方案 A: ACP 作为 Provider（推荐）

**概念**: 将 ACP Agent 包装为 HotPlex Provider

**架构**：
```
ChatApp (Slack/TG)
      ↓
HotPlex Engine
      ↓
ACP Provider (implements Provider interface)
      ↓
HTTP Client → ACP Server (External Agent)
```

**实现要点**：
1. 新增 `acp_provider.go` 实现 `Provider` 接口
2. `BuildInputMessage`: 构造 ACP 请求 JSON
3. `ParseEvent`: 解析 ACP 响应流（SSE/chunked）
4. `DetectTurnEnd`: 识别 ACP 任务完成信号
5. `ValidateBinary`: 验证 ACP endpoint 可达性

**优势**：
- 复用现有 Engine 和 ChatApps 层
- 统一的会话管理（`ProviderSessionID`）
- 支持混合部署（本地 CLI + 远程 ACP Agent）

**代码示例**：
```go
type ACPProvider struct {
    ProviderBase
    endpoint string // ACP server URL
    client   *http.Client
}

func (p *ACPProvider) BuildInputMessage(prompt, taskInstructions string) (map[string]any, error) {
    return map[string]any{
        "messages": []map[string]any{
            {
                "role": "user",
                "parts": []map[string]any{
                    {"content": prompt, "type": "text/plain"},
                },
            },
        },
        "context": taskInstructions,
    }, nil
}

func (p *ACPProvider) ParseEvent(line string) ([]*ProviderEvent, error) {
    // Parse SSE event from ACP server
    var acpEvent ACPEvent
    if err := json.Unmarshal([]byte(line), &acpEvent); err != nil {
        return nil, err
    }
    return []*ProviderEvent{{
        Type:    acpEvent.Type,
        Content: acpEvent.Content,
    }}, nil
}
```

---

### 方案 B: ACP 作为 Relay 通道

**概念**: 利用 ACP 协议实现 HotPlex Agent 间通信

**架构**：
```
HotPlex Agent A (Slack Bot)
      ↓
ACP Client → ACP Server (Broker)
      ↓
HotPlex Agent B (Telegram Bot)
```

**实现要点**：
1. 在 `internal/relay/` 新增 `acp_relay.go`
2. 实现 `RelayTransport` 接口（基于 ACP 协议）
3. 支持 Agent 发现（ACP Discovery）
4. 路由与负载均衡

**优势**：
- 跨平台 Agent 协作（Slack ↔ Telegram）
- 支持第三方 ACP Agent 加入
- 标准化通信协议

**挑战**：
- 需要独立的 ACP Broker 服务
- 配置复杂度提升

---

### 方案 C: ACP Server 模式（托管 Agent）

**概念**: HotPlex 自身作为 ACP Server，托管多个内部 Agent

**架构**：
```
External ACP Client
      ↓
ACP Server (HotPlex)
      ↓
Engine → CLI Provider (Claude/OpenCode)
```

**实现要点**：
1. 在 `internal/server/` 新增 `acp_server.go`
2. 实现 ACP REST API 端点
3. 将 ACP 请求转发给 Engine
4. 暴露 Agent 元数据（Discovery）

**优势**：
- HotPlex 成为标准 ACP 服务
- 可被其他 ACP 客户端发现和调用
- 适合企业内部部署

**挑战**：
- 需要完整实现 ACP spec
- 安全认证（API key/JWT）

---

## 4. 技术选型建议

### 4.1 优先级排序

1. **方案 A (ACP Provider)** - **推荐首先实现**
   - 理由: 复用现有架构，最小化改动
   - 价值: 立即获得远程 Agent 能力
   - 风险: 低

2. **方案 B (Relay)** - **中期规划**
   - 理由: 需要明确的跨平台协作场景
   - 价值: 解锁 Agent-to-Agent 工作流
   - 风险: 中（依赖外部 Broker）

3. **方案 C (ACP Server)** - **长期规划**
   - 理由: 需要完整的 ACP spec 实现
   - 价值: HotPlex 成为平台级服务
   - 风险: 高（安全、性能、合规）

### 4.2 实现路径

**Phase 1: ACP Provider (2-3 周)**
- [ ] 研究 ACP SDK (Python/TypeScript)
- [ ] 实现 `acp_provider.go`
- [ ] 支持 SSE 流式响应解析
- [ ] 单元测试 + 集成测试

**Phase 2: ACP Relay (4-6 周)**
- [ ] 评估 ACP Broker 方案（BeeAI? 自建?）
- [ ] 实现 `acp_relay.go`
- [ ] Agent 发现机制
- [ ] 跨平台测试（Slack ↔ Telegram）

**Phase 3: ACP Server (8-12 周)**
- [ ] 实现完整 ACP spec
- [ ] 安全认证（OAuth2/JWT）
- [ ] Agent 元数据管理
- [ ] 性能优化与监控

---

## 5. 风险与缓解

### 5.1 协议演进风险
- **风险**: ACP 协议仍在快速演进（2026 年可能合并到 A2A）
- **缓解**: 抽象协议层（`acp_protocol.go`），支持版本切换

### 5.2 网络依赖风险
- **风险**: 远程 ACP Agent 可能不可达
- **缓解**: 实现超时、重试、降级机制

### 5.3 安全风险
- **风险**: 暴露 HTTP endpoint 增加攻击面
- **缓解**: 使用 `internal/security/` WAF，API key 验证

---

## 6. 重要：两个 ACP 协议的区别

### 命名混淆澄清

| 协议全称 | 缩写 | 官方站点 | 用途 |
|---------|------|---------|------|
| **Agent Communication Protocol** | ACP | agentcommunicationprotocol.dev | Agent ↔ Agent 通信 |
| **Agent Client Protocol** | ACP | agentclientprotocol.com | Agent ↔ IDE/编辑器 通信 |

**本文档聚焦**: **Agent Communication Protocol** (Agent 间协作)

### Agent Client Protocol 概览

**定义**: Agent 与 IDE/编辑器间的标准化协议（JSON-RPC 2.0）

**核心特点**:
- 基于 JSON-RPC 2.0
- Agent 作为 Client 的子进程运行
- Methods: `initialize`, `session/new`, `session/prompt`
- Notifications: `session/cancel`, `session/update`

**与 HotPlex 的关系**:
- **场景**: 如果要让 HotPlex 接入 IDE（如 Cursor/VSCode）
- **集成点**: HotPlex Agent 暴露 Agent Client Protocol 接口
- **优先级**: 低（HotPlex 主要面向 ChatApp，非 IDE）

### 协议对比总结

| 协议 | 用途 | 通信方式 | 适用场景 |
|------|------|---------|---------|
| **MCP** | Agent ↔ Tools | Client-Server | 访问外部 API/DB |
| **Agent Communication Protocol** | Agent ↔ Agent | REST (Peer-to-Peer) | 多 Agent 协作 |
| **Agent Client Protocol** | Agent ↔ IDE | JSON-RPC 2.0 | 编辑器集成 |
| **HotPlex Provider** | Engine ↔ CLI | stdin/stdout | CLI 工具包装 |

---

## 7. 参考资源

### 7.1 Agent Communication Protocol (本文档焦点)
- [ACP 官方站点](https://agentcommunicationprotocol.dev/)
- [IBM ACP 介绍](https://www.ibm.com/think/topics/agent-communication-protocol)
- [GitHub - i-am-bee/acp](https://github.com/i-am-bee/acp)

### 7.2 Agent Client Protocol (补充参考)
- [Agent Client Protocol 官方站点](https://agentclientprotocol.com/)
- [GitHub - agent-client-protocol](https://github.com/agentclientprotocol/agent-client-protocol)
- [JetBrains ACP](https://www.jetbrains.com/acp/)
- [Zed ACP](https://zed.dev/acp)

### 7.3 相关协议
- [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) - Anthropic
- [A2A (Agent-to-Agent)](https://akka.io/blog/mcp-a2a-acp-what-does-it-all-mean) - Google

### 7.4 竞品分析
- BeeAI Platform (IBM) - ACP 参考实现
- LangChain Agent 执行器
- CrewAI 多 Agent 框架

---

## 7. 下一步行动

### 7.1 立即行动
1. **阅读 ACP Spec**: 详细研究 REST API 定义
2. **验证可行性**: 用 curl 测试 ACP server（如 BeeAI）
3. **原型开发**: 实现 `acp_provider.go` POC

### 7.2 短期（1 个月）
- 完成 ACP Provider 实现
- 集成测试（Slack → ACP Agent）
- 文档更新（CLAUDE.md）

### 7.3 中期（3 个月）
- 评估 Relay 方案的业务价值
- 探索与 MCP 的协同（MCP 提供工具，ACP 编排 Agent）

---

**总结**: ACP 协议为 HotPlex 提供了标准化 Agent 通信能力。推荐首先实现 **方案 A (ACP Provider)**，复用现有架构，最小化风险。后续可根据业务需求逐步推进 Relay 和 Server 模式。
