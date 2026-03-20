package feishu

import (
	"encoding/json"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

func TestEncodeDecodeActionValueWithContext(t *testing.T) {
	value, err := EncodeActionValueWithContext("perm_allow_once", "sess123", "msg456", "Bash", "rm -rf /")
	if err != nil {
		t.Fatalf("EncodeActionValueWithContext() error = %v", err)
	}

	decoded, err := DecodeActionValueWithContext(value)
	if err != nil {
		t.Fatalf("DecodeActionValueWithContext() error = %v", err)
	}
	if decoded.Action != "perm_allow_once" {
		t.Errorf("Action = %q, want perm_allow_once", decoded.Action)
	}
	if decoded.SessionID != "sess123" {
		t.Errorf("SessionID = %q, want sess123", decoded.SessionID)
	}
	if decoded.MessageID != "msg456" {
		t.Errorf("MessageID = %q, want msg456", decoded.MessageID)
	}
	if decoded.Tool != "Bash" {
		t.Errorf("Tool = %q, want Bash", decoded.Tool)
	}
	if decoded.Command != "rm -rf /" {
		t.Errorf("Command = %q, want rm -rf /", decoded.Command)
	}
}

func TestBuildFeishuPermissionCard(t *testing.T) {
	data := base.PermissionCardData{
		BotID:     "ou_abc",
		SessionID: "sess123",
		MessageID: "msg456",
		UserID:    "user789",
		Tool:      "Bash",
		Command:   "rm -rf /tmp",
	}

	card := BuildPermissionCard(data)
	if card == nil {
		t.Fatal("BuildPermissionCard returned nil")
	}
	if card.Header.Template != CardTemplateOrange {
		t.Errorf("Header.Template = %q, want orange", card.Header.Template)
	}
	if len(card.Elements) == 0 {
		t.Error("expected non-empty elements")
	}

	// Verify it's serializable
	data2, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal card failed: %v", err)
	}
	if len(data2) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestBuildPermissionResultCard(t *testing.T) {
	tests := []string{"allow", "allow_once", "allow_always", "deny", "deny_once", "deny_all", "unknown"}
	for _, decision := range tests {
		t.Run(decision, func(t *testing.T) {
			card := BuildPermissionResultCard(decision, "Bash", "echo hello")
			if card == nil {
				t.Errorf("BuildPermissionResultCard(%q) returned nil", decision)
			}
		})
	}
}

func TestBuildPermissionDeniedCard(t *testing.T) {
	card := BuildPermissionDeniedCard("Bash", "chmod 777 /etc", "用户拒绝")
	if card == nil {
		t.Fatal("BuildPermissionDeniedCard returned nil")
	}
	if card.Header.Template != CardTemplateOrange {
		t.Errorf("Header.Template = %q, want orange", card.Header.Template)
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal denied card failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}
