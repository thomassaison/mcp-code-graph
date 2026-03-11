package vector

import (
	"path/filepath"
	"testing"
)

func TestVectorStore_InsertAndSearch(t *testing.T) {
	store := newTestStore(t)

	summaryEmb := []float32{1, 0, 0}
	codeEmb := []float32{0, 1, 0}

	if err := store.Insert("fn1", "does authentication", summaryEmb, "func authenticate()", codeEmb); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := store.Insert("fn2", "handles requests", []float32{0, 0, 1}, "func handle()", []float32{1, 0, 0}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	results, err := store.Search(summaryEmb, 1)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned 0 results")
	}
	if results[0].NodeID != "fn1" {
		t.Errorf("Search()[0].NodeID = %q, want fn1", results[0].NodeID)
	}
	if results[0].Text != "does authentication" {
		t.Errorf("Search()[0].Text = %q, want summary text", results[0].Text)
	}
}

func TestVectorStore_WeightedScore(t *testing.T) {
	store := newTestStore(t)

	// query = {1, 0}
	// fn1: summaryEmb={1,0} (perfect match), codeEmb={0,1} (no match)
	//      score = 0.6*1.0 + 0.4*0.0 = 0.6
	// fn2: summaryEmb={0,1} (no match), codeEmb={1,0} (perfect match)
	//      score = 0.6*0.0 + 0.4*1.0 = 0.4
	// fn1 must rank higher than fn2
	query := []float32{1, 0}

	if err := store.Insert("fn1", "summary1", []float32{1, 0}, "code1", []float32{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Insert("fn2", "summary2", []float32{0, 1}, "code2", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search(query, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].NodeID != "fn1" {
		t.Errorf("results[0].NodeID = %q, want fn1 (score 0.6)", results[0].NodeID)
	}
	if results[1].NodeID != "fn2" {
		t.Errorf("results[1].NodeID = %q, want fn2 (score 0.4)", results[1].NodeID)
	}
}

func TestVectorStore_DegradedScore_SummaryOnly(t *testing.T) {
	store := newTestStore(t)

	summaryEmb := []float32{1, 0}
	if err := store.Insert("fn1", "summary only", summaryEmb, "", nil); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search(summaryEmb, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("node with summary-only embedding should appear in results")
	}
	if results[0].NodeID != "fn1" {
		t.Errorf("results[0].NodeID = %q, want fn1", results[0].NodeID)
	}
}

func TestVectorStore_DegradedScore_CodeOnly(t *testing.T) {
	store := newTestStore(t)

	codeEmb := []float32{1, 0}
	if err := store.Insert("fn1", "", nil, "func foo()", codeEmb); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search(codeEmb, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("node with code-only embedding should appear in results")
	}
}

func TestVectorStore_NoEmbeddings_ExcludedFromSearch(t *testing.T) {
	store := newTestStore(t)

	if err := store.Insert("fn1", "", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search([]float32{1, 0}, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.NodeID == "fn1" {
			t.Error("node with no embeddings should be excluded from search results")
		}
	}
}

func TestVectorStore_HasEmbeddings(t *testing.T) {
	store := newTestStore(t)

	store.Insert("both", "s", []float32{1}, "c", []float32{1})    //nolint:errcheck
	store.Insert("summary-only", "s", []float32{1}, "", nil)       //nolint:errcheck
	store.Insert("code-only", "", nil, "c", []float32{1})          //nolint:errcheck
	store.Insert("neither", "", nil, "", nil)                      //nolint:errcheck

	tests := []struct {
		nodeID      string
		wantSummary bool
		wantCode    bool
	}{
		{"both", true, true},
		{"summary-only", true, false},
		{"code-only", false, true},
		{"neither", false, false},
		{"missing", false, false},
	}
	for _, tt := range tests {
		hasSummary, hasCode := store.HasEmbeddings(tt.nodeID)
		if hasSummary != tt.wantSummary || hasCode != tt.wantCode {
			t.Errorf("HasEmbeddings(%q) = (%v, %v), want (%v, %v)",
				tt.nodeID, hasSummary, hasCode, tt.wantSummary, tt.wantCode)
		}
	}
}

func TestVectorStore_ScoreNodes(t *testing.T) {
	store := newTestStore(t)

	store.Insert("fn1", "auth", []float32{1, 0}, "code1", []float32{0, 1}) //nolint:errcheck
	store.Insert("fn2", "http", []float32{0, 1}, "code2", []float32{1, 0}) //nolint:errcheck
	store.Insert("fn3", "db", []float32{0, 1}, "code3", []float32{0, 1})   //nolint:errcheck

	// Only score fn1 and fn2 (not fn3)
	results := store.ScoreNodes([]float32{1, 0}, []string{"fn1", "fn2"}, 10)
	for _, r := range results {
		if r.NodeID == "fn3" {
			t.Error("fn3 should not appear in ScoreNodes results (not in nodeIDs list)")
		}
	}
	if len(results) == 0 {
		t.Error("expected results for fn1 and fn2")
	}
}

func TestVectorStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Insert with first store instance
	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s1.Insert("fn1", "summary", []float32{1, 0}, "code", []float32{0, 1}) //nolint:errcheck
	s1.Close()

	// Reload and verify
	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	hasSummary, hasCode := s2.HasEmbeddings("fn1")
	if !hasSummary || !hasCode {
		t.Errorf("HasEmbeddings after reload = (%v, %v), want (true, true)", hasSummary, hasCode)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(store.Close)
	return store
}
