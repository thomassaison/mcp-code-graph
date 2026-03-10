package mcp

import (
	"testing"

	"github.com/thomas-saison/mcp-code-graph/internal/embedding"
)

func TestServer_WithEmbedding(t *testing.T) {
	cfg := &Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: t.TempDir(),
		Embedding: &embedding.Config{
			Provider: "openai",
			APIKey:   "test-key",
			Model:    "text-embedding-3-small",
		},
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if server.embedding == nil {
		t.Error("Server.embedding is nil, expected provider")
	}
}

func TestServer_WithoutEmbedding(t *testing.T) {
	cfg := &Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: t.TempDir(),
		Embedding:   nil,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if server.embedding != nil {
		t.Error("Server.embedding should be nil when not configured")
	}
}
