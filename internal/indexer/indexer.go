package indexer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thomassaison/mcp-code-graph/internal/behavior"
	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
	"github.com/thomassaison/mcp-code-graph/internal/types"
)

type Indexer struct {
	graph            *graph.Graph
	parser           parser.Parser
	behaviorAnalyzer behavior.Analyzer
}

func New(g *graph.Graph, p parser.Parser) *Indexer {
	return &Indexer{
		graph:  g,
		parser: p,
	}
}

func NewWithBehaviorAnalyzer(g *graph.Graph, p parser.Parser, analyzer behavior.Analyzer) *Indexer {
	return &Indexer{
		graph:            g,
		parser:           p,
		behaviorAnalyzer: analyzer,
	}
}

func (idx *Indexer) IndexModule(root string) error {
	slog.Debug("indexing module", "root", root)
	result, err := idx.parser.ParseModule(root)
	if err != nil {
		return fmt.Errorf("parse module: %w", err)
	}

	checker := types.NewChecker()
	typeResult, err := checker.Check(root)
	if err != nil {
		slog.Warn("type check failed, continuing without interface data", "error", err)
	}

	newGraph := graph.New()

	for _, node := range result.Nodes {
		newGraph.AddNode(node)
	}

	resolveEdges(result)

	for _, edge := range result.Edges {
		newGraph.AddEdge(edge)
	}

	if err == nil {
		for _, node := range typeResult.Interfaces {
			newGraph.AddNode(node)
		}
		for _, node := range typeResult.Types {
			newGraph.AddNode(node)
		}
		for _, edge := range typeResult.Edges {
			newGraph.AddEdge(edge)
		}
	}

	idx.analyzeBehaviors(context.Background(), newGraph, result.Nodes)

	idx.graph.ReplaceAll(newGraph)

	slog.Debug("indexing module complete", "nodes", len(result.Nodes), "edges", len(result.Edges))
	return nil
}

func (idx *Indexer) analyzeBehaviors(ctx context.Context, g *graph.Graph, nodes []*graph.Node) {
	if idx.behaviorAnalyzer == nil {
		return
	}

	for _, node := range nodes {
		if node.Type != graph.NodeTypeFunction && node.Type != graph.NodeTypeMethod {
			continue
		}

		req := behavior.AnalysisRequest{
			PackageName:  node.Package,
			FunctionName: node.Name,
			Signature:    node.Signature,
			Docstring:    node.Docstring,
		}

		if codeRaw, ok := node.Metadata["code"]; ok {
			if code, ok := codeRaw.(string); ok {
				req.Code = code
			}
		}

		behaviors, err := idx.behaviorAnalyzer.Analyze(ctx, req)
		if err != nil {
			slog.Debug("behavior analysis failed", "function", node.Name, "error", err)
			continue
		}

		if node.Metadata == nil {
			node.Metadata = make(map[string]any)
		}
		node.Metadata["behaviors"] = behaviors
	}
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

	// Remove stale nodes from this specific file before re-indexing
	// to avoid accumulating deleted/renamed functions.
	// We scope to the file (not the whole package) to avoid destroying
	// sibling files' nodes during incremental updates.
	idx.graph.RemoveNodesForFile(path)

	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
	}

	resolveEdges(result)

	for _, edge := range result.Edges {
		idx.graph.AddEdge(edge)
	}

	return nil
}

func (idx *Indexer) Graph() *graph.Graph {
	return idx.graph
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
