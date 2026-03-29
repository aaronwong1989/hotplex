# HotPlex v1.0.0 配置设计

> 版本：v1.0  
> 日期：2026-03-29  
> 状态：完整配置格式定义

---

## 1. 配置概述

### 1.1 设计原则

- **YAML 优先**：主要配置文件使用 YAML 格式
- **环境变量覆盖**：支持 `HOTPLEX_xxx` 环境变量覆盖配置
- **渐进式配置**：只有必填字段需要配置，其他使用默认值
- **验证友好**：配置加载时进行 schema 校验

### 1.2 配置加载顺序

```
1. 默认配置 (内置)
2. 配置文件 (hotplex.yaml)
3. 环境变量 (HOTPLEX_xxx)
4. 命令行 flags (最高优先级)
```

---

## 2. 完整配置格式

### 2.1 主配置文件

```yaml
# hotplex.yaml

# ============================================
# HotPlex 配置 (v1.0.0)
# ============================================

hotplex:
  # 服务器配置
  server:
    host: "0.0.0.0"
    port: 8080
    mode: "release"                    # "debug" | "release" | "profile"
    read_timeout: 60s
    write_timeout: 300s               # 流式输出需要较长超时
    idle_timeout: 120s
    max_header_bytes: 1048576         # 1MB
    
    # TLS 配置
    tls:
      enabled: false
      cert_file: ""
      key_file: ""
    
    # CORS 配置
    cors:
      enabled: true
      allowed_origins: ["*"]
      allowed_methods: ["GET", "POST", "OPTIONS"]
      allowed_headers: ["*"]
    
    # 速率限制
    rate_limit:
      enabled: true
      requests_per_second: 100
      burst: 200
    
    # 链路追踪
    tracing:
      enabled: true
      endpoint: "http://localhost:14268"
      sample_rate: 0.1

  # Channel 配置 (多渠道)
  channels:
    # 飞书配置
    feishu:
      enabled: true
      debug: false
      
      # 认证
      app_id: "${FEISHU_APP_ID}"
      app_secret: "${FEISHU_APP_SECRET}"
      
      # 机器人配置
      bot:
        name: "HotPlex"
        avatar: ""
      
      # 事件配置
      events:
        verification_token: "${FEISHU_VERIFICATION_TOKEN}"
        encrypt_key: "${FEISHU_ENCRYPT_KEY}"
        types:
          - "im.message.receive_v1"
      
      # 消息配置
      messages:
        max_length: 4000
        support_md: true
        streaming: true
      
      # 限流配置
      rate_limit:
        enabled: true
        max_requests_per_minute: 60
      
      # WAF 配置
      waf:
        enabled: true
        block_patterns:
          - "^/admin"
        warn_patterns:
          - "^/debug"
      
      # 回调配置
      callbacks:
        message_url: "https://your-domain.com/callback/feishu"
        event_url: "https://your-domain.com/callback/feishu/event"

    # Slack 配置
    slack:
      enabled: false
      debug: false
      
      # OAuth
      client_id: "${SLACK_CLIENT_ID}"
      client_secret: "${SLACK_CLIENT_SECRET}"
      signing_secret: "${SLACK_SIGNING_SECRET}"
      
      # Bot Token (xoxb-xxx)
      bot_token: "${SLACK_BOT_TOKEN}"
      
      # App Level Token (xapp-xxx)
      app_level_token: "${SLACK_APP_LEVEL_TOKEN}"
      
      # Socket Mode
      socket_mode:
        enabled: true
        auto_reconnect: true
        ping_interval: 30s
      
      # 消息配置
      messages:
        max_length: 4000
        support_md: true
        streaming: true
        response_timeout: 300s
      
      # Block Kit
      blocks:
        enabled: true
        max_columns: 5
      
      # WAF
      waf:
        enabled: true
        block_patterns: []
        warn_patterns: []

    # WebSocket 配置
    ws:
      enabled: false
      debug: false
      
      # 地址
      address: "0.0.0.0:8081"
      
      # 认证
      auth:
        type: "token"                 # "token" | "jwt" | "none"
        token: "${WS_AUTH_TOKEN}"
        jwt_secret: "${WS_JWT_SECRET}"
      
      # 消息配置
      messages:
        max_length: 10000
        ping_interval: 30s
        pong_timeout: 10s
        read_buffer_size: 4096
        write_buffer_size: 4096
      
      # 流式
      streaming:
        enabled: true
        chunk_size: 1024

  # Worker 配置
  workers:
    # Claude Code Worker
    claude_code:
      enabled: true
      debug: false
      
      # CLI 配置
      cli:
        path: "/usr/local/bin/claude-code"
        args: ["--print"]
      
      # 并发控制
      max_concurrent: 3
      max_queue_size: 10
      
      # 执行配置
      execution:
        timeout: 300s
        idle_timeout: 60s
        max_retries: 2
        
        # 重试配置
        retry:
          initial_delay: 1s
          max_delay: 30s
          backoff_multiplier: 2.0
          retryable_errors:
            - "TIMEOUT"
            - "RATE_LIMIT"
      
      # 进程配置
      process:
        work_dir: "/tmp/hotplex"
        env:
          PATH: "/usr/local/bin:/usr/bin:/bin"
        memory_limit_bytes: 1073741824    # 1GB
        cpu_limit_percent: 80
        read_only_fs: true
        allowed_dirs:
          - "/tmp/hotplex"
          - "/home/node"
      
      # 模型配置
      model:
        default: "claude-sonnet-4-20250514"
        allowed:
          - "claude-sonnet-4-20250514"
          - "claude-opus-4-20250514"
      
      # 工具配置
      tools:
        - name: "Read"
          enabled: true
        - name: "Write"
          enabled: true
        - name: "Edit"
          enabled: true
        - name: "Bash"
          enabled: true
        - name: "Grep"
          enabled: true
        - name: "Glob"
          enabled: true
        - name: "WebSearch"
          enabled: false
        - name: "WebFetch"
          enabled: false

    # OpenCode Worker
    open_code:
      enabled: false
      debug: false
      
      cli:
        path: "/usr/local/bin/opencode"
        args: ["--print"]
      
      max_concurrent: 2
      max_queue_size: 5
      
      execution:
        timeout: 300s
        idle_timeout: 60s
        max_retries: 2
      
      process:
        work_dir: "/tmp/hotplex-opencode"
        env:
          PATH: "/usr/local/bin:/usr/bin:/bin"
        memory_limit_bytes: 536870912    # 512MB
        cpu_limit_percent: 80
        read_only_fs: true
        allowed_dirs:
          - "/tmp/hotplex-opencode"
      
      model:
        default: "gpt-4o"
        allowed:
          - "gpt-4o"
          - "gpt-4-turbo"

  # Brain 配置
  brain:
    # 默认 Brain
    default: "llm"                    # "llm" | "rule" | "keyword"
    
    # LLM Brain
    llm:
      enabled: true
      
      # Provider
      provider: "anthropic"           # "anthropic" | "openai" | "siliconflow"
      
      # 模型
      model: "claude-sonnet-4-20250514"
      
      # 提示词
      prompts:
        intent_classification: |
          你是一个意图分类器。
          用户消息: {input}
          
          分类选项:
          - code_gen: 代码生成/修改
          - chat: 问答对话
          - admin: 管理命令
          - cron: 定时任务
          - system: 系统命令
          - unknown: 未知
          
          返回 JSON 格式: {"intent": "...", "confidence": 0.9, "params": {}}
      
      # 上下文
      context:
        max_history_messages: 10
        max_history_tokens: 4000
        summarize_threshold: 0.8
      
      # WAF
      waf:
        enabled: true
        level: "strict"               # "strict" | "moderate" | "permissive"
        rules:
          - id: "profanity"
            enabled: true
            level: "block"
          - id: "pattern_block"
            enabled: true
            level: "block"
            patterns:
              - "^/admin.*"
          - id: "pattern_warn"
            enabled: true
            level: "warn"
            patterns:
              - "^/debug.*"
      
      # 超时
      timeout: 10s
    
    # Rule Brain (备选)
    rule:
      enabled: true
      
      # 意图规则
      intent_rules:
        - pattern: "^生成代码|^写代码|^代码"
          intent: "code_gen"
        - pattern: "^解释|^什么是|^怎么"
          intent: "chat"
        - pattern: "^/admin"
          intent: "admin"
        - pattern: "^定时|^cron"
          intent: "cron"
      
      # WAF 规则
      waf_rules:
        - id: "admin_block"
          level: "block"
          patterns:
            - "^/admin"
        - id: "dangerous_cmds"
          level: "block"
          patterns:
            - "rm\\s+-rf"
            - "drop\\s+database"
      
      # 路由规则
      route_rules:
        - intent: "code_gen"
          worker: "claude-code"
        - intent: "chat"
          worker: "open-code"
        - intent: "admin"
          worker: "builtin"
    
    # Keyword Brain (快速路由)
    keyword:
      enabled: false
      
      keywords:
        code:
          - "代码"
          - "生成"
          - "write"
          - "code"
          worker: "claude-code"
        chat:
          - "?"
          - "怎么"
          - "什么"
          worker: "open-code"

  # Storage 配置
  storage:
    # 默认存储
    default: "sqlite"                 # "sqlite" | "postgres" | "redis"
    
    # SQLite
    sqlite:
      enabled: true
      path: "/var/hotplex/messages.db"
      journal_mode: "WAL"
      synchronous: "NORMAL"
      cache_size: 10000
      busy_timeout: 5000
    
    # PostgreSQL
    postgres:
      enabled: false
      dsn: "${POSTGRES_DSN}"
      
      # 连接池
      pool:
        max_conns: 20
        min_conns: 5
        max_conn_lifetime: 1h
        max_conn_idle_time: 30m
      
      # Schema
      schema: "hotplex"
      
      # TLS
      ssl:
        enabled: true
        mode: "require"              # "disable" | "require" | "verify-ca" | "verify-full"
        cert_file: ""
        key_file: ""
        ca_file: ""
    
    # Redis
    redis:
      enabled: false
      addr: "${REDIS_ADDR}"
      password: "${REDIS_PASSWORD}"
      db: 0
      
      # 连接池
      pool:
        max_idle: 10
        max_active: 100
        min_idle: 5
      
      # TTL
      ttl: 24h
    
    # 搜索配置
    search:
      enabled: true
      fts_enabled: true
      max_results: 100

  # Session 配置
  session:
    # 存储
    storage: "memory"                 # "memory" | "sqlite" | "postgres"
    
    # TTL
    ttl: 24h
    max_idle: 30m
    
    # 限制
    max_per_user: 5
    max_total: 1000
    
    # 历史
    max_history_messages: 100
    max_history_bytes: 1048576        # 1MB
    
    # 清理
    cleanup:
      enabled: true
      interval: 1h
      batch_size: 100

  # Supervisor 配置
  supervisor:
    # 策略
    restart_policy:
      mode: "backoff"                # "never" | "on-failure" | "always" | "backoff"
      max_restarts: 5
      max_restart_interval: 60s
      initial_interval: 1s
    
    # 健康检查
    health_check:
      enabled: true
      interval: 30s
      timeout: 10s
    
    # 事件
    events:
      enabled: true
      channel_size: 100

  # Provider 配置
  providers:
    # Anthropic
    anthropic:
      enabled: true
      api_key: "${ANTHROPIC_API_KEY}"
      base_url: "https://api.anthropic.com"
      timeout: 60s
      
      # 模型
      models:
        - id: "claude-sonnet-4-20250514"
          max_tokens: 8192
          context_window: 200000
        - id: "claude-opus-4-20250514"
          max_tokens: 8192
          context_window: 200000
    
    # OpenAI
    openai:
      enabled: true
      api_key: "${OPENAI_API_KEY}"
      base_url: "https://api.openai.com"
      organization: "${OPENAI_ORG_ID}"
      timeout: 60s
      
      models:
        - id: "gpt-4o"
          max_tokens: 4096
          context_window: 128000
        - id: "gpt-4-turbo"
          max_tokens: 4096
          context_window: 128000
    
    # SiliconFlow (国内模型路由)
    siliconflow:
      enabled: false
      api_key: "${SILICONFLOW_API_KEY}"
      base_url: "https://api.siliconflow.cn"
      timeout: 60s
      
      models:
        - id: "deepseek-ai/DeepSeek-V3"
          max_tokens: 4096
          context_window: 64000

  # 插件配置
  plugins:
    enabled: true
    dir: "/etc/hotplex/plugins"
    
    # 内置插件
    builtins:
      - name: "feishu"
        enabled: true
      - name: "slack"
        enabled: false
      - name: "claude-code"
        enabled: true
      - name: "open-code"
        enabled: false
      - name: "sqlite"
        enabled: true
    
    # 外部插件
    external: []

  # 日志配置
  logging:
    level: "info"                    # "debug" | "info" | "warn" | "error"
    format: "json"                   # "json" | "text"
    
    # 输出
    outputs:
      - type: "stdout"
        level: "info"
      - type: "file"
        level: "debug"
        path: "/var/log/hotplex/hotplex.log"
        max_size_mb: 100
        max_backups: 10
        max_age: 30
        compress: true
    
    # 特定模块日志级别
    modules:
      "channel.feishu": "info"
      "channel.slack": "info"
      "worker": "debug"
      "brain": "debug"
      "storage": "info"

  # 监控配置
  observability:
    # 指标
    metrics:
      enabled: true
      port: 9090
      path: "/metrics"
    
    # 健康检查
    health:
      enabled: true
      port: 8080
      path: "/health"
      checks:
        - name: "storage"
          timeout: 5s
        - name: "workers"
          timeout: 10s
    
    # Profiling
    profiling:
      enabled: false
      port: 6060
    
    # 告警
    alerts:
      enabled: false
      webhook: "${ALERT_WEBHOOK}"

  # 安全配置
  security:
    # CORS
    cors:
      enabled: true
      allowed_origins:
        - "https://your-domain.com"
      allowed_methods:
        - "GET"
        - "POST"
        - "OPTIONS"
      allowed_headers:
        - "Authorization"
        - "Content-Type"
    
    # Rate Limiting
    rate_limit:
      enabled: true
      global:
        requests_per_second: 1000
        burst: 2000
      per_user:
        requests_per_minute: 60
        burst: 100
    
    # WAF
    waf:
      enabled: true
      rules_dir: "/etc/hotplex/waf"
```

---

## 3. 环境变量

### 3.1 支持的环境变量

| 环境变量 | 类型 | 说明 |
|----------|------|------|
| `HOTPLEX_CONFIG` | string | 配置文件路径 |
| `HOTPLEX_LOG_LEVEL` | string | 日志级别 |
| `HOTPLEX_SERVER_PORT` | int | 服务端口 |

### 3.2 Provider 环境变量

| 环境变量 | 说明 |
|----------|------|
| `ANTHROPIC_API_KEY` | Anthropic API Key |
| `OPENAI_API_KEY` | OpenAI API Key |
| `OPENAI_ORG_ID` | OpenAI Organization ID |
| `SILICONFLOW_API_KEY` | SiliconFlow API Key |

### 3.3 Channel 环境变量

| 环境变量 | Channel | 说明 |
|----------|---------|------|
| `FEISHU_APP_ID` | Feishu | App ID |
| `FEISHU_APP_SECRET` | Feishu | App Secret |
| `FEISHU_VERIFICATION_TOKEN` | Feishu | Verification Token |
| `FEISHU_ENCRYPT_KEY` | Feishu | Encrypt Key |
| `SLACK_CLIENT_ID` | Slack | Client ID |
| `SLACK_CLIENT_SECRET` | Slack | Client Secret |
| `SLACK_SIGNING_SECRET` | Slack | Signing Secret |
| `SLACK_BOT_TOKEN` | Slack | Bot Token |
| `SLACK_APP_LEVEL_TOKEN` | Slack | App Level Token |

### 3.4 Storage 环境变量

| 环境变量 | 说明 |
|----------|------|
| `POSTGRES_DSN` | PostgreSQL DSN |
| `REDIS_ADDR` | Redis 地址 |
| `REDIS_PASSWORD` | Redis 密码 |

---

## 4. 配置验证

### 4.1 Schema 校验

```go
// 配置校验
func ValidateConfig(cfg *Config) error {
    // Server 校验
    if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
        return errors.New("server.port must be between 1 and 65535")
    }
    
    // Channel 校验
    if !cfg.Channels.Feishu.Enabled && 
       !cfg.Channels.Slack.Enabled && 
       !cfg.Channels.WS.Enabled {
        return errors.New("at least one channel must be enabled")
    }
    
    // Worker 校验
    if !cfg.Workers.ClaudeCode.Enabled && !cfg.Workers.OpenCode.Enabled {
        return errors.New("at least one worker must be enabled")
    }
    
    // Brain 校验
    if cfg.Brain.Default != "llm" && 
       cfg.Brain.Default != "rule" && 
       cfg.Brain.Default != "keyword" {
        return errors.New("brain.default must be llm, rule, or keyword")
    }
    
    // Storage 校验
    if cfg.Storage.Default != "sqlite" && 
       cfg.Storage.Default != "postgres" && 
       cfg.Storage.Default != "redis" {
        return errors.New("storage.default must be sqlite, postgres, or redis")
    }
    
    return nil
}
```

---

## 5. 配置示例

### 5.1 开发环境

```yaml
hotplex:
  server:
    port: 8080
    mode: debug
  
  channels:
    feishu:
      enabled: true
      debug: true
      app_id: "dev-feishu-app-id"
      app_secret: "dev-feishu-app-secret"
  
  workers:
    claude_code:
      enabled: true
      max_concurrent: 1
  
  brain:
    default: "rule"
    rule:
      enabled: true
  
  storage:
    default: "sqlite"
    sqlite:
      path: "/tmp/hotplex-dev.db"
  
  logging:
    level: debug
```

### 5.2 生产环境

```yaml
hotplex:
  server:
    port: 8080
    mode: release
  
  channels:
    feishu:
      enabled: true
      app_id: "${FEISHU_APP_ID}"
      app_secret: "${FEISHU_APP_SECRET}"
  
  workers:
    claude_code:
      enabled: true
      max_concurrent: 5
      execution:
        timeout: 300s
  
  brain:
    default: "llm"
    llm:
      provider: "anthropic"
      model: "claude-opus-4-20250514"
  
  storage:
    default: "postgres"
    postgres:
      dsn: "${POSTGRES_DSN}"
  
  observability:
    metrics:
      enabled: true
      port: 9090
```

---

*文档版本：v1.0 | 最后更新：2026-03-29*
