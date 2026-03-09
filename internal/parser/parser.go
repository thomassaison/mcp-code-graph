package parser

import "github.com/thomas-saison/mcp-code-graph/internal/graph"

type ParseResult struct {
	Nodes []*graph.Node
	Edges []*graph.Edge
}

type Parser interface {
	ParseFile(path string) (*ParseResult, error)

	ParsePackage(dir string) (*ParseResult, error)

	ParseModule(root string) (*ParseResult, error)
}
