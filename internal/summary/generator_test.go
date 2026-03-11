package summary

import (
	"context"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
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

func TestGenerator_PassesCodeToProvider(t *testing.T) {
	var capturedReq SummaryRequest
	capture := &captureProvider{fn: func(req SummaryRequest) {
		capturedReq = req
	}}
	gen := NewGenerator(capture, "test-model")

	node := &graph.Node{
		Type:      graph.NodeTypeFunction,
		Package:   "main",
		Name:      "add",
		Signature: "func add(a, b int) int",
		Code:      "func add(a, b int) int { return a + b }",
	}

	if err := gen.Generate(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	if capturedReq.Code != node.Code {
		t.Errorf("SummaryRequest.Code = %q, want %q", capturedReq.Code, node.Code)
	}
}

type captureProvider struct {
	fn func(SummaryRequest)
}

func (c *captureProvider) GenerateSummary(_ context.Context, req SummaryRequest) (string, error) {
	c.fn(req)
	return `{"text":"summary"}`, nil
}

func (c *captureProvider) Generate(_ context.Context, _ string) (string, error) {
	return "", nil
}
