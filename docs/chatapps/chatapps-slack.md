# Slack Adapter: 用户与开发者手册

HotPlex Slack 适配器允许用户通过 Slack 与 AI Agent 进行自然语言交互。本手册涵盖本地开发配置、生产环境部署、以及功能特性说明。

---

## 1. 快速开始 (Quick Start)

### 1.1 本地开发 (MacBook - 无需公网地址)

Slack **Socket Mode** 支持在防火墙内运行，无需 ngrok 或公网地址。

```bash
# 1. 配置环境变量 (.env)
CHATAPPS_ENABLED=true
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-token
SLACK_MODE=socket
SLACK_SERVER_ADDR=:8080

# 2. 启动 HotPlex
go run cmd/hotplexd/main.go

# 3. 在 Slack 中 @ 你的机器人开始对话
```

### 1.2 生产环境 (需要公网地址)

```bash
# 1. 配置环境变量 (.env)
CHATAPPS_ENABLED=true
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_MODE=http

# 2. 配置 ngrok
ngrok http 8080

# 3. 将 ngrok URL 配置到 Slack App 的 Event Subscriptions
```

---

## 2. Slack App 创建指南

### 2.1 创建 App

1. 访问 [https://api.slack.com/apps](https://api.slack.com/apps)
2. 点击 **Create New App** → **From scratch**
3. 输入 App 名称，选择目标 Workspace
4. 点击 **Create App**

### 2.2 Socket Mode 配置 (本地开发)

1. 进入 App 左侧菜单 → **Socket Mode**
2. 点击 **Enable Socket Mode**
3. 生成并保存 **App-Level Token** (`xapp-...`)
4. 在 **Event Subscriptions** 中订阅:
   - `app_mention` - @机器人时触发
   - `message.channels` - 频道消息
   - `message.groups` - 私群消息

### 2.3 HTTP Mode 配置 (生产环境)

1. 进入 App 左侧菜单 → **Event Subscriptions**
2. 点击 **Enable Events**
3. 输入公网可访问的 Request URL (如 `https://your-domain.com/webhook/events`)
4. 订阅相同的事件
5. 进入 **Install App** → **Install to Workspace**
6. 保存 **Bot User OAuth Token** (`xoxb-...`)
7. 在 **Basic Information** → **App Credentials** 中获取 **Signing Secret**

---

## 3. 配置详解

### 3.1 环境变量 (.env)

| 变量名 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `CHATAPPS_ENABLED` | ✅ | 启用 ChatApps | `true` |
| `SLACK_BOT_TOKEN` | ✅ | Bot Token (xoxb-) | `xoxb-xxx` |
| `SLACK_MODE` | ✅ | 连接模式 | `socket` / `http` |
| `SLACK_APP_TOKEN` | ⚠️ | App Token (Socket Mode) | `xapp-xxx` |
| `SLACK_SIGNING_SECRET` | ⚠️ | 签名密钥 (HTTP Mode) | `xxx` |
| `SLACK_SERVER_ADDR` | - | 服务地址 | `:8080` |

### 3.2 YAML 配置 (chatapps/configs/slack.yaml)

```yaml
platform: slack
provider:
  type: claude-code
  default_model: sonnet
  default_permission_mode: bypass-permissions

mode: ${SLACK_MODE:-http}
server_addr: ${SLACK_SERVER_ADDR:-:8080}

system_prompt: |
  你是一个 AI 助手，运行在 Slack 中。
  始终在线程中回复用户。

features:
  chunking:
    enabled: true      # 自动分片 >4000 字符
    max_chars: 4000
  threading:
    enabled: true      # 线程支持
  rate_limit:
    enabled: true      # 指数退避重试
    max_attempts: 3
  markdown:
    enabled: true      # Markdown 转 mrkdwn
```

---

## 4. 功能特性

### 4.1 Socket Mode (WebSocket)

- **低延迟**: 实时 WebSocket 连接，无需轮询
- **无需公网**: 防火墙内可直接运行
- **自动重连**: 断线自动重连 (指数退避，最多 5 次)

### 4.2 消息分片

Slack 单条消息限制 **4000 字符**。启用分片后:

- 自动分割超长消息
- 添加 `[1/N]` 编号
- 保持线程上下文

### 4.3 线程支持

- 自动解析 `thread_ts` 字段
- 在线程中回复用户
- 支持 `reply_broadcast`

### 4.4 Rate Limit 处理

- 429 错误自动重试
- 指数退避: 500ms → 1s → 2s → 4s
- 最大 3 次重试

### 4.5 Markdown 转换

自动转换 Markdown 到 Slack mrkdwn 格式:

| Markdown | mrkdwn |
|----------|---------|
| `**bold**` | `*bold*` |
| `*italic*` | `_italic_` |
| `[link](url)` | `<url\|text>` |
| `` `code` `` | `` `code` `` |

---

## 5. 架构说明

### 5.1 消息流程

```
用户 @机器人
    ↓
Slack Events API / WebSocket
    ↓
handleEvent / handleSocketModeEvent
    ↓
解析 thread_ts, channel_id
    ↓
GetOrCreateSession(channel:user)
    ↓
Webhook → Engine.Execute()
    ↓
AI Response → SendToChannel()
    ↓
Slack API / WebSocket
    ↓
用户收到回复
```

### 5.2 文件结构

```
chatapps/slack/
├── adapter.go      # 核心适配器
├── config.go      # 配置结构
├── socket_mode.go # WebSocket 连接管理
├── chunker.go     # 消息分片
├── retry.go      # 重试机制
├── sender.go     # 发送管道 (含 Markdown 转换)
└── config.yaml   # 配置示例
```

---

## 6. 安全考虑

### 6.1 签名验证 (HTTP Mode)

HTTP Mode 下启用签名验证:

```go
// adapter.go 中已实现
func (a *Adapter) verifySignature(body []byte, timestamp, signature string) bool
```

- 使用 HMAC-SHA256
- 时间戳 5 分钟内有效
- 常数时间比较防止 timing attack

### 6.2 会话隔离

- Session Key: `{channelId}:{userId}`
- 每个用户在每个频道有独立会话
- 30 分钟无活动自动清理

---

## 7. 常见问题

### Q1: 收不到消息回调

**检查清单:**
- [ ] Socket Mode: 确认 App Token 正确
- [ ] HTTP Mode: 确认 ngrok/公网 URL 可访问
- [ ] Event Subscriptions: 确认已订阅 `app_mention`
- [ ] Bot 已安装到 Workspace
- [ ] Bot 已添加到频道

### Q2: 消息发送失败

**可能原因:**
- Bot Token 过期 → 重新安装 App
- 权限不足 → 检查 OAuthScopes
- 超出速率限制 → 已启用自动重试

### Q3: Socket Mode 连接失败

**排查步骤:**
1. 确认 `SLACK_MODE=socket`
2. 确认 App Token 以 `xapp-` 开头
3. 确认 App 已启用 Socket Mode
4. 检查防火墙/代理是否阻止 WebSocket

### Q4: 消息被分片但格式混乱

**解决方案:**
- 禁用分片: `features.chunking.enabled: false`
- 或使用流式消息 (Draft Stream) - 后续版本支持

---

## 8. 开发参考

### 8.1 调试日志

```bash
# 启用详细日志
LOG_LEVEL=debug go run cmd/hotplexd/main.go
```

### 8.2 测试消息发送

```go
// 直接调用发送接口
adapter.SendToChannel(ctx, "C12345678", "Hello from HotPlex!", "")
```

### 8.3 自定义 Sender

```go
adapter.SetSender(func(ctx context.Context, sessionID string, msg *base.ChatMessage) error {
    // 自定义发送逻辑
    return adapter.SendToChannel(ctx, channelID, msg.Content, threadTS)
})
```

---

## 9. 相关资源

- [Slack API 文档](https://api.slack.com/)
- [Socket Mode 文档](https://api.slack.com/apis/socket-mode)
- [Slack Events API](https://api.slack.com/apis/connectors)
- [技术分析文档](./chatapps-slack-analysis.md)
- [ChatApps 架构指南](./chatapps-guide.md)
