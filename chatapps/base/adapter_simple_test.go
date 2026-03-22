package base

import (
	"context"
	"net/http"
	"testing"
	"time"

	"log/slog"

	"github.com/hrygo/hotplex/chatapps/session"
)

// TestAdapterOptions tests all With* option functions
func TestAdapterOptions(t *testing.T) {
	adapter := &Adapter{
		config: Config{
			SystemPrompt: "default",
		},
		sessions:      make(map[string]*Session),
		httpHandlers:  make(map[string]http.HandlerFunc),
		messageParser: func(body []byte, metadata map[string]any) (*ChatMessage, error) { return nil, nil },
	}

	// Test WithSessionTimeout
	opt := WithSessionTimeout(5 * time.Minute)
	opt(adapter)
	if adapter.sessionTimeout != 5*time.Minute {
		t.Errorf("WithSessionTimeout: got %v, want %v", adapter.sessionTimeout, 5*time.Minute)
	}

	// Test WithSessionTimeout with 0 (should not change)
	opt = WithSessionTimeout(0)
	opt(adapter)
	if adapter.sessionTimeout != 5*time.Minute {
		t.Errorf("WithSessionTimeout(0): should not change, got %v", adapter.sessionTimeout)
	}

	// Test WithCleanupInterval
	opt = WithCleanupInterval(30 * time.Second)
	opt(adapter)
	if adapter.cleanupInterval != 30*time.Second {
		t.Errorf("WithCleanupInterval: got %v, want %v", adapter.cleanupInterval, 30*time.Second)
	}

	// Test WithMetadataExtractor
	extractor := func(update any) map[string]any { return nil }
	opt = WithMetadataExtractor(extractor)
	opt(adapter)
	if adapter.metadataExtract == nil {
		t.Error("WithMetadataExtractor: should not be nil")
	}

	// Test WithMessageParser
	parser := func(body []byte, metadata map[string]any) (*ChatMessage, error) { return nil, nil }
	opt = WithMessageParser(parser)
	opt(adapter)
	if adapter.messageParser == nil {
		t.Error("WithMessageParser: should not be nil")
	}

	// Test WithMessageSender
	sender := func(ctx context.Context, sessionID string, msg *ChatMessage) error { return nil }
	opt = WithMessageSender(sender)
	opt(adapter)
	if adapter.messageSender == nil {
		t.Error("WithMessageSender: should not be nil")
	}

	// Test WithHTTPHandler
	handler := func(w http.ResponseWriter, r *http.Request) {}
	opt = WithHTTPHandler("/test", handler)
	opt(adapter)
	if adapter.httpHandlers["/test"] == nil {
		t.Error("WithHTTPHandler: should add handler")
	}

	// Test WithoutServer
	opt = WithoutServer()
	opt(adapter)
	if !adapter.disableServer {
		t.Error("WithoutServer: should set disableServer to true")
	}
}

// TestAdapterGetterSetters tests all getter and setter methods
func TestAdapterGetterSetters(t *testing.T) {
	logger := slog.Default()
	adapter := &Adapter{
		platformName: "test-platform",
		config: Config{
			SystemPrompt: "test prompt",
		},
		sessions:     make(map[string]*Session),
		httpHandlers: make(map[string]http.HandlerFunc),
		logger:       logger,
	}

	// Test Platform
	if adapter.Platform() != "test-platform" {
		t.Errorf("Platform(): got %s, want test-platform", adapter.Platform())
	}

	// Test SystemPrompt
	if adapter.SystemPrompt() != "test prompt" {
		t.Errorf("SystemPrompt(): got %s, want 'test prompt'", adapter.SystemPrompt())
	}

	// Test SetHandler and Handler
	handler := func(ctx context.Context, msg *ChatMessage) error { return nil }
	adapter.SetHandler(handler)
	if adapter.Handler() == nil {
		t.Error("SetHandler/Handler: handler should not be nil")
	}

	// Test SetMessageStore
	store := &MessageStorePlugin{}
	adapter.SetMessageStore(store)
	if adapter.messageStore != store {
		t.Error("SetMessageStore: store mismatch")
	}

	// Test SetSessionManager
	mgr := session.NewSessionManager("test")
	adapter.SetSessionManager(mgr)
	if adapter.sessionMgr != mgr {
		t.Error("SetSessionManager: manager mismatch")
	}

	// Test SetProviderType
	adapter.SetProviderType("test-provider")
	if adapter.providerType != "test-provider" {
		t.Errorf("SetProviderType: got %s, want test-provider", adapter.providerType)
	}

	// Test Logger
	if adapter.Logger() != logger {
		t.Error("Logger: logger mismatch")
	}

	// Test SetLogger
	newLogger := slog.Default()
	adapter.SetLogger(newLogger)
	if adapter.Logger() != newLogger {
		t.Error("SetLogger: logger not updated")
	}

	// Test WebhookPath (method call, not field access)
	// Note: webhookPath is accessed via WebhookPath() method, not a field
}
