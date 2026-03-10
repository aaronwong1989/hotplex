package diag

import (
	"testing"
)

func TestRedactorRedact(t *testing.T) {
	redactor := NewRedactor(RedactStandard)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API key",
			input:    "api_key=sk-abcdef1234567890abcdef1234567890",
			expected: "[REDACTED_API_KEY]",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: bearer [REDACTED_TOKEN]",
		},
		{
			name:     "Slack token",
			input:    "xoxb-placeholder-token-for-testing",
			expected: "[REDACTED_SLACK_TOKEN]",
		},
		{
			name:     "GitHub token",
			input:    "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "[REDACTED_GITHUB_TOKEN]",
		},
		{
			name:     "Anthropic key",
			input:    "sk-ant-api03-abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz",
			expected: "[REDACTED_ANTHROPIC_KEY]",
		},
		{
			name:     "OpenAI key",
			input:    "sk-ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzAB",
			expected: "[REDACTED_OPENAI_KEY]",
		},
		{
			name:     "Password",
			input:    "password=MySecretPassword123",
			expected: "[REDACTED_SECRET]",
		},
		{
			name:     "AWS Access Key ID",
			input:    "AKIAIOSFODNN7EXAMPLE",
			expected: "[REDACTED_AWS_KEY]",
		},
		{
			name:     "Connection string",
			input:    "postgres://user:password@localhost:5432/db",
			expected: "postgres://[REDACTED_USER]:[REDACTED_PASS]@localhost:5432/db",
		},
		{
			name:     "JWT",
			input:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: "[REDACTED_JWT]",
		},
		{
			name:     "Email",
			input:    "contact@example.com",
			expected: "[REDACTED_EMAIL]",
		},
		{
			name:     "Credit card",
			input:    "1234-5678-9012-3456",
			expected: "[REDACTED_CC]",
		},
		{
			name:     "No sensitive data",
			input:    "Hello, this is a normal message",
			expected: "Hello, this is a normal message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRedactorRedactBytes(t *testing.T) {
	redactor := NewRedactor(RedactStandard)
	input := []byte("xoxb-placeholder-token-for-testing")
	result := redactor.RedactBytes(input)

	expected := "[REDACTED_SLACK_TOKEN]"
	if string(result) != expected {
		t.Errorf("Expected %q, got %q", expected, string(result))
	}
}

func TestRedactorRedactMapValues(t *testing.T) {
	redactor := NewRedactor(RedactStandard)

	t.Run("nil map", func(t *testing.T) {
		result := redactor.RedactMapValues(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("sensitive keys", func(t *testing.T) {
		input := map[string]any{
			"username": "john",
			"password": "secret123",
			"api_key":  "sk-1234567890",
		}
		result := redactor.RedactMapValues(input)

		if result["username"] != "john" {
			t.Errorf("Expected username to be john, got %v", result["username"])
		}
		if result["password"] != "[REDACTED]" {
			t.Errorf("Expected password to be [REDACTED], got %v", result["password"])
		}
		if result["api_key"] != "[REDACTED]" {
			t.Errorf("Expected api_key to be [REDACTED], got %v", result["api_key"])
		}
	})

	t.Run("nested map", func(t *testing.T) {
		input := map[string]any{
			"config": map[string]any{
				"password": "nested_secret",
			},
		}
		result := redactor.RedactMapValues(input)

		nested, ok := result["config"].(map[string]any)
		if !ok {
			t.Fatal("Expected nested map")
		}
		if nested["password"] != "[REDACTED]" {
			t.Errorf("Expected nested password to be [REDACTED], got %v", nested["password"])
		}
	})
}

func TestRedactConvenienceFunctions(t *testing.T) {
	input := "xoxb-placeholder-token-for-testing"

	result := Redact(input)
	if result != "[REDACTED_SLACK_TOKEN]" {
		t.Errorf("Expected [REDACTED_SLACK_TOKEN], got %q", result)
	}

	bytesInput := []byte(input)
	bytesResult := RedactBytes(bytesInput)
	if string(bytesResult) != "[REDACTED_SLACK_TOKEN]" {
		t.Errorf("Expected [REDACTED_SLACK_TOKEN], got %q", string(bytesResult))
	}
}

func TestAggressiveRedactor(t *testing.T) {
	redactor := NewRedactor(RedactAggressive)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Private IP 10.x.x.x",
			input:    "Server at 10.0.0.1",
			expected: "Server at [REDACTED_IP]",
		},
		{
			name:     "Private IP 172.16.x.x",
			input:    "Server at 172.16.0.1",
			expected: "Server at [REDACTED_IP]",
		},
		{
			name:     "Private IP 192.168.x.x",
			input:    "Server at 192.168.1.1",
			expected: "Server at [REDACTED_IP]",
		},
		{
			name:     "Localhost",
			input:    "Connect to localhost:8080",
			expected: "Connect to [REDACTED_HOST]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"password", "passwd", "pwd",
		"secret", "token", "api_key", "apikey",
		"private_key", "privatekey",
		"access_key", "accesskey",
		"auth", "credential", "cred",
	}

	nonSensitive := []string{
		"username", "email", "name",
		"timestamp", "created_at", "id",
	}

	for _, key := range sensitive {
		if !isSensitiveKey(key) {
			t.Errorf("Expected %s to be sensitive", key)
		}
	}

	for _, key := range nonSensitive {
		if isSensitiveKey(key) {
			t.Errorf("Expected %s to not be sensitive", key)
		}
	}
}
