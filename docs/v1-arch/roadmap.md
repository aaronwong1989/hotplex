# HotPlex v1.0.0 实现路线图

> 版本：v1.0-final  
> 日期：2026-03-29  
> 状态：**已确认**

---

## 一、总体计划

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          HotPlex v1.0.0 实现路线图                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Phase 1: MVP                    Phase 2: 完整实现           Phase 3: 生态  │
│  ════════════                    ══════════════════           ═══════════  │
│                                                                             │
│  ┌─────────────┐                  ┌─────────────┐             ┌───────────┐ │
│  │ Feishu     │                  │ Storage     │             │ Observab. │ │
│  │ Channel    │                  │ 完整实现    │             │ 完整实现  │ │
│  └─────────────┘                  └─────────────┘             └───────────┘ │
│  ┌─────────────┐                  ┌─────────────┐             ┌───────────┐ │
│  │ Native      │                  │ Session     │             │ Prometheus│ │
│  │ Brain      │                  │ 分层存储    │             │ + Grafana │ │
│  └─────────────┘                  └─────────────┘             └───────────┘ │
│  ┌─────────────┐                  ┌─────────────┐             ┌───────────┐ │
│  │ ClaudeCode  │                  │ Memory      │             │ Open      │ │
│  │ Worker     │                  │ 长期记忆    │             │ Telemetry │ │
│  └─────────────┘                  └─────────────┘             └───────────┘ │
│  ┌─────────────┐                  ┌─────────────┐             ┌───────────┐ │
│  │ Docker     │                  │ 安全模型    │             │ 多 Worker │ │
│  │ 插件系统   │                  │ 完整落地    │             │ 协同      │ │
│  └─────────────┘                  └─────────────┘             └───────────┘ │
│                                                                             │
│  ──────────────────────────────────────────────────────────────────────   │
│                                                                             │
│  Week 1-2     Week 3-4          Week 5-8          Week 9-12                 │
│  ──────────────────────────────────────────────────────────────────────   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 二、Phase 1: MVP（第 1-4 周）

### 目标
- 最小可用版本，验证核心架构
- 支持飞书通道 + ClaudeCode Worker
- Docker 插件系统可用

### 交付物

| 编号 | 组件 | 任务 | 周数 | 负责人 |
|------|------|------|------|--------|
| 1.1 | Channel | 飞书通道实现（接收/发送/回调） | Week 1 | - |
| 1.2 | Channel | 飞书事件处理（消息/加群/退群） | Week 1 | - |
| 1.3 | Brain | 意图路由基础（闲聊/任务/代码分类） | Week 1-2 | - |
| 1.4 | Brain | WAF 基础（敏感信息检测） | Week 1-2 | - |
| 1.5 | Worker | ClaudeCode Worker Docker 镜像 | Week 2 | - |
| 1.6 | Worker | Supervisor + Worker 池 | Week 2-3 | - |
| 1.7 | Worker | Docker 插件 Gateway（UDS） | Week 2-3 | - |
| 1.8 | Storage | Session SQLite 存储 | Week 3 | - |
| 1.9 | Storage | Memory LRU 缓存 | Week 3 | - |
| 1.10 | Provider | Anthropic Provider | Week 3 | - |
| 1.11 | Config | YAML 配置加载 | Week 3 | - |
| 1.12 | Security | Docker 安全加固（Capabilities） | Week 4 | - |
| 1.13 | E2E | 端到端集成测试 | Week 4 | - |

### 验收标准
- [ ] 飞书消息收发正常
- [ ] ClaudeCode Worker 容器可正常启动
- [ ] WAF 可拦截敏感信息
- [ ] Session 可正常存储/读取
- [ ] 端到端消息流完整

### 里程碑检查点

```
Week 1 (Day 5): 
  □ Channel 层可用
  □ 飞书消息可接收

Week 2 (Day 10): 
  □ Brain 意图路由可用
  □ Docker 容器可启动

Week 3 (Day 15): 
  □ Worker 池可用
  □ Session 存储可用

Week 4 (Day 20): 
  □ 端到端测试通过
  □ Phase 1 完成
```

---

## 三、Phase 2: 完整实现（第 5-8 周）

### 目标
- 实现完整 Session/Memory 存储体系
- 完善安全模型
- 增加 Slack/WS 通道

### 交付物

| 编号 | 组件 | 任务 | 周数 | 负责人 |
|------|------|------|------|--------|
| 2.1 | Channel | Slack 通道实现 | Week 5 | - |
| 2.2 | Channel | WebSocket 通道实现 | Week 5 | - |
| 2.3 | Brain | 上下文增强（历史摘要） | Week 5-6 | - |
| 2.4 | Brain | Memory Manager（分层记忆） | Week 5-6 | - |
| 2.5 | Storage | Session 分层存储（LRU+SQLite） | Week 6 | - |
| 2.6 | Storage | Memory 长期存储 | Week 6 | - |
| 2.7 | Security | Seccomp 白名单 | Week 6-7 | - |
| 2.8 | Security | 网络隔离策略 | Week 6-7 | - |
| 2.9 | Worker | OpenCode Worker | Week 7 | - |
| 2.10 | Provider | OpenAI Provider | Week 7 | - |
| 2.11 | Provider | SiliconFlow Provider | Week 7 | - |
| 2.12 | E2E | 完整集成测试 | Week 8 | - |

### 验收标准
- [ ] Slack/WS 通道可用
- [ ] Memory 分层存储完整
- [ ] Seccomp 白名单生效
- [ ] 网络隔离配置生效
- [ ] 多 Provider 切换正常

---

## 四、Phase 3: 插件生态（第 9-12 周）

### 目标
- 完善观测性体系
- 多 Worker 协同
- 插件生态基础

### 交付物

| 编号 | 组件 | 任务 | 周数 | 负责人 |
|------|------|------|------|--------|
| 3.1 | Observability | Prometheus 指标导出 | Week 9 | - |
| 3.2 | Observability | OpenTelemetry 追踪 | Week 9 | - |
| 3.3 | Observability | Grafana Dashboard | Week 9-10 | - |
| 3.4 | Worker | 多 Worker 协同机制 | Week 10 | - |
| 3.5 | Worker | Worker 任务分发策略 | Week 10 | - |
| 3.6 | Plugin | 插件注册表 | Week 10-11 | - |
| 3.7 | Plugin | 插件热更新机制 | Week 10-11 | - |
| 3.8 | Plugin | 自定义 Brain 插件 | Week 11 | - |
| 3.9 | Admin | Admin Dashboard | Week 11-12 | - |
| 3.10 | Admin | 监控告警 | Week 11-12 | - |
| 3.11 | E2E | 完整测试 + 性能测试 | Week 12 | - |

### 验收标准
- [ ] Prometheus 指标完整
- [ ] 链路追踪可用
- [ ] 多 Worker 可协同工作
- [ ] 插件热更新可用
- [ ] Admin Dashboard 可用

---

## 五、里程碑

### 5.1 里程碑总览

| 里程碑 | 日期 | 内容 |
|--------|------|------|
| M1: MVP | Week 4 | Phase 1 完成，可基本运行 |
| M2: 多通道 | Week 8 | Phase 2 完成，支持多通道 |
| M3: 可观测 | Week 12 | Phase 3 完成，完整可观测 |
| M4: v1.0.0 GA | Week 16 | 正式发布 |

### 5.2 详细里程碑

```
M1: MVP (Week 4 - Day 20)
├── Channel: Feishu ✓
├── Brain: Intent Router ✓
├── Brain: WAF ✓
├── Worker: ClaudeCode ✓
├── Storage: Session ✓
└── Security: Docker ✓

M2: 多通道 (Week 8 - Day 40)
├── Channel: Slack ✓
├── Channel: WebSocket ✓
├── Brain: Memory Manager ✓
├── Storage: 分层存储 ✓
├── Security: Seccomp ✓
└── Security: 网络隔离 ✓

M3: 可观测 (Week 12 - Day 60)
├── Observability: Prometheus ✓
├── Observability: OpenTelemetry ✓
├── Worker: 多 Worker 协同 ✓
├── Plugin: 热更新 ✓
└── Admin: Dashboard ✓

M4: v1.0.0 GA (Week 16 - Day 80)
├── 性能测试通过 ✓
├── 安全审计通过 ✓
├── 文档完善 ✓
├── 正式发布 ✓
└── 社区反馈 ✓
```

---

## 六、风险与依赖

### 6.1 已知风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| Docker 容器性能开销 | 中 | 高 | 优化镜像大小，Worker 池复用 |
| ClaudeCode Worker 稳定性 | 高 | 中 | 健康检查，自动重启 |
| WAF 误拦截 | 中 | 低 | 规则细化，支持白名单 |
| 多 Provider 切换延迟 | 低 | 低 | 预热连接池 |

### 6.2 外部依赖

| 依赖 | 类型 | 说明 |
|------|------|------|
| Docker | 运行时 | 必须安装 Docker |
| Anthropic API | 外部服务 | ClaudeCode 依赖 |
| 飞书开放平台 | 外部服务 | 消息收发 |

---

## 七、测试策略

### 7.1 测试分层

```
┌─────────────────────────────────────────────────────────────┐
│                        测试金字塔                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│                         ▲                                    │
│                        /E2E\         端到端测试               │
│                       /─────\        (用户故事)              │
│                      / Unit  \       单元测试                │
│                     /─────────\      集成测试                │
│                    /  Contract  \    契约测试                │
│                   /─────────────\                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 7.2 测试覆盖率目标

| 层级 | 覆盖率目标 |
|------|------------|
| 单元测试 | 80%+ |
| 集成测试 | 70%+ |
| E2E 测试 | 关键路径 100% |

---

_最后更新：2026-03-29_
