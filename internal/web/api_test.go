package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func newTestGraph() *graph.Graph {
	g := graph.New()
	g.AddNode(&graph.Node{
		ID:        "f1",
		Type:      graph.NodeTypeFunction,
		Package:   "pkg1",
		Name:      "Func1",
		File:      "pkg1/func1.go",
		Line:      10,
		Signature: "func Func1() error",
		Docstring: "Func1 does something",
		Summary:   &graph.Summary{Text: "A test function"},
	})
	g.AddNode(&graph.Node{
		ID:      "f2",
		Type:    graph.NodeTypeFunction,
		Package: "pkg1",
		Name:    "Func2",
		File:    "pkg1/func2.go",
		Line:    20,
	})
	g.AddNode(&graph.Node{
		ID:      "t1",
		Type:    graph.NodeTypeInterface,
		Package: "pkg2",
		Name:    "Iface1",
		Methods: []graph.Method{
			{Name: "Do", Signature: "Do() error"},
		},
	})
	g.AddEdge(&graph.Edge{From: "f1", To: "f2", Type: graph.EdgeTypeCalls})
	return g
}

func TestHandlePackages(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/packages", nil)
	rec := httptest.NewRecorder()

	h.handlePackages(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var pkgs []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pkgs))
	assert.Equal(t, []string{"pkg1", "pkg2"}, pkgs) // sorted
}

func TestHandlePackageNodes(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/packages/pkg1/nodes", nil)
	rec := httptest.NewRecorder()

	h.handlePackageNodes(rec, req, "pkg1")

	assert.Equal(t, http.StatusOK, rec.Code)

	var nodes []PackageNode
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes))
	assert.Len(t, nodes, 2)
}

func TestHandlePackageNodes_Empty(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/packages/nonexistent/nodes", nil)
	rec := httptest.NewRecorder()

	h.handlePackageNodes(rec, req, "nonexistent")

	assert.Equal(t, http.StatusOK, rec.Code)

	var nodes []PackageNode
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes))
	assert.Empty(t, nodes)
}

func TestHandleNode(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/nodes/f1", nil)
	rec := httptest.NewRecorder()

	h.handleNode(rec, req, "f1")

	assert.Equal(t, http.StatusOK, rec.Code)

	var node NodeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &node))
	assert.Equal(t, "f1", node.ID)
	assert.Equal(t, "Func1", node.Name)
	assert.Equal(t, "function", node.Type)
	assert.Equal(t, "pkg1", node.Package)
	assert.Equal(t, "pkg1/func1.go", node.File)
	assert.Equal(t, 10, node.Line)
	assert.Equal(t, "func Func1() error", node.Signature)
	assert.Equal(t, "Func1 does something", node.Docstring)
	assert.Equal(t, "A test function", node.Summary)
}

func TestHandleNode_NotFound(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/nodes/nonexistent", nil)
	rec := httptest.NewRecorder()

	h.handleNode(rec, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "node not found", errResp.Error)
}

func TestHandleNode_WithInterface(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/nodes/t1", nil)
	rec := httptest.NewRecorder()

	h.handleNode(rec, req, "t1")

	assert.Equal(t, http.StatusOK, rec.Code)

	var node NodeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &node))
	assert.Equal(t, "interface", node.Type)
	assert.Len(t, node.Methods, 1)
	assert.Equal(t, "Do", node.Methods[0].Name)
}

func TestHandleNeighborhood(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/nodes/f1/neighborhood?depth=1", nil)
	rec := httptest.NewRecorder()

	h.handleNeighborhood(rec, req, "f1")

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp NeighborhoodResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "f1", resp.Center.ID)
	assert.GreaterOrEqual(t, len(resp.Nodes), 2) // f1 + f2
	assert.GreaterOrEqual(t, len(resp.Edges), 1) // f1->f2
}

func TestHandleNeighborhood_NotFound(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/nodes/nonexistent/neighborhood", nil)
	rec := httptest.NewRecorder()

	h.handleNeighborhood(rec, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleNeighborhood_DefaultDepth(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/nodes/f1/neighborhood", nil)
	rec := httptest.NewRecorder()

	h.handleNeighborhood(rec, req, "f1")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleNeighborhood_InvalidDepth(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	// depth=5 should be clamped to default 1
	req := httptest.NewRequest("GET", "/api/nodes/f1/neighborhood?depth=5", nil)
	rec := httptest.NewRecorder()

	h.handleNeighborhood(rec, req, "f1")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleSearch(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/search?q=Func1", nil)
	rec := httptest.NewRecorder()

	h.handleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var nodes []PackageNode
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes))
	assert.Len(t, nodes, 1)
	assert.Equal(t, "Func1", nodes[0].Name)
}

func TestHandleSearch_Empty(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/search?q=", nil)
	rec := httptest.NewRecorder()

	h.handleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var nodes []PackageNode
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes))
	assert.Empty(t, nodes)
}

func TestHandleSearch_NoResults(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/search?q=Nonexistent", nil)
	rec := httptest.NewRecorder()

	h.handleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var nodes []PackageNode
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes))
	assert.Empty(t, nodes)
}

func TestHandleStats(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/stats", nil)
	rec := httptest.NewRecorder()

	h.handleStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats StatsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
	assert.Equal(t, 3, stats.NodeCount)
	assert.Equal(t, 1, stats.EdgeCount)
	assert.Equal(t, 2, stats.ByType["function"])
	assert.Equal(t, 1, stats.ByType["interface"])
	assert.Equal(t, 2, stats.ByPackage["pkg1"])
	assert.Equal(t, 1, stats.ByPackage["pkg2"])
}

func TestHandleGraph(t *testing.T) {
	h := NewHandler(newTestGraph(), "")
	req := httptest.NewRequest("GET", "/api/graph", nil)
	rec := httptest.NewRecorder()

	h.handleGraph(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp GraphResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 3, len(resp.Nodes))
	assert.Equal(t, 1, len(resp.Edges))

	// Verify node details are populated
	for _, n := range resp.Nodes {
		if n.Name == "Func1" {
			assert.Equal(t, "func Func1() error", n.Signature)
			assert.Equal(t, "A test function", n.Summary)
		}
	}
}

func TestHandleGraph_ScopeFiltering(t *testing.T) {
	g := graph.New()
	// Project-local nodes
	g.AddNode(&graph.Node{ID: "p1", Type: graph.NodeTypeFunction, Package: "example.com/mymod/internal/web", Name: "Handler"})
	g.AddNode(&graph.Node{ID: "p2", Type: graph.NodeTypeFunction, Package: "example.com/mymod/internal/graph", Name: "Graph"})
	// External/stdlib nodes
	g.AddNode(&graph.Node{ID: "e1", Type: graph.NodeTypeFunction, Package: "fmt", Name: "Println"})
	g.AddNode(&graph.Node{ID: "e2", Type: graph.NodeTypeFunction, Package: "github.com/other/lib", Name: "Do"})
	// Edges: project-to-project, project-to-external
	g.AddEdge(&graph.Edge{From: "p1", To: "p2", Type: graph.EdgeTypeCalls})
	g.AddEdge(&graph.Edge{From: "p1", To: "e1", Type: graph.EdgeTypeCalls})
	g.AddEdge(&graph.Edge{From: "e1", To: "e2", Type: graph.EdgeTypeCalls})

	t.Run("default scope filters to project", func(t *testing.T) {
		h := NewHandler(g, "example.com/mymod")
		req := httptest.NewRequest("GET", "/api/graph", nil)
		rec := httptest.NewRecorder()
		h.handleGraph(rec, req)

		var resp GraphResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, 2, len(resp.Nodes), "should only include project nodes")
		assert.Equal(t, 1, len(resp.Edges), "should only include edges between project nodes")
	})

	t.Run("scope=all returns everything", func(t *testing.T) {
		h := NewHandler(g, "example.com/mymod")
		req := httptest.NewRequest("GET", "/api/graph?scope=all", nil)
		rec := httptest.NewRecorder()
		h.handleGraph(rec, req)

		var resp GraphResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, 4, len(resp.Nodes), "should include all nodes")
		assert.Equal(t, 3, len(resp.Edges), "should include all edges")
	})

	t.Run("no modulePath returns all nodes", func(t *testing.T) {
		h := NewHandler(g, "")
		req := httptest.NewRequest("GET", "/api/graph", nil)
		rec := httptest.NewRecorder()
		h.handleGraph(rec, req)

		var resp GraphResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, 4, len(resp.Nodes), "with empty modulePath should include all")
		assert.Equal(t, 3, len(resp.Edges), "with empty modulePath should include all edges")
	})
}

func TestIsProjectPackage(t *testing.T) {
	tests := []struct {
		pkg        string
		modulePath string
		want       bool
	}{
		{"example.com/mymod/internal/web", "example.com/mymod", true},
		{"example.com/mymod", "example.com/mymod", true},
		{"fmt", "example.com/mymod", false},
		{"github.com/other/lib", "example.com/mymod", false},
		{"main", "example.com/mymod", true},
	}
	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			assert.Equal(t, tt.want, isProjectPackage(tt.pkg, tt.modulePath))
		})
	}
}

func TestServeHTTP_Routing(t *testing.T) {
	h := NewHandler(newTestGraph(), "")

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/api/graph", http.StatusOK},
		{"/api/packages", http.StatusOK},
		{"/api/packages/pkg1/nodes", http.StatusOK},
		{"/api/nodes/f1", http.StatusOK},
		{"/api/nodes/nonexistent", http.StatusNotFound},
		{"/api/nodes/f1/neighborhood", http.StatusOK},
		{"/api/search?q=Func1", http.StatusOK},
		{"/api/stats", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
