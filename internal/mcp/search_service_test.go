// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/vector"
)

func newTestSearchService(t *testing.T, nodes []*graph.Node) *SearchService {
	t.Helper()
	g := graph.New()
	for _, n := range nodes {
		g.AddNode(n)
	}
	vec, err := vector.NewStore(t.TempDir() + "/vec.db")
	if err != nil {
		t.Fatalf("vector.NewStore: %v", err)
	}
	t.Cleanup(func() { vec.Close() })
	return NewSearchService(g, vec, nil, nil)
}

func TestSearchService_nameSearch_basicSubstringMatch(t *testing.T) {
	t.Parallel()
	ss := newTestSearchService(t, []*graph.Node{
		{ID: "f1", Name: "HandleRequest", Package: "handler", Type: graph.NodeTypeFunction, Signature: "func HandleRequest()"},
		{ID: "f2", Name: "ProcessOrder", Package: "service", Type: graph.NodeTypeFunction, Signature: "func ProcessOrder()"},
		{ID: "f3", Name: "HandleResponse", Package: "handler", Type: graph.NodeTypeFunction, Signature: "func HandleResponse()"},
	})

	result, err := ss.nameSearch("Handle", 10)
	if err != nil {
		t.Fatalf("nameSearch: %v", err)
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(result), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 results for 'Handle', got %d: %s", len(items), result)
	}
	if strings.Contains(result, "ProcessOrder") {
		t.Error("ProcessOrder should not appear in 'Handle' search")
	}
}

func TestSearchService_nameSearch_caseInsensitive(t *testing.T) {
	t.Parallel()
	ss := newTestSearchService(t, []*graph.Node{
		{ID: "f1", Name: "HandleRequest", Package: "handler", Type: graph.NodeTypeFunction, Signature: "func HandleRequest()"},
	})

	result, err := ss.nameSearch("handlerequest", 10)
	if err != nil {
		t.Fatalf("nameSearch: %v", err)
	}

	if !strings.Contains(result, "HandleRequest") {
		t.Errorf("expected case-insensitive match, got: %s", result)
	}
}

func TestSearchService_nameSearch_respectsLimit(t *testing.T) {
	t.Parallel()
	nodes := make([]*graph.Node, 10)
	for i := range nodes {
		nodes[i] = &graph.Node{
			ID:        "f" + string(rune('0'+i)),
			Name:      "FooFunc" + string(rune('A'+i)),
			Package:   "pkg",
			Type:      graph.NodeTypeFunction,
			Signature: "func FooFunc()",
		}
	}
	ss := newTestSearchService(t, nodes)

	result, err := ss.nameSearch("Foo", 3)
	if err != nil {
		t.Fatalf("nameSearch: %v", err)
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(result), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("expected limit of 3, got %d", len(items))
	}
}

func TestSearchService_nameSearch_noMatchReturnsEmptyArray(t *testing.T) {
	t.Parallel()
	ss := newTestSearchService(t, []*graph.Node{
		{ID: "f1", Name: "DoSomething", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func DoSomething()"},
	})

	result, err := ss.nameSearch("XyzNonExistent", 10)
	if err != nil {
		t.Fatalf("nameSearch: %v", err)
	}

	// Should be a valid JSON null or empty array (marshaling nil slice gives "null")
	if result == "" {
		t.Fatal("result should not be empty string")
	}
	// nil slice marshals to "null", not "[]"
	if result != "null" {
		var items []map[string]any
		if err := json.Unmarshal([]byte(result), &items); err != nil {
			t.Fatalf("expected valid JSON, got: %s, err: %v", result, err)
		}
		if len(items) != 0 {
			t.Errorf("expected empty result, got %d items", len(items))
		}
	}
}

func TestSearchService_packageScopedSearch_filtersByPackage(t *testing.T) {
	t.Parallel()
	g := graph.New()
	g.AddNode(&graph.Node{ID: "f1", Name: "FetchUser", Package: "users", Type: graph.NodeTypeFunction, Signature: "func FetchUser()"})
	g.AddNode(&graph.Node{ID: "f2", Name: "FetchOrder", Package: "orders", Type: graph.NodeTypeFunction, Signature: "func FetchOrder()"})
	g.AddNode(&graph.Node{ID: "f3", Name: "FetchProduct", Package: "users", Type: graph.NodeTypeFunction, Signature: "func FetchProduct()"})

	vec, err := vector.NewStore(t.TempDir() + "/vec.db")
	if err != nil {
		t.Fatalf("vector.NewStore: %v", err)
	}
	t.Cleanup(func() { vec.Close() })

	ss := NewSearchService(g, vec, nil, nil)

	result, err := ss.packageScopedSearch(context.Background(), "Fetch", "users", 10)
	if err != nil {
		t.Fatalf("packageScopedSearch: %v", err)
	}

	if !strings.Contains(result, "FetchUser") {
		t.Error("expected FetchUser in users-scoped result")
	}
	if !strings.Contains(result, "FetchProduct") {
		t.Error("expected FetchProduct in users-scoped result")
	}
	if strings.Contains(result, "FetchOrder") {
		t.Error("FetchOrder (orders package) should not appear in users-scoped result")
	}
}

func TestSearchService_semanticSearch_fallsBackToNameSearch_whenNilProvider(t *testing.T) {
	t.Parallel()
	// embeddingProvider is nil — semanticSearch should not be callable directly
	// (it would panic on nil.Embed). Instead, validate that the search path used by
	// handleSearchFunctions falls back correctly. We test via a Server with nil provider.
	srv := newTestServerWithNodes(t, []*graph.Node{
		{ID: "f1", Name: "AuthenticateUser", Package: "auth", Type: graph.NodeTypeFunction, Signature: "func AuthenticateUser()"},
		{ID: "f2", Name: "LogoutUser", Package: "auth", Type: graph.NodeTypeFunction, Signature: "func LogoutUser()"},
	}, nil)

	// Server has no embeddingProvider, so handleSearchFunctions uses nameSearch
	result, err := srv.handleSearchFunctions(context.Background(), map[string]any{
		"query": "Authenticate",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearchFunctions: %v", err)
	}

	if !strings.Contains(result, "AuthenticateUser") {
		t.Errorf("expected AuthenticateUser in fallback name-search result, got: %s", result)
	}
	if strings.Contains(result, "LogoutUser") {
		t.Error("LogoutUser should not appear in 'Authenticate' search")
	}
}
