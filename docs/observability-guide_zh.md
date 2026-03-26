*Read this in other languages: [English](observability-guide.md), [简体中文](observability-guide_zh.md).*

# 可观测性指南

## 概述

HotPlex 通过 OpenTelemetry 链路追踪、Prometheus 指标和健康检查提供全面的可观测性。

## OpenTelemetry 链路追踪

### 配置

```bash
export OTEL_ENDPOINT="localhost:4317"
export OTEL_SERVICE_NAME="hotplex"
export OTEL_SAMPLING_RATE="1.0"
```

### Spans (追踪跨度)

| Span 名称               | 描述         |
| ----------------------- | ------------ |
| `session.execute`       | 完整会话执行 |
| `tool.use`              | 工具调用     |
| `security.danger_block` | WAF 拦截事件 |

### 属性

| 属性               | 描述         |
| ------------------ | ------------ |
| `session.id`       | 会话标识符   |
| `namespace`        | 命名空间     |
| `tool.name`        | 工具名称     |
| `tool.id`          | 工具调用 ID  |
| `danger.operation` | 被拦截的操作 |

## Prometheus 指标

### 接口地址

```
GET /metrics
```

### 指标项

| 指标                               | 类型      | 描述             |
| ---------------------------------- | --------- | ---------------- |
| `hotplex_sessions_active`          | gauge     | 活动中的会话数   |
| `hotplex_sessions_total`           | counter   | 已创建的会话总数 |
| `hotplex_sessions_errors`          | counter   | 会话错误总数     |
| `hotplex_tools_invoked`            | counter   | 工具调用次数     |
| `hotplex_dangers_blocked`          | counter   | WAF 拦截次数     |
| `hotplex_request_duration_seconds` | histogram | 请求延迟分布     |

### Grafana 仪表盘

从 `docs/grafana-dashboard.json` 导入仪表盘。

## Token 与上下文监控

HotPlex 会动态追踪受支持 Provider（如 Claude Code）的 Token 消耗和上下文窗口使用情况。

### 实时使用统计
在会话期间，可以通过 `session_stats` 事件和遥测数据获取以下指标：
- **输入 Token (Input Tokens)**: 发送到模型的 Token 数（不含缓存命中部分）。
- **缓存读写 (Cache Read/Write)**: 提示词缓存效率（命中部分可享受 90% 折扣）。
- **上下文占用百分比 (Context Percentage)**: 模型上下文窗口（如 200K 或 1M）的实时占用情况。

### 计算公式
```text
占用率 % = (输入 + 缓存读 + 缓存写) / 上下文窗口总量 * 100
```

## 健康检查

### 接口地址

```
GET /health       # 基础健康检查
GET /health/ready # 就绪探针
GET /health/live  # 存活探针
```

### 响应示例

```json
{
  "status": "healthy",
  "checks": {
    "engine": true,
    "pool": true
  }
}
```

## 日志记录

### 结构化日志

```json
{"level":"info","msg":"session started","session_id":"abc123","namespace":"default"}
```

### 日志级别

| 级别  | 使用场景     |
| ----- | ------------ |
| debug | 详细调试信息 |
| info  | 正常运行信息 |
| warn  | 可恢复的错误 |
| error | 执行失败错误 |
