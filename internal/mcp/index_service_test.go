// anthropic/claude-sonnet-4-6
package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/indexer"
	goparser "github.com/thomassaison/mcp-code-graph/internal/parser/go"
	"github.com/thomassaison/mcp-code-graph/internal/summary"
)

func newTestIndexService(t *testing.T, projectPath string) *IndexService {
	t.Helper()
	g := graph.New()
	p := goparser.New()
	idx := indexer.NewWithBehaviorAnalyzer(g, p, nil)

	dbPath := filepath.Join(t.TempDir(), "test.db.graph.db")
	persister, err := graph.NewPersister(dbPath)
	if err != nil {
		t.Fatalf("graph.NewPersister: %v", err)
	}
	t.Cleanup(func() { _ = persister.Close() })

	gen := summary.NewGenerator(nil, "")
	cfg := &Config{
		DBPath:      filepath.Join(t.TempDir(), "test.db"),
		ProjectPath: projectPath,
	}
	return NewIndexService(g, idx, persister, gen, cfg)
}

func TestIndexService_LoadGraph_emptyPersister(t *testing.T) {
	t.Parallel()
	is := newTestIndexService(t, t.TempDir())

	// Should not panic or return error — empty database loads zero nodes.
	is.LoadGraph()
}

func TestIndexService_IndexProject_simpleGoFile(t *testing.T) {
	t.Parallel()

	// Create a minimal Go module in a temp directory.
	dir := t.TempDir()

	goModContent := "module example.com/testmod\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	goSrc := `package main

func Add(a, b int) int {
	return a + b
}

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goSrc), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	is := newTestIndexService(t, dir)
	if err := is.IndexProject(); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	// After indexing, the graph should have at least the Add function.
	nodes := is.graph.GetNodesByType(graph.NodeTypeFunction)
	var foundAdd bool
	for _, n := range nodes {
		if n.Name == "Add" {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		names := make([]string, 0, len(nodes))
		for _, n := range nodes {
			names = append(names, n.Name)
		}
		t.Errorf("expected 'Add' function in indexed graph, got: %v", names)
	}
}
