package base

import (
	"sync"
	"testing"
	"time"
)

func TestPermissionApprovalRegistry_RegisterResolve(t *testing.T) {
	r := &PermissionApprovalRegistry{}

	// Register a permission
	ch := r.RegisterPermission("session-1")
	if ch == nil {
		t.Fatal("RegisterPermission returned nil channel")
	}

	// Resolve it
	decision := PermissionDecision{Allow: true, Reason: "allow_once"}
	ok := r.ResolvePermission("session-1", decision)
	if !ok {
		t.Error("ResolvePermission returned false, want true")
	}

	// Channel should receive the decision
	select {
	case d := <-ch:
		if !d.Allow || d.Reason != "allow_once" {
			t.Errorf("received %+v, want Allow=true, Reason=allow_once", d)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for decision on channel")
	}

	// Second resolve should return false (already resolved)
	ok = r.ResolvePermission("session-1", decision)
	if ok {
		t.Error("ResolvePermission returned true for resolved session, want false")
	}
}

func TestPermissionApprovalRegistry_Cancel(t *testing.T) {
	r := &PermissionApprovalRegistry{}

	ch := r.RegisterPermission("session-1")
	r.CancelPermission("session-1")

	// Channel should not receive anything
	select {
	case <-ch:
		t.Error("received on cancelled channel, want no value")
	case <-time.After(50 * time.Millisecond):
		// Expected: timeout, no value received
	}
}

func TestPermissionApprovalRegistry_ResolveUnknown(t *testing.T) {
	r := &PermissionApprovalRegistry{}

	// Resolve non-existent session
	decision := PermissionDecision{Allow: false, Reason: "deny_all"}
	ok := r.ResolvePermission("nonexistent-session", decision)
	if ok {
		t.Error("ResolvePermission returned true for unknown session, want false")
	}
}

func TestPermissionApprovalRegistry_Concurrent(t *testing.T) {
	r := &PermissionApprovalRegistry{}

	var wg sync.WaitGroup
	for i := range 10 {
		sid := "session-" + string(rune('0'+i))
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			ch := r.RegisterPermission(id)
			r.ResolvePermission(id, PermissionDecision{Allow: true, Reason: "test"})
			<-ch
		}(sid)
	}
	wg.Wait()
}

func TestGlobalPermissionContext_StoreLoad(t *testing.T) {
	// Clear any existing state
	GlobalPermissionContext = sync.Map{}

	StorePermissionContext("action-1", "Bash:rm -rf /")
	val, ok := LoadPermissionContext("action-1")
	if !ok {
		t.Fatal("LoadPermissionContext returned false, want true")
	}
	if val != "Bash:rm -rf /" {
		t.Errorf("LoadPermissionContext = %q, want %q", val, "Bash:rm -rf /")
	}

	// Second load should return false (deleted on first load)
	_, ok = LoadPermissionContext("action-1")
	if ok {
		t.Error("second LoadPermissionContext returned true, want false (deleted)")
	}
}

func TestGlobalPermissionContext_LegacyString(t *testing.T) {
	// Ensure backwards compatibility with plain string stored in map
	GlobalPermissionContext = sync.Map{}
	GlobalPermissionContext.Store("legacy-action", "Bash:wget")

	val, ok := LoadPermissionContext("legacy-action")
	if !ok {
		t.Fatal("LoadPermissionContext returned false for legacy string entry")
	}
	if val != "Bash:wget" {
		t.Errorf("LoadPermissionContext = %q, want legacy string", val)
	}
}

func TestPermissionCardData(t *testing.T) {
	d := PermissionCardData{
		BotID:     "BOT123",
		SessionID: "session-1",
		MessageID: "msg-abc",
		UserID:    "user-XYZ",
		Tool:      "Bash",
		Command:   "rm -rf /",
	}

	if d.BotID != "BOT123" {
		t.Errorf("BotID = %q, want BOT123", d.BotID)
	}
	if d.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want session-1", d.SessionID)
	}
	if d.MessageID != "msg-abc" {
		t.Errorf("MessageID = %q, want msg-abc", d.MessageID)
	}
	if d.UserID != "user-XYZ" {
		t.Errorf("UserID = %q, want user-XYZ", d.UserID)
	}
	if d.Tool != "Bash" {
		t.Errorf("Tool = %q, want Bash", d.Tool)
	}
	if d.Command != "rm -rf /" {
		t.Errorf("Command = %q, want rm -rf /", d.Command)
	}
}

func TestDangerApprovalRegistry_RegisterResolve(t *testing.T) {
	r := &DangerApprovalRegistry{}

	ch := r.Register("session-1")
	if ch == nil {
		t.Fatal("Register returned nil channel")
	}

	// Resolve approved
	ok := r.Resolve("session-1", true)
	if !ok {
		t.Error("Resolve returned false, want true")
	}

	select {
	case approved := <-ch:
		if !approved {
			t.Errorf("received %v, want true", approved)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for approval")
	}

	// Resolve denied
	ch2 := r.Register("session-2")
	ok = r.Resolve("session-2", false)
	if !ok {
		t.Error("Resolve returned false for session-2")
	}

	select {
	case approved := <-ch2:
		if approved {
			t.Errorf("received %v, want false", approved)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for denial")
	}
}

func TestDangerApprovalRegistry_Cancel(t *testing.T) {
	r := &DangerApprovalRegistry{}

	ch := r.Register("session-cancel")
	r.Cancel("session-cancel")

	select {
	case <-ch:
		t.Error("received on cancelled channel, want no value")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestDangerApprovalRegistry_ResolveUnknown(t *testing.T) {
	r := &DangerApprovalRegistry{}

	ok := r.Resolve("unknown-session", true)
	if ok {
		t.Error("Resolve returned true for unknown session")
	}
}
