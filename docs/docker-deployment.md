*Read this in other languages: [English](docker-deployment.md), [简体中文](docker-deployment_zh.md).*

# Docker Deployment Guide

## Quick Start

### 1. Build the Image

```bash
docker build -t hotplex:latest .
```

### 2. Run the Container

```bash
docker run -d \
  --name hotplex \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e CLAUDE_API_KEY=your-key \
  hotplex:latest
```

## Docker Compose

```yaml
version: '3.8'
services:
  hotplex:
    image: hotplex:latest
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - hotplex-data:/data
    environment:
      - HOTPLEX_PORT=8080
      - HOTPLEX_LOG_LEVEL=info
      - HOTPLEX_IDLE_TIMEOUT=30m
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    restart: unless-stopped

volumes:
  hotplex-data:
```

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hotplex
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hotplex
  template:
    metadata:
      labels:
        app: hotplex
    spec:
      containers:
      - name: hotplex
        image: hotplex:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: hotplex
spec:
  selector:
    app: hotplex
  ports:
  - port: 80
    targetPort: 8080
```

## Configuration

| Variable      | Default | Description             |
| ------------- | ------- | ----------------------- |
| HOTPLEX_PORT          | 8080    | Server port             |
| HOTPLEX_LOG_LEVEL     | info    | Log level               |
| HOTPLEX_IDLE_TIMEOUT  | 30m     | Session idle timeout    |
| OTEL_ENDPOINT | -       | OpenTelemetry endpoint  |
| MAX_SESSIONS  | 1000    | Max concurrent sessions |
