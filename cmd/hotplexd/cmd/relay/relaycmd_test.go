package relaycmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setTempHome sets HOME to a temp directory for test isolation.
func setTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

// captureStdout replaces os.Stdout with a pipe, executes fn, and returns
// captured output. Restores os.Stdout after fn completes.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.String()
}

// newRootCmd creates a fresh relay command tree for isolated testing.
// This avoids flag state leaking between tests.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "relay"}

	listCmd := &cobra.Command{
		Use:  "list_bindings",
		Args: cobra.NoArgs,
		RunE: runListBindings,
	}

	addCmd := &cobra.Command{
		Use:  "add_binding",
		Args: cobra.NoArgs,
		RunE: runAddBinding,
	}
	addCmd.Flags().String("chat-id", "", "Chat/Channel ID")
	addCmd.Flags().String("platform", "", "Platform name")
	addCmd.Flags().StringToString("bot", nil, "Bot name:URL pair")

	delCmd := &cobra.Command{
		Use:  "del_binding",
		Args: cobra.NoArgs,
		RunE: runDelBinding,
	}
	delCmd.Flags().String("chat-id", "", "Chat/Channel ID")

	testCmd := &cobra.Command{
		Use:  "test_relay",
		Args: cobra.NoArgs,
		RunE: runTestRelay,
	}
	testCmd.Flags().String("to", "", "Target agent name")
	testCmd.Flags().String("content", "ping", "Message content")

	root.AddCommand(listCmd, addCmd, delCmd, testCmd)
	return root
}

// execCmd executes a fresh relay command with given args and returns captured stdout.
func execCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var execErr error
	out := captureStdout(t, func() {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		execErr = cmd.Execute()
	})
	return out, execErr
}

// ---------------------------------------------------------------------------
// list_bindings
// ---------------------------------------------------------------------------

func TestListBindings_Empty(t *testing.T) {
	setTempHome(t)

	out, err := execCmd(t, "list_bindings")
	if err != nil {
		t.Fatalf("list_bindings: %v", err)
	}
	if !strings.Contains(out, "No relay bindings found") {
		t.Errorf("expected 'No relay bindings found', got: %q", out)
	}
}

func TestListBindings_WithBindings(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "--chat-id", "C123", "--platform", "slack", "--bot", "agent-a=http://a:8080")
	if err != nil {
		t.Fatalf("add_binding: %v", err)
	}

	out, err := execCmd(t, "list_bindings")
	if err != nil {
		t.Fatalf("list_bindings: %v", err)
	}

	if !strings.Contains(out, "slack") {
		t.Errorf("expected platform 'slack' in output, got: %q", out)
	}
	if !strings.Contains(out, "C123") {
		t.Errorf("expected chat ID 'C123' in output, got: %q", out)
	}
	if !strings.Contains(out, "agent-a") {
		t.Errorf("expected bot 'agent-a' in output, got: %q", out)
	}
}

func TestListBindings_RejectsArgs(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "list_bindings", "unexpected-arg")
	if err == nil {
		t.Fatal("expected error for unexpected args, got nil")
	}
}

// ---------------------------------------------------------------------------
// add_binding
// ---------------------------------------------------------------------------

func TestAddBinding_Success(t *testing.T) {
	setTempHome(t)

	out, err := execCmd(t, "add_binding", "--chat-id", "C456", "--platform", "telegram", "--bot", "agent-b=http://b:9090")
	if err != nil {
		t.Fatalf("add_binding: %v", err)
	}
	if !strings.Contains(out, "Relay binding added: telegram:C456") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestAddBinding_MultipleBots(t *testing.T) {
	setTempHome(t)

	out, err := execCmd(t,
		"add_binding",
		"--chat-id", "C789",
		"--platform", "discord",
		"--bot", "agent-a=http://a:8080",
		"--bot", "agent-b=http://b:8080",
	)
	if err != nil {
		t.Fatalf("add_binding: %v", err)
	}
	if !strings.Contains(out, "Relay binding added: discord:C789") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestAddBinding_MissingChatID(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "--platform", "slack", "--bot", "a=http://a")
	if err == nil {
		t.Fatal("expected error for missing --chat-id, got nil")
	}
	if !strings.Contains(err.Error(), "--chat-id") {
		t.Errorf("error should mention --chat-id, got: %v", err)
	}
}

func TestAddBinding_MissingPlatform(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "--chat-id", "C123", "--bot", "a=http://a")
	if err == nil {
		t.Fatal("expected error for missing --platform, got nil")
	}
	if !strings.Contains(err.Error(), "--platform") {
		t.Errorf("error should mention --platform, got: %v", err)
	}
}

func TestAddBinding_MissingBot(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "--chat-id", "C123", "--platform", "slack")
	if err == nil {
		t.Fatal("expected error for missing --bot, got nil")
	}
	if !strings.Contains(err.Error(), "--bot") {
		t.Errorf("error should mention --bot, got: %v", err)
	}
}

func TestAddBinding_AllFlagsMissing(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding")
	if err == nil {
		t.Fatal("expected error for missing all flags, got nil")
	}
}

func TestAddBinding_RejectsArgs(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "unexpected-arg")
	if err == nil {
		t.Fatal("expected error for unexpected args, got nil")
	}
}

// ---------------------------------------------------------------------------
// del_binding
// ---------------------------------------------------------------------------

func TestDelBinding_Success(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "--chat-id", "CDEL", "--platform", "slack", "--bot", "a=http://a")
	if err != nil {
		t.Fatalf("add_binding: %v", err)
	}

	out, err := execCmd(t, "del_binding", "--chat-id", "CDEL")
	if err != nil {
		t.Fatalf("del_binding: %v", err)
	}
	if !strings.Contains(out, "Relay binding deleted: CDEL") {
		t.Errorf("unexpected output: %q", out)
	}

	// Verify deletion via list.
	listOut, _ := execCmd(t, "list_bindings")
	if strings.Contains(listOut, "CDEL") {
		t.Errorf("binding should be deleted, but list shows: %q", listOut)
	}
}

func TestDelBinding_NotFound(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "del_binding", "--chat-id", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent binding, got nil")
	}
}

func TestDelBinding_MissingChatID(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "del_binding")
	if err == nil {
		t.Fatal("expected error for missing --chat-id, got nil")
	}
	if !strings.Contains(err.Error(), "--chat-id") {
		t.Errorf("error should mention --chat-id, got: %v", err)
	}
}

func TestDelBinding_RejectsArgs(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "del_binding", "unexpected-arg")
	if err == nil {
		t.Fatal("expected error for unexpected args, got nil")
	}
}

// ---------------------------------------------------------------------------
// test_relay
// ---------------------------------------------------------------------------

func TestTestRelay_MissingTo(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "test_relay")
	if err == nil {
		t.Fatal("expected error for missing --to, got nil")
	}
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --to, got: %v", err)
	}
}

func TestTestRelay_DefaultContent(t *testing.T) {
	setTempHome(t)

	out, err := execCmd(t, "test_relay", "--to", "agent-x")
	if err != nil {
		t.Fatalf("test_relay: %v", err)
	}
	if !strings.Contains(out, "Sending relay to agent-x: ping") {
		t.Errorf("expected default content 'ping', got: %q", out)
	}
}

func TestTestRelay_CustomContent(t *testing.T) {
	setTempHome(t)

	out, err := execCmd(t, "test_relay", "--to", "agent-x", "--content", "hello world")
	if err != nil {
		t.Fatalf("test_relay: %v", err)
	}
	if !strings.Contains(out, "Sending relay to agent-x: hello world") {
		t.Errorf("expected custom content, got: %q", out)
	}
}

func TestTestRelay_AgentNotFound_Warning(t *testing.T) {
	setTempHome(t)

	out, err := execCmd(t, "test_relay", "--to", "nonexistent-agent")
	if err != nil {
		t.Fatalf("test_relay: %v", err)
	}
	if !strings.Contains(out, `Warning: agent "nonexistent-agent" not found`) {
		t.Errorf("expected warning for nonexistent agent, got: %q", out)
	}
}

func TestTestRelay_AgentFound_NoWarning(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "add_binding", "--chat-id", "CFOUND", "--platform", "slack", "--bot", "found-agent=http://a:8080")
	if err != nil {
		t.Fatalf("add_binding: %v", err)
	}

	out, err := execCmd(t, "test_relay", "--to", "found-agent")
	if err != nil {
		t.Fatalf("test_relay: %v", err)
	}
	if strings.Contains(out, "Warning") {
		t.Errorf("should not warn when agent exists, got: %q", out)
	}
}

func TestTestRelay_RejectsArgs(t *testing.T) {
	setTempHome(t)

	_, err := execCmd(t, "test_relay", "unexpected-arg")
	if err == nil {
		t.Fatal("expected error for unexpected args, got nil")
	}
}

// ---------------------------------------------------------------------------
// Parent command
// ---------------------------------------------------------------------------

func TestSessionCmd_HasSubcommands(t *testing.T) {
	subcmds := SessionCmd.Commands()
	expected := map[string]bool{
		"list_bindings": false,
		"add_binding":   false,
		"del_binding":   false,
		"test_relay":    false,
	}

	for _, cmd := range subcmds {
		if _, ok := expected[cmd.Name()]; ok {
			expected[cmd.Name()] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing subcommand %q", name)
		}
	}
}
