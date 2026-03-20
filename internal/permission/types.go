package permission

import (
	"regexp"
	"strings"
	"time"
)

// Decision represents the result of a permission check.
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
	DecisionBlocked
	DecisionUnknown
)

func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// PatternEntry represents a permission pattern with metadata.
type PatternEntry struct {
	Pattern   string    `json:"pattern"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// PermissionsFile represents the on-disk JSON structure.
type PermissionsFile struct {
	BotID     string         `json:"bot_id"`
	Whitelist []PatternEntry `json:"whitelist,omitempty"`
	Blacklist []PatternEntry `json:"blacklist,omitempty"`
}

// Pattern parses and matches a permission pattern.
// Format: {ToolName}:{CommandRegex}
// If no ":" is present, matches any command for the tool.
type Pattern struct {
	Value string
}

var toolCommandSplit = regexp.MustCompile(`:(.+)`)

// Match returns true if the pattern matches the given tool name and input.
func (p Pattern) Match(tool, input string) bool {
	if p.Value == "" {
		return false
	}

	matches := toolCommandSplit.FindStringSubmatch(p.Value)
	if len(matches) == 2 {
		// Format: ToolName:CommandRegex
		toolName := strings.TrimSuffix(p.Value, ":"+matches[1])
		if !strings.EqualFold(tool, toolName) {
			return false
		}
		re, err := regexp.Compile(matches[1])
		if err != nil {
			return false
		}
		return re.MatchString(input)
	}

	// No ":" — match any command for this tool
	return strings.EqualFold(tool, p.Value)
}
