package diag

import (
	"context"
	"errors"
	"testing"

	"log/slog"
	"os"
)

// mockBrain implements brain.Brain for testing
type mockBrain struct {
	response string
	err      error
}

func (m *mockBrain) Chat(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockBrain) Analyze(ctx context.Context, prompt string, target any) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func TestNewDiagnoser(t *testing.T) {
	t.Run("with logger and brain", func(t *testing.T) {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		brain := &mockBrain{response: "test"}
		diagnoser := NewDiagnoser(logger, brain)

		if diagnoser == nil {
			t.Fatal("Expected non-nil diagnoser")
		}
		if diagnoser.logger != logger {
			t.Error("Expected logger to be set")
		}
		if diagnoser.brain != brain {
			t.Error("Expected brain to be set")
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		brain := &mockBrain{response: "test"}
		diagnoser := NewDiagnoser(nil, brain)

		if diagnoser == nil {
			t.Fatal("Expected non-nil diagnoser")
		}
		// Should use default logger
		_ = diagnoser.logger
	})
}

func TestDiagnoserDiagnose(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("successful diagnosis", func(t *testing.T) {
		brain := &mockBrain{
			response: "Root cause: API rate limit exceeded. Solution: Wait and retry.",
		}
		diagnoser := NewDiagnoser(logger, brain)

		diagCtx := &DiagContext{
			OriginalSessionID: "test-session",
			Platform:          "slack",
			Trigger:           TriggerAuto,
		}

		result, err := diagnoser.Diagnose(context.Background(), diagCtx)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Analysis == "" {
			t.Error("Expected non-empty analysis")
		}
		if result.IssueCreated {
			t.Error("Expected IssueCreated to be false")
		}
	})

	t.Run("brain error", func(t *testing.T) {
		brain := &mockBrain{
			err: errors.New("brain unavailable"),
		}
		diagnoser := NewDiagnoser(logger, brain)

		diagCtx := &DiagContext{
			OriginalSessionID: "test-session",
		}

		result, err := diagnoser.Diagnose(context.Background(), diagCtx)

		if err == nil {
			t.Error("Expected error")
		}
		if result != nil {
			t.Error("Expected nil result on error")
		}
	})
}

func TestDiagnoserDiagnoseAndCreateIssue(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("issue suggested", func(t *testing.T) {
		brain := &mockBrain{
			response: "This is a bug. We should create an issue to track this.",
		}
		diagnoser := NewDiagnoser(logger, brain)

		diagCtx := &DiagContext{
			OriginalSessionID: "test-session",
		}

		result, err := diagnoser.DiagnoseAndCreateIssue(context.Background(), diagCtx)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !result.IssueCreated {
			t.Error("Expected IssueCreated to be true")
		}
		if result.IssueURL == "" {
			t.Error("Expected non-empty issue URL")
		}
	})

	t.Run("no issue suggested", func(t *testing.T) {
		brain := &mockBrain{
			response: "This is a configuration error. Fix the config file.",
		}
		diagnoser := NewDiagnoser(logger, brain)

		diagCtx := &DiagContext{
			OriginalSessionID: "test-session",
		}

		result, err := diagnoser.DiagnoseAndCreateIssue(context.Background(), diagCtx)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result.IssueCreated {
			t.Error("Expected IssueCreated to be false")
		}
	})
}

func TestContainsIssueSuggestion(t *testing.T) {
	tests := []struct {
		name     string
		analysis string
		expected bool
	}{
		{
			name:     "create issue",
			analysis: "We should create an issue for this bug.",
			expected: true,
		},
		{
			name:     "should create",
			analysis: "This is a problem that should create a ticket.",
			expected: true,
		},
		{
			name:     "open a bug",
			analysis: "Please open a bug report.",
			expected: true,
		},
		{
			name:     "report this",
			analysis: "You should report this issue.",
			expected: true,
		},
		{
			name:     "file a report",
			analysis: "We need to file a report on GitHub.",
			expected: true,
		},
		{
			name:     "no issue",
			analysis: "This is a configuration error. Please fix your config.",
			expected: false,
		},
		{
			name:     "empty",
			analysis: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIssueSuggestion(tt.analysis)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
