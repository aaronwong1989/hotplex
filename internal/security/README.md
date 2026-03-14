# Security Package (`internal/security`)

Regex-based WAF and danger detection for HotPlex.

## Overview

This package implements the core security engine with a **Danger Detector** that uses high-performance regex matching to prevent dangerous commands before execution.

## Key Features

- **Pattern Matching**: Regex-based detection of dangerous commands
- **Rule Sources**: Extensible rule loading interface
- **Audit Logging**: All security decisions are logged
- **Severity Levels**: Warning vs. Block actions

## Usage

```go
import "github.com/hrygo/hotplex/internal/security"

// Create detector
detector := security.NewDangerDetector(logger)

// Check input
result := detector.CheckInput(ctx, userInput)
if result.Blocked {
    log.Warn("Dangerous input blocked", "pattern", result.MatchedPattern)
    return ErrSecurityBlock
}
```

## Detection Rules

The detector blocks patterns like:
- `rm -rf /` - Recursive root deletion
- Credential exfiltration attempts
- Shell injection patterns
- Sensitive file access

## Architecture

```
RuleSource interface
    ├── LoadRules(ctx) ([]SecurityRule, error)
    └── Name() string

SecurityRule
    ├── Pattern *regexp.Regexp
    ├── Severity SeverityLevel
    └── Description string
```

## Files

| File | Purpose |
|------|---------|
| `detector.go` | Core detection engine and interfaces |
