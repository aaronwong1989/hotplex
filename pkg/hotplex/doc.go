// Package hotplex provides the core SDK for managing persistent, hot-multiplexed
// Large Language Model (LLM) CLI agent sessions (e.g. Claude Code).
//
// It resolves the "cold start" latency issue sequence by maintaining a pool of
// long-lived execution environments (sandboxes). The SDK enforces strict security
// controls via an embedded Web Application Firewall (WAF) to detect and block
// dangerous commands before they are dispatched to the host system.
//
// HotPlex offers fine-grained stream events and rich usage statistics, making it
// an ideal backend integration layer for AI IDEs or multi-agent orchestration frameworks.
package hotplex
