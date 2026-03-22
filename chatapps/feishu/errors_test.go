package feishu

import (
	"strings"
	"testing"
)

func TestFeishuErrors(t *testing.T) {
	errs := []struct {
		name string
		err  error
		want string
	}{
		{"missing_app_id", ErrConfigMissingAppID, "app_id"},
		{"missing_app_secret", ErrConfigMissingAppSecret, "app_secret"},
		{"missing_verification_token", ErrConfigMissingVerificationToken, "verification_token"},
		{"missing_encrypt_key", ErrConfigMissingEncryptKey, "encrypt_key"},
		{"invalid_signature", ErrInvalidSignature, "invalid signature"},
		{"invalid_challenge", ErrInvalidChallenge, "invalid challenge"},
		{"event_parse_failed", ErrEventParseFailed, "failed to parse event"},
		{"unsupported_event", ErrUnsupportedEventType, "unsupported event"},
		{"token_fetch_failed", ErrTokenFetchFailed, "failed to fetch"},
		{"message_send_failed", ErrMessageSendFailed, "failed to send"},
		{"message_update_failed", ErrMessageUpdateFailed, "failed to update"},
		{"message_delete_failed", ErrMessageDeleteFailed, "failed to delete"},
	}

	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error should not be nil")
			}
			if !strings.Contains(tt.err.Error(), tt.want) {
				t.Errorf("error message should contain %q, got %q", tt.want, tt.err.Error())
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{Code: 9999, Msg: "test error"}
	s := err.Error()
	if !strings.Contains(s, "9999") {
		t.Errorf("should contain code 9999, got %q", s)
	}
	if !strings.Contains(s, "test error") {
		t.Errorf("should contain message, got %q", s)
	}
	if !strings.Contains(s, "feishu API error") {
		t.Errorf("should contain prefix, got %q", s)
	}
}

func TestAPIError_ZeroValues(t *testing.T) {
	err := &APIError{}
	s := err.Error()
	if !strings.Contains(s, "code=0") {
		t.Errorf("should handle zero code, got %q", s)
	}
}
