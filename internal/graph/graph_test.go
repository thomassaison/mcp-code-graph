package graph

import (
	"testing"
)

func TestGraphAddNode(t *testing.T) {
	g := New()
	node := &Node{
		Type:    NodeTypeFunction,
		Package: "main",
		Name:    "test",
		File:    "test.go",
		Line:    1,
	}
	node.ID = node.GenerateID()

	g.AddNode(node)

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount() = %d, want 1", g.NodeCount())
	}

	got, err := g.GetNode(node.ID)
	if err != nil {
		t.Fatalf("GetNode() error = %v", err)
	}
	if got.Name != node.Name {
		t.Errorf("GetNode().Name = %q, want %q", got.Name, node.Name)
	}
}

func TestGraphAddEdge(t *testing.T) {
	g := New()

	from := &Node{Type: NodeTypeFunction, Package: "main", Name: "caller", File: "test.go", Line: 1}
	from.ID = from.GenerateID()

	to := &Node{Type: NodeTypeFunction, Package: "main", Name: "callee", File: "test.go", Line: 5}
	to.ID = to.GenerateID()

	g.AddNode(from)
	g.AddNode(to)
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeCalls})

	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1", g.EdgeCount())
	}

	callers := g.GetCallers(to.ID)
	if len(callers) != 1 {
		t.Errorf("GetCallers() = %d nodes, want 1", len(callers))
	}
}

func TestGraphGetCallees(t *testing.T) {
	g := New()

	caller := &Node{Type: NodeTypeFunction, Package: "main", Name: "caller", File: "test.go", Line: 1}
	caller.ID = caller.GenerateID()

	callee := &Node{Type: NodeTypeFunction, Package: "main", Name: "callee", File: "test.go", Line: 5}
	callee.ID = callee.GenerateID()

	g.AddNode(caller)
	g.AddNode(callee)
	g.AddEdge(&Edge{From: caller.ID, To: callee.ID, Type: EdgeTypeCalls})

	callees := g.GetCallees(caller.ID)
	if len(callees) != 1 {
		t.Errorf("GetCallees() = %d nodes, want 1", len(callees))
	}
}

func TestGraphGetNodesByType(t *testing.T) {
	g := New()

	fn1 := &Node{Type: NodeTypeFunction, Package: "main", Name: "fn1", File: "test.go", Line: 1}
	fn1.ID = fn1.GenerateID()
	g.AddNode(fn1)

	fn2 := &Node{Type: NodeTypeFunction, Package: "main", Name: "fn2", File: "test.go", Line: 10}
	fn2.ID = fn2.GenerateID()
	g.AddNode(fn2)

	typ := &Node{Type: NodeTypeType, Package: "main", Name: "MyType", File: "test.go", Line: 20}
	g.AddNode(typ)

	functions := g.GetNodesByType(NodeTypeFunction)
	if len(functions) != 2 {
		t.Errorf("GetNodesByType(function) = %d nodes, want 2", len(functions))
	}

	functionIDs := make(map[string]bool)
	for _, f := range functions {
		functionIDs[f.ID] = true
	}
	if !functionIDs[fn1.ID] || !functionIDs[fn2.ID] {
		t.Error("GetNodesByType(function) missing expected nodes")
	}
}

func TestGraphGetNodesByPackage(t *testing.T) {
	g := New()

	fn1 := &Node{Type: NodeTypeFunction, Package: "main", Name: "fn1", File: "test.go", Line: 1}
	fn1.ID = fn1.GenerateID()
	g.AddNode(fn1)

	fn2 := &Node{Type: NodeTypeFunction, Package: "pkg", Name: "fn2", File: "pkg.go", Line: 1}
	fn2.ID = fn2.GenerateID()
	g.AddNode(fn2)

	mainNodes := g.GetNodesByPackage("main")
	if len(mainNodes) != 1 {
		t.Errorf("GetNodesByPackage(main) = %d nodes, want 1", len(mainNodes))
	}
}

func TestGraphRemoveNodesForPackage(t *testing.T) {
	g := New()

	fn1 := &Node{Type: NodeTypeFunction, Package: "main", Name: "fn1", File: "test.go", Line: 1}
	fn1.ID = fn1.GenerateID()
	g.AddNode(fn1)

	fn2 := &Node{Type: NodeTypeFunction, Package: "pkg", Name: "fn2", File: "pkg.go", Line: 1}
	fn2.ID = fn2.GenerateID()
	g.AddNode(fn2)

	g.AddEdge(&Edge{From: fn1.ID, To: fn2.ID, Type: EdgeTypeCalls})

	g.RemoveNodesForPackage("pkg")

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount() = %d, want 1", g.NodeCount())
	}

	if _, err := g.GetNode(fn2.ID); err == nil {
		t.Error("GetNode(fn2) should fail, node should be removed")
	}
}

func TestGraphAllNodes(t *testing.T) {
	g := New()

	fn1 := &Node{Type: NodeTypeFunction, Package: "main", Name: "fn1", File: "test.go", Line: 1}
	fn1.ID = fn1.GenerateID()
	g.AddNode(fn1)

	fn2 := &Node{Type: NodeTypeFunction, Package: "main", Name: "fn2", File: "test.go", Line: 10}
	fn2.ID = fn2.GenerateID()
	g.AddNode(fn2)

	all := g.AllNodes()
	if len(all) != 2 {
		t.Errorf("AllNodes() = %d nodes, want 2", len(all))
	}
}

func TestGraph_GetNodesByName(t *testing.T) {
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
	g.AddNode(&Node{
		ID:      "func_other_NewServer_test.go:15",
		Type:    NodeTypeFunction,
		Package: "other",
		Name:    "NewServer",
	})

	nodes := g.GetNodesByName("main")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

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

	nodes := g.GetNodesByNameAndPackage("main", "main")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Package != "main" {
		t.Fatalf("expected package 'main', got '%s'", nodes[0].Package)
	}

	nodes = g.GetNodesByNameAndPackage("main", "nonexistent")
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}

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

	implementors := g.byInterface[iface.ID]
	if len(implementors) != 1 {
		t.Fatalf("expected 1 implementor, got %d", len(implementors))
	}
	if implementors[0].ID != typ.ID {
		t.Errorf("expected implementor %s, got %s", typ.ID, implementors[0].ID)
	}

	interfaces := g.byTypeImpl[typ.ID]
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if interfaces[0].ID != iface.ID {
		t.Errorf("expected interface %s, got %s", iface.ID, interfaces[0].ID)
	}
}

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

func TestGraph_GetNodesByBehaviors(t *testing.T) {
	g := New()

	node1 := &Node{
		ID:      "func_test_Log_test.go:10",
		Type:    NodeTypeFunction,
		Package: "test",
		Name:    "Log",
		Metadata: map[string]any{
			"behaviors": []string{"logging", "error-handle"},
		},
	}

	node2 := &Node{
		ID:      "func_test_Handle_test.go:20",
		Type:    NodeTypeFunction,
		Package: "test",
		Name:    "Handle",
		Metadata: map[string]any{
			"behaviors": []string{"http-client", "error-handle"},
		},
	}

	node3 := &Node{
		ID:      "func_test_Process_test.go:30",
		Type:    NodeTypeFunction,
		Package: "test",
		Name:    "Process",
		Metadata: map[string]any{
			"behaviors": []string{"database"},
		},
	}

	g.AddNode(node1)
	g.AddNode(node2)
	g.AddNode(node3)

	tests := []struct {
		name      string
		behaviors []string
		wantCount int
	}{
		{
			name:      "single behavior",
			behaviors: []string{"logging"},
			wantCount: 1,
		},
		{
			name:      "multiple behaviors (AND)",
			behaviors: []string{"logging", "error-handle"},
			wantCount: 1,
		},
		{
			name:      "behavior not found",
			behaviors: []string{"file-io"},
			wantCount: 0,
		},
		{
			name:      "empty behaviors",
			behaviors: []string{},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := g.GetNodesByBehaviors(tt.behaviors)
			if len(nodes) != tt.wantCount {
				t.Errorf("GetNodesByBehaviors() returned %d nodes, want %d", len(nodes), tt.wantCount)
			}
		})
	}
}
