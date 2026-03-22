package slack

import (
	"errors"
	"strings"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

// --- SanitizeErrorMessage tests ---

func TestSanitizeErrorMessage_Nil(t *testing.T) {
	if SanitizeErrorMessage(nil) != "" {
		t.Error("nil error should return empty string")
	}
}

func TestSanitizeErrorMessage_Basic(t *testing.T) {
	result := SanitizeErrorMessage(errors.New("something went wrong"))
	if result != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", result)
	}
}

func TestSanitizeErrorMessage_RemovesSecrets(t *testing.T) {
	result := SanitizeErrorMessage(errors.New("api_key=secret123"))
	if strings.Contains(result, "secret123") {
		t.Error("secret should be redacted")
	}
}

func TestSanitizeErrorMessage_RemovesPaths(t *testing.T) {
	result := SanitizeErrorMessage(errors.New("error at /usr/local/file.txt"))
	if strings.Contains(result, "/usr/local/file.txt") {
		t.Error("file path should be redacted")
	}
}

func TestSanitizeErrorMessage_RemovesGoroutines(t *testing.T) {
	msg := "error: something\ngoroutine 1 [running]:\nmain()"
	result := SanitizeErrorMessage(errors.New(msg))
	if strings.Contains(result, "goroutine") {
		t.Error("goroutine info should be removed")
	}
}

func TestSanitizeErrorMessage_Truncates(t *testing.T) {
	longMsg := strings.Repeat("a", 600)
	result := SanitizeErrorMessage(errors.New(longMsg))
	if len(result) > 500 {
		t.Errorf("should be truncated to 500, got %d", len(result))
	}
}

// --- ValidateButtonValue tests ---

func TestValidateButtonValue_Valid(t *testing.T) {
	behavior, sessionID, messageID, err := ValidateButtonValue("allow:session-1:msg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if behavior != "allow" || sessionID != "session-1" || messageID != "msg-1" {
		t.Errorf("unexpected values: %s, %s, %s", behavior, sessionID, messageID)
	}
}

func TestValidateButtonValue_Deny(t *testing.T) {
	behavior, _, _, err := ValidateButtonValue("deny:sess:msg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if behavior != "deny" {
		t.Errorf("expected deny, got %s", behavior)
	}
}

func TestValidateButtonValue_InvalidFormat(t *testing.T) {
	_, _, _, err := ValidateButtonValue("invalid")
	if err == nil {
		t.Error("should fail for invalid format")
	}
}

func TestValidateButtonValue_InvalidBehavior(t *testing.T) {
	_, _, _, err := ValidateButtonValue("delete:sess:msg")
	if err == nil {
		t.Error("should fail for invalid behavior")
	}
}

func TestValidateButtonValue_SessionIDTooLong(t *testing.T) {
	longID := strings.Repeat("a", 256)
	_, _, _, err := ValidateButtonValue("allow:" + longID + ":msg")
	if err == nil {
		t.Error("should fail for too long session ID")
	}
}

func TestValidateButtonValue_MessageIDTooLong(t *testing.T) {
	longID := strings.Repeat("a", 256)
	_, _, _, err := ValidateButtonValue("allow:sess:" + longID)
	if err == nil {
		t.Error("should fail for too long message ID")
	}
}

// --- ValidateURL tests ---

func TestValidateURL_Empty(t *testing.T) {
	err := ValidateURL("")
	if err == nil {
		t.Error("empty URL should fail")
	}
}

func TestValidateURL_ValidHTTP(t *testing.T) {
	err := ValidateURL("https://example.com")
	if err != nil {
		t.Errorf("https URL should be valid: %v", err)
	}
}

func TestValidateURL_ValidFTP(t *testing.T) {
	err := ValidateURL("ftp://files.example.com")
	if err != nil {
		t.Errorf("ftp URL should be valid: %v", err)
	}
}

func TestValidateURL_InvalidScheme(t *testing.T) {
	err := ValidateURL("javascript:alert(1)")
	if err == nil {
		t.Error("javascript: scheme should fail")
	}
}

func TestValidateURL_DataURI(t *testing.T) {
	err := ValidateURL("data:text/html,<script>alert(1)</script>")
	if err == nil {
		t.Error("data: URI should fail")
	}
}

func TestValidateURL_JavascriptInPath(t *testing.T) {
	err := ValidateURL("https://example.com/javascript:alert(1)")
	if err == nil {
		t.Error("javascript: in path should fail")
	}
}

// --- ValidateActionID tests ---

func TestValidateActionID_Valid(t *testing.T) {
	if err := ValidateActionID("action_123"); err != nil {
		t.Errorf("valid action_id should pass: %v", err)
	}
}

func TestValidateActionID_Empty(t *testing.T) {
	if err := ValidateActionID(""); err == nil {
		t.Error("empty action_id should fail")
	}
}

func TestValidateActionID_TooLong(t *testing.T) {
	longID := strings.Repeat("a", 256)
	if err := ValidateActionID(longID); err == nil {
		t.Error("too long action_id should fail")
	}
}

func TestValidateActionID_InvalidChars(t *testing.T) {
	if err := ValidateActionID("action with spaces"); err == nil {
		t.Error("action_id with spaces should fail")
	}
}

// --- ValidateOptionValue tests ---

func TestValidateOptionValue_Valid(t *testing.T) {
	if err := ValidateOptionValue("option1"); err != nil {
		t.Errorf("valid option value should pass: %v", err)
	}
}

func TestValidateOptionValue_Empty(t *testing.T) {
	if err := ValidateOptionValue(""); err == nil {
		t.Error("empty option should fail")
	}
}

func TestValidateOptionValue_TooLong(t *testing.T) {
	if err := ValidateOptionValue(strings.Repeat("a", 76)); err == nil {
		t.Error("too long option should fail")
	}
}

func TestValidateOptionValue_SlackSyntax(t *testing.T) {
	if err := ValidateOptionValue("<!here>"); err == nil {
		t.Error("Slack special syntax should fail")
	}
	if err := ValidateOptionValue("<@U123>"); err == nil {
		t.Error("Slack user mention should fail")
	}
}

// --- SanitizeForDisplay tests ---

func TestSanitizeForDisplay_Basic(t *testing.T) {
	result := SanitizeForDisplay("hello world", 100)
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestSanitizeForDisplay_Truncate(t *testing.T) {
	result := SanitizeForDisplay("hello world", 5)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestSanitizeForDisplay_RemoveNullBytes(t *testing.T) {
	result := SanitizeForDisplay("hello\x00world", 100)
	if result != "helloworld" {
		t.Errorf("expected null bytes removed, got %q", result)
	}
}

func TestSanitizeForDisplay_RemoveControlChars(t *testing.T) {
	result := SanitizeForDisplay("hello\x01world", 100)
	if result != "helloworld" {
		t.Errorf("expected control chars removed, got %q", result)
	}
}

func TestSanitizeForDisplay_KeepNewlines(t *testing.T) {
	result := SanitizeForDisplay("hello\nworld", 100)
	if !strings.Contains(result, "\n") {
		t.Error("newlines should be preserved")
	}
}

// --- ValidateToolName tests ---

func TestValidateToolName_Valid(t *testing.T) {
	result := ValidateToolName("ReadFile")
	if result != "ReadFile" {
		t.Errorf("expected 'ReadFile', got %q", result)
	}
}

func TestValidateToolName_SpecialChars(t *testing.T) {
	result := ValidateToolName("read_file-v2.go")
	if result != "read_file-v2.go" {
		t.Errorf("expected 'read_file-v2.go', got %q", result)
	}
}

func TestValidateToolName_OnlySpecial(t *testing.T) {
	result := ValidateToolName("<<>>")
	// < and > are removed by the filter, leaving empty -> unknown_tool
	if result != "unknown_tool" {
		t.Errorf("expected 'unknown_tool', got %q", result)
	}
}

// --- SanitizeCommand tests ---

func TestSanitizeCommand_Basic(t *testing.T) {
	result := SanitizeCommand("ls -la")
	if result != "ls -la" {
		t.Errorf("expected 'ls -la', got %q", result)
	}
}

func TestSanitizeCommand_RemoveBackticks(t *testing.T) {
	result := SanitizeCommand("echo `hello`")
	if strings.Contains(result, "`") && !strings.Contains(result, "\\`") {
		t.Error("backticks should be escaped")
	}
}

func TestSanitizeCommand_RemoveSlackSyntax(t *testing.T) {
	result := SanitizeCommand("<!here> <@U123> <#C123>")
	if strings.Contains(result, "<!") || strings.Contains(result, "<@") || strings.Contains(result, "<#") {
		t.Error("Slack special syntax should be escaped")
	}
}

func TestSanitizeCommand_Truncate(t *testing.T) {
	longCmd := strings.Repeat("a", 2500)
	result := SanitizeCommand(longCmd)
	if base.RuneCount(result) > 2003 {
		t.Errorf("should be truncated, got %d runes", base.RuneCount(result))
	}
}

// --- ValidateBlockID tests ---

func TestValidateBlockID_Short(t *testing.T) {
	result := ValidateBlockID("block_123")
	if result != "block_123" {
		t.Errorf("expected 'block_123', got %q", result)
	}
}

func TestValidateBlockID_TooLong(t *testing.T) {
	longID := strings.Repeat("a", 300)
	result := ValidateBlockID(longID)
	if len(result) > MaxBlockIDLen {
		t.Errorf("should be truncated to %d, got %d", MaxBlockIDLen, len(result))
	}
}

// --- ValidatePlainText tests ---

func TestValidatePlainText_Short(t *testing.T) {
	result := ValidatePlainText("hello", false)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestValidatePlainText_TooLong(t *testing.T) {
	longText := strings.Repeat("a", 200)
	result := ValidatePlainText(longText, false)
	if base.RuneCount(result) > 153 {
		t.Errorf("should be truncated with ellipsis, got %d runes", base.RuneCount(result))
	}
}

func TestValidatePlainText_WithEmoji(t *testing.T) {
	result := ValidatePlainText("hello", true)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

// --- ValidateMrkdwnText tests ---

func TestValidateMrkdwnText_BalancedCodeBlocks(t *testing.T) {
	result := ValidateMrkdwnText("before\n```\ncode\n```\nafter")
	if !strings.Contains(result, "before") {
		t.Error("should preserve text around code blocks")
	}
}

func TestValidateMrkdwnText_UnbalancedCodeBlocks(t *testing.T) {
	result := ValidateMrkdwnText("```\ncode without close")
	if !strings.HasSuffix(result, "```") {
		t.Error("should add closing code block")
	}
}

func TestValidateMrkdwnText_TooLong(t *testing.T) {
	longText := strings.Repeat("a", 4000)
	result := ValidateMrkdwnText(longText)
	if base.RuneCount(result) > 3003 {
		t.Errorf("should be truncated, got %d runes", base.RuneCount(result))
	}
}

// --- balanceCodeBlocks tests ---

func TestBalanceCodeBlocks_Even(t *testing.T) {
	result := balanceCodeBlocks("```\ncode\n```")
	if result != "```\ncode\n```" {
		t.Errorf("balanced code blocks should be unchanged: %q", result)
	}
}

func TestBalanceCodeBlocks_Odd(t *testing.T) {
	result := balanceCodeBlocks("```\ncode")
	if !strings.HasSuffix(result, "```") {
		t.Errorf("should add closing: %q", result)
	}
}

func TestBalanceCodeBlocks_None(t *testing.T) {
	result := balanceCodeBlocks("no code blocks")
	if result != "no code blocks" {
		t.Errorf("text without code blocks should be unchanged: %q", result)
	}
}

// --- IsAllowedScheme tests ---

func TestIsAllowedScheme(t *testing.T) {
	tests := []struct {
		scheme  string
		allowed bool
	}{
		{"http://", true},
		{"https://", true},
		{"mailto:", true},
		{"ftp://", true},
		{"javascript:", false},
		{"data:", false},
	}
	for _, tt := range tests {
		result := IsAllowedScheme(tt.scheme)
		if result != tt.allowed {
			t.Errorf("IsAllowedScheme(%q) = %v, want %v", tt.scheme, result, tt.allowed)
		}
	}
}

// --- ValidateInitialValue tests ---

func TestValidateInitialValue_Short(t *testing.T) {
	result := ValidateInitialValue("hello", 100)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestValidateInitialValue_TooLong(t *testing.T) {
	long := strings.Repeat("a", 200)
	result := ValidateInitialValue(long, 100)
	if base.RuneCount(result) > 103 {
		t.Errorf("should be truncated, got %d runes", base.RuneCount(result))
	}
}

// --- ValidateConfirmationDialog tests ---

func TestValidateConfirmationDialog_Short(t *testing.T) {
	title, text, confirm, deny := ValidateConfirmationDialog("title", "text", "yes", "no")
	if title != "title" || text != "text" || confirm != "yes" || deny != "no" {
		t.Error("short values should pass through")
	}
}

func TestValidateConfirmationDialog_TooLong(t *testing.T) {
	long := strings.Repeat("a", 100)
	title, _, _, _ := ValidateConfirmationDialog(long, long, long, long)
	if base.RuneCount(title) > 75 {
		t.Errorf("should be truncated to 75, got %d", base.RuneCount(title))
	}
}

// --- ValidateEmoji tests ---

func TestValidateEmoji_Allow(t *testing.T) {
	result := ValidateEmoji("hello", true)
	if result != "hello" {
		t.Errorf("should pass through when allowed: %q", result)
	}
}

func TestValidateEmoji_Remove(t *testing.T) {
	// Test with a simple emoticon range character
	result := ValidateEmoji("hello", false)
	if result != "hello" {
		t.Errorf("text without emoji should pass: %q", result)
	}
}

// --- ValidateRegexPattern tests ---

func TestValidateRegexPattern_Valid(t *testing.T) {
	result, err := ValidateRegexPattern("[a-z]+")
	if err != nil {
		t.Errorf("valid pattern should pass: %v", err)
	}
	if result != "[a-z]+" {
		t.Error("pattern should be returned unchanged")
	}
}

func TestValidateRegexPattern_TooLong(t *testing.T) {
	_, err := ValidateRegexPattern(strings.Repeat("a", 1001))
	if err == nil {
		t.Error("too long pattern should fail")
	}
}

// --- ValidateTokenFormat tests ---

func TestValidateTokenFormat_Valid(t *testing.T) {
	tests := []string{"xoxb-123456-abc", "xoxp-token123", "xoxa-app123"}
	for _, token := range tests {
		if !ValidateTokenFormat(token) {
			t.Errorf("token %q should be valid", token)
		}
	}
}

func TestValidateTokenFormat_Invalid(t *testing.T) {
	tests := []string{"invalid", "short", strings.Repeat("a", 101)}
	for _, token := range tests {
		if ValidateTokenFormat(token) {
			t.Errorf("token %q should be invalid", token)
		}
	}
}

// --- SanitizeForRegex tests ---

func TestSanitizeForRegex(t *testing.T) {
	result := SanitizeForRegex("hello.world")
	if result != "hello\\.world" {
		t.Errorf("expected 'hello\\.world', got %q", result)
	}
}

// --- ValidateEmailFormat tests ---

func TestValidateEmailFormat(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"test.user@domain.co", true},
		{"", false},
		{"no-at-sign", false},
		{"@missing-local.com", false},
		{"user@", false},
	}
	for _, tt := range tests {
		result := ValidateEmailFormat(tt.email)
		if result != tt.valid {
			t.Errorf("ValidateEmailFormat(%q) = %v, want %v", tt.email, result, tt.valid)
		}
	}
}

// --- ValidateURLFormat tests ---

func TestValidateURLFormat(t *testing.T) {
	if !ValidateURLFormat("https://example.com") {
		t.Error("https URL should be valid")
	}
	if !ValidateURLFormat("http://example.com") {
		t.Error("http URL should be valid")
	}
	if ValidateURLFormat("javascript:alert(1)") {
		t.Error("javascript URL should be invalid")
	}
	if ValidateURLFormat("") {
		t.Error("empty URL should be invalid")
	}
}

// --- RateLimitKey tests ---

func TestRateLimitKey_WithIP(t *testing.T) {
	result := RateLimitKey("U123", "1.2.3.4")
	if result != "U123:1.2.3.4" {
		t.Errorf("expected 'U123:1.2.3.4', got %q", result)
	}
}

func TestRateLimitKey_WithoutIP(t *testing.T) {
	result := RateLimitKey("U123", "")
	if result != "U123" {
		t.Errorf("expected 'U123', got %q", result)
	}
}

// --- SanitizeMarkdown tests ---

func TestSanitizeMarkdown_Basic(t *testing.T) {
	result := SanitizeMarkdown("hello world")
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestSanitizeMarkdown_ScriptInjection(t *testing.T) {
	result := SanitizeMarkdown("<script>alert(1)</script>")
	if strings.Contains(result, "<script") {
		t.Error("script tag should be escaped")
	}
}

// --- ValidateMentionFormat tests ---

func TestValidateMentionFormat(t *testing.T) {
	tests := []struct {
		mention string
		valid   bool
	}{
		{"<!here>", true},
		{"<!channel>", true},
		{"<@U123>", true},
		{"<#C123>", true},
		{"<@U123|user>", true},
		{"invalid", false},
		{"<notvalid", false},
	}
	for _, tt := range tests {
		result := ValidateMentionFormat(tt.mention)
		if result != tt.valid {
			t.Errorf("ValidateMentionFormat(%q) = %v, want %v", tt.mention, result, tt.valid)
		}
	}
}
