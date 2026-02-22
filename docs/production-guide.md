# Production Deployment Guide

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Load Balancer                           │
│                  (nginx / cloud LB)                         │
└─────────────────────────────────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ HotPlex  │       │ HotPlex  │       │ HotPlex  │
   │  Node 1  │       │  Node 2  │       │  Node 3  │
   └──────────┘       └──────────┘       └──────────┘
         │                  │                  │
         └──────────────────┴──────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐       ┌──────────┐       ┌──────────┐
   │ Prometheus│       │  Jaeger  │       │  Loki    │
   │ (metrics)│       │ (traces) │       │  (logs)  │
   └──────────┘       └──────────┘       └──────────┘
```

## Scaling Guidelines

| Concurrent Users | Instances | CPU/Instance | Memory/Instance |
|------------------|-----------|--------------|-----------------|
| 1-100 | 1 | 0.5 core | 512MB |
| 100-500 | 2-3 | 1 core | 1GB |
| 500-2000 | 5-10 | 2 core | 2GB |
| 2000+ | 10+ | 2-4 core | 2-4GB |

## Monitoring Stack

### Prometheus

```yaml
scrape_configs:
  - job_name: 'hotplex'
    static_configs:
      - targets: ['hotplex:8080']
    metrics_path: /metrics
```

### Grafana Dashboard

Key panels:
- Active Sessions
- Request Latency (p50, p95, p99)
- Error Rate
- Tool Invocation Rate

### Alerting Rules

```yaml
groups:
- name: hotplex
  rules:
  - alert: HighErrorRate
    expr: rate(hotplex_sessions_errors[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: High error rate detected

  - alert: SessionPoolExhausted
    expr: hotplex_sessions_active > 800
    for: 2m
    labels:
      severity: critical
```

## Security Checklist

- [ ] Enable TLS termination at LB
- [ ] Set up network policies
- [ ] Configure rate limiting
- [ ] Enable authentication
- [ ] Set resource limits
- [ ] Enable audit logging

## Backup & Recovery

### Session State

Sessions are ephemeral. No persistent state to backup.

### Configuration

```bash
kubectl get configmap hotplex-config -o yaml > hotplex-config-backup.yaml
```

## Troubleshooting

### High Memory Usage

```bash
kubectl exec -it hotplex-xxx -- curl localhost:8080/debug/pprof/heap
```

### Slow Requests

Check traces in Jaeger for bottleneck spans.

### Session Leaks

```bash
curl http://hotplex:8080/metrics | grep hotplex_sessions_active
```
