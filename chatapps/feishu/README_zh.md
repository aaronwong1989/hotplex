# Feishu (Lark) Adapter for HotPlex

飞书（Lark）适配器，为 HotPlex 提供中国企业 IM 集成能力。

**状态**: ✅ Phase 3 生产就绪  
**测试覆盖率**: 50.4%  
**最后更新**: 2026-03-03

---

## 📖 目录

- [快速开始](#-快速开始)
- [配置说明](#-配置说明)
- [飞书开发者后台配置](#-飞书开发者后台配置)
- [核心功能](#-核心功能)
- [API 参考](#-api-参考)
- [错误处理](#-错误处理)
- [测试指南](#-测试指南)
- [常见问题](#-常见问题)
- [参考文档](#-参考文档)

---

## 🚀 快速开始

### 1. 配置环境变量

```bash
# 必填：飞书应用凭证
export HOTPLEX_FEISHU_APP_ID=cli_a1b2c3d4e5f6g7h8
export HOTPLEX_FEISHU_APP_SECRET=xxxxxxxxxxxxxxxx
export HOTPLEX_FEISHU_VERIFICATION_TOKEN=xxxxxxxx
export HOTPLEX_FEISHU_ENCRYPT_KEY=xxxxxxxxxxxxxxxx

# 可选：服务器配置
export HOTPLEX_FEISHU_SERVER_ADDR=:8082
export FEISHU_MAX_MESSAGE_LEN=4096
```

### 2. 创建适配器实例

```go
import (
    "context"
    "log"
    "os"
    
    "github.com/hrygo/hotplex/chatapps/feishu"
    "github.com/hrygo/hotplex/chatapps/base"
)

func main() {
    ctx := context.Background()
    logger := base.NewLogger()
    
    config := &feishu.Config{
        AppID:             os.Getenv("HOTPLEX_FEISHU_APP_ID"),
        AppSecret:         os.Getenv("HOTPLEX_FEISHU_APP_SECRET"),
        VerificationToken: os.Getenv("HOTPLEX_FEISHU_VERIFICATION_TOKEN"),
        EncryptKey:        os.Getenv("HOTPLEX_FEISHU_ENCRYPT_KEY"),
        ServerAddr:        os.Getenv("HOTPLEX_FEISHU_SERVER_ADDR"),
        MaxMessageLen:     4096,
    }
    
    adapter, err := feishu.NewAdapter(config, logger)
    if err != nil {
        log.Fatal(err)
    }
    
    // 设置消息处理器
    adapter.SetHandler(myHandler)
    
    // 启动适配器
    if err := adapter.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### 3. 验证部署

```bash
# 检查端点可达性
curl -X POST https://your-domain.com/feishu/events \
  -H "Content-Type: application/json" \
  -d '{"challenge": "test"}'

# 预期响应：返回 challenge 值
```

---

## ⚙️ 配置说明

### 必填配置

| 配置项 | 环境变量 | 说明 | 获取方式 |
|--------|----------|------|----------|
| `AppID` | `HOTPLEX_FEISHU_APP_ID` | 飞书应用 App ID | 飞书开发者后台 → 应用凭证 |
| `AppSecret` | `HOTPLEX_FEISHU_APP_SECRET` | 飞书应用 App Secret | 飞书开发者后台 → 应用凭证 |
| `VerificationToken` | `HOTPLEX_FEISHU_VERIFICATION_TOKEN` | 事件订阅验证 Token | 飞书开发者后台 → 事件订阅 |
| `EncryptKey` | `HOTPLEX_FEISHU_ENCRYPT_KEY` | 消息加密 Key | 飞书开发者后台 → 事件订阅 |

### 可选配置

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| `ServerAddr` | `HOTPLEX_FEISHU_SERVER_ADDR` | `:8082` | Webhook 服务器监听地址 |
| `MaxMessageLen` | `FEISHU_MAX_MESSAGE_LEN` | `4096` | 单消息最大长度（字节） |
| `SystemPrompt` | - | - | 系统提示词（可选） |
| `CommandRateLimit` | - | `10.0` | 命令速率限制（次/秒） |
| `CommandRateBurst` | - | `20` | 命令突发容量 |

---

## 🔧 飞书开发者后台配置

### 步骤 1: 创建企业自建应用

1. 登录 [飞书开放平台](https://open.feishu.cn/)
2. 进入「企业自建应用」→「创建应用」
3. 填写应用名称、图标、描述
4. 记录 **App ID** 和 **App Secret**

### 步骤 2: 配置权限

在「权限管理」页面添加以下权限：

| 权限名称 | 权限标识 | 用途 |
|----------|----------|------|
| 发送消息 | `im:message` | 向用户/群组发送消息 |
| 接收消息 | `im:message.receive_v1` | 接收用户消息事件 |
| 机器人配置 | `im:bot` | 配置机器人能力 |

### 步骤 3: 配置事件订阅

1. 进入「事件订阅」页面
2. 启用「事件订阅」开关
3. 填写请求地址：`https://your-domain.com/feishu/events`
4. 复制 **Verification Token** 和 **Encrypt Key**
5. 订阅以下事件：
   - ✅ `im.message.receive_v1` - 接收消息
   - ✅ `im.message.read_v1` - 消息已读（可选）

### 步骤 4: 配置机器人命令

1. 进入「机器人」→「命令配置」
2. 添加以下命令：

| 命令 | 描述 | 权限 |
|------|------|------|
| `/reset` | 重置会话上下文 | 所有成员 |
| `/dc` | 断开当前连接 | 所有成员 |

### 步骤 5: 发布应用

1. 进入「版本管理与发布」
2. 创建新版本
3. 提交审核（如需）
4. 启用应用

---

## 🎯 核心功能

### 1. CardBuilder - 卡片构建器

提供类型安全的飞书互动卡片构建能力：

```go
import "github.com/hrygo/hotplex/chatapps/feishu"

builder := feishu.NewCardBuilder()

// 构建思考中卡片
thinkingCard := builder.BuildThinkingCard("正在分析您的问题...")

// 构建工具调用卡片
toolCard := builder.BuildToolUseCard("search", "搜索相关信息")

// 构建权限请求卡片
permCard := builder.BuildPermissionCard("需要访问您的日历")

// 构建回答卡片
answerCard := builder.BuildAnswerCard("这是 AI 的回答内容")

// 构建错误卡片
errorCard := builder.BuildErrorCard("发生错误：网络连接失败")

// 构建会话统计卡片
statsCard := builder.BuildSessionStatsCard(
    feishu.SessionStats{
        TotalMessages: 100,
        TokenUsage:    5000,
        Duration:      "10m",
    },
)
```

### 2. InteractiveHandler - 交互处理器

处理飞书卡片回调事件：

```go
// 自动注册到适配器
adapter, _ := feishu.NewAdapter(config, logger)

// 内部处理逻辑：
// 1. URL 验证（飞书回调验证）
// 2. 按钮点击回调
// 3. 表单提交回调
// 4. 权限授权回调
```

**支持的交互类型**:

| 类型 | 事件 | 处理器 |
|------|------|--------|
| URL 验证 | `url_verification` | `handleURLVerification` |
| 按钮回调 | `interactive` | `handleButtonCallback` |
| 权限授权 | `interactive` | `handlePermissionCallback` |

### 3. CommandHandler - 命令处理器

处理飞书机器人命令：

```go
// 内置命令
/reset    - 重置会话上下文
/dc       - 断开当前连接

// 自定义命令注册
registry := command.NewRegistry()
registry.Register("status", handleStatusCommand)
adapter.SetCommandRegistry(registry)
```

**命令处理特性**:

- ✅ 频率限制（默认 10 次/秒，突发 20 次）
- ✅ 命令映射和路由
- ✅ 错误处理和用户提示
- ✅ 支持自定义命令扩展

### 4. EventHandler - 事件处理层

统一的飞书事件处理层（DRY 原则）：

```go
// 内部架构：
// EventParser → EventHandler → CommandHandler/InteractiveHandler
//
// 1. EventParser: 解析飞书原始事件
// 2. EventHandler: 路由到对应处理器
// 3. CommandHandler: 处理命令事件
// 4. InteractiveHandler: 处理交互事件
```

---

## 📚 API 参考

### Adapter 接口

```go
type Adapter interface {
    // 启动适配器
    Start(ctx context.Context) error
    
    // 停止适配器
    Stop(ctx context.Context) error
    
    // 设置消息处理器
    SetHandler(handler base.MessageHandler)
    
    // 发送消息到频道
    SendToChannel(ctx context.Context, chatID, text, threadID string) error
    
    // 发送卡片消息
    SendCard(ctx context.Context, chatID string, card *feishu.Card) error
    
    // 更新消息
    UpdateMessage(ctx context.Context, msgID, text string) error
    
    // 记录日志
    Logger() *slog.Logger
}
```

### Config 结构

```go
type Config struct {
    AppID             string  // 必填：App ID
    AppSecret         string  // 必填：App Secret
    VerificationToken string  // 必填：验证 Token
    EncryptKey        string  // 必填：加密 Key
    ServerAddr        string  // 可选：服务器地址（默认 :8082）
    MaxMessageLen     int     // 可选：最大消息长度（默认 4096）
    CommandRateLimit  float64 // 可选：命令速率限制（默认 10.0）
    CommandRateBurst  int     // 可选：命令突发容量（默认 20）
}
```

---

## ❌ 错误处理

### 错误类型

```go
import (
    "errors"
    "github.com/hrygo/hotplex/chatapps/feishu"
)

if err != nil {
    var apiErr *feishu.APIError
    if errors.As(err, &apiErr) {
        // API 错误，查看错误码
        log.Printf("API error: code=%d, msg=%s", apiErr.Code, apiErr.Msg)
    } else if errors.Is(err, feishu.ErrInvalidSignature) {
        // 签名验证失败
        log.Println("Invalid signature")
    } else if errors.Is(err, feishu.ErrTokenExpired) {
        // Token 过期
        log.Println("Token expired, will refresh")
    }
}
```

### 常见错误码

| 错误码 | 说明 | 解决方案 |
|--------|------|----------|
| `99991663` | app access token invalid | 检查 AppID/AppSecret 是否正确 |
| `99991668` | Invalid access token | Token 过期，等待自动刷新（30 分钟） |
| `99991671` | No permission | 检查应用权限配置，重新授权 |
| `99991664` | Invalid verification token | 检查 Verification Token 配置 |
| `99991670` | Encrypt key error | 检查 Encrypt Key 配置 |
| `99991672` | Webhook URL invalid | 检查 Webhook URL 是否可访问 |

### 错误处理最佳实践

```go
// 1. 认证错误 - 立即告警
if errors.Is(err, feishu.ErrAuthFailed) {
    alertAdmin("Feishu auth failed, check credentials")
    return err
}

// 2. 速率限制 - 等待重试
if errors.Is(err, feishu.ErrRateLimited) {
    time.Sleep(time.Second)
    return retry()
}

// 3. 网络错误 - 指数退避重试
if isNetworkError(err) {
    return retryWithBackoff()
}
```

---

## 🧪 测试指南

### 运行单元测试

```bash
# 运行所有测试
go test ./chatapps/feishu/... -v

# 运行特定测试
go test ./chatapps/feishu/... -run TestCardBuilder -v

# 生成覆盖率报告
go test ./chatapps/feishu/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# 运行集成测试（需要真实环境）
go test ./chatapps/feishu/... -tags=integration -v
```

### 测试覆盖范围

| 模块 | 覆盖率 | 测试文件 |
|------|--------|----------|
| CardBuilder | 85% | `card_builder_test.go` |
| CommandHandler | 72% | `command_handler_test.go` |
| InteractiveHandler | 68% | `interactive_handler_test.go` |
| EventParser | 90% | `event_parser_test.go` |
| Signature | 95% | `signature_test.go` |
| **总计** | **50.4%** | - |

### 压力测试

```bash
# 运行压力测试（需要真实飞书环境）
export HOTPLEX_FEISHU_APP_ID=xxx
export HOTPLEX_FEISHU_APP_SECRET=xxx
export HOTPLEX_FEISHU_VERIFICATION_TOKEN=xxx
export HOTPLEX_FEISHU_ENCRYPT_KEY=xxx

go test ./chatapps/feishu/... -bench=. -benchtime=1m
```

**压力测试场景**:

1. **并发消息发送**: 100 并发，持续 1 分钟
2. **卡片交互响应**: 测量 P95/P99 延迟
3. **命令速率限制**: 验证限流效果
4. **长连接稳定性**: 30 分钟持续运行

---

## 🤔 常见问题

### Q1: 事件收不到怎么办？

**检查清单**:

1. ✅ 飞书开发者后台事件订阅已启用
2. ✅ Webhook URL 可公网访问（使用 ngrok 测试）
3. ✅ Verification Token 配置正确
4. ✅ 服务器日志无签名验证错误

**调试命令**:

```bash
# 检查端点可达性
curl -X POST https://your-domain.com/feishu/events \
  -H "Content-Type: application/json" \
  -d '{"challenge": "test"}'

# 查看服务器日志
tail -f /var/log/hotplex/feishu.log | grep -i error
```

### Q2: 卡片回调不触发？

**可能原因**:

1. 卡片 action 配置错误
2. 回调 URL 未配置
3. 签名验证失败

**解决方案**:

```go
// 确保卡片 action 配置正确
card := builder.BuildAnswerCard("内容").
    AddButton("点击", "action_value").
    SetCallbackURL("/feishu/interactive")
```

### Q3: Token 频繁过期？

**原因**: 飞书 access_token 有效期为 2 小时

**解决方案**:

- ✅ 适配器已实现自动刷新（过期前 5 分钟）
- ✅ 检查日志确认刷新逻辑正常工作
- ✅ 如仍过期，检查系统时间是否同步

### Q4: 消息发送失败？

**排查步骤**:

1. 检查错误码（参考错误码表）
2. 验证用户/群组 ID 格式
3. 确认应用权限已授予
4. 检查消息内容是否违规

---

## 📖 参考文档

### 官方文档

- [飞书开放平台](https://open.feishu.cn/)
- [事件订阅机制](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN)
- [消息发送 API](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN)
- [互动卡片指南](https://open.feishu.cn/document/ukTMukTMukTM/uQjNwUjLyYDM14iO2ATN)
- [机器人命令配置](https://open.feishu.cn/document/ukTMukTMukTM/ucjM14iN2YDM14iO2ATN)

### 项目文档

- [HotPlex 架构设计](../../docs/architecture.md)
- [ChatApps 审计计划](../../docs/chatapps-audit-and-fix-plan.md)
- [生产部署指南](../../docs/production-guide.md)

---

## 📊 开发状态

| Phase | 状态 | 完成日期 |
|-------|------|----------|
| Phase 1: 基础通信（Adapter + API Client） | ✅ 完成 | 2026-03-03 |
| Phase 2: 交互增强（CardBuilder + Handlers） | ✅ 完成 | 2026-03-03 |
| Phase 3: 生产就绪（文档 + 压测） | 🔄 进行中 | - |

**下一步**: 压力测试 (#142) → 生产部署检查清单 (#143)

---

## 📝 变更日志

### v0.3.0 (2026-03-03)

- ✅ Phase 2.3: CommandHandler + Adapter 整合
- ✅ DRY/SOLID 架构重构
- ✅ 测试覆盖率提升至 50.4%

### v0.2.0 (2026-03-03)

- ✅ Phase 2.1: CardBuilder 卡片构建器
- ✅ Phase 2.2: InteractiveHandler 交互处理器

### v0.1.0 (2026-03-03)

- ✅ Phase 1: Feishu Adapter 基础框架

---

*维护者*: HotPlex Team  
*许可证*: MIT
