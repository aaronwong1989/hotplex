package provider

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hrygo/hotplex/internal/persistence"
)

// toWorkspaceKey converts a working directory path to Claude Code's workspace key format.
// Claude Code uses this format to create project-specific directories in ~/.claude/projects/.
// Algorithm: Replace "/." (hidden directories) with "--" first, then replace "/" with "-".
// Example: "/Users/user/.hotplex/projects/app" -> "-Users-user--hotplex-projects-app"
func toWorkspaceKey(cwd string) string {
	// First replace "/." with "--" to handle hidden directories like .hotplex
	// Then replace "/" with "-" to flatten the path
	return strings.ReplaceAll(strings.ReplaceAll(cwd, "/.", "--"), "/", "-")
}

// isValidSessionID checks if a session ID is safe to use in file paths.
// It blocks obvious path traversal attempts while allowing test IDs.
// A valid session ID must:
// - Not be empty
// - Not contain path separators (/ or \)
// - Not contain parent directory references (..)
// - Be alphanumeric with hyphens/underscores only (allows UUIDs and test IDs)
var validSessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isValidSessionID(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	// Block path traversal attempts
	if strings.Contains(sessionID, "..") ||
		strings.Contains(sessionID, "/") ||
		strings.Contains(sessionID, "\\") {
		return false
	}
	return validSessionIDPattern.MatchString(sessionID)
}

// ClaudeCodeProvider implements the Provider interface for Claude Code CLI.
// This is the default provider and maintains full backward compatibility
// with the existing HotPlex implementation.
var _ Provider = (*ClaudeCodeProvider)(nil)

type ClaudeCodeProvider struct {
	ProviderBase
	opts          ProviderConfig
	markerStore   persistence.SessionMarkerStore
	promptBuilder *PromptBuilder
}

// NewClaudeCodeProvider creates a new Claude Code provider instance.
func NewClaudeCodeProvider(cfg ProviderConfig, logger *slog.Logger) (*ClaudeCodeProvider, error) {
	if logger == nil {
		logger = slog.Default()
	}

	meta := ProviderMeta{
		Type:        ProviderTypeClaudeCode,
		DisplayName: "Claude Code",
		BinaryName:  "claude",
		InstallHint: "npm install -g @anthropic-ai/claude-code",
		Features: ProviderFeatures{
			SupportsResume:      true,
			SupportsStreamJSON:  true,
			SupportsSSE:         false,
			SupportsHTTPAPI:     false,
			SupportsSessionID:   true,
			SupportsPermissions: true,
			MultiTurnReady:      true,
		},
	}

	// Resolve binary path using helper
	binaryPath, err := ResolveBinaryPath(cfg, meta)
	if err != nil {
		return nil, err
	}

	// Initialize marker store for session persistence
	markerStore := persistence.NewDefaultFileMarkerStore()

	return &ClaudeCodeProvider{
		ProviderBase: ProviderBase{
			meta:       meta,
			binaryPath: binaryPath,
			logger:     logger.With("provider", "claude-code"),
		},
		opts:          cfg,
		markerStore:   markerStore,
		promptBuilder: NewPromptBuilder(true), // Use CDATA for Claude
	}, nil
}

// BuildCLIArgs constructs Claude Code CLI arguments.
func (p *ClaudeCodeProvider) BuildCLIArgs(providerSessionID string, opts *ProviderSessionOptions) []string {
	args := []string{
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--include-partial-messages",       // Enable streaming of partial content (thinking, etc.)
		"--settings", `{"fastMode":false}`, // Force disable fastMode for Agent SDK compatibility
	}

	// Session management
	if opts.ResumeSession {
		args = append(args, "--resume", providerSessionID)
		p.logger.Debug("Resuming existing Claude Code session", "session_id", providerSessionID)
	} else {
		args = append(args, "--session-id", providerSessionID)
		p.logger.Debug("Creating new Claude Code session", "session_id", providerSessionID)
		// NOTE: Marker is NOT created here anymore!
		// It will be created in pool.go AFTER the CLI process successfully starts.
		// This prevents creating markers for failed session starts.
	}

	// Permission mode
	permMode := opts.PermissionMode
	if permMode == "" && p.opts.DefaultPermissionMode != "" {
		permMode = p.opts.DefaultPermissionMode
	}
	if permMode != "" {
		args = append(args, "--permission-mode", permMode)
	}

	// DangerouslySkipPermissions bypasses all permission checks
	skipPerms := opts.DangerouslySkipPermissions || BoolValue(p.opts.DangerouslySkipPermissions, false)
	if skipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}

	p.logger.Debug("[PROVIDER] permission config",
		"permission_mode", permMode,
		"dangerously_skip_permissions", skipPerms,
		"opts_dangerously_skip", opts.DangerouslySkipPermissions,
		"provider_dangerously_skip", BoolValue(p.opts.DangerouslySkipPermissions, false))

	// Tool restrictions (merge provider-level and session-level)
	allowedTools := mergeStringSlices(p.opts.AllowedTools, opts.AllowedTools)
	if len(allowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(allowedTools, ","))
	}

	disallowedTools := mergeStringSlices(p.opts.DisallowedTools, opts.DisallowedTools)
	if len(disallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(disallowedTools, ","))
	}

	// System prompt (base level only - task prompt is injected per-turn)
	if opts.BaseSystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.BaseSystemPrompt)
	}

	// Model override
	model := opts.Model
	if model == "" && p.opts.DefaultModel != "" {
		model = p.opts.DefaultModel
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// Extra arguments from config
	if len(p.opts.ExtraArgs) > 0 {
		args = append(args, p.opts.ExtraArgs...)
	}

	return args
}

// BuildInputMessage constructs the stream-json input message.
// baseSystemPrompt is ignored — Claude Code handles it via --append-system-prompt in BuildCLIArgs.
func (p *ClaudeCodeProvider) BuildInputMessage(prompt string, taskInstructions string, _ string) (map[string]any, error) {
	finalPrompt := p.promptBuilder.Build(prompt, taskInstructions)

	return map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": finalPrompt},
			},
		},
	}, nil
}

// ParseEvent parses a Claude Code stream-json line into one or more ProviderEvents.
func (p *ClaudeCodeProvider) ParseEvent(line string) ([]*ProviderEvent, error) {
	var msg StreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		// Not valid JSON, return as raw content
		return []*ProviderEvent{{
			Type:    EventTypeRaw,
			RawType: "raw",
			Content: line,
			RawLine: line,
		}}, nil
	}

	// Helper to create a base event from common fields
	newBaseEvent := func(evtType ProviderEventType) *ProviderEvent {
		return &ProviderEvent{
			Type:      evtType,
			RawType:   msg.Type,
			SessionID: msg.SessionID,
			Status:    msg.Status,
			Error:     msg.Error,
			IsError:   msg.IsError,
			RawLine:   line,
			Timestamp: time.Now(),
		}
	}

	var events []*ProviderEvent

	// Map Claude Code types to normalized types
	switch msg.Type {
	case "result":
		if msg.Result == "" && msg.Content == nil && len(msg.ModelUsage) == 0 {
			p.logger.Warn("[PROVIDER] received empty result event", "line", line)
		}
		event := newBaseEvent(EventTypeResult)
		event.Content = msg.Result
		// Always create metadata for result events to ensure tokens are tracked
		dur := msg.Duration
		if dur == 0 {
			dur = msg.DurationMs
		}
		event.Metadata = &ProviderEventMeta{
			DurationMs:      int64(dur),
			TotalDurationMs: int64(dur),
			TotalCostUSD:    msg.TotalCostUSD,
		}
		// Extract tokens from ModelUsage (new Claude Code versions) or fallback to Usage
		var totalInput, totalOutput, totalCacheWrite, totalCacheRead int32
		var totalCost float64
		hasModelUsage := false
		var contextWindow int32
		var maxOutputTokens int32
		var primaryModelName string
		var primaryModelInput int32
		if len(msg.ModelUsage) > 0 {
			for modelName, mUsage := range msg.ModelUsage {
				totalInput += mUsage.InputTokens
				totalOutput += mUsage.OutputTokens
				totalCacheWrite += mUsage.CacheCreationInputTokens
				totalCacheRead += mUsage.CacheReadInputTokens
				totalCost += mUsage.CostUSD
				// Select the most active model (highest inputTokens) for context window.
				// This handles multi-model sessions where /model switching creates
				// multiple entries in modelUsage with different contextWindow values.
				if mUsage.InputTokens > primaryModelInput || primaryModelName == "" {
					primaryModelInput = mUsage.InputTokens
					primaryModelName = modelName
					if mUsage.ContextWindow > 0 {
						contextWindow = mUsage.ContextWindow
					}
					if mUsage.MaxOutputTokens > 0 {
						maxOutputTokens = mUsage.MaxOutputTokens
					}
				}
			}
			if totalInput > 0 || totalOutput > 0 || totalCacheWrite > 0 || totalCacheRead > 0 {
				hasModelUsage = true
			}
		}

		if hasModelUsage {
			event.Metadata.InputTokens = totalInput
			event.Metadata.OutputTokens = totalOutput
			event.Metadata.CacheWriteTokens = totalCacheWrite
			event.Metadata.CacheReadTokens = totalCacheRead
			event.Metadata.ContextWindow = contextWindow
			event.Metadata.MaxOutputTokens = maxOutputTokens
			event.Metadata.ModelName = primaryModelName
			if event.Metadata.TotalCostUSD == 0 {
				event.Metadata.TotalCostUSD = totalCost
			}
		} else if msg.Usage != nil {
			event.Metadata.InputTokens = msg.Usage.InputTokens
			event.Metadata.OutputTokens = msg.Usage.OutputTokens
			event.Metadata.CacheWriteTokens = msg.Usage.CacheWriteInputTokens
			event.Metadata.CacheReadTokens = msg.Usage.CacheReadInputTokens
		} else {
			// Debug: log that usage is missing
			p.logger.Warn("[PROVIDER] result event missing usage data", "line", line)
		}

		// Extract permission denials - each denied tool gets its own event
		if len(msg.PermissionDenials) > 0 {
			p.logger.Debug("[PROVIDER] found permission denials in result event",
				"count", len(msg.PermissionDenials))
			for _, denial := range msg.PermissionDenials {
				denialEvent := newBaseEvent(EventTypePermissionDenied)
				denialEvent.ToolName = denial.ToolName
				denialEvent.ToolID = denial.ToolUseID
				denialEvent.ToolInput = denial.ToolInput
				denialEvent.Content = fmt.Sprintf("Tool '%s' permission denied", denial.ToolName)
				events = append(events, denialEvent)
			}
		}

		events = append(events, event)

	case "error":
		if msg.Error == "" {
			msg.Error = "Unknown provider error (empty error field)"
		}
		event := newBaseEvent(EventTypeError)
		event.Content = msg.Error
		events = append(events, event)

	case "thinking", "think", "think_delta", "status":
		blocks := msg.GetContentBlocks()
		if len(blocks) > 0 {
			for _, block := range blocks {
				switch block.Type {
				case "text":
					if block.Text != "" {
						ev := newBaseEvent(EventTypeThinking)
						if msg.Subtype == "plan_generation" {
							ev.Type = EventTypePlanMode
						}
						ev.Content = block.Text
						events = append(events, ev)
					}
				case "tool_use":
					ev := newBaseEvent(EventTypeToolUse)
					ev.ToolName = block.Name
					ev.ToolID = block.ID
					ev.ToolInput = block.Input
					events = append(events, ev)
				}
			}
		}

		// Fallback for direct content or status if no blocks were processed
		if len(events) == 0 {
			ev := newBaseEvent(EventTypeThinking)
			if msg.Subtype == "plan_generation" {
				ev.Type = EventTypePlanMode
			}
			ev.Content = msg.Status
			if ev.Content == "" {
				ev.Content = "Thinking..."
			}
			events = append(events, ev)
		}

	case "tool_use":
		// Handle special tool types
		switch msg.Name {
		case "ExitPlanMode":
			event := newBaseEvent(EventTypeExitPlanMode)
			event.ToolName = msg.Name
			if msg.Input != nil {
				if plan, ok := msg.Input["plan"].(string); ok {
					event.Content = plan
				}
			}
			events = append(events, event)
		case "AskUserQuestion":
			event := newBaseEvent(EventTypeAskUserQuestion)
			event.ToolName = msg.Name
			if msg.Input != nil {
				if question, ok := msg.Input["question"].(string); ok {
					event.Content = question
				}
				event.ToolInput = msg.Input
			}
			events = append(events, event)
		default:
			// Normal tool use
			event := newBaseEvent(EventTypeToolUse)
			event.ToolName = msg.Name
			event.ToolInput = msg.Input
			// Try to get more info from blocks (Message-based format)
			for _, block := range msg.GetContentBlocks() {
				if block.Type == "tool_use" {
					if event.ToolName == "" {
						event.ToolName = block.Name
					}
					if event.ToolID == "" {
						event.ToolID = block.ID
					}
					if event.ToolInput == nil {
						event.ToolInput = block.Input
					}
				}
			}
			events = append(events, event)
		}

	case "tool_result":
		event := newBaseEvent(EventTypeToolResult)
		event.Content = msg.Output
		event.ToolID = msg.MessageID // Some use MessageID for result
		event.ToolName = msg.Name

		// Debug log: track tool name for skill detection
		if msg.Name != "" {
			p.logger.Debug("Tool result",
				"tool_name", msg.Name,
				"subtype", msg.Subtype)
		}

		for _, block := range msg.GetContentBlocks() {
			if block.Type == "tool_result" {
				if event.ToolID == "" {
					event.ToolID = block.GetUnifiedToolID()
				}
				if event.ToolName == "" {
					event.ToolName = block.Name
				}
				if event.Content == "" {
					event.Content = block.Content
				}
				if block.IsError {
					event.Status = "error"
					event.IsError = true
				}
			}
		}
		if event.Status == "" {
			event.Status = "success"
		}
		events = append(events, event)

	case "assistant", "message", "content", "text", "delta":
		// Multi-part messages
		blocks := msg.GetContentBlocks()
		for _, block := range blocks {
			switch block.Type {
			case "text":
				if block.Text != "" {
					ev := newBaseEvent(EventTypeAnswer)
					ev.Content = block.Text
					events = append(events, ev)
				}
			case "tool_use":
				ev := newBaseEvent(EventTypeToolUse)
				ev.ToolName = block.Name
				ev.ToolID = block.ID
				ev.ToolInput = block.Input
				events = append(events, ev)
			case "tool_result":
				ev := newBaseEvent(EventTypeToolResult)
				ev.ToolName = block.Name
				ev.ToolID = block.GetUnifiedToolID()
				ev.Content = block.Content
				if block.IsError {
					ev.Status = "error"
					ev.IsError = true
				} else {
					ev.Status = "success"
				}
				events = append(events, ev)
			}
		}
		// Fallback for direct text
		if len(events) == 0 && msg.Result != "" {
			ev := newBaseEvent(EventTypeAnswer)
			ev.Content = msg.Result
			events = append(events, ev)
		}

	case "permission_request":
		event := newBaseEvent(EventTypePermissionRequest)
		if msg.Permission != nil {
			event.ToolName = msg.Permission.Name
			event.Content = msg.Permission.Input
		}
		if msg.Decision != nil {
			event.ToolID = msg.MessageID
			if msg.Decision.Reason != "" {
				event.Content = msg.Decision.Reason + "\n" + event.Content
			}
		}
		events = append(events, event)

	default:
		// Skip "user" type messages - these are tool results or system inputs meant for the AI,
		// not content to be displayed to end users in chat apps.
		if msg.Type == "user" {
			p.logger.Debug("[PROVIDER] skipping user message (likely tool result)", "content_len", len(msg.Content))
			return events, nil
		}
		// Fallback for other unknown types - try to extract ANY text
		for _, block := range msg.GetContentBlocks() {
			if block.Type == "text" && block.Text != "" {
				ev := newBaseEvent(EventTypeAnswer)
				ev.Content = block.Text
				events = append(events, ev)
			}
		}
	}

	return events, nil
}

// DetectTurnEnd checks if the event signals turn completion.
func (p *ClaudeCodeProvider) DetectTurnEnd(event *ProviderEvent) bool {
	return event != nil && (event.Type == EventTypeResult || event.Type == EventTypeError)
}

// ValidateBinary checks if the Claude CLI is available.
func (p *ClaudeCodeProvider) ValidateBinary() (string, error) {
	if p.binaryPath != "" {
		return p.binaryPath, nil
	}
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude Code CLI not found: %w", err)
	}
	return path, nil
}

// GetMarkerDir returns the session marker directory path.
func (p *ClaudeCodeProvider) GetMarkerDir() string {
	return p.markerStore.Dir()
}

// CleanupSession overrides ProviderBase to delete Claude Code's project session file.
// This is necessary when starting a fresh session to prevent "Session ID is already in use" errors
// or when executing a /reset command to completely scrub the context.
func (p *ClaudeCodeProvider) CleanupSession(providerSessionID string, workDir string) error {
	if providerSessionID == "" {
		return nil
	}

	// SECURITY: Validate session ID to prevent path traversal attacks
	if !isValidSessionID(providerSessionID) {
		return fmt.Errorf("invalid session ID format: potential path traversal attack")
	}

	cwd := workDir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = os.TempDir()
		}
	}

	// Claude Code stores sessions in ~/.claude/projects/<workspace-key>/<providerSessionID>.jsonl
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	workspaceKey := toWorkspaceKey(cwd)
	sessionPath := filepath.Join(projectsDir, workspaceKey, providerSessionID+".jsonl")

	// SECURITY: Verify the resolved path is still within projectsDir (prevent path traversal)
	absPath, err := filepath.Abs(sessionPath)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}
	absProjectsDir, err := filepath.Abs(projectsDir)
	if err != nil {
		return fmt.Errorf("resolve projects dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absProjectsDir+string(filepath.Separator)) {
		return fmt.Errorf("path traversal detected: session path outside projects directory")
	}

	// Best effort deletion
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Claude Code session file: %w", err)
	}

	p.logger.Debug("Cleaned up Claude Code session file", "path", absPath)
	return nil
}

// CheckSessionMarker checks if a session marker exists for the given ID.
func (p *ClaudeCodeProvider) CheckSessionMarker(providerSessionID string) bool {
	return p.markerStore.Exists(providerSessionID)
}

// VerifySession checks if a CLI session data file exists and can be resumed.
// This prevents "No conversation found with session ID" errors when the marker
// exists but the CLI session data has been deleted or expired.
func (p *ClaudeCodeProvider) VerifySession(providerSessionID string, workDir string) bool {
	if providerSessionID == "" {
		return false
	}

	// SECURITY: Validate session ID to prevent path traversal attacks
	if !isValidSessionID(providerSessionID) {
		p.logger.Warn("Invalid session ID format rejected", "session_id", providerSessionID)
		return false
	}

	cwd := workDir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = os.TempDir()
		}
	}

	// Claude Code stores sessions in ~/.claude/projects/<workspace-key>/<providerSessionID>.jsonl
	homeDir, err := os.UserHomeDir()
	if err != nil {
		p.logger.Warn("Failed to get home directory for session verification", "error", err)
		return false
	}
	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	workspaceKey := toWorkspaceKey(cwd)
	sessionPath := filepath.Join(projectsDir, workspaceKey, providerSessionID+".jsonl")

	// SECURITY: Verify the resolved path is still within projectsDir (prevent path traversal)
	absPath, err := filepath.Abs(sessionPath)
	if err != nil {
		p.logger.Warn("Failed to resolve absolute path", "error", err)
		return false
	}
	absProjectsDir, err := filepath.Abs(projectsDir)
	if err != nil {
		p.logger.Warn("Failed to resolve projects dir", "error", err)
		return false
	}
	if !strings.HasPrefix(absPath, absProjectsDir+string(filepath.Separator)) {
		p.logger.Warn("Path traversal detected in session verification", "path", absPath)
		return false
	}

	// Check if the session file exists and is not empty
	info, err := os.Stat(absPath)
	if err != nil {
		p.logger.Debug("CLI session file not found, cannot resume",
			"provider_session_id", providerSessionID,
			"path", absPath,
			"error", err)
		return false
	}

	// Check if file is not empty (has actual session data)
	if info.Size() == 0 {
		p.logger.Debug("CLI session file is empty, cannot resume",
			"provider_session_id", providerSessionID,
			"path", absPath)
		return false
	}

	p.logger.Debug("CLI session file exists and can be resumed",
		"provider_session_id", providerSessionID,
		"path", absPath,
		"size", info.Size())
	return true
}

// mergeStringSlices merges two string slices with deduplication.
func mergeStringSlices(base, overlay []string) []string {
	if len(base) == 0 {
		return overlay
	}
	if len(overlay) == 0 {
		return base
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(base)+len(overlay))

	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range overlay {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// claudeCodePlugin implements ProviderPlugin for Claude Code.
type claudeCodePlugin struct{}

func (p *claudeCodePlugin) Type() ProviderType {
	return ProviderTypeClaudeCode
}

func (p *claudeCodePlugin) New(cfg ProviderConfig, logger *slog.Logger) (Provider, error) {
	return NewClaudeCodeProvider(cfg, logger)
}

func (p *claudeCodePlugin) Meta() ProviderMeta {
	return ProviderMeta{
		Type:        ProviderTypeClaudeCode,
		DisplayName: "Claude Code",
		BinaryName:  "claude",
		InstallHint: "npm install -g @anthropic-ai/claude-code",
		Features: ProviderFeatures{
			SupportsResume:      true,
			SupportsStreamJSON:  true,
			SupportsSSE:         false,
			SupportsHTTPAPI:     false,
			SupportsSessionID:   true,
			SupportsPermissions: true,
			MultiTurnReady:      true,
		},
	}
}

func init() {
	RegisterPlugin(&claudeCodePlugin{})
}
