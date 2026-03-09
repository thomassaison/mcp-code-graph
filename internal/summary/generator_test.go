package summary

import (
	"context"
	"testing"

	"github.com/thomas-saison/mcp-code-graph/internal/graph"
)

func TestGenerator(t *testing.T) {
	provider := &MockProvider{}
	gen := NewGenerator(provider, "test-model")

	node := &graph.Node{
		Type:      graph.NodeTypeFunction,
		Package:   "main",
		Name:      "test",
		Signature: "func test() string",
	}

	if err := gen.Generate(context.Background(), node); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if node.Summary == nil {
		t.Fatal("Summary not set")
	}

	if node.Summary.GeneratedBy != "llm" {
		t.Errorf("GeneratedBy = %q, want %q", node.Summary.GeneratedBy, "llm")
	}
}
