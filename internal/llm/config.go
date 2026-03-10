package llm

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrUnknownProvider = errors.New("unknown LLM provider")

// ParseConfig parses an LLM provider configuration from a JSON string.
// Returns nil, nil if s is empty or if no provider is specified, indicating
// that LLM is not configured (graceful fallback to MockProvider).
// Returns an error if the JSON is malformed.
func ParseConfig(s string) (*Config, error) {
	if s == "" {
		return nil, nil
	}

	var cfg Config
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return nil, fmt.Errorf("parsing LLM config: %w", err)
	}

	if cfg.Provider == "" {
		return nil, nil
	}

	return &cfg, nil
}

// NewProviderFromConfig creates an LLM provider from the given configuration.
// Returns MockProvider if cfg is nil or has no provider specified, indicating that
// LLM is not configured (graceful fallback).
// Returns ErrUnknownProvider if the provider type is not recognized.
func NewProviderFromConfig(cfg *Config) (LLMProvider, error) {
	if cfg == nil || cfg.Provider == "" {
		return &MockProvider{}, nil
	}

	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, cfg.Provider)
	}
}
