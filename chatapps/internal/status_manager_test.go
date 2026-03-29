package internal

import (
	"context"
	"log/slog"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

// mockStatusProvider is a mock implementation of StatusProvider for testing
type mockStatusProvider struct {
	calls []struct {
		method    string
		channelID string
		threadTS  string
		status    base.StatusType
		text      string
	}
}

func (m *mockStatusProvider) SetStatus(ctx context.Context, channelID, threadTS string, status base.StatusType, text string) error {
	m.calls = append(m.calls, struct {
		method    string
		channelID string
		threadTS  string
		status    base.StatusType
		text      string
	}{
		method:    "SetStatus",
		channelID: channelID,
		threadTS:  threadTS,
		status:    status,
		text:      text,
	})
	return nil
}

func (m *mockStatusProvider) ClearStatus(ctx context.Context, channelID, threadTS string) error {
	m.calls = append(m.calls, struct {
		method    string
		channelID string
		threadTS  string
		status    base.StatusType
		text      string
	}{
		method:    "ClearStatus",
		channelID: channelID,
		threadTS:  threadTS,
	})
	return nil
}

func TestStatusManager_Notify(t *testing.T) {
	provider := &mockStatusProvider{}
	logger := testLogger()
	manager := NewStatusManager(provider, logger)

	ctx := context.Background()

	// First notification should call provider
	err := manager.Notify(ctx, "C123", "T100", base.StatusThinking, "Thinking...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(provider.calls))
	}

	if provider.calls[0].method != "SetStatus" {
		t.Errorf("expected SetStatus, got %s", provider.calls[0].method)
	}

	if provider.calls[0].status != base.StatusThinking {
		t.Errorf("expected StatusThinking, got %s", provider.calls[0].status)
	}
}

func TestStatusManager_Notify_Deduplication(t *testing.T) {
	provider := &mockStatusProvider{}
	logger := testLogger()
	manager := NewStatusManager(provider, logger)

	ctx := context.Background()

	// First notification
	_ = manager.Notify(ctx, "C123", "T100", base.StatusThinking, "Thinking...")

	// Second notification with same status and text - should be deduplicated
	_ = manager.Notify(ctx, "C123", "T100", base.StatusThinking, "Thinking...")

	if len(provider.calls) != 1 {
		t.Fatalf("expected 1 call (deduplicated), got %d", len(provider.calls))
	}
}

func TestStatusManager_Notify_StatusChange(t *testing.T) {
	provider := &mockStatusProvider{}
	logger := testLogger()
	manager := NewStatusManager(provider, logger)

	ctx := context.Background()

	// First notification - thinking
	_ = manager.Notify(ctx, "C123", "T100", base.StatusThinking, "Thinking...")

	// Status change - should trigger new call
	_ = manager.Notify(ctx, "C123", "T100", base.StatusToolUse, "Using tool...")

	if len(provider.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(provider.calls))
	}

	if provider.calls[1].status != base.StatusToolUse {
		t.Errorf("expected StatusToolUse, got %s", provider.calls[1].status)
	}
}

func TestStatusManager_Clear(t *testing.T) {
	provider := &mockStatusProvider{}
	logger := testLogger()
	manager := NewStatusManager(provider, logger)

	ctx := context.Background()

	// Set a status first
	_ = manager.Notify(ctx, "C123", "T100", base.StatusThinking, "Thinking...")

	// Clear status
	err := manager.Clear(ctx, "C123", "T100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(provider.calls))
	}

	if provider.calls[1].method != "ClearStatus" {
		t.Errorf("expected ClearStatus, got %s", provider.calls[1].method)
	}
}

// TestStatusManager_Notify_ClearThenRestore verifies that clearing status via Notify("")
// (e.g., session_stats with empty text) correctly updates internal state so that
// subsequent Notify with the same non-empty text as before is NOT incorrectly
// deduplicated. This is a regression test for the bug where step_finish status
// ("✅ 当前任务阶段构建完成") was stuck because the clear (text="") didn't update
// internal state, causing the next step_finish Notify to be blocked by the
// throttle check (m.current == status && m.lastText == text).
func TestStatusManager_Notify_ClearThenRestore(t *testing.T) {
	provider := &mockStatusProvider{}
	logger := testLogger()
	manager := NewStatusManager(provider, logger)

	ctx := context.Background()

	// Step 1: Set a non-empty status (e.g., step_finish)
	_ = manager.Notify(ctx, "C123", "T100", base.StatusStepFinish, "✅ 当前任务阶段构建完成")

	if len(provider.calls) != 1 {
		t.Fatalf("step 1: expected 1 call, got %d", len(provider.calls))
	}
	if provider.calls[0].method != "SetStatus" {
		t.Errorf("step 1: expected SetStatus, got %s", provider.calls[0].method)
	}

	// Step 2: Clear status via Notify with empty text (how handleSessionStats clears)
	_ = manager.Notify(ctx, "C123", "T100", base.StatusSessionStats, "")

	// BUG: before the fix, ClearStatus was called but internal state was NOT updated.
	// This meant the next step_finish Notify would see:
	//   m.current = StatusStepFinish, m.lastText = "✅ 当前任务阶段构建完成"
	// and throttle incorrectly blocked it.
	//
	// After fix: ClearStatus is called AND internal state is updated, so the
	// subsequent step_finish Notify is correctly dispatched.
	if len(provider.calls) != 2 {
		t.Fatalf("step 2: expected 2 calls, got %d", len(provider.calls))
	}
	if provider.calls[1].method != "ClearStatus" {
		t.Errorf("step 2: expected ClearStatus, got %s", provider.calls[1].method)
	}

	// Step 3: Notify with same non-empty text as step 1.
	// This should NOT be deduplicated (should call SetStatus again).
	// BUG: before fix, this was incorrectly blocked by throttle.
	_ = manager.Notify(ctx, "C123", "T100", base.StatusStepFinish, "✅ 当前任务阶段构建完成")

	if len(provider.calls) != 3 {
		t.Fatalf("step 3: expected 3 calls (clear-then-restore should NOT deduplicate), got %d", len(provider.calls))
	}
	if provider.calls[2].method != "SetStatus" {
		t.Errorf("step 3: expected SetStatus, got %s", provider.calls[2].method)
	}
	if provider.calls[2].text != "✅ 当前任务阶段构建完成" {
		t.Errorf("step 3: expected text '✅ 当前任务阶段构建完成', got %q", provider.calls[2].text)
	}
}

func TestStatusManager_Current(t *testing.T) {
	provider := &mockStatusProvider{}
	logger := testLogger()
	manager := NewStatusManager(provider, logger)

	ctx := context.Background()

	// Initial state should be empty (not yet set)
	if manager.Current() != "" {
		t.Errorf("expected initial empty status, got %s", manager.Current())
	}

	// After notify, should be updated
	_ = manager.Notify(ctx, "C123", "T100", base.StatusThinking, "Thinking...")
	if manager.Current() != base.StatusThinking {
		t.Errorf("expected StatusThinking, got %s", manager.Current())
	}

	// After clear, should be idle
	_ = manager.Clear(ctx, "C123", "T100")
	if manager.Current() != base.StatusIdle {
		t.Errorf("expected StatusIdle after clear, got %s", manager.Current())
	}
}

func testLogger() *slog.Logger {
	return slog.Default()
}
