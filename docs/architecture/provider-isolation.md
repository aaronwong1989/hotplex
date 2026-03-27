# Provider 架构设计

## 问题背景

**Issue**: #99 (架构违规修复)

**错误做法**: 在 `main.go` 中预加载 ChatApps 配置并用于创建全局 Engine

```go
// ❌ 错误示例 - 架构违规
func runDaemon() {
    // 预加载 ChatApps 配置
    cfg := loader.GetConfig("slack")
    chatappsProviderType := cfg.Provider.Type
    chatappsOpenCodeCfg := cfg.Provider.OpenCode

    // 使用 ChatApps 配置创建全局 Engine
    engine := createEngine(logger, serverCfg, chatappsProviderType, chatappsOpenCodeCfg)
}
```

**为什么错误**:
- 会有多个 ChatApp（Slack、Telegram、Discord、Feishu 等）
- 每个 ChatApp 可以有自己的 provider 配置
- 全局 Engine 不应该绑定到特定 ChatApp 的配置

---

## 正确架构

### 1. 配置层级

```
全局 Engine
├── Provider 来源: 环境变量 / server.yaml
└── 用途: 默认 provider，用于所有未指定 provider 的场景

ChatApp Adapter (Slack)
├── Provider 来源: admin/slack.yaml
└── 用途: 该 adapter 专用的 provider

ChatApp Adapter (Telegram)
├── Provider 来源: admin/telegram.yaml
└── 用途: 该 adapter 专用的 provider
```

### 2. main.go 职责

**只负责创建全局 Engine**，使用默认配置：

```go
// ✅ 正确 - 全局 Engine 使用环境变量
func createEngine(logger *slog.Logger, serverCfg *config.ServerLoader) (*hotplex.Engine, string) {
    providerType := provider.ProviderType(os.Getenv("HOTPLEX_PROVIDER_TYPE"))
    if providerType == "" {
        providerType = provider.ProviderTypeClaudeCode
    }

    var openCodeCfg *provider.OpenCodeConfig
    if providerType == provider.ProviderTypeOpenCodeServer {
        openCodeCfg = &provider.OpenCodeConfig{
            ServerURL: os.Getenv("HOTPLEX_OPEN_CODE_SERVER_URL"),
            Password:  os.Getenv("HOTPLEX_OPEN_CODE_PASSWORD"),
        }
        // ...
    }

    prv, _ := provider.CreateProvider(provider.ProviderConfig{
        Type:     providerType,
        OpenCode: openCodeCfg,
    })

    return hotplex.NewEngine(hotplex.EngineOptions{
        Provider: prv,
        // ...
    })
}
```

### 3. ChatApp Adapter 职责

**每个 adapter 使用自己的 provider 配置**：

```go
// chatapps/setup.go
func Setup(ctx context.Context, logger *slog.Logger, configDir string) {
    loader, _ := chatapps.NewConfigLoader(configDir, logger)

    for platform, cfg := range loader.GetAllConfigs() {
        // 创建 adapter 专用的 provider
        prv, _ := provider.CreateProvider(provider.ProviderConfig{
            Type:     cfg.Provider.Type,
            OpenCode: cfg.Provider.OpenCode,
        })

        // 注入给 adapter
        adapter := CreateAdapter(platform, prv, cfg)
    }
}
```

---

## 配置文件示例

### 全局配置

```bash
# .env
HOTPLEX_PROVIDER_TYPE=claude-code
HOTPLEX_PROVIDER_MODEL=claude-sonnet-4-6
```

### ChatApp 专用配置

```yaml
# ~/.hotplex/configs/admin/slack.yaml
platform: slack
provider:
  type: opencode-server
  opencode:
    server_url: ${HOTPLEX_OPEN_CODE_SERVER_URL}
    port: 4096
    password: "5aP7vIXat+f7j/ZvfZ23RgdF9Bh2dbf1Mbbtjtbbb1M="
```

```yaml
# ~/.hotplex/configs/admin/telegram.yaml
platform: telegram
provider:
  type: opencode-server
  opencode:
    server_url: ${HOTPLEX_OPEN_CODE_SERVER_URL}
    port: 4097
    password: "different-password-here"
```

---

## 日志验证

修复后的日志显示配置加载正确：

```
# 全局 Engine 使用环境变量
level=DEBUG source=provider/opencode_server_provider.go msg="OpenCode password loaded" password_preview=5aP7vIXat+
level=DEBUG source=provider/transport_http.go msg="Health check with Basic Auth" username=opencode password_len=44

# Slack adapter 使用自己的配置
level=INFO source=chatapps/config.go msg="Loaded OpenCode provider config" platform=slack
level=DEBUG source=provider/opencode_server_provider.go msg="OpenCode password loaded" password_preview=5aP7vIXat+
```

---

## 关键原则

1. **全局 Engine** = 默认 provider（环境变量 / server.yaml）
2. **每个 ChatApp** = 可选的专用 provider（自己的 YAML 配置）
3. **main.go** = 不读取 ChatApps 配置，只创建全局 Engine
4. **chatapps/setup.go** = 为每个 adapter 创建专用 provider

---

## 修复记录

- **日期**: 2026-03-27
- **Issue**: #99
- **Commit**: refactor(provider): remove ChatApps config binding from global Engine
- **文件**:
  - `cmd/hotplexd/main.go` (移除 ChatApps 配置预加载)
  - `chatapps/setup.go` (每个 adapter 创建自己的 provider)
