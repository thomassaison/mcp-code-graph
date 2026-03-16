// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/indexer"
	"github.com/thomassaison/mcp-code-graph/internal/summary"
)

// IndexService owns indexing concerns: parsing source files, persisting the graph,
// and generating LLM summaries.
type IndexService struct {
	graph     *graph.Graph
	indexer   *indexer.Indexer
	persister *graph.Persister
	summary   *summary.Generator
	config    *Config
}

func NewIndexService(g *graph.Graph, idx *indexer.Indexer, p *graph.Persister, sum *summary.Generator, cfg *Config) *IndexService {
	return &IndexService{
		graph:     g,
		indexer:   idx,
		persister: p,
		summary:   sum,
		config:    cfg,
	}
}

// LoadGraph loads the previously persisted graph from the database.
// This is fast (<1s) and should be called synchronously at startup so tools
// have data available immediately while a background reindex runs.
func (is *IndexService) LoadGraph() {
	if err := is.persister.Load(is.graph); err != nil {
		slog.Warn("failed to load persisted graph (first run or corrupted)", "error", err)
	}
}

// IndexProject re-parses all source files and saves the updated graph.
// LoadGraph() should be called first to hydrate the graph from the database.
// Safe to call from a background goroutine.
func (is *IndexService) IndexProject() error {
	if err := is.indexer.IndexModule(is.config.ProjectPath); err != nil {
		return fmt.Errorf("index project: %w", err)
	}

	if err := is.persister.Save(is.graph); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}

	return nil
}

// GenerateSummaries generates LLM summaries for all functions.
// If a SearchService with an embedding provider is supplied, also computes
// and stores vector embeddings. Pass nil for search to skip embeddings.
func (is *IndexService) GenerateSummaries(ctx context.Context, search *SearchService) error {
	if search == nil || search.embeddingProvider == nil {
		return is.summary.GenerateAll(ctx, is.graph)
	}
	for _, fn := range is.graph.GetNodesByType(graph.NodeTypeFunction) {
		if err := search.ensureFunctionEmbedding(ctx, fn); err != nil {
			slog.Warn("failed to embed function", "function", fn.Name, "error", err)
		}
	}
	return nil
}
