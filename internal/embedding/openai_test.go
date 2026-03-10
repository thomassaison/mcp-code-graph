package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [{
				"embedding": [0.1, 0.2, 0.3],
				"index": 0
			}]
		}`))
	}))
	defer server.Close()

	cfg := &Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		BaseURL:  server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	embedding, err := provider.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 3 {
		t.Errorf("len(embedding) = %d, want 3", len(embedding))
	}
}

func TestOpenAIProvider_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"embedding": [0.1, 0.2, 0.3], "index": 0},
				{"embedding": [0.4, 0.5, 0.6], "index": 1}
			]
		}`))
	}))
	defer server.Close()

	cfg := &Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		BaseURL:  server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	embeddings, err := provider.EmbedBatch(context.Background(), []string{"text1", "text2"})
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("len(embeddings) = %d, want 2", len(embeddings))
	}
}
