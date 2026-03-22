package base

import "strings"

// ToolCategory represents the classification of a tool based on its functionality.
// Used for generating contextual status labels in the Assistant Status API.
type ToolCategory int

const (
	ToolCategoryUnknown ToolCategory = iota
	ToolCategoryFileRead
	ToolCategoryFileWrite
	ToolCategoryFileEdit
	ToolCategoryBash
	ToolCategorySearch
	ToolCategoryWebFetch
	ToolCategoryList
	ToolCategoryAgent    // Task delegation, subagent spawning
	ToolCategorySkill    // Skill/plugin/MCP invocation
	ToolCategoryNotebook // Jupyter notebook operations
	ToolCategoryGit      // Version control operations
	ToolCategoryTest     // Test execution
	ToolCategoryDebug    // Debugging tools
	ToolCategoryBrowser  // Browser automation
	ToolCategoryAPI      // API/HTTP calls
	ToolCategoryDatabase // Database operations
	ToolCategoryThink    // Thinking/planning/reasoning
	ToolCategoryLSP      // Language server protocol
	ToolCategorySchedule // Scheduled tasks/cron
)

// Status label templates for internationalization support.
// These can be replaced with i18n lookup in the future.
var (
	StatusLabelFileRead    = "📄 正在读取文件 [%s]"
	StatusLabelFileWrite   = "📝 正在写入文件 [%s]"
	StatusLabelFileEdit    = "✂️ 正在编辑文件 [%s]"
	StatusLabelBash        = "⚡ 正在执行命令 [%s]"
	StatusLabelSearch      = "🔍 正在搜索 [%s]"
	StatusLabelWebFetch    = "🔗 正在请求网络 [%s]"
	StatusLabelList        = "📋 正在列出文件 [%s]"
	StatusLabelAgent       = "🎯 正在委派任务 [%s]"
	StatusLabelSkill       = "🧩 正在调用技能 [%s]"
	StatusLabelNotebook    = "📊 正在操作笔记本 [%s]"
	StatusLabelGit         = "🐙 正在操作 GitHub [%s]"
	StatusLabelTest        = "✅ 正在运行测试 [%s]"
	StatusLabelDebug       = "🔧 正在调试 [%s]"
	StatusLabelBrowser     = "🌎 正在浏览网页 [%s]"
	StatusLabelAPI         = "📡 正在调用API [%s]"
	StatusLabelDatabase    = "🗄 正在查询数据 [%s]"
	StatusLabelThink       = "🧠 正在思考分析 [%s]"
	StatusLabelLSP         = "💡 正在分析代码 [%s]"
	StatusLabelSchedule    = "⏰ 正在调度任务 [%s]"
	StatusLabelUnknown     = "⚙️ 正在执行 [%s]"
	StatusLabelUnknownTool = "未知工具"
)

// categoryPatterns defines keyword patterns for tool categorization.
// Order determines priority: earlier entries match first.
// IMPORTANT: More specific patterns must come BEFORE generic ones.
// E.g., "notebook" before "read", "git" before "write", "browser" before "bash".
var categoryPatterns = []struct {
	category ToolCategory
	keywords []string
}{
	// Notebook operations (before File operations - "notebook_edit" should be Notebook, not FileEdit)
	{ToolCategoryNotebook, []string{"notebook", "ipynb", "jupyter"}},

	// Git/Version control (before File operations - "git_write" should be Git, not FileWrite)
	{ToolCategoryGit, []string{"git", "commit", "push", "pull", "merge", "rebase", "checkout", "stash", "cherry", "clone"}},

	// Branch operations (before File operations - "create_branch" should be Git)
	{ToolCategoryGit, []string{"branch"}},

	// File operations (specific before generic)
	{ToolCategoryFileRead, []string{"read", "view", "cat", "head", "tail"}},
	{ToolCategoryFileWrite, []string{"write", "save"}},
	{ToolCategoryFileEdit, []string{"edit", "multiedit", "replace", "patch"}},

	// Agent/Task delegation (before Think - "task" is more specific than "plan")
	{ToolCategoryAgent, []string{"agent", "delegate", "spawn", "subagent", "dispatch", "teammate", "worker"}},

	// Task (before Bash - "task" is more specific than "run")
	{ToolCategoryAgent, []string{"task"}},

	// Skill/Plugin invocation
	{ToolCategorySkill, []string{"skill", "plugin", "mcp", "extension"}},

	// Thinking/Planning (before Agent - "plan" matches Think)
	{ToolCategoryThink, []string{"think", "plan", "reason", "brainstorm", "analyze"}},

	// Memory operations (before File operations)
	{ToolCategoryThink, []string{"memory"}},

	// LSP operations
	{ToolCategoryLSP, []string{"lsp", "definition", "reference", "hover", "symbol", "implementation"}},

	// Testing (before Bash - "run_test" should be Test, not Bash)
	{ToolCategoryTest, []string{"test", "spec", "coverage", "benchmark", "pytest", "jest"}},

	// Debugging
	{ToolCategoryDebug, []string{"debug", "breakpoint", "trace", "profile", "inspect"}},

	// Browser automation (before Bash - "run_selenium" should be Browser, not Bash)
	{ToolCategoryBrowser, []string{"browser", "playwright", "selenium", "puppeteer", "webdriver", "chrome", "firefox"}},

	// Command execution
	{ToolCategoryBash, []string{"bash", "shell", "exec", "run", "command", "terminal", "cli"}},

	// WebFetch (before Search - "websearch" → WebFetch, not Search)
	{ToolCategoryWebFetch, []string{"webfetch", "websearch", "webread", "curl", "wget", "http", "fetch", "request"}},

	// API/Webhook calls
	{ToolCategoryAPI, []string{"api", "webhook", "endpoint", "graphql", "rest"}},

	// Database operations
	{ToolCategoryDatabase, []string{"db", "database", "sql", "query", "migration", "postgres", "mysql", "sqlite", "mongo"}},

	// Search
	{ToolCategorySearch, []string{"search", "glob", "grep", "find", "ripgrep", "locate"}},

	// Directory listing (after FileRead - "tree_view" should be List, not FileRead)
	{ToolCategoryList, []string{"ls", "list", "dir", "tree"}},

	// Scheduled tasks (after Agent - "schedule_task" should be Schedule, not Agent)
	{ToolCategorySchedule, []string{"cron", "schedule", "timer", "interval", "periodic"}},

	// Create operations (after Git - "create_file" should be FileWrite, not Git)
	{ToolCategoryFileWrite, []string{"create"}},
}

// categoryMetadata is the single source of truth for category display properties.
var categoryMetadata = map[ToolCategory]struct {
	emoji       string
	statusLabel *string
}{
	ToolCategoryFileRead:  {"📄", &StatusLabelFileRead},
	ToolCategoryFileWrite: {"📝", &StatusLabelFileWrite},
	ToolCategoryFileEdit:  {"✂️", &StatusLabelFileEdit},
	ToolCategoryBash:      {"⚡", &StatusLabelBash},
	ToolCategorySearch:    {"🔍", &StatusLabelSearch},
	ToolCategoryWebFetch:  {"🔗", &StatusLabelWebFetch},
	ToolCategoryList:      {"📋", &StatusLabelList},
	ToolCategoryAgent:     {"🎯", &StatusLabelAgent},
	ToolCategorySkill:     {"🧩", &StatusLabelSkill},
	ToolCategoryNotebook:  {"📊", &StatusLabelNotebook},
	ToolCategoryGit:       {"🐙", &StatusLabelGit},
	ToolCategoryTest:      {"✅", &StatusLabelTest},
	ToolCategoryDebug:     {"🔧", &StatusLabelDebug},
	ToolCategoryBrowser:   {"🌎", &StatusLabelBrowser},
	ToolCategoryAPI:       {"📡", &StatusLabelAPI},
	ToolCategoryDatabase:  {"🗄", &StatusLabelDatabase},
	ToolCategoryThink:     {"🧠", &StatusLabelThink},
	ToolCategoryLSP:       {"💡", &StatusLabelLSP},
	ToolCategorySchedule:  {"⏰", &StatusLabelSchedule},
	ToolCategoryUnknown:   {"⚙️", &StatusLabelUnknown},
}

// CategorizeTool analyzes the tool name and returns its category.
// Uses substring matching for flexibility with different naming conventions.
// Priority is determined by categoryPatterns order (earlier = higher priority).
func CategorizeTool(toolName string) ToolCategory {
	name := strings.ToLower(toolName)

	for _, pattern := range categoryPatterns {
		if containsAny(name, pattern.keywords...) {
			return pattern.category
		}
	}
	return ToolCategoryUnknown
}

// CategoryEmoji returns the emoji representation for a tool category.
func CategoryEmoji(cat ToolCategory) string {
	if meta, ok := categoryMetadata[cat]; ok {
		return meta.emoji
	}
	return categoryMetadata[ToolCategoryUnknown].emoji
}

// CategoryStatusLabel returns the status label template for a tool category.
// Template format: "<emoji> <action> [%s]" where %s is replaced by tool name.
func CategoryStatusLabel(cat ToolCategory) string {
	if meta, ok := categoryMetadata[cat]; ok {
		return *meta.statusLabel
	}
	return StatusLabelUnknown
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
