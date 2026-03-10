# Implicit Interface Resolver Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add implicit interface resolution to mcp-code-graph, enabling queries for which types implement an interface and which interfaces a type satisfies.

**Architecture:** Type checker runs as a separate pass after AST parsing. Uses `go/packages` with `NeedTypes | NeedImports | NeedDeps` to load type information including stdlib and external dependencies. Creates `implements` edges between types and interfaces.

**Tech Stack:** Go, `golang.org/x/tools/go/packages`, `go/types`

---

## Task 1: Add Method type and Methods field to Node

**Files:**
- Modify: `internal/graph/node.go`
- Test: `internal/graph/node_test.go`

**Step 1: Add Method type and Methods field to Node struct**

In `internal/graph/node.go`, add after the `Summary` struct:

```go
// Method represents a method signature for interfaces
type Method struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}
```

Then add `Methods` field to the `Node` struct:

```go
type Node struct {
	ID        string
	Type      NodeType
	Package   string
	Name      string
	File      string
	Line      int
	Column    int
	Signature string
	Docstring string
	Summary   *Summary
	Methods   []Method `json:"methods,omitempty"` // For interfaces: required method signatures
	Metadata  map[string]any
}
```

**Step 2: Write test for Node with Methods**

Create `internal/graph/node_test.go`:

```go
package graph

import (
	"testing"
)

func TestNodeWithMethods(t *testing.T) {
	node := &Node{
		ID:      "type_io.Reader",
		Type:    NodeTypeInterface,
		Package: "io",
		Name:    "Reader",
		Methods: []Method{
			{Name: "Read", Signature: "Read(p []byte) (n int, err error)"},
		},
	}

	if len(node.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(node.Methods))
	}
	if node.Methods[0].Name != "Read" {
		t.Errorf("expected method name Read, got %s", node.Methods[0].Name)
	}
}
```

**Step 3: Run test**

Run: `go test ./internal/graph/... -v -run TestNodeWithMethods`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/graph/node.go internal/graph/node_test.go
git commit -m "feat(graph): add Method type and Methods field for interface nodes"
```

---

## Task 2: Add implementation indexes to Graph

**Files:**
- Modify: `internal/graph/graph.go`
- Test: `internal/graph/graph_test.go`

**Step 1: Add byInterface and byTypeImpl indexes to Graph struct**

In `internal/graph/graph.go`, update the Graph struct:

```go
type Graph struct {
	mu sync.RWMutex

	nodes   map[string]*Node
	edges   map[string][]*Edge
	inEdges map[string][]*Edge

	byType    map[NodeType]map[string]*Node
	byPackage map[string]map[string]*Node
	byName    map[string]map[string]*Node

	// NEW: indexes for interface implementation lookups
	byInterface map[string][]*Node // interface ID -> implementing types
	byTypeImpl  map[string][]*Node // type ID -> implemented interfaces
}
```

**Step 2: Initialize new indexes in New()**

```go
func New() *Graph {
	return &Graph{
		nodes:       make(map[string]*Node),
		edges:       make(map[string][]*Edge),
		inEdges:     make(map[string][]*Edge),
		byType:      make(map[NodeType]map[string]*Node),
		byPackage:   make(map[string]map[string]*Node),
		byName:      make(map[string]map[string]*Node),
		byInterface: make(map[string][]*Node),
		byTypeImpl:  make(map[string][]*Node),
	}
}
```

**Step 3: Write test for new indexes**

Add to `internal/graph/graph_test.go`:

```go
func TestGraphImplementationIndexes(t *testing.T) {
	g := New()

	iface := &Node{
		ID:      "type_io.Reader",
		Type:    NodeTypeInterface,
		Package: "io",
		Name:    "Reader",
	}
	typ := &Node{
		ID:      "type_os.File",
		Type:    NodeTypeType,
		Package: "os",
		Name:    "File",
	}

	g.AddNode(iface)
	g.AddNode(typ)

	edge := &Edge{
		From: typ.ID,
		To:   iface.ID,
		Type: EdgeTypeImplements,
		Metadata: map[string]any{
			"pointer_receiver": true,
		},
	}
	g.AddEdge(edge)

	// Verify byInterface index
	implementors := g.byInterface[iface.ID]
	if len(implementors) != 1 {
		t.Fatalf("expected 1 implementor, got %d", len(implementors))
	}
	if implementors[0].ID != typ.ID {
		t.Errorf("expected implementor %s, got %s", typ.ID, implementors[0].ID)
	}

	// Verify byTypeImpl index
	interfaces := g.byTypeImpl[typ.ID]
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if interfaces[0].ID != iface.ID {
		t.Errorf("expected interface %s, got %s", iface.ID, interfaces[0].ID)
	}
}
```

**Step 4: Run test**

Run: `go test ./internal/graph/... -v -run TestGraphImplementationIndexes`
Expected: FAIL (indexes not populated in AddEdge yet)

**Step 5: Update AddEdge to populate implementation indexes**

Modify `AddEdge` in `internal/graph/graph.go`:

```go
func (g *Graph) AddEdge(edge *Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges[edge.From] = append(g.edges[edge.From], edge)
	g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)

	// Populate implementation indexes for implements edges
	if edge.Type == EdgeTypeImplements {
		if fromNode, ok := g.nodes[edge.From]; ok {
			g.byInterface[edge.To] = append(g.byInterface[edge.To], fromNode)
		}
		if toNode, ok := g.nodes[edge.To]; ok {
			g.byTypeImpl[edge.From] = append(g.byTypeImpl[edge.From], toNode)
		}
	}
}
```

**Step 6: Run test again**

Run: `go test ./internal/graph/... -v -run TestGraphImplementationIndexes`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/graph/graph.go internal/graph/graph_test.go
git commit -m "feat(graph): add implementation indexes for O(1) interface lookups"
```

---

## Task 3: Add GetImplementors and GetInterfaces methods to Graph

**Files:**
- Modify: `internal/graph/graph.go`
- Test: `internal/graph/graph_test.go`

**Step 1: Add GetImplementors method**

Add to `internal/graph/graph.go`:

```go
// GetImplementors returns all types that implement the given interface.
func (g *Graph) GetImplementors(interfaceID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.byInterface[interfaceID]
}
```

**Step 2: Add GetInterfaces method**

```go
// GetInterfaces returns all interfaces that the given type implements.
func (g *Graph) GetInterfaces(typeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.byTypeImpl[typeID]
}
```

**Step 3: Write tests for new methods**

Add to `internal/graph/graph_test.go`:

```go
func TestGraphGetImplementors(t *testing.T) {
	g := New()

	iface := &Node{ID: "type_io.Reader", Type: NodeTypeInterface, Package: "io", Name: "Reader"}
	typ1 := &Node{ID: "type_os.File", Type: NodeTypeType, Package: "os", Name: "File"}
	typ2 := &Node{ID: "type_bytes.Buffer", Type: NodeTypeType, Package: "bytes", Name: "Buffer"}

	g.AddNode(iface)
	g.AddNode(typ1)
	g.AddNode(typ2)

	g.AddEdge(&Edge{From: typ1.ID, To: iface.ID, Type: EdgeTypeImplements})
	g.AddEdge(&Edge{From: typ2.ID, To: iface.ID, Type: EdgeTypeImplements})

	implementors := g.GetImplementors(iface.ID)
	if len(implementors) != 2 {
		t.Fatalf("expected 2 implementors, got %d", len(implementors))
	}
}

func TestGraphGetInterfaces(t *testing.T) {
	g := New()

	iface1 := &Node{ID: "type_io.Reader", Type: NodeTypeInterface, Package: "io", Name: "Reader"}
	iface2 := &Node{ID: "type_io.Writer", Type: NodeTypeInterface, Package: "io", Name: "Writer"}
	typ := &Node{ID: "type_os.File", Type: NodeTypeType, Package: "os", Name: "File"}

	g.AddNode(iface1)
	g.AddNode(iface2)
	g.AddNode(typ)

	g.AddEdge(&Edge{From: typ.ID, To: iface1.ID, Type: EdgeTypeImplements})
	g.AddEdge(&Edge{From: typ.ID, To: iface2.ID, Type: EdgeTypeImplements})

	interfaces := g.GetInterfaces(typ.ID)
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/graph/... -v -run "TestGraphGetImplementors|TestGraphGetInterfaces"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/graph.go internal/graph/graph_test.go
git commit -m "feat(graph): add GetImplementors and GetInterfaces query methods"
```

---

## Task 4: Create internal/types package with Checker

**Files:**
- Create: `internal/types/checker.go`
- Create: `internal/types/checker_test.go`

**Step 1: Create checker.go**

Create `internal/types/checker.go`:

```go
package types

import (
	"fmt"
	"go/token"
	"go/types"
	"log/slog"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"golang.org/x/tools/go/packages"
)

// Checker performs type checking to extract interfaces and implementations.
type Checker struct {
	fset *token.FileSet
}

// CheckResult contains the extracted interfaces, types, and implementation edges.
type CheckResult struct {
	Interfaces []*graph.Node
	Types      []*graph.Node
	Edges      []*graph.Edge
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	return &Checker{
		fset: token.NewFileSet(),
	}
}

// Check performs type checking on the module at root and extracts interface information.
func (c *Checker) Check(root string) (*CheckResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedSyntax | packages.NeedTypes |
			packages.NeedImports | packages.NeedDeps,
		Dir:  root,
		Fset: c.fset,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	// Check for package errors
	var hasErrors bool
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			slog.Warn("package error", "pkg", pkg.PkgPath, "error", err)
			hasErrors = true
		}
	})

	result := &CheckResult{
		Interfaces: make([]*graph.Node, 0),
		Types:      make([]*graph.Node, 0),
		Edges:      make([]*graph.Edge, 0),
	}

	// Collect all interfaces and types
	interfaces := make(map[string]*types.Interface)
	allTypes := make(map[string]types.Type)

	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)

			// Extract interfaces
			if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
				node := c.interfaceToNode(pkg.PkgPath, name, iface)
				result.Interfaces = append(result.Interfaces, node)
				interfaces[node.ID] = iface
			}

			// Extract named types (structs, type aliases, etc.)
			if _, ok := obj.(*types.TypeName); ok {
				if !types.IsInterface(obj.Type()) {
					node := c.typeToNode(pkg.PkgPath, name, obj.Type())
					result.Types = append(result.Types, node)
					allTypes[node.ID] = obj.Type()
				}
			}
		}
	})

	// Find implementations
	for typeID, typ := range allTypes {
		for ifaceID, iface := range interfaces {
			// Check if T implements interface
			if types.Implements(typ, iface) {
				result.Edges = append(result.Edges, &graph.Edge{
					From: typeID,
					To:   ifaceID,
					Type: graph.EdgeTypeImplements,
					Metadata: map[string]any{
						"pointer_receiver": false,
					},
				})
			}
			// Check if *T implements interface
			ptrType := types.NewPointer(typ)
			if types.Implements(ptrType, iface) {
				result.Edges = append(result.Edges, &graph.Edge{
					From: typeID,
					To:   ifaceID,
					Type: graph.EdgeTypeImplements,
					Metadata: map[string]any{
						"pointer_receiver": true,
					},
				})
			}
		}
	}

	slog.Info("type check complete", "interfaces", len(result.Interfaces), "types", len(result.Types), "edges", len(result.Edges))
	return result, nil
}

func (c *Checker) interfaceToNode(pkgPath, name string, iface *types.Interface) *graph.Node {
	methods := make([]graph.Method, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		methods[i] = graph.Method{
			Name:      m.Name(),
			Signature: m.Type().String(),
		}
	}

	return &graph.Node{
		ID:      fmt.Sprintf("type_%s.%s", pkgPath, name),
		Type:    graph.NodeTypeInterface,
		Package: pkgPath,
		Name:    name,
		Methods: methods,
	}
}

func (c *Checker) typeToNode(pkgPath, name string, typ types.Type) *graph.Node {
	kind := "type"
	switch typ.Underlying().(type) {
	case *types.Struct:
		kind = "struct"
	}

	return &graph.Node{
		ID:       fmt.Sprintf("type_%s.%s", pkgPath, name),
		Type:     graph.NodeTypeType,
		Package:  pkgPath,
		Name:     name,
		Metadata: map[string]any{"kind": kind},
	}
}
```

**Step 2: Write basic test**

Create `internal/types/checker_test.go`:

```go
package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckerCheck(t *testing.T) {
	// Create a temporary test module
	tmpDir := t.TempDir()

	goMod := `module testpkg

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	goCode := `package testpkg

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type File struct{}

func (f *File) Read(p []byte) (n int, err error)  { return 0, nil }
func (f *File) Write(p []byte) (n int, err error) { return 0, nil }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	checker := NewChecker()
	result, err := checker.Check(tmpDir)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should find 2 interfaces
	if len(result.Interfaces) < 2 {
		t.Errorf("expected at least 2 interfaces, got %d", len(result.Interfaces))
	}

	// Should find File type
	if len(result.Types) < 1 {
		t.Errorf("expected at least 1 type, got %d", len(result.Types))
	}

	// Should find implementations (File implements Reader and Writer via pointer)
	if len(result.Edges) < 2 {
		t.Errorf("expected at least 2 implementation edges, got %d", len(result.Edges))
	}

	t.Logf("Found %d interfaces, %d types, %d edges", len(result.Interfaces), len(result.Types), len(result.Edges))
}
```

**Step 3: Run test**

Run: `go test ./internal/types/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/types/checker.go internal/types/checker_test.go
git commit -m "feat(types): add type checker for interface extraction"
```

---

## Task 5: Integrate type checker into indexer

**Files:**
- Modify: `internal/indexer/indexer.go`

**Step 1: Add type checking pass to indexer**

Read `internal/indexer/indexer.go` and add import:

```go
import (
	// ... existing imports
	"github.com/thomassaison/mcp-code-graph/internal/types"
)
```

Then modify the `Reindex` method to add type checking after AST parsing:

```go
func (i *Indexer) Reindex(ctx context.Context) error {
	// ... existing AST parsing code ...

	// NEW: Run type checker for interface resolution
	checker := types.NewChecker()
	result, err := checker.Check(i.projectDir)
	if err != nil {
		// Log warning but don't fail - AST data is still valid
		slog.Warn("type check failed, continuing without interface data", "error", err)
	} else {
		// Add interface and type nodes
		for _, node := range result.Interfaces {
			i.graph.AddNode(node)
		}
		for _, node := range result.Types {
			i.graph.AddNode(node)
		}
		// Add implementation edges
		for _, edge := range result.Edges {
			i.graph.AddEdge(edge)
		}
	}

	// ... persist ...
}
```

**Step 2: Run existing tests**

Run: `go test ./internal/indexer/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/indexer/indexer.go
git commit -m "feat(indexer): integrate type checker for interface resolution"
```

---

## Task 6: Add get_implementors MCP tool

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

**Step 1: Add get_implementors handler**

In `internal/mcp/tools.go`, add:

```go
func (s *Server) handleGetImplementors(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	interfaceID, _ := args["interface_id"].(string)
	if interfaceID == "" {
		return nil, fmt.Errorf("interface_id is required")
	}

	includePointer := true
	if v, ok := args["include_pointer"].(bool); ok {
		includePointer = v
	}

	// Get the interface node
	ifaceNode, err := s.graph.GetNode(interfaceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("interface not found: %s", interfaceID)), nil
	}

	// Get implementors
	implementors := s.graph.GetImplementors(interfaceID)

	// Filter by pointer_receiver if needed
	result := map[string]interface{}{
		"interface": map[string]interface{}{
			"id":      ifaceNode.ID,
			"name":    ifaceNode.Name,
			"package": ifaceNode.Package,
			"methods": ifaceNode.Methods,
		},
		"implementors": make([]map[string]interface{}, 0),
	}

	implList := make([]map[string]interface{}, 0)
	for _, impl := range implementors {
		implList = append(implList, map[string]interface{}{
			"id":      impl.ID,
			"name":    impl.Name,
			"package": impl.Package,
			"kind":    impl.Metadata["kind"],
		})
	}
	result["implementors"] = implList

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
```

**Step 2: Register tool in server.go**

In `internal/mcp/server.go`, add to the tools list in `Start()`:

```go
s.mcpServer.AddTools(
	// ... existing tools ...
	mcp.NewTool("get_implementors",
		mcp.WithDescription("Find all types that implement a given interface"),
		mcp.WithString("interface_id", mcp.Required(), mcp.Description("Interface to query (e.g., io.Reader)")),
		mcp.WithBoolean("include_pointer", mcp.Description("Include pointer variants (default: true)")),
	),
	mcp.NewTool("get_interfaces",
		mcp.WithDescription("Find all interfaces that a type implements"),
		mcp.WithString("type_id", mcp.Required(), mcp.Description("Type to query (e.g., os.File)")),
	),
)
```

And add handler:

```go
s.mcpServer.AddToolHandler("get_implementors", s.handleGetImplementors)
s.mcpServer.AddToolHandler("get_interfaces", s.handleGetInterfaces)
```

**Step 3: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/server.go
git commit -m "feat(mcp): add get_implementors tool"
```

---

## Task 7: Add get_interfaces MCP tool

**Files:**
- Modify: `internal/mcp/tools.go`
- Test: `internal/mcp/server_test.go`

**Step 1: Add get_interfaces handler**

In `internal/mcp/tools.go`, add:

```go
func (s *Server) handleGetInterfaces(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	typeID, _ := args["type_id"].(string)
	if typeID == "" {
		return nil, fmt.Errorf("type_id is required")
	}

	// Get the type node
	typeNode, err := s.graph.GetNode(typeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("type not found: %s", typeID)), nil
	}

	// Get implemented interfaces
	interfaces := s.graph.GetInterfaces(typeID)

	result := map[string]interface{}{
		"type": map[string]interface{}{
			"id":      typeNode.ID,
			"name":    typeNode.Name,
			"package": typeNode.Package,
			"kind":    typeNode.Metadata["kind"],
		},
		"interfaces": make([]map[string]interface{}, 0),
	}

	ifaceList := make([]map[string]interface{}, 0)
	for _, iface := range interfaces {
		// Find the edge to get pointer_receiver metadata
		pointerReceiver := false
		for _, edge := range s.graph.GetEdgesFrom(typeID) {
			if edge.To == iface.ID && edge.Type == graph.EdgeTypeImplements {
				if pr, ok := edge.Metadata["pointer_receiver"].(bool); ok {
					pointerReceiver = pr
				}
				break
			}
		}

		ifaceList = append(ifaceList, map[string]interface{}{
			"id":               iface.ID,
			"name":             iface.Name,
			"package":          iface.Package,
			"pointer_receiver": pointerReceiver,
		})
	}
	result["interfaces"] = ifaceList

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
```

**Step 2: Add GetEdgesFrom helper to Graph**

In `internal/graph/graph.go`:

```go
// GetEdgesFrom returns all edges from the given node.
func (g *Graph) GetEdgesFrom(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edges[nodeID]
}
```

**Step 3: Commit**

```bash
git add internal/mcp/tools.go internal/graph/graph.go
git commit -m "feat(mcp): add get_interfaces tool"
```

---

## Task 8: Write integration tests

**Files:**
- Modify: `internal/mcp/server_test.go`

**Step 1: Add integration test**

```go
func TestGetImplementorsTool(t *testing.T) {
	s := NewServer(testGraph(t))

	// Add test data
	iface := &graph.Node{
		ID:      "type_io.Reader",
		Type:    graph.NodeTypeInterface,
		Package: "io",
		Name:    "Reader",
		Methods: []graph.Method{{Name: "Read", Signature: "Read(p []byte) (n int, err error)"}},
	}
	typ := &graph.Node{
		ID:       "type_os.File",
		Type:     graph.NodeTypeType,
		Package:  "os",
		Name:     "File",
		Metadata: map[string]any{"kind": "struct"},
	}

	s.graph.AddNode(iface)
	s.graph.AddNode(typ)
	s.graph.AddEdge(&graph.Edge{
		From:     typ.ID,
		To:       iface.ID,
		Type:     graph.EdgeTypeImplements,
		Metadata: map[string]any{"pointer_receiver": true},
	})

	result, err := s.handleGetImplementors(context.Background(), map[string]interface{}{
		"interface_id": "type_io.Reader",
	})
	if err != nil {
		t.Fatalf("handleGetImplementors failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	implementors := data["implementors"].([]interface{})
	if len(implementors) != 1 {
		t.Errorf("expected 1 implementor, got %d", len(implementors))
	}
}

func TestGetInterfacesTool(t *testing.T) {
	s := NewServer(testGraph(t))

	// Add test data
	iface := &graph.Node{
		ID:      "type_io.Reader",
		Type:    graph.NodeTypeInterface,
		Package: "io",
		Name:    "Reader",
	}
	typ := &graph.Node{
		ID:       "type_os.File",
		Type:     graph.NodeTypeType,
		Package:  "os",
		Name:     "File",
		Metadata: map[string]any{"kind": "struct"},
	}

	s.graph.AddNode(iface)
	s.graph.AddNode(typ)
	s.graph.AddEdge(&graph.Edge{
		From:     typ.ID,
		To:       iface.ID,
		Type:     graph.EdgeTypeImplements,
		Metadata: map[string]any{"pointer_receiver": true},
	})

	result, err := s.handleGetInterfaces(context.Background(), map[string]interface{}{
		"type_id": "type_os.File",
	})
	if err != nil {
		t.Fatalf("handleGetInterfaces failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	interfaces := data["interfaces"].([]interface{})
	if len(interfaces) != 1 {
		t.Errorf("expected 1 interface, got %d", len(interfaces))
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/mcp/... -v -run "TestGetImplementorsTool|TestGetInterfacesTool"`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/mcp/server_test.go
git commit -m "test(mcp): add integration tests for interface tools"
```

---

## Task 9: Update documentation

**Files:**
- Modify: `README.md`
- Create: `adr/0014-implicit-interface-resolver.md`
- Modify: `adr/README.md`

**Step 1: Update README MCP Tools table**

Add the new tools to the table:

```markdown
| `get_implementors` | Find all types that implement an interface |
| `get_interfaces` | Find all interfaces a type implements |
```

**Step 2: Create ADR**

Create `adr/0014-implicit-interface-resolver.md`:

```markdown
# 14. Implicit Interface Resolver

Date: 2026-03-10

## Status

Accepted

## Context

Go interfaces are satisfied implicitly — there is no explicit declaration that a type implements an interface. This makes it difficult for AI assistants to discover which types can be used where an interface is expected.

When an AI assistant sees a function parameter of type `io.Reader`, it cannot easily find all possible types that could be passed to that function without analyzing the entire codebase and its dependencies.

## Decision

Implement implicit interface resolution using Go's type checker (`go/types`). The type checker runs as a separate pass after AST parsing and:

1. Extracts all interface definitions (including stdlib and external packages)
2. Extracts all named types
3. Uses `types.Implements()` to detect which types satisfy which interfaces
4. Creates `implements` edges in the graph

Provide two MCP tools:
- `get_implementors(interface_id)` — find types that implement an interface
- `get_interfaces(type_id)` — find interfaces a type implements

## Consequences

**Positive:**
- AI assistants can discover all implementations of any interface
- Supports stdlib interfaces (`io.Reader`, `http.Handler`, etc.)
- Tracks pointer receiver distinction (`*T` vs `T`)

**Negative:**
- Type checking adds overhead to indexing
- Full reindex required for type information updates
- External dependencies must be available for complete resolution
```

**Step 3: Update ADR index**

Add to `adr/README.md`:

```markdown
| [0014](0014-implicit-interface-resolver.md) | Implicit Interface Resolver | Accepted |
```

**Step 4: Commit**

```bash
git add README.md adr/0014-implicit-interface-resolver.md adr/README.md
git commit -m "docs: add documentation for implicit interface resolver"
```

---

## Task 10: Run full test suite and verify

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint` or `golangci-lint run`
Expected: No errors

**Step 3: Build and test locally**

Run: `make build && make run`
Test with an actual Go project to verify the tools work.

**Step 4: Push changes**

```bash
git push
```

---

## Summary

This plan implements the implicit interface resolver feature in 10 tasks:

1. Add `Method` type and `Methods` field to Node
2. Add `byInterface` and `byTypeImpl` indexes to Graph
3. Add `GetImplementors` and `GetInterfaces` query methods
4. Create type checker package with interface extraction
5. Integrate type checker into indexer
6. Add `get_implementors` MCP tool
7. Add `get_interfaces` MCP tool
8. Write integration tests
9. Update documentation
10. Verify and push
