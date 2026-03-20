package slack

import (
	"encoding/json"
	"testing"
)

func TestBuildPermissionCardBlocks(t *testing.T) {
	blocks := BuildPermissionCardBlocks(
		"U0AHRCL1KCM",
		"abc123",
		"msg456",
		"Bash",
		"rm -rf /tmp/test",
		"user789",
	)

	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// Verify it's valid Slack blocks JSON
	data, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("Marshal blocks failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestBuildPermissionResultBlocks(t *testing.T) {
	tests := []struct {
		decision  string
		wantEmoji bool
	}{
		{"allow", true},
		{"allow_once", true},
		{"allow_always", true},
		{"deny", true},
		{"deny_once", true},
		{"deny_all", true},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.decision, func(t *testing.T) {
			blocks := BuildPermissionResultBlocks(tt.decision, "Bash", "rm -rf /")
			if len(blocks) == 0 {
				t.Errorf("BuildPermissionResultBlocks(%q) returned empty", tt.decision)
			}
		})
	}
}

func TestBuildPermissionDeniedCard(t *testing.T) {
	blocks := BuildPermissionDeniedCard("Bash", "chmod 777 /etc", "用户拒绝")
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	data, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("Marshal denied card failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestMakePermissionActionID(t *testing.T) {
	sessionID := "sess_abc"
	msgID := "msg_123"

	tests := []struct {
		action   string
		expected string
	}{
		{"allow_once", "perm_allow_once:sess_abc:msg_123"},
		{"allow_always", "perm_allow_always:sess_abc:msg_123"},
		{"deny_once", "perm_deny_once:sess_abc:msg_123"},
		{"deny_all", "perm_deny_all:sess_abc:msg_123"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := MakePermissionActionID(tt.action, sessionID, msgID)
			if got != tt.expected {
				t.Errorf("MakePermissionActionID(%q) = %q, want %q", tt.action, got, tt.expected)
			}
		})
	}
}

func TestParsePermissionActionID(t *testing.T) {
	tests := []struct {
		actionID string
		wantOK   bool
		wantAct  string
		wantSess string
		wantMsg  string
	}{
		{"perm_allow_once:sess_abc:msg_123", true, "perm_allow_once", "sess_abc", "msg_123"},
		{"perm_deny_all:xxx:yyy", true, "perm_deny_all", "xxx", "yyy"},
		{"invalid", false, "", "", ""},
		{"perm_only:sess", false, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.actionID, func(t *testing.T) {
			action, sessionID, msgID, ok := ParsePermissionActionID(tt.actionID)
			if ok != tt.wantOK {
				t.Errorf("ParsePermissionActionID(%q) ok = %v, want %v", tt.actionID, ok, tt.wantOK)
			}
			if ok {
				if action != tt.wantAct {
					t.Errorf("action = %q, want %q", action, tt.wantAct)
				}
				if sessionID != tt.wantSess {
					t.Errorf("sessionID = %q, want %q", sessionID, tt.wantSess)
				}
				if msgID != tt.wantMsg {
					t.Errorf("msgID = %q, want %q", msgID, tt.wantMsg)
				}
			}
		})
	}
}
