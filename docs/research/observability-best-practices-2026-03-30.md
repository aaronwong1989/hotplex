# 可观测性 — 最佳实践调研

> 调研日期：2026-03-30
> 调研范围：结构化日志、Prometheus Metrics、分布式追踪、告警与 SLO

---

## 1. 结构化日志规范

### 1.1 核心原则：Schema First

OpenTelemetry 明确指出：**生产环境应使用结构化日志**，核心价值在于稳定的 schema，使下游系统能够可靠地解析、验证、关联 traces/metrics。

```
# 结构化 vs 非结构化
非结构化: "User login failed for user123 at 2026-03-30"
结构化:  {"user":"user123","action":"login","status":"failed","ts":"2026-03-30T..."}
```

格式（JSON / protobuf）本身是次要的，**稳定的字段名、类型、语义**才是关键。

### 1.2 日志级别规范

| 级别 | SeverityNumber | 场景 |
|------|---------------|------|
| FATAL / CRITICAL | 21-22 | 进程无法继续（不应在业务代码中使用）|
| ERROR | 17 | 操作失败，需要关注 |
| WARN | 13 | 潜在问题，非预期但可恢复 |
| INFO | 9 | 正常业务里程碑（启动、关闭、重大操作）|
| DEBUG | 5 | 开发调试，详细执行路径 |
| TRACE | 1 | 最细粒度（高并发场景谨慎使用）|

**HotPlex 实践建议**：
- ERROR: WAF 拦截、Session 失败、PGID 清理失败
- WARN: 配置缺失使用默认值、连接池耗尽前兆
- INFO: 会话创建/销毁、CLI 进程启动/退出、关键路由决策
- DEBUG: I/O 管道数据片段（仅在诊断模式启用）

### 1.3 字段命名与 OTel 日志数据模型

OpenTelemetry 定义的标准字段（hot-level log record fields）：

| 字段 | 说明 | HotPlex 示例 |
|------|------|-------------|
| `timestamp` | 日志产生时间 | - |
| `observed_timestamp` | 日志被记录的时间（多组件场景）| - |
| `trace_id` / `span_id` | 关联分布式追踪 | `trace_id: "abc123..."` |
| `trace_flags` | W3C 格式，sampled 位 | - |
| `body` | 主消息文本 | `"Session 123 terminated"` |
| `resource` | 日志来源元信息 | `service.name=hotplex` |
| `instrumentation_scope` | 发出组件 | `provider/opencode` |
| `attributes` | 业务上下文键值对 | `session_id`, `user_id` |

**Resource 语义约定**（SemConv）：

```go
// 推荐 Resource attributes
resource.Attributes(
    semconv.ServiceName("hotplex"),
    semconv.ServiceVersion("0.36.0"),
    semconv.HostName(hostname),
    semconv.DeploymentEnvironment(env),
)
```

### 1.4 日志采样策略

| 策略 | 适用场景 | HotPlex 推荐 |
|------|---------|------------|
| **确定性采样**（every Nth）| 规则简单，低开销 | DEBUG 日志每 10 条采样 1 条 |
| **速率采样**（probabilistic）| 通用降量 | 生产环境 INFO+ 保留 100% |
| **基于等级的采样** | 错误日志全量 | ERROR/WARN 全量，DEBUG 按比例 |
| **自适应采样** | 流量波动大 | 高并发时动态调整采样率 |

**推荐实现**：基于日志级别的混合采样 — ERROR/WARN 全量，INFO 降采样，DEBUG 仅在诊断时启用。

### 1.5 LogQL 查询最佳实践（Grafana Loki）

LogQL 是 Loki 的查询语言，语法源自 PromQL：

```logql
# 基础结构
{ log stream selector } | log pipeline

# 示例：查询特定 Session 的错误日志
{service_name="hotplex", session_id="abc123"} |= "ERROR" | json
  | line_format "{{.timestamp}} [{{.level}}] {{.message}}"

# 统计每秒错误率（LogQL 指标查询）
sum by (service_name) (
  rate({service_name="hotplex"} |= "ERROR" [1m])
)

# 结构化解析
{service_name="hotplex"} | json | status_code >= 500
```

**最佳实践**：
- 先用 label selector 缩小范围，再做 line filter
- 优先使用 JSON/logfmt 结构化格式，解析效率更高
- `line_format` 仅改变展示，不改变存储数据

---

## 2. Prometheus Metrics 设计

### 2.1 四大指标类型对比

| 类型 | 行为 | 典型场景 | HotPlex 示例 |
|------|------|---------|-------------|
| **Counter** | 只增，重启归零 | 请求总数、错误总数、完成数 | `hotplex_sessions_total`, `hotplex_errors_total` |
| **Gauge** | 可增可减 | 当前值、瞬时状态 | `hotplex_active_sessions`, `hotplex_cpu_usage` |
| **Histogram** | 桶计数，服务端聚合 | 请求延迟、响应大小 | `hotplex_request_duration_seconds{le="0.1"}` |
| **Summary** | 客户端计算分位数 | 需要精确分位数，无法聚合 | 高精度 P99（跨实例不可聚合时）|

**核心决策**：需要跨实例聚合用 Histogram；需要客户端精确分位数用 Summary。

### 2.2 RED 方法（Rate / Errors / Duration）

适用于**无状态服务/微服务**，关注请求链路：

```
Rate:     sum(rate(hotplex_http_requests_total[5m]))
Errors:   sum(rate(hotplex_http_requests_total{status=~"5.."}[5m]))
Duration: histogram_quantile(0.99, sum(rate(hotplex_request_duration_seconds_bucket[5m])) by (le))
```

### 2.3 USE 方法（Utilization / Saturation / Errors）

适用于**基础设施资源**（CPU、Memory、Disk、Network）：

```
Utilization: hotplex_process_cpu_seconds_total
Saturation:  hotplex_session_pool_available / hotplex_session_pool_capacity
Errors:      rate(hotplex_waf_blocks_total[5m])
```

### 2.4 指标命名规范（Prometheus 官方）

```
<metric_name>{<labels>} <value>

命名规范（官方优先级）：
1. 应用前缀（单个词）: hotplex_
2. 逻辑组（可选多级）: hotplex_session_
3. 度量名称: total / created / seconds / bytes
4. 单位后缀（复数）: _seconds, _bytes, _total

禁止：
- 混合单位：_seconds 和 _milliseconds 不能共存
- 标签嵌入名称：不用 operation_create + operation_delete
- 高基数标签：user_id、email、session_id（临时诊断用标签即可）
```

**HotPlex 指标命名示例**：

```go
// Counter: 会话总数
hotplex_sessions_total{provider="opencode", reason="completed"}

// Gauge: 活跃会话数
hotplex_active_sessions{provider="opencode"}

// Histogram: 请求延迟
hotplex_request_duration_seconds{provider="opencode", operation="execute"}
// Bucket: _bucket{le="0.005"}/_bucket{le="0.01"}/_bucket{le="0.05"}/_bucket{le="0.1"}/_bucket{le="0.5"}/_bucket{le="+Inf"}
// _sum, _count 自动生成

// Counter: WAF 拦截
hotplex_waf_blocks_total{rule="dangerous_command", provider="opencode"}
```

### 2.5 Multi-tenant Metrics 隔离

| 方案 | 优点 | 缺点 | 推荐场景 |
|------|------|------|---------|
| **标签隔离** | 实现简单，查询灵活 | 高基数风险，数据分离依赖查询层 | 小规模多租户 |
| **Metrics federation** | 物理隔离 | 聚合复杂 | 大规模/合规要求高 |
| **Per-tenant PushGateway** | 完全隔离 | 运维复杂 | 高隔离要求 |
| **relabel_configs** | 无代码改造，灵活 | 需要 Prometheus 配置 | 推荐作为首选 |

**HotPlex 推荐**：标签隔离 + `__tenant_id__` label，通过 Prometheus `relabel_configs` 注入，查询时按 tenant 过滤。

---

## 3. 分布式追踪方案

### 3.1 OpenTelemetry 标准架构

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│ Application │────▶│ OTel Collector   │────▶│ Backend     │
│ (OTel SDK)  │     │ (Processor/Export)│     │ (Jaeger/    │
└─────────────┘     └──────────────────┘     │  Tempo)     │
      │                                         └─────────────┘
      │  trace context (W3C TraceContext) ──────────────▶
```

### 3.2 追踪语义约定

**Trace**：完整请求路径，跨服务关联所有操作。

**Span**：最小工作单元，每个 Span 包含：

| 字段 | 说明 |
|------|------|
| `name` | 操作名称（低基数：HTTP 方法 + 路径模板）|
| `span_id` | 当前 Span 唯一标识（16 hex）|
| `parent_span_id` | 上游 Span（根 Span 无父）|
| `trace_id` | 全局请求唯一标识（32 hex）|
| `start_time` / `end_time` | 时间范围 |
| `attributes` | 业务元数据 |
| `events` | 时间点事件（异常日志等）|
| `status` | OK / ERROR |

**HotPlex Span 设计**：

```go
// 根 Span: WebSocket 连接
{span_name: "ws_connection", attributes: {session_id, user_id}}

// 子 Span 1: 会话管理
{span_name: "session_create", parent: ws_connection.span_id}
{span_name: "session_execute", parent: ws_connection.span_id}
{span_name: "session_terminate", parent: ws_connection.span_id}

// 子 Span 2: CLI 进程
{span_name: "cli_process_spawn", parent: session_execute.span_id}
{span_name: "cli_io_multiplex", parent: session_execute.span_id}

// 事件：关键节点
span.AddEvent("waf_check_passed", attributes: {rule="safe_input"})
span.AddEvent("cli_process_exited", attributes: {exit_code: 0})
span.RecordError(err)  // 自动关联到 span status=ERROR
```

### 3.3 Trace Context 传播（W3C TraceContext）

两个 HTTP Header 完成上下文传播：

```
traceparent: <version>-<trace-id>-<parent-id>-<trace-flags>
  例: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
       ↑   ↑                                ↑                        ↑
     版本  32位trace_id                   16位span_id             sampled=1

tracestate: congo=t61rcWkgMzE,rojo=00f067aa0ba902b7
  携带供应商特定信息（如 Cloud Trace、Jaeger 的 extra data）
```

**关键约定**：传播时必须保留 `traceparent`，可选 `tracestate`。W3C 标准不定义 Baggage，Baggage 是 OTel 独立扩展。

### 3.4 Baggage 传播

Baggage 是随 trace 上下文跨服务传播的键值对：

```go
// 设置 Baggage
propagation := baggage.New()
ctx := baggage.ContextWithBaggage(ctx, baggage.Set("tenant_id", "org-123"))

// 在任何后续 Span 中可通过 attributes 访问
// 需要 W3C Baggage 传播头: baggage-<key>: <value>
```

**HotPlex 适用场景**：在 WebSocket 握手时注入 `user_id`、`tenant_id`，后续所有 CLI 执行 Span 自动携带，无需层层透传。

### 3.5 采样策略对比

| 策略 | 决策时机 | 优点 | 缺点 | HotPlex 适用 |
|------|---------|------|------|------------|
| **Head-based（概率）** | Trace 创建时（只看 trace_id）| 低开销、易实现、全 trace 完整 | 无法针对错误/慢请求加权 | 开发/预发环境 |
| **Head-based（规则）** | 创建时按属性决策 | 可针对特定路径 | 需预知规则 | 固定高价值端点 |
| **Tail-based** | Trace 完成后（所有 spans 就绪）| 错误/Slow trace 全量保留，可跨 Span 决策 | 需要有状态基础设施，资源消耗大 | 生产环境（推荐）|

**HotPlex 推荐**：Head + Tail 混合

```
阶段1: Head sampling (100% 错误 traces)
  → 发现 status=ERROR，立即标记 sampled=1

阶段2: Tail sampling (复杂多条件)
  → 完整 trace 数据到达 Collector 后：
     - 保留所有含错误的 trace（100%）
     - 保留 P99 超阈值 trace（10%）
     - 均匀采样正常 trace（1%）
```

### 3.6 Jaeger vs Tempo vs Zipkin

| 特性 | Jaeger | Grafana Tempo | Zipkin |
|------|--------|--------------|--------|
| 存储后端 | Elasticsearch/Cassandra/SQL | Object storage (S3/GCS/Azure) | SQL/Elasticsearch |
| 成本模型 | 运维存储集群 | 对象存储 + 查询层 | 取决于存储后端 |
| OTel 兼容 | 原生支持 | 原生支持（推荐用于 OTel）| 原生支持 |
| Metrics 关联 | 通过 Trace ID 关联 | Grafana 生态无缝集成 | 需额外配置 |
| 适合规模 | 中小规模 | 超大规模（成本最优）| 小规模/简单场景 |

**HotPlex 推荐**：Grafana Tempo
- 与 Grafana 已有良好集成（HotPlex 监控面板）
- 对象存储成本低（对比 Elasticsearch）
- OTel 原生兼容，未来可平滑迁移

---

## 4. 告警与 SLO

### 4.1 四大黄金信号（Google SRE）

| 信号 | 测量方式 | HotPlex 关键指标 |
|------|---------|----------------|
| **Latency** | Histogram P50/P95/P99 | `hotplex_request_duration_seconds` |
| **Traffic** | Rate (RPS) | `rate(hotplex_sessions_total[5m])` |
| **Errors** | Error Rate % | `rate(hotplex_errors_total[5m]) / rate(hotplex_requests_total[5m])` |
| **Saturation** | 资源利用率 | `hotplex_session_pool_utilization` |

### 4.2 AlertManager 告警最佳实践

**核心原则：告警症状而非根因**

```
错误做法（基于原因）:
  alert: KafkaBrokerDown
  expr: kafka_broker_up == 0
  # 问题：开发者看到告警不知道该做什么

正确做法（基于症状）:
  alert: HighSessionFailureRate
  expr: |
    (
      sum(rate(hotplex_sessions_total{reason="failed"}[5m]))
      /
      sum(rate(hotplex_sessions_total[5m]))
    ) > 0.05
  labels:
    severity: critical
  annotations:
    summary: "Session 失败率超过 5%"
    runbook_url: "https://wiki.hotplex.io/runbooks/high-session-failure"
```

**告警分级**：

| 分级 | 响应时间 | 示例 |
|------|---------|------|
| P1 Critical | 5 分钟 | 所有 session 创建失败 |
| P2 High | 15 分钟 | WAF 误拦截率上升 |
| P3 Medium | 1 小时 | 延迟 P99 超过阈值 |
| P4 Low | 1 天 | 配置不一致告警 |

### 4.3 SLO / SLI 设计

**SLI（Service Level Indicator）**：可测量的指标
**SLO（Service Level Objective）**：目标值

```yaml
# OpenSLO 格式示例
apiVersion: openslo/v1
kind: SLO
metadata:
  name: hotplex-session-availability
spec:
  service: hotplex
  sli:
    thresholdMetric:
      source: prometheus
      queryType: promQL
      query: |
        sum(hotplex_sessions_total{status="success"})
        /
        sum(hotplex_sessions_total)
  objectives:
    - displayName: "99.5% Availability"
      target: 0.995
      timeSliceTarget: 0.995
      timeSliceWindow: 1m
  timeWindow:
    - duration: 30d
      isRolling: true
  errorBudgetPolicy: |
    // Burn rate alert: 1% budget consumed in 1h → P2
    //                    5% budget consumed in 1h → P1
```

**HotPlex 关键 SLO**：

| SLO 名称 | SLI | Target | 测量窗口 |
|---------|-----|--------|---------|
| 会话创建成功率 | `sessions_total{success} / sessions_total` | 99.5% | 30d rolling |
| 会话响应延迟 P99 | `histogram_quantile(0.99, ...)` | < 5s | 5m |
| WAF 准确率 | `1 - (误拦截数 / 总通过数)` | > 99.9% | 30d rolling |
| CLI 进程健康率 | `cli_processes{exit_code=0} / cli_processes_total` | 99% | 7d rolling |

### 4.4 OpenSLO 标准

OpenSLO（Declarative SLO Definition）核心结构：

```yaml
# 核心概念
# 1. SLI 定义：引用 Prometheus/Grafana 数据源
# 2. Objectives：目标值 + 时间窗口
# 3. Time Window：滚动窗口（rolling）或日历窗口（calendar）
# 4. Error Budget Policy：燃尽速率告警规则

# OpenSLO 工具链
# - sloth (Prometheus Native): 生成 SLO + Burn-rate alerts
# - Nobl9 (SaaS): 可视化 SLO + 多数据源
# - Grafana SLO: Grafana 原生 SLO 面板（推荐用于 HotPlex）
```

---

## 5. 推荐方案

### 5.1 HotPlex 可观测性架构推荐

```
┌──────────────────────────────────────────────────────────────┐
│                     HotPlex Process                           │
│                                                              │
│  ┌──────────┐   ┌──────────┐   ┌─────────────────────────┐  │
│  │ Zerolog  │   │ otel-sdk │   │ prometheus-client-go   │  │
│  │ (Logs)   │   │ (Traces) │   │ (Metrics)               │  │
│  └────┬─────┘   └────┬─────┘   └───────────┬─────────────┘  │
│       │               │                     │                │
│       ▼               ▼                     ▼                │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              OTLP Exporter (GRPC/HTTP)                   │ │
│  └────────────────────────┬────────────────────────────────┘ │
└───────────────────────────┼─────────────────────────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │   OTel Collector            │
              │  - Tail Sampling Processor  │
              │  - Batch Processor          │
              │  - Resource Detection       │
              └────────────┬────────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
    ┌─────────┐      ┌──────────┐      ┌─────────┐
    │ Grafana │      │ Grafana  │      │ Grafana │
    │ Loki    │      │ Tempo    │      │         │
    │ (Logs)  │      │ (Traces) │      │ Dash-   │
    └────┬────┘      └────┬─────┘      │ boards  │
         │                │             └────┬────┘
         └────────────────┴──────────────────┘
                            │
                     ┌──────┴──────┐
                     │ Prometheus  │
                     │ (Metrics)   │
                     └──────┬──────┘
                            │
                     ┌──────┴──────┐
                     │ AlertManager│
                     └─────────────┘
```

### 5.2 组件选型

| 组件 | 推荐方案 | 理由 |
|------|---------|------|
| **日志库** | `zerolog` / OTel SDK | zerolog 高性能；OTel SDK 统一遥测格式 |
| **日志后端** | Grafana Loki | 与 Grafana 无缝集成，存储成本低 |
| **Trace 后端** | Grafana Tempo | OTel 原生，对象存储成本低，Grafana 集成 |
| **Metrics** | Prometheus + Grafana | 业界标准，Grafana 原生支持 |
| **OTel Collector** | OTel Collector (DaemonSet/Sidecar) | 解耦应用与后端，支持 Tail Sampling |
| **SLO 工具** | Grafana SLO | 集成在 Grafana 中，支持多数据源 |
| **告警** | AlertManager + Grafana Alerting | 与指标体系一致 |

### 5.3 关键指标清单（HotPlex MVP）

**Session 生命周期指标**：

```go
// Counter: 按 reason 和 provider 区分
hotplex_sessions_total{provider, reason}
// reason: created | completed | failed | timeout | killed

// Gauge: 当前活跃会话
hotplex_active_sessions{provider, pool}

// Histogram: 会话执行时间
hotplex_session_duration_seconds{provider, operation}
```

**CLI 进程指标**：

```go
hotplex_cli_processes_total{provider, exit_code}
// exit_code: 0 (success) | 1-255 (error)

hotplex_cli_io_bytes_total{provider, direction}
// direction: stdin | stdout | stderr
```

**WAF 指标**：

```go
hotplex_waf_checks_total{result, rule_category}
// result: pass | block | error

hotplex_waf_block_duration_seconds{rule}  // WAF 决策耗时
```

**Engine 指标**：

```go
hotplex_engine_pool_capacity{provider}
hotplex_engine_pool_available{provider}
hotplex_engine_pool_utilization_ratio{provider}  // = available/capacity
```

**WebSocket 指标**：

```go
hotplex_ws_connections_total{status}
// status: connected | disconnected | error

hotplex_ws_message_bytes_total{direction}
// direction: received | sent
```

---

## 6. 关键决策点

### 决策 1：日志格式 — JSON + OTel Schema

**选择**：JSON 结构化日志，遵循 OTel Log Data Model 字段规范。

**理由**：
- OTel 语义约定保证跨组件一致解析
- 与 Grafana Loki LogQL 完美兼容（JSON parser）
- `zerolog` 输出 JSON 性能优于 text parsing

**行动项**：统一所有 provider adapter 输出格式，强制使用 OTel 字段名（如 `trace_id` 而非 `traceId`）。

### 决策 2：指标暴露 — Prometheus 拉取模式

**选择**：应用通过 `/metrics` 端点暴露，Prometheus Server 主动拉取。

**理由**：
- 解耦应用与监控系统（应用无需感知 Prometheus 地址）
- 支持多 Prometheus 实例（高可用/多集群）
- 与 Grafana 原生集成

**注意**：WebSocket 长连接场景使用 Gauge 记录活跃连接数，通过心跳定期刷新。

### 决策 3：追踪采样 — Head + Tail 混合

**选择**：Head sampling 优先丢弃正常 trace，Tail sampling 兜底保留错误/Slow trace。

**配置**：

```yaml
# OTel Collector tail_sampling processor
processors:
  tail_sampling:
    decision_wait: 10s
    num_traces: 50000
    policies:
      - name: errors-policy
        type: status_code
        status_code: {status_codes: [ERROR]}
      - name: slow-traces-policy
        type: latency
        latency: {threshold_ms: 5000}
      - name: probabilistic-policy
        type: probabilistic
        probabilistic: {sampling_percentage: 1}
```

### 决策 4：存储分层 — 热温冷分离

| 数据类型 | 热度 | 保留策略 | 存储层 |
|---------|------|---------|-------|
| Metrics | 热 | 30d（高分辨率）| Prometheus TSDB |
| Metrics | 温 | 13m（低分辨率）| Thanos/Mimir |
| Logs | 热 | 7d | Loki (SSD) |
| Logs | 冷 | 90d | Loki (GCS/S3) |
| Traces | 热 | 7d | Tempo (SSD) |
| Traces | 冷 | 30d | Tempo (GCS/S3) |

### 决策 5：SLO 工具 — Grafana SLO

**选择**：使用 Grafana 内置 SLO 功能。

**理由**：
- 无需引入额外系统（Grafana 已有）
- 支持 Prometheus/Grafana Mimir 作为数据源
- 内置 Burn-rate Alert 生成
- OpenSLO 格式导入支持

### 决策 6：OpenTelemetry SDK 集成策略

**推荐**：应用层使用 OTel SDK，统一埋点出口。

```
推荐架构：
  zerolog (JSON) ──→ OTel Log Bridge ──→ OTLP Exporter
  OTel SDK (Tracer) ──→ OTLP Exporter
  prometheus-client-go ──→ /metrics ──→ Prometheus

不推荐：
  - 每个 provider adapter 独立日志库
  - 自行实现 trace propagation
```

**理由**：OTel 是云原生可观测性的行业标准，确保 vendor-neutral，未来可切换后端而不改应用代码。

### 决策 7：多租户隔离 — Label 标记

**选择**：所有指标/日志/traces 携带 `tenant_id` label，Prometheus relabel 做物理隔离。

**理由**：
- 实现成本低，无需 per-tenant 存储
- Grafana Cloud / Managed Prometheus 天然支持 per-tenant 查询
- HotPlex 作为 Cli-as-a-Service，多租户隔离是核心需求

---

## 附录：参考文档

- [OpenTelemetry Logs Specification](https://opentelemetry.io/docs/concepts/signals/logs/)
- [OpenTelemetry Sampling](https://opentelemetry.io/docs/concepts/sampling/)
- [Prometheus Metric Types](https://prometheus.io/docs/concepts/metric_types/)
- [Prometheus Naming Conventions](https://prometheus.io/docs/practices/naming/)
- [Prometheus Histograms](https://prometheus.io/docs/practices/histograms/)
- [Grafana Observability Best Practices](https://grafana.com/docs/grafana/latest/best-practices/)
- [Grafana Loki LogQL](https://grafana.com/docs/loki/latest/query/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [OpenSLO Specification](https://openslo.com/)
