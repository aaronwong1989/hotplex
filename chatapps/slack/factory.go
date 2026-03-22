package slack

import (
	"context"
	"log/slog"
	"os"

	"github.com/hrygo/hotplex/chatapps/base"
	"github.com/hrygo/hotplex/chatapps/slack/apphome"
)

// AdapterFactory implements base.AdapterFactory for Slack.
type AdapterFactory struct{}

var _ base.AdapterFactory = (*AdapterFactory)(nil)

// Platform implements AdapterFactory.
func (f *AdapterFactory) Platform() string { return "slack" }

// RequiredEnvVars implements AdapterFactory.
func (f *AdapterFactory) RequiredEnvVars() []string {
	return []string{"HOTPLEX_SLACK_BOT_TOKEN"}
}

// New implements AdapterFactory by creating a Slack adapter from the given config.
func (f *AdapterFactory) New(pc *base.PlatformConfig) any {
	token := os.Getenv("HOTPLEX_SLACK_BOT_TOKEN")
	if token == "" {
		return nil
	}

	// Mode: YAML config takes precedence; env var is override.
	// Empty mode defaults to Socket Mode (handled by Validate/IsSocketMode).
	mode := os.Getenv("HOTPLEX_SLACK_MODE")
	config := &Config{
		BotToken:      token,
		AppToken:      os.Getenv("HOTPLEX_SLACK_APP_TOKEN"),
		SigningSecret: os.Getenv("HOTPLEX_SLACK_SIGNING_SECRET"),
		ServerAddr:    os.Getenv("HOTPLEX_SLACK_SERVER_ADDR"),
	}

	if pc != nil {
		// YAML config is the SSOT for platform settings
		if pc.Mode != "" {
			config.Mode = pc.Mode
		} else if mode != "" {
			config.Mode = mode
		}
		config.SystemPrompt = pc.SystemPrompt
		config.BotUserID = pc.Security.Permission.BotUserID
		config.VerifySignature = pc.Security.VerifySignature
		config.DMPolicy = pc.Security.Permission.DMPolicy
		config.GroupPolicy = pc.Security.Permission.GroupPolicy
		config.AllowedUsers = pc.Security.Permission.AllowedUsers
		config.BlockedUsers = pc.Security.Permission.BlockedUsers
		config.SlashCommandRateLimit = pc.Security.Permission.SlashCommandRateLimit

		if pc.Security.Owner != nil {
			config.Owner = &OwnerConfig{
				Primary: pc.Security.Owner.Primary,
				Trusted: pc.Security.Owner.Trusted,
				Policy:  OwnerPolicy(pc.Security.Owner.Policy),
			}
		}

		if pc.Security.Permission.ThreadOwnership != nil {
			config.ThreadOwnership = &ThreadOwnershipConfig{
				Enabled: pc.Security.Permission.ThreadOwnership.Enabled,
				TTL:     pc.Security.Permission.ThreadOwnership.TTL,
				Persist: pc.Security.Permission.ThreadOwnership.Persist,
			}
		}

		config.Features = FeaturesConfig{
			Chunking: ChunkingConfig{
				Enabled:  pc.Features.Chunking.Enabled,
				MaxChars: pc.Features.Chunking.MaxChars,
			},
			Threading: ThreadingConfig{
				Enabled: pc.Features.Threading.Enabled,
			},
			RateLimit: RateLimitConfig{
				Enabled:     pc.Features.RateLimit.Enabled,
				MaxAttempts: pc.Features.RateLimit.MaxAttempts,
				BaseDelayMs: pc.Features.RateLimit.BaseDelayMs,
				MaxDelayMs:  pc.Features.RateLimit.MaxDelayMs,
			},
			Markdown: MarkdownConfig{
				Enabled: pc.Features.Markdown.Enabled,
			},
		}

		if pc.MessageStore.Enabled != nil {
			config.Storage = &StorageConfig{
				Enabled:       pc.MessageStore.Enabled,
				Type:          pc.MessageStore.Type,
				SQLitePath:    pc.MessageStore.SQLite.Path,
				PostgreSQLURL: pc.MessageStore.Postgres.DSN,
				StreamEnabled: pc.MessageStore.Streaming.Enabled,
				StreamTimeout: pc.MessageStore.Streaming.Timeout,
			}
		}

		config.SetBroadcastResponse(pc.Security.Permission.BroadcastResponse)

		if config.AppToken == "" && pc.Options != nil {
			if appToken, ok := pc.Options["app_token"].(string); ok {
				config.AppToken = os.ExpandEnv(appToken)
			}
		}

	}

	var opts []base.AdapterOption
	if pc != nil {
		opts = append(opts, base.WithSessionTimeout(pc.Session.Timeout))
		opts = append(opts, base.WithCleanupInterval(pc.Session.CleanupInterval))
	}
	opts = append(opts, base.WithoutServer())
	return NewAdapter(config, slog.Default(), opts...)
}

// PostSetup implements AdapterFactory — sets up AppHome capability center.
func (f *AdapterFactory) PostSetup(_ context.Context, adapter, setupCtx any) {
	ctx, ok := setupCtx.(*base.SetupContext)
	if !ok || ctx == nil || ctx.Logger == nil {
		return
	}
	logger := ctx.Logger

	slackAdapter, ok := adapter.(*Adapter)
	if !ok {
		logger.Debug("PostSetup: adapter is not a Slack Adapter")
		return
	}

	client := slackAdapter.GetSlackClient()
	if client == nil {
		logger.Debug("PostSetup: Slack client not available yet")
		return
	}

	appHomeConfig := apphome.Config{
		Enabled:          true,
		CapabilitiesPath: os.Getenv("HOTPLEX_SLACK_CAPABILITIES_PATH"),
	}

	handler, _, _ := apphome.Setup(client, nil, appHomeConfig, logger)
	if handler != nil {
		slackAdapter.SetAppHomeHandler(handler)
		logger.Info("AppHome capability center initialized", "platform", "slack")
	}
}

func init() {
	base.GlobalAdapterRegistry().Register(&AdapterFactory{})
}
