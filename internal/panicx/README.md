# PanicX Package (`internal/panicx`)

Panic recovery utilities for goroutine safety.

## Overview

This package ensures that panics in spawned goroutines are caught and logged rather than crashing the entire process.

## Recovery Policies

| Policy | Behavior |
|--------|----------|
| `PolicyLogAndContinue` | Log panic, continue operation (default) |
| `PolicyLogAndRestart` | Log panic, signal for restart |
| `PolicyLogAndShutdown` | Log panic, trigger graceful shutdown |

## Usage

```go
import "github.com/hrygo/hotplex/internal/panicx"

// Safe goroutine launch
panicx.SafeGo(logger, func() {
    // This panic will be caught and logged
    panic("something went wrong")
})

// With context
panicx.SafeGoWithContext(ctx, logger, func(ctx context.Context) {
    // Context-aware goroutine
    select {
    case <-ctx.Done():
        return
    case result := <-ch:
        process(result)
    }
})

// With recovery policy
panicx.SafeGoWithPolicy(logger, panicx.PolicyLogAndShutdown, "critical-worker", func() {
    // Critical goroutine - shutdown on panic
})
```

## Functions

| Function | Purpose |
|----------|---------|
| `SafeGo` | Launch goroutine with panic recovery |
| `SafeGoWithContext` | Context-aware goroutine with recovery |
| `SafeGoWithPolicy` | Goroutine with specific recovery policy |
| `Recover` | Low-level recovery function |

## Files

| File | Purpose |
|------|---------|
| `goroutine.go` | Panic recovery utilities |
