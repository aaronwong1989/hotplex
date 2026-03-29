# HotPlex v1.0.0 配置格式

> 版本：v1.0-final  
> 日期：2026-03-29  
> 状态：**已确认**

---

## 一、完整 YAML 配置

### 1.1 主配置文件结构

```yaml
# HotPlex 主配置
# 路径: configs/hotplex.yaml

version: "1.0"

# 应用配置
app:
  name: hotplex
  version: "1.0.0"
  environment: production  # development | production | test
  log_level: info         # debug | info | warn | error
  log_format: json        # json | text

# 服务器配置
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  max_connections: 1000

# 通道配置
channels:
  # 飞书通道
  feishu:
    enabled: true
    app_id: "${FEISHU_APP_ID}"
    app_secret: "${FEISHU_APP_SECRET}"
    encrypt_key: "${FEISHU_ENCRYPT_KEY}"
    verification_token: "${FEISHU_VERIFICATION_TOKEN}"
    bot_name: "HotPlex Bot"
    use_long_polling: true
    event_callback_url: "/webhooks/feishu"
    message_callback_url: "/webhooks/feishu/message"

  # Slack 通道
  slack:
    enabled: false
    bot_token: "${SLACK_BOT_TOKEN}"
    signing_secret: "${SLACK_SIGNING_SECRET}"
    app_token: "${SLACK_APP_TOKEN}"
    use_socket_mode: true
    bot_name: "HotPlex Bot"

  # WebSocket 通道
  ws:
    enabled: true
    path: "/ws"
    auth_enabled: true
    auth_secret: "${WS_AUTH_SECRET}"
    max_message_size: 10MB
    ping_interval: 30s
    pong_timeout: 10s

  # REST API 通道
  api:
    enabled: true
    prefix: "/api/v1"
    auth_enabled: true
    api_keys:
      - "${API_KEY_1}"
      - "${API_KEY_2}"
    rate_limit:
      enabled: true
      requests_per_minute: 100
      burst: 20

# 智能层配置
brain:
  # 意图识别
  intent:
    enabled: true
    model: "claude-3-5-sonnet"
    confidence_threshold: 0.7
    fallback_intent: "chat"
    cache_enabled: true
    cache_ttl: 5m

  # WAF 配置
  waf:
    enabled: true
    rules_file: "./configs/waf_rules.yaml"
    default_action: "warn"
    log_violations: true
    block_duration: 5m

  # 上下文增强
  context:
    enabled: true
    max_history_messages: 50
    max_context_tokens: 200000
    summarization_threshold: 150000

  # 记忆管理
  memory:
    enabled: true
    importance_threshold: 3
    retention_days: 90
    cleanup_interval: 1h

# Worker 配置
workers:
  # Worker 池配置
  pool:
    min_size: 2
    max_size: 10
    max_idle_time: 10m
    acquire_timeout: 30s
    health_check_interval: 30s

  # ClaudeCode Worker
  claude_code:
    enabled: true
    image: "hotplex/claude-code-worker:latest"
    replicas: 5
    timeout: 5m
    command:
      - "/bin/sh"
      - "-c"
      - "/opt/claude-code/run.sh"
    env:
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
      CLAUDE_CODE_VERSION: "latest"
    resources:
      cpu: "2"
      memory: "4Gi"
      disk: "10Gi"
    security:
      capabilities: "worker"
      seccomp: "true"
      read_only_root: true
      no_new_privileges: true

  # OpenCode Worker
  open_code:
    enabled: true
    image: "hotplex/open-code-worker:latest"
    replicas: 3
    timeout: 5m
    command:
      - "/bin/sh"
      - "-c"
      - "/opt/open-code/run.sh"
    env:
      OPENAI_API_KEY: "${OPENAI_API_KEY}"
      OPENCODE_MODEL: "gpt-4o"
    resources:
      cpu: "2"
      memory: "4Gi"
      disk: "10Gi"
    security:
      capabilities: "worker"
      seccomp: "true"
      read_only_root: true
      no_new_privileges: true

# Provider 配置
providers:
  # Anthropic
  anthropic:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
    base_url: "https://api.anthropic.com"
    timeout: 60s
    max_retries: 3
    retry_backoff: 1s
    models:
      - id: "claude-3-5-sonnet"
        name: "Claude 3.5 Sonnet"
        context_window: 200000
        input_cost: 3.0
        output_cost: 15.0
      - id: "claude-3-opus"
        name: "Claude 3 Opus"
        context_window: 200000
        input_cost: 15.0
        output_cost: 75.0

  # OpenAI
  openai:
    enabled: true
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com"
    timeout: 60s
    organization: "${OPENAI_ORG_ID}"

  # SiliconFlow
  siliconflow:
    enabled: false
    api_key: "${SILICONFLOW_API_KEY}"
    base_url: "https://api.siliconflow.cn"

# 存储配置
storage:
  # Session 存储
  session:
    type: sqlite  # sqlite | memory
    sqlite:
      path: "./data/sessions.db"
      max_open_conns: 10
      max_idle_conns: 5
      conn_max_lifetime: 1h
    memory:
      max_size: 10000
      ttl: 30m

  # Memory 存储
  memory:
    type: sqlite
    path: "./data/memory.db"
    max_size: 100000
    cleanup_interval: 1h

  # 文件存储
  file:
    type: local  # local | s3 | oss
    local:
      base_path: "./data/files"
      max_file_size: 100MB
      allowed_extensions:
        - ".txt"
        - ".md"
        - ".json"
        - ".yaml"
        - ".yml"
        - ".pdf"
        - ".jpg"
        - ".png"
    # s3:
    #   bucket: "${S3_BUCKET}"
    #   region: "cn-hangzhou"
    #   access_key: "${S3_ACCESS_KEY}"
    #   secret_key: "${S3_SECRET_KEY}"

# Docker 配置
docker:
  host: "unix:///var/run/docker.sock"
  default_network: "hotplex-bridge"
  registry_auth:
    - registry: "registry.hotplex.io"
      username: "${REGISTRY_USER}"
      password: "${REGISTRY_PASSWORD}"
  max_container_size: 10Gi
  cleanup_interval: 1h

# 安全配置
security:
  # Linux Capabilities 配置
  capabilities:
    profiles:
      # Worker 最严格配置
      worker:
        drop:
          - CAP_NET_RAW
          - CAP_NET_ADMIN
          - CAP_SYS_ADMIN
          - CAP_SYS_MODULE
          - CAP_SYS_RAWIO
          - CAP_SYS_PTRACE
          - CAP_SYS_TIME
          - CAP_SYS_BOOT
          - CAP_LEASE
          - CAP_SYSLOG
          - CAP_SETPCAP
        keep:
          - CAP_CHOWN
          - CAP_DAC_OVERRIDE
          - CAP_FOWNER
          - CAP_FSETID
          - CAP_KILL
          - CAP_SETGID
          - CAP_SETUID
          - CAP_SETPCAP
          - CAP_NET_BIND_SERVICE

      # Channel 中等配置
      channel:
        drop:
          - CAP_SYS_ADMIN
          - CAP_SYS_MODULE
          - CAP_NET_RAW
        keep:
          - CAP_NET_BIND_SERVICE
          - CAP_CHOWN

      # 自定义 Brain 配置
      brain:
        drop:
          - CAP_SYS_ADMIN
          - CAP_NET_ADMIN
        keep:
          - CAP_NET_BIND_SERVICE
          - CAP_SYS_NICE

  # Seccomp 配置
  seccomp:
    enabled: true
    profile: "hotplex-worker"  # hotplex-worker | default | unconfined
    custom_profile_path: "./configs/seccomp-worker.json"

  # No New Privileges
  no_new_privileges: true

  # 网络安全
  network:
    mode: "bridge"  # bridge | host | none
    lan_only: true
    allowed_outbound:
      - "api.anthropic.com:443"
      - "api.openai.com:443"
      - "api.siliconflow.cn:443"
    blocked_outbound:
      - "*.onion"
      - "*.i2p"

  # 文件系统安全
  filesystem:
    read_only_root: true
    allowed_writable_paths:
      - "/tmp"
      - "/workspace"
    tmp_size: "1Gi"

# 观测性配置
observability:
  # Prometheus 指标
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
    export_interval: 15s

  # OpenTelemetry 追踪
  tracing:
    enabled: true
    service_name: "hotplex"
    exporter: "otlp"  # otlp | jaeger | zipkin
    endpoint: "${OTEL_ENDPOINT}"
    sample_rate: 0.1

  # 日志配置
  logging:
    level: "info"
    format: "json"
    output: "stdout"  # stdout | file
    file:
      path: "./logs/hotplex.log"
      max_size: 100MB
      max_age: 7d
      max_backups: 10
    fields:
      service: "hotplex"
      version: "1.0.0"

# 插件配置
plugins:
  # 插件注册表
  registry:
    type: "file"  # file | etcd | consul
    path: "./plugins/registry.yaml"

  # 内置插件
  built_in:
    - name: "claude-code-worker"
      type: "worker"
      enabled: true
    - name: "open-code-worker"
      type: "worker"
      enabled: true
```

---

## 二、安全配置详解

### 2.1 Linux Capabilities 配置

```yaml
# Capabilities 详细配置
security:
  capabilities:
    profiles:
      worker:
        # 丢弃的危险 capabilities
        drop:
          # 网络安全
          - CAP_NET_RAW         # 禁止 raw socket（防止嗅探）
          - CAP_NET_ADMIN       # 禁止网络管理（防止修改路由）
          
          # 系统安全
          - CAP_SYS_ADMIN       # 禁止系统管理（挂载/设备）
          - CAP_SYS_MODULE      # 禁止加载内核模块
          - CAP_SYS_RAWIO       # 禁止裸 I/O 访问
          - CAP_SYS_PTRACE      # 禁止调试其他进程
          - CAP_SYS_TIME        # 禁止修改系统时间
          - CAP_SYS_BOOT        # 禁止重启系统
          
          # 其他危险
          - CAP_LEASE           # 禁止文件租约
          - CAP_SYSLOG          # 禁止 syslog 操作
          - CAP_SETPCAP         # 禁止修改 capabilities
        
        # 保留的必要 capabilities
        keep:
          - CAP_CHOWN           # 允许更改文件所有权
          - CAP_DAC_OVERRIDE    # 允许绕过 DAC 限制
          - CAP_FOWNER          # 允许所有者权限检查
          - CAP_FSETID          # 允许保留文件 flags
          - CAP_KILL            # 允许发送信号
          - CAP_SETGID          # 允许设置 GID
          - CAP_SETUID          # 允许设置 UID
          - CAP_NET_BIND_SERVICE # 允许绑定 < 1024 端口
```

### 2.2 Seccomp 白名单配置

```json
// configs/seccomp-worker.json
{
  "defaultAction": "SCMP_ACT_ERRNO",
  "syscalls": [
    // 基础系统调用
    {"names": ["read", "write", "open", "close"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["brk", "mmap", "mprotect", "munmap"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["fstat", "pipe", "poll", "select"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["lseek", "readv", "writev", "getdents"], "action": "SCMP_ACT_ALLOW"},
    
    // 网络（只读）
    {"names": ["socket", "connect", "listen", "accept"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["sendto", "recvfrom", "sendmsg", "recvmsg"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["shutdown", "bind", "getsockname", "getpeername"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["setsockopt", "getsockopt"], "action": "SCMP_ACT_ALLOW"},
    
    // 进程
    {"names": ["clone", "exit", "wait4", "kill"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["prctl", "gettid", "getpid", "geteuid"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["nanosleep", "getppid", "setsid"], "action": "SCMP_ACT_ALLOW"},
    
    // 文件系统
    {"names": ["unlink", "mkdir", "rmdir", "rename"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["chmod", "chown", "truncate", "access"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["stat", "lstat", "readlink"], "action": "SCMP_ACT_ALLOW"},
    
    // 环境
    {"names": ["getgroups", "setgroups", "getuid", "getgid"], "action": "SCMP_ACT_ALLOW"},
    {"names": ["getrlimit", "setrlimit", "umask"], "action": "SCMP_ACT_ALLOW"},
    
    // 禁止的系统调用
    {"names": ["mount", "umount2", "pivot_root"], "action": "SCMP_ACT_ERRNO"},
    {"names": ["init_module", "delete_module"], "action": "SCMP_ACT_ERRNO"},
    {"names": ["lookup_dcookie"], "action": "SCMP_ACT_ERRNO"},
    {"names": ["perf_event_open"], "action": "SCMP_ACT_ERRNO"}
  ]
}
```

---

## 三、Docker 插件配置

### 3.1 Worker 容器配置

```yaml
workers:
  claude_code:
    # 基础配置
    image: "hotplex/claude-code-worker:latest"
    replicas: 5
    
    # 容器命名
    container_name_prefix: "hotplex-worker-claude"
    
    # 启动命令
    command:
      - "/bin/sh"
      - "-c"
      - |
        export ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
        /opt/claude-code/run.sh --mode=streaming --verbosity=info
    
    # 环境变量
    env:
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
      CLAUDE_CODE_VERSION: "latest"
      CLAUDE_CODE_FLAGS: "--no-input --output-format=stream-json"
      HOME: "/workspace"
      PATH: "/usr/local/bin:/usr/bin:/bin:/opt/claude-code/bin"
    
    # 资源限制
    resources:
      # CPU
      cpu: "2"
      cpu_period: 100000
      cpu_quota: 200000
      
      # 内存
      memory: "4Gi"
      memory_swap: "4Gi"
      memory_reservation: "2Gi"
      
      # PIDs
      pids_limit: 256
    
    # 挂载
    mounts:
      # 工作空间（读写）
      - type: "bind"
        source: "./workspace"
        target: "/workspace"
        read_only: false
      
      # 配置文件（只读）
      - type: "bind"
        source: "./configs"
        target: "/etc/hotplex"
        read_only: true
      
      # 临时文件（tmpfs）
      - type: "tmpfs"
        target: "/tmp"
        size: "1Gi"
    
    # 网络配置
    network:
      mode: "bridge"
      name: "hotplex-worker-net"
      ipv4_address: "172.20.0.0/16"
    
    # 端口映射
    ports: []
    
    # 安全配置
    security:
      # Capabilities
      capabilities:
        drop:
          - CAP_NET_RAW
          - CAP_NET_ADMIN
          - CAP_SYS_ADMIN
          - CAP_SYS_MODULE
          - CAP_SYS_RAWIO
          - CAP_SYS_PTRACE
          - CAP_SYS_TIME
          - CAP_SYS_BOOT
        keep:
          - CAP_NET_BIND_SERVICE
      
      # Seccomp
      seccomp:
        profile: "hotplex-worker"
      
      # No New Privileges
      no_new_privileges: true
      
      # 只读根文件系统
      read_only_root_fs: true
      
      # 用户
      user: "1000:1000"
    
    # 健康检查
    healthcheck:
      enabled: true
      test: ["/bin/sh", "-c", "ps aux | grep -v grep | grep claude-code"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
    
    # 生命周期钩子
    lifecycle:
      post_start:
        - type: "exec"
          command: ["/bin/sh", "-c", "echo 'Worker started'"]
      pre_stop:
        - type: "exec"
          command: ["/bin/sh", "-c", "kill -SIGTERM 1"]
    
    # 自动恢复
    restart_policy:
      enabled: true
      max_attempts: 3
      backoff: 5s
```

### 3.2 网络隔离配置

```yaml
# Docker 网络配置
docker:
  networks:
    # Worker 网络（隔离）
    worker_net:
      driver: "bridge"
      name: "hotplex-worker-net"
      ipam:
        driver: "default"
        config:
          - subnet: "172.20.0.0/16"
            gateway: "172.20.0.1"
    
    # Host 网络（仅必要服务）
    host_net:
      driver: "host"
```

---

## 四、环境变量

### 4.1 必需环境变量

```bash
# HotPlex 核心
export HOTPLEX_ENV=production
export HOTPLEX_LOG_LEVEL=info

# 飞书配置
export FEISHU_APP_ID=cli_xxx
export FEISHU_APP_SECRET=xxx
export FEISHU_ENCRYPT_KEY=xxx

# API Keys
export ANTHROPIC_API_KEY=sk-ant-xxx
export OPENAI_API_KEY=sk-xxx
export SILICONFLOW_API_KEY=xxx

# Docker
export DOCKER_HOST=unix:///var/run/docker.sock

# 观测性
export OTEL_ENDPOINT=https://otel.hotplex.io:4317
```

### 4.2 可选环境变量

```bash
# Redis（如果使用 Redis Session）
export REDIS_URL=redis://localhost:6379/0

# S3 文件存储
export S3_BUCKET=hotplex-files
export S3_ACCESS_KEY=xxx
export S3_SECRET_KEY=xxx

# LDAP 认证
export LDAP_URL=ldap://localhost:389
export LDAP_BASE_DN=dc=hotplex,dc=io
```

---

_最后更新：2026-03-29_
