package summary

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

type Generator struct {
	provider LLMProvider
	model    string
}

func NewGenerator(provider LLMProvider, model string) *Generator {
	return &Generator{
		provider: provider,
		model:    model,
	}
}

func (g *Generator) Generate(ctx context.Context, node *graph.Node) error {
	if node.Summary != nil && node.Summary.GeneratedBy == "human" {
		return nil
	}

	slog.Debug("generating summary", "function", node.Name, "package", node.Package)

	req := SummaryRequest{
		FunctionName: node.Name,
		Signature:    node.Signature,
		Package:      node.Package,
		Docstring:    node.Docstring,
		File:         node.File,
		Language:     "Go",
		Code:         node.Code,
	}

	summary, err := g.provider.GenerateSummary(ctx, req)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	slog.Debug("summary generated", "function", node.Name)

	node.Summary = &graph.Summary{
		Text:        summary,
		GeneratedBy: "llm",
		Model:       g.model,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	return nil
}

func (g *Generator) GenerateAll(ctx context.Context, gr *graph.Graph) error {
	functions := gr.GetNodesByType(graph.NodeTypeFunction)
	for _, fn := range functions {
		slog.Log(ctx, debug.LevelTrace, "processing function for summary", "function", fn.Name)
		if err := g.Generate(ctx, fn); err != nil {
			continue
		}
		if fn.Summary != nil {
			gr.SetNodeSummary(fn.ID, fn.Summary) //nolint:errcheck
		}
	}
	return nil
}
