# HotPlex v1.0.0 架构总览

> 版本：v1.0-final  
> 日期：2026-03-29  
> 状态：**已确认**

---

## 一、核心架构

### 1.1 架构概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HotPlex Runtime                                │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Channel Layer                                 │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │   │
│  │  │ Feishu   │  │  Slack   │  │    WS    │  │  API     │           │   │
│  │  │ Channel  │  │ Channel  │  │ Channel  │  │ Channel  │           │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Brain Layer (Native)                          │   │
│  │                                                                 ====   │   │
│  │   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │   │
│  │   │  Intent   │  │   WAF     │  │ Context   │  │  Memory   │    │   │
│  │   │  Router   │  │  Filter   │  │  Enhancer │  │  Manager  │    │   │
│  │   └────────────┘  └────────────┘  └────────────┘  └────────────┘    │   │
│  │                                                                      │   │
│  │   ★ Native Brain 不是 Worker，是独立智能层                           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Supervisor                                    │   │
│  │              (Worker 生命周期管理 + 健康检查)                           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Docker Plugin Gateway                             │   │
│  │            (Unix Domain Socket / HTTP → Docker API)                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                              │       │       │                            │
│                              ▼       ▼       ▼                            │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ ClaudeCode │  │ ClaudeCode │  │ OpenCode   │  │  Custom    │         │
│  │  Worker    │  │  Worker    │  │  Worker    │  │  Plugin    │         │
│  │ Container  │  │ Container  │  │ Container  │  │ Container  │         │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘         │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      Storage Layer (Local)                            │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │   │
│  │  │  SQLite      │  │    Redis    │  │    File      │              │   │
│  │  │  Session    │  │   Cache     │  │   Storage    │              │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 核心组件职责

| 组件 | 类型 | 职责 |
|------|------|------|
| **Channel** | Native | 消息收发协议适配（Feishu/Slack/WS/API） |
| **Brain** | Native | 意图路由、WAF 过滤、上下文增强、记忆管理 |
| **Supervisor** | Native | Worker 生命周期管理、健康检查、容器编排 |
| **Worker** | Docker Container | ClaudeCode 或 OpenCode，执行具体任务 |
| **Provider** | Native | 模型调用路由（Anthropic/OpenAI/SiliconFlow） |
| **Storage** | Native | Session 存储、Memory 存储、文件存储 |

### 1.3 与 OpenClaw 的核心差异

| 维度 | HotPlex | OpenClaw |
|------|---------|----------|
| **插件隔离** | Docker 容器（已废弃 Go .so） | Docker 容器 |
| **Native Brain** | ✅ 原生智能层（意图路由+WAF+上下文） | ❌ 无独立 Brain 层 |
| **Worker 类型** | ClaudeCode / OpenCode | Claude Code / Codex / OpenCode |
| **Session 存储** | SQLite（本地）+ Memory LRU | 内存 + 磁盘 |
| **部署模式** | 单机 + 插件按需拉起 | 单机 + 持久化 Agent |
| **多租户** | 通过 Channel 隔离 | 通过 Workspace 隔离 |

---

## 二、模块目录结构

```
hotplex/
├── cmd/
│   └── hotplex/
│       └── main.go              # 程序入口
│
├── pkg/
│   ├── channel/                 # 通道层
│   │   ├── feishu/             # 飞书频道
│   │   ├── slack/               # Slack 频道
│   │   ├── ws/                 # WebSocket 频道
│   │   └── api/                # REST API 频道
│   │
│   ├── brain/                   # 原生智能层
│   │   ├── intent/             # 意图识别
│   │   ├── waf/                # WAF 过滤
│   │   ├── context/            # 上下文增强
│   │   └── memory/             # 记忆管理
│   │
│   ├── worker/                  # Worker 核心
│   │   ├── supervisor.go       # 监管者
│   │   ├── pool.go             # Worker 池
│   │   └── lifecycle.go        # 生命周期
│   │
│   ├── plugin/                  # 插件系统
│   │   ├── container.go        # 容器插件基类
│   │   ├── registry.go         # 插件注册表
│   │   └── docker.go           # Docker 驱动
│   │
│   ├── provider/               # 模型提供者
│   │   ├── anthropic.go        # Anthropic
│   │   ├── openai.go           # OpenAI
│   │   └── siliconflow.go      # 硅基流动
│   │
│   ├── storage/                # 存储层
│   │   ├── session.go          # Session 存储
│   │   ├── memory.go           # Memory 存储
│   │   └── file.go             # 文件存储
│   │
│   ├── observability/          # 观测性
│   │   ├── metrics.go          # Prometheus 指标
│   │   ├── tracing.go          # OpenTelemetry
│   │   └── logging.go          # 结构化日志
│   │
│   └── security/               # 安全模块
│       ├── seccomp.go          # Seccomp 配置
│       ├── capability.go       # Capabilities 管理
│       └── network.go          # 网络策略
│
├── plugins/                    # 内置插件源码
│   ├── claude-code-worker/     # ClaudeCode Worker
│   └── open-code-worker/       # OpenCode Worker
│
├── configs/
│   └── hotplex.yaml            # 主配置
│
└── docs/
    └── v1-arch/                # 架构文档
        ├── README.md           # 本文件
        ├── interface-design.md # 接口定义
        ├── docker-plugin-system.md  # Docker 插件系统
        ├── security-model.md    # 安全模型
        ├── message-flow.md     # 消息流
        ├── configuration.md     # 配置格式
        └── roadmap.md          # 实现路线图
```

---

## 三、关键设计约束

### 3.1 插件隔离：Docker 容器

```
┌─────────────────────────────────────┐
│     HotPlex Host (root process)      │
│                                      │
│   ┌───────────────────────────────┐ │
│   │   Plugin Gateway (UDS/HTTP)   │ │
│   └───────────────────────────────┘ │
│                  │                   │
│   ┌──────────────┼──────────────┐   │
│   │  Docker API  │              │   │
│   └──────────────┼──────────────┘   │
│                  ▼                   │
│   ┌───────────────────────────────┐ │
│   │  Worker Container (non-root)  │ │
│   │  ├── Alpine/Ubuntu base       │ │
│   │  ├── Claude Code / OpenCode   │ │
│   │  └── 独立网络/PID/挂载命名空间│ │
│   └───────────────────────────────┘ │
└─────────────────────────────────────┘
```

### 3.2 安全加固

| 安全机制 | 实现 |
|----------|------|
| **Capabilities** | Worker 丢弃所有危险 capabilities，仅保留必要权限 |
| **Seccomp** | 白名单模式，仅允许安全 syscalls |
| **No-New-Privileges** | 禁止 setuid/setgid |
| **Read-Only Root** | 根文件系统只读，仅白名单目录可写 |
| **网络隔离** | 容器网络独立，LAN-only 出站 |
| **资源限制** | CPU/内存/磁盘配额 |

---

## 四、文档索引

| 文档 | 内容 |
|------|------|
| **[README.md](README.md)** | 架构总览、目录结构、差异说明 |
| **[interface-design.md](interface-design.md)** | 7 大接口、Docker 插件、观测性接口定义 |
| **[docker-plugin-system.md](docker-plugin-system.md)** | Docker 容器隔离插件系统详解 |
| **[security-model.md](security-model.md)** | 纵深安全模型、Capabilities、Seccomp |
| **[message-flow.md](message-flow.md)** | 消息流时序图、异常路径 |
| **[configuration.md](configuration.md)** | YAML 配置格式、安全配置 |
| **[roadmap.md](roadmap.md)** | 3 阶段实现计划 |

---

## 五、设计原则

1. **Native Brain 独立** - Brain 不是 Worker，是独立智能层，负责意图路由、WAF、上下文增强
2. **Worker 极简** - Worker 仅负责执行 ClaudeCode/OpenCode，不处理协议
3. **安全第一** - Docker 容器隔离 + Capabilities + Seccomp + No-New-Privileges
4. **本地优先** - Session 存储在 SQLite，不依赖外部 Redis
5. **可观测** - Prometheus + OpenTelemetry 内置，Phase 1 即包含

---

_最后更新：2026-03-29_
