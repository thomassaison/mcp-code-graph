package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func newTestServerWithNodes(t *testing.T, nodes []*graph.Node, edges []*graph.Edge) *Server {
	t.Helper()
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	for _, n := range nodes {
		srv.graph.AddNode(n)
	}
	for _, e := range edges {
		srv.graph.AddEdge(e)
	}
	return srv
}

func TestTool_SearchEmptyQuery(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, []*graph.Node{
		{ID: "f1", Name: "Foo", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func Foo()"},
	}, nil)

	result, err := srv.handleSearchFunctions(context.Background(), map[string]any{
		"query": "",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearchFunctions: %v", err)
	}
	// Empty query matches all functions since "" is a substring of everything
	if !strings.Contains(result, "Foo") {
		t.Errorf("empty query matches all, expected Foo in result, got: %s", result)
	}
}

func TestTool_GetCallers_nonexistentFunction(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, nil, nil)

	result, err := srv.handleGetCallers(context.Background(), map[string]any{
		"function_id": "nonexistent",
	})
	if err != nil {
		t.Fatalf("handleGetCallers: %v", err)
	}
	// No edges means callers returns nil which serializes to null
	if result != "null" {
		t.Errorf("expected null for nonexistent function, got: %s", result)
	}
}

func TestTool_GetCallees_nonexistentFunction(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, nil, nil)

	result, err := srv.handleGetCallees(context.Background(), map[string]any{
		"function_id": "nonexistent",
	})
	if err != nil {
		t.Fatalf("handleGetCallees: %v", err)
	}
	if result != "null" {
		t.Errorf("expected null for nonexistent function, got: %s", result)
	}
}

func TestTool_GetFunctionByName_noMatch(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, []*graph.Node{
		{ID: "f1", Name: "Foo", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func Foo()"},
	}, nil)

	result, err := srv.handleGetFunctionByName(context.Background(), map[string]any{
		"name": "NonExistent",
	})
	if err != nil {
		t.Fatalf("handleGetFunctionByName: %v", err)
	}
	if !strings.Contains(result, "[]") {
		t.Errorf("expected empty result for no-match query, got: %s", result)
	}
}

func TestTool_GetImplementors_nonexistentInterface(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, nil, nil)

	_, err := srv.handleGetImplementors(context.Background(), map[string]any{
		"interface_id": "type_nonexistent.Iface",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent interface, got nil")
	}
	if !strings.Contains(err.Error(), "interface not found") {
		t.Errorf("expected 'interface not found' error, got: %v", err)
	}
}

func TestTool_GetInterfaces_nonexistentType(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, nil, nil)

	_, err := srv.handleGetInterfaces(context.Background(), map[string]any{
		"type_id": "type_nonexistent.Type",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent type, got nil")
	}
	if !strings.Contains(err.Error(), "type not found") {
		t.Errorf("expected 'type not found' error, got: %v", err)
	}
}

func TestTool_UpdateSummary_nonexistentFunction(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, nil, nil)

	_, err := srv.handleUpdateSummary(context.Background(), map[string]any{
		"function_id": "nonexistent",
		"summary":     "some summary",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent function, got nil")
	}
	if !strings.Contains(err.Error(), "function not found") {
		t.Errorf("expected 'function not found' error, got: %v", err)
	}
}

func TestTool_SearchFunctions_nameSearchMatch(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, []*graph.Node{
		{ID: "f1", Name: "HandleRequest", Package: "handler", Type: graph.NodeTypeFunction, Signature: "func HandleRequest()"},
		{ID: "f2", Name: "ProcessOrder", Package: "service", Type: graph.NodeTypeFunction, Signature: "func ProcessOrder()"},
	}, nil)

	result, err := srv.handleSearchFunctions(context.Background(), map[string]any{
		"query": "Handle",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearchFunctions: %v", err)
	}
	if !strings.Contains(result, "HandleRequest") {
		t.Errorf("expected HandleRequest in result, got: %s", result)
	}
	if strings.Contains(result, "ProcessOrder") {
		t.Error("ProcessOrder should not appear in Handle search")
	}
}

func TestTool_GetCallers_returnsResult(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, []*graph.Node{
		{ID: "caller", Name: "Main", Package: "main", Type: graph.NodeTypeFunction, Signature: "func Main()"},
		{ID: "callee", Name: "Helper", Package: "util", Type: graph.NodeTypeFunction, Signature: "func Helper()"},
	}, []*graph.Edge{
		{From: "caller", To: "callee", Type: graph.EdgeTypeCalls},
	})

	result, err := srv.handleGetCallers(context.Background(), map[string]any{
		"function_id": "callee",
	})
	if err != nil {
		t.Fatalf("handleGetCallers: %v", err)
	}
	if !strings.Contains(result, "Main") {
		t.Errorf("expected Main in callers result, got: %s", result)
	}
}

func TestTool_GetCallees_returnsResult(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, []*graph.Node{
		{ID: "caller", Name: "Main", Package: "main", Type: graph.NodeTypeFunction, Signature: "func Main()"},
		{ID: "callee", Name: "Helper", Package: "util", Type: graph.NodeTypeFunction, Signature: "func Helper()"},
	}, []*graph.Edge{
		{From: "caller", To: "callee", Type: graph.EdgeTypeCalls},
	})

	result, err := srv.handleGetCallees(context.Background(), map[string]any{
		"function_id": "caller",
	})
	if err != nil {
		t.Fatalf("handleGetCallees: %v", err)
	}
	if !strings.Contains(result, "Helper") {
		t.Errorf("expected Helper in callees result, got: %s", result)
	}
}

func TestTool_SearchByBehavior_noMatch(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithNodes(t, []*graph.Node{
		{
			ID: "f1", Name: "Log", Package: "p", Type: graph.NodeTypeFunction, Signature: "func Log()",
			Metadata: map[string]any{"behaviors": []string{"logging"}},
		},
	}, nil)

	result, err := srv.handleSearchByBehavior(context.Background(), map[string]any{
		"query":     "query",
		"behaviors": []any{"database"},
	})
	if err != nil {
		t.Fatalf("handleSearchByBehavior: %v", err)
	}
	if strings.Contains(result, "Log") {
		t.Errorf("expected no Log in result for non-matching behavior, got: %s", result)
	}
}
