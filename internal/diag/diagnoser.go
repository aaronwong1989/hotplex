package diag

import (
	"context"
	"log/slog"

	"github.com/hrygo/hotplex/brain"
)

// Diagnoser performs diagnostic analysis using Brain.
type Diagnoser struct {
	logger *slog.Logger
	brain  brain.Brain
}

// NewDiagnoser creates a new Diagnoser with the given brain instance.
func NewDiagnoser(logger *slog.Logger, brain brain.Brain) *Diagnoser {
	if logger == nil {
		logger = slog.Default()
	}
	return &Diagnoser{
		logger: logger,
		brain:  brain,
	}
}

// Diagnose performs diagnostic analysis on the given context.
// It uses Brain to analyze the error and provide recommendations.
func (d *Diagnoser) Diagnose(ctx context.Context, diagCtx *DiagContext) (*NotifyResult, error) {
	// Build diagnostic prompt
	prompt := BuildDiagnosticPrompt(diagCtx)

	// Use Brain to analyze
	analysis, err := d.brain.Chat(ctx, prompt)
	if err != nil {
		d.logger.Error("Failed to get diagnostic analysis",
			"error", err,
			"session", diagCtx.OriginalSessionID)
		return nil, err
	}

	d.logger.Info("Diagnostic analysis completed",
		"session", diagCtx.OriginalSessionID,
		"analysis_length", len(analysis))

	return &NotifyResult{
		Analysis:     analysis,
		IssueCreated: false,
		IssueURL:     "",
	}, nil
}

// DiagnoseAndCreateIssue performs diagnosis and optionally creates a GitHub issue.
func (d *Diagnoser) DiagnoseAndCreateIssue(ctx context.Context, diagCtx *DiagContext) (*NotifyResult, error) {
	// Build diagnostic prompt with issue creation request
	prompt := BuildDiagnosticPrompt(diagCtx)
	prompt += "\n\nPlease analyze the error and if it's a valid bug, " +
		"create a GitHub issue with the title starting with [auto-generated].\n"

	// Use Brain to analyze and potentially create issue
	analysis, err := d.brain.Chat(ctx, prompt)
	if err != nil {
		d.logger.Error("Failed to get diagnostic analysis",
			"error", err,
			"session", diagCtx.OriginalSessionID)
		return nil, err
	}

	// Check if analysis suggests issue creation
	// This is a simple heuristic - in production, could parse structured response
	issueCreated := false
	issueURL := ""
	if containsIssueSuggestion(analysis) {
		issueCreated = true
		issueURL = "https://github.com/hrygo/hotplex/issues"
	}

	d.logger.Info("Diagnostic analysis completed",
		"session", diagCtx.OriginalSessionID,
		"issue_created", issueCreated)

	return &NotifyResult{
		Analysis:     analysis,
		IssueCreated: issueCreated,
		IssueURL:     issueURL,
	}, nil
}

// containsIssueSuggestion checks if the analysis suggests creating an issue.
func containsIssueSuggestion(analysis string) bool {
	// Simple heuristics for issue creation suggestion
	indicators := []string{
		"create an issue",
		"should create",
		"open a bug",
		"report this",
		"file a report",
	}
	lower := analysis
	for _, indicator := range indicators {
		if len(lower) > 0 && len(indicator) > 0 {
			// Simple substring check
			for i := 0; i <= len(lower)-len(indicator); i++ {
				if i+len(indicator) <= len(lower) {
					found := true
					for j := 0; j < len(indicator); j++ {
						if lower[i+j] != indicator[j] && lower[i+j] != indicator[j]-'a'+'A' {
							found = false
							break
						}
					}
					if found {
						return true
					}
				}
			}
		}
	}
	return false
}
