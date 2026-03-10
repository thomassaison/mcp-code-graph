package embedding

import (
	"errors"
	"testing"
)

func TestParseConfig(t *testing.T) {
	jsonConfig := `{"provider":"openai","api_key":"sk-test","model":"text-embedding-3-small","base_url":"http://localhost:11434/v1"}`

	cfg, err := ParseConfig(jsonConfig)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
	if cfg.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-test")
	}
	if cfg.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q, want %q", cfg.Model, "text-embedding-3-small")
	}
	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:11434/v1")
	}
}

func TestParseConfig_Empty(t *testing.T) {
	cfg, err := ParseConfig("")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg != nil {
		t.Errorf("ParseConfig('') = %v, want nil", cfg)
	}
}

func TestNewProviderFromConfig(t *testing.T) {
	cfg := &Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		BaseURL:  "http://localhost:11434/v1",
	}

	provider, err := NewProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewProviderFromConfig() error = %v", err)
	}

	if provider == nil {
		t.Error("NewProviderFromConfig() returned nil")
	}
}

func TestNewProviderFromConfig_Nil(t *testing.T) {
	provider, err := NewProviderFromConfig(nil)
	if err != nil {
		t.Fatalf("NewProviderFromConfig(nil) error = %v", err)
	}
	if provider != nil {
		t.Error("NewProviderFromConfig(nil) should return nil provider")
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	cfg, err := ParseConfig("{invalid json")
	if err == nil {
		t.Error("ParseConfig() expected error for invalid JSON, got nil")
	}
	if cfg != nil {
		t.Errorf("ParseConfig() = %v, want nil", cfg)
	}
}

func TestNewProviderFromConfig_UnknownProvider(t *testing.T) {
	cfg := &Config{
		Provider: "unknown",
		APIKey:   "test-key",
	}

	provider, err := NewProviderFromConfig(cfg)
	if err == nil {
		t.Error("NewProviderFromConfig() expected error for unknown provider, got nil")
	}
	if !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("NewProviderFromConfig() error = %v, want ErrUnknownProvider", err)
	}
	if provider != nil {
		t.Error("NewProviderFromConfig() should return nil provider for unknown provider")
	}
}
