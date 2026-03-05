# Feishu (Lark) Adapter for HotPlex

*View other languages: [简体中文](README_zh.md)*

Feishu (Lark) adapter providing Chinese enterprise IM integration capabilities for HotPlex.

**Status**: ✅ Phase 3 Production Ready  
**Test Coverage**: 50.4%  
**Last Updated**: 2026-03-03

---

## 📖 Table of Contents

- [Quick Start](#-quick-start)
- [Configuration](#-configuration)
- [Feishu Developer Console Setup](#-feishu-developer-console-setup)
- [Core Features](#-core-features)
- [API Reference](#-api-reference)
- [Error Handling](#-error-handling)
- [Testing Guide](#-testing-guide)
- [FAQ](#-faq)
- [References](#-references)

---

## 🚀 Quick Start

### 1. Configure Environment Variables

```bash
# Required: Feishu application credentials
export HOTPLEX_FEISHU_APP_ID=cli_a1b2c3d4e5f6g7h8
export HOTPLEX_FEISHU_APP_SECRET=xxxxxxxxxxxxxxxx
export HOTPLEX_FEISHU_VERIFICATION_TOKEN=xxxxxxxx
export HOTPLEX_FEISHU_ENCRYPT_KEY=xxxxxxxxxxxxxxxx

# Optional: Server configuration
export HOTPLEX_FEISHU_SERVER_ADDR=:8082
export FEISHU_MAX_MESSAGE_LEN=4096
```

### 2. Create Adapter Instance

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
    
    // Set message handler
    adapter.SetHandler(myHandler)
    
    // Start adapter
    if err := adapter.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### 3. Verify Deployment

```bash
# Check endpoint accessibility
curl -X POST https://your-domain.com/feishu/events \
  -H "Content-Type: application/json" \
  -d '{"challenge": "test"}'

# Expected response: returns challenge value
```

---

## ⚙️ Configuration

### Required Configuration

| Config | Environment Variable | Description | How to Obtain |
|--------|---------------------|-------------|---------------|
| `AppID` | `HOTPLEX_FEISHU_APP_ID` | Feishu App ID | Feishu Developer Console → App Credentials |
| `AppSecret` | `HOTPLEX_FEISHU_APP_SECRET` | Feishu App Secret | Feishu Developer Console → App Credentials |
| `VerificationToken` | `HOTPLEX_FEISHU_VERIFICATION_TOKEN` | Event Subscription Verification Token | Feishu Developer Console → Event Subscription |
| `EncryptKey` | `HOTPLEX_FEISHU_ENCRYPT_KEY` | Message Encryption Key | Feishu Developer Console → Event Subscription |

### Optional Configuration

| Config | Environment Variable | Default | Description |
|--------|---------------------|---------|-------------|
| `ServerAddr` | `HOTPLEX_FEISHU_SERVER_ADDR` | `:8082` | Webhook server listen address |
| `MaxMessageLen` | `FEISHU_MAX_MESSAGE_LEN` | `4096` | Maximum message length (bytes) |
| `SystemPrompt` | - | - | System prompt (optional) |
| `CommandRateLimit` | - | `10.0` | Command rate limit (requests/second) |
| `CommandRateBurst` | - | `20` | Command burst capacity |

---

## 🔧 Feishu Developer Console Setup

### Step 1: Create Enterprise Self-Built App

1. Login to [Feishu Open Platform](https://open.feishu.cn/)
2. Go to "Enterprise Self-Built Apps" → "Create App"
3. Fill in app name, icon, description
4. Record **App ID** and **App Secret**

### Step 2: Configure Permissions

Add the following permissions on "Permissions" page:

| Permission Name | Permission ID | Purpose |
|----------------|---------------|---------|
| Send Messages | `im:message` | Send messages to users/groups |
| Receive Messages | `im:message.receive_v1` | Receive message events |
| Bot Configuration | `im:bot` | Configure bot capabilities |

### Step 3: Configure Event Subscription

1. Go to "Event Subscription" page
2. Enable "Event Subscription" switch
3. Fill in request URL: `https://your-domain.com/feishu/events`
4. Copy **Verification Token** and **Encrypt Key**
5. Subscribe to the following events:
   - ✅ `im.message.receive_v1` - Receive messages
   - ✅ `im.message.read_v1` - Message read (optional)

### Step 4: Configure Bot Commands

1. Go to "Bot" → "Command Configuration"
2. Add the following commands:

| Command | Description | Permission |
|---------|-------------|------------|
| `/reset` | Reset session context | All members |
| `/dc` | Disconnect current connection | All members |

### Step 5: Publish App

1. Go to "Version Management & Release"
2. Create new version
3. Submit for review (if required)
4. Enable app

---

## 🎯 Core Features

### 1. CardBuilder - Card Builder

Provides type-safe Feishu interactive card building capabilities:

```go
import "github.com/hrygo/hotplex/chatapps/feishu"

builder := feishu.NewCardBuilder()

// Build thinking card
thinkingCard := builder.BuildThinkingCard("Analyzing your question...")

// Build tool use card
toolCard := builder.BuildToolUseCard("search", "Searching for relevant information")

// Build permission request card
permCard := builder.BuildPermissionCard("Needs access to your calendar")

// Build answer card
answerCard := builder.BuildAnswerCard("This is the AI's response")

// Build error card
errorCard := builder.BuildErrorCard("Error occurred: Network connection failed")

// Build session stats card
statsCard := builder.BuildSessionStatsCard(
    feishu.SessionStats{
        TotalMessages: 100,
        TokenUsage:    5000,
        Duration:      "10m",
    },
)
```

### 2. InteractiveHandler - Interaction Handler

Handles Feishu card callback events:

```go
// Automatically registered to adapter
adapter, _ := feishu.NewAdapter(config, logger)

// Internal processing logic:
// 1. URL verification (Feishu callback verification)
// 2. Button click callback
// 3. Form submission callback
// 4. Permission authorization callback
```

**Supported Interaction Types**:

| Type | Event | Handler |
|------|-------|---------|
| URL Verification | `url_verification` | `handleURLVerification` |
| Button Callback | `interactive` | `handleButtonCallback` |
| Permission Authorization | `interactive` | `handlePermissionCallback` |

### 3. CommandHandler - Command Handler

Handles Feishu bot commands:

```go
// Built-in commands
/reset    - Reset session context
/dc       - Disconnect current connection

// Custom command registration
registry := command.NewRegistry()
registry.Register("status", handleStatusCommand)
adapter.SetCommandRegistry(registry)
```

**Command Handler Features**:

- ✅ Rate limiting (default 10 req/s, burst 20)
- ✅ Command mapping and routing
- ✅ Error handling and user prompts
- ✅ Support for custom command extension

### 4. EventHandler - Event Processing Layer

Unified Feishu event processing layer (DRY principle):

```go
// Internal architecture:
// EventParser → EventHandler → CommandHandler/InteractiveHandler
//
// 1. EventParser: Parse Feishu raw events
// 2. EventHandler: Route to corresponding handlers
// 3. CommandHandler: Handle command events
// 4. InteractiveHandler: Handle interaction events
```

---

## 📚 API Reference

### Adapter Interface

```go
type Adapter interface {
    // Start adapter
    Start(ctx context.Context) error
    
    // Stop adapter
    Stop(ctx context.Context) error
    
    // Set message handler
    SetHandler(handler base.MessageHandler)
    
    // Send message to channel
    SendToChannel(ctx context.Context, chatID, text, threadID string) error
    
    // Send card message
    SendCard(ctx context.Context, chatID string, card *feishu.Card) error
    
    // Update message
    UpdateMessage(ctx context.Context, msgID, text string) error
    
    // Log
    Logger() *slog.Logger
}
```

### Config Structure

```go
type Config struct {
    AppID             string  // Required: App ID
    AppSecret         string  // Required: App Secret
    VerificationToken string  // Required: Verification Token
    EncryptKey        string  // Required: Encryption Key
    ServerAddr        string  // Optional: Server address (default :8082)
    MaxMessageLen     int     // Optional: Max message length (default 4096)
    CommandRateLimit  float64 // Optional: Command rate limit (default 10.0)
    CommandRateBurst  int     // Optional: Command burst capacity (default 20)
}
```

---

## ❌ Error Handling

### Error Types

```go
import (
    "errors"
    "github.com/hrygo/hotplex/chatapps/feishu"
)

if err != nil {
    var apiErr *feishu.APIError
    if errors.As(err, &apiErr) {
        // API error, check error code
        log.Printf("API error: code=%d, msg=%s", apiErr.Code, apiErr.Msg)
    } else if errors.Is(err, feishu.ErrInvalidSignature) {
        // Signature verification failed
        log.Println("Invalid signature")
    } else if errors.Is(err, feishu.ErrTokenExpired) {
        // Token expired
        log.Println("Token expired, will refresh")
    }
}
```

### Common Error Codes

| Error Code | Description | Solution |
|------------|-------------|----------|
| `99991663` | app access token invalid | Check if AppID/AppSecret is correct |
| `99991668` | Invalid access token | Token expired, wait for auto-refresh (30 min) |
| `99991671` | No permission | Check app permission configuration, re-authorize |
| `99991664` | Invalid verification token | Check Verification Token configuration |
| `99991670` | Encrypt key error | Check Encrypt Key configuration |
| `99991672` | Webhook URL invalid | Check if Webhook URL is accessible |

### Error Handling Best Practices

```go
// 1. Authentication error - alert immediately
if errors.Is(err, feishu.ErrAuthFailed) {
    alertAdmin("Feishu auth failed, check credentials")
    return err
}

// 2. Rate limit - wait and retry
if errors.Is(err, feishu.ErrRateLimited) {
    time.Sleep(time.Second)
    return retry()
}

// 3. Network error - exponential backoff retry
if isNetworkError(err) {
    return retryWithBackoff()
}
```

---

## 🧪 Testing Guide

### Run Unit Tests

```bash
# Run all tests
go test ./chatapps/feishu/... -v

# Run specific test
go test ./chatapps/feishu/... -run TestCardBuilder -v

# Generate coverage report
go test ./chatapps/feishu/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run integration tests (requires real environment)
go test ./chatapps/feishu/... -tags=integration -v
```

### Test Coverage

| Module | Coverage | Test File |
|--------|----------|-----------|
| CardBuilder | 85% | `card_builder_test.go` |
| CommandHandler | 72% | `command_handler_test.go` |
| InteractiveHandler | 68% | `interactive_handler_test.go` |
| EventParser | 90% | `event_parser_test.go` |
| Signature | 95% | `signature_test.go` |
| **Total** | **50.4%** | - |

### Pressure Tests

```bash
# Run pressure tests (requires real Feishu environment)
export HOTPLEX_FEISHU_APP_ID=xxx
export HOTPLEX_FEISHU_APP_SECRET=xxx
export HOTPLEX_FEISHU_VERIFICATION_TOKEN=xxx
export HOTPLEX_FEISHU_ENCRYPT_KEY=xxx

go test ./chatapps/feishu/... -tags=pressure -v
```

**Pressure Test Scenarios**:

1. **Concurrent Message Send**: 100 concurrency, 1 minute duration
2. **Card Interaction Response**: Measure P95/P99 latency
3. **Command Rate Limit**: Verify rate limiting effect
4. **Long Connection Stability**: 30 minutes continuous run

---

## 🤔 FAQ

### Q1: Not receiving events?

**Checklist**:

1. ✅ Feishu developer console event subscription is enabled
2. ✅ Webhook URL is publicly accessible (test with ngrok)
3. ✅ Verification Token is configured correctly
4. ✅ No signature verification errors in server logs

**Debug Commands**:

```bash
# Check endpoint accessibility
curl -X POST https://your-domain.com/feishu/events \
  -H "Content-Type: application/json" \
  -d '{"challenge": "test"}'

# Check server logs
tail -f /var/log/hotplex/feishu.log | grep -i error
```

### Q2: Card callbacks not triggering?

**Possible Causes**:

1. Card action configuration error
2. Callback URL not configured
3. Signature verification failed

**Solution**:

```go
// Ensure card action is configured correctly
card := builder.BuildAnswerCard("Content").
    AddButton("Click", "action_value").
    SetCallbackURL("/feishu/interactive")
```

### Q3: Token frequently expires?

**Cause**: Feishu access_token validity is 2 hours

**Solution**:

- ✅ Adapter implements auto-refresh (5 minutes before expiration)
- ✅ Check logs to confirm refresh logic is working
- ✅ If still expiring, check if system time is synchronized

### Q4: Message send failed?

**Troubleshooting Steps**:

1. Check error code (refer to error code table)
2. Verify user/group ID format
3. Confirm app permissions are granted
4. Check if message content violates policies

---

## 📖 References

### Official Documentation

- [Feishu Open Platform](https://open.feishu.cn/)
- [Event Subscription Mechanism](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN)
- [Message Send API](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN)
- [Interactive Card Guide](https://open.feishu.cn/document/ukTMukTMukTM/uQjNwUjLyYDM14iO2ATN)
- [Bot Command Configuration](https://open.feishu.cn/document/ukTMukTMukTM/ucjM14iN2YDM14iO2ATN)

### Project Documentation

- [HotPlex Architecture Design](../../docs/architecture.md)
- [ChatApps Audit Plan](../../docs/chatapps-audit-and-fix-plan.md)
- [Production Deployment Guide](../../docs/production-guide.md)

---

## 📊 Development Status

| Phase | Status | Completion Date |
|-------|--------|-----------------|
| Phase 1: Basic Communication (Adapter + API Client) | ✅ Done | 2026-03-03 |
| Phase 2: Interactive Enhancement (CardBuilder + Handlers) | ✅ Done | 2026-03-03 |
| Phase 3: Production Ready (Docs + Pressure Tests) | 🔄 In Progress | - |

**Next Steps**: Pressure Tests (#142) → Production Deployment Checklist (#143)

---

## 📝 Changelog

### v0.3.0 (2026-03-03)

- ✅ Phase 2.3: CommandHandler + Adapter Integration
- ✅ DRY/SOLID Architecture Refactoring
- ✅ Test coverage improved to 50.4%

### v0.2.0 (2026-03-03)

- ✅ Phase 2.1: CardBuilder Card Builder
- ✅ Phase 2.2: InteractiveHandler Interaction Handler

### v0.1.0 (2026-03-03)

- ✅ Phase 1: Feishu Adapter Basic Framework

---

*Maintainer*: HotPlex Team  
*License*: MIT
