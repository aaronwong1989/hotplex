package diag

import (
	"context"
	"testing"

	"log/slog"
	"os"
)

func TestNewNotifier(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		notifier := NewNotifier(logger)
		if notifier == nil {
			t.Fatal("Expected non-nil notifier")
		}
		if notifier.logger != logger {
			t.Error("Expected logger to be set")
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		notifier := NewNotifier(nil)
		if notifier == nil {
			t.Fatal("Expected non-nil notifier")
		}
		// Should use default logger
		_ = notifier.logger
	})
}

func TestNotifierNotify(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	notifier := NewNotifier(logger)

	t.Run("normal result", func(t *testing.T) {
		result := &NotifyResult{
			Analysis:     "Root cause: invalid API key",
			IssueCreated: false,
			IssueURL:     "",
		}

		err := notifier.Notify(context.Background(), "slack", "C123", "T456", result)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("with issue created", func(t *testing.T) {
		result := &NotifyResult{
			Analysis:     "Analyzed and created issue",
			IssueCreated: true,
			IssueURL:     "https://github.com/hrygo/hotplex/issues/123",
		}

		err := notifier.Notify(context.Background(), "telegram", "C789", "", result)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("with empty result", func(t *testing.T) {
		result := &NotifyResult{}

		err := notifier.Notify(context.Background(), "slack", "C000", "T000", result)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestNotifyResult(t *testing.T) {
	result := &NotifyResult{
		Analysis:     "test analysis",
		IssueCreated: true,
		IssueURL:     "https://github.com/hrygo/hotplex/issues/100",
	}

	if result.Analysis != "test analysis" {
		t.Errorf("Expected analysis, got %s", result.Analysis)
	}
	if !result.IssueCreated {
		t.Error("Expected IssueCreated to be true")
	}
	if result.IssueURL != "https://github.com/hrygo/hotplex/issues/100" {
		t.Errorf("Expected issue URL, got %s", result.IssueURL)
	}
}
