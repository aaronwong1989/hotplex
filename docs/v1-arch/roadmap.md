# HotPlex v1.0.0 实现路线图

> 版本：v1.0  
> 日期：2026-03-29  
> 状态：完整实施计划

---

## 1. 概述

### 1.1 版本目标

**HotPlex v1.0.0** 是大版本重构，目标是：
- 建立模块化、插件化架构
- 实现核心接口抽象
- 保留向后兼容性
- 提升可维护性和扩展性

### 1.2 关键指标

| 指标 | 当前 | 目标 |
|------|------|------|
| 模块耦合度 | 高 | 低 (接口解耦) |
| 新增 Channel | 需要改核心 | 插件即可 |
| 新增 Worker | 需要改核心 | 插件即可 |
| 配置方式 | flag + yaml | 纯 yaml |
| 测试覆盖率 | ~40% | >70% |

---

## 2. 实现阶段

### Phase 1: MVP (v1.0.0-alpha)

**目标**：验证核心架构，实现最小可用版本

**时间**：4-6 周

#### 1.1 接口抽象层

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| 定义 Channel 接口 | - | - | ⏳ |
| 定义 Worker 接口 | - | - | ⏳ |
| 定义 Brain 接口 | - | - | ⏳ |
| 定义 Session 接口 | - | - | ⏳ |
| 定义 Storage 接口 | - | - | ⏳ |
| 定义 Provider 接口 | - | - | ⏳ |

#### 1.2 核心实现

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| 配置系统重构 (Viper + YAML) | - | 1.1 | ⏳ |
| Feishu Channel 迁移 | - | 1.1 | ⏳ |
| LLM Brain 实现 | - | 1.1 | ⏳ |
| ClaudeCode Worker 重构 | - | 1.1 | ⏳ |
| Session Manager 实现 | - | 1.1 | ⏳ |
| SQLite Storage 实现 | - | 1.1 | ⏳ |

#### 1.3 集成测试

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| 端到端测试 | - | 1.2 | ⏳ |
| 性能基准测试 | - | 1.2 | ⏳ |

**Phase 1 交付物**：
- `pkg/channel/channel.go` - Channel 接口 + Feishu 实现
- `pkg/worker/worker.go` - Worker 接口 + ClaudeCode 实现
- `pkg/brain/brain.go` - Brain 接口 + LLM 实现
- `pkg/session/session.go` - Session 管理
- `pkg/storage/storage.go` - Storage 接口 + SQLite 实现
- 完整配置系统

---

### Phase 2: 完整实现 (v1.0.0-beta)

**目标**：实现所有计划功能

**时间**：6-8 周

#### 2.1 Channel 扩展

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| Slack Channel 实现 | - | Phase 1 | ⏳ |
| WebSocket Channel 实现 | - | Phase 1 | ⏳ |
| Channel 中间件机制 | - | Phase 1 | ⏳ |

#### 2.2 Worker 扩展

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| OpenCode Worker 实现 | - | Phase 1 | ⏳ |
| Worker 进程池优化 | - | Phase 1 | ⏳ |
| Supervisor 重启策略 | - | Phase 1 | ⏳ |

#### 2.3 Brain 扩展

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| Rule Brain 实现 | - | Phase 1 | ⏳ |
| Context Enricher | - | Phase 1 | ⏳ |
| Response Formatter | - | Phase 1 | ⏳ |

#### 2.4 Storage 扩展

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| PostgreSQL Storage | - | Phase 1 | ⏳ |
| Redis Storage | - | Phase 1 | ⏳ |
| 全文搜索优化 | - | Phase 1 | ⏳ |

#### 2.5 Provider 扩展

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| OpenAI Provider | - | Phase 1 | ⏳ |
| SiliconFlow Provider | - | Phase 1 | ⏳ |
| Provider 熔断器 | - | Phase 1 | ⏳ |

**Phase 2 交付物**：
- 所有计划的 Channel 实现
- 所有计划的 Worker 实现
- 所有计划的 Brain 实现
- 所有计划的 Storage 实现
- 所有计划的 Provider 实现
- Supervisor 完整实现

---

### Phase 3: 插件化 (v1.0.0)

**目标**：完成插件系统，支持外部扩展

**时间**：4-6 周

#### 3.1 插件系统

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| 插件注册表 | - | Phase 2 | ⏳ |
| 动态加载 (.so) | - | Phase 2 | ⏳ |
| 插件配置机制 | - | Phase 2 | ⏳ |
| 插件签名验证 | - | Phase 2 | ⏳ |
| 插件安全沙箱 | - | Phase 2 | ⏳ |

#### 3.2 观测性

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| Prometheus 指标 | - | Phase 2 | ⏳ |
| OpenTelemetry 链路追踪 | - | Phase 2 | ⏳ |
| 健康检查 API | - | Phase 2 | ⏳ |
| 告警机制 | - | Phase 2 | ⏳ |

#### 3.3 文档

| 任务 | 负责人 | 依赖 | 状态 |
|------|--------|------|------|
| API 文档 | - | Phase 2 | ⏳ |
| 插件开发指南 | - | 3.1 | ⏳ |
| 部署指南 | - | Phase 2 | ⏳ |

**Phase 3 交付物**：
- 完整插件系统
- 完整观测性支持
- 完整文档

---

## 3. 迁移计划

### 3.1 从 v0.36.x 迁移

#### 配置迁移

```bash
# 自动转换旧配置
hotplex migrate config --from v0.36 --to v1.0 --input old-config.yaml --output new-config.yaml
```

#### 数据迁移

```bash
# 迁移消息数据
hotplex migrate data --storage sqlite --input old.db --output new.db
```

### 3.2 兼容性策略

| 特性 | v0.36.x | v1.0.0 | 兼容性 |
|------|---------|--------|--------|
| 配置格式 | yaml + flags | yaml | 部分兼容 |
| API 端点 | `/api/*` | `/api/v1/*` | 重定向 |
| WebSocket | 兼容 | 兼容 | 兼容 |
| Feishu 事件 | v1 | v1 | 兼容 |

---

## 4. 测试计划

### 4.1 单元测试

| 模块 | 覆盖率目标 | 关键测试 |
|------|-----------|----------|
| channel | >80% | 消息解析、格式转换 |
| worker | >80% | 任务执行、超时、取消 |
| brain | >70% | 意图分类、WAF |
| session | >80% | 创建、查询、过期 |
| storage | >80% | 增删改查、搜索 |

### 4.2 集成测试

| 测试 | 场景 |
|------|------|
| E2E-Feishu | 飞书消息 → Brain → Worker → Storage → 响应 |
| E2E-Slack | Slack 消息 → Brain → Worker → Storage → 响应 |
| E2E-WebSocket | WS 消息 → Brain → Worker → 流式响应 |
| 并发测试 | 多用户同时请求 |
| 故障恢复 | Worker 失败、Storage 失败 |

### 4.3 性能测试

| 指标 | 目标 |
|------|------|
| 吞吐量 | >100 req/s |
| P99 延迟 | <500ms (不含 Worker 执行) |
| 内存占用 | <500MB (idle) |
| 启动时间 | <5s |

---

## 5. 发布计划

### 5.1 版本节奏

| 版本 | 特性 | 日期 |
|------|------|------|
| v1.0.0-alpha.1 | 接口定义 + Feishu Channel | T+4 周 |
| v1.0.0-alpha.2 | Worker + Brain | T+6 周 |
| v1.0.0-beta.1 | Slack + WS + OpenCode | T+10 周 |
| v1.0.0-beta.2 | Postgres + Redis | T+12 周 |
| v1.0.0-rc.1 | 插件系统 + 观测性 | T+14 周 |
| v1.0.0 | 正式发布 | T+16 周 |

### 5.2 发布检查清单

- [ ] 所有单元测试通过
- [ ] 集成测试通过
- [ ] 性能测试达标
- [ ] 文档完整
- [ ] CHANGELOG 更新
- [ ] 版本标签打上
- [ ] Docker 镜像构建
- [ ] 安装脚本更新

---

## 6. 风险与缓解

### 6.1 技术风险

| 风险 | 影响 | 概率 | 缓解 |
|------|------|------|------|
| 接口设计不合理 | 高 | 中 | Phase 1 充分评审 |
| 性能下降 | 中 | 低 | 持续性能测试 |
| 迁移复杂度 | 中 | 中 | 提供迁移工具 |
| 插件安全 | 高 | 低 | 沙箱 + 签名验证 |

### 6.2 资源风险

| 风险 | 影响 | 概率 | 缓解 |
|------|------|------|------|
| 开发人员不足 | 高 | 中 | 优先级排序 |
| 依赖外部服务 | 中 | 低 | 准备备选 |

---

## 7. 里程碑

### M1: 接口定义完成 ✅

- [x] README.md - 架构概述
- [x] interface-design.md - 接口定义
- [x] message-flow.md - 消息流
- [x] configuration.md - 配置格式
- [x] plugin-system.md - 插件机制
- [x] error-handling.md - 错误处理
- [x] roadmap.md - 实现路线图

### M2: Phase 1 完成

- [ ] Channel 接口 + Feishu 实现
- [ ] Worker 接口 + ClaudeCode 实现
- [ ] Brain 接口 + LLM 实现
- [ ] Session Manager
- [ ] SQLite Storage
- [ ] 配置系统

### M3: Phase 2 完成

- [ ] Slack Channel
- [ ] WebSocket Channel
- [ ] OpenCode Worker
- [ ] Rule Brain
- [ ] PostgreSQL Storage
- [ ] Redis Storage

### M4: Phase 3 完成

- [ ] 插件系统
- [ ] 动态加载
- [ ] Prometheus 指标
- [ ] OpenTelemetry 追踪
- [ ] 完整文档

### M5: v1.0.0 发布

- [ ] 所有测试通过
- [ ] 性能达标
- [ ] 文档完整
- [ ] 正式发布

---

## 8. 附录

### 8.1 术语表

| 术语 | 定义 |
|------|------|
| Channel | 平台适配层（飞书、Slack、WebSocket） |
| Brain | 原生智能层（意图分类、WAF、路由） |
| Worker | 任务执行层（ClaudeCode、OpenCode） |
| Provider | LLM 提供商（Anthropic、OpenAI） |
| Session | 用户会话 |
| Task | 执行任务 |
| Plugin | 插件扩展 |

### 8.2 参考文档

- 接口设计：`interface-design.md`
- 消息流：`message-flow.md`
- 配置格式：`configuration.md`
- 插件机制：`plugin-system.md`
- 错误处理：`error-handling.md`

---

*文档版本：v1.0 | 最后更新：2026-03-29*
