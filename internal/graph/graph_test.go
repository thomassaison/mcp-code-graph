package graph

import (
	"fmt"
	"sort"
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

func TestGraphAddEdge_Idempotent(t *testing.T) {
	g := New()

	from := &Node{Type: NodeTypeFunction, Package: "main", Name: "caller", File: "test.go", Line: 1}
	from.ID = from.GenerateID()
	to := &Node{Type: NodeTypeFunction, Package: "main", Name: "callee", File: "test.go", Line: 5}
	to.ID = to.GenerateID()

	g.AddNode(from)
	g.AddNode(to)

	// Add the same edge three times
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeCalls})

	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1 (duplicate edges should be ignored)", g.EdgeCount())
	}

	callers := g.GetCallers(to.ID)
	if len(callers) != 1 {
		t.Errorf("GetCallers() = %d nodes, want 1", len(callers))
	}

	callees := g.GetCallees(from.ID)
	if len(callees) != 1 {
		t.Errorf("GetCallees() = %d nodes, want 1", len(callees))
	}
}

func TestGraphAddEdge_DifferentTypes(t *testing.T) {
	g := New()

	from := &Node{Type: NodeTypeFunction, Package: "main", Name: "caller", File: "test.go", Line: 1}
	from.ID = from.GenerateID()
	to := &Node{Type: NodeTypeFunction, Package: "main", Name: "callee", File: "test.go", Line: 5}
	to.ID = to.GenerateID()

	g.AddNode(from)
	g.AddNode(to)

	// Different edge types between the same pair should both be kept
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeUses})

	if g.EdgeCount() != 2 {
		t.Errorf("EdgeCount() = %d, want 2 (different types should be kept)", g.EdgeCount())
	}
}

func TestRemoveNodesForFile(t *testing.T) {
	g := New()

	// Two functions in same package but different files
	fn1 := &Node{ID: "func_pkg_fn1_a.go:1", Type: NodeTypeFunction, Package: "pkg", Name: "fn1", File: "a.go", Line: 1}
	fn2 := &Node{ID: "func_pkg_fn2_a.go:10", Type: NodeTypeFunction, Package: "pkg", Name: "fn2", File: "a.go", Line: 10}
	fn3 := &Node{ID: "func_pkg_fn3_b.go:1", Type: NodeTypeFunction, Package: "pkg", Name: "fn3", File: "b.go", Line: 1}

	g.AddNode(fn1)
	g.AddNode(fn2)
	g.AddNode(fn3)
	g.AddEdge(&Edge{From: fn1.ID, To: fn3.ID, Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: fn3.ID, To: fn2.ID, Type: EdgeTypeCalls})

	// Remove only file a.go — fn3 in b.go should survive
	g.RemoveNodesForFile("a.go")

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount() = %d, want 1", g.NodeCount())
	}
	if _, err := g.GetNode(fn3.ID); err != nil {
		t.Errorf("fn3 should still exist: %v", err)
	}
	if _, err := g.GetNode(fn1.ID); err == nil {
		t.Error("fn1 should be removed")
	}
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (all edges touched deleted nodes)", g.EdgeCount())
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

func TestAllPackages(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "func1", Type: NodeTypeFunction, Package: "pkg1", Name: "Func1"})
	g.AddNode(&Node{ID: "func2", Type: NodeTypeFunction, Package: "pkg2", Name: "Func2"})
	g.AddNode(&Node{ID: "func3", Type: NodeTypeFunction, Package: "pkg1", Name: "Func3"})

	pkgs := g.AllPackages()
	sort.Strings(pkgs)
	if len(pkgs) != 2 {
		t.Errorf("AllPackages() = %d packages, want 2", len(pkgs))
	}
	if pkgs[0] != "pkg1" || pkgs[1] != "pkg2" {
		t.Errorf("AllPackages() = %v, want [pkg1, pkg2]", pkgs)
	}
}

func TestGetNeighborhood(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeCalls})

	nodes, edges := g.GetNeighborhood("b", 1)
	if len(nodes) != 3 {
		t.Errorf("GetNeighborhood(b, 1) nodes = %d, want 3", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("GetNeighborhood(b, 1) edges = %d, want 2", len(edges))
	}

	nodes, edges = g.GetNeighborhood("b", 2)
	if len(nodes) != 3 {
		t.Errorf("GetNeighborhood(b, 2) nodes = %d, want 3", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("GetNeighborhood(b, 2) edges = %d, want 2", len(edges))
	}
}

func TestAllEdges(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImplements})

	edges := g.AllEdges()
	if len(edges) != 2 {
		t.Errorf("AllEdges() = %d, want 2", len(edges))
	}
}

func TestGraphReplaceAll_populatedGraph(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "old1", Type: NodeTypeFunction, Package: "pkg", Name: "Old1"})
	g.AddNode(&Node{ID: "old2", Type: NodeTypeFunction, Package: "pkg", Name: "Old2"})
	g.AddEdge(&Edge{From: "old1", To: "old2", Type: EdgeTypeCalls})

	if g.NodeCount() != 2 {
		t.Fatalf("pre-replace NodeCount() = %d, want 2", g.NodeCount())
	}

	other := New()
	other.AddNode(&Node{ID: "new1", Type: NodeTypeFunction, Package: "newpkg", Name: "New1"})
	other.AddNode(&Node{ID: "new2", Type: NodeTypeFunction, Package: "newpkg", Name: "New2"})
	other.AddNode(&Node{ID: "new3", Type: NodeTypeFunction, Package: "newpkg", Name: "New3"})
	other.AddEdge(&Edge{From: "new1", To: "new2", Type: EdgeTypeCalls})

	g.ReplaceAll(other)

	if g.NodeCount() != 3 {
		t.Errorf("post-replace NodeCount() = %d, want 3", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Errorf("post-replace EdgeCount() = %d, want 1", g.EdgeCount())
	}

	if _, err := g.GetNode("new1"); err != nil {
		t.Error("new1 should be accessible after ReplaceAll")
	}
	if _, err := g.GetNode("old1"); err == nil {
		t.Error("old1 should not be accessible after ReplaceAll")
	}
}

func TestGraphReplaceAll_emptyGraph(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "n1", Type: NodeTypeFunction, Package: "p", Name: "N"})

	other := New()
	g.ReplaceAll(other)

	if g.NodeCount() != 0 {
		t.Errorf("post-replace with empty, NodeCount() = %d, want 0", g.NodeCount())
	}
}

func TestGraphReplaceAll_preservesPointer(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "old", Type: NodeTypeFunction, Package: "p", Name: "Old"})

	other := New()
	other.AddNode(&Node{ID: "new", Type: NodeTypeFunction, Package: "p", Name: "New"})

	g.ReplaceAll(other)

	if _, err := g.GetNode("new"); err != nil {
		t.Error("data from other graph should be accessible via original receiver")
	}
}

func TestGraphReplaceAll_replacesIndexes(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "func_old_F", Type: NodeTypeFunction, Package: "old", Name: "F"})
	g.AddNode(&Node{ID: "type_old_T", Type: NodeTypeType, Package: "old", Name: "T"})

	other := New()
	other.AddNode(&Node{ID: "func_new_G", Type: NodeTypeFunction, Package: "new", Name: "G"})
	other.AddNode(&Node{ID: "type_new_U", Type: NodeTypeType, Package: "new", Name: "U"})

	g.ReplaceAll(other)

	pkgs := g.AllPackages()
	if len(pkgs) != 1 || pkgs[0] != "new" {
		t.Errorf("AllPackages() = %v, want [new]", pkgs)
	}

	nodes := g.GetNodesByPackage("old")
	if len(nodes) != 0 {
		t.Error("old package should be gone after ReplaceAll")
	}
}

func TestRemoveNodesForPackage_CleansInEdges(t *testing.T) {
	g := New()

	// Foo (pkg1) calls Bar (pkg2) - when we remove pkg1,
	// Bar's inEdges should be cleaned up
	g.AddNode(&Node{ID: "func_pkg1_Foo", Type: NodeTypeFunction, Package: "pkg1", Name: "Foo"})
	g.AddNode(&Node{ID: "func_pkg2_Bar", Type: NodeTypeFunction, Package: "pkg2", Name: "Bar"})

	// Foo calls Bar - so Bar has an incoming edge from Foo
	g.AddEdge(&Edge{From: "func_pkg1_Foo", To: "func_pkg2_Bar", Type: EdgeTypeCalls})

	g.RemoveNodesForPackage("pkg1")

	_, err := g.GetNode("func_pkg2_Bar")
	if err != nil {
		t.Fatalf("Bar should still exist: %v", err)
	}

	// Bar's inEdges should be cleaned up - no ghost edge from deleted Foo
	g.mu.RLock()
	inEdges := g.inEdges["func_pkg2_Bar"]
	g.mu.RUnlock()

	if len(inEdges) != 0 {
		t.Errorf("inEdges for Bar should be empty, got %d edges", len(inEdges))
		for _, e := range inEdges {
			t.Errorf("  ghost edge: %s -> %s", e.From, e.To)
		}
	}
}

// ---- GetImpact tests ----

func TestGetImpact_CallChain(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeCalls})

	report := g.GetImpact("c")

	if report.NodeID != "c" {
		t.Errorf("NodeID = %q, want %q", report.NodeID, "c")
	}
	if len(report.DirectCallers) != 1 || report.DirectCallers[0].ID != "b" {
		t.Errorf("DirectCallers = %v, want [b]", report.DirectCallers)
	}
	if len(report.IndirectCallers) != 1 || report.IndirectCallers[0].ID != "a" {
		t.Errorf("IndirectCallers = %v, want [a]", report.IndirectCallers)
	}
	if report.TotalReach != 2 {
		t.Errorf("TotalReach = %d, want 2", report.TotalReach)
	}
}

func TestGetImpact_TestFunctionInChain(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "fn", Type: NodeTypeFunction, Package: "pkg", Name: "Fn"})
	g.AddNode(&Node{ID: "test_fn", Type: NodeTypeFunction, Package: "pkg", Name: "TestFn", File: "fn_test.go"})
	g.AddEdge(&Edge{From: "test_fn", To: "fn", Type: EdgeTypeCalls})

	report := g.GetImpact("fn")

	if len(report.Tests) != 1 || report.Tests[0].ID != "test_fn" {
		t.Errorf("Tests = %v, want [test_fn]", report.Tests)
	}
	if len(report.DirectCallers) != 1 {
		t.Errorf("DirectCallers = %d, want 1", len(report.DirectCallers))
	}
}

func TestGetImpact_RiskLevel(t *testing.T) {
	// low: < 5 callers
	g := New()
	g.AddNode(&Node{ID: "target", Type: NodeTypeFunction, Package: "pkg", Name: "Target"})
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("caller%d", i)
		g.AddNode(&Node{ID: id, Type: NodeTypeFunction, Package: "pkg", Name: id})
		g.AddEdge(&Edge{From: id, To: "target", Type: EdgeTypeCalls})
	}
	report := g.GetImpact("target")
	if report.RiskLevel != "low" {
		t.Errorf("RiskLevel = %q, want %q", report.RiskLevel, "low")
	}

	// medium: >= 5 callers, < 20
	g2 := New()
	g2.AddNode(&Node{ID: "target", Type: NodeTypeFunction, Package: "pkg", Name: "Target"})
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("caller%d", i)
		g2.AddNode(&Node{ID: id, Type: NodeTypeFunction, Package: "pkg", Name: id})
		g2.AddEdge(&Edge{From: id, To: "target", Type: EdgeTypeCalls})
	}
	report2 := g2.GetImpact("target")
	if report2.RiskLevel != "medium" {
		t.Errorf("RiskLevel = %q, want %q", report2.RiskLevel, "medium")
	}

	// high: >= 20 callers
	g3 := New()
	g3.AddNode(&Node{ID: "target", Type: NodeTypeFunction, Package: "pkg", Name: "Target"})
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("caller%d", i)
		g3.AddNode(&Node{ID: id, Type: NodeTypeFunction, Package: "pkg", Name: id})
		g3.AddEdge(&Edge{From: id, To: "target", Type: EdgeTypeCalls})
	}
	report3 := g3.GetImpact("target")
	if report3.RiskLevel != "high" {
		t.Errorf("RiskLevel = %q, want %q", report3.RiskLevel, "high")
	}
}

func TestGetImpact_NonExistentNode(t *testing.T) {
	g := New()
	report := g.GetImpact("nonexistent")

	if report.NodeID != "nonexistent" {
		t.Errorf("NodeID = %q, want %q", report.NodeID, "nonexistent")
	}
	if len(report.DirectCallers) != 0 {
		t.Errorf("DirectCallers = %d, want 0", len(report.DirectCallers))
	}
	if len(report.IndirectCallers) != 0 {
		t.Errorf("IndirectCallers = %d, want 0", len(report.IndirectCallers))
	}
	if report.TotalReach != 0 {
		t.Errorf("TotalReach = %d, want 0", report.TotalReach)
	}
}

// ---- FindPath tests ----

func TestFindPath_ShortestPath(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeCalls})

	path, err := g.FindPath("a", "c", 0)
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}
	if len(path) != 3 {
		t.Fatalf("FindPath() path len = %d, want 3", len(path))
	}
	if path[0].ID != "a" || path[1].ID != "b" || path[2].ID != "c" {
		t.Errorf("FindPath() path = [%s, %s, %s], want [a, b, c]", path[0].ID, path[1].ID, path[2].ID)
	}
}

func TestFindPath_SameNode(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})

	path, err := g.FindPath("a", "a", 0)
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}
	if len(path) != 1 || path[0].ID != "a" {
		t.Errorf("FindPath(same) = %v, want [a]", path)
	}
}

func TestFindPath_NoPath(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})

	_, err := g.FindPath("a", "b", 0)
	if err != ErrNoPath {
		t.Errorf("FindPath() error = %v, want ErrNoPath", err)
	}
}

func TestFindPath_MaxDepth(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "c", Type: NodeTypeFunction, Package: "pkg", Name: "C"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeCalls})

	// maxDepth=1 should not reach c (path a->b->c is depth 2)
	_, err := g.FindPath("a", "c", 1)
	if err != ErrNoPath {
		t.Errorf("FindPath(maxDepth=1) error = %v, want ErrNoPath", err)
	}

	// maxDepth=2 should find the path
	path, err := g.FindPath("a", "c", 2)
	if err != nil {
		t.Fatalf("FindPath(maxDepth=2) error = %v", err)
	}
	if len(path) != 3 {
		t.Errorf("FindPath(maxDepth=2) path len = %d, want 3", len(path))
	}
}

func TestFindPath_NonExistentNode(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})

	_, err := g.FindPath("a", "nonexistent", 0)
	if err != ErrNodeNotFound {
		t.Errorf("FindPath(missing to) error = %v, want ErrNodeNotFound", err)
	}

	_, err = g.FindPath("nonexistent", "a", 0)
	if err != ErrNodeNotFound {
		t.Errorf("FindPath(missing from) error = %v, want ErrNodeNotFound", err)
	}
}

// ---- GetContract tests ----

func TestGetContract_CallerCalleeCounts(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "fn", Type: NodeTypeFunction, Package: "pkg", Name: "Fn"})
	g.AddNode(&Node{ID: "caller1", Type: NodeTypeFunction, Package: "pkg", Name: "Caller1"})
	g.AddNode(&Node{ID: "caller2", Type: NodeTypeFunction, Package: "pkg", Name: "Caller2"})
	g.AddNode(&Node{ID: "callee1", Type: NodeTypeFunction, Package: "pkg", Name: "Callee1"})
	g.AddEdge(&Edge{From: "caller1", To: "fn", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "caller2", To: "fn", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "fn", To: "callee1", Type: EdgeTypeCalls})

	contract := g.GetContract("fn")
	if contract == nil {
		t.Fatal("GetContract() = nil, want non-nil")
	}
	if contract.CallerCount != 2 {
		t.Errorf("CallerCount = %d, want 2", contract.CallerCount)
	}
	if contract.CalleeCount != 1 {
		t.Errorf("CalleeCount = %d, want 1", contract.CalleeCount)
	}
}

func TestGetContract_MethodWithInterface(t *testing.T) {
	g := New()

	// Type node for receiver
	typeNode := &Node{ID: "type_pkg_MyType", Type: NodeTypeType, Package: "pkg", Name: "MyType"}
	g.AddNode(typeNode)

	// Interface that MyType implements
	ifaceNode := &Node{ID: "type_pkg_MyInterface", Type: NodeTypeInterface, Package: "pkg", Name: "MyInterface"}
	g.AddNode(ifaceNode)

	// Add implements edge: MyType -> MyInterface
	g.AddEdge(&Edge{From: typeNode.ID, To: ifaceNode.ID, Type: EdgeTypeImplements})

	// Method on MyType
	method := &Node{
		ID:      "func_pkg_DoSomething_file.go:10",
		Type:    NodeTypeMethod,
		Package: "pkg",
		Name:    "DoSomething",
		Metadata: map[string]any{
			"receiver": "MyType",
		},
	}
	g.AddNode(method)

	contract := g.GetContract(method.ID)
	if contract == nil {
		t.Fatal("GetContract() = nil, want non-nil")
	}
	if contract.ReceiverType != "MyType" {
		t.Errorf("ReceiverType = %q, want %q", contract.ReceiverType, "MyType")
	}
	if len(contract.TypeInterfaces) != 1 || contract.TypeInterfaces[0].ID != ifaceNode.ID {
		t.Errorf("TypeInterfaces = %v, want [%s]", contract.TypeInterfaces, ifaceNode.ID)
	}
}

func TestGetContract_NonExistentNode(t *testing.T) {
	g := New()
	contract := g.GetContract("nonexistent")
	if contract != nil {
		t.Errorf("GetContract(nonexistent) = %v, want nil", contract)
	}
}

func TestGetContract_TestFunctions(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "fn", Type: NodeTypeFunction, Package: "pkg", Name: "Fn"})
	g.AddNode(&Node{ID: "test_fn", Type: NodeTypeFunction, Package: "pkg", Name: "TestFn", File: "fn_test.go"})
	g.AddEdge(&Edge{From: "test_fn", To: "fn", Type: EdgeTypeCalls})

	contract := g.GetContract("fn")
	if contract == nil {
		t.Fatal("GetContract() = nil, want non-nil")
	}
	if len(contract.TestFunctions) != 1 || contract.TestFunctions[0].ID != "test_fn" {
		t.Errorf("TestFunctions = %v, want [test_fn]", contract.TestFunctions)
	}
}

// ---- DiscoverPatterns tests ----

func TestDiscoverPatterns_Constructors(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "new_foo", Type: NodeTypeFunction, Package: "pkg", Name: "NewFoo"})
	g.AddNode(&Node{ID: "new_bar", Type: NodeTypeFunction, Package: "pkg", Name: "NewBar"})
	g.AddNode(&Node{ID: "helper", Type: NodeTypeFunction, Package: "pkg", Name: "helper"})
	g.AddNode(&Node{ID: "new_lower", Type: NodeTypeFunction, Package: "pkg", Name: "Newlower"}) // not a constructor (lowercase after New)

	result := g.DiscoverPatterns("pkg", PatternConstructors)
	if result.Count != 2 {
		t.Errorf("DiscoverPatterns(constructors) count = %d, want 2", result.Count)
	}
	names := make(map[string]bool)
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}
	if !names["NewFoo"] || !names["NewBar"] {
		t.Errorf("DiscoverPatterns(constructors) missing expected functions, got %v", names)
	}
}

func TestDiscoverPatterns_Tests(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "test_foo", Type: NodeTypeFunction, Package: "pkg", Name: "TestFoo", File: "foo_test.go"})
	g.AddNode(&Node{ID: "test_bar", Type: NodeTypeFunction, Package: "pkg", Name: "TestBar", File: "bar_test.go"})
	g.AddNode(&Node{ID: "do_thing", Type: NodeTypeFunction, Package: "pkg", Name: "DoThing"})

	result := g.DiscoverPatterns("pkg", PatternTests)
	if result.Count != 2 {
		t.Errorf("DiscoverPatterns(tests) count = %d, want 2", result.Count)
	}
}

func TestDiscoverPatterns_EntryPoints(t *testing.T) {
	g := New()
	// Exported, no internal callers -> entry point
	g.AddNode(&Node{ID: "exported_a", Type: NodeTypeFunction, Package: "pkg", Name: "ExportedA"})
	// Exported, but called by another pkg function -> not entry point
	g.AddNode(&Node{ID: "exported_b", Type: NodeTypeFunction, Package: "pkg", Name: "ExportedB"})
	g.AddNode(&Node{ID: "internal_caller", Type: NodeTypeFunction, Package: "pkg", Name: "internalCaller"})
	g.AddEdge(&Edge{From: "internal_caller", To: "exported_b", Type: EdgeTypeCalls})
	// unexported -> not entry point
	g.AddNode(&Node{ID: "unexported", Type: NodeTypeFunction, Package: "pkg", Name: "unexported"})

	result := g.DiscoverPatterns("pkg", PatternEntryPoints)
	if result.Count != 1 {
		t.Errorf("DiscoverPatterns(entrypoints) count = %d, want 1", result.Count)
	}
	if result.Functions[0].ID != "exported_a" {
		t.Errorf("DiscoverPatterns(entrypoints) got %s, want exported_a", result.Functions[0].ID)
	}
}

func TestDiscoverPatterns_Sinks(t *testing.T) {
	// Sink: has callers but no callees.
	// Use an external node as the caller so it doesn't appear in the pkg results.
	gs := New()
	gs.AddNode(&Node{ID: "sink", Type: NodeTypeFunction, Package: "pkg", Name: "Sink"})
	gs.AddNode(&Node{ID: "ext_caller", Type: NodeTypeFunction, Package: "ext", Name: "ExtCaller"})
	gs.AddEdge(&Edge{From: "ext_caller", To: "sink", Type: EdgeTypeCalls})

	sinks := gs.DiscoverPatterns("pkg", PatternSinks)
	if sinks.Count != 1 || sinks.Functions[0].ID != "sink" {
		t.Errorf("DiscoverPatterns(sinks) count = %d functions = %v, want [sink]", sinks.Count, sinks.Functions)
	}

	// Source: has callees but no callers.
	// Use an external node as the callee so it doesn't appear in the pkg results.
	gso := New()
	gso.AddNode(&Node{ID: "source", Type: NodeTypeFunction, Package: "pkg", Name: "Source"})
	gso.AddNode(&Node{ID: "ext_dep", Type: NodeTypeFunction, Package: "ext", Name: "ExtDep"})
	gso.AddEdge(&Edge{From: "source", To: "ext_dep", Type: EdgeTypeCalls})

	sources := gso.DiscoverPatterns("pkg", PatternSources)
	if sources.Count != 1 || sources.Functions[0].ID != "source" {
		t.Errorf("DiscoverPatterns(sources) count = %d functions = %v, want [source]", sources.Count, sources.Functions)
	}
}

func TestDiscoverPatterns_Hotspots(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "hot", Type: NodeTypeFunction, Package: "pkg", Name: "Hot"})
	g.AddNode(&Node{ID: "cold", Type: NodeTypeFunction, Package: "pkg", Name: "Cold"})
	// 3 callers for hot
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("caller%d", i)
		g.AddNode(&Node{ID: id, Type: NodeTypeFunction, Package: "pkg", Name: id})
		g.AddEdge(&Edge{From: id, To: "hot", Type: EdgeTypeCalls})
	}
	// 1 caller for cold
	g.AddNode(&Node{ID: "cold_caller", Type: NodeTypeFunction, Package: "pkg", Name: "coldCaller"})
	g.AddEdge(&Edge{From: "cold_caller", To: "cold", Type: EdgeTypeCalls})

	result := g.DiscoverPatterns("pkg", PatternHotspots)
	if result.Count < 2 {
		t.Fatalf("DiscoverPatterns(hotspots) count = %d, want >= 2", result.Count)
	}
	// hotspots ordered by caller count descending, so Hot should be first
	if result.Functions[0].ID != "hot" {
		t.Errorf("DiscoverPatterns(hotspots) first = %s, want hot", result.Functions[0].ID)
	}
}

// ---- GetNodesByPackageAndType, GetNeighborsByEdgeType, FindTests tests ----

func TestGetNodesByPackageAndType(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "fn1", Type: NodeTypeFunction, Package: "mypkg", Name: "Fn1"})
	g.AddNode(&Node{ID: "fn2", Type: NodeTypeFunction, Package: "mypkg", Name: "Fn2"})
	g.AddNode(&Node{ID: "typ1", Type: NodeTypeType, Package: "mypkg", Name: "MyType"})
	g.AddNode(&Node{ID: "fn3", Type: NodeTypeFunction, Package: "otherpkg", Name: "Fn3"})

	fns := g.GetNodesByPackageAndType("mypkg", NodeTypeFunction)
	if len(fns) != 2 {
		t.Errorf("GetNodesByPackageAndType(mypkg, function) = %d, want 2", len(fns))
	}

	types := g.GetNodesByPackageAndType("mypkg", NodeTypeType)
	if len(types) != 1 {
		t.Errorf("GetNodesByPackageAndType(mypkg, type) = %d, want 1", len(types))
	}

	none := g.GetNodesByPackageAndType("nonexistent", NodeTypeFunction)
	if len(none) != 0 {
		t.Errorf("GetNodesByPackageAndType(nonexistent) = %d, want 0", len(none))
	}
}

func TestGetNeighborsByEdgeType(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodeTypeFunction, Package: "pkg", Name: "A"})
	g.AddNode(&Node{ID: "b", Type: NodeTypeFunction, Package: "pkg", Name: "B"})
	g.AddNode(&Node{ID: "iface", Type: NodeTypeInterface, Package: "pkg", Name: "Iface"})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "a", To: "iface", Type: EdgeTypeImplements})

	callNeighbors := g.GetNeighborsByEdgeType("a", EdgeTypeCalls)
	if len(callNeighbors) != 1 || callNeighbors[0].ID != "b" {
		t.Errorf("GetNeighborsByEdgeType(calls) = %v, want [b]", callNeighbors)
	}

	implNeighbors := g.GetNeighborsByEdgeType("a", EdgeTypeImplements)
	if len(implNeighbors) != 1 || implNeighbors[0].ID != "iface" {
		t.Errorf("GetNeighborsByEdgeType(implements) = %v, want [iface]", implNeighbors)
	}

	noNeighbors := g.GetNeighborsByEdgeType("a", EdgeTypeReturns)
	if len(noNeighbors) != 0 {
		t.Errorf("GetNeighborsByEdgeType(returns) = %d, want 0", len(noNeighbors))
	}
}

func TestFindTests(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "fn", Type: NodeTypeFunction, Package: "pkg", Name: "Fn"})
	g.AddNode(&Node{ID: "helper", Type: NodeTypeFunction, Package: "pkg", Name: "helper"})
	g.AddNode(&Node{ID: "test_fn", Type: NodeTypeFunction, Package: "pkg", Name: "TestFn", File: "fn_test.go"})
	// test -> helper -> fn
	g.AddEdge(&Edge{From: "test_fn", To: "helper", Type: EdgeTypeCalls})
	g.AddEdge(&Edge{From: "helper", To: "fn", Type: EdgeTypeCalls})

	tests := g.FindTests("fn")
	if len(tests) != 1 || tests[0].ID != "test_fn" {
		t.Errorf("FindTests() = %v, want [test_fn]", tests)
	}

	// No tests for a node not transitively called by any test
	g2 := New()
	g2.AddNode(&Node{ID: "isolated", Type: NodeTypeFunction, Package: "pkg", Name: "Isolated"})
	tests2 := g2.FindTests("isolated")
	if len(tests2) != 0 {
		t.Errorf("FindTests(isolated) = %d, want 0", len(tests2))
	}
}
