package telemetry

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	Sampled        float64
}

type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	logger   *slog.Logger
	enabled  bool
}

var (
	globalTracer *Tracer
	once         sync.Once
)

func NewTracer(cfg Config, logger *slog.Logger) (*Tracer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.OTLPEndpoint == "" {
		logger.Info("OpenTelemetry disabled: no OTLP endpoint configured")
		return &Tracer{logger: logger, enabled: false}, nil
	}

	ctx := context.Background()

	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	))
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	sampleRatio := cfg.Sampled
	if sampleRatio <= 0 {
		sampleRatio = 1.0
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRatio)),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := provider.Tracer(cfg.ServiceName)

	return &Tracer{
		tracer:   tracer,
		provider: provider,
		logger:   logger,
		enabled:  true,
	}, nil
}

func (t *Tracer) StartSession(ctx context.Context, sessionID, namespace string) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, "session.execute",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("namespace", namespace),
		),
		trace.WithTimestamp(time.Now()),
	)
	return ctx, span
}

func (t *Tracer) EndSession(span trace.Span, err error) {
	if !t.enabled || span == nil {
		return
	}

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "true"))
	}
	span.End()
}

func (t *Tracer) StartToolUse(ctx context.Context, toolName, toolID string) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, "tool.use",
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
			attribute.String("tool.id", toolID),
		),
	)
	return ctx, span
}

func (t *Tracer) RecordDangerBlock(ctx context.Context, operation, reason string) {
	if !t.enabled {
		return
	}

	_, span := t.tracer.Start(ctx, "security.danger_block",
		trace.WithAttributes(
			attribute.String("danger.operation", operation),
			attribute.String("danger.reason", reason),
			attribute.String("security.level", "blocked"),
		),
	)
	span.End()
}

func (t *Tracer) Close(ctx context.Context) error {
	if !t.enabled || t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

func (t *Tracer) Enabled() bool {
	return t.enabled
}

func Init(cfg Config, logger *slog.Logger) error {
	var initErr error
	once.Do(func() {
		globalTracer, initErr = NewTracer(cfg, logger)
	})
	return initErr
}

func Get() *Tracer {
	if globalTracer == nil {
		return &Tracer{enabled: false}
	}
	return globalTracer
}

// ========================================
// Security Event Tracing Extensions
// ========================================

// SecurityEventType represents types of security events for tracing.
type SecurityEventType string

const (
	SecurityEventDangerBlock      SecurityEventType = "danger_block"
	SecurityEventThreatDetected  SecurityEventType = "threat_detected"
	SecurityEventWorkspaceAccess  SecurityEventType = "workspace_access"
	SecurityEventPermissionDenied SecurityEventType = "permission_denied"
	SecurityEventAuditLog         SecurityEventType = "audit_log"
	SecurityEventBypassAttempt    SecurityEventType = "bypass_attempt"
	SecurityEventLandlockEnforce  SecurityEventType = "landlock_enforce"
)

// DangerLevel represents the severity level of security events.
type DangerLevel int

const (
	DangerLevelSafe      DangerLevel = -1
	DangerLevelCritical  DangerLevel = 0
	DangerLevelHigh      DangerLevel = 1
	DangerLevelModerate  DangerLevel = 2
	DangerLevelLow       DangerLevel = 3
)

// LandlockEventType represents types of Landlock filesystem enforcement events.
type LandlockEventType string

const (
	LandlockEventAccessDenied  LandlockEventType = "access_denied"
	LandlockEventPathViolation LandlockEventType = "path_violation"
	LandlockEventRuleApplied   LandlockEventType = "rule_applied"
)

// StartSecurityEvent starts a new security event span.
func (t *Tracer) StartSecurityEvent(ctx context.Context, eventType SecurityEventType, operation string) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, "security.event",
		trace.WithAttributes(
			attribute.String("security.event_type", string(eventType)),
			attribute.String("security.operation", operation),
			attribute.Int64("security.timestamp", time.Now().UnixMilli()),
		),
	)
	return ctx, span
}

// RecordDangerDetection records a dangerous command detection event.
func (t *Tracer) RecordDangerDetection(ctx context.Context, event *DangerDetectionEvent) {
	if !t.enabled || event == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("security.event_type", string(SecurityEventDangerBlock)),
		attribute.String("danger.operation", event.Operation),
		attribute.String("danger.reason", event.Reason),
		attribute.String("danger.pattern_matched", event.PatternMatched),
		attribute.Int("danger.level", int(event.Level)),
		attribute.String("danger.category", event.Category),
		attribute.Bool("danger.bypass_allowed", event.BypassAllowed),
	}

	if event.SessionID != "" {
		attrs = append(attrs, attribute.String("session.id", event.SessionID))
	}
	if event.UserID != "" {
		attrs = append(attrs, attribute.String("user.id", event.UserID))
	}
	if event.WorkspaceID != "" {
		attrs = append(attrs, attribute.String("workspace.id", event.WorkspaceID))
	}

	_, span := t.tracer.Start(ctx, "security.danger_detected",
		trace.WithAttributes(attrs...),
	)
	span.End()
}

// DangerDetectionEvent contains details about a detected dangerous operation.
type DangerDetectionEvent struct {
	Operation      string
	Reason         string
	PatternMatched string
	Level          DangerLevel
	Category       string
	BypassAllowed  bool
	SessionID      string
	UserID         string
	WorkspaceID    string
}

// RecordThreatDetection records a threat detection event (AI Guard).
func (t *Tracer) RecordThreatDetection(ctx context.Context, event *ThreatDetectionEvent) {
	if !t.enabled || event == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("security.event_type", string(SecurityEventThreatDetected)),
		attribute.String("threat.input_type", event.InputType),
		attribute.String("threat.category", event.Category),
		attribute.Float64("threat.score", event.Score),
		attribute.Bool("threat.blocked", event.Blocked),
		attribute.String("threat.verdict", event.Verdict),
	}

	if event.SessionID != "" {
		attrs = append(attrs, attribute.String("session.id", event.SessionID))
	}
	if len(event.Details) > 0 {
		attrs = append(attrs, attribute.String("threat.details", event.Details))
	}

	_, span := t.tracer.Start(ctx, "security.threat_detected",
		trace.WithAttributes(attrs...),
	)
	span.End()
}

// ThreatDetectionEvent contains details about an AI Guard threat detection.
type ThreatDetectionEvent struct {
	InputType  string
	Category   string
	Score      float64
	Blocked    bool
	Verdict    string
	SessionID  string
	Details    string
}

// RecordWorkspaceAccess records workspace isolation access events.
func (t *Tracer) RecordWorkspaceAccess(ctx context.Context, event *WorkspaceAccessEvent) {
	if !t.enabled || event == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("security.event_type", string(SecurityEventWorkspaceAccess)),
		attribute.String("workspace.id", event.WorkspaceID),
		attribute.String("workspace.operation", event.Operation),
		attribute.Bool("workspace.allowed", event.Allowed),
	}

	if event.Path != "" {
		attrs = append(attrs, attribute.String("workspace.path", event.Path))
	}
	if event.SessionID != "" {
		attrs = append(attrs, attribute.String("session.id", event.SessionID))
	}
	if event.UserID != "" {
		attrs = append(attrs, attribute.String("user.id", event.UserID))
	}

	_, span := t.tracer.Start(ctx, "security.workspace_access",
		trace.WithAttributes(attrs...),
	)
	span.End()
}

// WorkspaceAccessEvent contains details about workspace access operations.
type WorkspaceAccessEvent struct {
	WorkspaceID string
	Operation   string
	Path        string
	Allowed     bool
	SessionID   string
	UserID      string
}

// RecordLandlockEvent records Landlock filesystem enforcement events.
func (t *Tracer) RecordLandlockEvent(ctx context.Context, event *LandlockEvent) {
	if !t.enabled || event == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("security.event_type", string(SecurityEventLandlockEnforce)),
		attribute.String("landlock.event_type", string(event.EventType)),
		attribute.String("landlock.operation", event.Operation),
		attribute.String("landlock.path", event.Path),
		attribute.Bool("landlock.allowed", event.Allowed),
	}

	if event.WorkspaceID != "" {
		attrs = append(attrs, attribute.String("workspace.id", event.WorkspaceID))
	}
	if len(event.AccessMask) > 0 {
		// Convert string slice to comma-separated string
		accessMaskStr := ""
		for i, m := range event.AccessMask {
			if i > 0 {
				accessMaskStr += ","
			}
			accessMaskStr += m
		}
		attrs = append(attrs, attribute.String("landlock.access_mask", accessMaskStr))
	}

	_, span := t.tracer.Start(ctx, "security.landlock_enforce",
		trace.WithAttributes(attrs...),
	)
	span.End()
}

// LandlockEvent contains details about a Landlock enforcement event.
type LandlockEvent struct {
	EventType   LandlockEventType
	Operation   string
	Path        string
	Allowed     bool
	WorkspaceID string
	AccessMask  []string
}

// RecordPermissionDenied records permission denial events.
func (t *Tracer) RecordPermissionDenied(ctx context.Context, event *PermissionDeniedEvent) {
	if !t.enabled || event == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("security.event_type", string(SecurityEventPermissionDenied)),
		attribute.String("permission.resource", event.Resource),
		attribute.String("permission.operation", event.Operation),
		attribute.String("permission.reason", event.Reason),
	}

	if event.SessionID != "" {
		attrs = append(attrs, attribute.String("session.id", event.SessionID))
	}
	if event.UserID != "" {
		attrs = append(attrs, attribute.String("user.id", event.UserID))
	}

	_, span := t.tracer.Start(ctx, "security.permission_denied",
		trace.WithAttributes(attrs...),
	)
	span.End()
}

// PermissionDeniedEvent contains details about a permission denial.
type PermissionDeniedEvent struct {
	Resource  string
	Operation string
	Reason    string
	SessionID string
	UserID    string
}

// RecordBypassAttempt records security bypass attempts.
func (t *Tracer) RecordBypassAttempt(ctx context.Context, event *BypassAttemptEvent) {
	if !t.enabled || event == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("security.event_type", string(SecurityEventBypassAttempt)),
		attribute.String("bypass.target_rule", event.TargetRule),
		attribute.Bool("bypass.success", event.Success),
		attribute.String("bypass.attempted_by", event.AttemptedBy),
	}

	if event.SessionID != "" {
		attrs = append(attrs, attribute.String("session.id", event.SessionID))
	}

	_, span := t.tracer.Start(ctx, "security.bypass_attempt",
		trace.WithAttributes(attrs...),
	)
	span.End()
}

// BypassAttemptEvent contains details about a security bypass attempt.
type BypassAttemptEvent struct {
	TargetRule  string
	Success     bool
	AttemptedBy string
	SessionID   string
}
