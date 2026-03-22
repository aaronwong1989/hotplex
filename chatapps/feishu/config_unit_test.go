package feishu

import (
	"testing"
)

func TestConfig_Validate_Defaults(t *testing.T) {
	cfg := &Config{
		AppID:             "app123",
		AppSecret:         "secret",
		VerificationToken: "token",
		EncryptKey:        "encrypt",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
	if cfg.ServerAddr != ":8082" {
		t.Errorf("default server addr should be :8082, got %s", cfg.ServerAddr)
	}
	if cfg.MaxMessageLen != 4096 {
		t.Errorf("default max message len should be 4096, got %d", cfg.MaxMessageLen)
	}
}

func TestConfig_Validate_CustomValues(t *testing.T) {
	cfg := &Config{
		AppID:             "app123",
		AppSecret:         "secret",
		VerificationToken: "token",
		EncryptKey:        "encrypt",
		ServerAddr:        ":9090",
		MaxMessageLen:     8000,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerAddr != ":9090" {
		t.Errorf("custom server addr should be preserved, got %s", cfg.ServerAddr)
	}
	if cfg.MaxMessageLen != 8000 {
		t.Errorf("custom max message len should be preserved, got %d", cfg.MaxMessageLen)
	}
}

func TestConfig_Validate_MissingEachField(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{"missing app_id", &Config{AppSecret: "s", VerificationToken: "t", EncryptKey: "e"}, true},
		{"missing app_secret", &Config{AppID: "a", VerificationToken: "t", EncryptKey: "e"}, true},
		{"missing verification_token", &Config{AppID: "a", AppSecret: "s", EncryptKey: "e"}, true},
		{"missing encrypt_key", &Config{AppID: "a", AppSecret: "s", VerificationToken: "t"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%s) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestConfig_UseWebSocket(t *testing.T) {
	cfg := &Config{
		AppID:             "app123",
		AppSecret:         "secret",
		VerificationToken: "token",
		EncryptKey:        "encrypt",
		UseWebSocket:      true,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.UseWebSocket {
		t.Error("UseWebSocket should be preserved")
	}
}
