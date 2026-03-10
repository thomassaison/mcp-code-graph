package embedding

import "encoding/json"

func ParseConfig(s string) (*Config, error) {
	if s == "" {
		return nil, nil
	}

	var cfg Config
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return nil, err
	}

	if cfg.Provider == "" {
		return nil, nil
	}

	return &cfg, nil
}

func NewProviderFromConfig(cfg *Config) (EmbeddingProvider, error) {
	if cfg == nil || cfg.Provider == "" {
		return nil, nil
	}

	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg)
	default:
		return nil, nil
	}
}
