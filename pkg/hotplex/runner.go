package hotplex

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// EngineOptions defines the configuration parameters for initializing a new HotPlex Engine.
// It allows customization of timeouts, logging, pricing models, and session isolation namespaces.
type EngineOptions struct {
	Timeout time.Duration
	Logger  *slog.Logger
	// Configurable pricing model for token calculation (USD per million tokens)
	InputCostPerMillion  float64
	OutputCostPerMillion float64
	// Namespace is a configurable string used to generate deterministic UUID v5 Session IDs.
	// This ensures that the same Conversation ID creates an isolated but persistent sandbox.
	Namespace string

	// Foundational Security & Context (Engine-level boundaries)
	PermissionMode     string   // Controls CLI permissions: "default", "bypassPermissions", etc.
	BaseSystemPrompt   string   // Foundational system prompt injected at startup for all sessions
	GlobalAllowedPaths []string // Baseline path whitelist for file access (shared across all sessions)
	ForbiddenPaths     []string // Absolute path blacklist for file access (cannot be overridden by sessions)
}

// Engine is the unified process integration layer for Hot-Multiplexing.
// Configured as a long-lived Singleton, it provides a persistent execution engine that manages
// a pool of active Claude Code CLI processes, applying security rules (WAF) and routing traffic.
type Engine struct {
	opts           EngineOptions
	cliPath        string
	logger         *slog.Logger
	manager        SessionManager
	dangerDetector *Detector
	// Session stats for the last execution (thread-safe)
	statsMu      sync.RWMutex
	currentStats *SessionStats
}

// ConversationIDToSessionID converts a database ConversationID to a deterministic string.
// By combining a namespace (e.g., "geek_userId" or "evolution_userId") with the conversation ID,
// we guarantee physical sandbox isolation between different modes and users, while enabling
// reliable session resume (Hot-Multiplexing) across backend requests.
func (r *Engine) ConversationIDToSessionID(conversationID int64) string {
	// Let SessionManager handle the UUID mapping, we just return a deterministic string
	return fmt.Sprintf("%s:conversation:%d", r.opts.Namespace, conversationID)
}

// NewEngine creates a new HotPlex Engine instance.
func NewEngine(options EngineOptions) (HotPlexClient, error) {
	cliPath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude Code CLI not found: %w", err)
	}

	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if options.Timeout == 0 {
		options.Timeout = 5 * time.Minute
	}

	if options.Namespace == "" {
		options.Namespace = "default"
	}

	// Initialize danger detector for security
	dangerDetector := NewDetector(logger)

	return &Engine{
		opts:           options,
		cliPath:        cliPath,
		logger:         logger,
		manager:        NewSessionPool(logger, 30*time.Minute, options), // Default 30m idle timeout
		dangerDetector: dangerDetector,
	}, nil
}

// Close terminates all active sessions managed by this runner and cleans up resources.
// It triggers Graceful Shutdown by cascading termination signals down to the SessionManager,
// which drops the entire process group (PGID) to prevent zombie processes.
func (r *Engine) Close() error {
	r.logger.Info("Closing Engine and sweeping all active pgid sessions", "component", "Engine")

	r.manager.Shutdown()

	return nil
}

// Execute runs Claude Code CLI with the given configuration and streams
func (r *Engine) Execute(ctx context.Context, cfg *Config, prompt string, callback Callback) error {
	// Security check: Detect dangerous operations before execution
	// All prompts now undergo WAF checking regardless of origin
	if dangerEvent := r.dangerDetector.CheckInput(prompt); dangerEvent != nil {
		r.logger.Warn("Dangerous operation blocked by regex firewall",
			"operation", dangerEvent.Operation,
			"reason", dangerEvent.Reason,
			"level", dangerEvent.Level,
		)
		// Send danger block event to client (non-critical - error already being returned)
		callbackSafe := WrapSafe(r.logger, callback)
		if callbackSafe != nil {
			callbackSafe("danger_block", dangerEvent)
		}
		return ErrDangerBlocked
	}

	// Derive SessionID from ConversationID using UUID v5 for deterministic mapping.
	if cfg.SessionID == "" && cfg.ConversationID > 0 {
		cfg.SessionID = r.ConversationIDToSessionID(cfg.ConversationID)
		r.logger.Debug("Engine: derived SessionID from ConversationID",
			"conversation_id", cfg.ConversationID,
			"session_id", cfg.SessionID)
	}

	// Validate configuration
	if err := r.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Ensure working directory exists
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	// Initialize session stats for observability
	stats := &SessionStats{
		SessionID: cfg.SessionID,
		StartTime: time.Now(),
	}

	// Send thinking event
	callbackSafe := WrapSafe(r.logger, callback)
	if callbackSafe != nil {
		meta := &EventMeta{
			Status:          "running",
			TotalDurationMs: 0,
		}
		callbackSafe("thinking", &EventWithMeta{EventType: "thinking", EventData: "ai.thinking", Meta: meta})
	}

	r.logger.Info("Engine: session pipeline ready for hot-multiplexing",
		"session_id", cfg.SessionID,
		"user_id", cfg.UserID,
	)

	// Execute via multiplexed persistent session
	if err := r.executeWithMultiplex(ctx, cfg, prompt, callback, stats); err != nil {
		r.logger.Error("Engine: execution failed",
			"user_id", cfg.UserID,
			"error", err)
		return err
	}

	// Finalize and save session stats
	if stats.TotalDurationMs <= 1 {
		measuredDuration := time.Since(stats.StartTime).Milliseconds()
		if measuredDuration > stats.TotalDurationMs {
			stats.TotalDurationMs = measuredDuration
		}
	}
	r.statsMu.Lock()
	r.currentStats = stats
	r.statsMu.Unlock()

	r.logger.Info("Engine: Session completed",
		"session_id", stats.SessionID,
		"total_duration_ms", stats.TotalDurationMs,
		"tool_duration_ms", stats.ToolDurationMs,
		"tool_calls", stats.ToolCallCount,
		"tools_used", len(stats.ToolsUsed))

	return nil
}

// GetSessionStats returns a copy of the current session stats.
func (r *Engine) GetSessionStats() *SessionStats {
	r.statsMu.Lock()
	defer r.statsMu.Unlock()

	if r.currentStats == nil {
		return nil
	}

	// Finalize any ongoing phases before copying
	return r.currentStats.FinalizeDuration()
}

// ValidateConfig validates the Config.
func (r *Engine) ValidateConfig(cfg *Config) error {
	if cfg.WorkDir == "" {
		return fmt.Errorf("work_dir is required")
	}
	if cfg.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if cfg.UserID == 0 {
		return fmt.Errorf("user_id is required")
	}
	return nil
}

// executeWithMultiplex uses the SessionManager for persistent process Hot-Multiplexing.
// Instead of repeatedly spawning heavy Node.js CLI processes, it looks up the deterministic SessionID.
// If missing, it performs a Cold Start. If present, it directly pipes the `prompt` via Stdin (Hot-Multiplexing).
// System prompt is injected only at cold startup; subsequent turns send user messages via stdin.
func (r *Engine) executeWithMultiplex(
	ctx context.Context,
	cfg *Config,
	prompt string,
	callback Callback,
	stats *SessionStats,
) error {
	// Build system prompt (passed to SessionManager for first-time process creation only)
	taskSystemPrompt := cfg.TaskSystemPrompt
	if taskSystemPrompt == "" {
		taskSystemPrompt = BuildSystemPrompt(cfg.WorkDir, cfg.SessionID, cfg.UserID, cfg.DeviceContext)
	}

	smCfg := Config{
		WorkDir:             cfg.WorkDir,
		TaskSystemPrompt:    taskSystemPrompt,
		SessionAllowedPaths: cfg.SessionAllowedPaths,
	}

	// GetOrCreateSession reuses existing process or starts a new one
	sess, err := r.manager.GetOrCreateSession(ctx, cfg.SessionID, smCfg)
	if err != nil {
		return fmt.Errorf("get or create session: %w", err)
	}

	r.logger.Info("Engine: session pipeline ready for hot-multiplexing",
		"session_id", cfg.SessionID,
		"user_id", cfg.UserID)

	// Wait for session to be ready (process fully started)
	readyCtx, readyCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readyCancel()
	for {
		status := sess.GetStatus()
		if status == SessionStatusReady || status == SessionStatusBusy {
			break
		}
		if status == SessionStatusDead {
			return fmt.Errorf("session %s is dead, cannot execute", cfg.SessionID)
		}
		select {
		case <-readyCtx.Done():
			return fmt.Errorf("session %s not ready within 10s (status: %s)", cfg.SessionID, status)
		case <-time.After(200 * time.Millisecond):
			// poll again
		}
	}

	// Create doneChan for this turn
	doneChan := make(chan struct{})

	// Bridge callback: wraps the caller's Callback with metadata enrichment
	bridge := func(eventType string, data any) error {
		if eventType == "runner_exit" {
			select {
			case <-doneChan:
			default:
				close(doneChan)
			}
			return nil
		}

		if eventType == "raw_line" {
			line, ok := data.(string)
			if !ok {
				return nil
			}

			var msg StreamMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				// Not JSON, handle gracefully
				callbackSafe := WrapSafe(r.logger, callback)
				if callbackSafe != nil {
					callbackSafe("answer", line)
				}
				return nil
			}

			// Handle result message — extract stats and send session_stats event
			if msg.Type == "result" {
				r.handleResultMessage(msg, stats, cfg, callback)
				select {
				case <-doneChan:
				default:
					close(doneChan)
				}
				return nil
			}

			if msg.Type == "error" {
				select {
				case <-doneChan:
				default:
					close(doneChan)
				}
			}

			// Silently consume system messages (init, hooks)
			if msg.Type == "system" {
				return nil
			}

			// Dispatch all other events (assistant, tool_use, error, etc.) with metadata
			if callback != nil {
				return r.dispatchCallback(msg, callback, stats)
			}
			return nil
		}

		// Legacy path for pre-parsed
		msg, ok := data.(StreamMessage)
		if !ok {
			callbackSafe := WrapSafe(r.logger, callback)
			if callbackSafe != nil {
				callbackSafe(eventType, data)
			}
			return nil
		}

		if msg.Type == "result" {
			r.handleResultMessage(msg, stats, cfg, callback)
			return nil
		}

		if msg.Type == "system" {
			return nil
		}

		if callback != nil {
			return r.dispatchCallback(msg, callback, stats)
		}
		return nil
	}

	sess.SetCallback(bridge)

	// Build stream-json user message payload
	msgPayload := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": prompt},
			},
		},
	}

	// Send user message to CLI stdin
	if err := sess.WriteInput(msgPayload); err != nil {
		return fmt.Errorf("write input: %w", err)
	}

	// Wait for turn completion with timeout
	timer := time.NewTimer(r.opts.Timeout)
	defer timer.Stop()

	select {
	case <-doneChan:
		// Turn completed successfully
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("execution timeout after %v", r.opts.Timeout)
	}
}

// handleResultMessage processes the result message from CLI, extracts statistics,
// and sends session_stats event to frontend.
func (r *Engine) handleResultMessage(msg StreamMessage, stats *SessionStats, cfg *Config, callback Callback) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	// Update final duration from CLI report
	if msg.Duration > 0 {
		stats.TotalDurationMs = int64(msg.Duration)
	}

	// Update token usage from CLI report
	if msg.Usage != nil {
		stats.InputTokens = msg.Usage.InputTokens
		stats.OutputTokens = msg.Usage.OutputTokens
		stats.CacheWriteTokens = msg.Usage.CacheWriteInputTokens
		stats.CacheReadTokens = msg.Usage.CacheReadInputTokens
	}

	// Collect tools used (convert map to slice)
	toolsUsed := make([]string, 0, len(stats.ToolsUsed))
	for tool := range stats.ToolsUsed {
		toolsUsed = append(toolsUsed, tool)
	}

	// Collect file paths (with deduplication)
	filePathsSet := make(map[string]bool, len(stats.FilePaths))
	for _, path := range stats.FilePaths {
		if path != "" {
			filePathsSet[path] = true
		}
	}
	filePaths := make([]string, 0, len(filePathsSet))
	for path := range filePathsSet {
		filePaths = append(filePaths, path)
	}

	// Calculate total cost with fallback if CLI doesn't report it
	totalCostUSD := msg.TotalCostUSD
	if totalCostUSD == 0 && stats.InputTokens+stats.OutputTokens > 0 {
		var inputCostPerMillion, outputCostPerMillion float64
		if r.opts.InputCostPerMillion > 0 {
			inputCostPerMillion = r.opts.InputCostPerMillion
		}
		if r.opts.OutputCostPerMillion > 0 {
			outputCostPerMillion = r.opts.OutputCostPerMillion
		}

		inputCost := float64(stats.InputTokens) * inputCostPerMillion / 1_000_000
		outputCost := float64(stats.OutputTokens) * outputCostPerMillion / 1_000_000
		totalCostUSD = inputCost + outputCost
	}

	// Log session completion stats with explicit performance markers
	r.logger.Info("Engine: multiplexed turn completed",
		"session_id", cfg.SessionID,
		"duration_ms", stats.TotalDurationMs,
		"input_tokens", stats.InputTokens,
		"output_tokens", stats.OutputTokens,
		"total_cost_usd", msg.TotalCostUSD,
		"tool_calls", stats.ToolCallCount,
		"files_modified", stats.FilesModified)

	// Send session_stats event to frontend (non-critical)
	if callback != nil {
		callbackSafe := WrapSafe(r.logger, callback)
		callbackSafe("session_stats", &SessionStatsData{
			SessionID:            cfg.SessionID,
			ConversationID:       cfg.ConversationID,
			UserID:               cfg.UserID,
			StartTime:            stats.StartTime.Unix(),
			EndTime:              time.Now().Unix(),
			TotalDurationMs:      stats.TotalDurationMs,
			ThinkingDurationMs:   stats.ThinkingDurationMs,
			ToolDurationMs:       stats.ToolDurationMs,
			GenerationDurationMs: stats.GenerationDurationMs,
			InputTokens:          stats.InputTokens,
			OutputTokens:         stats.OutputTokens,
			CacheWriteTokens:     stats.CacheWriteTokens,
			CacheReadTokens:      stats.CacheReadTokens,
			TotalTokens:          stats.InputTokens + stats.OutputTokens,
			ToolCallCount:        stats.ToolCallCount,
			ToolsUsed:            toolsUsed,
			FilesModified:        stats.FilesModified,
			FilePaths:            filePaths,
			ModelUsed:            "claude-code",
			TotalCostUSD:         totalCostUSD,
			IsError:              msg.IsError,
			ErrorMessage:         msg.Error,
		})
	}
}

// dispatchCallback dispatches stream events to the callback with metadata.
// IMPORTANT: This function is called from stream goroutines. The callback MUST:
// 1. Return quickly (< 5 seconds) to avoid blocking stream processing
// 2. NOT call back into Session/Engine methods (risk of deadlock)
// 3. Be safe for concurrent invocation from multiple goroutines
func (r *Engine) dispatchCallback(msg StreamMessage, callback Callback, stats *SessionStats) error {
	// Skip processing if stats is nil (can happen during session warmup or reuse)
	if stats == nil {
		r.logger.Debug("dispatchCallback: stats is nil, skipping event processing",
			"type", msg.Type, "subtype", msg.Subtype)
		return nil
	}

	// Calculate total duration
	totalDuration := time.Since(stats.StartTime).Milliseconds()

	switch msg.Type {
	case "error":
		if msg.Error != "" {
			return callback("error", msg.Error)
		}
	case "system":
		// System messages (init, hook_started, hook_response) are already handled
		// by SessionMonitor for CLI readiness detection. No additional processing needed.
	case "thinking", "status":
		// Start thinking phase tracking (ended in other cases or by defer)
		stats.StartThinking()
		// Ensure thinking is ended even if we return early from this case
		// Note: if control flows to another case (tool_use, assistant), they will end thinking explicitly
		defer func() {
			stats.EndThinking()
		}()

		for _, block := range msg.GetContentBlocks() {
			if block.Type == "text" && block.Text != "" {
				meta := &EventMeta{
					Status:          "running",
					TotalDurationMs: totalDuration,
				}
				if err := callback("thinking", &EventWithMeta{EventType: "thinking", EventData: block.Text, Meta: meta}); err != nil {
					return err
				}
			}
		}
	case "tool_use":
		// Tool use ends thinking, starts tool execution
		stats.EndThinking()

		if msg.Name != "" {
			// Extract tool ID and input from content blocks
			var toolID string
			var inputSummary string
			var filePath string
			for _, block := range msg.GetContentBlocks() {
				if block.Type == "tool_use" {
					toolID = block.ID
					if block.Input != nil {
						// Create a human-readable summary of the input
						inputSummary = SummarizeInput(block.Input)

						// Extract file path for Write/Edit operations
						if msg.Name == "Write" || msg.Name == "Edit" || msg.Name == "WriteFile" || msg.Name == "EditFile" {
							if path, ok := block.Input["path"].(string); ok {
								filePath = path
							}
						}
					}
				}
			}
			stats.RecordToolUse(msg.Name, toolID)

			// Record file modification for Write/Edit tools
			if filePath != "" {
				stats.RecordFileModification(filePath)
			}

			meta := &EventMeta{
				ToolName:        msg.Name,
				ToolID:          toolID,
				Status:          "running",
				TotalDurationMs: totalDuration,
				InputSummary:    inputSummary,
			}
			r.logger.Debug("Engine: sending tool_use event", "tool_name", msg.Name, "tool_id", toolID)
			if err := callback("tool_use", &EventWithMeta{EventType: "tool_use", EventData: msg.Name, Meta: meta}); err != nil {
				return err
			}
		}
	case "tool_result":
		if msg.Output != "" {
			durationMs := stats.RecordToolResult()

			// Extract tool ID and name from content blocks for matching with tool_use
			// tool_result blocks use tool_use_id to reference the corresponding tool_use
			var toolID string
			var toolName string
			for _, block := range msg.GetContentBlocks() {
				if block.Type == "tool_result" {
					toolID = block.GetUnifiedToolID()
					toolName = block.Name // Tool name from content block
					break
				}
			}

			meta := &EventMeta{
				ToolName:        toolName,
				ToolID:          toolID,
				Status:          "success",
				DurationMs:      durationMs,
				TotalDurationMs: totalDuration,
				OutputSummary:   TruncateString(msg.Output, 500),
			}
			r.logger.Debug("Engine: sending tool_result event", "tool_name", toolName, "tool_id", toolID, "output_length", len(msg.Output), "duration_ms", durationMs)
			if err := callback("tool_result", &EventWithMeta{EventType: "tool_result", EventData: msg.Output, Meta: meta}); err != nil {
				return err
			}
		}
	case "message", "content", "text", "delta", "assistant":
		// Assistant message starts generation phase
		r.logger.Debug("dispatchCallback: processing assistant message",
			"type", msg.Type,
			"has_message", msg.Message != nil,
			"has_direct_content", len(msg.Content) > 0,
			"blocks_count", len(msg.GetContentBlocks()))
		stats.EndThinking()
		stats.StartGeneration()

		for _, block := range msg.GetContentBlocks() {
			if block.Type == "text" && block.Text != "" {
				if err := callback("answer", &EventWithMeta{EventType: "answer", EventData: block.Text, Meta: &EventMeta{TotalDurationMs: totalDuration}}); err != nil {
					return err
				}
			} else if block.Type == "tool_use" && block.Name != "" {
				// Tool use is nested inside assistant message content
				// End generation when tool is about to be used
				stats.EndGeneration()

				r.logger.Debug("Engine: processing tool_use block", "tool_name", block.Name, "tool_id", block.ID)

				stats.RecordToolUse(block.Name, block.ID)

				// Record file modification for Write/Edit tools
				if block.Name == "Write" || block.Name == "Edit" || block.Name == "WriteFile" || block.Name == "EditFile" {
					if block.Input != nil {
						if path, ok := block.Input["path"].(string); ok {
							stats.RecordFileModification(path)
						}
					}
				}

				meta := &EventMeta{
					ToolName:        block.Name,
					ToolID:          block.ID,
					Status:          "running",
					TotalDurationMs: totalDuration,
					InputSummary:    SummarizeInput(block.Input),
				}
				if err := callback("tool_use", &EventWithMeta{EventType: "tool_use", EventData: block.Name, Meta: meta}); err != nil {
					return err
				}
				r.logger.Debug("Engine: tool_use callback completed", "tool_name", block.Name, "tool_id", block.ID)
			}
		}
	case "user":
		// Tool results come as type:"user" with nested tool_result blocks
		for _, block := range msg.GetContentBlocks() {
			if block.Type != "tool_result" {
				continue
			}

			durationMs := stats.RecordToolResult()

			// tool_result blocks use tool_use_id to reference the corresponding tool_use
			// The Name field is typically empty in tool_result blocks
			toolID := block.GetUnifiedToolID()

			meta := &EventMeta{
				ToolID:          toolID,     // Use tool_use_id for matching
				ToolName:        block.Name, // May be empty for tool_result blocks
				Status:          "success",
				DurationMs:      durationMs,
				TotalDurationMs: totalDuration,
				OutputSummary:   TruncateString(block.Content, 500),
			}
			r.logger.Debug("Engine: sending tool_result event from user message", "tool_name", block.Name, "tool_id", toolID, "tool_use_id", block.ToolUseID, "duration_ms", durationMs)
			if err := callback("tool_result", &EventWithMeta{EventType: "tool_result", EventData: block.Content, Meta: meta}); err != nil {
				return err
			}
		}
	default:
		// Log unknown message type for debugging
		r.logger.Warn("Engine: unknown message type",
			"type", msg.Type,
			"role", msg.Role,
			"name", msg.Name,
			"has_content", len(msg.Content) > 0,
			"has_message", msg.Message != nil,
			"has_error", msg.Error != "",
			"has_output", msg.Output != "")

		// Try to extract any text content (non-critical - use safe callback)
		callbackSafe := WrapSafe(r.logger, callback)
		for _, block := range msg.GetContentBlocks() {
			if block.Type == "text" && block.Text != "" {
				if callbackSafe != nil {
					callbackSafe("answer", &EventWithMeta{EventType: "answer", EventData: block.Text, Meta: &EventMeta{TotalDurationMs: totalDuration}})
				}
			}
		}
	}
	return nil
}

// GetCLIVersion returns the Claude Code CLI version.
func (r *Engine) GetCLIVersion() (string, error) {
	cmd := exec.Command(r.cliPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get CLI version: %w", err)
	}
	return string(output), nil
}

// StopSession terminates a running session by session ID.
// This is the implementation for session.stop from the spec.
func (r *Engine) StopSession(sessionID string, reason string) error {
	r.logger.Info("Engine: stopping session",
		"session_id", sessionID,
		"reason", reason)

	return r.manager.TerminateSession(sessionID)
}

// StopSessionByConversationID terminates a session by its conversation ID.
func (r *Engine) StopSessionByConversationID(conversationID int64, reason string) error {
	sessionID := r.ConversationIDToSessionID(conversationID)
	return r.StopSession(sessionID, reason)
}

// SetDangerAllowPaths sets the allowed safe paths for the danger detector.
func (r *Engine) SetDangerAllowPaths(paths []string) {
	r.dangerDetector.SetAllowPaths(paths)
}

// SetDangerBypassEnabled enables or disables danger detection bypass.
// WARNING: Only use for Evolution mode (admin only).
func (r *Engine) SetDangerBypassEnabled(enabled bool) {
	r.dangerDetector.SetBypassEnabled(enabled)
}

// GetDangerDetector returns the danger detector instance.
func (r *Engine) GetDangerDetector() *Detector {
	return r.dangerDetector
}

// BuildSystemPrompt provides minimal, high-signal context for Claude Code CLI.
func BuildSystemPrompt(workDir, sessionID string, userID int32, deviceContext string) string {
	return BuildSystemPromptWithRuntime(workDir, sessionID, userID, deviceContext, getRuntimeInfo())
}

// BuildSystemPromptWithRuntime is the implementation that allows runtime info injection.
func BuildSystemPromptWithRuntime(workDir, sessionID string, userID int32, deviceContext string, runtimeInfo RuntimeInfo) string {
	osName := runtimeInfo.OS
	arch := runtimeInfo.Arch
	if osName == "darwin" {
		osName = "macOS"
	}

	timestamp := runtimeInfo.Timestamp.Format("2006-01-02 15:04:05")

	// Try to parse device context for better formatting
	var contextMap map[string]any
	userAgent := "Unknown"
	deviceInfo := "Unknown"
	if deviceContext != "" {
		// Optimization: only attempt JSON parse if it looks like JSON
		trimmed := strings.TrimSpace(deviceContext)
		if strings.HasPrefix(trimmed, "{") {
			if err := json.Unmarshal([]byte(deviceContext), &contextMap); err == nil {
				if ua, ok := contextMap["userAgent"].(string); ok {
					userAgent = ua
				}
				if mobile, ok := contextMap["isMobile"].(bool); ok {
					if mobile {
						deviceInfo = "Mobile"
					} else {
						deviceInfo = "Desktop"
					}
				}
				// Add more fields if available (screen, language, etc.)
				if w, ok := contextMap["screenWidth"].(float64); ok {
					if h, ok := contextMap["screenHeight"].(float64); ok {
						deviceInfo = fmt.Sprintf("%s (%dx%d)", deviceInfo, int(w), int(h))
					}
				}
				if lang, ok := contextMap["language"].(string); ok {
					deviceInfo = fmt.Sprintf("%s, Language: %s", deviceInfo, lang)
				}
			} else {
				// Fallback: use raw string if JSON parse failed
				userAgent = deviceContext
			}
		} else {
			// Not JSON - use raw string
			userAgent = deviceContext
		}
	}

	return fmt.Sprintf(`# Context

You are running inside DivineSense, an intelligent assistant system.

**User Interaction**: Users type questions in their web browser, which invokes you via a Go backend. Your response streams back to their browser in real-time. **Always respond in Chinese (Simplified).**

- **User ID**: %d
- **Client Device**: %s
- **User Agent**: %s
- **Server OS**: %s (%s)
- **Time**: %s
- **Workspace**: %s
- **Mode**: Non-interactive headless (--print)
- **Session**: %s (persists via --session-id/--resume)
`, userID, deviceInfo, userAgent, osName, arch, timestamp, workDir, sessionID)
}

// RuntimeInfo contains runtime information for system prompt generation.
type RuntimeInfo struct {
	OS        string
	Arch      string
	Timestamp time.Time
}

// getRuntimeInfo returns the current runtime information.
func getRuntimeInfo() RuntimeInfo {
	return RuntimeInfo{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Timestamp: time.Now(),
	}
}
