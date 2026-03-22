# 调研报告：Cron 调度器 & Bot-to-Bot Relay — 竞品分析 & HotPlex 适配设计

**日期**: 2026-03-22
**基于**: Issue #325 + cc-connect 竞品源码 + OpenClaw AgentScope 框架实践
**目标**: 为 HotPlex 设计符合其架构风格的 Cron + Relay 系统

---

## 1. 竞品概览

| 系统 | 架构语言 | Cron 实现 | Relay 实现 | 持久化 | 测试覆盖 |
|------|---------|----------|-----------|--------|---------|
| **cc-connect** | Go (2300+ stars) | CronScheduler + CronStore | RelayManager + Engine.HandleRelay | JSON 文件 | cron_test.go (13KB) |
| **OpenClaw AgentScope** | Python + FastAPI | CronManager 单例 | HTTP Client → 目标实例 | cron_jobs.json | — |
| **HotPlex** (现状) | Go | **无** | **无** | Session marker (SessionPool) | — |

---

## 2. cc-connect cron.go 深度解析

### 2.1 核心数据结构

```go
// CronJob: 丰富的任务描述
type CronJob struct {
    ID          string    `json:"id"`
    Project     string    `json:"project"`         // 引擎/项目标识
    SessionKey  string    `json:"session_key"`     // 目标会话
    CronExpr    string    `json:"cron_expr"`       // cron 表达式
    Prompt      string    `json:"prompt"`          // 自然语言 prompt
    Exec        string    `json:"exec,omitempty"`  // 可选：直接 shell 命令
    WorkDir     string    `json:"work_dir,omitempty"`
    Description string    `json:"description"`
    Enabled     bool     `json:"enabled"`
    Silent      *bool    `json:"silent,omitempty"`  // 是否静默（不通知）
    Mute        bool     `json:"mute,omitempty"`
    SessionMode string   `json:"session_mode,omitempty"` // 专用 session
    TimeoutMins *int     `json:"timeout_mins,omitempty"`  // 超时控制
    CreatedAt   time.Time `json:"created_at"`
    LastRun     time.Time `json:"last_run,omitempty"`
    LastError   string    `json:"last_error,omitempty"`
}
```

### 2.2 CronStore: 持久化层

```go
type CronStore struct {
    path string
    mu   sync.Mutex
    jobs []*CronJob
}
// 核心方法：
// - NewCronStore(dataDir string)        // 创建目录 + 加载已有任务
// - Add(job *CronJob) error             // 原子写入 jobs.json
// - Remove(id string) bool
// - List() []*CronJob                   // 返回副本
// - Get(id string) *CronJob
// - MarkRun(id string, err error)       // 更新 LastRun + LastError
```

**关键设计**: `sync.Mutex` + 原子写入，避免并发写入损坏

### 2.3 CronScheduler: 调度循环

```go
type CronScheduler struct {
    store   *CronStore
    cron    *cron.Cron            // github.com/robfig/cron/v3
    engines map[string]*Engine    // project → Engine
    entries map[string]cron.EntryID  // jobID → EntryID
}

func (cs *CronScheduler) executeJob(jobID string) {
    // 1. 获取 job
    // 2. context.WithTimeout(ctx, *job.TimeoutMins)
    // 3. 调用 engine.ExecuteCronJob(job)
    // 4. 更新 LastRun / LastError
}
```

**关键设计**:
- 依赖 `robfig/cron/v3`（广泛使用的 cron 库）
- 5字段 cron（分 时 日 月 周），支持 `@every`、宏
- `executeJob` 在 goroutine 中执行，支持超时控制
- `entries map[jobID]cron.EntryID` 便于删除/禁用

### 2.4 Agent 工具集成

cc-connect 在 `AGENTS.md` system prompt 中注入工具：

```
## 定时任务
cc-connect cron add --cron "0 6 * * *" --prompt "..." --desc "...";
cc-connect cron list;
cc-connect cron del --id <id>;
```

**HotPlex 适配**: Brain 意图识别 → 构造成 AddCronJob/DeleteCronJob/SuspendCronJob 请求

---

## 3. cc-connect relay.go 深度解析

### 3.1 核心数据结构

```go
type RelayBinding struct {
    Platform  string            `json:"platform"`
    ChatID   string            `json:"chat_id"`
    Bots     map[string]string `json:"bots"` // project name → bot display name
}

type RelayManager struct {
    engines   map[string]*Engine   // project → Engine
    bindings  map[string]*RelayBinding // chatID → binding
    storePath string
    timeout   time.Duration
}

type RelayRequest struct {
    From        string `json:"from"`
    To          string `json:"to"`
    SessionKey  string `json:"session_key"`
    Message     string `json:"message"`
}

type RelayResponse struct {
    Response string `json:"response"`
}
```

### 3.2 Send 流程

```
User: "@bot2 帮我看看这个问题"
  → Bridge 识别 @bot2
  → RelayManager.Send(to="bot2", message="帮我看看这个问题")
    → 解析 SessionKey
    → 验证 binding 和目标 engine
    → Engine.HandleRelay(request)
    → 目标 bot 执行任务
    → 返回响应
    → 在 group chat 发布 visibility 消息
```

**关键设计**:
- Binding 机制：Platform + ChatID 绑定一组 bots
- HTTP 无直接处理，依赖 Engine.HandleRelay
- 持久化 relay_bindings.json

---

## 4. OpenClaw AgentScope 框架实践

### 4.1 CronManager 设计

| 特性 | 实现 |
|------|------|
| **秒级精度** | 6字段 cron（秒 分 时 日 月 周） |
| **表达式示例** | `*/30 * * * * *` 每30秒；`0 */5 * * * *` 每5分钟 |
| **任务注册** | 内置工具：`add_cron`, `del_cron`, `list_crons` |
| **持久化** | `cron_jobs.json`，重启自动恢复 |
| **执行隔离** | 专用 `cronjob` session，串行执行 |
| **并发控制** | SessionManager 统一管理，任务不并发冲突 |

### 4.2 ReAct Agent 执行

```python
# cron job 触发时
agent = ReActAgent(config)
result = await agent.run(prompt)

# Agent 可主动调用 memory_search 等工具
# 内置 pre_reasoning hook 防幻觉
```

### 4.3 关键启示

1. **工具化注册**: Cron 作为内置工具，Agent 可以主动管理
2. **专用 Session**: `cronjob` session 与普通用户 session 完全隔离
3. **持久化**: JSON 文件，简单可靠，支持重启恢复

---

## 5. HotPlex 现有架构适配分析

### 5.1 可复用基础设施

| 组件 | 位置 | 复用价值 |
|------|------|---------|
| `SessionPool` | `internal/engine/pool.go` | Cron/Relay 任务的 session 管理 |
| `bridgewire` | `internal/bridgewire/wire.go` | WireMessage 格式可扩展支持 relay |
| `Engine` | `internal/engine/session.go` | Cron job 执行入口 |
| `safetyGuard` | `brain/guard.go` | Cron/Relay 意图识别扩展 |
| `hotplex.go` | `hotplex.go` | SDK 暴露 cron/relay 接口 |

### 5.2 架构差距

```
当前 HotPlex:
  Bridge ← Platform (Slack/TG/Ding)
              ↓
           Engine ← SessionPool
                        (无 cron、无 relay)

cc-connect:
  Bridge ← Platform
              ↓
  CronScheduler ← CronStore (JSON 持久化)
  RelayManager ← Engine
                    ↓
              CronJob 执行 / Relay Send
```

---

## 6. 设计方案：三阶段演进

### Phase 0: 架构基础（新增）

建立 cron + relay 的类型定义和存储层。

```
internal/cron/
├── job.go         # CronJob, CronStore 类型
├── scheduler.go   # CronScheduler (调度循环)
└── store.go       # 原子写入 JSON

internal/relay/
├── binding.go   # RelayBinding, RelayManager
└── sender.go   # HTTP Sender
```

### Phase 1: Cron 调度器（核心）

| 功能 | 实现方式 |
|------|---------|
| 表达式解析 | `github.com/robfig/cron/v3` |
| 持久化 | `cron_jobs.json`，原子写入 |
| 执行 | 专用 `cronjob` session（复用 SessionPool） |
| 自然语言解析 | Brain intent 扩展（guard.go） |
| 工具注入 | `add_cron`, `del_cron`, `list_crons`, `pause_cron`, `resume_cron` |

**关键决策**:
- 使用 `robfig/cron/v3`（Go 生态最成熟）
- Session 隔离：Cron job 使用 `namespace="hotplex-cron"` 独立于普通会话
- 配置驱动：YAML 配置 + 运行时 API

### Phase 2: Bot-to-Bot Relay（扩展）

| 功能 | 实现方式 |
|------|---------|
| 协议 | 复用 `bridgewire` WireMessage |
| 传输 | HTTP POST 到目标 HotPlex 实例 |
| 路由 | RelayBinding: Platform + ChatID → Bots |
| 安全 | API Key 认证 + WAF 检查 |
| 超时 | 可配置 timeout（默认 120s） |

**关键决策**:
- 不新建协议，复用 bridgewire 的 WireMessage
- Relay 作为 Bridge 的扩展，而非独立系统
- 目标 HotPlex 实例通过配置注册（URL + API Key）

---

## 7. 技术选型建议

| 决策点 | 推荐方案 | 理由 |
|--------|---------|------|
| Cron 库 | `robfig/cron/v3` | Go 生态标准，稳定可靠 |
| 持久化 | JSON 文件（原子写入） | 简单、无外部依赖、cc-connect 验证 |
| 执行隔离 | 独立 SessionPool (`cron-` 前缀) | HotPlex SessionPool 已支持多 namespace |
| 意图解析 | Brain 扩展（guard.go） | 架构一致，自然语言驱动 |
| Relay 协议 | 复用 bridgewire | 避免重复造轮子 |
| 自然语言注册 | 工具化（cc-connect 模式） | Agent 可主动管理 |

---

## 8. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| Cron 表达式解析错误 | 使用 `cron.ParseStandard()` + 验证 |
| 并发写入 jobs.json | `sync.Mutex` + 原子写入（os.Rename） |
| Cron job 执行阻塞 | context.WithTimeout + SessionGC |
| Relay 超时 | 可配置 timeout，优雅处理 |
| 安全风险 | WAF 检查 relay 消息内容 |

---

## 9. 参考资料

- [cc-connect 源码 core/cron.go](https://github.com/chenhg5/cc-connect/blob/main/core/cron.go)
- [cc-connect 源码 core/relay.go](https://github.com/chenhg5/cc-connect/blob/main/core/relay.go)
- [OpenClaw CronManager](https://github.com/owenliang/openclaw)
- [robfig/cron/v3](https://pkg.go.dev/github.com/robfig/cron/v3)
- [Issue #325 - Cron调度器 + Bot-to-Bot Relay](https://github.com/hrygo/hotplex/issues/325)
