# 输入验证与命令注入防护 - 最佳实践调研

> 调研时间：2026-03-30
> 调研范围：OWASP、CWE、Go 标准库文档、HashiCorp Vault、AWS Secrets Manager

---

## 1. 命令注入防护

### 1.1 威胁模型（CWE-78）

CWE-78（OS Command Injection）指产品通过外部输入构造 OS 命令时，未能中和特殊元素（如 `;`、`|`、`&`、`$()`），导致攻击者可以注入任意命令。

**典型攻击路径：**
```
用户输入 "file; rm -rf /" → 拼接到命令字符串 → system(command)
→ 实际执行: ls file; rm -rf /
```

**OWASP 分类的影响范围**：
- 直接执行：`system()`、`popen()`、`exec()`（shell 模式）
- 间接执行：通过 CLI 工具调用（`sendmail`、`rsync`）

### 1.2 Go os/exec 的安全设计

Go 的 `os/exec` 包从设计上防止了大多数命令注入：

| 特性 | 说明 |
|------|------|
| **无 Shell 调用** | `exec.Command("ls", userInput)` 不会调用 `/bin/sh`，metacharacter 作为字面值传递 |
| **参数化执行** | 参数作为 `[]string` 传递，不会被 shell 解析 |
| **禁止当前目录查找**（Go 1.19+） | `exec.LookPath` 拒绝 `./program`，防止 `PATH` 污染攻击 |
| **ErrDot 机制** | 若路径解析到当前目录返回 `exec.ErrDot`，需显式 opt-in |

```go
// ✅ 安全：参数化传递，无 shell 解析
cmd := exec.Command("grep", "-r", userPattern, directory)

// ❌ 危险：string 拼接，等效于 shell -c
cmd := exec.Command("sh", "-c", "grep -r "+userPattern+" "+directory)
```

**关键原则**：永远不要将用户输入拼接到命令字符串中传递给 `sh -c` 或 `bash -c`。

### 1.3 exec.Command vs exec.CommandContext

| 函数 | 适用场景 | 安全性 |
|------|----------|--------|
| `exec.Command(name, args...)` | 基础参数化执行 | 高（无 shell） |
| `exec.CommandContext(ctx, name, args...)` | 需要超时/取消控制 | 高 + 可控超时 |
| `exec.Command("sh", "-c", str)` | **禁止接受用户输入** | 危险 |

```go
// ✅ 推荐：带超时的参数化执行
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
cmd := exec.CommandContext(ctx, "git", "clone", url, targetDir)
cmd.Dir = "/allowed/workdir"  // 限制工作目录
out, err := cmd.CombinedOutput()
```

### 1.4 白名单命令策略

OWASP 明确要求："When it comes to the commands used, these must be validated against a list of allowed commands."

```go
var allowedCommands = map[string]bool{
    "git":   true,
    "docker": true,
    "ls":    true,
    "cat":   true,
}

// 白名单检查
func validateCommand(name string) error {
    if !allowedCommands[name] {
        return fmt.Errorf("command %q not allowed", name)
    }
    path, err := exec.LookPath(name)
    if err != nil {
        return fmt.Errorf("command %q not found in PATH: %w", name, err)
    }
    return nil
}
```

### 1.5 Shell Metacharacter 过滤（纵深防御）

即使使用 `exec.Command`，某些场景下仍需过滤 metacharacter（作为纵深防御）：

```go
// 高风险命令参数：禁止 shell metacharacter
var dangerousMetachars = regexp.MustCompile(`[;&|$`<>()!\\'"{}[\]]`)

func sanitizeArg(arg string) (string, error) {
    if dangerousMetachars.MatchString(arg) {
        return "", fmt.Errorf("forbidden characters in argument")
    }
    if len(arg) > 4096 {
        return "", fmt.Errorf("argument too long (max 4096)")
    }
    return arg, nil
}
```

### 1.6 目录隔离与工作目录限制

CLI 工具的 `WorkDir` 必须限定在已知安全目录内：

```go
// ✅ 始终指定工作目录，不依赖默认行为
cmd := exec.CommandContext(ctx, "git", "status")
cmd.Dir = "/var/hotplex/sessions/session-123"  // 隔离 session 目录

// ❌ 危险：未限定工作目录
cmd := exec.Command("git", "status")
// 若进程 cwd 被攻击者控制，可能执行任意代码
```

### 1.7 最小权限原则

CLI 进程应使用最小权限运行：

- 使用 `syscall.Credential` 限制运行用户（而非 root）
- 通过 `cmd.SysProcAttr` 设置 `Gid`/`Uid`
- 容器环境下使用只读文件系统 + 临时目录

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Credential: &syscall.Credential{
        Uid: 65534,  // nobody
        Gid: 65534,
    },
    Setpgid: true,  // 进程组隔离（HotPlex 已有）
}
```

---

## 2. 输入验证策略

### 2.1 核心原则：白名单优先

OWASP Input Validation Cheat Sheet 明确指出：

> "Allowlist validation is appropriate for all input fields. Allowlist validation involves defining exactly what IS authorized, and by definition, everything else is not authorized."
>
> Denylist（黑名单）验证是 "massively flawed" 和 "easily bypassed"。

**白名单 vs 黑名单对比：**

| 策略 | 做法 | 风险 |
|------|------|------|
| **白名单** | `^[a-zA-Z0-9_-]{1,64}$` | 过于严格可能拒绝合法输入 |
| **黑名单** | 过滤 `;` `|` `&` 等 | 易被绕过（如 URL 编码、双重编码） |

### 2.2 验证层次

OWASP 要求两层验证：

1. **语法验证**（Syntactic）：格式、长度、字符集
2. **语义验证**（Semantic）：值在业务上下文中是否合法

```go
// 语法验证示例
type SessionID string

func (s SessionID) Validate() error {
    const sessionIDPattern = `^[a-zA-Z0-9_-]{8,64}$`
    if !regexp.MustCompile(sessionIDPattern).MatchString(string(s)) {
        return errors.New("invalid session ID format")
    }
    return nil
}

// 语义验证示例
func validateSessionExists(s SessionID) error {
    if !sessionStore.Exists(s) {
        return errors.New("session does not exist")
    }
    return nil
}
```

### 2.3 长度限制

OWASP 建议所有输入定义最小和最大长度：

```go
const (
    MinSessionIDLen = 8
    MaxSessionIDLen = 64
    MaxMessageLen   = 16_000  // Slack 消息限制
    MaxCommandArgs  = 32
)

func validateLength(s string, min, max int) error {
    n := len(s)
    if n < min || n > max {
        return fmt.Errorf("length must be between %d and %d, got %d", min, max, n)
    }
    return nil
}
```

### 2.4 类型检查与强制转换

Go 作为强类型语言，应充分利用类型系统：

```go
// ✅ 使用结构体 + 验证方法
type ChatMessage struct {
    Platform  string `json:"platform"`
    ChannelID string `json:"channel_id"`
    Text      string `json:"text"`
}

func (m *ChatMessage) Validate() error {
    if m.Platform != "slack" && m.Platform != "telegram" {
        return errors.New("unsupported platform")
    }
    if !channelIDPattern.MatchString(m.ChannelID) {
        return errors.New("invalid channel ID")
    }
    return nil
}

// ✅ 数值边界检查
type Pagination struct {
    Page    int `json:"page"`
    PerPage int `json:"per_page"`
}

func (p *Pagination) Validate() error {
    if p.Page < 1 {
        return errors.New("page must be >= 1")
    }
    if p.PerPage < 1 || p.PerPage > 100 {
        return errors.New("per_page must be between 1 and 100")
    }
    return nil
}
```

### 2.5 JSON Schema 验证

对于结构化 JSON 输入，JSON Schema 验证是标准化方案：

```go
import "github.com/xeipuuv/gojsonschema"

func validateJSONSchema(data []byte, schemaPath string) error {
    schema := gojsonschema.NewReferenceLoader(schemaPath)
    document := gojsonschema.NewBytesLoader(data)

    result, err := gojsonschema.Validate(schema, document)
    if err != nil {
        return fmt.Errorf("schema validation error: %w", err)
    }
    if !result.Valid() {
        var errs []string
        for _, desc := range result.Errors() {
            errs = append(errs, desc.String())
        }
        return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
    }
    return nil
}
```

### 2.6 服务端验证强制要求

OWASP 明确警告：

> "Server-side validation is mandatory. Client-side JavaScript can be circumvented by an attacker."

即所有客户端验证（JS 表单验证）都必须在服务端重复执行。

---

## 3. 环境变量安全

### 3.1 12-Factor App Config 原则

[12-Factor App Config](https://12factor.net/config) 定义了核心原则：

- **配置与代码严格分离**：凭证绝不写在代码或默认配置中
- **正交性**：每个环境变量独立管理，不按 "dev/prod" 捆绑
- **开源测试**：检查代码是否能在不暴露凭证的情况下开源

**HotPlex 当前做法**：`.env` 文件存储凭证，通过 `os.ExpandEnv` 注入 YAML 配置。这符合 12-Factor，但存在以下风险点。

### 3.2 Go os.ExpandEnv 的 Shell 默认值陷阱

**关键陷阱**（HotPlex 项目已有教训）：

```go
// ❌ 错误：Go 不支持 ${VAR:-default} 语法
os.ExpandEnv("${HOTPLEX_SLACK_BOT_USER_ID:-}")

// ✅ 正确：只用 ${VAR}，在 .env 中提供默认值
os.ExpandEnv("${HOTPLEX_SLACK_BOT_USER_ID}")
```

Go 的 `os.ExpandEnv` 使用 `os.Getenv`，只支持 `$VAR` 和 `${VAR}` 两种形式，不支持 shell 风格的 `${VAR:-default}`。

### 3.3 Secret 管理方案对比

| 方案 | 适用场景 | HotPlex 推荐 |
|------|----------|-------------|
| **本地 .env** | 开发/单机部署 | ✅ 当前方案，简单够用 |
| **HashiCorp Vault** | 多服务/生产环境 | ⭐ 推荐，动态凭证 |
| **AWS Secrets Manager** | AWS 部署 | ⭐ 推荐，ECS/EKS 原生集成 |
| **K8s Secrets** | Kubernetes | 适合容器化 HotPlex |

### 3.4 环境变量隔离策略

**核心原则**：不同来源的环境变量应隔离，防止 host 环境变量污染容器配置。

```go
// 方案 1：仅从受限文件加载环境变量
func loadEnvFromFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    for _, line := range strings.Split(string(data), "\n") {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue
        }
        os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
    }
    return nil
}

// 方案 2：Docker Compose 环境隔离
// docker-compose.yml 中明确指定 env_file，不继承 host 环境
```

### 3.5 Vault Agent Template 动态注入

生产环境推荐使用 HashiCorp Vault：

```hcl
# Vault Agent 配置示例
template {
  source      = "/etc/secrets/db.tpl"
  destination = "/run/secrets/db.env"
}
```

```bash
# db.tpl 内容
{{- with secret "database/creds/hotplex-db" -}}
DB_USER={{ .Data.username }}
DB_PASS={{ .Data.password }}
{{- end }}
```

### 3.6 环境变量注入最佳实践

| 实践 | 说明 |
|------|------|
| **不打印 Secret** | 日志中禁止打印 `*_KEY`、`*_TOKEN`、`*_SECRET` |
| **不传递超长值** | 警惕 `argv` 溢出（部分 OS 限制 128KB） |
| **使用 Env 切片** | `cmd.Env` 覆盖而非追加，避免继承恶意变量 |
| **区分配置 vs Secret** | 配置（超时、路径）可日志，Secret 绝对不能 |

```go
// ✅ 安全：显式构建 Env，不继承 host 环境
cmd.Env = []string{
    "HOME=/var/hotplex",
    "LANG=en_US.UTF-8",
    "HOTPLEX_SESSION_ID=" + sessionID,
    // 注意：不包含 USER、PATH 等可能影响行为的变量
}
```

---

## 4. 路径安全

### 4.1 威胁模型（CWE-22）

Path Traversal（路径遍历）指应用程序使用外部输入构造文件路径时，未能正确中和 `..` 序列，导致攻击者访问受限目录之外的文件。

**典型攻击**：
```
请求: GET /files?path=../../etc/passwd
拼接: /var/www/files/../../etc/passwd
实际: /etc/passwd
```

### 4.2 Go 路径安全工具

Go 提供了 `path/filepath` 包用于路径规范化：

| 函数 | 行为 |
|------|------|
| `filepath.Clean(path)` | 规范化路径：`/a/b/../c` → `/a/c`，但 **不解析符号链接** |
| `filepath.EvalSymlinks(path)` | 解析符号链接，获取真实路径 |
| `filepath.Rel(base, path)` | 计算相对路径 |
| `filepath.Join(base, sub...)` | 连接路径，自动处理分隔符 |

**关键陷阱**：

```go
// ❌ filepath.Clean 不解析 symlink
// 攻击：创建 /tmp/attack -> /etc 符号链接
cleaned := filepath.Clean("/tmp/attack/../secret")  // = "/etc/secret" 但路径合法
resolved, _ := filepath.EvalSymlinks(cleaned)     // = "/etc/secret"（真实路径）

// ✅ 必须验证规范化路径仍在允许范围内
func safeJoin(base, sub string) (string, error) {
    target := filepath.Join(base, sub)
    resolved, err := filepath.EvalSymlinks(target)
    if err != nil {
        return "", fmt.Errorf("path error: %w", err)
    }
    absBase, _ := filepath.Abs(base)
    absResolved, _ := filepath.Abs(resolved)
    if !strings.HasPrefix(absResolved, absBase) {
        return "", fmt.Errorf("path escape attempt detected")
    }
    return resolved, nil
}
```

### 4.3 目录隔离策略

HotPlex 的会话目录隔离策略：

```go
// 会话工作目录：与主进程隔离
const sessionRoot = "/var/hotplex/sessions"

// 创建会话目录时
func createSessionDir(sessionID string) (string, error) {
    dir := filepath.Join(sessionRoot, sessionID)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return "", fmt.Errorf("create session dir: %w", err)
    }
    // 验证创建结果
    resolved, err := filepath.EvalSymlinks(dir)
    if err != nil || !strings.HasPrefix(resolved, sessionRoot) {
        return "", fmt.Errorf("path escape in session dir")
    }
    return dir, nil
}

// CLI 执行时限定工作目录
cmd.Dir = sessionDir  // 强制在 session 目录内
```

### 4.4 文件操作安全检查清单

| 检查项 | 实现 |
|--------|------|
| 路径规范化 | `filepath.Clean` + `filepath.EvalSymlinks` |
| 前缀验证 | 解析后路径必须以允许目录开头 |
| 文件类型检查 | `os.Stat()` 检查是否为 regular file |
| 符号链接检测 | `os.Lstat()` 检测 symlink（防止 link 攻击） |
| 权限检查 | 验证文件不属于其他用户或高权限 |
| 目录遍历限制 | 禁止 `..` 在白名单路径内出现 |

---

## 5. 推荐方案

### 5.1 HotPlex 当前安全状态评估

| 领域 | 当前状态 | 风险等级 |
|------|----------|----------|
| **命令注入** | 使用 `exec.Command` 参数化 ✅ | 低 |
| **Shell 调用** | 无 `sh -c` 拼接 ✅ | 低 |
| **超时控制** | `CommandContext` ✅ | 低 |
| **PGID 隔离** | `Setpgid: true` ✅ | 低 |
| **输入验证** | 部分字段有验证 ⚠️ | 中 |
| **白名单验证** | 无全局白名单 ⚠️ | 中 |
| **路径安全** | 依赖 `filepath.Join` ⚠️ | 中 |
| **环境变量** | `.env` 文件 ⚠️ | 中 |
| **Secret 管理** | 无动态 Secret ❌ | 高 |

### 5.2 推荐方案：分层防御体系

#### Layer 1：命令执行安全（高优先级）

```go
// internal/security/command.go
package security

import (
    "context"
    "fmt"
    "os/exec"
    "regexp"
)

var allowedCommands = map[string]bool{
    "git":    true,
    "docker": true,
}

var dangerousChars = regexp.MustCompile(`[;&|$`<>()!\\'"{}[\]]`)

// SafeExecute 参数化执行，只允许白名单命令
func SafeExecute(ctx context.Context, name string, args ...string) ([]byte, error) {
    if !allowedCommands[name] {
        return nil, fmt.Errorf("command %q not in allowlist", name)
    }
    // 纵深防御：参数检查
    for _, arg := range args {
        if dangerousChars.MatchString(arg) {
            return nil, fmt.Errorf("forbidden characters in argument")
        }
        if len(arg) > 4096 {
            return nil, fmt.Errorf("argument too long")
        }
    }
    ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Dir = "/var/hotplex/allowed"  // 固定工作目录
    return cmd.CombinedOutput()
}
```

#### Layer 2：输入验证框架（高优先级）

```go
// internal/security/validator.go
package security

import (
    "errors"
    "fmt"
    "regexp"
    "strings"
)

type Validator struct {
    rules map[string]*Rule
}

type Rule struct {
    MinLen, MaxLen int
    Pattern        *regexp.Regexp
    AllowedValues  []string
}

func (v *Validator) Validate(field, value string) error {
    rule, ok := v.rules[field]
    if !ok {
        return fmt.Errorf("no validation rule for %q", field)
    }
    n := len(value)
    if n < rule.MinLen || n > rule.MaxLen {
        return fmt.Errorf("%q length must be %d-%d, got %d", field, rule.MinLen, rule.MaxLen, n)
    }
    if rule.Pattern != nil && !rule.Pattern.MatchString(value) {
        return fmt.Errorf("%q does not match required pattern", field)
    }
    if len(rule.AllowedValues) > 0 {
        allowed := false
        for _, v := range rule.AllowedValues {
            if value == v {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("%q not in allowed values", field)
        }
    }
    return nil
}

// 预定义规则
var DefaultValidator = &Validator{
    rules: map[string]*Rule{
        "session_id": {MinLen: 8, MaxLen: 64, Pattern: regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)},
        "platform":   {MinLen: 1, MaxLen: 32, AllowedValues: []string{"slack", "telegram", "dingtalk"}},
        "message":    {MinLen: 1, MaxLen: 16000},
        "command":    {MinLen: 1, MaxLen: 128, Pattern: regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)},
    },
}
```

#### Layer 3：路径安全工具（中优先级）

```go
// internal/security/path.go
package security

import (
    "errors"
    "os"
    "path/filepath"
    "strings"
)

var ErrPathEscape = errors.New("path escape attempt detected")

// SafePathJoin 安全路径拼接 + 验证
func SafePathJoin(base, sub string) (string, error) {
    absBase, err := filepath.Abs(base)
    if err != nil {
        return "", err
    }
    // 规范化 + 解析符号链接
    target := filepath.Join(absBase, sub)
    resolved, err := filepath.EvalSymlinks(target)
    if err != nil && !errors.Is(err, os.ErrNotExist) {
        return "", err
    }
    // 验证路径未逃逸
    absResolved, _ := filepath.Abs(resolved)
    if !strings.HasPrefix(absResolved, absBase) {
        return "", ErrPathEscape
    }
    return resolved, nil
}
```

#### Layer 4：环境变量安全（中优先级）

```go
// internal/security/env.go
package security

import (
    "os"
    "strings"
)

// SanitizedEnv 构建干净的执行环境，不继承 host 变量
func SanitizedEnv(sessionID, workDir string) []string {
    return []string{
        "HOME=" + workDir,
        "LANG=en_US.UTF-8",
        "LC_ALL=en_US.UTF-8",
        "HOTPLEX_SESSION_ID=" + sessionID,
        // 不继承 PATH（防止 PATH 污染），使用绝对路径调用命令
    }
}

// MaskSecret 用于日志输出时脱敏
func MaskSecret(v string) string {
    if len(v) <= 8 {
        return "****"
    }
    return v[:4] + "****" + v[len(v)-4:]
}
```

#### Layer 5：Secret 管理（长期规划）

| 阶段 | 方案 | 成本 |
|------|------|------|
| **当前（v0.36）** | `.env` 文件 | 低 |
| **v0.40** | Vault Agent + Static Secrets | 中 |
| **v1.0** | Vault Dynamic Secrets | 高 |

### 5.3 验证框架集成到 HotPlex

```go
// internal/security/waf.go
package security

import (
    "github.com/hotplex/hotplex/internal/security"
    "github.com/hotplex/hotplex/types"
)

// CheckInput WAF 入口（现有 detector.go 增强）
func CheckInput(msg *types.ChatMessage) error {
    if err := security.DefaultValidator.Validate("platform", msg.Platform); err != nil {
        return err
    }
    if err := security.DefaultValidator.Validate("message", msg.Text); err != nil {
        return err
    }
    // 路径安全：验证会话目录
    safePath, err := security.SafePathJoin(sessionRoot, msg.SessionID)
    if err != nil {
        return err
    }
    _ = safePath
    return nil
}
```

---

## 6. 关键决策点

### 决策 1：命令白名单 vs 动态命令

| 选项 | 做法 | 推荐 |
|------|------|------|
| **白名单** | 只允许 `git`、`docker` 等固定命令 | ✅ HotPlex 采用 |
| **动态黑名单** | 允许所有命令，过滤危险参数 | ❌ OWASP 反对 |

HotPlex 是 AI Agent 控制平面，支持动态 AI 命令（Claude Code、OpenCode），但 AI 的输入必须经过 `detector.go` WAF 检查。

### 决策 2：路径验证时机

| 时机 | 做法 | 风险 |
|------|------|------|
| **Join 前验证** | 检查用户输入不含 `..` | ❌ 不够（`/a/b/../c` 可绕过） |
| **Clean 后验证** | `filepath.Clean` 后检查前缀 | ⚠️ 可用，但不解析 symlink |
| **EvalSymlinks 后验证** | 完全解析后验证前缀 | ✅ 推荐 |

### 决策 3：环境变量来源

| 来源 | 优点 | 缺点 |
|------|------|------|
| **Host 环境** | 无配置 | ❌ host 变量可能泄漏 |
| **.env 文件** | 隔离、版本可控 | ⚠️ 需确保不提交 git |
| **Vault Agent** | 动态轮换、集中管理 | ⚠️ 运维复杂度增加 |
| **K8s Secrets** | 云原生 | ⚠️ 仅 K8s 环境 |

### 决策 4：输入验证架构

OWASP 推荐 **集中验证**，不要分散到各个 handler：

```
用户输入 → [集中验证层] → 业务逻辑 → 数据库/命令执行
                  ↓
           统一的错误响应（不泄露内部细节）
```

---

## 参考来源

- [OWASP Command Injection Prevention](https://owasp.org/www-community/attacks/Command_Injection)
- [CWE-78: OS Command Injection](https://cwe.mitre.org/data/definitions/78.html)
- [OWASP Input Validation Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html)
- [OWASP Injection Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Injection_Prevention_Cheat_Sheet.html)
- [CWE-22: Path Traversal](https://cwe.mitre.org/data/definitions/22.html)
- [Go os/exec Package Documentation](https://pkg.go.dev/os/exec)
- [12-Factor App Config](https://12factor.net/config)
- [HashiCorp Vault Documentation](https://developer.hashicorp.com/vault/docs)
- [AWS Secrets Manager Best Practices](https://docs.aws.amazon.com/secretsmanager/latest/userguide/best-practices.html)
- [OWASP Go Secure Coding Practices Guide](https://github.com/OWASP/Go-SCP)
