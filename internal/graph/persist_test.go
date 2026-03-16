package graph

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
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
	defer func() { _ = p.Close() }()
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

func TestPersistDoubleLoad_NoDuplication(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	g := New()
	n1 := &Node{ID: "func_a", Type: NodeTypeFunction, Package: "pkg", Name: "A", File: "a.go", Line: 1}
	n2 := &Node{ID: "func_b", Type: NodeTypeFunction, Package: "pkg", Name: "B", File: "b.go", Line: 1}
	g.AddNode(n1)
	g.AddNode(n2)
	g.AddEdge(&Edge{From: n1.ID, To: n2.ID, Type: EdgeTypeCalls})

	p, err := NewPersister(dbPath)
	if err != nil {
		t.Fatalf("NewPersister() error = %v", err)
	}
	defer func() { _ = p.Close() }()
	if err := p.Save(g); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Simulate the reload bug: load into the same graph twice
	g2 := New()
	if err := p.Load(g2); err != nil {
		t.Fatalf("Load #1 error = %v", err)
	}
	if err := p.Load(g2); err != nil {
		t.Fatalf("Load #2 error = %v", err)
	}

	// Edges should NOT be duplicated
	if g2.EdgeCount() != 1 {
		t.Errorf("EdgeCount() after double-load = %d, want 1", g2.EdgeCount())
	}

	callers := g2.GetCallers(n2.ID)
	if len(callers) != 1 {
		t.Errorf("GetCallers() after double-load = %d, want 1", len(callers))
	}
}

func TestPersistMigration_LegacySchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a legacy-schema database manually
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE nodes (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			package TEXT NOT NULL,
			name TEXT NOT NULL,
			file TEXT NOT NULL,
			line INTEGER NOT NULL,
			column INTEGER,
			signature TEXT,
			docstring TEXT,
			summary TEXT,
			metadata TEXT
		);
		CREATE TABLE edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			type TEXT NOT NULL,
			metadata TEXT
		);
	`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	// Insert some duplicate edges in legacy format
	_, _ = db.Exec("INSERT INTO edges (from_id, to_id, type) VALUES ('a', 'b', 'calls')")
	_, _ = db.Exec("INSERT INTO edges (from_id, to_id, type) VALUES ('a', 'b', 'calls')")
	_ = db.Close()

	// Open with NewPersister which should migrate
	p, err := NewPersister(dbPath)
	if err != nil {
		t.Fatalf("NewPersister() after migration error = %v", err)
	}
	defer func() { _ = p.Close() }()

	// The migration drops the old table, so it should be empty but have UNIQUE constraint
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A", File: "a.go", Line: 1})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B", File: "b.go", Line: 1})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})

	if err := p.Save(g); err != nil {
		t.Fatalf("Save() after migration error = %v", err)
	}

	g2 := New()
	if err := p.Load(g2); err != nil {
		t.Fatalf("Load() after migration error = %v", err)
	}

	if g2.EdgeCount() != 1 {
		t.Errorf("EdgeCount() after migration = %d, want 1", g2.EdgeCount())
	}
}

func TestPersist_UniqueConstraintPreventsDBDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	p, err := NewPersister(dbPath)
	if err != nil {
		t.Fatalf("NewPersister() error = %v", err)
	}
	defer func() { _ = p.Close() }()

	// Save a graph, then save again — INSERT OR IGNORE should prevent duplicates
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A", File: "a.go", Line: 1})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B", File: "b.go", Line: 1})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})

	if err := p.Save(g); err != nil {
		t.Fatalf("Save #1 error = %v", err)
	}
	if err := p.Save(g); err != nil {
		t.Fatalf("Save #2 error = %v", err)
	}

	// Load and verify no duplicates
	g2 := New()
	if err := p.Load(g2); err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if g2.EdgeCount() != 1 {
		t.Errorf("EdgeCount() after double-save = %d, want 1", g2.EdgeCount())
	}
}
