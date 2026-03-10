package embedding

import "context"

// EmbeddingProvider generates vector embeddings from text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Config holds configuration for an embedding provider.
type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
}

// MockProvider is a mock embedding provider for testing.
type MockProvider struct {
	Dimensions int
}

func (m *MockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.Dimensions)
	return vec, nil
}

func (m *MockProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, m.Dimensions)
		result[i] = vec
	}
	return result, nil
}
