# Claude Code Provider Integration Guide

*Read this in other languages: [English](claudecode.md), [简体中文](claudecode_zh.md).*

This document describes how to integrate and use **Claude Code** within the HotPlex ecosystem.

## Overview

**Claude Code** is the default and most advanced provider in HotPlex. It is highly optimized for interactive, multi-turn coding tasks and integrates seamlessly with HotPlex's **Hot-Multiplexing** technology to provide millisecond-level response times.

> **Note:** HotPlex is fully tested and compatible with **Claude Code v2.1.x** and above, including full support for their latest `modelUsage` token tracking mechanisms.

## Installation

Claude Code must be installed via npm:

```bash
npm install -g @anthropic-ai/claude-code
```

Verify the installation:
```bash
claude --version
```

## Authentication

Before using Claude Code with HotPlex, ensure you are authenticated:
```bash
claude auth login
```

HotPlex will inherit the machine's authentication state.

## Configuration

Claude Code is configured via `hotplex.ClaudeCodeProvider`.

### Provider Options

| Option                  | Type       | Description                                                      |
| :---------------------- | :--------- | :--------------------------------------------------------------- |
| `DefaultPermissionMode` | `string`   | Set to `"bypassPermissions"` to auto-authorize tool usage.       |
| `AllowedTools`          | `[]string` | Whitelist of tools the agent can use (e.g., `["Bash", "Edit"]`). |
| `DisallowedTools`       | `[]string` | Blacklist of tools to forbid.                                    |
| `Model`                 | `string`   | Override the default model (e.g., `claude-3-5-sonnet-20241022`). |

## Key Features

### 1. Hot-Multiplexing
HotPlex maintains a pool of warm Claude Code processes. When a request arrives, it is immediately dispatched to an idle process, eliminating the multi-second cold start of the Node.js CLI.

### 2. Session Persistence
HotPlex uses **Marker Files** to track session state. If the HotPlex engine restarts, it can automatically resume previous Claude sessions by passing the `--resume` flag to the CLI.

### 3. Full-Duplex Streaming
HotPlex standardizes Claude's complex terminal output (including progress bars and spinners) into clean, structured JSON events:
- `thinking`: Captured from Claude's reasoning blocks.
- `tool_use`: Captured when Claude invokes local shell or editor tools.
- `answer`: The final textual response.

## Example: Go Integration

```go
package main

import (
	"context"
	"fmt"
	"github.com/hrygo/hotplex"
)

func main() {
	// 1. Initialize Claude Code Provider (Default)
	claudePrv, _ := hotplex.NewClaudeCodeProvider(hotplex.ProviderConfig{
		DefaultPermissionMode: "bypassPermissions",
		AllowedTools:          []string{"Bash", "Read", "Edit"},
	}, nil)

	// 2. Wrap in Engine
	engine, _ := hotplex.NewEngine(hotplex.EngineOptions{
		Provider: claudePrv,
	})
	defer engine.Close()

	// 3. Execute
	ctx := context.Background()
	engine.Execute(ctx, &hotplex.Config{SessionID: "debug-session"}, "Fix the bug in main.go", 
		func(eventType string, data any) error {
			if eventType == "answer" {
				fmt.Print(data.(*hotplex.EventWithMeta).EventData)
			}
			return nil
		})
}
```

## Security & Isolation

- **PGID Isolation**: HotPlex ensures that every Claude process and its subprocesses (e.g., a dev server started by Claude) are properly tracked and terminated.
- **Instruction WAF**: HotPlex inspects the stream to intercept and block high-risk commands before they execute.
