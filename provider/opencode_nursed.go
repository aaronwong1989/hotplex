package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// openCodeNursedMeta is the metadata for OpenCodeNursedProvider.
var openCodeNursedMeta = ProviderMeta{
	Type:        ProviderTypeOpenCodeServer,
	DisplayName: "OpenCode (Nursed)",
	BinaryName:  "opencode",
	InstallHint: "brew install anomalyco/tap/opencode",
	Features: ProviderFeatures{
		SupportsResume:     true,
		SupportsStreamJSON: true,
		SupportsSSE:       true,
		SupportsHTTPAPI:   true,
		SupportsSessionID: true,
		MultiTurnReady:    true,
	},
}

// OpenCodeNursedProvider implements Provider interface with full process lifecycle management.
// It wraps OpenCodeServerProvider with ProcessGuardian for automatic process restart,
// HTTPSessionManager for deterministic session mapping, and EventRelay for SSE routing.
type OpenCodeNursedProvider struct {
	ProviderBase
	config   *OpenCodeConfig
	guardian *ProcessGuardian
	sessions *HTTPSessionManager
	relay    *EventRelay
	transport *HTTPTransport
	opts     ProviderConfig
	promptBuilder *PromptBuilder

	logger *slog.Logger
}

// Compile-time interface check
var _ Provider = (*OpenCodeNursedProvider)(nil)

// NewOpenCodeNursedProvider creates a new OpenCodeNursedProvider.
func NewOpenCodeNursedProvider(cfg ProviderConfig, logger *slog.Logger) (*OpenCodeNursedProvider, error) {
	if logger == nil {
		logger = slog.Default()
	}

	ocCfg := cfg.OpenCode
	if ocCfg == nil {
		ocCfg = &OpenCodeConfig{}
	}

	// Build server URL
	url := ocCfg.ServerURL
	if url == "" {
		port := 4096
		if ocCfg.Port > 0 {
			port = ocCfg.Port
		}
		url = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	// Create HTTP transport
	transport := NewHTTPTransport(HTTPTransportConfig{
		Endpoint: url,
		Password: ocCfg.Password,
		Logger:   logger.With("component", "oc_transport"),
		Timeout:  30 * time.Second,
		WorkDir:  ocCfg.WorkDir,
	})

	// Create process guardian if binary path is specified
	var guardian *ProcessGuardian
	if ocCfg.BinaryPath != "" || ocCfg.ServeArgs != nil {
		binary := ocCfg.BinaryPath
		if binary == "" {
			// Try to find opencode in PATH
			path, err := exec.LookPath("opencode")
			if err != nil {
				return nil, fmt.Errorf("opencode binary not found in PATH: %w", err)
			}
			binary = path
		}

		args := ocCfg.ServeArgs
		if args == nil {
			args = []string{"serve"}
			// Add port if specified
			if ocCfg.Port > 0 {
				args = append(args, "--port", fmt.Sprintf("%d", ocCfg.Port))
			}
		}

		// Build guardian config from OpenCodeConfig fields
		guardianCfg := DefaultGuardianConfig()
		if ocCfg.HealthCheckInterval != "" {
			if d, err := time.ParseDuration(ocCfg.HealthCheckInterval); err == nil {
				guardianCfg.HealthCheckInterval = d
			}
		}
		if ocCfg.StartupTimeout != "" {
			if d, err := time.ParseDuration(ocCfg.StartupTimeout); err == nil {
				guardianCfg.StartupTimeout = d
			}
		}
		if len(ocCfg.RestartBackoff) > 0 {
			guardianCfg.Backoff = nil
			for _, s := range ocCfg.RestartBackoff {
				if d, err := time.ParseDuration(s); err == nil {
					guardianCfg.Backoff = append(guardianCfg.Backoff, d)
				}
			}
			if len(guardianCfg.Backoff) == 0 {
				guardianCfg.Backoff = DefaultGuardianConfig().Backoff
			}
		}
		if ocCfg.MaxFailBurst > 0 {
			guardianCfg.MaxFailBurst = ocCfg.MaxFailBurst
		}

		guardian = NewProcessGuardianWithConfig(binary, args, ocCfg.Password, ocCfg.WorkDir, transport, guardianCfg, logger)

		// Set callbacks
		guardian.SetStateChangeCallback(func(state GuardianState) {
			logger.Info("Guardian state changed", "state", state.String())
		})
		guardian.SetFailureCallback(func(entry FailureEntry) {
			logger.Warn("Process failure recorded",
				"attempt", entry.Attempt,
				"reason", entry.Reason)
		})
	}

	// Create session manager
	sessionManager := NewHTTPSessionManager(transport, logger)

	// Create event relay
	eventRelay := NewEventRelay(transport, logger)

	p := &OpenCodeNursedProvider{
		ProviderBase: ProviderBase{
			meta:   openCodeNursedMeta,
			logger: logger.With("provider", "opencode-nursed"),
		},
		config:   ocCfg,
		guardian: guardian,
		sessions: sessionManager,
		relay:    eventRelay,
		transport: transport,
		opts:     cfg,
		promptBuilder: NewPromptBuilder(false),
		logger:   logger,
	}

	// Set permission callback on relay
	eventRelay.SetPermissionCallback(func(evt PermissionEvent) {
		logger.Info("Permission event received",
			"session_id", evt.SessionID,
			"permission_id", evt.PermissionID,
			"type", evt.Type,
			"title", evt.Title)
	})

	return p, nil
}

// Metadata returns the provider metadata.
func (p *OpenCodeNursedProvider) Metadata() ProviderMeta {
	return p.meta
}

// Name returns the provider name.
func (p *OpenCodeNursedProvider) Name() string {
	return "opencode-nursed"
}

// ValidateBinary validates that the opencode binary exists and is runnable.
// If ProcessGuardian is configured, this also validates the binary can be executed.
func (p *OpenCodeNursedProvider) ValidateBinary() (string, error) {
	// If we have a guardian, the binary is managed by us
	if p.guardian != nil {
		binary := p.config.BinaryPath
		if binary == "" {
			path, err := exec.LookPath("opencode")
			if err != nil {
				return "", fmt.Errorf("opencode binary not found: %w", err)
			}
			return path, nil
		}
		return binary, nil
	}

	// Otherwise, check if server is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.transport.Health(ctx); err != nil {
		return "", fmt.Errorf("opencode server unreachable at %s: %w", p.transport.baseURL, err)
	}
	return p.transport.baseURL, nil
}

// BuildCLIArgs returns nil for nursed mode (process is managed by guardian).
func (p *OpenCodeNursedProvider) BuildCLIArgs(_ string, _ *ProviderSessionOptions) []string {
	return nil
}

// BuildInputMessage creates an input message for the OpenCode API.
func (p *OpenCodeNursedProvider) BuildInputMessage(prompt, taskInstructions, baseSystemPrompt string) (map[string]any, error) {
	msg := map[string]any{
		"parts": []map[string]any{{
			"type": OCPartText,
			"text": p.promptBuilder.Build(prompt, taskInstructions),
		}},
	}
	return msg, nil
}

// ParseEvent parses an SSE event line into provider events.
func (p *OpenCodeNursedProvider) ParseEvent(line string) ([]*ProviderEvent, error) {
	return p.relay.MapEvent(line)
}

// mapEvent converts an OpenCode SSE event to provider events.
func (p *OpenCodeNursedProvider) mapEvent(evt OCSSEEvent) ([]*ProviderEvent, error) {
	switch evt.Type {
	case OCEventMessagePartUpdated:
		var props OCPartUpdateProps
		if err := unmarshalJSON(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse part update: %w", err)
		}
		return p.mapPart(props.Part, props.Delta)

	case OCEventMessageUpdated:
		var props struct {
			Info OCAssistantMessage `json:"info"`
		}
		if err := unmarshalJSON(evt.Properties, &props); err != nil {
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
		if err := unmarshalJSON(evt.Properties, &props); err != nil {
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
			return nil, nil
		}

	case OCEventSessionError:
		var props OCSessionErrorProps
		if err := unmarshalJSON(evt.Properties, &props); err != nil {
			return nil, fmt.Errorf("parse session error: %w", err)
		}
		msg := "unknown error"
		if props.Error.Name != "" {
			msg = props.Error.Name
		}
		errData, _ := marshalJSON(props.Error.Data)
		return []*ProviderEvent{{
			Type:    EventTypeError,
			Error:   msg,
			IsError: true,
			Content: string(errData),
		}}, nil

	case OCEventPermissionUpdated:
		var perm OCPermissionProps
		if err := unmarshalJSON(evt.Properties, &perm); err != nil {
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
func (p *OpenCodeNursedProvider) mapPart(part OCPart, delta string) ([]*ProviderEvent, error) {
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
		status := part.GetStatus()
		toolName := part.GetToolName()

		switch status {
		case ToolStatusPending, ToolStatusRunning:
			return []*ProviderEvent{{
				Type:      EventTypeToolUse,
				ToolName:  toolName,
				ToolID:    part.ID,
				ToolInput: part.Input,
				Status:    ToolStatusRunning,
			}}, nil
		case ToolStatusCompleted:
			return []*ProviderEvent{{
				Type:     EventTypeToolResult,
				ToolName: toolName,
				ToolID:   part.ID,
				Content:  part.Output,
				Status:   "success",
			}}, nil
		case ToolStatusError:
			return []*ProviderEvent{{
				Type:     EventTypeToolResult,
				ToolName: toolName,
				ToolID:   part.ID,
				Error:    part.Error,
				IsError:  true,
				Status:   ToolStatusError,
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

// DetectTurnEnd detects if an event indicates the end of a turn.
func (p *OpenCodeNursedProvider) DetectTurnEnd(e *ProviderEvent) bool {
	return e != nil && (e.Type == EventTypeResult || e.Type == EventTypeError)
}

// CleanupSession removes session resources.
func (p *OpenCodeNursedProvider) CleanupSession(providerSessionID, workDir string) error {
	// Get server session ID from mapping
	info, ok := p.sessions.GetSession(providerSessionID)
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.sessions.DeleteSession(ctx, info.ServerSessionID)
}

// StartServer starts the opencode serve process (if managed) and establishes connection.
func (p *OpenCodeNursedProvider) StartServer(ctx context.Context) error {
	// Start guardian if we manage the process
	if p.guardian != nil {
		if err := p.guardian.Start(ctx); err != nil {
			return fmt.Errorf("start guardian: %w", err)
		}
	}

	// Connect transport
	if err := p.transport.Connect(ctx, TransportConfig{}); err != nil {
		return fmt.Errorf("connect transport: %w", err)
	}

	// Start event relay
	go p.relay.Start(context.Background())

	// Recover session mappings
	if err := p.sessions.RecoverMappings(ctx); err != nil {
		p.logger.Warn("Failed to recover session mappings", "error", err)
	}

	return nil
}

// StopServer stops the opencode serve process.
func (p *OpenCodeNursedProvider) StopServer(ctx context.Context) error {
	if p.guardian != nil {
		return p.guardian.Stop(ctx)
	}
	return nil
}

// CreateSession creates a new session or returns existing one.
func (p *OpenCodeNursedProvider) CreateSession(ctx context.Context, namespace, sessionID, workDir string) (string, error) {
	// Resolve workdir from template
	workDir = ResolveWorkDir(p.config.WorkDirTemplate, namespace, sessionID)

	return p.sessions.CreateSession(ctx, namespace, sessionID, workDir)
}

// SubscribeEvents subscribes to events for a server session.
func (p *OpenCodeNursedProvider) SubscribeEvents(serverSessionID string) (<-chan *ProviderEvent, func()) {
	return p.relay.Subscribe(serverSessionID)
}

// SetPermissionCallback sets the callback for permission events.
func (p *OpenCodeNursedProvider) SetPermissionCallback(fn func(PermissionEvent)) {
	p.relay.SetPermissionCallback(fn)
}

// GetHTTPTransport returns the underlying HTTP transport.
func (p *OpenCodeNursedProvider) GetHTTPTransport() Transport {
	return p.transport
}

// GetGuardian returns the process guardian (may be nil).
func (p *OpenCodeNursedProvider) GetGuardian() *ProcessGuardian {
	return p.guardian
}

// Close closes the provider and releases resources.
func (p *OpenCodeNursedProvider) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if p.guardian != nil {
		_ = p.guardian.Stop(ctx)
	}

	return p.transport.Close()
}

// VerifySession checks if a session can be resumed.
func (p *OpenCodeNursedProvider) VerifySession(providerSessionID string, workDir string) bool {
	_, ok := p.sessions.GetSession(providerSessionID)
	return ok
}

// ─── Plugin Registration ───

func init() {
	RegisterPlugin(&openCodeNursedPlugin{})
}

type openCodeNursedPlugin struct{}

func (p *openCodeNursedPlugin) Type() ProviderType {
	return ProviderTypeOpenCodeServer // Same type as OpenCodeServerProvider
}

func (p *openCodeNursedPlugin) New(cfg ProviderConfig, logger *slog.Logger) (Provider, error) {
	return NewOpenCodeNursedProvider(cfg, logger)
}

func (p *openCodeNursedPlugin) Meta() ProviderMeta {
	return openCodeNursedMeta
}

// Helper functions to avoid import conflicts
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
