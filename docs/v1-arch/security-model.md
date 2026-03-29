# HotPlex v1.0.0 安全模型

> 版本：v1.0-final  
> 日期：2026-03-29  
> 状态：**新增文档**（基于 OpenClaw 最佳实践）

---

## 一、安全架构总览

### 1.1 纵深防御体系

```
┌─────────────────────────────────────────────────────────────────┐
│                        HotPlex Runtime                           │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Layer 1: 进程隔离 (Docker Container)                     │   │
│  │  - 插件隔离在不同容器                                     │   │
│  │  - 独立网络命名空间                                      │   │
│  │  - 独立 PID 命名空间                                    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Layer 2: Linux Capabilities 限制                         │   │
│  │  - 丢弃所有危险 capabilities                             │   │
│  │  - 仅保留最小必要权限                                    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Layer 3: Seccomp 白名单                                  │   │
│  │  - 仅允许安全系统调用                                     │   │
│  │  - 阻止所有非必要 syscall                                │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Layer 4: No-New-Privileges                               │   │
│  │  - 禁止 setuid/setgid                                   │   │
│  │  - 防止权限提升                                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Layer 5: 网络策略 (LAN-only)                            │   │
│  │  - 仅绑定 127.0.0.1                                     │   │
│  │  - 出站连接白名单                                        │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Layer 6: 文件系统隔离                                    │   │
│  │  - 只读根文件系统                                        │   │
│  │  - 白名单写目录                                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 安全原则

| 原则 | 实现 |
|------|------|
| **最小特权** | 容器仅获得完成任务所需的最小权限 |
| **纵深防御** | 多层安全机制相互补强 |
| **默认安全** | 安全配置开箱即用，无需额外配置 |
| **隔离优先** | 插件间完全隔离，无共享内存/网络 |

---

## 二、Linux Capabilities 限制

### 2.1 为什么要限制 Capabilities

Linux capabilities 将 root 权限分解为多个独立单元。容器默认以 root 运行，但通过丢弃不需要的 capabilities，可以限制潜在危害。

### 2.2 Capabilities 配置

#### Worker（最严格）

```yaml
# Worker 默认丢弃的 capabilities
capabilities:
  drop:
    # 网络安全
    - CAP_NET_RAW         # 禁止创建 raw socket（嗅探）
    - CAP_NET_ADMIN       # 禁止网络管理（修改路由）
    
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
  
  keep:
    # 文件操作（必需）
    - CAP_CHOWN           # 允许修改文件所有权
    - CAP_DAC_OVERRIDE   # 允许绕过 DAC 权限检查
    - CAP_FOWNER         # 允许修改文件所有者
    - CAP_FSETID         # 允许设置文件标志
    - CAP_SETGID         # 允许设置进程组 ID
    - CAP_SETUID         # 允许设置进程用户 ID
```

#### Channel（中等）

```yaml
# Channel 保留的 capabilities
capabilities:
  drop:
    - CAP_NET_RAW         # 禁止 raw socket
    - CAP_NET_ADMIN       # 禁止网络管理
    - CAP_SYS_ADMIN       # 禁止系统管理
  keep:
    - CAP_NET_BIND_SERVICE # 允许绑定低端口（可选）
```

### 2.3 Go 代码实现

```go
// pkg/security/capabilities.go

package security

// Capabilities Linux capabilities 配置
type Capabilities struct {
    Keep []string  // 保留的能力
    Drop []string  // 丢弃的能力
}

// DefaultWorkerCapabilities Worker 默认 capabilities
var DefaultWorkerCapabilities = Capabilities{
    Keep: []string{
        "CAP_CHOWN",
        "CAP_DAC_OVERRIDE",
        "CAP_FOWNER",
        "CAP_FSETID",
        "CAP_SETGID",
        "CAP_SETUID",
    },
    Drop: []string{
        "CAP_NET_RAW",
        "CAP_NET_ADMIN",
        "CAP_SYS_ADMIN",
        "CAP_SYS_MODULE",
        "CAP_SYS_RAWIO",
        "CAP_SYS_PTRACE",
        "CAP_SYS_TIME",
        "CAP_SYS_BOOT",
        "CAP_LEASE",
        "CAP_SYSLOG",
        "CAP_SETPCAP",
    },
}

// ToDockerArgs 转换为 Docker 参数
func (c *Capabilities) ToDockerArgs() []string {
    var args []string
    
    // 先丢弃所有
    args = append(args, "--cap-drop=ALL")
    
    // 再按需添加
    for _, cap := range c.Keep {
        args = append(args, "--cap-add="+cap)
    }
    
    return args
}
```

---

## 三、Seccomp 白名单

### 3.1 Seccomp 简介

Seccomp (Secure Computing Mode) 允许过滤系统调用。配置为白名单模式时，仅允许预定义的系统调用。

### 3.2 默认 Seccomp 配置

```json
{
  "defaultAction": "SCMP_ACT_ERRNO",
  " architectures": [
    "SCMP_ARCH_X86_64",
    "SCMP_ARCH_AARCH64"
  ],
  "syscalls": [
    {
      "names": [
        "read",
        "write",
        "open",
        "close",
        "stat",
        "fstat",
        "lstat",
        "poll",
        "lseek",
        "mprotect",
        "mmap",
        "brk",
        "rt_sigaction",
        "rt_sigprocmask",
        "ioctl",
        "pread64",
        "pwrite64",
        "readv",
        "writev",
        "access"
      ],
      "action": "SCMP_ACT_ALLOW"
    },
    {
      "names": [
        "pipe",
        "sched_yield",
        "madvise",
        "dup",
        "dup2",
        "pause",
        "nanosleep",
        "getitimer",
        "alarm",
        "setitimer"
      ],
      "action": "SCMP_ACT_ALLOW"
    },
    {
      "names": [
        "socket",
        "connect",
        "bind",
        "listen",
        "accept"
      ],
      "action": "SCMP_ACT_ALLOW"
    },
    {
      "names": [
        "shmget",
        "shmat",
        "shmctl"
      ],
      "action": "SCMP_ACT_ALLOW"
    }
  ]
}
```

### 3.3 Go 生成 Seccomp 配置

```go
// pkg/security/seccomp.go

package security

import (
    "encoding/json"
)

// DefaultSyscalls 默认允许的系统调用
var DefaultSyscalls = []string{
    // 文件操作
    "read", "write", "open", "close", "stat", "fstat",
    "lstat", "poll", "lseek", "mprotect", "mmap", "brk",
    "rt_sigaction", "rt_sigprocmask", "ioctl", "pread64",
    "pwrite64", "readv", "writev", "access", "pipe",
    
    // 进程
    "sched_yield", "madvise", "dup", "dup2", "pause",
    "nanosleep", "getitimer", "alarm", "setitimer",
    
    // 网络（仅 Channel 需要）
    "socket", "connect", "bind", "listen", "accept",
    
    // 内存
    "shmget", "shmat", "shmctl",
}

// SeccompProfile Seccomp 配置
type SeccompProfile struct {
    DefaultAction string            `json:"defaultAction"`
    Architectures []string          `json:"architectures"`
    Syscalls      []SyscallRule     `json:"syscalls"`
}

// SyscallRule 系统调用规则
type SyscallRule struct {
    Names  []string `json:"names"`
    Action string  `json:"action"`
}

// GenerateDefaultProfile 生成默认配置文件
func GenerateDefaultProfile(allowNetwork bool) *SeccompProfile {
    profile := &SeccompProfile{
        DefaultAction: "SCMP_ACT_ERRNO",  // 默认拒绝
        Architectures: []string{
            "SCMP_ARCH_X86_64",
            "SCMP_ARCH_AARCH64",
        },
        Syscalls: []SyscallRule{
            {Names: DefaultSyscalls, Action: "SCMP_ACT_ALLOW"},
        },
    }
    
    if !allowNetwork {
        // Worker：移除网络相关 syscall
        filtered := filterSyscalls(profile.Syscalls[0].Names, []string{
            "socket", "connect", "bind", "listen", "accept",
        })
        profile.Syscalls[0].Names = filtered
    }
    
    return profile
}

// ToJSON 序列化为 JSON
func (s *SeccompProfile) ToJSON() ([]byte, error) {
    return json.MarshalIndent(s, "", "  ")
}
```

---

## 四、No-New-Privileges

### 4.1 什么是 No-New-Privileges

`no-new-privileges` 标志阻止进程及其子进程通过 `setuid`/`setgid` 或文件 capabilities 获取新权限。

### 4.2 配置

```yaml
security:
  no_new_privileges: true
```

### 4.3 Docker 参数

```bash
docker run --security-opt=no-new-privileges:true ...
```

### 4.4 为什么要启用

| 攻击场景 | 无 no-new-privileges | 有 no-new-privileges |
|----------|---------------------|---------------------|
| SUID 二进制滥用 | 进程可能获取 root | ❌ 被阻止 |
| setuid shell 脚本 | 可能提权 | ❌ 被阻止 |
| execve() 覆盖权限 | 可能提权 | ❌ 被阻止 |

---

## 五、网络策略

### 5.1 绑定地址限制

```yaml
# 仅监听本地（LAN-only）
server:
  host: "127.0.0.1"
  port: 8080
```

### 5.2 出站连接白名单

```yaml
network:
  outbound:
    allowed:
      # LLM Provider
      - "api.anthropic.com/32"
      - "api.openai.com/32"
      - "api.siliconflow.com/32"
      
      # 内网（可选）
      - "10.0.0.0/8"
      - "172.16.0.0/12"
      - "192.168.0.0/16"
    
    blocked_ports:
      - 25    # SMTP（防垃圾邮件）
      - 465   # SMTPS
      - 587   # SMTP
      - 1194  # OpenVPN
      - 1723  # PPTP
```

### 5.3 容器网络模式

| 模式 | 描述 | 适用场景 |
|------|------|----------|
| `none` | 无网络 | Worker（推荐）|
| `bridge` | 桥接网络 | Channel（需要出站）|
| `host` | 共享宿主机网络 | ❌ 不推荐 |

```go
// 默认 Worker 网络配置
var WorkerNetworkConfig = map[string]interface{}{
    "NetworkMode": "none",  // 最安全
}

// Channel 网络配置
var ChannelNetworkConfig = map[string]interface{}{
    "NetworkMode": "bridge",
}
```

---

## 六、容器隔离策略

### 6.1 资源限制

```yaml
# Worker 资源限制
worker:
  resources:
    cpu: 1.0              # 1 个 CPU
    memory: "512m"        # 512MB 内存
    memory_swap: "512m"   # 禁用 swap
    pids: 64              # 最多 64 个进程

# Channel 资源限制
channel:
  resources:
    cpu: 0.5
    memory: "256m"
    pids: 32
```

### 6.2 PID 限制

```yaml
# 防止 fork 炸弹
pids_limit: 64
```

### 6.3 存储驱动

```yaml
# 推荐使用 overlay2
storage_driver: "overlay2"

# 容器根文件系统只读
readonly_rootfs: true

# 允许的写目录
tmpfs:
  - "/tmp:size=64m,mode=1777"
```

### 6.4 用户命名空间

```yaml
# 可选：用户命名空间映射
userns_mode: "host"  # 或使用自定义映射
```

---

## 七、WAF/Guard 机制

### 7.1 Brain Guard 层级

```
Message
    │
    ▼
┌─────────────────────────────────────────┐
│           Layer 1: KeywordBrain          │
│  - 快速关键词匹配                         │
│  - 已知恶意模式拦截                       │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│              Layer 2: LLMBrain           │
│  - 意图分类                              │
│  - 语义安全检查                          │
│  - 上下文感知                            │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│             Layer 3: RuleBrain           │
│  - 白名单/黑名单规则                       │
│  - 正则表达式匹配                         │
│  - 敏感信息检测                          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
[Allowed] 或 [Blocked]
```

### 7.2 规则配置

```yaml
# Rule Brain 配置
brain:
  rule:
    # 敏感词过滤
    sensitive_words:
      - pattern: "(密码|password).*[:：]"
        action: block
        severity: high
      - pattern: "(身份证|银行卡).*号"
        action: block
        severity: high
    
    # SQL 注入检测
    sql_injection:
      - pattern: "('|(\\'\')|;|DROP|UNION|SELECT|--)"
        action: block
        severity: high
    
    # 命令注入检测
    command_injection:
      - pattern: "([;|`$]|&&|\\|\\|)"
        action: block
        severity: high
    
    # 文件路径遍历检测
    path_traversal:
      - pattern: "(\\.\\./|\\.\\.\\\\)"
        action: block
        severity: medium
    
    # 白名单规则（放行）
    allowed_patterns:
      - "^/status"
      - "^/health"
      - "^/ping"
```

### 7.3 违规响应级别

| Level | Action | Response |
|-------|--------|----------|
| `block` | 直接拒绝 | 返回拦截提示 |
| `warn` | 记录日志 | 放行但告警 |
| `pass` | 放行 | 无操作 |

---

## 八、密钥管理

### 8.1 环境变量注入

```yaml
# 仅通过环境变量注入敏感信息
env:
  - name: "ANTHROPIC_API_KEY"
    value: "${ANTHROPIC_API_KEY}"
  - name: "SLACK_BOT_TOKEN"
    value: "${SLACK_BOT_TOKEN}"
```

### 8.2 密钥轮换

```yaml
secrets:
  # 自动从 Vault/SSM 获取
  provider: "vault"  # "env" | "vault" | "aws-secrets"
  
  vault:
    addr: "https://vault.example.com"
    path: "secret/hotplex"
  
  # 自动轮换
  rotation:
    enabled: true
    interval: 24h
```

### 8.3 容器内密钥访问

```go
// 容器内获取密钥
type SecretManager struct {
    provider SecretProvider
}

// GetSecret 获取密钥（从环境变量或密钥服务）
func (m *SecretManager) GetSecret(ctx context.Context, key string) (string, error) {
    // 优先从环境变量获取
    if val := os.Getenv(key); val != "" {
        return val, nil
    }
    
    // 否则从密钥服务获取
    return m.provider.Get(ctx, key)
}
```

---

## 九、审计日志

### 9.1 审计事件

```go
// AuditEvent 审计事件
type AuditEvent struct {
    Timestamp   time.Time
    EventType   string        // "message" | "plugin_load" | "config_change" | "auth"
    UserID      string
    SessionID   string
    PluginID    string
    Action      string
    Result      string        // "success" | "denied" | "error"
    Details     map[string]interface{}
    ClientIP    string
}
```

### 9.2 审计日志配置

```yaml
audit:
  enabled: true
  output: "/var/log/hotplex/audit.log"
  format: "json"
  
  # 记录的事件类型
  events:
    - message_received
    - message_blocked
    - plugin_load
    - plugin_unload
    - config_change
    - auth_attempt
    - secret_access
  
  # 敏感字段脱敏
  redact:
    - "api_key"
    - "password"
    - "token"
    - "secret"
```

---

## 十、安全检查清单

### 10.1 部署前检查

| 检查项 | 验证方法 |
|--------|----------|
| capabilities 限制 | `docker inspect` 检查 CapEffective |
| no-new-privileges | 检查 SecurityOpt |
| 网络绑定 | `netstat -tlnp` 确认仅 127.0.0.1 |
| 资源限制 | `docker stats` 验证 |
| 只读根文件系统 | `docker inspect` 检查 ReadonlyRootfs |
| Seccomp | 检查 SecurityOpt |

### 10.2 运行时监控

| 监控项 | 告警条件 |
|--------|----------|
| 容器重启次数 | > 5次/小时 |
| CPU 使用率 | 持续 > 90% |
| 内存使用率 | 持续 > 90% |
| 拒绝的网络连接 | > 100次/分钟 |
| 异常系统调用 | Seccomp 阻止 |

---

## 十一、合规性

### 11.1 符合的标准

| 标准 | 说明 |
|------|------|
| CIS Docker Benchmark | 遵循 CIS 安全基线 |
| NIST SP 800-190 | 容器安全指南 |
| GDPR | 个人信息处理合规 |

### 11.2 安全报告

```bash
# 生成安全报告
hotplex security report --format html --output security-report.html

# 检查配置
hotplex security check --strict
```

---

*文档版本：v1.0-final | 最后更新：2026-03-29*
