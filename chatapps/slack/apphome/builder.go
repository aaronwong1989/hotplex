package apphome

import (
	"fmt"

	"github.com/slack-go/slack"
)

const (
	// ActionIDPrefix is the prefix for capability button action IDs.
	ActionIDPrefix = "cap_click:"

	// HomeTitle is the primary title displayed in the App Home.
	HomeTitle = "🔥 HotPlex 能力中心"

	// HomeSubtitle is the secondary title or tagline.
	HomeSubtitle = "AI-Driven Developer Capability Center • Powered by Native Brain"

	// MaxCapabilitiesPerRow is the maximum number of capability cards per row.
	MaxCapabilitiesPerRow = 3
)

// Builder constructs Slack App Home Tab views.
type Builder struct {
	registry *Registry
}

// NewBuilder creates a new App Home builder.
func NewBuilder(registry *Registry) *Builder {
	return &Builder{
		registry: registry,
	}
}

// HomeState represents the dynamic state of the App Home tab.
type HomeState struct {
	UserID    string
	UserName  string
	EngineOK  bool
	TaskCount int
	ModelInfo string
}

// BuildHomeTab constructs the complete Home Tab view with default state.
func (b *Builder) BuildHomeTab() *slack.HomeTabViewRequest {
	return b.BuildFullHomeView(HomeState{})
}

// BuildBlocks constructs the block set for the Home Tab.
func (b *Builder) BuildBlocks() []slack.Block {
	var blocks []slack.Block

	// Header
	blocks = append(blocks, b.buildHeader()...)

	// Group capabilities by category
	categories := b.registry.GetCategories()
	capabilities := b.registry.GetAll()

	// Build capability map by category
	capByCategory := make(map[string][]Capability)
	for _, cap := range capabilities {
		capByCategory[cap.Category] = append(capByCategory[cap.Category], cap)
	}

	// Build each category section
	for _, cat := range categories {
		caps, ok := capByCategory[cat.ID]
		if !ok || len(caps) == 0 {
			continue
		}

		// Category header
		blocks = append(blocks, b.buildCategoryHeader(cat))

		// Capability cards (grouped in rows)
		for i := 0; i < len(caps); i += MaxCapabilitiesPerRow {
			end := i + MaxCapabilitiesPerRow
			if end > len(caps) {
				end = len(caps)
			}
			row := caps[i:end]
			blocks = append(blocks, b.buildCapabilityRow(row))
		}

		// Spacer between categories
		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Footer with help text
	blocks = append(blocks, b.buildFooter())

	return blocks
}

// buildHeader creates the main header block with a subtitle.
func (b *Builder) buildHeader() []slack.Block {
	headerText := slack.NewTextBlockObject(slack.PlainTextType, HomeTitle, false, false)
	header := slack.NewHeaderBlock(headerText)

	subtitleText := slack.NewTextBlockObject(slack.MarkdownType, HomeSubtitle, false, false)
	subtitle := slack.NewContextBlock("", subtitleText)

	return []slack.Block{header, subtitle}
}

// buildWelcome creates a personalized welcome block.
func (b *Builder) buildWelcome(userID, userName string) slack.Block {
	text := fmt.Sprintf("👋 欢迎回来, <@%s>!", userID)
	if userName != "" {
		text = fmt.Sprintf("👋 欢迎回来, *%s*!", userName)
	}
	welcomeText := slack.NewTextBlockObject(slack.MarkdownType, text, false, false)
	return slack.NewSectionBlock(welcomeText, nil, nil)
}

// buildStatsSection creates a status and statistics summary.
func (b *Builder) buildStatsSection(state HomeState) []slack.Block {
	statusEmoji := "🟢"
	statusText := "运行良好"
	if !state.EngineOK {
		statusEmoji = "🔴"
		statusText = "引擎离线"
	}

	fields := []*slack.TextBlockObject{
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*系统状态*\n%s %s", statusEmoji, statusText), false, false),
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*累计任务*\n🚀 %d", state.TaskCount), false, false),
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*当前模型*\n🧠 %s", state.ModelInfo), false, false),
	}

	refreshBtn := slack.NewButtonBlockElement("app_home_refresh", "refresh", slack.NewTextBlockObject(slack.PlainTextType, "🔄 刷新状态", false, false))

	return []slack.Block{
		slack.NewSectionBlock(nil, fields, slack.NewAccessory(refreshBtn)),
	}
}

// buildCategoryHeader creates a category section header.
func (b *Builder) buildCategoryHeader(cat CategoryInfo) slack.Block {
	text := fmt.Sprintf("%s *%s*", cat.Icon, cat.Name)
	headerText := slack.NewTextBlockObject(slack.MarkdownType, text, false, false)
	return slack.NewSectionBlock(headerText, nil, nil)
}

// buildCapabilityRow creates a row of capability cards.
// Note: This returns a section with fields. Individual capabilities should use BuildCapabilitySection.
func (b *Builder) buildCapabilityRow(caps []Capability) slack.Block {
	if len(caps) == 0 {
		return nil
	}

	// For Slack, we use section blocks with fields for multi-column layout
	var fields []*slack.TextBlockObject
	for _, cap := range caps {
		text := fmt.Sprintf("%s *%s*\n_%s_", cap.Icon, cap.Name, cap.Description)
		fields = append(fields, slack.NewTextBlockObject(slack.MarkdownType, text, false, false))
	}

	// Use fields for multi-column layout
	return slack.NewSectionBlock(nil, fields, nil)
}

// BuildCapabilitySection creates a section block for a single capability with a button.
func (b *Builder) BuildCapabilitySection(cap Capability) slack.Block {
	// Main text
	text := fmt.Sprintf("%s *%s*\n_%s_", cap.Icon, cap.Name, cap.Description)
	mainText := slack.NewTextBlockObject(slack.MarkdownType, text, false, false)

	// Execute button
	btn := slack.NewButtonBlockElement(
		ActionIDPrefix+cap.ID,
		cap.ID,
		slack.NewTextBlockObject(slack.PlainTextType, "执行", false, false),
	)

	return slack.NewSectionBlock(mainText, nil, slack.NewAccessory(btn))
}

// buildFooter creates a footer with help text.
func (b *Builder) buildFooter() slack.Block {
	helpText := slack.NewTextBlockObject(
		slack.MarkdownType,
		"_点击能力卡片上的「执行」按钮开始使用。_\n_能力中心由 Native Brain 智能驱动。_",
		false, false,
	)
	return slack.NewContextBlock("", helpText)
}

// BuildCapabilityBlocks builds all capability blocks organized by category.
func (b *Builder) BuildCapabilityBlocks() []slack.Block {
	var blocks []slack.Block

	categories := b.registry.GetCategories()
	capabilities := b.registry.GetAll()

	// Build capability map by category
	capByCategory := make(map[string][]Capability)
	for _, cap := range capabilities {
		capByCategory[cap.Category] = append(capByCategory[cap.Category], cap)
	}

	// Build each category section
	for _, cat := range categories {
		caps, ok := capByCategory[cat.ID]
		if !ok || len(caps) == 0 {
			continue
		}

		// Category header
		blocks = append(blocks, b.buildCategoryHeader(cat))

		// Each capability as a section with button
		for _, cap := range caps {
			blocks = append(blocks, b.BuildCapabilitySection(cap))
		}
	}

	return blocks
}

// BuildFullHomeView builds the complete Home Tab with dynamic state.
func (b *Builder) BuildFullHomeView(state HomeState) *slack.HomeTabViewRequest {
	var blocks []slack.Block

	// 1. Header & Subtitle
	blocks = append(blocks, b.buildHeader()...)
	blocks = append(blocks, slack.NewDividerBlock())

	// 2. Personalized Welcome
	if state.UserID != "" {
		blocks = append(blocks, b.buildWelcome(state.UserID, state.UserName))
	}

	// 3. Stats Section
	if state.ModelInfo == "" {
		state.ModelInfo = "Claude 3.5 Sonnet" // Default fallback
	}
	blocks = append(blocks, b.buildStatsSection(state)...)
	blocks = append(blocks, slack.NewDividerBlock())

	// 4. Capability Catalog Header
	catalogTitle := slack.NewTextBlockObject(slack.MarkdownType, "*🔭 能力目录*", false, false)
	blocks = append(blocks, slack.NewSectionBlock(catalogTitle, nil, nil))

	// 5. Capabilities by category
	blocks = append(blocks, b.BuildCapabilityBlocks()...)

	// 6. Footer
	blocks = append(blocks, slack.NewDividerBlock())
	blocks = append(blocks, b.buildFooter())

	return &slack.HomeTabViewRequest{
		Type:   slack.VTHomeTab,
		Blocks: slack.Blocks{BlockSet: blocks},
	}
}
