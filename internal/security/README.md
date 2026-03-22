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

// Create detector with default rules
detector := security.NewDetector(logger)

// Check input (returns nil if safe, or *DangerBlockEvent if blocked)
result := detector.CheckInput(userInput)
if result != nil {
    log.Warn("Dangerous input blocked",
        "reason", result.Reason,
        "level", result.Level,
        "pattern", result.PatternMatched)
    return ErrSecurityBlock
}
```

The detector uses a 4-level severity system:
- **Critical (0)**: Irreparable damage (e.g., recursive root deletion, disk wiping)
- **High (1)**: Significant damage potential (e.g., deleting user home, modifying system config)
- **Moderate (2)**: Unintended side effects (e.g., resetting Git history, sensitive recon)
- **Safe (-1)**: Allowlisted patterns that bypass further checks

Commonly blocked patterns:
- `rm -rf /` - Recursive root deletion
- Credential exfiltration via `curl` or `cat /etc/shadow`
- Reverse shells and network listener attempts
- Sudo/Privilege escalation

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
| `detector.go` | Core WAF engine and regex signature implementation |
| `rules/` | Extensible rule sources (File, Memory, API) |
| `audit/` | Forensic logging and session audit persistence |
| `doc.go` | Package-level documentation and overview |
