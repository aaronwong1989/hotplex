package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// openCodeServerMeta is the shared metadata for OpenCode Server provider.
var openCodeServerMeta = ProviderMeta{
	Type:        ProviderTypeOpenCodeServer,
	DisplayName: "OpenCode (Server)",
	BinaryName:  "opencode",
	InstallHint: "brew install anomalyco/tap/opencode",
	Features: ProviderFeatures{
		SupportsResume:     true,
		SupportsStreamJSON: true,
		SupportsSSE:        true,
		SupportsHTTPAPI:    true,
		SupportsSessionID:  true,
		MultiTurnReady:     true,
	},
}

// OpenCodeServerProvider implements the Provider interface for OpenCode HTTP API mode.
// Unlike the CLI-based OpenCodeProvider, this uses HTTP transport to communicate
// with a running opencode serve instance.
type OpenCodeServerProvider struct {
	ProviderBase
	transport     *HTTPTransport
	opts          ProviderConfig
	promptBuilder *PromptBuilder
}

// NewOpenCodeServerProvider creates a new HTTP-based OpenCode provider.
func NewOpenCodeServerProvider(cfg ProviderConfig, logger *slog.Logger) (*OpenCodeServerProvider, error) {
	ocCfg := cfg.OpenCode
	if ocCfg == nil {
		ocCfg = &OpenCodeConfig{}
	}

	// Debug: Log password status (first 10 chars only for security)
	if logger != nil {
		if ocCfg.Password != "" {
			logger.Debug("OpenCode password loaded", "password_preview", ocCfg.Password[:min(10, len(ocCfg.Password))])
		} else {
			logger.Warn("OpenCode password is EMPTY - will fail auth")
		}
	}

	url := ocCfg.ServerURL
	if url == "" {
		port := 4096
		if ocCfg.Port > 0 {
			port = ocCfg.Port
		}
		url = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	return &OpenCodeServerProvider{
		ProviderBase: ProviderBase{
			meta:   openCodeServerMeta,
			logger: logger.With("provider", "opencode-server"),
		},
		transport: NewHTTPTransport(HTTPTransportConfig{
			Endpoint: url,
			Password: ocCfg.Password,
			Logger:   logger.With("component", "oc_transport"),
			Timeout:  30 * time.Second,
			WorkDir:  ocCfg.WorkDir, // Pass working directory from config
		}),
		opts:          cfg,
		promptBuilder: NewPromptBuilder(false),
	}, nil
}

// Compile-time interface check
var _ Provider = (*OpenCodeServerProvider)(nil)

// ValidateBinary checks if the OpenCode server is reachable.
func (p *OpenCodeServerProvider) ValidateBinary() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.transport.Health(ctx); err != nil {
		return "", fmt.Errorf("opencode server unreachable at %s: %w", p.transport.baseURL, err)
	}
	return p.transport.baseURL, nil
}

// BuildCLIArgs returns nil for server mode (no CLI subprocess).
func (p *OpenCodeServerProvider) BuildCLIArgs(_ string, _ *ProviderSessionOptions) []string {
	return nil
}

// BuildInputMessage creates an input message for the OpenCode API.
func (p *OpenCodeServerProvider) BuildInputMessage(prompt, taskInstructions string) (map[string]any, error) {
	msg := map[string]any{
		"parts": []map[string]any{{
			"type": OCPartText,
			"text": p.promptBuilder.Build(prompt, taskInstructions),
		}},
	}

	// Debug log to verify message structure
	if p.logger != nil {
		b, err := json.Marshal(msg)
		if err != nil {
			p.logger.Error("Failed to marshal message for debug log", "error", err)
		} else {
			p.logger.Debug("BuildInputMessage created",
				"prompt_len", len(prompt),
				"msg_json", string(b))
		}
	}

	return msg, nil
}

// ParseEvent parses an SSE event line into provider events.
func (p *OpenCodeServerProvider) ParseEvent(line string) ([]*ProviderEvent, error) {
	var sseEvt OCSSEEvent
	if err := json.Unmarshal([]byte(line), &sseEvt); err != nil {
		return nil, fmt.Errorf("parse SSE event: %w", err)
	}
	return p.mapEvent(sseEvt)
}

// mapEvent converts an OpenCode SSE event to provider events.
func (p *OpenCodeServerProvider) mapEvent(evt OCSSEEvent) ([]*ProviderEvent, error) {
	switch evt.Type {
	case OCEventMessagePartUpdated:
		var props OCPartUpdateProps
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse part update: %w", err)
		}
		return p.mapPart(props.Part, props.Delta)

	case OCEventMessageUpdated:
		var props struct {
			Info OCAssistantMessage `json:"info"`
		}
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse message updated: %w", err)
		}
		if props.Info.Finish != "" {
			contextWindow := estimateContextWindow(props.Info.ModelID)
			return []*ProviderEvent{{
				Type:    EventTypeResult,
				RawType: evt.Type,
				Metadata: &ProviderEventMeta{
					InputTokens:   props.Info.Tokens.Input,
					OutputTokens:  props.Info.Tokens.Output,
					TotalCostUSD:  props.Info.Cost,
					Model:         props.Info.ModelID,
					ModelName:     props.Info.ModelID,
					ContextWindow: contextWindow,
				},
			}}, nil
		}
		return nil, nil

	case OCEventSessionIdle:
		return []*ProviderEvent{{Type: EventTypeResult, RawType: evt.Type}}, nil

	case OCEventSessionStatus:
		var props OCSessionStatusProps
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse session status: %w", err)
		}
		switch props.Status.Type {
		case "busy":
			return []*ProviderEvent{{Type: EventTypeSystem, Status: "running"}}, nil
		case "idle":
			return []*ProviderEvent{{Type: EventTypeResult, RawType: evt.Type}}, nil
		case "retry":
			msg := "Retrying"
			if props.Status.Attempt > 0 {
				msg = fmt.Sprintf("Retrying (attempt %d)", props.Status.Attempt)
			}
			return []*ProviderEvent{{
				Type:    EventTypeSystem,
				Status:  "retrying",
				Content: msg,
			}}, nil
		default:
			p.logger.Debug("Unknown session status type", "status_type", props.Status.Type)
			return nil, nil
		}

	case OCEventSessionError:
		var props OCSessionErrorProps
		if err := json.Unmarshal(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse session error: %w", err)
		}
		msg := "unknown error"
		if props.Error.Name != "" {
			msg = props.Error.Name
		}
		errData, _ := json.Marshal(props.Error.Data)
		return []*ProviderEvent{{
			Type:    EventTypeError,
			Error:   msg,
			IsError: true,
			Content: string(errData),
		}}, nil

	case OCEventPermissionUpdated:
		var perm OCPermissionProps
		if err := json.Unmarshal(evt.Properties, &perm); err != nil {
			return nil, fmt.Errorf("parse permission: %w", err)
		}
		return []*ProviderEvent{{
			Type:     EventTypePermissionRequest,
			ToolName: perm.Title,
			ToolID:   perm.ID,
			Content:  fmt.Sprintf("[Permission] %s: %s", perm.Type, perm.Title),
		}}, nil

	default:
		return nil, nil
	}
}

// mapPart converts an OpenCode part to provider events.
func (p *OpenCodeServerProvider) mapPart(part OCPart, delta string) ([]*ProviderEvent, error) {
	switch part.Type {
	case OCPartText:
		c := delta
		if c == "" {
			c = part.Text
		}
		return []*ProviderEvent{{Type: EventTypeAnswer, Content: c}}, nil

	case OCPartReasoning:
		c := delta
		if c == "" {
			c = part.Text
		}
		meta := &ProviderEventMeta{}
		if part.Tokens != nil {
			meta.OutputTokens = part.Tokens.Output
		}
		return []*ProviderEvent{{Type: EventTypeThinking, Content: c, Metadata: meta}}, nil

	case OCPartTool:
		// Support both v1.3.2 nested state and legacy flat structure for backward compatibility
		status := part.GetStatus()
		toolName := part.GetToolName()

		switch status {
		case "pending", "running":
			return []*ProviderEvent{{
				Type:      EventTypeToolUse,
				ToolName:  toolName,
				ToolID:    part.ID,
				ToolInput: part.Input,
				Status:    "running",
			}}, nil
		case "completed":
			return []*ProviderEvent{{
				Type:     EventTypeToolResult,
				ToolName: toolName,
				ToolID:   part.ID,
				Content:  part.Output,
				Status:   "success",
			}}, nil
		case "error":
			return []*ProviderEvent{{
				Type:     EventTypeToolResult,
				ToolName: toolName,
				ToolID:   part.ID,
				Error:    part.Error,
				IsError:  true,
				Status:   "error",
			}}, nil
		}

	case OCPartStepStart:
		return []*ProviderEvent{{Type: EventTypeStepStart}}, nil

	case OCPartStepFinish:
		meta := &ProviderEventMeta{}

		if part.Tokens != nil {
			meta.InputTokens = part.Tokens.Input
			meta.OutputTokens = part.Tokens.Output
			if part.Tokens.Cache != nil {
				meta.CacheReadTokens = part.Tokens.Cache.Read
				meta.CacheWriteTokens = part.Tokens.Cache.Write
			}
		}

		meta.CurrentStep = int32(part.StepNumber)
		meta.TotalSteps = int32(part.TotalSteps)

		// Map finish reason to user-friendly message
		// Uses map lookup for extensibility (OCP-compliant)
		content := finishReasonMessages[FinishReason(part.Reason)]

		return []*ProviderEvent{{
			Type:     EventTypeStepFinish,
			Content:  content,
			Status:   part.Reason,
			Metadata: meta,
		}}, nil
	}

	return nil, nil
}

// estimateContextWindow returns a heuristic context window size based on model ID.
// Uses strings.Contains matching instead of a hardcoded registry to avoid staleness.
// Falls back to 200K for unknown models.
func estimateContextWindow(modelID string) int32 {
	m := strings.ToLower(modelID)

	// Claude 4 / Opus 4.5+ — 200K
	if strings.Contains(m, "opus") {
		return 200_000
	}
	// Claude 3.5 Haiku — 200K
	if strings.Contains(m, "haiku") {
		return 200_000
	}
	// Claude Sonnet family — 200K (3.5, 4)
	if strings.Contains(m, "sonnet") {
		return 200_000
	}
	// Gemini models — 1M+
	if strings.Contains(m, "gemini") {
		return 1_000_000
	}
	// GPT-4 family — 128K
	if strings.Contains(m, "gpt-4") {
		return 128_000
	}
	// Default conservative fallback
	return 200_000
}

// DetectTurnEnd detects if an event indicates the end of a turn.
func (p *OpenCodeServerProvider) DetectTurnEnd(e *ProviderEvent) bool {
	return e != nil && (e.Type == EventTypeResult || e.Type == EventTypeError)
}

// CleanupSession removes session resources.
func (p *OpenCodeServerProvider) CleanupSession(_ string, _ string) error {
	// HTTP sessions are managed by the server, no local cleanup needed
	return nil
}

// Events returns the SSE event channel (deprecated: use Subscribe for fan-out).
func (p *OpenCodeServerProvider) Events() <-chan string {
	return p.transport.Subscribe()
}

// Connect establishes connection to the OpenCode server.
func (p *OpenCodeServerProvider) Connect(ctx context.Context, cfg TransportConfig) error {
	return p.transport.Connect(ctx, cfg)
}

// Close closes the provider and releases resources.
func (p *OpenCodeServerProvider) Close() error {
	return p.transport.Close()
}

// GetHTTPTransport returns the underlying HTTP transport for HTTPSessionStarter.
// This method is used by the engine to create HTTP-based sessions.
func (p *OpenCodeServerProvider) GetHTTPTransport() Transport {
	return p.transport
}

// ─── Plugin Registration ───

func init() {
	RegisterPlugin(&openCodeServerPlugin{})
}

type openCodeServerPlugin struct{}

func (p *openCodeServerPlugin) Type() ProviderType {
	return ProviderTypeOpenCodeServer
}

func (p *openCodeServerPlugin) New(cfg ProviderConfig, logger *slog.Logger) (Provider, error) {
	return NewOpenCodeServerProvider(cfg, logger)
}

func (p *openCodeServerPlugin) Meta() ProviderMeta {
	return openCodeServerMeta
}
