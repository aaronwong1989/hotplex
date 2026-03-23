# Stream Message Store 配置说明

## 约定大于配置

所有配置参数都有**合理的默认值**，无需手动配置即可使用。如需自定义，可通过环境变量覆盖。

## 默认值（最佳实践）

| 参数 | 默认值 | 说明 |
|------|--------|------|
| Timeout | 5m | 缓冲区超时时间 |
| MaxBuffers | 1000 | 最大并发缓冲区数量 |
| AutoCommitEnabled | true | 自动提交开关 |
| AutoCommitInterval | 30s | 自动提交间隔 |
| SaveOnTermination | true | 会话终止前保存 |

## 环境变量配置

可通过以下环境变量覆盖默认值：

```bash
# 缓冲区超时 (e.g., "5m", "10m")
export HOTPLEX_MESSAGE_STORE_TIMEOUT=5m

# 最大缓冲区数量 (e.g., "1000", "2000")
export HOTPLEX_MESSAGE_STORE_MAX_BUFFERS=1000

# 自动提交开关 ("true"/"false")
export HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_ENABLED=true

# 自动提交间隔 (e.g., "30s", "1m")
export HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_INTERVAL=30s

# 会话终止前保存 ("true"/"false")
export HOTPLEX_MESSAGE_STORE_SAVE_ON_TERMINATION=true
```

## YAML 配置（可选）

如果使用 YAML 配置文件，格式如下：

```yaml
message_store:
  streaming:
    timeout: 5m
    max_buffers: 1000
    auto_commit:
      enabled: true
      interval: 30s
    save_on_termination: true
```

## 使用示例

### 方式 1: 使用默认配置（推荐）

```go
import (
    "github.com/hrygo/hotplex/chatapps/base"
    "github.com/hrygo/hotplex/plugins/storage"
)

// 使用默认配置 + 环境变量
store := base.LoadStreamMessageStoreConfig(storageBackend, logger)
```

### 方式 2: 使用 YAML 配置

```go
import "github.com/hrygo/hotplex/chatapps/base"

// 从 YAML 加载配置
yamlConfig := map[string]interface{}{
    "streaming": map[string]interface{}{
        "timeout": "10m",
        "auto_commit": map[string]interface{}{
            "enabled": true,
            "interval": "1m",
        },
    },
}
store := base.LoadStreamMessageStoreConfigFromYAML(storageBackend, yamlConfig, logger)
```

### 方式 3: 手动配置

```go
import "github.com/hrygo/hotplex/chatapps/base"

// 手动创建配置
config := &base.StreamMessageStoreConfig{
    Timeout:            10 * time.Minute,
    MaxBuffers:         2000,
    AutoCommitEnabled:  true,
    AutoCommitInterval: time.Minute,
    SaveOnTermination:  true,
}
store := base.NewStreamMessageStoreWithConfig(storageBackend, config, logger)
```

## 配置优先级

**环境变量 > YAML 配置 > 默认值**

## 监控指标

通过 `GetMetrics()` 获取实时指标：

```go
metrics := store.GetMetrics()
// metrics.ActiveBuffers     - 活跃缓冲区数量
// metrics.CompletedBuffers  - 已完成但未提交的缓冲区
// metrics.TotalChunks       - 总 chunk 数量
// metrics.MaxBuffers        - 最大缓冲区数量
// metrics.TimeoutSeconds    - 超时时间（秒）
```

## 最佳实践

1. **生产环境**: 使用默认配置即可，必要时通过环境变量微调
2. **高并发场景**: 增加 `MaxBuffers` 到 2000-5000
3. **长会话场景**: 增加 `Timeout` 到 10-15 分钟
4. **调试模式**: 设置 `HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_INTERVAL=10s` 加快提交频率
