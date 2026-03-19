# HotPlex 配置 Schema 验证方案设计

**日期**: 2026-03-17
**状态**: Draft
**作者**: Claude

---

## 1. 概述

为 HotPlex 所有配置文件添加运行时验证机制，支持多级别校验结果，提供友好的错误提示和最佳实践建议。

### 1.1 核心目标

- **快速失败**：启动时校验，错误提前暴露
- **友好提示**：指明错误字段、期望值、可选范围
- **渐进式校验**：FATAL/ERROR/WARNING/INFO 多级别
- **最佳实践**：通过警告引导用户优化配置

### 1.2 设计原则

- **方案 B**：手写校验函数，无外部依赖
- **严格但温和**：FATAL 阻止启动，WARNING 仅提示
- **向后兼容**：新增校验不影响现有正常配置

---

## 2. 校验结果级别

| 级别 | 影响 | 行为 |
|------|------|------|
| **FATAL** | 无法启动 | os.Exit(1)，无 --force 覆盖 |
| **ERROR** | 功能异常 | 默认阻止启动，提供 --force 标志覆盖 |
| **WARNING** | 最佳实践 | 打印建议，继续启动 |
| **INFO** | 信息提示 | 打印信息，继续启动 |

> **注意**：ERROR 级别不提供交互式确认（hotplexd 作为守护进程运行，stdin 不可用）

---

## 3. 规则清单

### 3.1 FATAL（致命 - 阻止启动）

| 配置 | 字段 | 规则 | 错误信息 |
|------|------|------|----------|
| Server | `engine.work_dir` | 目录存在或可创建 | `work_dir 不存在且无法创建: {path}` |
| Server | `server.port` | 有效端口 1-65535 或 Unix socket | `无效端口: {value}` |
| Server | `server.log_level` | debug/info/warn/error | `log_level 必须是 debug/info/warn/error，当前: {value}` |
| Server | allowed/disallowed | 互斥 | `allowed_tools 和 disallowed_tools 不能同时设置` |
| Slack | `bot_user_id` | 非空 | `bot_user_id 不能为空` |
| Slack | `bot_token` | 运行时校验 | `bot_token 未配置` |

### 3.2 ERROR（错误 - 功能异常）

| 配置 | 字段 | 规则 | 错误信息 |
|------|------|------|----------|
| Server | `engine.timeout` | > 0 | `timeout 必须大于 0` |
| Server | `engine.idle_timeout` | > 0 | `idle_timeout 必须大于 0` |
| Server | `security.permission_mode` | strict/bypassPermissions/空 | `permission_mode 必须是 strict、bypassPermissions 或空` |
| Server | `engine.timeout` | ≤ 24h | `timeout 不能超过 24 小时` |
| Server | `engine.idle_timeout` | ≤ 7d | `idle_timeout 不能超过 7 天` |
| Slack | `signing_secret` | 存在（已配置签名验证时）| `已启用签名验证但未配置 signing_secret` |

### 3.3 WARNING（警告 - 最佳实践）

| 配置 | 字段 | 规则 | 建议信息 |
|------|------|------|----------|
| Server | `security.api_key` | 长度 < 32 | 建议使用强 API Key（至少 32 字符）|
| Server | `server.log_level` | = debug（生产环境）| 建议生产环境使用 info 级别 |
| Server | `security.permission_mode` | 空或 bypassPermissions | 建议使用 strict 模式，更安全 |
| Slack | `allowed_tools` | 未限制工具 | 建议通过 allowed_tools 限制可用工具 |
| Slack | `bot_user_id` | 不以 U 开头 | Slack BotUserID 应以 U 开头（如 U1234567890）|
| Engine | `timeout` | < 30s | timeout 小于 30s 可能导致长任务失败 |
| Engine | `idle_timeout` | < 60s | idle_timeout 小于 60s 可能频繁创建新会话 |
| Storage | `message_store.path` | 目录不存在 | 建议确保消息存储目录存在 |

### 3.4 INFO（信息）

| 场景 | 内容 |
|------|------|
| 首次配置 | 检测到首次运行，打印快速入门提示 |
| 配置文件 | 显示最终生效的配置来源（继承链）|
| 继承链 | 显示配置继承路径（如 `configs/admin/slack.yaml → configs/base/slack.yaml`）|

### 3.5 继承链验证（FATAL）

| 规则 | 错误信息 |
|------|----------|
| `inherits` 指向的文件必须存在 | `继承文件不存在: {path}` |

---

## 4. 核心数据结构

### 4.1 验证结果

```go
// ValidationIssue 单个验证问题
type ValidationIssue struct {
    Level   IssueLevel   // FATAL/ERROR/WARNING/INFO
    Field   string       // 字段路径，如 "server.log_level"
    Message string       // 错误/建议信息
    Value   any          // 当前值（可选）
}

// IssueLevel 验证级别
type IssueLevel int

const (
    LevelFatal IssueLevel = iota
    LevelError
    LevelWarning
    LevelInfo
)

// ValidationResult 验证结果
type ValidationResult struct {
    Issues []ValidationIssue
    // 辅助方法
    HasFatal() bool
    HasError() bool
    FatalCount() int
    ErrorCount() int
    WarningCount() int
}
```

### 4.2 配置结构

```go
// Validator 定义配置验证接口
type Validator interface {
    Validate() *ValidationResult
}

// ServerConfig 验证方法
func (c *ServerConfig) Validate() *ValidationResult

// SlackConfig 验证方法
func (c *SlackConfig) Validate() *ValidationResult

// 编译时接口验证（遵循 Uber Go Style Guide）
var _ Validator = (*ServerConfig)(nil)
var _ Validator = (*SlackConfig)(nil)
var _ Validator = (*FeishuConfig)(nil)
```

---

## 5. 校验流程

```
        开始校验
           │
           ▼
    ┌──────────────┐
    │  FATAL 数量  │──→ os.Exit(1)
    │   > 0 ?      │
    └──────────────┘
           │ No
           ▼
    ┌──────────────┐
    │  ERROR 数量  │──→ 检查 --force 标志
    │   > 0 ?      │     无 --force → os.Exit(1)
    └──────────────┘     有 --force → 继续
           │
           ▼
    打印 WARNING + INFO
           │
           ▼
      启动服务
```

> **--force 标志**：允许在 ERROR 级别下强制启动，用于某些配置问题已知但可接受的场景。

---

## 6. 输出示例

```bash
🔴 配置验证失败

[FATAL] server.port: 无效端口 "99999" (有效范围: 1-65535)
[FATAL] engine.allowed_tools 与 engine.disallowed_tools 不能同时设置

[WARNING] server.api_key: Key 长度 16，建议使用至少 32 字符的强 Key
[WARNING] server.log_level: 生产环境建议使用 "info" 而非 "debug"
[WARNING] engine.timeout: 10s 可能不足以处理复杂任务，建议 >= 30s

[INFO] 检测到配置继承: configs/admin/slack.yaml → configs/base/slack.yaml
[INFO] 首次运行? 参见文档: https://hotplex.dev/docs/quick-start

❌ 启动失败 (2 个致命错误)
```

---

## 7. 文件变更

```
internal/config/
├── server_config.go      # 新增 Validate() 方法
├── slack_config.go       # 新增 Validate() 方法
├── feishu_config.go     # 新增 Validate() 方法
├── validator.go         # 新增: ValidationResult 类型和公共逻辑
├── validator_test.go    # 新增: 单元测试
└── README.md            # 更新: 文档说明
```

---

## 8. 与现有系统集成

### 8.1 启动流程

```go
// cmd/hotplexd/main.go
var forceStart = flag.Bool("force", false, "Force start even with ERROR level issues")

func main() {
    flag.Parse()

    // 1. 加载配置
    serverCfg, err := config.NewServerLoader(configPath, logger)

    // 2. 验证配置（新增）
    result := serverCfg.Validate()

    // 3. FATAL 级别直接退出
    if result.HasFatal() {
        result.PrintFATAL(os.Stderr)
        os.Exit(1)
    }

    // 4. ERROR 级别：检查 --force 标志
    if result.HasError() && !*forceStart {
        result.PrintERROR(os.Stderr)
        fmt.Fprintf(os.Stderr, "\n使用 --force 强制启动（风险自担）\n")
        os.Exit(1)
    }

    // 5. 打印 WARNING 和 INFO
    result.PrintWARNINGAndINFO(os.Stdout)

    // 6. 启动服务
    // ...
}
```

### 8.2 热更新集成

```go
// HotReloadableConfig 接口
type HotReloadableConfig interface {
    HotReloadFields() []string
    ValidateReload(field string, value any) *ValidationResult
}
```

---

## 9. 实施计划

| 阶段 | 任务 | 文件 |
|------|------|------|
| 1 | 创建 validator.go 定义数据结构 | internal/config/validator.go |
| 2 | 实现 ServerConfig.Validate() | internal/config/server_config.go |
| 3 | 实现 SlackConfig.Validate() | internal/config/slack_config.go |
| 4 | 集成到启动流程 | cmd/hotplexd/main.go |
| 5 | 添加单元测试 | internal/config/*_test.go |
| 6 | 更新文档 | internal/config/README.md |

---

## 10. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 校验规则过于严格 | 现有配置无法启动 | ERROR 级别提供交互式确认 |
| 性能影响 | 启动变慢 | 校验逻辑简单，预期 < 10ms |
| 规则遗漏 | 关键问题未检测 | 持续根据实际案例补充规则 |

---

## 11. 参考资料

- [HotPlex CLAUDE.md](../../CLAUDE.md)
- [统一配置方案设计](./2026-03-16-unified-config-design.md)
- [Uber Go Style Guide](../rules/uber-go-style-guide.md)
