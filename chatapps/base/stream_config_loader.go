package base

import (
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/hrygo/hotplex/plugins/storage"
)

// StreamMessageStoreConfigLoader 流式存储配置加载器
// 采用"约定大于配置"原则，所有参数都有合理的默认值
// 配置优先级: 环境变量 > 配置文件 > 默认值
type StreamMessageStoreConfigLoader struct {
	// 默认值
	defaultTimeout            time.Duration
	defaultMaxBuffers         int
	defaultAutoCommitEnabled  bool
	defaultAutoCommitInterval time.Duration
	defaultSaveOnTermination  bool
}

// NewStreamMessageStoreConfigLoader 创建配置加载器
func NewStreamMessageStoreConfigLoader() *StreamMessageStoreConfigLoader {
	return &StreamMessageStoreConfigLoader{
		// 最佳实践默认值
		defaultTimeout:            5 * time.Minute,  // 5 分钟超时
		defaultMaxBuffers:         1000,              // 最多 1000 个并发会话
		defaultAutoCommitEnabled:  true,              // 默认启用自动提交
		defaultAutoCommitInterval: 30 * time.Second,  // 30 秒自动提交一次
		defaultSaveOnTermination:  true,              // 默认在会话终止前保存
	}
}

// --- 私有辅助函数 (DRY) ---

// parseEnvBool 解析环境变量布尔值
func parseEnvBool(v string) bool {
	b, _ := strconv.ParseBool(v)
	return b
}

// parseEnvDuration 解析环境变量 duration
func parseEnvDuration(v string) (time.Duration, bool) {
	dur, err := time.ParseDuration(v)
	return dur, err == nil
}

// parseEnvInt 解析环境变量整数
func parseEnvInt(v string) (int, bool) {
	i, err := strconv.Atoi(v)
	return i, err == nil && i > 0
}

// --- 公共方法 ---

// LoadFromEnv 从环境变量加载配置（覆盖默认值）
// 环境变量:
//   - HOTPLEX_MESSAGE_STORE_TIMEOUT: 缓冲区超时 (e.g., "5m", "10m")
//   - HOTPLEX_MESSAGE_STORE_MAX_BUFFERS: 最大缓冲区数量 (e.g., "1000")
//   - HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_ENABLED: 是否启用自动提交 ("true"/"false")
//   - HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_INTERVAL: 自动提交间隔 (e.g., "30s", "1m")
//   - HOTPLEX_MESSAGE_STORE_SAVE_ON_TERMINATION: 是否在终止前保存 ("true"/"false")
func (l *StreamMessageStoreConfigLoader) LoadFromEnv(baseConfig *StreamMessageStoreConfig) *StreamMessageStoreConfig {
	if baseConfig == nil {
		baseConfig = l.Default()
	}

	if v := os.Getenv("HOTPLEX_MESSAGE_STORE_TIMEOUT"); v != "" {
		if dur, ok := parseEnvDuration(v); ok {
			baseConfig.Timeout = dur
		}
	}

	if v := os.Getenv("HOTPLEX_MESSAGE_STORE_MAX_BUFFERS"); v != "" {
		if i, ok := parseEnvInt(v); ok {
			baseConfig.MaxBuffers = i
		}
	}

	if v := os.Getenv("HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_ENABLED"); v != "" {
		baseConfig.AutoCommitEnabled = parseEnvBool(v)
	}

	if v := os.Getenv("HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_INTERVAL"); v != "" {
		if dur, ok := parseEnvDuration(v); ok {
			baseConfig.AutoCommitInterval = dur
		}
	}

	if v := os.Getenv("HOTPLEX_MESSAGE_STORE_SAVE_ON_TERMINATION"); v != "" {
		baseConfig.SaveOnTermination = parseEnvBool(v)
	}

	return baseConfig
}

// LoadFromYAML 从 YAML 配置加载（覆盖现有配置）
// YAML 格式示例:
//
//	message_store:
//	  streaming:
//	    timeout: 5m
//	    max_buffers: 1000
//	    auto_commit:
//	      enabled: true
//	      interval: 30s
//	    save_on_termination: true
func (l *StreamMessageStoreConfigLoader) LoadFromYAML(yamlConfig map[string]interface{}, baseConfig *StreamMessageStoreConfig) *StreamMessageStoreConfig {
	if baseConfig == nil {
		baseConfig = l.Default()
	}

	if yamlConfig == nil {
		return baseConfig
	}

	// 解析 streaming 配置节
	streaming, ok := yamlConfig["streaming"].(map[string]interface{})
	if !ok {
		return baseConfig
	}

	if v, ok := streaming["timeout"].(string); ok {
		if dur, ok := parseEnvDuration(v); ok {
			baseConfig.Timeout = dur
		}
	}

	if v, ok := streaming["max_buffers"].(int); ok && v > 0 {
		baseConfig.MaxBuffers = v
	}

	if autoCommit, ok := streaming["auto_commit"].(map[string]interface{}); ok {
		if enabled, ok := autoCommit["enabled"].(bool); ok {
			baseConfig.AutoCommitEnabled = enabled
		}
		if interval, ok := autoCommit["interval"].(string); ok {
			if dur, ok := parseEnvDuration(interval); ok {
				baseConfig.AutoCommitInterval = dur
			}
		}
	}

	if v, ok := streaming["save_on_termination"].(bool); ok {
		baseConfig.SaveOnTermination = v
	}

	return baseConfig
}

// Default 返回默认配置
func (l *StreamMessageStoreConfigLoader) Default() *StreamMessageStoreConfig {
	return &StreamMessageStoreConfig{
		Timeout:            l.defaultTimeout,
		MaxBuffers:         l.defaultMaxBuffers,
		AutoCommitEnabled:  l.defaultAutoCommitEnabled,
		AutoCommitInterval: l.defaultAutoCommitInterval,
		SaveOnTermination:  l.defaultSaveOnTermination,
	}
}

// LoadWithDefaults 加载配置（环境变量 > 默认值）
// 这是最常用的加载方式，适合大多数场景
func (l *StreamMessageStoreConfigLoader) LoadWithDefaults() *StreamMessageStoreConfig {
	return l.LoadFromEnv(l.Default())
}

// Load 加载完整配置（环境变量 > YAML > 默认值）
// 适合需要 YAML 配置文件的场景
func (l *StreamMessageStoreConfigLoader) Load(yamlConfig map[string]interface{}) *StreamMessageStoreConfig {
	return l.LoadFromEnv(l.LoadFromYAML(yamlConfig, l.Default()))
}

// --- 便捷函数 ---

// LoadStreamMessageStoreConfig 便捷函数：加载配置并创建 Store
// 使用默认 logger 和环境变量配置
func LoadStreamMessageStoreConfig(store storage.WriteOnlyStore, logger *slog.Logger) *StreamMessageStore {
	loader := NewStreamMessageStoreConfigLoader()
	config := loader.LoadWithDefaults()
	return NewStreamMessageStoreWithConfig(store, config, logger)
}

// LoadStreamMessageStoreConfigFromYAML 便捷函数：从 YAML 加载配置
func LoadStreamMessageStoreConfigFromYAML(store storage.WriteOnlyStore, yamlConfig map[string]interface{}, logger *slog.Logger) *StreamMessageStore {
	loader := NewStreamMessageStoreConfigLoader()
	config := loader.Load(yamlConfig)
	return NewStreamMessageStoreWithConfig(store, config, logger)
}
