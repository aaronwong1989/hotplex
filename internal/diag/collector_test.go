package diag

import (
	"context"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		collector := NewCollector(nil)
		if collector == nil {
			t.Fatal("Expected non-nil collector")
		}
		if collector.config == nil {
			t.Fatal("Expected non-nil config")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &Config{
			Enabled:               false,
			LogSizeLimit:          1024,
			ConversationSizeLimit: 512,
		}
		collector := NewCollector(cfg)
		if collector.config.Enabled {
			t.Error("Expected Enabled to be false")
		}
		if collector.config.LogSizeLimit != 1024 {
			t.Errorf("Expected LogSizeLimit 1024, got %d", collector.config.LogSizeLimit)
		}
	})
}

func TestCollectorCollect(t *testing.T) {
	collector := NewCollector(nil)

	errInfo := &ErrorInfo{
		Type:      ErrorTypeExit,
		Message:   "test error",
		ExitCode:  1,
		Timestamp: time.Now(),
	}

	trigger := NewBaseTrigger(TriggerAuto, "session-123", errInfo).
		SetPlatform("slack").
		SetUserID("user-456").
		SetChannelID("channel-789").
		SetThreadID("thread-012")

	ctx := context.Background()
	diagCtx, err := collector.Collect(ctx, trigger)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if diagCtx.OriginalSessionID != "session-123" {
		t.Errorf("Expected session-123, got %s", diagCtx.OriginalSessionID)
	}
	if diagCtx.Platform != "slack" {
		t.Errorf("Expected slack, got %s", diagCtx.Platform)
	}
	if diagCtx.UserID != "user-456" {
		t.Errorf("Expected user-456, got %s", diagCtx.UserID)
	}
	if diagCtx.ChannelID != "channel-789" {
		t.Errorf("Expected channel-789, got %s", diagCtx.ChannelID)
	}
	if diagCtx.ThreadID != "thread-012" {
		t.Errorf("Expected thread-012, got %s", diagCtx.ThreadID)
	}
	if diagCtx.Trigger != TriggerAuto {
		t.Errorf("Expected TriggerAuto, got %s", diagCtx.Trigger)
	}
	if diagCtx.Error == nil {
		t.Error("Expected non-nil Error")
	}
	if diagCtx.Environment == nil {
		t.Error("Expected non-nil Environment")
	}
	if diagCtx.Conversation == nil {
		t.Error("Expected non-nil Conversation")
	}
	if diagCtx.Logs == nil {
		t.Error("Expected non-nil Logs")
	}
}

func TestCollectorCollectEnvInfo(t *testing.T) {
	collector := NewCollector(nil)
	envInfo := collector.collectEnvInfo()

	// collectEnvInfo always returns non-nil, but we check for defensive coding
	if envInfo == nil {
		t.Fatal("Expected non-nil EnvInfo")
	}

	// Now we can safely access fields
	if envInfo.HotPlexVersion == "" {
		t.Error("Expected non-empty HotPlexVersion")
	}
	if envInfo.GoVersion == "" {
		t.Error("Expected non-empty GoVersion")
	}
	if envInfo.OS == "" {
		t.Error("Expected non-empty OS")
	}
	if envInfo.Arch == "" {
		t.Error("Expected non-empty Arch")
	}
	// Note: Uptime may be negative or very small since the test runs quickly
	_ = envInfo.Uptime
}

func TestCollectorCollectWithNilError(t *testing.T) {
	collector := NewCollector(nil)

	trigger := NewBaseTrigger(TriggerCommand, "session-456", nil).
		SetPlatform("telegram").
		SetUserID("user-789")

	ctx := context.Background()
	diagCtx, err := collector.Collect(ctx, trigger)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if diagCtx.Error != nil {
		t.Error("Expected nil Error")
	}
}
