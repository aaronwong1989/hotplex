# Slack 聊天适配器示例

本示例演示如何使用 HotPlex Slack 适配器创建聊天机器人，并与 HotPlex AI 引擎集成。

## 前置条件

### 1. Slack 应用配置

1. 在 https://api.slack.com/apps 创建 Slack 应用
2. 启用以下 OAuth 作用域：
   - `chat:write`
   - `channels:read`
   - `groups:read`
   - `im:read`
   - `mpim:read`
   - `app_mentions:read`
3. 如需 Socket 模式：
   - 在应用设置中启用 Socket Mode
   - 生成应用令牌 (`xapp-...`)

### 2. 环境变量

```bash
export HOTPLEX_SLACK_BOT_TOKEN=xoxb-...  # 必填
export HOTPLEX_SLACK_APP_TOKEN=xapp-...   # Socket 模式必填
export HOTPLEX_SLACK_SIGNING_SECRET=...   # HTTP 模式必填
export HOTPLEX_SLACK_SERVER_ADDR=:8080    # 可选，默认 :8080
export HOTPLEX_SLACK_MODE=http             # 可选: "http" 或 "socket"
export HOTPLEX_SLACK_BOT_USER_ID=U...     # 可选: 机器人用户 ID
```

## 快速开始

```bash
cd _examples/chatapps_slack
go run main.go
```

## 运行模式

### HTTP 模式（默认）

- 使用 Webhook 接收事件
- 需要配置 `SigningSecret`
- 事件到达 `/events` 端点

### Socket 模式

- 使用 WebSocket 连接实现实时事件
- 需要配置 `AppToken`
- 更适合高流量应用

## 权限策略

可以在配置中设置权限策略：

```go
config := &slack.Config{
    // 私信策略: "allow", "pairing", "block"
    DMPolicy: "allow",

    // 群组消息策略: "allow", "mention", "multibot", "block"
    GroupPolicy: "allow",

    // 白名单用户
    AllowedUsers: []string{"U1234567890"},

    // 黑名单用户
    BlockedUsers: []string{"U0987654321"},
}
```

### 策略说明

| 策略 | 说明 |
|------|------|
| `allow` | 允许所有消息 |
| `mention` | 仅当提及机器人时响应 |
| `multibot` | 多机器人路由：无 @ 时广播，有 @ 时仅响应 |
| `pairing` | 仅当用户已配对时允许 |
| `block` | 阻止所有消息 |

## 与 HotPlex 引擎集成

修改消息处理器以集成 HotPlex 引擎：

```go
adapter.SetHandler(func(ctx context.Context, msg *chatapps.ChatMessage) error {
    // 创建 HotPlex 引擎
    engine, err := hotplex.NewEngine(hotplex.EngineOptions{
        Namespace:        "slack_bot",
        BaseSystemPrompt: "You are a helpful assistant in Slack.",
        // 其他配置...
    })
    if err != nil {
        return err
    }
    defer engine.Close()

    // 执行提示词
    cfg := &hotplex.Config{
        SessionID:        msg.SessionID,
        WorkDir:          "/tmp",
        TaskInstructions: "Keep responses concise for Slack.",
    }

    return engine.Execute(ctx, cfg, msg.Content, func(eventType string, data any) error {
        // 将流式事件发送回 Slack
        // ...
        return nil
    })
})
```

### 系统提示词注入

在 Slack 适配器中，可以通过两种方式配置系统提示词：

1. **YAML 配置文件**：`chatapps/configs/slack.yaml` 中的 `system_prompt` 和 `task_instructions` 字段
2. **代码配置**：`EngineOptions.BaseSystemPrompt` 和 `Config.TaskInstructions`

详细说明请参考 [Claude 生命周期示例](../go_claude_lifecycle)。

## 端点说明

| 端点 | 说明 |
|------|------|
| `/events` | Slack Events API Webhook |
| `/interactive` | 交互组件回调 |
| `/slash` | 斜杠命令处理器 |
| `/health` | 健康检查 |

## 代码结构

### 1. 初始化适配器

```go
config := &slack.Config{
    BotToken:      os.Getenv("HOTPLEX_SLACK_BOT_TOKEN"),
    AppToken:      os.Getenv("HOTPLEX_SLACK_APP_TOKEN"),
    SigningSecret: os.Getenv("HOTPLEX_SLACK_SIGNING_SECRET"),
    Mode:          "http",
    ServerAddr:    ":8080",
}

adapter := slack.NewAdapter(config, logger)
```

### 2. 设置消息处理器

```go
adapter.SetHandler(func(ctx context.Context, msg *chatapps.ChatMessage) error {
    // 处理消息...
    return nil
})
```

### 3. 启动适配器

```go
if err := adapter.Start(context.Background()); err != nil {
    log.Fatal(err)
}
```

## ChatMessage 结构

```go
type ChatMessage struct {
    Platform   string            // 平台标识 "slack"
    SessionID  string            // 会话标识符
    ChannelID string            // 频道 ID
    UserID    string            // 用户 ID
    Content   string            // 消息内容
    Timestamp string            // 时间戳
    Metadata  map[string]any    // 额外元数据
}
```

## 运行要求

- Go 1.25+
- 有效的 Slack 应用配置

## 扩展阅读

- [Slack 适配器源码](../../chatapps/slack)
- [飞书适配器示例](../chatapps_feishu)
- [ChatApps 架构文档](../../chatapps/README_zh.md)
