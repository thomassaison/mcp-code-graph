package indexer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
	"github.com/thomassaison/mcp-code-graph/internal/types"
)

type Indexer struct {
	graph  *graph.Graph
	parser parser.Parser
}

func New(g *graph.Graph, p parser.Parser) *Indexer {
	return &Indexer{
		graph:  g,
		parser: p,
	}
}

func (idx *Indexer) IndexModule(root string) error {
	slog.Debug("indexing module", "root", root)
	result, err := idx.parser.ParseModule(root)
	if err != nil {
		return fmt.Errorf("parse module: %w", err)
	}

	seen := make(map[string]struct{})
	for _, node := range idx.graph.AllNodes() {
		if _, ok := seen[node.Package]; !ok {
			seen[node.Package] = struct{}{}
			idx.graph.RemoveNodesForPackage(node.Package)
		}
	}

	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
	}

	resolveEdges(result)

	for _, edge := range result.Edges {
		idx.graph.AddEdge(edge)
	}

	// Run type checker for interface resolution
	checker := types.NewChecker()
	typeResult, err := checker.Check(root)
	if err != nil {
		slog.Warn("type check failed, continuing without interface data", "error", err)
	} else {
		// Add interface and type nodes
		for _, node := range typeResult.Interfaces {
			idx.graph.AddNode(node)
		}
		for _, node := range typeResult.Types {
			idx.graph.AddNode(node)
		}
		// Add implementation edges
		for _, edge := range typeResult.Edges {
			idx.graph.AddEdge(edge)
		}
	}

	slog.Debug("indexing module complete", "nodes", len(result.Nodes), "edges", len(result.Edges))
	return nil
}

func (idx *Indexer) IndexPackage(dir string) error {
	result, err := idx.parser.ParsePackage(dir)
	if err != nil {
		return fmt.Errorf("parse package: %w", err)
	}

	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
	}

	resolveEdges(result)

	for _, edge := range result.Edges {
		idx.graph.AddEdge(edge)
	}

	return nil
}

func (idx *Indexer) IndexFile(path string) error {
	result, err := idx.parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
	}

	resolveEdges(result)

	for _, edge := range result.Edges {
		idx.graph.AddEdge(edge)
	}

	return nil
}

func resolveEdges(result *parser.ParseResult) {
	placeholderToID := make(map[string]*graph.Node, len(result.Nodes))
	for _, node := range result.Nodes {
		placeholder := fmt.Sprintf("func_%s_%s", node.Package, node.Name)
		placeholderToID[placeholder] = node
	}

	for _, edge := range result.Edges {
		if resolved, ok := placeholderToID[edge.To]; ok {
			edge.To = resolved.ID
			slog.Log(context.Background(), debug.LevelTrace, "edge resolved", "from", edge.From, "to", edge.To)
		}
	}
}
