# Config Package (`internal/config`)

Server configuration loading and hot-reload support.

## Overview

This package handles YAML-based configuration for the `hotplexd` server, including engine settings, server options, and security configuration.

## Key Types

type ServerConfig struct {
    Engine    EngineConfig     `yaml:"engine"`
    Server    ServerSettings   `yaml:"server"`
    Security  SecurityConfig   `yaml:"security"`
    AgentCard *AgentCardConfig `yaml:"agent_card,omitempty"`
}

type ServerSettings struct {
    Port        string `yaml:"port"`
    LogLevel    string `yaml:"log_level"`
    LogFormat   string `yaml:"log_format"`
    BridgePort  string `yaml:"bridge_port"`
    BridgeToken string `yaml:"bridge_token"`
}

type SecurityConfig struct {
    APIKey         string `yaml:"api_key"`
    PermissionMode string `yaml:"permission_mode"`
}

## Usage

```go
import "github.com/hrygo/hotplex/internal/config"

// Load server config
loader, err := config.NewServerLoader("configs/server.yaml", logger)
if err != nil {
    log.Fatal(err)
}

// Get current config
cfg := loader.Get()

// Hot-reload support
loader.Watch(ctx, func(newCfg *ServerConfig) {
    log.Info("Config reloaded")
})
```

## Features

- **YAML Parsing**: Uses `gopkg.in/yaml.v3`
- **Env Expansion**: Automatically expands `${VAR}` or `$VAR` in YAML values
- **Hot-Reload**: Watch for config file changes (via `Watch` method)
- **Thread-Safe**: Safe concurrent access via `sync.RWMutex`
- **Validation**: Strict validation of timeouts, log levels, and permission modes

## Files

| File | Purpose |
|------|---------|
| `server_config.go` | Server configuration types and loader |
| `hotreload_yaml.go` | YAML file watcher for hot-reload |
