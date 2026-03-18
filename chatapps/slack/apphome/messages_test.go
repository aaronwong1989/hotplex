package apphome

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultMessagesConfig(t *testing.T) {
	cfg := DefaultMessagesConfig()
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.HomeTitle)
	assert.NotEmpty(t, cfg.HomeSubtitle)
	assert.NotEmpty(t, cfg.FormSubmitText)
	assert.NotEmpty(t, cfg.FormCancelText)
	assert.NotEmpty(t, cfg.FormExecuteText)
}

func TestMessagesConfig_GetHomeTitle(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{HomeTitle: "Custom Title"}
		assert.Equal(t, "Custom Title", cfg.GetHomeTitle())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().HomeTitle, cfg.GetHomeTitle())
	})
}

func TestMessagesConfig_GetHomeSubtitle(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{HomeSubtitle: "Custom Subtitle"}
		assert.Equal(t, "Custom Subtitle", cfg.GetHomeSubtitle())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().HomeSubtitle, cfg.GetHomeSubtitle())
	})
}

func TestMessagesConfig_GetFormSubmitText(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{FormSubmitText: "Submit"}
		assert.Equal(t, "Submit", cfg.GetFormSubmitText())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().FormSubmitText, cfg.GetFormSubmitText())
	})
}

func TestMessagesConfig_GetFormCancelText(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{FormCancelText: "Cancel"}
		assert.Equal(t, "Cancel", cfg.GetFormCancelText())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().FormCancelText, cfg.GetFormCancelText())
	})
}

func TestMessagesConfig_GetFormExecuteText(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{FormExecuteText: "Run"}
		assert.Equal(t, "Run", cfg.GetFormExecuteText())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().FormExecuteText, cfg.GetFormExecuteText())
	})
}

func TestMessagesConfig_GetRefreshButtonText(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{RefreshButtonText: "🔁"}
		assert.Equal(t, "🔁", cfg.GetRefreshButtonText())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().RefreshButtonText, cfg.GetRefreshButtonText())
	})
}

func TestMessagesConfig_GetExecutorHeader(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{ExecutorHeaderTemplate: "Running %s"}
		assert.Equal(t, "Running Test Cap", cfg.GetExecutorHeader("Test Cap"))
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		result := cfg.GetExecutorHeader("Test Cap")
		assert.Contains(t, result, "Test Cap")
	})
}

func TestMessagesConfig_GetWelcomeMessage(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{WelcomeTemplate: "Hello @%s!"}
		assert.Equal(t, "Hello @U123!", cfg.GetWelcomeMessage("U123"))
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		result := cfg.GetWelcomeMessage("U123")
		assert.Contains(t, result, "U123")
	})
}

func TestMessagesConfig_GetFooterHelp(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{FooterHelpText: "Custom help"}
		assert.Equal(t, "Custom help", cfg.GetFooterHelp())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().FooterHelpText, cfg.GetFooterHelp())
	})
}

func TestMessagesConfig_GetFooterPoweredBy(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{FooterPoweredBy: "Custom AI"}
		assert.Equal(t, "Custom AI", cfg.GetFooterPoweredBy())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().FooterPoweredBy, cfg.GetFooterPoweredBy())
	})
}

func TestMessagesConfig_GetRequiredFieldSuffix(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{RequiredFieldSuffix: " (required)"}
		assert.Equal(t, " (required)", cfg.GetRequiredFieldSuffix())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().RequiredFieldSuffix, cfg.GetRequiredFieldSuffix())
	})
}

func TestMessagesConfig_GetStatusInfo(t *testing.T) {
	t.Run("engine OK", func(t *testing.T) {
		cfg := &MessagesConfig{
			StatusEmoji:        "✅",
			StatusRunning:      "Online",
			StatusOfflineEmoji: "❌",
			StatusOffline:      "Offline",
		}
		emoji, text := cfg.GetStatusInfo(true)
		assert.Equal(t, "✅", emoji)
		assert.Equal(t, "Online", text)
	})

	t.Run("engine not OK", func(t *testing.T) {
		cfg := &MessagesConfig{
			StatusEmoji:        "✅",
			StatusRunning:      "Online",
			StatusOfflineEmoji: "❌",
			StatusOffline:      "Offline",
		}
		emoji, text := cfg.GetStatusInfo(false)
		assert.Equal(t, "❌", emoji)
		assert.Equal(t, "Offline", text)
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		var cfg *MessagesConfig
		defaultCfg := DefaultMessagesConfig()

		emoji, text := cfg.GetStatusInfo(true)
		assert.Equal(t, defaultCfg.StatusEmoji, emoji)
		assert.Equal(t, defaultCfg.StatusRunning, text)

		emoji, text = cfg.GetStatusInfo(false)
		assert.Equal(t, defaultCfg.StatusOfflineEmoji, emoji)
		assert.Equal(t, defaultCfg.StatusOffline, text)
	})
}

func TestMessagesConfig_GetTaskCountPrefix(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{TaskCountPrefix: "📊"}
		assert.Equal(t, "📊", cfg.GetTaskCountPrefix())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().TaskCountPrefix, cfg.GetTaskCountPrefix())
	})
}

func TestMessagesConfig_GetModelPrefix(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{ModelPrefix: "🤖"}
		assert.Equal(t, "🤖", cfg.GetModelPrefix())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().ModelPrefix, cfg.GetModelPrefix())
	})
}

func TestMessagesConfig_GetCatalogTitle(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := &MessagesConfig{CatalogTitle: "*Custom Catalog*"}
		assert.Equal(t, "*Custom Catalog*", cfg.GetCatalogTitle())
	})

	t.Run("nil config", func(t *testing.T) {
		var cfg *MessagesConfig
		assert.Equal(t, DefaultMessagesConfig().CatalogTitle, cfg.GetCatalogTitle())
	})
}

func TestSprintf(t *testing.T) {
	result := sprintf("Hello %s, you have %d items", "World", 42)
	assert.Equal(t, "Hello World, you have 42 items", result)
}
