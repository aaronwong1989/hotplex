# Interactive Permission UI - Design Specification

> **Issue**: #321 - Claude Code Permission Protocol
> **Status**: Approved
> **Date**: 2026-03-20

## 背景

Claude Code CLI (v2.1.78) 不支持 `--permission-prompt-tool stdio` 参数，无法通过 stdin 实现原生的 `control_request/control_response` 双向权限协议。

本方案采用**输入层权限控制**替代 CLI 协议层权限控制，结合 Interactive UI 实现权限管理的用户体验。

## 决策汇总

| # | 问题 | 决策 |
|---|------|------|
| 1 | 交互入口 | 卡片按钮 |
| 2 | 触发时机 | WAF 命中 + CLI permission_denials |
| 3 | 按钮操作 | Allow Once / Allow Always / Deny Once / Deny All |
| 4 | 生效范围 | Bot 级别，跨会话持久化 |
| 5 | 卡片更新 | 更新原卡片 |
| 6 | 存储方式 | 内存 + 文件双写 |
| 7 | Pattern 格式 | 工具名 + 命令（如 `Bash:rm.*-rf`） |
| 8 | 文件格式 | JSON |
| 9 | Pattern 管理 | 分离存储 + 分层匹配 |
| 10 | 多 Bot | 当前 Bot 专属 |
| 11 | 文件路径 | `~/.hotplex/instances/{bot_id}/permissions.json` |
| 12 | 过期机制 | 永不过期 |
| 13 | Allow Always 机制 | 更新 `--allowed-tools` |
| 14 | 更新生效 | 立即生效（热更新） |

## 架构图

```
用户输入
    │
    ▼
┌─────────────────────┐
│ PermissionMatcher   │
│                     │
│ 1. WAF Pattern?   │──命中──→ 拦截 → [Block] 卡片
│                     │
│ 2. Whitelist?      │──命中──→ 放行 → 继续执行
│                     │
│ 3. Blacklist?     │──命中──→ 拦截 → [Deny] 卡片
│                     │
│ 4. 无匹配          │──→ 正常放行
└─────────────────────┘
    │
    │ CLI 执行后
    ▼
┌─────────────────────┐
│ permission_denials  │
│ (来自 result 事件) │
└─────────────────────┘
    │ 有拒绝记录
    ▼
权限只读卡片（无操作按钮）
```

## 文件结构

```
chatapps/
├── base/
│   └── permission.go              # 公共接口定义
├── slack/
│   ├── permission_card.go         # Slack 权限卡片
│   └── interactive.go            # (扩展) 权限按钮处理
└── feishu/
    ├── permission_card.go        # 飞书权限卡片
    └── interactive_handler.go    # (扩展) 权限按钮处理

internal/
└── permission/
    ├── store.go                # PermissionStore (内存 + JSON)
    ├── matcher.go              # PermissionMatcher
    └── types.go                # 数据结构定义
```

## 数据结构

### permissions.json

```json
{
  "bot_id": "U0AHRCL1KCM",
  "whitelist": [
    {
      "pattern": "Bash:rm.*-rf",
      "created_at": "2026-03-20T10:00:00Z",
      "created_by": "user123"
    }
  ],
  "blacklist": [
    {
      "pattern": "Bash:chmod 777",
      "created_at": "2026-03-20T10:00:00Z",
      "created_by": "user123"
    }
  ]
}
```

### Pattern 格式

Pattern 格式：`{ToolName}:{CommandPattern}`

- 工具名：Claude Code 工具名（如 `Bash`, `Edit`, `Write`, `Read`）
- 命令：正则表达式匹配工具输入
- 示例：`Bash:rm.*-rf`, `Bash:wget.*`, `Edit:.*/etc/.*`

## 核心接口

### PermissionStore

```go
type PermissionStore interface {
    // 加载持久化配置
    Load(botID string) error
    // 保存到文件
    Save(botID string) error

    // Pattern 管理
    AddWhitelist(botID, pattern string) error
    AddBlacklist(botID, pattern string) error
    RemoveWhitelist(botID, pattern string) error
    RemoveBlacklist(botID, pattern string) error

    // 查询
    GetWhitelist(botID) []string
    GetBlacklist(botID) []string
    IsAllowed(botID, tool, command string) (bool, string)
}
```

### PermissionMatcher

```go
type PermissionMatcher struct {
    wafPatterns []string
    stores      sync.Map  // botID → PermissionStore
}

type Decision int
const (
    DecisionAllow Decision = iota
    DecisionDeny
    DecisionBlocked
    DecisionUnknown
)

func (m *PermissionMatcher) Check(botID, tool, command string) Decision
```

## 卡片设计

### 触发卡片（有操作按钮）

```
┌─────────────────────────────────────────┐
│ 🚨 权限请求                              │
├─────────────────────────────────────────┤
│ 工具: Bash                              │
│ 命令: rm -rf /tmp/test                   │
│                                         │
│ Session: abc123                         │
├─────────────────────────────────────────┤
│ [✅ Allow Once] [🔒 Allow Always]        │
│ [🚫 Deny Once] [⛔ Deny All]           │
└─────────────────────────────────────────┘
```

### 结果卡片（更新后）

```
┌─────────────────────────────────────────┐
│ ✅ 已允许（本次）                        │
├─────────────────────────────────────────┤
│ 工具: Bash                              │
│ 命令: rm -rf /tmp/test                   │
│ 决策时间: 2026-03-20 10:30:00          │
└─────────────────────────────────────────┘
```

### CLI 拒绝卡片（只读）

```
┌─────────────────────────────────────────┐
│ ⚠️ 权限被拒绝（CLI）                     │
├─────────────────────────────────────────┤
│ 工具: Bash                              │
│ 命令: chmod 777 /etc/passwd             │
│                                         │
│ 原因: 用户拒绝了此操作                  │
│ 请联系管理员调整权限配置                  │
└─────────────────────────────────────────┘
```

## 按钮 Action 映射

| 按钮 | Action ID | 处理逻辑 |
|------|-----------|---------|
| Allow Once | `perm_allow_once` | 放行本次（无持久化） |
| Allow Always | `perm_allow_always` | 添加到 Whitelist + 更新 AllowedTools |
| Deny Once | `perm_deny_once` | 拦截本次（无持久化） |
| Deny All | `perm_deny_all` | 添加到 Blacklist |

## 热更新流程

```
用户点击 [Allow Always]
    │
    ▼
InteractiveHandler.handlePermission()
    │
    ▼
PermissionStore.AddWhitelist(botID, pattern)
    │
    ├─→ 写入内存
    │
    └─→ 保存到 ~/.hotplex/instances/{botID}/permissions.json
    │
    ▼
Engine.SetAllowedTools(tools)
    │
    ▼
当前会话 + 后续会话立即生效
```

## 影响范围

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/permission/types.go` | 数据结构定义 |
| `internal/permission/store.go` | PermissionStore 实现 |
| `internal/permission/matcher.go` | PermissionMatcher 实现 |
| `chatapps/slack/permission_card.go` | Slack 权限卡片 |
| `chatapps/feishu/permission_card.go` | 飞书权限卡片 |

### 修改文件

| 文件 | 修改内容 |
|------|---------|
| `chatapps/slack/interactive.go` | 扩展权限按钮处理 |
| `chatapps/feishu/interactive_handler.go` | 扩展权限按钮处理 |
| `chatapps/base/adapter.go` | 集成 PermissionMatcher |
| `chatapps/manager.go` | Bot 级别 PermissionStore 管理 |

## 存储路径

权限 Pattern 文件存储在 Bot 实例目录下：

```
~/.hotplex/instances/{bot_id}/permissions.json
```

与现有 Bot 实例目录结构保持一致。
