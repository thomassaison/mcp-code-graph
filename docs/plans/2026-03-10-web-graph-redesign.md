# Web Graph Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rework web visualization into a full-page graph view with all nodes/edges visible, tooltips on hover, and edge type toggles.

**Architecture:** Keep existing Go API handlers, add `/api/graph` endpoint returning all nodes+edges. Replace frontend HTML/CSS/JS to be graph-centric with Cytoscape.js full-page, tippy.js tooltips, and floating edge toggle panel.

**Tech Stack:** Go stdlib, Cytoscape.js + cytoscape-popper + tippy.js (CDN), vanilla JS

---

## Task 1: Add AllEdges Method to Graph

**Files:**
- Modify: `internal/graph/graph.go`
- Modify: `internal/graph/graph_test.go`

**Step 1: Write failing test**

Add to `internal/graph/graph_test.go`:

```go
func TestAllEdges(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImplements})

	edges := g.AllEdges()
	assert.Len(t, edges, 2)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/... -run TestAllEdges -v`
Expected: FAIL - method doesn't exist

**Step 3: Implement AllEdges**

Add to `internal/graph/graph.go` after `AllNodes()`:

```go
func (g *Graph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var edges []*Edge
	for _, edgeList := range g.edges {
		edges = append(edges, edgeList...)
	}
	return edges
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/... -run TestAllEdges -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/
git commit -m "feat: add AllEdges method to Graph"
```

---

## Task 2: Add /api/graph Endpoint

**Files:**
- Modify: `internal/web/api.go`
- Modify: `internal/web/handler.go`
- Modify: `internal/web/api_test.go`

**Step 1: Write failing test**

Add to `internal/web/api_test.go`:

```go
func TestHandleGraph(t *testing.T) {
	h := NewHandler(newTestGraph())
	req := httptest.NewRequest("GET", "/api/graph", nil)
	rec := httptest.NewRecorder()

	h.handleGraph(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp GraphResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 3, len(resp.Nodes))  // f1, f2, t1 from newTestGraph
	assert.Equal(t, 1, len(resp.Edges))  // f1->f2
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestHandleGraph -v`
Expected: FAIL - type/method doesn't exist

**Step 3: Add GraphResponse type and handler**

Add to `internal/web/api.go` after `ErrorResponse`:

```go
type GraphResponse struct {
	Nodes []NodeResponse `json:"nodes"`
	Edges []EdgeResponse `json:"edges"`
}
```

Add handler:

```go
func (h *Handler) handleGraph(w http.ResponseWriter, r *http.Request) {
	allNodes := h.graph.AllNodes()
	allEdges := h.graph.AllEdges()

	nodes := make([]NodeResponse, 0, len(allNodes))
	for _, n := range allNodes {
		resp := NodeResponse{
			ID:        n.ID,
			Name:      n.Name,
			Type:      string(n.Type),
			Package:   n.Package,
			File:      n.File,
			Line:      n.Line,
			Signature: n.Signature,
			Docstring: n.Docstring,
		}
		if n.Summary != nil {
			resp.Summary = n.Summary.Text
		}
		if n.Metadata != nil {
			if beh, ok := n.Metadata["behaviors"].([]string); ok {
				resp.Behaviors = beh
			}
		}
		for _, m := range n.Methods {
			resp.Methods = append(resp.Methods, MethodResp{
				Name:      m.Name,
				Signature: m.Signature,
			})
		}
		nodes = append(nodes, resp)
	}

	edges := make([]EdgeResponse, 0, len(allEdges))
	for _, e := range allEdges {
		edges = append(edges, EdgeResponse{
			From: e.From,
			To:   e.To,
			Type: string(e.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GraphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}
```

**Step 4: Add route to handler.go**

In `internal/web/handler.go`, add before the `/api/packages` case:

```go
case path == "/api/graph":
    h.handleGraph(w, r)
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/web/... -run TestHandleGraph -v`
Expected: PASS

**Step 6: Also add routing test**

Update `TestServeHTTP_Routing` in `api_test.go` — add entry:

```go
{"/api/graph", http.StatusOK},
```

**Step 7: Run all web tests**

Run: `go test ./internal/web/... -v`
Expected: All PASS

**Step 8: Commit**

```bash
git add internal/web/
git commit -m "feat: add /api/graph endpoint returning all nodes and edges"
```

---

## Task 3: Replace Frontend — HTML

**Files:**
- Modify: `internal/web/static/index.html`

**Step 1: Replace index.html**

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Code Graph Explorer</title>
    <link rel="stylesheet" href="/style.css">
    <link rel="stylesheet" href="https://unpkg.com/tippy.js@6/themes/light-border.css">
</head>
<body>
    <header>
        <h1>Code Graph Explorer</h1>
        <div id="search-container">
            <input type="search" id="search" placeholder="Search nodes..." autocomplete="off">
            <div id="search-results"></div>
        </div>
    </header>
    <div id="graph-wrapper">
        <div id="graph"></div>
        <div id="edge-toggles">
            <h3>Edge Types</h3>
            <label><input type="checkbox" data-edge="calls" checked> calls</label>
            <label><input type="checkbox" data-edge="implements" checked> implements</label>
            <label><input type="checkbox" data-edge="uses" checked> uses</label>
            <label><input type="checkbox" data-edge="returns" checked> returns</label>
            <label><input type="checkbox" data-edge="accepts" checked> accepts</label>
            <label><input type="checkbox" data-edge="embeds" checked> embeds</label>
            <label><input type="checkbox" data-edge="defines" checked> defines</label>
            <label><input type="checkbox" data-edge="imports" checked> imports</label>
        </div>
        <div id="loading">Loading graph...</div>
    </div>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.28.1/cytoscape.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="https://unpkg.com/tippy.js@6"></script>
    <script src="https://unpkg.com/cytoscape-popper@2.0.0/cytoscape-popper.js"></script>
    <script src="/app.js"></script>
</body>
</html>
```

**Step 2: Commit**

```bash
git add internal/web/static/index.html
git commit -m "feat: replace HTML with graph-first layout"
```

---

## Task 4: Replace Frontend — CSS

**Files:**
- Modify: `internal/web/static/style.css`

**Step 1: Replace style.css**

```css
* { box-sizing: border-box; margin: 0; padding: 0; }

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    background: #1a1a2e;
    color: #eee;
    overflow: hidden;
}

header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 1rem;
    background: #16213e;
    border-bottom: 1px solid #0f3460;
    height: 44px;
    z-index: 10;
}

header h1 {
    font-size: 1rem;
    color: #e94560;
    white-space: nowrap;
}

#search-container { position: relative; }

#search {
    padding: 0.3rem 0.6rem;
    border-radius: 4px;
    border: 1px solid #0f3460;
    background: #1a1a2e;
    color: #eee;
    width: 260px;
    font-size: 0.8125rem;
}

#search:focus { outline: none; border-color: #e94560; }

#search-results {
    display: none;
    position: absolute;
    top: 100%;
    right: 0;
    width: 300px;
    max-height: 280px;
    overflow-y: auto;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 0 0 4px 4px;
    z-index: 200;
}

#search-results.visible { display: block; }

#search-results .result {
    padding: 0.35rem 0.6rem;
    cursor: pointer;
    font-size: 0.8125rem;
    border-bottom: 1px solid #0f3460;
}

#search-results .result:hover { background: #0f3460; }
#search-results .result-type { color: #888; font-size: 0.75rem; margin-left: 0.4rem; }

#graph-wrapper {
    position: relative;
    height: calc(100vh - 44px);
}

#graph {
    width: 100%;
    height: 100%;
}

#edge-toggles {
    position: absolute;
    top: 0.75rem;
    left: 0.75rem;
    background: rgba(22, 33, 62, 0.92);
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    border: 1px solid #0f3460;
    z-index: 100;
    font-size: 0.75rem;
}

#edge-toggles h3 {
    font-size: 0.6875rem;
    color: #e94560;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 0.4rem;
}

#edge-toggles label {
    display: block;
    padding: 0.15rem 0;
    cursor: pointer;
    color: #ccc;
}

#edge-toggles label:hover { color: #fff; }

#edge-toggles input[type="checkbox"] {
    margin-right: 0.4rem;
    accent-color: #e94560;
}

#loading {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: #888;
    font-size: 1rem;
}

#loading.hidden { display: none; }

/* Tippy tooltip styling */
.tippy-box[data-theme~='graph'] {
    background: #16213e;
    color: #eee;
    border: 1px solid #0f3460;
    font-size: 0.8125rem;
    max-width: 360px;
}

.tippy-box[data-theme~='graph'] .tippy-arrow { color: #0f3460; }

.tooltip-header {
    font-weight: 700;
    font-size: 0.875rem;
    margin-bottom: 0.25rem;
}

.tooltip-type {
    display: inline-block;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-left: 0.4rem;
}

.tooltip-type.function { background: #0f3460; color: #53a8e2; }
.tooltip-type.method { background: #1a3a5c; color: #7bc4f0; }
.tooltip-type.type { background: #2d1b4e; color: #b48ede; }
.tooltip-type.interface { background: #1b4332; color: #74c69d; }
.tooltip-type.package { background: #3d2e14; color: #e9c46a; }

.tooltip-file { color: #888; font-size: 0.75rem; margin-bottom: 0.3rem; }

.tooltip-sig {
    background: #0f3460;
    padding: 0.3rem 0.4rem;
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.75rem;
    margin: 0.3rem 0;
    overflow-x: auto;
    white-space: pre;
}

.tooltip-summary { margin-top: 0.3rem; line-height: 1.3; }

.tooltip-behaviors {
    margin-top: 0.3rem;
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
}

.tooltip-behavior {
    background: #0f3460;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
    font-size: 0.6875rem;
    color: #53a8e2;
}
```

**Step 2: Commit**

```bash
git add internal/web/static/style.css
git commit -m "feat: replace CSS with graph-first styling"
```

---

## Task 5: Replace Frontend — JavaScript

**Files:**
- Modify: `internal/web/static/app.js`

**Step 1: Replace app.js**

```javascript
let cy = null;
let allData = null;

document.addEventListener('DOMContentLoaded', async () => {
    initCytoscape();
    initEdgeToggles();
    initSearch();
    await loadGraph();
});

// --- Load Full Graph ---

async function loadGraph() {
    const loading = document.getElementById('loading');
    try {
        const res = await fetch('/api/graph');
        if (!res.ok) throw new Error('Failed to load graph');
        allData = await res.json();
        renderGraph(allData);
    } catch (err) {
        loading.textContent = 'Failed to load graph: ' + err.message;
        return;
    }
    loading.classList.add('hidden');
}

function renderGraph(data) {
    cy.elements().remove();

    for (const n of data.nodes) {
        cy.add({
            data: {
                id: 'n-' + n.id,
                nodeId: n.id,
                name: n.name,
                type: n.type,
                pkg: n.package,
                file: n.file,
                line: n.line,
                signature: n.signature || '',
                docstring: n.docstring || '',
                summary: n.summary || '',
                behaviors: n.behaviors || [],
                methods: n.methods || []
            }
        });
    }

    for (const e of data.edges) {
        cy.add({
            data: {
                id: 'e-' + e.from + '-' + e.to + '-' + e.type,
                source: 'n-' + e.from,
                target: 'n-' + e.to,
                edgeType: e.type
            }
        });
    }

    applyEdgeFilters();

    cy.layout({
        name: 'cose',
        animate: false,
        nodeDimensionsIncludeLabels: true,
        nodeRepulsion: function() { return 8000; },
        idealEdgeLength: function() { return 120; },
        gravity: 0.3,
        numIter: 300
    }).run();

    cy.fit(undefined, 40);
    setupTooltips();
}

// --- Cytoscape Init ---

function initCytoscape() {
    cy = cytoscape({
        container: document.getElementById('graph'),
        style: [
            {
                selector: 'node',
                style: {
                    'label': 'data(name)',
                    'color': '#ccc',
                    'font-size': 10,
                    'text-valign': 'bottom',
                    'text-margin-y': 5,
                    'width': 30,
                    'height': 30,
                    'border-width': 2
                }
            },
            { selector: 'node[type="function"]', style: { 'background-color': '#0f3460', 'border-color': '#53a8e2' } },
            { selector: 'node[type="method"]', style: { 'background-color': '#1a3a5c', 'border-color': '#7bc4f0' } },
            { selector: 'node[type="type"]', style: { 'background-color': '#2d1b4e', 'border-color': '#b48ede' } },
            { selector: 'node[type="interface"]', style: { 'background-color': '#1b4332', 'border-color': '#74c69d' } },
            { selector: 'node[type="package"]', style: { 'background-color': '#3d2e14', 'border-color': '#e9c46a' } },
            { selector: 'node[type="file"]', style: { 'background-color': '#3d1414', 'border-color': '#e07676' } },
            {
                selector: 'node.highlighted',
                style: {
                    'border-width': 4,
                    'border-color': '#e94560',
                    'font-weight': 'bold',
                    'color': '#fff',
                    'z-index': 10
                }
            },
            {
                selector: 'node.neighbor',
                style: {
                    'border-width': 3,
                    'opacity': 1,
                    'z-index': 5
                }
            },
            {
                selector: 'node.dimmed',
                style: { 'opacity': 0.15 }
            },
            {
                selector: 'edge',
                style: {
                    'width': 1.5,
                    'line-color': '#2a3f5f',
                    'target-arrow-color': '#2a3f5f',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'arrow-scale': 0.7,
                    'opacity': 0.6
                }
            },
            { selector: 'edge[edgeType="calls"]', style: { 'line-color': '#53a8e2', 'target-arrow-color': '#53a8e2' } },
            { selector: 'edge[edgeType="implements"]', style: { 'line-color': '#74c69d', 'target-arrow-color': '#74c69d', 'line-style': 'dashed' } },
            { selector: 'edge[edgeType="uses"]', style: { 'line-color': '#888', 'target-arrow-color': '#888', 'line-style': 'dotted' } },
            { selector: 'edge[edgeType="returns"]', style: { 'line-color': '#666', 'target-arrow-color': '#666', 'width': 1 } },
            { selector: 'edge[edgeType="accepts"]', style: { 'line-color': '#666', 'target-arrow-color': '#666', 'width': 1 } },
            { selector: 'edge[edgeType="embeds"]', style: { 'line-color': '#b48ede', 'target-arrow-color': '#b48ede', 'line-style': 'dashed' } },
            { selector: 'edge[edgeType="defines"]', style: { 'line-color': '#e9c46a', 'target-arrow-color': '#e9c46a', 'width': 1 } },
            { selector: 'edge[edgeType="imports"]', style: { 'line-color': '#555', 'target-arrow-color': '#555', 'width': 1, 'line-style': 'dotted' } },
            { selector: 'edge.highlighted', style: { 'opacity': 1, 'width': 2.5, 'z-index': 10 } },
            { selector: 'edge.dimmed', style: { 'opacity': 0.05 } }
        ],
        layout: { name: 'preset' },
        minZoom: 0.1,
        maxZoom: 4,
        wheelSensitivity: 0.3
    });

    // Click node: highlight neighborhood
    cy.on('tap', 'node', (evt) => {
        highlightNode(evt.target);
    });

    // Click background: reset
    cy.on('tap', (evt) => {
        if (evt.target === cy) resetHighlight();
    });

    // Double-click: copy file:line
    cy.on('dbltap', 'node', (evt) => {
        const d = evt.target.data();
        if (d.file) {
            navigator.clipboard.writeText(d.file + ':' + d.line);
        }
    });
}

// --- Highlight ---

function highlightNode(node) {
    cy.elements().removeClass('highlighted neighbor dimmed');

    const neighborhood = node.neighborhood().add(node);
    const others = cy.elements().difference(neighborhood);

    node.addClass('highlighted');
    node.neighborhood().nodes().addClass('neighbor');
    node.connectedEdges().addClass('highlighted');
    others.addClass('dimmed');
}

function resetHighlight() {
    cy.elements().removeClass('highlighted neighbor dimmed');
}

// --- Tooltips ---

function setupTooltips() {
    if (!cy.nodes().length) return;

    cy.nodes().forEach((node) => {
        const ref = node.popperRef();
        const d = node.data();

        const content = buildTooltipContent(d);

        tippy(ref, {
            content: content,
            trigger: 'manual',
            placement: 'right',
            allowHTML: true,
            theme: 'graph',
            interactive: false,
            appendTo: document.body,
            arrow: true
        });

        node.on('mouseover', () => {
            const tip = node.tippy;
            if (!tip) {
                node.tippy = node.popperRef()._tippy;
            }
            if (node.popperRef()._tippy) {
                node.popperRef()._tippy.show();
            }
        });

        node.on('mouseout', () => {
            if (node.popperRef()._tippy) {
                node.popperRef()._tippy.hide();
            }
        });
    });
}

function buildTooltipContent(d) {
    let html = '<div class="tooltip-header">' + esc(d.name) +
        '<span class="tooltip-type ' + d.type + '">' + d.type + '</span></div>';

    if (d.file) {
        html += '<div class="tooltip-file">' + esc(d.file) + ':' + d.line + '</div>';
    }

    if (d.signature) {
        html += '<div class="tooltip-sig">' + esc(d.signature) + '</div>';
    }

    if (d.summary) {
        html += '<div class="tooltip-summary">' + esc(d.summary) + '</div>';
    }

    if (d.behaviors && d.behaviors.length) {
        html += '<div class="tooltip-behaviors">';
        for (const b of d.behaviors) {
            html += '<span class="tooltip-behavior">' + esc(b) + '</span>';
        }
        html += '</div>';
    }

    return html;
}

// --- Edge Toggles ---

function initEdgeToggles() {
    document.querySelectorAll('#edge-toggles input[type="checkbox"]').forEach((cb) => {
        cb.addEventListener('change', applyEdgeFilters);
    });
}

function applyEdgeFilters() {
    const visible = new Set();
    document.querySelectorAll('#edge-toggles input:checked').forEach((cb) => {
        visible.add(cb.dataset.edge);
    });

    cy.edges().forEach((edge) => {
        const type = edge.data('edgeType');
        if (visible.has(type)) {
            edge.show();
        } else {
            edge.hide();
        }
    });
}

// --- Search ---

function initSearch() {
    const input = document.getElementById('search');
    const results = document.getElementById('search-results');
    let timeout = null;

    input.addEventListener('input', () => {
        clearTimeout(timeout);
        const q = input.value.trim();
        if (!q) {
            results.classList.remove('visible');
            resetHighlight();
            return;
        }
        timeout = setTimeout(() => searchNodes(q), 200);
    });

    input.addEventListener('blur', () => {
        setTimeout(() => results.classList.remove('visible'), 200);
    });

    input.addEventListener('focus', () => {
        if (results.children.length > 0 && input.value.trim()) {
            results.classList.add('visible');
        }
    });

    input.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            input.value = '';
            results.classList.remove('visible');
            resetHighlight();
        }
    });
}

async function searchNodes(query) {
    const res = await fetch('/api/search?q=' + encodeURIComponent(query));
    if (!res.ok) return;
    const nodes = await res.json();

    const results = document.getElementById('search-results');
    results.innerHTML = '';

    if (nodes.length === 0) {
        results.innerHTML = '<div class="result" style="color:#888">No results</div>';
        results.classList.add('visible');
        return;
    }

    for (const node of nodes) {
        const el = document.createElement('div');
        el.className = 'result';
        el.innerHTML = '<span>' + esc(node.name) + '</span><span class="result-type">' + node.type + '</span>';
        el.addEventListener('mousedown', (e) => {
            e.preventDefault();
            results.classList.remove('visible');
            focusOnNode(node.id);
        });
        results.appendChild(el);
    }

    results.classList.add('visible');
}

function focusOnNode(nodeId) {
    const node = cy.getElementById('n-' + nodeId);
    if (node.length) {
        resetHighlight();
        highlightNode(node);
        cy.animate({
            center: { eles: node },
            zoom: 2,
            duration: 400
        });
    }
}

// --- Utility ---

function esc(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
```

**Step 2: Commit**

```bash
git add internal/web/static/app.js
git commit -m "feat: replace JS with full-graph visualization"
```

---

## Task 6: Verify Build and Tests

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS (including new TestHandleGraph)

**Step 2: Build**

Run: `go build -o bin/mcp-code-graph ./cmd/mcp-code-graph`
Expected: Build succeeds

**Step 3: Commit any remaining changes**

```bash
git add -A
git commit -m "chore: polish web graph redesign"
```

---

## Acceptance Criteria

- [ ] `go test ./...` passes (including TestHandleGraph)
- [ ] `MCP_CODE_GRAPH_WEB=:8080 bin/mcp-code-graph` serves full-graph UI
- [ ] All nodes visible on page load with force-directed layout
- [ ] Hovering a node shows tooltip with name, type, file, signature, summary
- [ ] Edge toggle checkboxes hide/show edge types
- [ ] Click node highlights its neighborhood
- [ ] Search highlights and centers matching nodes
- [ ] All edge types visible and color-coded
