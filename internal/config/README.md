# Config Package (`internal/config`)

Server configuration loading and hot-reload support.

## Overview

This package handles YAML-based configuration for the `hotplexd` server, including engine settings, server options, and security configuration.

## Key Types

```go
type ServerConfig struct {
    Engine   EngineConfig
    Server   ServerSettings
    Security SecurityConfig
}

type EngineConfig struct {
    Timeout         time.Duration
    IdleTimeout     time.Duration
    WorkDir         string
    SystemPrompt    string
    AllowedTools    []string
    DisallowedTools []string
}
```

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
- **Hot-Reload**: Watch for config file changes
- **Thread-Safe**: Safe concurrent access via `sync.RWMutex`
- **Validation**: Validates configuration values on load

## Files

| File | Purpose |
|------|---------|
| `server_config.go` | Server configuration types and loader |
| `hotreload_yaml.go` | YAML file watcher for hot-reload |
