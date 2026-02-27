package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// generateSignature creates a valid Slack signature for testing
func generateSignature(secret, timestamp, body string) string {
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(baseString))
	return "v0=" + hex.EncodeToString(h.Sum(nil))
}

// createTestRequest creates an http.Request with Slack signature headers for testing
func createTestRequest(t *testing.T, timestamp, signature string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	return req
}

func createTestRequestForBench(timestamp, signature string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	return req
}
func createTestAdapter(signingSecret string) *Adapter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAdapter(&Config{
		BotToken:      "xoxb-test-bot-token-123456789012-abcdef",
		SigningSecret: signingSecret,
		Mode:          "http",
	}, logger, base.WithoutServer())
}

// =============================================================================
// Signature Verification Tests (SDK-based)
// =============================================================================

// TestVerifySignature_SDK verifies the SDK's SecretsVerifier works correctly
func TestVerifySignature_SDK(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := []byte(`{"type":"event_callback","challenge":"test-challenge"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	signature := generateSignature(signingSecret, timestamp, string(body))

	adapter := createTestAdapter(signingSecret)
	req := createTestRequest(t, timestamp, signature, body)
	result := adapter.verifySignature(req, body)

	if !result {
		t.Error("Expected valid signature to pass verification")
	}
}

// TestVerifySignature_SDK_Expired verifies expired timestamps are rejected
func TestVerifySignature_SDK_Expired(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := []byte(`{"type":"event_callback"}`)
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	signature := generateSignature(signingSecret, oldTimestamp, string(body))

	adapter := createTestAdapter(signingSecret)
	req := createTestRequest(t, oldTimestamp, signature, body)
	result := adapter.verifySignature(req, body)

	if result {
		t.Error("Expected expired timestamp to fail verification")
	}
}

// TestVerifySignature_SDK_WrongSecret verifies wrong secret fails
func TestVerifySignature_SDK_WrongSecret(t *testing.T) {
	correctSecret := "correct-signing-secret-123456789012345"
	wrongSecret := "wrong-signing-secret-123456789012345"
	body := []byte(`{"type":"event_callback"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	signature := generateSignature(wrongSecret, timestamp, string(body))

	adapter := createTestAdapter(correctSecret)
	req := createTestRequest(t, timestamp, signature, body)
	result := adapter.verifySignature(req, body)

	if result {
		t.Error("Expected wrong secret to fail verification")
	}
}

// BenchmarkVerifySignature_SDK benchmarks the SDK signature verification
func BenchmarkVerifySignature_SDK(b *testing.B) {
	signingSecret := "test-signing-secret-123456789012345"
	body := []byte(`{"type":"event_callback","challenge":"test-challenge"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(signingSecret, timestamp, string(body))

	adapter := createTestAdapter(signingSecret)
	req := createTestRequestForBench(timestamp, signature, body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.verifySignature(req, body)
	}
}

// =============================================================================
// HTTP Handler Tests
// =============================================================================

// errorReader is a reader that returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// =============================================================================
// handleSlashCommand Tests
// =============================================================================

// TestHandleSlashCommand_MethodNotAllowed tests that GET requests return 405
func TestHandleSlashCommand_MethodNotAllowed(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodGet, "/webhook/slack", nil)
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestHandleSlashCommand_ClearCommand tests /clear command processing
func TestHandleSlashCommand_ClearCommand(t *testing.T) {
	adapter := createTestAdapter("")

	// The slash command handler processes in a goroutine, so we just verify
	// that the request is parsed correctly and returns 200
	body := "command=/clear&text=&user_id=U123&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleSlashCommand_ParseFormError tests handling of malformed form data
func TestHandleSlashCommand_ParseFormError(t *testing.T) {
	adapter := createTestAdapter("")

	// Create request with invalid form data
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader("invalid=data%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	// Should return 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestHandleSlashCommand_UnknownCommand tests handling of unknown commands
func TestHandleSlashCommand_UnknownCommand(t *testing.T) {
	adapter := createTestAdapter("")

	body := "command=/unknown&text=&user_id=U123&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	// Should still return 200 (acks immediately)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleSlashCommand_EngineNotSet tests behavior when engine is nil
func TestHandleSlashCommand_EngineNotSet(t *testing.T) {
	adapter := createTestAdapter("")
	// Don't set engine - adapter.eng is nil

	body := "command=/clear&text=&user_id=U123&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	// Should still return 200 (immediate ack)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleSlashCommand_WithText tests slash command with text argument
func TestHandleSlashCommand_WithText(t *testing.T) {
	adapter := createTestAdapter("")

	body := "command=/clear&text=all&user_id=U456&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// =============================================================================
// handleEvent Tests
// =============================================================================

// TestHandleEvent_ChallengeResponse tests URL verification challenge response
func TestHandleEvent_ChallengeResponse(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"url_verification","challenge":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Response is plain text, not JSON
	if w.Body.String() != "abc123" {
		t.Errorf("Expected challenge 'abc123', got %s", w.Body.String())
	}

	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
	}
}

// TestHandleEvent_SignatureVerification tests invalid signature rejection
func TestHandleEvent_SignatureVerification(t *testing.T) {
	signingSecret := "test-signing-secret"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message"}}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", "invalid")
	req.Header.Set("X-Slack-Request-Timestamp", "12345")
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestHandleEvent_MissingSignatureHeaders tests behavior when signature headers are missing
func TestHandleEvent_MissingSignatureHeaders(t *testing.T) {
	signingSecret := "test-signing-secret"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message"}}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	// No signature headers
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestHandleEvent_ValidSignatureWithToken tests valid signature with bot token
func TestHandleEvent_ValidSignatureWithToken(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message","channel":"C123","user":"U123","text":"hello"},"token":"xoxb-test-bot-token-123456789012-abcdef"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(signingSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleEvent_InvalidToken tests rejection of invalid token
func TestHandleEvent_InvalidToken(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"event_callback","event":{"type":"message"},"token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestHandleEvent_InvalidJSON tests handling of malformed JSON
func TestHandleEvent_InvalidJSON(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestHandleEvent_MethodNotAllowed tests that non-POST requests return 405
func TestHandleEvent_MethodNotAllowed(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestHandleEvent_ReadBodyError tests handling of body read errors
func TestHandleEvent_ReadBodyError(t *testing.T) {
	adapter := createTestAdapter("")

	// Create a request with a reader that returns error
	req := httptest.NewRequest(http.MethodPost, "/events", &errorReader{err: fmt.Errorf("read error")})
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestHandleEvent_NoSignatureForURLVerification tests that URL verification requires signature when secret is set
func TestHandleEvent_NoSignatureForURLVerification(t *testing.T) {
	adapter := createTestAdapter("") // No signing secret

	// URL verification should work without signature when no secret is set
	body := `{"type":"url_verification","challenge":"test-challenge"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Response is plain text, not JSON
	if w.Body.String() != "test-challenge" {
		t.Errorf("Expected challenge 'test-challenge', got %s", w.Body.String())
	}
}

// =============================================================================
// handleInteractive Tests
// =============================================================================

// TestHandleInteractive_MethodNotAllowed tests that non-POST requests return 405
func TestHandleInteractive_MethodNotAllowed(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodGet, "/interactive", nil)
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestHandleInteractive_ValidRequest tests valid interactive endpoint request
func TestHandleInteractive_ValidRequest(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"block_actions"}`
	req := httptest.NewRequest(http.MethodPost, "/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleInteractive_ReadBodyError tests handling of body read errors
func TestHandleInteractive_ReadBodyError(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodPost, "/interactive", &errorReader{err: fmt.Errorf("read error")})
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegration_SlashCommandToHandlerFlow tests full flow from slash command to handler
func TestIntegration_SlashCommandToHandlerFlow(t *testing.T) {
	adapter := createTestAdapter("")

	// Verify the handler processes the request correctly
	body := "command=/clear&text=all&user_id=U456&channel_id=C123&response_url=https://hooks.slack.com/commands/123"
	req := httptest.NewRequest(http.MethodPost, "/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Call handler
	adapter.handleSlashCommand(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestIntegration_EventWithSignatureFlow tests full flow with signature verification
func TestIntegration_EventWithSignatureFlow(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message","channel":"C123","channel_type":"dm","user":"U123","text":"test message","ts":"1234567890.123456"},"token":"xoxb-test-bot-token-123456789012-abcdef"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(signingSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestIntegration_ExpiredSignatureFlow tests that expired signatures are rejected
func TestIntegration_ExpiredSignatureFlow(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message"}}`
	// Timestamp from 10 minutes ago
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	signature := generateSignature(signingSecret, oldTimestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", oldTimestamp)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for expired signature, got %d", w.Code)
	}
}

// TestIntegration_NoSignatureForVerification tests URL verification works without signature when no secret set
func TestIntegration_NoSignatureForVerification(t *testing.T) {
	adapter := createTestAdapter("") // No signing secret

	// URL verification doesn't require signature when no secret is configured
	body := `{"type":"url_verification","challenge":"verify-token"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "verify-token" {
		t.Errorf("Expected challenge verify-token, got %s", w.Body.String())
	}
}

// TestIntegration_InteractiveEndpoint tests full interactive endpoint flow
func TestIntegration_InteractiveEndpoint(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"interactive_message","callback_id":"test_callback"}`
	req := httptest.NewRequest(http.MethodPost, "/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestConvertHashPrefixToSlash tests the #<command> to /<command> conversion
func TestConvertHashPrefixToSlash(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedText string
		expectedOk   bool
	}{
		// Supported commands
		{"reset command", "#reset", "/reset", true},
		{"reset with text", "#reset hello", "/reset hello", true},
		{"dc command", "#dc", "/dc", true},
		{"dc with text", "#dc reason", "/dc reason", true},
		// Not supported commands
		{"unknown command", "#unknown", "#unknown", false},
		{"partial match reset", "#resetx", "#resetx", false},
		{"partial match dco", "#dco", "#dco", false},
		// Not commands
		{"no hash prefix", "reset", "reset", false},
		{"normal message", "hello world", "hello world", false},
		{"empty string", "", "", false},
		{"hash only", "#", "#", false},
		{"hash with space", "# ", "# ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := convertHashPrefixToSlash(tt.input)
			if result != tt.expectedText {
				t.Errorf("convertHashPrefixToSlash(%q) = %q, want %q", tt.input, result, tt.expectedText)
			}
			if ok != tt.expectedOk {
				t.Errorf("convertHashPrefixToSlash(%q) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}
		})
	}
}

// TestIsSupportedCommand tests command validation
func TestIsSupportedCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		// Supported commands
		{"reset", "/reset", true},
		{"dc", "/dc", true},
		// Not supported commands
		{"unknown", "/unknown", false},
		{"empty", "", false},
		{"no slash", "reset", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSupportedCommand(tt.command)
			if result != tt.expected {
				t.Errorf("isSupportedCommand(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}
