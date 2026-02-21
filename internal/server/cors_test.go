package server

import (
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewCORSConfig_Defaults(t *testing.T) {
	// Clear environment
	_ = os.Unsetenv(EnvAllowedOrigins)
	_ = os.Unsetenv(EnvAPIKey)
	_ = os.Unsetenv(EnvAPIKeys)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	origins := config.ListOrigins()
	if len(origins) != len(DefaultAllowedOrigins) {
		t.Errorf("expected %d default origins, got %d", len(DefaultAllowedOrigins), len(origins))
	}

	if config.IsAPIKeyEnabled() {
		t.Error("API key should be disabled by default")
	}
}

func TestNewCORSConfig_CustomOrigins(t *testing.T) {
	_ = os.Setenv(EnvAllowedOrigins, "https://example.com,https://api.example.com")
	defer func() { _ = os.Unsetenv(EnvAllowedOrigins) }()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	origins := config.ListOrigins()
	if len(origins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(origins))
	}

	// Verify custom origins are present
	originMap := make(map[string]bool)
	for _, o := range origins {
		originMap[o] = true
	}
	if !originMap["https://example.com"] {
		t.Error("missing https://example.com")
	}
	if !originMap["https://api.example.com"] {
		t.Error("missing https://api.example.com")
	}
}

func TestNewCORSConfig_SingleAPIKey(t *testing.T) {
	_ = os.Setenv(EnvAPIKey, "test-secret-key")
	defer func() { _ = os.Unsetenv(EnvAPIKey) }()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	if !config.IsAPIKeyEnabled() {
		t.Error("API key should be enabled")
	}
}

func TestNewCORSConfig_MultipleAPIKeys(t *testing.T) {
	_ = os.Setenv(EnvAPIKeys, "key1,key2,key3")
	defer func() { _ = os.Unsetenv(EnvAPIKeys) }()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	if !config.IsAPIKeyEnabled() {
		t.Error("API key should be enabled")
	}
}

func TestCheckOrigin_ValidOrigin(t *testing.T) {
	_ = os.Unsetenv(EnvAPIKey)
	_ = os.Unsetenv(EnvAPIKeys)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	if !checkOrigin(req) {
		t.Error("valid origin should be allowed")
	}
}

func TestCheckOrigin_InvalidOrigin(t *testing.T) {
	_ = os.Unsetenv(EnvAPIKey)
	_ = os.Unsetenv(EnvAPIKeys)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://malicious-site.com")

	if checkOrigin(req) {
		t.Error("invalid origin should be rejected")
	}
}

func TestCheckOrigin_NoOrigin(t *testing.T) {
	_ = os.Unsetenv(EnvAPIKey)
	_ = os.Unsetenv(EnvAPIKeys)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws", nil)
	// No Origin header (non-browser client)

	if !checkOrigin(req) {
		t.Error("requests without Origin should be allowed for non-browser clients")
	}
}

func TestCheckOrigin_APIKeyInHeader(t *testing.T) {
	_ = os.Setenv(EnvAPIKey, "secret-key-123")
	defer func() { _ = os.Unsetenv(EnvAPIKey) }()
	_ = os.Unsetenv(EnvAllowedOrigins)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("X-API-Key", "secret-key-123")
	req.Header.Set("Origin", "http://localhost:3000")

	if !checkOrigin(req) {
		t.Error("valid API key in header should be allowed")
	}
}

func TestCheckOrigin_APIKeyInQuery(t *testing.T) {
	_ = os.Setenv(EnvAPIKey, "secret-key-123")
	defer func() { _ = os.Unsetenv(EnvAPIKey) }()
	_ = os.Unsetenv(EnvAllowedOrigins)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws?api_key=secret-key-123", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	if !checkOrigin(req) {
		t.Error("valid API key in query should be allowed")
	}
}

func TestCheckOrigin_InvalidAPIKey(t *testing.T) {
	_ = os.Setenv(EnvAPIKey, "secret-key-123")
	defer func() { _ = os.Unsetenv(EnvAPIKey) }()
	_ = os.Unsetenv(EnvAllowedOrigins)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	req.Header.Set("Origin", "http://localhost:3000")

	if checkOrigin(req) {
		t.Error("invalid API key should be rejected")
	}
}

func TestCheckOrigin_APIKeyMissing(t *testing.T) {
	_ = os.Setenv(EnvAPIKey, "secret-key-123")
	defer func() { _ = os.Unsetenv(EnvAPIKey) }()
	_ = os.Unsetenv(EnvAllowedOrigins)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)
	checkOrigin := config.CheckOrigin()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	// No API key provided

	if checkOrigin(req) {
		t.Error("missing API key should be rejected when API key is enabled")
	}
}

func TestAddRemoveOrigin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	// Add new origin
	config.AddOrigin("https://new-origin.com")

	origins := config.ListOrigins()
	found := false
	for _, o := range origins {
		if o == "https://new-origin.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("added origin not found")
	}

	// Remove origin
	config.RemoveOrigin("https://new-origin.com")

	origins = config.ListOrigins()
	for _, o := range origins {
		if o == "https://new-origin.com" {
			t.Error("removed origin still present")
			break
		}
	}
}

func TestAddRemoveAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	// Initially disabled
	if config.IsAPIKeyEnabled() {
		t.Error("API key should be disabled initially")
	}

	// Add API key
	config.AddAPIKey("new-secret-key")
	if !config.IsAPIKeyEnabled() {
		t.Error("API key should be enabled after adding")
	}

	// Verify key works
	checkOrigin := config.CheckOrigin()
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("X-API-Key", "new-secret-key")
	req.Header.Set("Origin", "http://localhost:3000")
	if !checkOrigin(req) {
		t.Error("newly added API key should work")
	}

	// Remove API key
	config.RemoveAPIKey("new-secret-key")
	if config.IsAPIKeyEnabled() {
		t.Error("API key should be disabled after removing all keys")
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234****6789"},
		{"very-long-api-key-12345", "very****2345"},
	}

	for _, tt := range tests {
		result := maskAPIKey(tt.input)
		if result != tt.expected {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCORSConfig_Concurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := NewCORSConfig(logger)

	// Run concurrent operations to test race conditions
	done := make(chan bool)

	// Writer goroutine 1
	go func() {
		for i := range 100 {
			config.AddOrigin("https://test" + string(rune(i)) + ".com")
		}
		done <- true
	}()

	// Writer goroutine 2
	go func() {
		for i := range 100 {
			config.AddAPIKey("key" + string(rune(i)))
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for range 100 {
			_ = config.ListOrigins()
			_ = config.IsAPIKeyEnabled()
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}
