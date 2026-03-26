*Read this in other languages: [English](observability-guide.md), [简体中文](observability-guide_zh.md).*

# Observability Guide

## Overview

HotPlex provides comprehensive observability through OpenTelemetry tracing, Prometheus metrics, and health checks.

## OpenTelemetry Tracing

### Configuration

```bash
export OTEL_ENDPOINT="localhost:4317"
export OTEL_SERVICE_NAME="hotplex"
export OTEL_SAMPLING_RATE="1.0"
```

### Spans

| Span Name               | Description            |
| ----------------------- | ---------------------- |
| `session.execute`       | Full session execution |
| `tool.use`              | Tool invocation        |
| `security.danger_block` | WAF block event        |

### Attributes

| Attribute          | Description        |
| ------------------ | ------------------ |
| `session.id`       | Session identifier |
| `namespace`        | Namespace          |
| `tool.name`        | Tool name          |
| `tool.id`          | Tool invocation ID |
| `danger.operation` | Blocked operation  |

## Prometheus Metrics

### Endpoints

```
GET /metrics
```

### Metrics

| Metric                             | Type      | Description            |
| ---------------------------------- | --------- | ---------------------- |
| `hotplex_sessions_active`          | gauge     | Active sessions        |
| `hotplex_sessions_total`           | counter   | Total sessions created |
| `hotplex_sessions_errors`          | counter   | Session errors         |
| `hotplex_tools_invoked`            | counter   | Tool invocations       |
| `hotplex_dangers_blocked`          | counter   | WAF blocks             |
| `hotplex_request_duration_seconds` | histogram | Request latency        |

### Grafana Dashboard

Import the dashboard from `docs/grafana-dashboard.json`.

## Token & Context Monitoring

HotPlex dynamically tracks token consumption and context window utilization for supported providers (e.g., Claude Code).

### Real-time Usage Stats
During a session, the following metrics are available via the `session_stats` event and telemetry:
- **Input Tokens**: Tokens sent to the model (excluding cache hits).
- **Cache Read/Write**: Prompt caching efficiency (90% discount for hits).
- **Context Percentage**: Real-time utilization of the model's context window (e.g., 200K or 1M).

### Calculation Formula
```text
Usage % = (input + cache_read + cache_write) / context_window * 100
```

## Health Checks

### Endpoints

```
GET /health       # Basic health
GET /health/ready # Readiness probe
GET /health/live  # Liveness probe
```

### Response

```json
{
  "status": "healthy",
  "checks": {
    "engine": true,
    "pool": true
  }
}
```

## Logging

### Structured Logging

```json
{"level":"info","msg":"session started","session_id":"abc123","namespace":"default"}
```

### Log Levels

| Level | Use Case           |
| ----- | ------------------ |
| debug | Detailed debugging |
| info  | Normal operations  |
| warn  | Recoverable errors |
| error | Failures           |
