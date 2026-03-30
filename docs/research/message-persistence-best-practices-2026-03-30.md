# 消息持久化 — 最佳实践调研

> 调研日期：2026-03-30 | 约束：引用权威来源，评估权衡，明确 MVP vs 生产方案

---

## 1. 事件溯源模式

### 1.1 核心定义（Martin Fowler）

事件溯源（Event Sourcing）将应用状态的每一次变更作为**离散事件序列**持久化，而非仅存储最新状态。其核心思想是：事件日志是唯一真相来源（Source of Truth），当前状态通过重放事件重建。

**三种重建能力**：

- **Complete Rebuild**：丢弃全部状态，从事件日志重头重放
- **Temporal Query**：重建任意时间点的状态（时间旅行）
- **Event Replay**：修正错误事件 — 反向补偿后用正确数据重放

> "A domain event is a fully-fledged part of the domain model, a representation of something that happened in the domain." — Eric Evans, *Domain-Driven Design*

### 1.2 CQRS 架构

CQRS（Command Query Responsibility Segregation）将读操作与写操作解耦。事件溯源天然与 CQRS 配合：写侧追加事件到 Event Store，读侧通过 Projections 构建查询模型。

```
Command Side                    Query Side
┌──────────────┐              ┌──────────────┐
│  发送 Command │              │  发送 Query   │
│  (业务意图)   │              │  (数据展示)   │
└──────┬───────┘              └──────▲───────┘
       │                             │
       ▼                             │
┌──────────────┐              ┌──────────────┐
│  Event Store  │  ──────▶    │ Projections  │
│ (Append-only) │   异步投影    │ (只读视图)   │
└──────────────┘              └──────────────┘
```

### 1.3 事件 Schema 演进

Gregory Young（CQRS/ES 领域权威）在 *Versioning in an Event Sourced System* 中定义的演进策略：

| 策略 | 适用场景 | 代价 |
|------|----------|------|
| **Upcasting**（运行时版本转换） | 新版应用兼容旧事件 | 需要维护 Upcaster 链 |
| **Event Versioning**（版本化事件类型） | 破坏性 Schema 变更 | 版本爆炸 |
| **Copy-and-Alter**（复制后修改） | 重大版本里程碑 | 全量重放，计算成本高 |

**HotPlex 建议**：采用 Upcasting 模式，主版本号内向前兼容，通过 `event_version` 字段标记 Schema 版本。

### 1.4 Aggregates 与 Stream 建模

- **Stream**：单一实体（Aggregate）的有序事件集合，以实体 ID 为 Stream Name
- **事件命名**：使用过去式动词（`UserRegistered`, `SessionStarted`），反映业务事实而非技术操作
- **When/Apply 模式**：Entity 通过 `When(event)` 方法根据事件类型变更状态

---

## 2. 事件存储设计

### 2.1 专用 Event Store（KurrentDB / Event Store DB）

KurrentDB（原 EventStoreDB）是原生事件存储的标杆实现：

```
┌─────────────────────────────────────────────┐
│              Event Store (KurrentDB)        │
│  • Append-only log (不可变)                  │
│  • Stream = 单一 Aggregate 的事件序列         │
│  • Native projections (异步构建只读视图)       │
│  • Subscriptions (实时推送事件)              │
│  • Built-in CQRS support                    │
└─────────────────────────────────────────────┘
```

**优势**：事件溯源一等公民，无需自己实现投影、重放、订阅等机制
**代价**：额外的运维组件，学习曲线

### 2.2 PostgreSQL JSONB 事件存储

PostgreSQL 是成熟的事件存储方案，HotPlex 生产级实现已采用此方案：

```sql
-- HotPlex 当前 PostgreSQL Schema（生产级）
CREATE TABLE messages (
    id VARCHAR(64) PRIMARY KEY,
    chat_session_id VARCHAR(128) NOT NULL,
    metadata JSONB,                    -- 灵活扩展字段
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted BOOLEAN DEFAULT FALSE,
    ...
) PARTITION BY RANGE (created_at);    -- 按时间分区，支撑 100M+ 行
```

**JSONB 事件存储设计要点**：

- 使用 GIN 索引加速 JSONB 字段查询（HotPlex 已实现 `USING GIN (metadata)`）
- `metadata JSONB` 允许存储事件的可选属性，无需 ALTER TABLE
- 时间分区（Partitioning）是水平扩展的关键 — 按月分区，自动管理历史数据
- 软删除（`deleted` 字段）而非物理删除，满足合规要求

**与专用 Event Store 的对比**：

| 维度 | PostgreSQL JSONB | KurrentDB |
|------|-----------------|-----------|
| 运维复杂度 | 低（共享团队技能） | 高（专用组件） |
| 查询灵活性 | 高（SQL + JSONB） | 有限（Projections） |
| 事件重放 | 需自行实现 | 原生支持 |
| 扩展性 | 分区 → 100M+ 行 | 流级别分区 |
| 事务保证 | ACID | Eventual（需幂等消费） |

### 2.3 SQLite WAL 模式在事件存储的应用

HotPlex MVP 实现已采用 SQLite WAL 模式：

```go
pragmas := []string{
    "PRAGMA journal_mode=WAL",   // Append-only 风格的并发安全
    "PRAGMA busy_timeout=5000",  // 5s 锁等待
    "PRAGMA synchronous=NORMAL", // 性能与安全的平衡
}
```

**WAL 模式特性**（[SQLite 官方文档](https://www.sqlite.org/wal.html)）：

- 写操作追加到独立的 WAL 文件，而非覆写 rollback journal
- 读操作不阻塞写，写操作不阻塞读（高并发场景优于默认 DELETE journal 模式）
- Checkpoint 将 WAL 内容合并回主数据库文件
- 适合 HotPlex 单实例多连接的场景

**局限**：WAL 是数据库内部机制，不是"事件溯源模式" — 它提供的是原子性和并发安全，而非事件语义层面的不可变日志。

### 2.4 Apache Kafka 作为事件存储

Kafka 通过 Log compaction 保留每个 Key 的最新值，适合作为事件流平台而非传统 Event Store：

- **优点**：天然分布式，Consumer Groups 消费，Retaintion 策略灵活
- **缺点**：无原生 Stream 概念，无事务边界保证，投影需自行构建
- **适用场景**：需要跨服务事件分发的 Event-Driven Architecture

---

## 3. 审计日志规范

### 3.1 审计日志设计原则

审计日志（Audit Log）是事件溯源的最强用例之一。核心设计原则：

1. **Append-only**：只追加，不更新，不删除（除非合规要求）
2. **不可变字段**：每次变更产生新记录，保留历史快照
3. **足够上下文**：Who、What、When、Where、Why
4. **防篡改**：哈希链（每条记录包含前一条记录的哈希）或外部签名

### 3.2 不可篡改性保证

| 机制 | 实现方式 | 适用场景 |
|------|----------|----------|
| **物理不可删除** | 数据库层面 REVOKE DELETE | 高合规要求 |
| **哈希链** | 每条记录含 `prev_hash`，Chain integrity | 链上数据 |
| **外部签名** | HSM / KMS 对记录签名 | SOC2 Type II |
| **WORM 存储** | 写入后光盘/磁带不可覆写 | 法规保留要求 |

### 3.3 合规要求映射

| 合规标准 | 关键要求 | 实现要点 |
|----------|----------|----------|
| **SOC2 Type II** | 审计日志完整性、操作可追溯 | Append-only + 哈希链 |
| **GDPR Art. 5(1)(e)** | 存储期限限制，不是立即删除 | 软删除 + 延迟物理删除 |
| **HIPAA** | 防篡改 + 访问控制 | RBAC + 加密 + WAL |
| **PCI-DSS** | 保留至少 1 年 | 分区保留策略 |

**GDPR 特别说明**：审计日志（系统操作日志）不同于用户数据。GDPR 规定的"删除权"通常不适用于审计日志 — 但需在 Privacy Policy 中明确区分。

### 3.4 Log Structured Storage

Linux auditd、AWS CloudTrail、Datadog 的审计日志均采用 Append-only Log 结构：

```
┌──────────────────────────────────────────────────┐
│  seq=1 | action=CREATE | user=alice | ts=...    │
│  seq=2 | action=UPDATE | user=bob   | ts=... | prev_hash=sha256(seq1) │
│  seq=3 | action=DELETE | user=alice | ts=... | prev_hash=sha256(seq2) │
└──────────────────────────────────────────────────┘
```

---

## 4. 会话重放方案

### 4.1 OpenTelemetry 事件模型

OpenTelemetry 定义了分布式追踪的核心数据模型：

- **Trace**：跨进程的一次完整请求链路（16-byte TraceId）
- **Span**：单一操作单元（8-byte SpanId），含 StartTime、EndTime、Attributes、Events
- **SpanEvent**：Span 上的带时间戳事件点，用于记录关键子步骤

```
Span: Session[id=abc]
  ├── Event: "message_received" {user_id: "u1", platform: "slack"}
  ├── Event: "engine_started" {engine: "opencode"}
  ├── Event: "response_sent" {latency_ms: 1200}
  └── Links: [SpanContext of parent session]
```

OpenTelemetry SpanEvent 可用于**会话内事件重放**，但 Span 是追踪基础设施，不适合作为主要事件存储。

### 4.2 会话重放的三个层次

| 层次 | 粒度 | 重放能力 | 实现难度 |
|------|------|----------|----------|
| **消息级重放** | 单条消息（ChatAppMessage） | 任意消息的发送/响应重新展示 | 低 |
| **会话级重放** | 完整会话历史（从头到尾） | 历史上下文重建，用户可见 | 中 |
| **引擎级重放** | AI CLI 内部操作 | AI CLI 内部状态恢复，研发调试 | 高 |

**HotPlex 当前能力**：

- 消息级重放：通过 `ChatAppMessageStore.List(ctx, query)` 查询历史消息
- 会话级重放：通过 `ExportToJSON` / `ImportFromJSON` 导出导入完整会话
- 引擎级重放：**未实现** — 需要 AI CLI 原生支持 Event Sourcing

### 4.3 会话恢复最佳实践

- **幂等消息 ID**：使用 UUID 作为消息 ID，确保重放不会产生重复消息
- **时间戳作为序列号**：`created_at` 用于事件排序（非单调递增的序列号）
- **快照 + 增量**：`session_metadata` 表（HotPlex 已实现）存储最新会话摘要，加速长会话加载
- **跨会话上下文**：`engine_session_id` 作为跨平台的会话链接键

---

## 5. 推荐方案（SQLite vs PostgreSQL）

### 5.1 决策矩阵

| 维度 | SQLite（WAL） | PostgreSQL（JSONB） |
|------|--------------|---------------------|
| **适用规模** | < 100 万条消息 | 100M+ 条消息 |
| **运维复杂度** | 极低（单文件） | 中（需要 DBA） |
| **并发写入** | 1 writer（可读并发） | 多 writer（连接池） |
| **分区支持** | 无（需手动归档） | 原生 Range Partitioning |
| **网络访问** | 本地文件 | TCP 连接 |
| **备份** | 文件复制 | pg_dump + WAL archiving |
| **HotPlex 当前状态** | 已实现（生产就绪） | 已实现（生产就绪） |
| **合规能力** | 基础（WAL 不可变保证弱） | 强（JSONB + RLS + pgaudit） |

### 5.2 HotPlex 适用判断

**HotPlex 的特点**：

- 消息体是聊天消息（而非金融交易），容错性较高
- 单实例部署（Docker），多 bot 并发
- 会话有自然 TTL（Idle timeout）

### 5.3 分阶段推荐

#### MVP 阶段（当前 ~v0.36.0）

**推荐：SQLite WAL 模式**（已实现）

- 零运维，单文件 `chatapp_messages.db`
- WAL 模式提供足够的并发安全和持久性
- `ExportToJSON` 支持会话迁移
- 适合独立部署场景

**需补充的 MVP 改进**：

```go
// 1. 事件化 Schema：将 messages 表视为 Append-only Event Log
ALTER TABLE messages ADD COLUMN event_version INT DEFAULT 1;
ALTER TABLE messages ADD COLUMN event_type TEXT;  -- 'user_message' | 'bot_response'

// 2. 不可变约束（触发器）
CREATE TRIGGER prevent_message_update
    BEFORE UPDATE ON messages
    FOR EACH ROW
    WHEN OLD.deleted = 0  -- 防止更新未删除记录
BEGIN
    SELECT RAISE(ABORT, 'Messages are immutable');
END;
```

#### 生产环境阶段（v1.0+）

**推荐：PostgreSQL JSONB + 分区**

- 时间分区（按月）管理历史数据，保留策略自动化
- JSONB `metadata` 字段存储事件化扩展属性
- `pgaudit` 扩展记录 DDL 和 DML 操作，满足 SOC2 要求
- 连接池（PgBouncer）支持多 worker 并发

```sql
-- 生产 Schema 增强
CREATE TABLE messages (...) PARTITION BY RANGE (created_at);

-- 每月自动创建分区（通过 pg_partman 或 cron）
CREATE TABLE messages_2026_04 PARTITION OF messages
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- GIN 索引加速 metadata 查询
CREATE INDEX idx_messages_metadata ON messages USING GIN (metadata);

-- pgaudit 配置（PostgreSQL conf）
pgaudit.log = 'WRITE, READ';  -- 记录所有写和读操作
```

---

## 6. 关键决策点

### 决策 1：当前 HotPlex 存储是 Event Sourcing 吗？

**不完全是**。当前实现是 **CRUD + 软删除** 模式：

- `INSERT OR REPLACE` 支持消息更新（`storeMessage` 中的 `upsert` 语义）
- 缺少事件类型（`event_type`）和版本（`event_version`）字段
- 没有事件溯源的核心能力：Temporal Query、Event Replay、Complete Rebuild

**差距分析**：

| Event Sourcing 能力 | 当前状态 | 改进建议 |
|---------------------|----------|----------|
| 完整历史不可变 | ❌ 支持 UPDATE | 改为纯 INSERT + soft-delete |
| 事件类型标记 | ❌ 无 | 添加 `event_type` 字段 |
| 事件版本演进 | ❌ 无 | 添加 `event_version` 字段 |
| 投影构建 | ❌ 直接查询 | 可复用现有查询层 |
| 快照支持 | 部分（session_metadata） | 定期快照加速长会话 |

### 决策 2：需要引入 Kafka / KurrentDB 吗？

**当前不需要**。HotPplex 是单实例 CLI 代理服务，不跨服务分发事件。引入专用 Event Store 会显著增加运维复杂度。PostgreSQL JSONB 事件存储已足够支撑当前和未来 1-2 年的规模。

### 决策 3：审计日志合规需要什么？

- **MVP**：Append-only SQLite + WAL 模式 + 软删除（当前方案已部分满足）
- **SOC2**：PostgreSQL + `pgaudit` 扩展 + 哈希链 + 物理不可删除触发器
- **GDPR**：软删除（`deleted` 字段）+ 延迟物理删除（保留 7 年）+ Privacy Policy 披露

### 决策 4：会话重放的边界在哪里？

- **消息级**（当前可实现）：通过 `List()` API 重建上下文
- **会话级**（需增强）：实现 `ExportToJSON` 定时快照 + 引擎级状态序列化
- **引擎级**（长期目标）：需要 OpenCode/Claude Code 原生 Event Sourcing 支持

---

## 附录：HotPlex 当前存储实现评估

### 已实现的能力

| 能力 | SQLite | PostgreSQL |
|------|--------|------------|
| WAL / Append 日志 | ✅ WAL 模式 | ✅ JSONB + 分区 |
| 软删除 | ✅ | ✅ |
| 会话元数据 | ✅ | ✅ |
| 元数据 JSON 字段 | ✅ TEXT（JSON 序列化） | ✅ JSONB（原生索引） |
| 消息查询（List/Count） | ✅ | ✅ |
| 导出/导入 | ✅ ExportToJSON | ✅（同接口） |
| 分区支持 | ❌ | ✅ |

### 改进路线图建议

```
v0.37: 添加 event_type, event_version 字段（向后兼容）
       SQLite 添加不可变触发器

v0.38: 实现 Temporal Query（按时间范围精确重建状态）
       添加快照策略（session_metadata 增强）

v1.0:  PostgreSQL 生产 Schema（含 pgaudit、哈希链）
       跨会话上下文关联（engine_session_id 链路追踪）
```

---

## 参考来源

- Martin Fowler, *Event Sourcing*, https://martinfowler.com/eaaDev/EventSourcing.html
- Martin Fowler, *CQRS*, https://martinfowler.com/patterns/cqrs/
- Alexey Zimarev (Kurrent), *What is Event Sourcing*, https://www.kurrent.io/blog/what-is-event-sourcing
- Gregory Young, *Versioning in an Event Sourced System* (Leanpub)
- SQLite, *Write-Ahead Logging Mode*, https://www.sqlite.org/wal.html
- OpenTelemetry, *Trace Specification*, https://opentelemetry.io/docs/specs/otel/trace/api/
- Confluent, *Event Sourcing & Event Streaming with Kafka*, https://www.confluent.io/blog/event-sourcing-event-streaming-with-kafka/
- Uber Go Style Guide（项目内规范，`.agent/rules/uber-go-style-guide.md`）
