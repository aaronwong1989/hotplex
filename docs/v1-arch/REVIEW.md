# HotPlex v1.0.0 设计方案 Review

> 审稿日期：2026-03-29  
> 审稿人：Avatar (数字分身)  
> 参考基准：OpenClaw 架构最佳实践

---

## OpenClaw 最佳实践调研

### 核心架构理念

OpenClaw 的架构设计遵循几个关键原则：

| 原则 | OpenClaw 实现 | HotPlex 是否遵循 |
|------|---------------|-----------------|
| **容器化隔离** | Docker 作为插件隔离机制（而非 Go .so） | ❌ 使用 Go plugin/.so |
| **配置外部化** | `.env` + `openclaw.json`，环境变量优先 | ⚠️ YAML + 环境变量，有但不够彻底 |
| **状态持久化** | Named Volumes 分层（热数据内存 + 冷数据卷） | ⚠️ Memory + SQLite，但缺分层设计 |
| **安全基线** | 最小特权：禁 NET_RAW/NET_ADMIN，no-new-privileges，LAN-only 绑定 | ❌ 未提及 |
| **交互式引导** | `make onboard` 交互式配置向导 | ❌ 未设计 |
| **版本镜像** | 多变体镜像（standard/go/java/office）按需切换 | ❌ 未设计 |
| **Workspace 同步** | 双向挂载，容器内外秒级同步 | ⚠️ 未明确 |
| **热配置加载** | 运行时 `openclaw config list` 查看/修改 | ❌ 未设计 |

### OpenClaw 关键设计决策

1. **Docker 作为插件边界**：而非 Go 官方 `plugin` 标准库（官方明确声明不保证 ABI 稳定性）
2. **分层编排**：`docker-compose.yml` (静态层) + `docker-compose.build.yml` (构建层) + `docker-compose.override.yml` (用户扩展)
3. **生命周期钩子**：`docker-entrypoint.sh` 在启动前完成 UID 自适应、种子注入、网络对齐
4. **零信任配置**：所有敏感配置通过环境变量注入，不碰源码

---

## 现有设计分析

### 文档完整性评分

| 文档 | 行数 | 质量评分 | 主要问题 |
|------|------|----------|----------|
| README.md | ~440 | ⭐⭐⭐⭐ | 缺少 OpenClaw 对齐的安全章节 |
| interface-design.md | ~2200 | ⭐⭐⭐⭐ | 接口偏重、部分设计过度 |
| plugin-system.md | ~1037 | ⭐⭐ | Go .so 方案有根本性问题 |
| message-flow.md | ~627 | ⭐⭐⭐⭐⭐ | 时序图清晰，异常路径完整 |
| configuration.md | ~823 | ⭐⭐⭐ | 缺少与 OpenClaw 安全实践的对齐 |
| error-handling.md | ~695 | ⭐⭐⭐⭐ | 错误码体系完整 |
| roadmap.md | ~353 | ⭐⭐⭐ | 里程碑定义清晰，但依赖关系不明确 |

**总分：约 5175 行文档，设计完整度约 75/100**

---

## 差距分析

### Gap 1: 插件隔离机制根本性问题

**严重程度：🔴 高**

OpenClaw 使用 Docker 容器作为插件隔离边界。HotPlex 选择 Go 官方 `plugin` 标准库存在以下问题：

- Go 官方明确声明 **不保证 plugin ABI 稳定性**（`plugin` 包文档原文："The compiler and linker do not guarantee that the symbol table of a plugin will be compatible across releases"）
- 不同 Go 版本编译的 .so **可能互相不兼容**
- 跨平台（linux/amd64, linux/arm64）交叉编译 .so 需要完整 GNU toolchain
- 实际上 HashiCorp 的 `go-plugin` 也是用 RPC + 进程隔离而非共享地址空间

**建议**：改用 **进程隔离模式**（类似 `go-plugin` 的 RPC 方案），或直接采用 Docker 作为隔离机制与 OpenClaw 对齐。

### Gap 2: 安全模型缺失

**严重程度：🔴 高**

HotPlex 设计中完全缺失以下 OpenClaw 已实现的安全实践：

| 安全机制 | OpenClaw | HotPlex |
|----------|----------|---------|
| 进程能力限制 (capabilities) | ✅ 禁 NET_RAW, NET_ADMIN | ❌ 未提及 |
| no-new-privileges | ✅ 已启用 | ❌ 未提及 |
| 网络绑定策略 | ✅ 仅 LAN (127.0.0.1) | ❌ 未提及 |
| Worker 资源限制 | ⚠️ 部分（memory/cpu limit） | ⚠️ 有但不完整 |
| 只读文件系统 | ⚠️ process.readonly_fs | ⚠️ 有但不彻底 |
| 插件签名验证 | ❌ 未实现 | ⚠️ 设计了但依赖 .so 本身不可靠 |
| 敏感信息注入 | ✅ .env 环境变量 | ⚠️ 有但不完整 |

### Gap 3: 配置系统缺少交互式引导

**严重程度：🟡 中**

OpenClaw 提供 `make onboard` 交互式引导完成 LLM Provider、飞书、Slack 等配置。HotPlex 只有静态 YAML 配置，缺少：

- 首次安装的交互式配置向导
- 配置语法校验和自动补全
- 运行中的热配置变更能力
- 配置导出/导入机制

### Gap 4: 插件接口重复定义

**严重程度：🟡 中**

`pkg/plugin/plugin.go` 定义了完整的 `ChannelPlugin`、`WorkerPlugin`、`BrainPlugin` 等接口，但这些接口的方法和 `pkg/channel/channel.go`、`pkg/worker/worker.go` 等有大量重复：

```go
// pkg/plugin/plugin.go 中定义的 WorkerPlugin
type WorkerPlugin interface {
    Run(ctx context.Context, task *Task) (*Result, error)
    Stream(ctx context.Context, task *Task, output chan<- *Result) error
    Abort(taskID string) error
    Status() *WorkerStatus
}

// pkg/worker/worker.go 中定义的 Worker 接口
type Worker interface {
    Run(ctx context.Context, task Task) (Result, error)
    Stream(ctx context.Context, task Task) (<-chan Result, error)
    Abort(taskID string) error
    Status() WorkerStatus
}
```

**问题**：
1. 同一个 Worker 要实现两套相似接口
2. 类型不兼容（`Task` vs `*Task`，`Result` vs `Result`）
3. 维护成本加倍

### Gap 5: Session 热冷数据分层不清晰

**严重程度：🟡 中**

设计提到 "Session 存储在 Memory（内存）+ SQLite（持久化）"，但：
- 没有明确热数据（活跃 Session）和冷数据（历史 Session）的分层策略
- 没有 LRU 淘汰机制的明确设计
- 冷数据查询路径不清晰

OpenClaw 的方案：`.openclaw-state` 作为命名卷，热数据走内存 Map + LRU Cache，冷数据落卷。

### Gap 6: Brain 编排链缺失

**严重程度：🟡 中**

设计了 `LLMBrain`、`RuleBrain`、`KeywordBrain` 三个独立 Brain，但没有设计 **Brain 编排链**（Pipeline）。实际场景中可能需要：

```
Message → KeywordBrain(快速路由) → LLMBrain(深度理解) → RuleBrain(合规检查)
```

OpenClaw 的 middleware chain 模式更灵活。

### Gap 7: 观测性设计滞后

**严重程度：🟡 中**

Roadmap 中 Prometheus + OpenTelemetry 被放在 Phase 3（最后阶段），但这些应该是 **贯穿全程的基础设施**：

- Phase 1 就需要 metrics 验证架构决策
- Phase 2 需要链路追踪调试跨模块问题
- Phase 3 才加观测性会导致大量返工

---

## 优化建议

### README.md

**建议补充：**

1. **安全章节**：补充 Security Model，包含：
   - 进程能力限制
   - 网络策略（LAN-only）
   - Worker 资源限制（memory/cpu/disk）
   - 只读文件系统配置
   - 敏感信息管理（环境变量注入）

2. **OpenClaw 对齐**：在参考实现章节补充：
   ```
   ## 与 OpenClaw 的差异
   - 插件隔离：Go 进程隔离 vs Docker 容器隔离
   - 配置格式：YAML vs JSON + .env
   - 状态持久化：内存+SQLite vs Named Volumes
   ```

3. **版本策略**：增加版本镜像设计说明（类似 OpenClaw 的 standard/go/java/office 变体）

---

### interface-design.md

**需要调整的接口：**

#### 1. 消除重复接口，合并 plugin 层

删除 `pkg/plugin/` 下的重复接口定义，改为：

```go
// pkg/plugin/registry.go
// 插件注册表 - 只负责发现和加载，实际接口复用 pkg/*/xxx.go 中的接口

type Registry struct {
    channelPlugins map[string]func(cfg map[string]interface{}) (channel.Channel, error)
    workerPlugins  map[string]func(cfg map[string]interface{}) (worker.Worker, error)
    // ...
}
```

这样 `DingtalkChannel` 实现 `channel.Channel` 接口即可注册，无需两套接口。

#### 2. Brain 增加 Pipeline 编排器

```go
// pkg/brain/pipeline.go
type Pipeline struct {
    stages []BrainStage
}

type BrainStage struct {
    Name  string
    Brain NativeBrain
}

func (p *Pipeline) Process(ctx context.Context, input BrainInput) (BrainOutput, error) {
    current := input
    for _, stage := range p.stages {
        output, err := stage.Brain.Process(ctx, current)
        if err != nil {
            return BrainOutput{}, err
        }
        if output.Blocked {
            return output, nil // 短路
        }
        current = BrainInput{
            Message:   output.Enhanced,
            SessionCtx: input.SessionCtx,
        }
    }
    return current, nil
}
```

#### 3. Session 分层存储策略

```go
// pkg/session/hierarchy.go
type HierarchyStore struct {
    hot    *MemoryStore    // LRU Cache，活跃 Session
    warm   *SQLiteStore    // 近期 Session（24h 内）
    cold   *SQLiteStore    // 历史 Session
}

func (s *HierarchyStore) Get(ctx context.Context, id string) (*Session, error) {
    // 1. 先查 hot
    if session, err := s.hot.Get(id); err == nil {
        return session, nil
    }
    // 2. 查 warm
    if session, err := s.warm.Get(id); err == nil {
        s.hot.Put(id, session) // 提升到 hot
        return session, nil
    }
    // 3. 查 cold
    return s.cold.Get(id)
}
```

#### 4. Worker Supervisor 职责分离

当前设计把 Process Pool 和 Supervisor 混在一起。建议分离：

```go
// pkg/supervisor/supervisor.go - 只负责 Worker 生命周期
type Supervisor struct {
    workers map[string]Worker
    policy  RestartPolicy
}

// pkg/worker/pool.go - 只负责进程池管理
type ProcessPool interface {
    Acquire(ctx context.Context) (*Process, error)
    Release(*Process) error
    Stats() PoolStats
}
```

#### 5. 增加 PluginLoader 而非依赖 Go .so

```go
// pkg/plugin/loader.go
// 不使用 Go plugin 标准库，改为进程 RPC 或 Docker

type PluginLoader interface {
    Load(ctx context.Context, kind Kind, name string, cfg map[string]interface{}) (Plugin, error)
}

// RPCLoader - 通过 Unix Domain Socket RPC 加载插件进程
type RPCLoader struct {
    socketPath string
}

// DockerLoader - 通过 Docker 容器加载插件（与 OpenClaw 对齐）
type DockerLoader struct {
    image string
}
```

---

### plugin-system.md

**核心问题：Go .so 方案不可靠，建议重新设计**

#### 推荐方案 A：RPC 进程隔离（推荐用于 Go 插件）

```
┌─────────────────┐     Unix Socket RPC      ┌─────────────────┐
│   HotPlex       │◄──────────────────────►│  Plugin Process │
│   (Main)        │    hotplex-channel-     │  (Dingtalk)     │
│                 │    dingtalk -runner     │                 │
└─────────────────┘                        └─────────────────┘
```

- 每个插件是独立进程，通过 gRPC/RPC 通信
- 独立编译，不受 Go 版本 ABI 影响
- 参考实现：`hashicorp/go-plugin`

#### 推荐方案 B：Docker 容器隔离（与 OpenClaw 对齐）

```
┌─────────────────┐     Docker API           ┌─────────────────┐
│   HotPlex       │◄──────────────────────►│  Docker Plugin  │
│   (Main)        │                        │  Container      │
│                 │                        │  (dingtalk)     │
└─────────────────┘                        └─────────────────┘
```

- 每个插件是独立 Docker 容器
- 完全隔离，任意语言实现
- 参考实现：OpenClaw DevKit

#### 需要删除的内容

- ❌ `//go:build plugin` 构建标签
- ❌ `.so` 文件动态加载相关代码
- ❌ `plugin.Open()` / `plugin.Lookup()` 相关实现

#### 需要补充的内容

- ✅ RPC 通信协议定义（Proto3）
- ✅ 生命周期管理（启动/停止/心跳）
- ✅ 错误传递和日志收集
- ✅ 资源限制（CPU/内存/时间）

---

### configuration.md

**建议补充：**

1. **安全配置章节**

```yaml
security:
  # 进程能力限制
  capabilities:
    drop:
      - NET_RAW
      - NET_ADMIN
      - SYS_ADMIN
  
  # 禁止提权
  no_new_privileges: true
  
  # 网络绑定
  network:
    bind_address: "127.0.0.1"  # 仅监听本地
    expose_ports: []            # 不暴露额外端口
  
  # 文件系统
  filesystem:
    readonly_root: true
    allowed_writes:
      - "/tmp/hotplex"
      - "/var/hotplex/uploads"
```

2. **交互式配置引导设计**

```yaml
# onboarding 配置
onboarding:
  enabled: true
  steps:
    - name: "provider"
      prompt: "请选择 LLM Provider"
      options: ["anthropic", "openai", "siliconflow"]
    - name: "channel"
      prompt: "请选择要启用的 Channel"
      options: ["feishu", "slack", "ws"]
```

3. **配置热加载设计**

```yaml
# 配置变更监听
config:
  hot_reload: true
  watch_paths:
    - "/etc/hotplex/hotplex.yaml"
    - "/etc/hotplex/plugins.d"
```

---

### error-handling.md

**基本完善，建议补充：**

1. **错误恢复模式**

```go
// 增加 ErrorRecovery 模式
type ErrorRecovery struct {
    // 自动重试次数
    MaxRetries int
    
    // 重试间隔
    Backoff time.Duration
    
    // 降级目标
    Fallback string
}
```

2. **错误聚合告警**

```go
// 当前只定义了告警规则，建议补充错误聚合逻辑
type ErrorAggregator struct {
    window   time.Duration
    maxCount int
    errors   map[Code][]*Error
}
```

---

### roadmap.md

**关键调整建议：**

#### 1. 观测性应前置到 Phase 1

| 当前 | 建议 |
|------|------|
| Phase 3: Prometheus + OpenTelemetry | Phase 1: 基础 metrics + 日志 |
| Phase 3: 健康检查 | Phase 1: 基础健康检查 |
| Phase 3: 告警 | Phase 2: 告警 |

#### 2. 明确 Phase 1 的验收标准

当前 Phase 1 只有任务列表，缺少明确的 **Done Criteria**：

```markdown
## Phase 1 验收标准

### 功能验收
- [ ] 飞书消息能完整经过：Channel → Brain → Worker → Storage → Response
- [ ] Claude Code Worker 能成功执行 `生成一个 Hello World Go 程序`
- [ ] Session 在预期 TTL 后自动过期

### 性能验收
- [ ] P99 延迟 < 500ms（不含 Worker 执行）
- [ ] 支持 10 并发请求

### 安全验收
- [ ] Worker 进程在受限目录内执行
- [ ] 内存限制生效（超限被 kill）
```

#### 3. 补充依赖关系图

```
Phase 1
├── [A] Channel 接口定义
│   └── [B] Feishu 实现
│       └── [C] 端到端测试
├── [D] Worker 接口定义
│   └── [E] ClaudeCode 实现
│       └── [C] 端到端测试
└── [F] Brain 接口定义
    └── [G] LLM Brain 实现
        └── [C] 端到端测试

Phase 2 依赖 Phase 1 全部完成
Phase 3 依赖 Phase 2 全部完成
```

#### 4. 缩短 Phase 3，聚焦核心

Phase 3 的插件系统应该简化：
- 移除 Go .so 支持
- 优先实现 RPC 进程隔离模式
- Docker 插件作为可选高级特性

---

## 总体评价

### 设计质量：⭐⭐⭐⭐ (75/100)

**优点：**
1. 文档结构完整，接口定义详细
2. 错误处理体系健全（50+ 错误码）
3. 消息流时序图清晰，Mermaid 图表规范
4. 配置格式全面，YAML 结构合理
5. Phase 划分清晰，优先级合理

**核心问题：**
1. 🔴 **插件隔离方案使用 Go .so 存在根本性缺陷**，应改为 RPC 进程隔离或 Docker 容器隔离
2. 🔴 **安全模型严重缺失**，未对齐 OpenClaw 的安全基线实践
3. 🟡 **配置系统缺少交互式引导**，用户上手体验不如 OpenClaw
4. 🟡 **接口有重复定义**，`pkg/plugin/` 与各模块接口重复
5. 🟡 **观测性后置到 Phase 3**，应该贯穿全程

**对齐 OpenClaw 的优先行动项：**

| 优先级 | 行动 | 影响 |
|--------|------|------|
| P0 | 重设计插件隔离机制（去掉 Go .so） | 架构根基 |
| P0 | 补充安全模型章节 | 生产可用性 |
| P1 | 消除 plugin 接口重复 | 代码质量 |
| P1 | 补充 Session 热冷分层设计 | 性能 |
| P1 | 前置观测性到 Phase 1 | 开发效率 |
| P2 | 增加交互式配置引导 | 用户体验 |

### 下一步建议

1. **优先解决 P0 问题**：与黄飞虹确认插件隔离方案选择（RPC vs Docker）
2. **补充安全章节**：基于 OpenClaw 安全模型补充到 README.md 和 configuration.md
3. **接口去重**：统一 plugin 接口和模块接口，避免重复维护
4. **细化验收标准**：每个 Phase 增加明确的 Done Criteria
5. **重构 plugin-system.md**：移除 Go .so 方案，补充 RPC/Docker 方案

---

*Review 版本：v1.0 | 审稿人：Avatar | 日期：2026-03-29*
