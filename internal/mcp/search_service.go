// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/embedding"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/summary"
	"github.com/thomassaison/mcp-code-graph/internal/vector"
)

// SearchService owns search-related concerns: semantic search, name search,
// embedding generation, and behavior-filtered search.
type SearchService struct {
	graph             *graph.Graph
	vector            *vector.Store
	embeddingProvider embedding.EmbeddingProvider
	summary           *summary.Generator
}

func NewSearchService(g *graph.Graph, vec *vector.Store, emb embedding.EmbeddingProvider, sum *summary.Generator) *SearchService {
	return &SearchService{
		graph:             g,
		vector:            vec,
		embeddingProvider: emb,
		summary:           sum,
	}
}

func (ss *SearchService) semanticSearch(ctx context.Context, query string, limit int) (string, error) {
	queryEmbedding, err := ss.embeddingProvider.Embed(ctx, query)
	if err != nil {
		slog.Warn("failed to embed query, falling back to name search", "err", err)
		return ss.nameSearch(query, limit)
	}
	slog.Debug("query embedded", "dim", len(queryEmbedding))

	results, err := ss.vector.Search(queryEmbedding, limit)
	if err != nil {
		slog.Warn("vector search failed, falling back to name search", "err", err)
		return ss.nameSearch(query, limit)
	}
	slog.Debug("vector search complete", "results", len(results))

	if len(results) == 0 {
		slog.Warn("no vector search results (embeddings may not be generated yet), falling back to name search")
		return ss.nameSearch(query, limit)
	}

	var output []map[string]any
	for _, r := range results {
		node, err := ss.graph.GetNode(r.NodeID)
		if err != nil {
			continue
		}
		output = append(output, map[string]any{
			"id":        node.ID,
			"name":      node.Name,
			"package":   node.Package,
			"signature": node.Signature,
			"summary":   node.SummaryText(),
			"score":     r.Score,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ss *SearchService) nameSearch(query string, limit int) (string, error) {
	functions := ss.graph.GetNodesByType(graph.NodeTypeFunction)
	var results []map[string]any

	queryLower := strings.ToLower(query)

	for _, fn := range functions {
		nameLower := strings.ToLower(fn.Name)
		pkgLower := strings.ToLower(fn.Package)

		if strings.Contains(nameLower, queryLower) || strings.Contains(pkgLower, queryLower) {
			results = append(results, map[string]any{
				"id":        fn.ID,
				"name":      fn.Name,
				"package":   fn.Package,
				"signature": fn.Signature,
			})

			if len(results) >= limit {
				break
			}
		}
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ss *SearchService) packageScopedSearch(ctx context.Context, query, pkg string, limit int) (string, error) {
	pkgNodes := ss.graph.GetNodesByPackageAndType(pkg, graph.NodeTypeFunction)

	if ss.embeddingProvider != nil {
		nodeIDs := make([]string, 0, len(pkgNodes))
		for _, n := range pkgNodes {
			if err := ss.ensureFunctionEmbedding(ctx, n); err != nil {
				continue
			}
			nodeIDs = append(nodeIDs, n.ID)
		}

		queryEmbedding, err := ss.embeddingProvider.Embed(ctx, query)
		if err != nil {
			// Fallback to name-scoped search
			return ss.nameSearchInNodes(query, pkgNodes, limit)
		}

		scored := ss.vector.ScoreNodes(queryEmbedding, nodeIDs, limit)
		var output []map[string]any
		for _, r := range scored {
			node, err := ss.graph.GetNode(r.NodeID)
			if err != nil {
				continue
			}
			output = append(output, map[string]any{
				"id":        node.ID,
				"name":      node.Name,
				"package":   node.Package,
				"signature": node.Signature,
				"summary":   node.SummaryText(),
				"score":     r.Score,
			})
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return ss.nameSearchInNodes(query, pkgNodes, limit)
}

func (ss *SearchService) nameSearchInNodes(query string, nodes []*graph.Node, limit int) (string, error) {
	queryLower := strings.ToLower(query)
	var results []map[string]any

	for _, fn := range nodes {
		nameLower := strings.ToLower(fn.Name)
		if strings.Contains(nameLower, queryLower) {
			results = append(results, map[string]any{
				"id":        fn.ID,
				"name":      fn.Name,
				"package":   fn.Package,
				"signature": fn.Signature,
			})
			if len(results) >= limit {
				break
			}
		}
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ss *SearchService) ensureFunctionEmbedding(ctx context.Context, node *graph.Node) error {
	slog.Log(ctx, debug.LevelTrace, "ensuring function embedding", "function", node.Name)

	hasSummary, hasCode := ss.vector.HasEmbeddings(node.ID)
	if hasSummary && hasCode {
		return nil // already fully embedded
	}

	if node.Summary == nil || node.Summary.Text == "" {
		if err := ss.summary.Generate(ctx, node); err != nil {
			return fmt.Errorf("generate summary: %w", err)
		}
		if node.Summary != nil {
			ss.graph.SetNodeSummary(node.ID, node.Summary) //nolint:errcheck
		}
	}

	summaryText := node.SummaryText()
	if summaryText == "" {
		summaryText = fmt.Sprintf("%s %s", node.Name, node.Signature)
	}

	summaryEmb, err := ss.embeddingProvider.Embed(ctx, summaryText)
	if err != nil {
		return fmt.Errorf("embed summary: %w", err)
	}

	var codeEmb []float32
	if node.Code != "" {
		codeEmb, err = ss.embeddingProvider.Embed(ctx, node.Code)
		if err != nil {
			slog.Warn("failed to embed code, proceeding with summary only", "function", node.Name, "error", err)
			codeEmb = nil
		}
	}

	if err := ss.vector.Insert(node.ID, summaryText, summaryEmb, node.Code, codeEmb); err != nil {
		return fmt.Errorf("store embedding: %w", err)
	}

	return nil
}

func (ss *SearchService) semanticBehaviorSearch(ctx context.Context, query string, nodes []*graph.Node, limit int) (string, error) {
	queryEmbedding, err := ss.embeddingProvider.Embed(ctx, query)
	if err != nil {
		slog.Debug("embedding failed, returning filtered results", "error", err)
		return ss.formatBehaviorResults(nodes, limit), nil
	}

	nodeIDs := make([]string, 0, len(nodes))
	nodeByID := make(map[string]*graph.Node, len(nodes))
	for _, node := range nodes {
		if err := ss.ensureFunctionEmbedding(ctx, node); err != nil {
			continue
		}
		nodeIDs = append(nodeIDs, node.ID)
		nodeByID[node.ID] = node
	}

	scored := ss.vector.ScoreNodes(queryEmbedding, nodeIDs, limit)

	var results []map[string]any
	for _, r := range scored {
		node, ok := nodeByID[r.NodeID]
		if !ok {
			continue
		}
		results = append(results, map[string]any{
			"id":        node.ID,
			"name":      node.Name,
			"package":   node.Package,
			"signature": node.Signature,
			"behaviors": node.Metadata["behaviors"],
			"summary":   node.SummaryText(),
			"score":     r.Score,
		})
	}

	resultJSON, _ := json.MarshalIndent(results, "", "  ")
	return string(resultJSON), nil
}

func (ss *SearchService) formatBehaviorResults(nodes []*graph.Node, limit int) string {
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}

	var results []map[string]any
	for _, node := range nodes {
		results = append(results, map[string]any{
			"id":        node.ID,
			"name":      node.Name,
			"package":   node.Package,
			"signature": node.Signature,
			"behaviors": node.Metadata["behaviors"],
			"summary":   node.SummaryText(),
		})
	}

	resultJSON, _ := json.MarshalIndent(results, "", "  ")
	return string(resultJSON)
}
