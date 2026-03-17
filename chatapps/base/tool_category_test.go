package base

import (
	"fmt"
	"strings"
	"testing"
)

func TestCategorizeTool(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     ToolCategory
	}{
		// File Read
		{"read", "Read", ToolCategoryFileRead},
		{"view", "view_file", ToolCategoryFileRead},
		{"cat", "cat_file", ToolCategoryFileRead},
		{"head", "head_file", ToolCategoryFileRead},
		{"tail", "tail_file", ToolCategoryFileRead},

		// File Write
		{"write", "Write", ToolCategoryFileWrite},
		{"save", "save_file", ToolCategoryFileWrite},

		// File Edit
		{"edit", "Edit", ToolCategoryFileEdit},
		{"multiedit", "MultiEdit", ToolCategoryFileEdit},
		{"replace", "replace_content", ToolCategoryFileEdit},
		{"patch", "patch_file", ToolCategoryFileEdit},

		// Notebook
		{"notebook", "NotebookEdit", ToolCategoryNotebook},
		{"ipynb", "open_ipynb", ToolCategoryNotebook},

		// Agent/Task
		{"agent", "Agent", ToolCategoryAgent},
		{"task", "TaskOutput", ToolCategoryAgent},
		{"delegate", "delegate_work", ToolCategoryAgent},
		{"spawn", "spawn_worker", ToolCategoryAgent},
		{"subagent", "subagent_start", ToolCategoryAgent},
		{"execute_task", "execute_task", ToolCategoryAgent}, // "task" matches Agent, not Bash

		// Skill
		{"skill", "Skill", ToolCategorySkill},
		{"plugin", "plugin_load", ToolCategorySkill},
		{"mcp", "mcp_tool", ToolCategorySkill},

		// Think/Plan
		{"think", "think_process", ToolCategoryThink},
		{"plan", "planning_mode", ToolCategoryThink},
		{"memory", "memory_recall", ToolCategoryThink},

		// LSP
		{"lsp", "lsp_definition", ToolCategoryLSP},
		{"definition", "go_to_definition", ToolCategoryLSP},
		{"reference", "find_references", ToolCategoryLSP},
		{"symbol", "workspace_symbol", ToolCategoryLSP},

		// Test
		{"test", "test_runner", ToolCategoryTest},
		{"spec", "run_specs", ToolCategoryTest},
		{"coverage", "code_coverage", ToolCategoryTest},
		{"pytest", "pytest_exec", ToolCategoryTest},

		// Debug
		{"debug", "debug_session", ToolCategoryDebug},
		{"breakpoint", "set_breakpoint", ToolCategoryDebug},
		{"trace", "stack_trace", ToolCategoryDebug},

		// Git
		{"git", "git_status", ToolCategoryGit},
		{"commit", "commit_changes", ToolCategoryGit},
		{"push", "push_remote", ToolCategoryGit},
		{"pull", "pull_origin", ToolCategoryGit},
		{"branch", "switch_branch", ToolCategoryGit},
		{"merge", "merge_code", ToolCategoryGit},

		// Bash
		{"bash", "Bash", ToolCategoryBash},
		{"shell", "shell_command", ToolCategoryBash},
		{"exec", "exec_process", ToolCategoryBash},
		{"run", "run_script", ToolCategoryBash},
		{"command", "xyz_command", ToolCategoryBash}, // "command" matches Bash
		{"terminal", "terminal_run", ToolCategoryBash},

		// Browser
		{"browser", "browser_navigate", ToolCategoryBrowser},
		{"playwright", "playwright_click", ToolCategoryBrowser},
		{"selenium", "selenium_start", ToolCategoryBrowser},
		{"puppeteer", "puppeteer_open", ToolCategoryBrowser},

		// Web Fetch
		{"webfetch", "WebFetch", ToolCategoryWebFetch},
		{"websearch", "websearch_query", ToolCategoryWebFetch},
		{"curl", "curl_request", ToolCategoryWebFetch},
		{"wget", "wget_download", ToolCategoryWebFetch},
		{"http", "http_request", ToolCategoryWebFetch},
		{"fetch", "fetch_url", ToolCategoryWebFetch},

		// API
		{"api", "api_call", ToolCategoryAPI},
		{"webhook", "webhook_send", ToolCategoryAPI},
		{"graphql", "graphql_query", ToolCategoryAPI},

		// Database
		{"db", "db_query", ToolCategoryDatabase},
		{"database", "database_connect", ToolCategoryDatabase},
		{"sql", "sql_query", ToolCategoryDatabase},
		{"postgres", "postgres_query", ToolCategoryDatabase},
		{"mongo", "mongo_find", ToolCategoryDatabase},

		// Search
		{"search", "search_code", ToolCategorySearch},
		{"glob", "Glob", ToolCategorySearch},
		{"grep", "grep_pattern", ToolCategorySearch},
		{"find", "find_files", ToolCategorySearch},
		{"ripgrep", "ripgrep_search", ToolCategorySearch},

		// List
		{"ls", "ls_directory", ToolCategoryList},
		{"list", "list_files", ToolCategoryList},
		{"dir", "dir_content", ToolCategoryList},
		{"tree", "tree_structure", ToolCategoryList},

		// Schedule
		{"cron", "cron_job", ToolCategorySchedule},
		{"schedule", "job_schedule", ToolCategorySchedule},
		{"timer", "timer_start", ToolCategorySchedule},

		// Unknown
		{"empty string", "", ToolCategoryUnknown},
		{"unknown", "random_tool", ToolCategoryUnknown},

		// Case insensitivity
		{"uppercase", "READ", ToolCategoryFileRead},
		{"mixed case", "BaSh", ToolCategoryBash},

		// Substring matching
		{"substring read", "my_read_tool", ToolCategoryFileRead},
		{"substring write", "file_write_handler", ToolCategoryFileWrite},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategorizeTool(tt.toolName); got != tt.want {
				t.Errorf("CategorizeTool(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestCategoryEmoji(t *testing.T) {
	tests := []struct {
		name     string
		category ToolCategory
		want     string
	}{
		{"file read", ToolCategoryFileRead, "📄"},
		{"file write", ToolCategoryFileWrite, "📝"},
		{"file edit", ToolCategoryFileEdit, "✂️"},
		{"bash", ToolCategoryBash, "⚡"},
		{"search", ToolCategorySearch, "🔍"},
		{"web fetch", ToolCategoryWebFetch, "🔗"},
		{"list", ToolCategoryList, "📋"},
		{"agent", ToolCategoryAgent, "🎯"},
		{"skill", ToolCategorySkill, "🧩"},
		{"notebook", ToolCategoryNotebook, "📊"},
		{"git", ToolCategoryGit, "🌿"},
		{"test", ToolCategoryTest, "✅"},
		{"debug", ToolCategoryDebug, "🔧"},
		{"browser", ToolCategoryBrowser, "🌎"},
		{"api", ToolCategoryAPI, "📡"},
		{"database", ToolCategoryDatabase, "🗄"},
		{"think", ToolCategoryThink, "🧠"},
		{"lsp", ToolCategoryLSP, "💡"},
		{"schedule", ToolCategorySchedule, "⏰"},
		{"unknown", ToolCategoryUnknown, "⚙️"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategoryEmoji(tt.category); got != tt.want {
				t.Errorf("CategoryEmoji(%v) = %q, want %q", tt.category, got, tt.want)
			}
		})
	}
}

func TestCategoryStatusLabel(t *testing.T) {
	tests := []struct {
		name     string
		category ToolCategory
		contains string
	}{
		{"file read", ToolCategoryFileRead, "读取"},
		{"file write", ToolCategoryFileWrite, "写入"},
		{"file edit", ToolCategoryFileEdit, "编辑"},
		{"bash", ToolCategoryBash, "命令"},
		{"search", ToolCategorySearch, "搜索"},
		{"web fetch", ToolCategoryWebFetch, "请求"},
		{"list", ToolCategoryList, "列出"},
		{"agent", ToolCategoryAgent, "委派"},
		{"skill", ToolCategorySkill, "技能"},
		{"notebook", ToolCategoryNotebook, "笔记本"},
		{"git", ToolCategoryGit, "版本"},
		{"test", ToolCategoryTest, "测试"},
		{"debug", ToolCategoryDebug, "调试"},
		{"browser", ToolCategoryBrowser, "浏览"},
		{"api", ToolCategoryAPI, "API"},
		{"database", ToolCategoryDatabase, "查询"},
		{"think", ToolCategoryThink, "思考"},
		{"lsp", ToolCategoryLSP, "分析"},
		{"schedule", ToolCategorySchedule, "调度"},
		{"unknown", ToolCategoryUnknown, "执行"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := CategoryStatusLabel(tt.category)

			// All labels should contain %s placeholder
			if !strings.Contains(label, "%s") {
				t.Errorf("CategoryStatusLabel(%v) missing %%s placeholder: %q", tt.category, label)
			}

			// All labels should contain expected keyword
			if !strings.Contains(label, tt.contains) {
				t.Errorf("CategoryStatusLabel(%v) should contain %q, got: %q", tt.category, tt.contains, label)
			}
		})
	}
}

func TestCategoryStatusLabel_FormatString(t *testing.T) {
	// Test that all status labels can be formatted with tool name
	categories := []struct {
		name     string
		category ToolCategory
	}{
		{"unknown", ToolCategoryUnknown},
		{"file read", ToolCategoryFileRead},
		{"file write", ToolCategoryFileWrite},
		{"file edit", ToolCategoryFileEdit},
		{"bash", ToolCategoryBash},
		{"search", ToolCategorySearch},
		{"web fetch", ToolCategoryWebFetch},
		{"list", ToolCategoryList},
	}

	for _, tt := range categories {
		t.Run(tt.name, func(t *testing.T) {
			label := CategoryStatusLabel(tt.category)
			// Should not panic when formatting
			result := fmt.Sprintf(label, "TestTool")
			if result == "" {
				t.Errorf("CategoryStatusLabel(%v) produced empty result", tt.category)
			}
		})
	}
}

// TestCategorizeTool_PriorityConflicts tests that categoryPatterns order is respected
// when a tool name matches multiple patterns.
func TestCategorizeTool_PriorityConflicts(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     ToolCategory
		reason   string
	}{
		// "websearch" contains both "websearch" and "search"
		// WebFetch comes before Search in categoryPatterns, so WebFetch wins
		{"websearch vs search", "websearch", ToolCategoryWebFetch, "WebFetch pattern should have priority over Search"},
		{"websearch_query", "websearch_query", ToolCategoryWebFetch, "WebFetch should match 'websearch' substring"},

		// "search" alone should still match Search category
		{"search alone", "search", ToolCategorySearch, "Plain 'search' should match Search category"},
		{"search_files", "search_files", ToolCategorySearch, "Plain search should match Search category"},

		// Edge cases with multiple keyword matches
		{"webfetch contains fetch", "webfetch", ToolCategoryWebFetch, "WebFetch should match 'webfetch'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CategorizeTool(tt.toolName)
			if got != tt.want {
				t.Errorf("CategorizeTool(%q) = %v, want %v. Reason: %s",
					tt.toolName, got, tt.want, tt.reason)
			}
		})
	}
}

// TestCategorizeTool_SpecialCharacters tests handling of unusual input.
func TestCategorizeTool_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     ToolCategory
	}{
		{"special chars only", "!!!@@@", ToolCategoryUnknown},
		{"numbers only", "12345", ToolCategoryUnknown},
		{"unicode chars", "读取文件", ToolCategoryUnknown},               // Chinese chars without latin keywords
		{"mixed valid and special", "read!!!", ToolCategoryFileRead}, // Contains "read"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategorizeTool(tt.toolName); got != tt.want {
				t.Errorf("CategorizeTool(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}
