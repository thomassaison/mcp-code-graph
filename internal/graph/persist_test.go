package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistSaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create and populate graph
	g := New()
	node1 := &Node{
		ID:        "func_main_test_test.go:1",
		Type:      NodeTypeFunction,
		Package:   "main",
		Name:      "test",
		File:      "test.go",
		Line:      1,
		Signature: "func test()",
	}
	node2 := &Node{
		ID:        "func_main_helper_test.go:5",
		Type:      NodeTypeFunction,
		Package:   "main",
		Name:      "helper",
		File:      "test.go",
		Line:      5,
		Signature: "func helper() int",
	}
	g.AddNode(node1)
	g.AddNode(node2)
	g.AddEdge(&Edge{From: node1.ID, To: node2.ID, Type: EdgeTypeCalls})

	// Save
	p, err := NewPersister(dbPath)
	if err != nil {
		t.Fatalf("NewPersister() error = %v", err)
	}
	defer p.Close()
	if err := p.Save(g); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file not created")
	}

	// Load into new graph
	g2 := New()
	if err := p.Load(g2); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify nodes
	if g2.NodeCount() != 2 {
		t.Errorf("NodeCount() = %d, want 2", g2.NodeCount())
	}

	// Verify edge
	callees := g2.GetCallees(node1.ID)
	if len(callees) != 1 {
		t.Errorf("GetCallees() = %d, want 1", len(callees))
	}
}
