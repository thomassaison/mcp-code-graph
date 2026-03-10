package embedding

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrUnknownProvider = errors.New("unknown embedding provider")

// ParseConfig parses an embedding provider configuration from a JSON string.
// Returns nil, nil if s is empty or if no provider is specified, indicating
// that embedding is not configured (graceful fallback to name-based search).
// Returns an error if the JSON is malformed.
func ParseConfig(s string) (*Config, error) {
	if s == "" {
		return nil, nil
	}

	var cfg Config
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return nil, fmt.Errorf("parsing embedding config: %w", err)
	}

	if cfg.Provider == "" {
		return nil, nil
	}

	return &cfg, nil
}

// NewProviderFromConfig creates an embedding provider from the given configuration.
// Returns nil, nil if cfg is nil or has no provider specified, indicating that
// embedding is not configured (graceful fallback to name-based search).
// Returns ErrUnknownProvider if the provider type is not recognized.
func NewProviderFromConfig(cfg *Config) (EmbeddingProvider, error) {
	if cfg == nil || cfg.Provider == "" {
		return nil, nil
	}

	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, cfg.Provider)
	}
}
