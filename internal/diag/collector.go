package diag

import (
	"context"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/hrygo/hotplex"
)

// Collector collects diagnostic context from various sources.
type Collector struct {
	config     *Config
	redactor   *Redactor
	version    string
	startTime  time.Time
}

// NewCollector creates a new diagnostic context collector.
func NewCollector(config *Config) *Collector {
	if config == nil {
		config = DefaultConfig()
	}
	return &Collector{
		config:     config,
		redactor:   NewRedactor(RedactStandard),
		version:    hotplex.Version,
		startTime:  time.Now(),
	}
}

// Collect gathers all diagnostic context for a given trigger.
func (c *Collector) Collect(ctx context.Context, trigger Trigger) (*DiagContext, error) {
	now := time.Now()

	diagCtx := &DiagContext{
		OriginalSessionID: trigger.SessionID(),
		Platform:         trigger.Platform(),
		UserID:           trigger.UserID(),
		ChannelID:        trigger.ChannelID(),
		ThreadID:         trigger.ThreadID(),
		Trigger:          trigger.Type(),
		Error:            trigger.Error(),
		Timestamp:        now,
	}

	// Collect environment info
	diagCtx.Environment = c.collectEnvInfo()

	// Conversation - placeholder for future implementation
	diagCtx.Conversation = &ConversationData{Processed: "", MessageCount: 0}

	// Logs - placeholder for future implementation
	diagCtx.Logs = []byte("")

	return diagCtx, nil
}

// collectEnvInfo gathers environment information.
func (c *Collector) collectEnvInfo() *EnvInfo {
	buildInfo, _ := debug.ReadBuildInfo()

	var goVersion string
	if buildInfo != nil {
		goVersion = buildInfo.GoVersion
	}

	return &EnvInfo{
		HotPlexVersion: c.version,
		GoVersion:      goVersion,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Uptime:         time.Since(c.startTime),
	}
}
