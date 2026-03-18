package apphome

import "fmt"

// MessagesConfig holds all user-facing text that can be customized.
type MessagesConfig struct {
	// Executor messages
	ExecutorHeaderTemplate string // Template for capability execution header (e.g., "🎯 *%s* — 执行能力任务")

	// Form buttons
	FormSubmitText  string // Modal submit button text (default: "执行")
	FormCancelText  string // Modal cancel button text (default: "取消")
	FormExecuteText string // Capability card button text (default: "执行")

	// Builder messages
	HomeTitle    string // Home tab title (default: "🔥 HotPlex 能力中心")
	HomeSubtitle string // Home tab subtitle (default: "AI-Driven Developer Capability Center")

	// Validation
	RequiredFieldSuffix string // Suffix for required field labels (default: " *")

	// Status messages
	StatusRunning      string // System running status (default: "运行良好")
	StatusOffline      string // System offline status (default: "引擎离线")
	StatusEmoji        string // Status emoji (default: "🟢")
	StatusOfflineEmoji string // Offline emoji (default: "🔴")

	// Capability labels
	TaskCountPrefix string // Prefix for task count (default: "🚀")
	ModelPrefix     string // Prefix for model info (default: "🧠")

	// Button labels
	RefreshButtonText string // Refresh button text (default: "🔄 刷新状态")

	// Footer
	FooterHelpText  string // Footer help text
	FooterPoweredBy string // Footer "powered by" text (default: "Native Brain")

	// Welcome message
	WelcomeTemplate string // Welcome message template (e.g., "👋 欢迎回来, <@%s>!")

	// Category headers (optional per-category customization)
	CategoryHeaders map[string]string // category ID -> header text
}

// DefaultMessagesConfig returns the default messages configuration.
func DefaultMessagesConfig() *MessagesConfig {
	return &MessagesConfig{
		ExecutorHeaderTemplate: "🎯 *%s* — 执行能力任务",

		FormSubmitText:  "执行",
		FormCancelText:  "取消",
		FormExecuteText: "执行",

		HomeTitle:    "🔥 HotPlex 能力中心",
		HomeSubtitle: "AI-Driven Developer Capability Center • Powered by Native Brain",

		RequiredFieldSuffix: " *",

		StatusRunning:      "运行良好",
		StatusOffline:      "引擎离线",
		StatusEmoji:        "🟢",
		StatusOfflineEmoji: "🔴",

		TaskCountPrefix: "🚀",
		ModelPrefix:     "🧠",

		RefreshButtonText: "🔄 刷新状态",

		FooterHelpText:  "_点击能力卡片上的「执行」按钮开始使用。_",
		FooterPoweredBy: "Native Brain",

		WelcomeTemplate: "👋 欢迎回来, <@%s>!",

		CategoryHeaders: make(map[string]string),
	}
}

// GetHomeTitle returns the home title with fallback.
func (c *MessagesConfig) GetHomeTitle() string {
	if c != nil && c.HomeTitle != "" {
		return c.HomeTitle
	}
	return DefaultMessagesConfig().HomeTitle
}

// GetHomeSubtitle returns the home subtitle with fallback.
func (c *MessagesConfig) GetHomeSubtitle() string {
	if c != nil && c.HomeSubtitle != "" {
		return c.HomeSubtitle
	}
	return DefaultMessagesConfig().HomeSubtitle
}

// GetFormSubmitText returns the form submit text with fallback.
func (c *MessagesConfig) GetFormSubmitText() string {
	if c != nil && c.FormSubmitText != "" {
		return c.FormSubmitText
	}
	return DefaultMessagesConfig().FormSubmitText
}

// GetFormCancelText returns the form cancel text with fallback.
func (c *MessagesConfig) GetFormCancelText() string {
	if c != nil && c.FormCancelText != "" {
		return c.FormCancelText
	}
	return DefaultMessagesConfig().FormCancelText
}

// GetFormExecuteText returns the execute button text with fallback.
func (c *MessagesConfig) GetFormExecuteText() string {
	if c != nil && c.FormExecuteText != "" {
		return c.FormExecuteText
	}
	return DefaultMessagesConfig().FormExecuteText
}

// GetRefreshButtonText returns the refresh button text with fallback.
func (c *MessagesConfig) GetRefreshButtonText() string {
	if c != nil && c.RefreshButtonText != "" {
		return c.RefreshButtonText
	}
	return DefaultMessagesConfig().RefreshButtonText
}

// GetExecutorHeader returns formatted executor header.
func (c *MessagesConfig) GetExecutorHeader(capName string) string {
	if c != nil && c.ExecutorHeaderTemplate != "" {
		return sprintf(c.ExecutorHeaderTemplate, capName)
	}
	return sprintf(DefaultMessagesConfig().ExecutorHeaderTemplate, capName)
}

// GetWelcomeMessage returns formatted welcome message.
func (c *MessagesConfig) GetWelcomeMessage(userID string) string {
	if c != nil && c.WelcomeTemplate != "" {
		return sprintf(c.WelcomeTemplate, userID)
	}
	return sprintf(DefaultMessagesConfig().WelcomeTemplate, userID)
}

// GetFooterHelp returns the footer help text.
func (c *MessagesConfig) GetFooterHelp() string {
	if c != nil && c.FooterHelpText != "" {
		return c.FooterHelpText
	}
	return DefaultMessagesConfig().FooterHelpText
}

// GetFooterPoweredBy returns the footer powered by text.
func (c *MessagesConfig) GetFooterPoweredBy() string {
	if c != nil && c.FooterPoweredBy != "" {
		return c.FooterPoweredBy
	}
	return DefaultMessagesConfig().FooterPoweredBy
}

// GetRequiredFieldSuffix returns the required field suffix.
func (c *MessagesConfig) GetRequiredFieldSuffix() string {
	if c != nil && c.RequiredFieldSuffix != "" {
		return c.RequiredFieldSuffix
	}
	return DefaultMessagesConfig().RequiredFieldSuffix
}

// GetStatusInfo returns status emoji and text based on engine state.
func (c *MessagesConfig) GetStatusInfo(engineOK bool) (emoji, text string) {
	if c != nil {
		if engineOK {
			return c.StatusEmoji, c.StatusRunning
		}
		return c.StatusOfflineEmoji, c.StatusOffline
	}
	cfg := DefaultMessagesConfig()
	if engineOK {
		return cfg.StatusEmoji, cfg.StatusRunning
	}
	return cfg.StatusOfflineEmoji, cfg.StatusOffline
}

// GetTaskCountPrefix returns the task count prefix.
func (c *MessagesConfig) GetTaskCountPrefix() string {
	if c != nil && c.TaskCountPrefix != "" {
		return c.TaskCountPrefix
	}
	return DefaultMessagesConfig().TaskCountPrefix
}

// GetModelPrefix returns the model prefix.
func (c *MessagesConfig) GetModelPrefix() string {
	if c != nil && c.ModelPrefix != "" {
		return c.ModelPrefix
	}
	return DefaultMessagesConfig().ModelPrefix
}

// sprintf is a simple fmt.Sprintf wrapper for internal use.
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
