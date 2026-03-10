# get_function_by_name Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a new MCP tool `get_function_by_name` for O(1) exact name lookups.

**Architecture:** Add `byName` index to the Graph struct, implement indexed lookup methods, and expose via a new MCP tool.

**Tech Stack:** Go, mcp-go library

---

### Task 1: Add byName Index to Graph

**Files:**
- Modify: `internal/graph/graph.go`
- Test: `internal/graph/graph_test.go`

**Step 1: Write the failing test**

Add to `internal/graph/graph_test.go`:

```go
func TestGraph_GetNodesByName(t *testing.T) {
	g := New()
	
	// Add nodes with same name in different packages
	g.AddNode(&Node{
		ID:      "func_main_main_test.go:10",
		Type:    NodeTypeFunction,
		Package: "main",
		Name:    "main",
	})
	g.AddNode(&Node{
		ID:      "func_cmd_main_test.go:5",
		Type:    NodeTypeFunction,
		Package: "cmd",
		Name:    "main",
	})
	g.AddNode(&Node{
		ID:      "func_other_NewServer_test.go:15",
		Type:    NodeTypeFunction,
		Package: "other",
		Name:    "NewServer",
	})

	// Test GetNodesByName
	nodes := g.GetNodesByName("main")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// Verify both nodes are returned
	names := make(map[string]bool)
	for _, n := range nodes {
		names[n.Package] = true
	}
	if !names["main"] || !names["cmd"] {
		t.Fatal("expected nodes from main and cmd packages")
	}

	// Test no match
	nodes = g.GetNodesByName("NonExistent")
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestGraph_GetNodesByNameAndPackage(t *testing.T) {
	g := New()
	
	g.AddNode(&Node{
		ID:      "func_main_main_test.go:10",
		Type:    NodeTypeFunction,
		Package: "main",
		Name:    "main",
	})
	g.AddNode(&Node{
		ID:      "func_cmd_main_test.go:5",
		Type:    NodeTypeFunction,
		Package: "cmd",
		Name:    "main",
	})

	// Test exact match
	nodes := g.GetNodesByNameAndPackage("main", "main")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Package != "main" {
		t.Fatalf("expected package 'main', got '%s'", nodes[0].Package)
	}

	// Test no match
	nodes = g.GetNodesByNameAndPackage("main", "nonexistent")
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/...`
Expected: FAIL - `GetNodesByName` and `GetNodesByNameAndPackage` not defined

**Step 3: Implement the index and methods**

Modify `internal/graph/graph.go`:

```go
// Add to Graph struct
type Graph struct {
	mu sync.RWMutex

	nodes   map[string]*Node
	edges   map[string][]*Edge
	inEdges map[string][]*Edge

	byType    map[NodeType]map[string]*Node
	byPackage map[string]map[string]*Node
	byName    map[string]map[string]*Node  // NEW: name -> id -> Node
}

// Update New()
func New() *Graph {
	return &Graph{
		nodes:     make(map[string]*Node),
		edges:     make(map[string][]*Edge),
		inEdges:   make(map[string][]*Edge),
		byType:    make(map[NodeType]map[string]*Node),
		byPackage: make(map[string]map[string]*Node),
		byName:    make(map[string]map[string]*Node),  // NEW
	}
}

// Update AddNode()
func (g *Graph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node

	if g.byType[node.Type] == nil {
		g.byType[node.Type] = make(map[string]*Node)
	}
	g.byType[node.Type][node.ID] = node

	if g.byPackage[node.Package] == nil {
		g.byPackage[node.Package] = make(map[string]*Node)
	}
	g.byPackage[node.Package][node.ID] = node

	// NEW: Add to byName index
	if g.byName[node.Name] == nil {
		g.byName[node.Name] = make(map[string]*Node)
	}
	g.byName[node.Name][node.ID] = node
}

// NEW: GetNodesByName returns all functions with the given name
func (g *Graph) GetNodesByName(name string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byName[name] {
		nodes = append(nodes, node)
	}
	return nodes
}

// NEW: GetNodesByNameAndPackage returns functions matching both name and package
func (g *Graph) GetNodesByNameAndPackage(name, pkg string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byName[name] {
		if node.Package == pkg {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/graph.go internal/graph/graph_test.go
git commit -m "feat(graph): add byName index for O(1) name lookups"
```

---

### Task 2: Add MCP Tool Handler

**Files:**
- Modify: `internal/mcp/tools.go`
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

Add to `internal/mcp/server_test.go`:

```go
func TestServer_GetFunctionByNameTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test nodes - two functions named "Handle" in different packages
	srv.graph.AddNode(&graph.Node{
		ID:        "func_handler_Handle_test.go:10",
		Name:      "Handle",
		Package:   "handler",
		Type:      graph.NodeTypeFunction,
		Signature: "func Handle(ctx context.Context) error",
		File:      "handler/test.go",
		Line:      10,
	})
	srv.graph.AddNode(&graph.Node{
		ID:        "func_service_Handle_test.go:20",
		Name:      "Handle",
		Package:   "service",
		Type:      graph.NodeTypeFunction,
		Signature: "func Handle(req *Request) (*Response, error)",
		File:      "service/test.go",
		Line:      20,
	})

	// Test 1: Find by name only - should return both
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_function_by_name"
	req.Params.Arguments = map[string]interface{}{
		"name": "Handle",
	}

	result, err := srv.handleGetFunctionByNameMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetFunctionByNameMCP: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	// Should contain both functions
	if !strings.Contains(textContent.Text, `"package": "handler"`) {
		t.Fatal("expected handler package in result")
	}
	if !strings.Contains(textContent.Text, `"package": "service"`) {
		t.Fatal("expected service package in result")
	}

	// Test 2: Find by name and package - should return one
	req2 := mcp.CallToolRequest{}
	req2.Params.Name = "get_function_by_name"
	req2.Params.Arguments = map[string]interface{}{
		"name":    "Handle",
		"package": "service",
	}

	result2, err := srv.handleGetFunctionByNameMCP(context.Background(), req2)
	if err != nil {
		t.Fatalf("handleGetFunctionByNameMCP: %v", err)
	}

	textContent2, ok := result2.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result2.Content[0])
	}

	// Should only contain service package
	if !strings.Contains(textContent2.Text, `"package": "service"`) {
		t.Fatal("expected service package in result")
	}
	if strings.Contains(textContent2.Text, `"package": "handler"`) {
		t.Fatal("should not contain handler package in result")
	}

	t.Logf("Result: %s", textContent2.Text)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/...`
Expected: FAIL - `handleGetFunctionByNameMCP` not defined

**Step 3: Implement the tool**

Add to `internal/mcp/tools.go`:

```go
// Add to GetTools() function
{
    Name:        "get_function_by_name",
    Description: "Find functions by exact name match",
    Parameters: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "name": map[string]any{
                "type":        "string",
                "description": "Function name to search for",
            },
            "package": map[string]any{
                "type":        "string",
                "description": "Filter by package name (optional)",
            },
            "file": map[string]any{
                "type":        "string",
                "description": "Filter by file path substring (optional)",
            },
        },
        "required": []string{"name"},
    },
    Handler: s.handleGetFunctionByName,
},

// Add new handler function
func (s *Server) handleGetFunctionByName(ctx context.Context, args map[string]any) (string, error) {
    name, ok := args["name"].(string)
    if !ok || name == "" {
        return "", fmt.Errorf("name must be a non-empty string")
    }

    pkg, _ := args["package"].(string)
    file, _ := args["file"].(string)

    var nodes []*graph.Node
    if pkg != "" {
        nodes = s.graph.GetNodesByNameAndPackage(name, pkg)
    } else {
        nodes = s.graph.GetNodesByName(name)
    }

    // Optional file filter
    if file != "" {
        filtered := nodes[:0]
        for _, n := range nodes {
            if strings.Contains(n.File, file) {
                filtered = append(filtered, n)
            }
        }
        nodes = filtered
    }

    var results []map[string]any
    for _, n := range nodes {
        results = append(results, map[string]any{
            "id":        n.ID,
            "name":      n.Name,
            "package":   n.Package,
            "signature": n.Signature,
            "file":      n.File,
            "line":      n.Line,
            "docstring": n.Docstring,
            "summary":   n.SummaryText(),
        })
    }

    data, err := json.MarshalIndent(results, "", "  ")
    if err != nil {
        return "", err
    }
    return string(data), nil
}

// Add MCP handler method
func (s *Server) handleGetFunctionByNameMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    name, err := req.RequireString("name")
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    args := map[string]any{
        "name": name,
    }

    if pkg, err := req.GetString("package", ""); err == nil && pkg != "" {
        args["package"] = pkg
    }
    if file, err := req.GetString("file", ""); err == nil && file != "" {
        args["file"] = file
    }

    result, err := s.handleGetFunctionByName(ctx, args)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    return mcp.NewToolResultText(result), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/server_test.go
git commit -m "feat(mcp): add get_function_by_name tool"
```

---

### Task 3: Register Tool and Update README

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `README.md`

**Step 1: Register the tool**

Update `internal/mcp/server.go` in `RegisterTools`:

```go
func (s *Server) RegisterTools(mcpServer *mcpserver.MCPServer) {
    s.addSearchFunctionsTool(mcpServer)
    s.addGetCallersTool(mcpServer)
    s.addGetCalleesTool(mcpServer)
    s.addReindexProjectTool(mcpServer)
    s.addUpdateSummaryTool(mcpServer)
    s.addGetFunctionByNameTool(mcpServer)  // NEW
}
```

Add helper in `internal/mcp/tools.go`:

```go
func (s *Server) addGetFunctionByNameTool(mcpServer *mcpserver.MCPServer) {
    tool := mcp.NewTool("get_function_by_name",
        mcp.WithDescription("Find functions by exact name match"),
        mcp.WithString("name",
            mcp.Required(),
            mcp.Description("Function name to search for"),
        ),
        mcp.WithString("package",
            mcp.Description("Filter by package name (optional)"),
        ),
        mcp.WithString("file",
            mcp.Description("Filter by file path substring (optional)"),
        ),
    )
    mcpServer.AddTool(tool, s.handleGetFunctionByNameMCP)
}
```

**Step 2: Update README**

Add to MCP Tools table in `README.md`:

```markdown
| `get_function_by_name` | Find functions by exact name |
```

**Step 3: Run tests**

Run: `go test ./...`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools.go README.md
git commit -m "docs: register get_function_by_name tool and update README"
```

---

### Task 4: Final Verification

**Step 1: Build and test**

Run: `go build ./... && go test ./...`
Expected: All PASS

**Step 2: Manual test**

Run the server and test the tool:
```bash
go run ./cmd/mcp-code-graph
```

The new tool should be available for MCP clients.

---

## Summary

- Added `byName` index for O(1) name lookups
- New `GetNodesByName` and `GetNodesByNameAndPackage` methods
- New `get_function_by_name` MCP tool
- Optional filtering by package and file
