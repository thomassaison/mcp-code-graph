package llm

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Config
		wantErr bool
	}{
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "valid config",
			input: `{"provider":"openai","api_key":"test-key","model":"gpt-4o-mini","base_url":"https://api.openai.com/v1"}`,
			want: &Config{
				Provider: "openai",
				APIKey:   "test-key",
				Model:    "gpt-4o-mini",
				BaseURL:  "https://api.openai.com/v1",
			},
		},
		{
			name:  "no provider returns nil",
			input: `{"model":"gpt-4o-mini"}`,
			want:  nil,
		},
		{
			name:    "invalid JSON returns error",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfig(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want == nil {
				if got != nil {
					t.Errorf("ParseConfig() = %v, want nil", got)
				}
				return
			}
			if got.Provider != tt.want.Provider {
				t.Errorf("Provider = %v, want %v", got.Provider, tt.want.Provider)
			}
			if got.APIKey != tt.want.APIKey {
				t.Errorf("APIKey = %v, want %v", got.APIKey, tt.want.APIKey)
			}
			if got.Model != tt.want.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.want.Model)
			}
			if got.BaseURL != tt.want.BaseURL {
				t.Errorf("BaseURL = %v, want %v", got.BaseURL, tt.want.BaseURL)
			}
		})
	}
}

func TestNewProviderFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "nil config returns MockProvider",
			cfg:  nil,
		},
		{
			name: "empty provider returns MockProvider",
			cfg:  &Config{},
		},
		{
			name: "openai provider",
			cfg: &Config{
				Provider: "openai",
				Model:    "gpt-4o-mini",
				BaseURL:  "https://api.openai.com/v1",
			},
		},
		{
			name: "openai without model returns error",
			cfg: &Config{
				Provider: "openai",
			},
			wantErr: true,
		},
		{
			name: "unknown provider returns error",
			cfg: &Config{
				Provider: "unknown",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProviderFromConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProviderFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("provider should not be nil")
			}
		})
	}
}
