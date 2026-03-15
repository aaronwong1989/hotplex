# StrUtil Package (`internal/strutil`)

High-performance string utilities.

## Overview

This package provides optimized string manipulation functions used throughout HotPlex, particularly in security-critical paths like regex scanning.

## Usage

```go
import "github.com/hrygo/hotplex/internal/strutil"

// Truncate string with ellipsis
short := strutil.Truncate("very long string...", 10)
// Result: "very lo..."

// Safe substring (handles Unicode)
sub := strutil.SafeSubstring("Hello 世界", 0, 8)
```

## Design Goals

- **Zero Allocation**: Critical paths minimize garbage collection
- **Unicode Safe**: Proper handling of multi-byte characters
- **Security Focused**: Used in WAF pattern matching

## Files

| File | Purpose |
|------|---------|
| `strutil.go` | String utility functions |
