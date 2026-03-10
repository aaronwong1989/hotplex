package diag

import (
	"fmt"
	"strings"
)

// DiagnosticPrompt is the system prompt for Provider to perform diagnosis.
const DiagnosticPrompt = `You are a diagnostic assistant for HotPlex, an AI agent control plane.

When an error occurs in the system, analyze the provided context and help resolve the issue.

## Your Capabilities

You can:
1. **Analyze** the error and provide root cause analysis
2. **Suggest** fixes or workarounds
3. **Create** a GitHub issue if needed (use gh CLI or GitHub API)

## Guidelines

- Be concise and practical
- Focus on actionable insights
- If creating an issue, include:
  - Clear title
  - Reproduction steps
  - Expected vs actual behavior
- Use appropriate labels: bug, enhancement, question

Now analyze the following error context:`

// BuildDiagnosticPrompt builds a prompt for Provider to analyze the error.
// The Provider will receive structured context and can decide how to help.
func BuildDiagnosticPrompt(diagCtx *DiagContext) string {
	var sb strings.Builder

	sb.WriteString(DiagnosticPrompt)
	sb.WriteString("\n\n")

	// Session Info
	sb.WriteString("## Session Information\n")
	fmt.Fprintf(&sb, "- Session ID: %s\n", diagCtx.OriginalSessionID)
	fmt.Fprintf(&sb, "- Platform: %s\n", diagCtx.Platform)
	fmt.Fprintf(&sb, "- Trigger: %s\n", diagCtx.Trigger)
	fmt.Fprintf(&sb, "- Time: %s\n\n", diagCtx.Timestamp.Format("2006-01-02 15:04:05"))

	// Error Info
	if diagCtx.Error != nil {
		sb.WriteString("## Error Details\n")
		fmt.Fprintf(&sb, "- Type: %s\n", diagCtx.Error.Type)
		fmt.Fprintf(&sb, "- Message: %s\n", diagCtx.Error.Message)
		if diagCtx.Error.ExitCode != 0 {
			fmt.Fprintf(&sb, "- Exit Code: %d\n", diagCtx.Error.ExitCode)
		}
		if diagCtx.Error.StackTrace != "" {
			sb.WriteString("\n```\n")
			sb.WriteString(diagCtx.Error.StackTrace)
			sb.WriteString("\n```\n")
		}
		sb.WriteString("\n")
	}

	// Environment
	if diagCtx.Environment != nil {
		sb.WriteString("## Environment\n")
		fmt.Fprintf(&sb, "- HotPlex: %s\n", diagCtx.Environment.HotPlexVersion)
		fmt.Fprintf(&sb, "- Go: %s\n", diagCtx.Environment.GoVersion)
		fmt.Fprintf(&sb, "- OS: %s/%s\n", diagCtx.Environment.OS, diagCtx.Environment.Arch)
		fmt.Fprintf(&sb, "- Uptime: %s\n\n", diagCtx.Environment.Uptime)
	}

	// Conversation
	if diagCtx.Conversation != nil && diagCtx.Conversation.Processed != "" {
		sb.WriteString("## Recent Conversation\n")
		fmt.Fprintf(&sb, "- Messages: %d\n", diagCtx.Conversation.MessageCount)
		sb.WriteString("```\n")
		sb.WriteString(diagCtx.Conversation.Processed)
		sb.WriteString("\n```\n\n")
	}

	// Logs
	if len(diagCtx.Logs) > 0 {
		sb.WriteString("## Recent Logs\n")
		sb.WriteString("```\n")
		sb.Write(diagCtx.Logs)
		sb.WriteString("\n```\n")
	}

	sb.WriteString("\n## Your Task\n")
	sb.WriteString("Analyze this error and provide your recommendation:\n")
	sb.WriteString("1. What's the likely root cause?\n")
	sb.WriteString("2. How can we fix or work around it?\n")
	sb.WriteString("3. Should we create a GitHub issue? (If yes, create it with proper labels)\n")

	return sb.String()
}
