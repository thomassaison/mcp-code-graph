package vector

import (
	"path/filepath"
	"testing"
)

func TestVectorStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	embedding1 := []float32{0.1, 0.2, 0.3}
	embedding2 := []float32{0.4, 0.5, 0.6}

	if err := store.Insert("node1", "function summary 1", embedding1); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := store.Insert("node2", "function summary 2", embedding2); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	results, err := store.Search(embedding1, 1)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}

	if results[0].NodeID != "node1" {
		t.Errorf("Search()[0].NodeID = %q, want %q", results[0].NodeID, "node1")
	}
}
