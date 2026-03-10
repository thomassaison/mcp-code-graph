# Web Visualization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add embedded web server for visual graph exploration with package tree, neighborhood graph, and metadata display.

**Architecture:** Single binary with embedded frontend (embed.FS), web handler reads from in-memory *graph.Graph directly, opt-in via MCP_CODE_GRAPH_WEB env var.

**Tech Stack:** Go stdlib (net/http, embed), Cytoscape.js (CDN), vanilla JS

---

## Task 1: Create Web Package Structure

**Files:**
- Create: `internal/web/embed.go`
- Create: `internal/web/handler.go`
- Create: `internal/web/api.go`

**Step 1: Create embed.go with empty static placeholder**

```go
package web

import "embed"

//go:embed static/*
var staticFS embed.FS
```

**Step 2: Create placeholder static directory**

```bash
mkdir -p internal/web/static
touch internal/web/static/.gitkeep
```

**Step 3: Create handler.go with Handler type**

```go
package web

import (
	"net/http"
	
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

type Handler struct {
	graph *graph.Graph
}

func NewHandler(g *graph.Graph) *Handler {
	return &Handler{graph: g}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serve index.html for root, API routes handled separately
	if r.URL.Path == "/" {
		http.ServeFileFS(w, r, staticFS, "static/index.html")
		return
	}
	http.NotFound(w, r)
}
```

**Step 4: Create api.go with response types**

```go
package web

type PackageNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type NodeResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Package   string         `json:"package"`
	File      string         `json:"file"`
	Line      int            `json:"line"`
	Signature string         `json:"signature,omitempty"`
	Docstring string         `json:"docstring,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Behaviors []string       `json:"behaviors,omitempty"`
	Methods   []MethodResp   `json:"methods,omitempty"`
}

type MethodResp struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

type NeighborhoodResponse struct {
	Center NodeResponse   `json:"center"`
	Nodes  []NodeResponse `json:"nodes"`
	Edges  []EdgeResponse `json:"edges"`
}

type EdgeResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type StatsResponse struct {
	NodeCount  int `json:"node_count"`
	EdgeCount  int `json:"edge_count"`
	ByType     map[string]int `json:"by_type"`
	ByPackage  map[string]int `json:"by_package"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
```

**Step 5: Commit**

```bash
git add internal/web/
git commit -m "feat: add web package structure with types"
```

---

## Task 2: Add Graph Helper Methods

**Files:**
- Modify: `internal/graph/graph.go`
- Test: `internal/graph/graph_test.go`

**Step 1: Write failing test for AllPackages**

```go
func TestAllPackages(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "func1", Type: NodeTypeFunction, Package: "pkg1", Name: "Func1"})
	g.AddNode(&Node{ID: "func2", Type: NodeTypeFunction, Package: "pkg2", Name: "Func2"})
	g.AddNode(&Node{ID: "func3", Type: NodeTypeFunction, Package: "pkg1", Name: "Func3"})

	pkgs := g.AllPackages()
	sort.Strings(pkgs)
	assert.Equal(t, []string{"pkg1", "pkg2"}, pkgs)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/... -run TestAllPackages -v`
Expected: FAIL - method doesn't exist

**Step 3: Implement AllPackages**

```go
func (g *Graph) AllPackages() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pkgs := make(map[string]bool)
	for pkg := range g.byPackage {
		pkgs[pkg] = true
	}
	
	result := make([]string, 0, len(pkgs))
	for pkg := range pkgs {
		result = append(result, pkg)
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/... -run TestAllPackages -v`
Expected: PASS

**Step 5: Write failing test for GetNeighborhood**

```go
func TestGetNeighborhood(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeCalls})

	nodes, edges := g.GetNeighborhood("b", 1)
	assert.Len(t, nodes, 3) // a, b, c
	assert.Len(t, edges, 2)

	nodes, edges = g.GetNeighborhood("b", 2)
	assert.Len(t, nodes, 3)
	assert.Len(t, edges, 2)
}
```

**Step 6: Run test to verify it fails**

Run: `go test ./internal/graph/... -run TestGetNeighborhood -v`
Expected: FAIL - method doesn't exist

**Step 7: Implement GetNeighborhood**

```go
func (g *Graph) GetNeighborhood(nodeID string, depth int) ([]*Node, []*Edge) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	nodes := make(map[string]*Node)
	edges := make(map[string]*Edge)

	var visit func(id string, d int)
	visit = func(id string, d int) {
		if d < 0 || visited[id] {
			return
		}
		visited[id] = true
		
		if node, ok := g.nodes[id]; ok {
			nodes[id] = node
		}

		for _, edge := range g.edges[id] {
			edgeKey := edge.From + "->" + edge.To
			edges[edgeKey] = edge
			visit(edge.To, d-1)
		}

		for _, edge := range g.inEdges[id] {
			edgeKey := edge.From + "->" + edge.To
			edges[edgeKey] = edge
			visit(edge.From, d-1)
		}
	}

	visit(nodeID, depth)

	result := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
	}

	edgeResult := make([]*Edge, 0, len(edges))
	for _, edge := range edges {
		edgeResult = append(edgeResult, edge)
	}

	return result, edgeResult
}
```

**Step 8: Run test to verify it passes**

Run: `go test ./internal/graph/... -run TestGetNeighborhood -v`
Expected: PASS

**Step 9: Commit**

```bash
git add internal/graph/
git commit -m "feat: add AllPackages and GetNeighborhood to Graph"
```

---

## Task 3: Implement API Handlers with Tests

**Files:**
- Modify: `internal/web/api.go`
- Create: `internal/web/api_test.go`

**Step 1: Write test for handlePackages**

```go
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func TestHandlePackages(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "f1", Type: graph.NodeTypeFunction, Package: "pkg1", Name: "Func1"})
	g.AddNode(&graph.Node{ID: "f2", Type: graph.NodeTypeFunction, Package: "pkg2", Name: "Func2"})

	h := NewHandler(g)
	req := httptest.NewRequest("GET", "/api/packages", nil)
	rec := httptest.NewRecorder()

	h.handlePackages(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	
	var pkgs []string
	json.Unmarshal(rec.Body.Bytes(), &pkgs)
	assert.Contains(t, pkgs, "pkg1")
	assert.Contains(t, pkgs, "pkg2")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestHandlePackages -v`
Expected: FAIL - method doesn't exist

**Step 3: Implement handlePackages**

```go
import (
	"encoding/json"
	"net/http"
)

func (h *Handler) handlePackages(w http.ResponseWriter, r *http.Request) {
	pkgs := h.graph.AllPackages()
	sort.Strings(pkgs)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pkgs)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/web/... -run TestHandlePackages -v`
Expected: PASS

**Step 5: Write test for handlePackageNodes**

```go
func TestHandlePackageNodes(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "f1", Type: graph.NodeTypeFunction, Package: "pkg1", Name: "Func1"})
	g.AddNode(&graph.Node{ID: "t1", Type: graph.NodeTypeInterface, Package: "pkg1", Name: "Interface1"})

	h := NewHandler(g)
	req := httptest.NewRequest("GET", "/api/packages/pkg1/nodes", nil)
	rec := httptest.NewRecorder()

	h.handlePackageNodes(rec, req, "pkg1")

	assert.Equal(t, http.StatusOK, rec.Code)
	
	var nodes []PackageNode
	json.Unmarshal(rec.Body.Bytes(), &nodes)
	assert.Len(t, nodes, 2)
}
```

**Step 6: Implement handlePackageNodes**

```go
import "strings"

func (h *Handler) handlePackageNodes(w http.ResponseWriter, r *http.Request, pkg string) {
	nodes := h.graph.GetNodesByPackage(pkg)
	
	result := make([]PackageNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, PackageNode{
			ID:   n.ID,
			Name: n.Name,
			Type: string(n.Type),
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
```

**Step 7: Write tests for remaining endpoints (handleNode, handleNeighborhood, handleSearch, handleStats)**

Following same TDD pattern - write test, run to fail, implement, run to pass.

**handleNode:**
```go
func (h *Handler) handleNode(w http.ResponseWriter, r *http.Request, id string) {
	node, err := h.graph.GetNode(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "node not found"})
		return
	}

	resp := NodeResponse{
		ID:        node.ID,
		Name:      node.Name,
		Type:      string(node.Type),
		Package:   node.Package,
		File:      node.File,
		Line:      node.Line,
		Signature: node.Signature,
		Docstring: node.Docstring,
	}
	
	if node.Summary != nil {
		resp.Summary = node.Summary.Text
	}
	
	if beh, ok := node.Metadata["behaviors"].([]string); ok {
		resp.Behaviors = beh
	}
	
	for _, m := range node.Methods {
		resp.Methods = append(resp.Methods, MethodResp{
			Name:      m.Name,
			Signature: m.Signature,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

**handleNeighborhood:**
```go
import "strconv"

func (h *Handler) handleNeighborhood(w http.ResponseWriter, r *http.Request, id string) {
	depth := 1
	if d := r.URL.Query().Get("depth"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n >= 1 && n <= 3 {
			depth = n
		}
	}

	center, err := h.graph.GetNode(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "node not found"})
		return
	}

	nodes, edges := h.graph.GetNeighborhood(id, depth)

	nodeResponses := make([]NodeResponse, 0, len(nodes))
	for _, n := range nodes {
		nodeResponses = append(nodeResponses, NodeResponse{
			ID:      n.ID,
			Name:    n.Name,
			Type:    string(n.Type),
			Package: n.Package,
		})
	}

	edgeResponses := make([]EdgeResponse, 0, len(edges))
	for _, e := range edges {
		edgeResponses = append(edgeResponses, EdgeResponse{
			From: e.From,
			To:   e.To,
			Type: string(e.Type),
		})
	}

	resp := NeighborhoodResponse{
		Center: NodeResponse{
			ID:      center.ID,
			Name:    center.Name,
			Type:    string(center.Type),
			Package: center.Package,
		},
		Nodes: nodeResponses,
		Edges: edgeResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

**handleSearch:**
```go
import "strings"

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PackageNode{})
		return
	}

	nodes := h.graph.GetNodesByName(query)
	
	result := make([]PackageNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, PackageNode{
			ID:   n.ID,
			Name: n.Name,
			Type: string(n.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
```

**handleStats:**
```go
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	byType := make(map[string]int)
	for _, node := range h.graph.AllNodes() {
		byType[string(node.Type)]++
	}

	byPackage := make(map[string]int)
	for _, pkg := range h.graph.AllPackages() {
		byPackage[pkg] = len(h.graph.GetNodesByPackage(pkg))
	}

	resp := StatsResponse{
		NodeCount: h.graph.NodeCount(),
		EdgeCount: h.graph.EdgeCount(),
		ByType:    byType,
		ByPackage: byPackage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

**Step 8: Run all tests**

Run: `go test ./internal/web/... -v`
Expected: All PASS

**Step 9: Commit**

```bash
git add internal/web/
git commit -m "feat: implement API handlers for web visualization"
```

---

## Task 4: Wire Up HTTP Routing

**Files:**
- Modify: `internal/web/handler.go`

**Step 1: Update handler.go with routing**

```go
package web

import (
	"net/http"
	"strings"
)

type Handler struct {
	graph *graph.Graph
}

func NewHandler(g *graph.Graph) *Handler {
	return &Handler{g}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/":
		http.ServeFileFS(w, r, staticFS, "static/index.html")
	case path == "/api/packages":
		h.handlePackages(w, r)
	case strings.HasPrefix(path, "/api/packages/"):
		pkg := strings.TrimPrefix(path, "/api/packages/")
		h.handlePackageNodes(w, r, pkg)
	case strings.HasPrefix(path, "/api/nodes/"):
		parts := strings.Split(strings.TrimPrefix(path, "/api/nodes/"), "/neighborhood")
		id := parts[0]
		if len(parts) > 1 {
			h.handleNeighborhood(w, r, id)
		} else {
			h.handleNode(w, r, id)
		}
	case path == "/api/search":
		h.handleSearch(w, r)
	case path == "/api/stats":
		h.handleStats(w, r)
	default:
		http.NotFound(w, r)
	}
}
```

**Step 2: Write test for routing**

```go
func TestServeHTTP_Routing(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "f1", Type: graph.NodeTypeFunction, Package: "pkg", Name: "Func1"})

	h := NewHandler(g)

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/api/packages", http.StatusOK},
		{"/api/packages/pkg/nodes", http.StatusOK},
		{"/api/nodes/f1", http.StatusOK},
		{"/api/nodes/nonexistent", http.StatusNotFound},
		{"/api/nodes/f1/neighborhood", http.StatusOK},
		{"/api/search?q=Func1", http.StatusOK},
		{"/api/stats", http.StatusOK},
		{"/nonexistent", http.StatusNotFound},
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
```

**Step 3: Run tests**

Run: `go test ./internal/web/... -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/web/
git commit -m "feat: wire up HTTP routing for API endpoints"
```

---

## Task 5: Add Web Server Startup to main.go

**Files:**
- Modify: `cmd/mcp-code-graph/main.go`

**Step 1: Add web server startup**

After line 89 (after logging indexed node count), add:

```go
// Start web server if configured
if webAddr := os.Getenv("MCP_CODE_GRAPH_WEB"); webAddr != "" {
	go func() {
		webHandler := web.NewHandler(server.Graph())
		slog.Info("Starting web server", "address", webAddr)
		if err := http.ListenAndServe(webAddr, webHandler); err != nil {
			slog.Error("Web server error", "error", err)
		}
	}()
}
```

Add import:
```go
import (
	"net/http"
	"github.com/thomassaison/mcp-code-graph/internal/web"
)
```

**Step 2: Commit**

```bash
git add cmd/mcp-code-graph/main.go
git commit -m "feat: add web server startup via MCP_CODE_GRAPH_WEB env var"
```

---

## Task 6: Create Frontend HTML

**Files:**
- Create: `internal/web/static/index.html`

**Step 1: Create index.html**

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MCP Code Graph Explorer</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <header>
        <h1>Code Graph Explorer</h1>
        <input type="search" id="search" placeholder="Search nodes...">
    </header>
    <main>
        <aside id="tree">
            <h2>Packages</h2>
            <div id="package-tree"></div>
        </aside>
        <section id="content">
            <div id="graph"></div>
            <div id="metadata">
                <h2>Node Details</h2>
                <div id="node-info">Select a node to view details</div>
            </div>
        </section>
    </main>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.28.1/cytoscape.min.js"></script>
    <script src="/static/app.js"></script>
</body>
</html>
```

**Step 2: Commit**

```bash
git add internal/web/static/
git commit -m "feat: add HTML template for web visualization"
```

---

## Task 7: Create CSS Styling

**Files:**
- Create: `internal/web/static/style.css`

**Step 1: Create style.css**

```css
* {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    background: #1a1a2e;
    color: #eee;
}

header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 2rem;
    background: #16213e;
    border-bottom: 1px solid #0f3460;
}

header h1 {
    font-size: 1.5rem;
    color: #e94560;
}

header input {
    padding: 0.5rem 1rem;
    border-radius: 4px;
    border: 1px solid #0f3460;
    background: #1a1a2e;
    color: #eee;
    width: 300px;
}

main {
    display: grid;
    grid-template-columns: 280px 1fr;
    height: calc(100vh - 60px);
}

#tree {
    background: #16213e;
    padding: 1rem;
    overflow-y: auto;
    border-right: 1px solid #0f3460;
}

#tree h2 {
    font-size: 1rem;
    margin-bottom: 1rem;
    color: #e94560;
}

#package-tree details {
    margin-bottom: 0.25rem;
}

#package-tree summary {
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
}

#package-tree summary:hover {
    background: #0f3460;
}

#package-tree .node {
    padding: 0.25rem 0.5rem 0.25rem 1.5rem;
    cursor: pointer;
    font-size: 0.875rem;
    border-radius: 4px;
}

#package-tree .node:hover {
    background: #0f3460;
}

#package-tree .node.selected {
    background: #e94560;
    color: white;
}

#content {
    display: grid;
    grid-template-rows: 1fr 200px;
    overflow: hidden;
}

#graph {
    background: #1a1a2e;
}

#metadata {
    background: #16213e;
    padding: 1rem;
    border-top: 1px solid #0f3460;
    overflow-y: auto;
}

#metadata h2 {
    font-size: 1rem;
    margin-bottom: 0.5rem;
    color: #e94560;
}

#node-info {
    font-size: 0.875rem;
}

#node-info .label {
    color: #888;
    margin-right: 0.5rem;
}

#node-info .value {
    color: #eee;
}

#node-info pre {
    background: #0f3460;
    padding: 0.5rem;
    border-radius: 4px;
    overflow-x: auto;
    margin-top: 0.5rem;
}
```

**Step 2: Commit**

```bash
git add internal/web/static/
git commit -m "feat: add CSS styling for web visualization"
```

---

## Task 8: Create JavaScript Application

**Files:**
- Create: `internal/web/static/app.js`

**Step 1: Create app.js with core functionality**

```javascript
let cy = null;
let selectedNode = null;

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', async () => {
    await loadPackages();
    initCytoscape();
    initSearch();
});

// Load packages and build tree
async function loadPackages() {
    const res = await fetch('/api/packages');
    const packages = await res.json();
    
    const tree = document.getElementById('package-tree');
    tree.innerHTML = '';
    
    for (const pkg of packages) {
        const details = document.createElement('details');
        const summary = document.createElement('summary');
        summary.textContent = pkg;
        details.appendChild(summary);
        
        summary.addEventListener('click', async (e) => {
            if (details.querySelector('.nodes')) return;
            await loadPackageNodes(pkg, details);
        });
        
        tree.appendChild(details);
    }
}

// Load nodes for a package
async function loadPackageNodes(pkg, container) {
    const res = await fetch(`/api/packages/${encodeURIComponent(pkg)}/nodes`);
    const nodes = await res.json();
    
    const div = document.createElement('div');
    div.className = 'nodes';
    
    for (const node of nodes) {
        const el = document.createElement('div');
        el.className = 'node';
        el.textContent = `${node.name} (${node.type})`;
        el.dataset.id = node.id;
        el.addEventListener('click', () => selectNode(node.id));
        div.appendChild(el);
    }
    
    container.appendChild(div);
}

// Initialize Cytoscape graph
function initCytoscape() {
    cy = cytoscape({
        container: document.getElementById('graph'),
        style: [
            {
                selector: 'node',
                style: {
                    'background-color': '#0f3460',
                    'label': 'data(name)',
                    'color': '#eee',
                    'font-size': 12,
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'width': 80,
                    'height': 80
                }
            },
            {
                selector: 'node.selected',
                style: {
                    'background-color': '#e94560',
                    'border-width': 2,
                    'border-color': '#fff'
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 2,
                    'line-color': '#0f3460',
                    'target-arrow-color': '#0f3460',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier'
                }
            }
        ],
        layout: {
            name: 'breadthfirst',
            directed: true
        }
    });
    
    cy.on('tap', 'node', (evt) => {
        selectNode(evt.target.id());
    });
}

// Select a node
async function selectNode(id) {
    // Update tree selection
    document.querySelectorAll('.node.selected').forEach(el => el.classList.remove('selected'));
    const nodeEl = document.querySelector(`.node[data-id="${id}"]`);
    if (nodeEl) nodeEl.classList.add('selected');
    
    // Load node details
    const res = await fetch(`/api/nodes/${encodeURIComponent(id)}`);
    if (!res.ok) return;
    const node = await res.json();
    
    // Update metadata
    updateMetadata(node);
    
    // Load neighborhood
    await loadNeighborhood(id);
}

// Load neighborhood graph
async function loadNeighborhood(id) {
    const depth = 1; // Could be configurable
    const res = await fetch(`/api/nodes/${encodeURIComponent(id)}/neighborhood?depth=${depth}`);
    if (!res.ok) return;
    const data = await res.json();
    
    // Update graph
    cy.elements().remove();
    
    cy.add(data.nodes.map(n => ({
        data: {
            id: n.id,
            name: n.name
        },
        classes: n.id === data.center.id ? 'selected' : ''
    })));
    
    cy.add(data.edges.map(e => ({
        data: {
            id: `${e.from}->${e.to}`,
            source: e.from,
            target: e.to,
            type: e.type
        }
    })));
    
    cy.layout({
        name: 'breadthfirst',
        directed: true,
        roots: `#${data.center.id}`
    }).run();
}

// Update metadata panel
function updateMetadata(node) {
    const info = document.getElementById('node-info');
    
    let html = `
        <div><span class="label">Name:</span><span class="value">${node.name}</span></div>
        <div><span class="label">Type:</span><span class="value">${node.type}</span></div>
        <div><span class="label">Package:</span><span class="value">${node.package}</span></div>
        <div><span class="label">File:</span><span class="value">${node.file}:${node.line}</span></div>
    `;
    
    if (node.signature) {
        html += `<div><span class="label">Signature:</span></div><pre>${escapeHtml(node.signature)}</pre>`;
    }
    
    if (node.docstring) {
        html += `<div><span class="label">Documentation:</span></div><pre>${escapeHtml(node.docstring)}</pre>`;
    }
    
    if (node.summary) {
        html += `<div><span class="label">Summary:</span><span class="value">${escapeHtml(node.summary)}</span></div>`;
    }
    
    if (node.behaviors && node.behaviors.length > 0) {
        html += `<div><span class="label">Behaviors:</span><span class="value">${node.behaviors.join(', ')}</span></div>`;
    }
    
    info.innerHTML = html;
}

// Initialize search
function initSearch() {
    const input = document.getElementById('search');
    let timeout = null;
    
    input.addEventListener('input', (e) => {
        clearTimeout(timeout);
        timeout = setTimeout(() => searchNodes(e.target.value), 300);
    });
}

// Search nodes
async function searchNodes(query) {
    if (!query) return;
    
    const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
    const nodes = await res.json();
    
    // Could display search results in a dropdown or highlight in tree
    if (nodes.length > 0) {
        selectNode(nodes[0].id);
    }
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
```

**Step 2: Commit**

```bash
git add internal/web/static/
git commit -m "feat: add JavaScript for web visualization"
```

---

## Task 9: Manual Testing and Polish

**Step 1: Build and run**

```bash
go build ./cmd/mcp-code-graph
MCP_CODE_GRAPH_WEB=:8080 ./mcp-code-graph
```

**Step 2: Test in browser**

Open http://localhost:8080

Verify:
- Package tree loads and expands
- Clicking node shows neighborhood graph
- Metadata panel updates
- Search works

**Step 3: Fix any issues**

Make any necessary fixes to CSS, JS, or handlers.

**Step 4: Final commit**

```bash
git add -A
git commit -m "fix: polish web visualization UI"
```

---

## Acceptance Criteria

- [ ] `go test ./internal/web/...` passes
- [ ] `MCP_CODE_GRAPH_WEB=:8080 mcp-code-graph` serves UI at localhost:8080
- [ ] Can browse packages in tree view
- [ ] Clicking node shows neighborhood in graph
- [ ] Metadata panel shows node details
- [ ] Search returns matching nodes
- [ ] ADR-0016 and design doc committed
