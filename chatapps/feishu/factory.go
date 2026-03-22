package feishu

import (
	"context"
	"log/slog"
	"os"

	"github.com/hrygo/hotplex/chatapps/base"
)

// AdapterFactory implements base.AdapterFactory for Feishu.
type AdapterFactory struct{}

var _ base.AdapterFactory = (*AdapterFactory)(nil)

// Platform implements AdapterFactory.
func (f *AdapterFactory) Platform() string { return "feishu" }

// RequiredEnvVars implements AdapterFactory.
func (f *AdapterFactory) RequiredEnvVars() []string {
	return []string{"HOTPLEX_FEISHU_APP_ID"}
}

// New implements AdapterFactory by creating a Feishu adapter from the given config.
func (f *AdapterFactory) New(pc *base.PlatformConfig) any {
	appID := os.Getenv("HOTPLEX_FEISHU_APP_ID")
	if appID == "" {
		return nil
	}

	config := &Config{
		AppID:             appID,
		AppSecret:         os.Getenv("HOTPLEX_FEISHU_APP_SECRET"),
		VerificationToken: os.Getenv("HOTPLEX_FEISHU_VERIFICATION_TOKEN"),
		EncryptKey:        os.Getenv("HOTPLEX_FEISHU_ENCRYPT_KEY"),
		ServerAddr:        os.Getenv("HOTPLEX_FEISHU_SERVER_ADDR"),
	}

	if pc != nil {
		config.SystemPrompt = pc.SystemPrompt
	}

	var opts []base.AdapterOption
	if pc != nil {
		opts = append(opts, base.WithSessionTimeout(pc.Session.Timeout))
		opts = append(opts, base.WithCleanupInterval(pc.Session.CleanupInterval))
	}
	opts = append(opts, base.WithoutServer())

	adapter, _ := NewAdapter(config, slog.Default(), opts...)
	return adapter
}

// PostSetup implements AdapterFactory — Feishu has no post-setup steps.
func (f *AdapterFactory) PostSetup(_ context.Context, _, _ any) {
	// Feishu requires no post-setup steps
}

func init() {
	base.GlobalAdapterRegistry().Register(&AdapterFactory{})
}
