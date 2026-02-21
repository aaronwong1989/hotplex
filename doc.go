// Package hotplex provides a production-grade Control Plane for managing persistent,
// hot-multiplexed AI CLI agent sessions (e.g., Claude Code, Aider, OpenCode).
//
// Following the "First Principle" of leveraging existing, state-of-the-art AI tools,
// HotPlex bridges the gap between terminal-based agents and cloud-native systems.
// It resolves "cold start" latency by maintaining a pool of long-lived, secure
// execution environments, allowing developers to build enterprise-grade AI assistants
// or CI/CD pipelines without reinventing agent logic.
//
// Security is enforced via a native tool capability whitelist and an integrated
// Web Application Firewall (WAF) that inspects commands before execution.
// HotPlex provides real-time streaming, token tracking, and cost reporting.
package hotplex
