package base

import (
	"os"
	"testing"
	"time"
)

func TestStreamMessageStoreConfigLoader_Default(t *testing.T) {
	loader := NewStreamMessageStoreConfigLoader()
	config := loader.Default()

	// 验证默认值
	if config.Timeout != 5*time.Minute {
		t.Errorf("expected default timeout 5m, got %v", config.Timeout)
	}
	if config.MaxBuffers != 1000 {
		t.Errorf("expected default max buffers 1000, got %d", config.MaxBuffers)
	}
	if !config.AutoCommitEnabled {
		t.Error("expected auto commit enabled by default")
	}
	if config.AutoCommitInterval != 30*time.Second {
		t.Errorf("expected default interval 30s, got %v", config.AutoCommitInterval)
	}
	if !config.SaveOnTermination {
		t.Error("expected save on termination enabled by default")
	}
}

func TestStreamMessageStoreConfigLoader_LoadFromEnv(t *testing.T) {
	// 设置环境变量
	if err := os.Setenv("HOTPLEX_MESSAGE_STORE_TIMEOUT", "10m"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	if err := os.Setenv("HOTPLEX_MESSAGE_STORE_MAX_BUFFERS", "2000"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	if err := os.Setenv("HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_ENABLED", "false"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	if err := os.Setenv("HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_INTERVAL", "1m"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	if err := os.Setenv("HOTPLEX_MESSAGE_STORE_SAVE_ON_TERMINATION", "false"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() {
		_ = os.Unsetenv("HOTPLEX_MESSAGE_STORE_TIMEOUT")
		_ = os.Unsetenv("HOTPLEX_MESSAGE_STORE_MAX_BUFFERS")
		_ = os.Unsetenv("HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_ENABLED")
		_ = os.Unsetenv("HOTPLEX_MESSAGE_STORE_AUTOCOMMIT_INTERVAL")
		_ = os.Unsetenv("HOTPLEX_MESSAGE_STORE_SAVE_ON_TERMINATION")
	}()

	loader := NewStreamMessageStoreConfigLoader()
	config := loader.LoadFromEnv(nil)

	// 验证环境变量覆盖
	if config.Timeout != 10*time.Minute {
		t.Errorf("expected timeout 10m from env, got %v", config.Timeout)
	}
	if config.MaxBuffers != 2000 {
		t.Errorf("expected max buffers 2000 from env, got %d", config.MaxBuffers)
	}
	if config.AutoCommitEnabled {
		t.Error("expected auto commit disabled from env")
	}
	if config.AutoCommitInterval != time.Minute {
		t.Errorf("expected interval 1m from env, got %v", config.AutoCommitInterval)
	}
	if config.SaveOnTermination {
		t.Error("expected save on termination disabled from env")
	}
}

func TestStreamMessageStoreConfigLoader_LoadFromYAML(t *testing.T) {
	yamlConfig := map[string]interface{}{
		"streaming": map[string]interface{}{
			"timeout":     "15m",
			"max_buffers": 3000,
			"auto_commit": map[string]interface{}{
				"enabled":  false,
				"interval": "2m",
			},
			"save_on_termination": false,
		},
	}

	loader := NewStreamMessageStoreConfigLoader()
	config := loader.LoadFromYAML(yamlConfig, nil)

	// 验证 YAML 配置
	if config.Timeout != 15*time.Minute {
		t.Errorf("expected timeout 15m from yaml, got %v", config.Timeout)
	}
	if config.MaxBuffers != 3000 {
		t.Errorf("expected max buffers 3000 from yaml, got %d", config.MaxBuffers)
	}
	if config.AutoCommitEnabled {
		t.Error("expected auto commit disabled from yaml")
	}
	if config.AutoCommitInterval != 2*time.Minute {
		t.Errorf("expected interval 2m from yaml, got %v", config.AutoCommitInterval)
	}
	if config.SaveOnTermination {
		t.Error("expected save on termination disabled from yaml")
	}
}

func TestStreamMessageStoreConfigLoader_Priority(t *testing.T) {
	// 设置环境变量
	if err := os.Setenv("HOTPLEX_MESSAGE_STORE_TIMEOUT", "8m"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Unsetenv("HOTPLEX_MESSAGE_STORE_TIMEOUT") }()

	yamlConfig := map[string]interface{}{
		"streaming": map[string]interface{}{
			"timeout":     "12m",
			"max_buffers": 2500,
		},
	}

	loader := NewStreamMessageStoreConfigLoader()
	config := loader.Load(yamlConfig)

	// 环境变量应覆盖 YAML
	if config.Timeout != 8*time.Minute {
		t.Errorf("expected timeout 8m from env (overrides yaml), got %v", config.Timeout)
	}
	// YAML 应覆盖默认值
	if config.MaxBuffers != 2500 {
		t.Errorf("expected max buffers 2500 from yaml, got %d", config.MaxBuffers)
	}
}
