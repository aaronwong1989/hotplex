# HotPlex 配置参考

> 完整的配置选项参考。
> 快速开始请参阅 [development_zh.md](development_zh.md)。
>
> **中文版** | **[English](configuration.md)**

## 目录

- [配置层级](#配置层级)
- [环境变量](#环境变量)
- [YAML 配置](#yaml-配置)
- [平台特定配置](#平台特定配置)
- [示例](#示例)

---

## 配置层级

HotPlex 使用分层配置系统，优先级从高到低：

```
1. 命令行参数     (--config, --env-file, --admin-port)
2. 环境变量       (HOTPLEX_*)
3. YAML 配置文件  (configs/base/*.yaml)
4. 默认值         (内置默认值)
```

### 各层用途

| 层级          | 内容                                    |
| ------------- | --------------------------------------- |
| **`.env`**    | 全局参数、bot 凭证、密钥、持久化配置    |
| **YAML 文件** | 平台行为、权限策略、功能开关、AI 提示词 |

---

## 配置继承 (v0.33.0+)

HotPlex v0.33.0+ 引入了统一配置系统，支持配置继承。

### 继承字段

```yaml
# configs/instances/bot-01/slack.yaml
inherits: ../../base/slack.yaml

# 仅覆盖需要自定义的字段
engine:
  work_dir: ~/projects/bot-01
```

### 相对路径解析

- 继承路径相对于当前配置文件位置
- 支持多级继承链
- 自动检测循环继承并报错

### 深度合并

嵌套配置对象进行深度合并，而非完全覆盖：

```yaml
# base.yaml
provider:
  type: claude-code
  default_model: sonnet
  allowed_tools:
    - Read
    - Edit

# instance.yaml (inherits base.yaml)
provider:
  default_model: opus
  allowed_tools:
    - Grep

# 结果：model=opus, tools=[Grep] (合并 allowed_tools)
```

### 实例隔离

多机器人部署时，每个实例可拥有独立配置目录：

```
configs/
├── base/           # 基础配置 (SSOT)
│   ├── slack.yaml
│   └── feishu.yaml
├── admin/          # Admin 机器人覆盖配置
│   └── slack.yaml
├── templates/      # 角色模板
│   └── roles/
│       ├── go.yaml
│       ├── frontend.yaml
│       └── devops.yaml
└── instances/      # 各实例独立配置
    ├── bot-01/
    │   └── slack.yaml
    └── bot-02/
        └── slack.yaml
```

> **注意**：`configs/chatapps/` 目录已废弃。请使用 `configs/base/` 作为所有基础配置。

---

## 角色模板 (v0.33.0+)

预定义的角色配置模板，位于 `configs/templates/roles/`：

| 模板 | 用途 |
| ---- | ---- |
| `go.yaml` | Go 开发 |
| `frontend.yaml` | 前端开发 |
| `devops.yaml` | DevOps 任务 |
| `custom.yaml` | 自定义角色 |

### 使用方式

1. **复制使用**：复制模板到目标位置后修改
2. **内联引用**：在配置中直接引用模板

```yaml
# 使用模板
inherits: ../templates/roles/go.yaml
```

---

## Admin 机器人配置 (v0.33.0+)

Admin 机器人配置位于 `configs/admin/`，继承自 `configs/base/`：

```yaml
# configs/admin/slack.yaml
inherits: ../base/slack.yaml

# Admin 机器人特定配置
assistant:
  bot_user_id: ${HOTPLEX_ADMIN_BOT_USER_ID}
  skills:
    - admin
    - code-review
    - security-audit
```

---

## 环境变量

### 核心服务器

| 变量                 | 默认值   | 描述                               |
| -------------------- | -------- | ---------------------------------- |
| `HOTPLEX_PORT`       | `8080`   | 服务器监听端口                     |
| `HOTPLEX_LOG_LEVEL`  | `INFO`   | 日志级别：DEBUG, INFO, WARN, ERROR |
| `HOTPLEX_LOG_FORMAT` | `json`   | 日志格式：json, text               |
| `HOTPLEX_API_KEY`    | *(必填)* | API 安全令牌                       |
| `HOTPLEX_API_KEYS`   | *(可选)* | 多个 API 密钥（逗号分隔）          |

### 引擎

| 变量                        | 默认值 | 描述                |
| --------------------------- | ------ | ------------------- |
| `HOTPLEX_EXECUTION_TIMEOUT` | `30m`  | AI 响应最大等待时间 |
| `HOTPLEX_IDLE_TIMEOUT`      | `1h`   | 会话空闲超时        |

### Provider

| 变量                                            | 默认值        | 描述                            |
| ----------------------------------------------- | ------------- | ------------------------------- |
| `HOTPLEX_PROVIDER_TYPE`                         | `claude-code` | Provider：claude-code, opencode |
| `HOTPLEX_PROVIDER_MODEL`                        | `sonnet`      | 默认模型：sonnet, haiku, opus   |
| `HOTPLEX_PROVIDER_BINARY`                       | *(自动检测)*  | CLI 二进制路径                  |
| `HOTPLEX_PROVIDER_DANGEROUSLY_SKIP_PERMISSIONS` | `false`       | 跳过所有权限检查                |
| `HOTPLEX_OPENCODE_COMPAT_ENABLED`               | `true`        | 启用 OpenCode HTTP API 兼容     |

### 项目目录 (Docker)

| 变量                   | 描述                          |
| ---------------------- | ----------------------------- |
| `HOTPLEX_PROJECTS_DIR` | 项目工作空间目录              |
| `HOTPLEX_GITCONFIG`    | git 配置路径（用于 bot 身份） |

### Native Brain（可选）

| 变量                              | 默认值        | 描述                                 |
| --------------------------------- | ------------- | ------------------------------------ |
| `HOTPLEX_BRAIN_API_KEY`           | *(未设置)*    | Brain API 密钥（设置后启用）         |
| `HOTPLEX_BRAIN_PROVIDER`          | `openai`      | Brain provider：openai, anthropic 等 |
| `HOTPLEX_BRAIN_MODEL`             | `gpt-4o-mini` | Brain 模型                           |
| `HOTPLEX_BRAIN_ENDPOINT`          | *(可选)*      | 自定义 API 端点                      |
| `HOTPLEX_BRAIN_TIMEOUT_S`         | `10`          | 请求超时（秒）                       |
| `HOTPLEX_BRAIN_CACHE_SIZE`        | `1000`        | 缓存大小                             |
| `HOTPLEX_BRAIN_MAX_RETRIES`       | `3`           | 最大重试次数                         |
| `HOTPLEX_BRAIN_RETRY_MIN_WAIT_MS` | `100`         | 最小重试等待                         |
| `HOTPLEX_BRAIN_RETRY_MAX_WAIT_MS` | `5000`        | 最大重试等待                         |

### 消息存储

| 变量                                       | 默认值                           | 描述                           |
| ------------------------------------------ | -------------------------------- | ------------------------------ |
| `HOTPLEX_MESSAGE_STORE_ENABLED`            | `true`                           | 启用消息持久化                 |
| `HOTPLEX_MESSAGE_STORE_TYPE`               | `sqlite`                         | 存储：sqlite, postgres, memory |
| `HOTPLEX_MESSAGE_STORE_SQLITE_PATH`        | `~/.config/hotplex/chatapp_messages.db` | SQLite 数据库路径              |
| `HOTPLEX_MESSAGE_STORE_SQLITE_MAX_SIZE_MB` | `1024`                           | 最大数据库大小                 |
| `HOTPLEX_MESSAGE_STORE_STREAMING_ENABLED`  | `true`                           | 启用流式存储                   |
| `HOTPLEX_MESSAGE_STORE_STREAMING_TIMEOUT`  | `5m`                             | 流式超时                       |

### CORS

| 变量                      | 描述                 |
| ------------------------- | -------------------- |
| `HOTPLEX_ALLOWED_ORIGINS` | 允许的源（逗号分隔） |

---

## 平台凭证

### Slack

| 变量                           | 必填        | 描述                      |
| ------------------------------ | ----------- | ------------------------- |
| `HOTPLEX_SLACK_PRIMARY_OWNER`  | **是**      | 主要所有者的 Slack 用户 ID |
| `HOTPLEX_SLACK_BOT_USER_ID`    | **是**      | Bot 用户 ID (UXXXXXXXXXX)   |
| `HOTPLEX_SLACK_BOT_TOKEN`      | **是**      | Bot Token (xoxb-...)        |
| `HOTPLEX_SLACK_APP_TOKEN`      | Socket Mode | App Token (xapp-...)        |
| `HOTPLEX_SLACK_SIGNING_SECRET` | HTTP Mode   | 签名验证密钥                |

### 飞书

| 变量                                | 描述       |
| ----------------------------------- | ---------- |
| `HOTPLEX_FEISHU_APP_ID`             | App ID     |
| `HOTPLEX_FEISHU_APP_SECRET`         | App secret |
| `HOTPLEX_FEISHU_VERIFICATION_TOKEN` | 验证 token |
| `HOTPLEX_FEISHU_ENCRYPT_KEY`        | 加密 key   |

---

## YAML 配置

### 结构

```yaml
# configs/base/slack.yaml

# [必填] 平台标识
platform: slack

# Provider 设置
provider:
  type: claude-code
  enabled: true
  default_model: sonnet
  default_permission_mode: bypassPermissions

# 引擎设置
engine:
  work_dir: ~/projects/myproject
  timeout: 30m
  idle_timeout: 1h

# 会话生命周期
session:
  timeout: 1h
  cleanup_interval: 5m

# 连接模式
mode: socket  # 或 "http"
server_addr: :8080

# AI 行为
system_prompt: |
  你是一个有帮助的助手...

# 功能开关
features:
  chunking:
    enabled: true
    max_chars: 4000
  threading:
    enabled: true

# 安全
security:
  verify_signature: true
  permission:
    dm_policy: allow
    group_policy: mention
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

### Provider 部分

| 字段                           | 描述                                                                        |
| ------------------------------ | --------------------------------------------------------------------------- |
| `type`                         | Provider 类型：`claude-code`, `opencode`                                    |
| `enabled`                      | 启用/禁用 provider                                                          |
| `default_model`                | 默认模型 ID                                                                 |
| `default_permission_mode`      | 权限模式：`bypassPermissions`, `acceptEdits`, `default`, `dontAsk`, `plan` |
| `dangerously_skip_permissions` | 跳过所有权限检查（Docker/CI）                                               |
| `binary_path`                  | 自定义二进制路径                                                            |
| `allowed_tools`                | 工具白名单                                                                  |
| `disallowed_tools`             | 工具黑名单                                                                  |

### Engine 部分

| 字段               | 描述             |
| ------------------ | ---------------- |
| `work_dir`         | Agent 工作目录   |
| `timeout`          | 最大执行时间     |
| `idle_timeout`     | 会话空闲超时     |
| `allowed_tools`    | 引擎级工具白名单 |
| `disallowed_tools` | 引擎级工具黑名单 |

### Features 部分

| 功能                       | 描述                          |
| -------------------------- | ----------------------------- |
| `chunking.enabled`         | 分割长消息                    |
| `chunking.max_chars`       | 每块最大字符数（Slack: 4000） |
| `threading.enabled`        | 在线程中回复                  |
| `rate_limit.enabled`       | 启用速率限制处理              |
| `rate_limit.max_attempts`  | 最大重试次数                  |
| `rate_limit.base_delay_ms` | 初始重试延迟                  |
| `rate_limit.max_delay_ms`  | 最大重试延迟                  |
| `markdown.enabled`         | 转换 Markdown 为平台格式      |

### Security 部分

| 字段                                  | 描述                                              |
| ------------------------------------- | ------------------------------------------------- |
| `verify_signature`                    | 验证平台签名（HTTP 模式）                         |
| `permission.dm_policy`                | 私聊策略：`allow`, `pairing`, `block`             |
| `permission.group_policy`             | 群聊策略：`allow`, `mention`, `multibot`, `block` |
| `permission.bot_user_id`              | Bot 用户 ID（必填）                               |
| `permission.allowed_users`            | 用户白名单                                        |
| `permission.blocked_users`            | 用户黑名单                                        |
| `permission.slash_command_rate_limit` | 每用户速率限制                                    |

---

## 平台特定配置

### Slack（Socket 模式）

```yaml
platform: slack
mode: socket  # 开发环境推荐

provider:
  type: claude-code

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    group_policy: multibot

features:
  chunking:
    enabled: true
    max_chars: 4000
  threading:
    enabled: true
```

### Slack（HTTP/Webhook 模式）

```yaml
platform: slack
mode: http
server_addr: :8080

security:
  verify_signature: true
```

---

## 示例

### 最小 Slack 配置

```yaml
platform: slack
mode: socket

provider:
  type: claude-code

engine:
  work_dir: ~/projects/myproject

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

### 多 Bot 配置

```yaml
platform: slack
mode: socket

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    group_policy: multibot  # 多 bot 关键设置
    broadcast_response: |
      请 @mention 我来获取帮助。
```

### Docker 生产配置

```yaml
platform: slack

provider:
  type: claude-code
  dangerously_skip_permissions: true  # 容器化环境

engine:
  work_dir: /app/workspace
  timeout: 30m
  idle_timeout: 2h

features:
  chunking:
    enabled: true
  rate_limit:
    enabled: true
    max_attempts: 5
```

---

## 相关文档

- [development_zh.md](development_zh.md) - 开发指南
- [architecture_zh.md](architecture_zh.md) - 架构概览
- [docker-deployment_zh.md](docker-deployment_zh.md) - Docker 部署
- [chatapps/slack-setup-beginner_zh.md](chatapps/slack-setup-beginner_zh.md) - Slack 设置指南
