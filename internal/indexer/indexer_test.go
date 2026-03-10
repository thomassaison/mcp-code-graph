package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	goparser "github.com/thomassaison/mcp-code-graph/internal/parser/go"
)

func TestIndexFile(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func add(a, b int) int { return a + b }
func main() { println(add(1, 2)) }
`), 0644); err != nil {
		t.Fatal(err)
	}

	g := graph.New()
	idx := New(g, goparser.New())

	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile() error = %v", err)
	}

	if g.NodeCount() < 2 {
		t.Errorf("NodeCount() = %d, want at least 2", g.NodeCount())
	}

	if g.EdgeCount() < 1 {
		t.Errorf("EdgeCount() = %d, want at least 1", g.EdgeCount())
	}
}

func TestIndexPackage(t *testing.T) {
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goMod, []byte("module test\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(goFile, []byte(`package main

func add(a, b int) int { return a + b }
func multiply(x, y int) int { return x * y }
func main() { 
	println(add(1, 2))
	println(multiply(3, 4))
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	p := goparser.New()

	fileResult, err := p.ParseFile(goFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if len(fileResult.Nodes) < 3 {
		t.Errorf("ParseFile() nodes = %d, want at least 3", len(fileResult.Nodes))
	}

	g := graph.New()
	idx := New(g, p)
	if err := idx.IndexPackage(tmpDir); err != nil {
		t.Fatalf("IndexPackage() error = %v", err)
	}
	if g.NodeCount() < 3 {
		t.Errorf("IndexPackage() NodeCount = %d, want at least 3", g.NodeCount())
	}
}

func TestIndexModule(t *testing.T) {
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goMod, []byte("module test\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(goFile, []byte(`package main

func add(a, b int) int { return a + b }
func main() { println(add(1, 2)) }
`), 0644); err != nil {
		t.Fatal(err)
	}

	g := graph.New()
	idx := New(g, goparser.New())

	if err := idx.IndexModule(tmpDir); err != nil {
		t.Fatalf("IndexModule() error = %v", err)
	}

	if g.NodeCount() < 2 {
		t.Errorf("NodeCount() = %d, want at least 2", g.NodeCount())
	}

	if g.EdgeCount() < 1 {
		t.Errorf("EdgeCount() = %d, want at least 1", g.EdgeCount())
	}
}
