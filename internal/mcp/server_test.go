package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func TestServer_RegisterTools(t *testing.T) {
	// Create server with temp DB
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add some test data to the graph
	node := &graph.Node{
		ID:        "test_func",
		Name:      "TestFunc",
		Package:   "testpkg",
		Type:      graph.NodeTypeFunction,
		Signature: "func TestFunc() error",
		File:      "test.go",
		Line:      10,
	}
	srv.graph.AddNode(node)

	// Create MCP server
	mcpSrv := mcpserver.NewMCPServer(
		"test-server",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	// Register tools - this is what we're testing
	srv.RegisterTools(mcpSrv)

	// Verify tools are registered by checking the server has them
	// We can't directly access the tools list, so we test by calling them
}

func TestServer_SearchFunctionsTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test nodes
	srv.graph.AddNode(&graph.Node{
		ID:        "func1",
		Name:      "HandleRequest",
		Package:   "handler",
		Type:      graph.NodeTypeFunction,
		Signature: "func HandleRequest(ctx context.Context) error",
	})
	srv.graph.AddNode(&graph.Node{
		ID:        "func2",
		Name:      "ProcessOrder",
		Package:   "service",
		Type:      graph.NodeTypeFunction,
		Signature: "func ProcessOrder(id string) (*Order, error)",
	})

	// Create MCP server and register tools
	mcpSrv := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	srv.RegisterTools(mcpSrv)

	// Call the search_functions tool
	req := mcp.CallToolRequest{}
	req.Params.Name = "search_functions"
	req.Params.Arguments = map[string]interface{}{
		"query": "HandleRequest",
		"limit": float64(10),
	}

	result, err := srv.handleSearchFunctionsMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSearchFunctionsMCP: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	// Check that we got text content back
	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	if textContent.Text == "" {
		t.Fatal("empty text content")
	}

	t.Logf("Result: %s", textContent.Text)
}

func TestServer_GetCallersTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test nodes and edges
	caller := &graph.Node{
		ID:        "caller_func",
		Name:      "Main",
		Package:   "main",
		Type:      graph.NodeTypeFunction,
		Signature: "func Main()",
	}
	callee := &graph.Node{
		ID:        "callee_func",
		Name:      "Helper",
		Package:   "util",
		Type:      graph.NodeTypeFunction,
		Signature: "func Helper() error",
	}
	srv.graph.AddNode(caller)
	srv.graph.AddNode(callee)
	srv.graph.AddEdge(&graph.Edge{
		From: caller.ID,
		To:   callee.ID,
		Type: graph.EdgeTypeCalls,
	})

	// Create MCP server and register tools
	mcpSrv := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	srv.RegisterTools(mcpSrv)

	// Call get_callers tool
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_callers"
	req.Params.Arguments = map[string]interface{}{
		"function_id": "callee_func",
	}

	result, err := srv.handleGetCallersMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetCallersMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Callers: %s", textContent.Text)
}

func TestServer_GetCalleesTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test nodes and edges
	caller := &graph.Node{
		ID:        "caller_func",
		Name:      "Process",
		Package:   "main",
		Type:      graph.NodeTypeFunction,
		Signature: "func Process()",
	}
	callee := &graph.Node{
		ID:        "callee_func",
		Name:      "Validate",
		Package:   "util",
		Type:      graph.NodeTypeFunction,
		Signature: "func Validate() bool",
	}
	srv.graph.AddNode(caller)
	srv.graph.AddNode(callee)
	srv.graph.AddEdge(&graph.Edge{
		From: caller.ID,
		To:   callee.ID,
		Type: graph.EdgeTypeCalls,
	})

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_callees"
	req.Params.Arguments = map[string]interface{}{
		"function_id": "caller_func",
	}

	result, err := srv.handleGetCalleesMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetCalleesMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Callees: %s", textContent.Text)
}

func TestServer_RegisterResources(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test node
	srv.graph.AddNode(&graph.Node{
		ID:        "test_func",
		Name:      "TestFunc",
		Package:   "testpkg",
		Type:      graph.NodeTypeFunction,
		Signature: "func TestFunc() error",
	})

	// Create MCP server
	mcpSrv := mcpserver.NewMCPServer(
		"test-server",
		"1.0.0",
		mcpserver.WithResourceCapabilities(true, true),
	)

	// Register resources
	srv.RegisterResources(mcpSrv)
}

func TestServer_FunctionResource(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test node
	srv.graph.AddNode(&graph.Node{
		ID:        "test_func",
		Name:      "TestFunc",
		Package:   "testpkg",
		Type:      graph.NodeTypeFunction,
		Signature: "func TestFunc() error",
		File:      "test.go",
		Line:      10,
		Docstring: "TestFunc does something",
	})

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "function://testpkg/TestFunc"

	result, err := srv.handleFunctionResourceMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFunctionResourceMCP: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("no resource contents")
	}

	textResource, ok := result[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", result[0])
	}

	if textResource.URI != "function://testpkg/TestFunc" {
		t.Errorf("wrong URI: %s", textResource.URI)
	}

	if textResource.MIMEType != "application/json" {
		t.Errorf("wrong MIME type: %s", textResource.MIMEType)
	}

	t.Logf("Resource: %s", textResource.Text)
}

func TestServer_PackageResource(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add test nodes
	srv.graph.AddNode(&graph.Node{
		ID:        "func1",
		Name:      "Func1",
		Package:   "mypkg",
		Type:      graph.NodeTypeFunction,
		Signature: "func Func1()",
	})
	srv.graph.AddNode(&graph.Node{
		ID:        "func2",
		Name:      "Func2",
		Package:   "mypkg",
		Type:      graph.NodeTypeFunction,
		Signature: "func Func2() error",
	})

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "package://mypkg"

	result, err := srv.handlePackageResourceMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handlePackageResourceMCP: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("no resource contents")
	}

	textResource, ok := result[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", result[0])
	}

	t.Logf("Package resource: %s", textResource.Text)
}

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

	// Test 3: Find by name and file - should return only service package function
	req3 := mcp.CallToolRequest{}
	req3.Params.Name = "get_function_by_name"
	req3.Params.Arguments = map[string]interface{}{
		"name": "Handle",
		"file": "service",
	}

	result3, err := srv.handleGetFunctionByNameMCP(context.Background(), req3)
	if err != nil {
		t.Fatalf("handleGetFunctionByNameMCP: %v", err)
	}

	textContent3, ok := result3.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result3.Content[0])
	}

	// Should only contain service package
	if !strings.Contains(textContent3.Text, `"package": "service"`) {
		t.Fatal("expected service package in result")
	}
	if strings.Contains(textContent3.Text, `"package": "handler"`) {
		t.Fatal("should not contain handler package in result when filtering by file")
	}

	t.Logf("Result: %s", textContent3.Text)
}

func TestServer_GetImplementorsTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

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

	srv.graph.AddNode(iface)
	srv.graph.AddNode(typ)
	srv.graph.AddEdge(&graph.Edge{
		From:     typ.ID,
		To:       iface.ID,
		Type:     graph.EdgeTypeImplements,
		Metadata: map[string]any{"pointer_receiver": true},
	})

	result, err := srv.handleGetImplementors(context.Background(), map[string]interface{}{
		"interface_id": "type_io.Reader",
	})
	if err != nil {
		t.Fatalf("handleGetImplementors failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	implementors := data["implementors"].([]interface{})
	if len(implementors) != 1 {
		t.Errorf("expected 1 implementor, got %d", len(implementors))
	}
}

func TestServer_GetInterfacesTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

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

	srv.graph.AddNode(iface)
	srv.graph.AddNode(typ)
	srv.graph.AddEdge(&graph.Edge{
		From:     typ.ID,
		To:       iface.ID,
		Type:     graph.EdgeTypeImplements,
		Metadata: map[string]any{"pointer_receiver": true},
	})

	result, err := srv.handleGetInterfaces(context.Background(), map[string]interface{}{
		"type_id": "type_os.File",
	})
	if err != nil {
		t.Fatalf("handleGetInterfaces failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	interfaces := data["interfaces"].([]interface{})
	if len(interfaces) != 1 {
		t.Errorf("expected 1 interface, got %d", len(interfaces))
	}
}

func TestServer_SearchByBehaviorTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	node := &graph.Node{
		ID:        "func_service_LogError_test.go:10",
		Type:      graph.NodeTypeFunction,
		Package:   "service",
		Name:      "LogError",
		Signature: "func LogError(msg string)",
		Summary:   &graph.Summary{Text: "[Logging] Logs error messages to stdout"},
		Metadata: map[string]any{
			"behaviors": []string{"logging", "error-handle"},
		},
	}
	srv.graph.AddNode(node)

	result, err := srv.handleSearchByBehavior(context.Background(), map[string]any{
		"query":     "log errors",
		"behaviors": []any{"logging", "error-handle"},
		"limit":     float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearchByBehavior() error = %v", err)
	}

	t.Logf("Result: %s", result)

	if !strings.Contains(result, "LogError") {
		t.Error("Result should contain LogError function")
	}
}

func TestServer_GetNeighborhoodTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up A→B→C
	nodeA := &graph.Node{ID: "func_A", Name: "FuncA", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncA()"}
	nodeB := &graph.Node{ID: "func_B", Name: "FuncB", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncB()"}
	nodeC := &graph.Node{ID: "func_C", Name: "FuncC", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncC()"}
	srv.graph.AddNode(nodeA)
	srv.graph.AddNode(nodeB)
	srv.graph.AddNode(nodeC)
	srv.graph.AddEdge(&graph.Edge{From: nodeA.ID, To: nodeB.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: nodeB.ID, To: nodeC.ID, Type: graph.EdgeTypeCalls})

	// Query neighborhood of B with depth 1
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_neighborhood"
	req.Params.Arguments = map[string]interface{}{
		"node_id": "func_B",
		"depth":   float64(1),
	}

	result, err := srv.handleGetNeighborhoodMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetNeighborhoodMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Neighborhood: %s", textContent.Text)

	// Should contain all 3 nodes and 2 edges
	if !strings.Contains(textContent.Text, "FuncA") {
		t.Error("expected FuncA in neighborhood result")
	}
	if !strings.Contains(textContent.Text, "FuncB") {
		t.Error("expected FuncB in neighborhood result")
	}
	if !strings.Contains(textContent.Text, "FuncC") {
		t.Error("expected FuncC in neighborhood result")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	nodes, ok := data["nodes"].([]interface{})
	if !ok {
		t.Fatal("expected nodes array in result")
	}
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	edges, ok := data["edges"].([]interface{})
	if !ok {
		t.Fatal("expected edges array in result")
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestServer_GetImpactTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up A→B→C with TestA calling A
	nodeA := &graph.Node{ID: "func_A", Name: "FuncA", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncA()"}
	nodeB := &graph.Node{ID: "func_B", Name: "FuncB", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncB()"}
	nodeC := &graph.Node{ID: "func_C", Name: "FuncC", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncC()"}
	nodeTestA := &graph.Node{ID: "func_TestA", Name: "TestFuncA", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func TestFuncA(t *testing.T)"}
	srv.graph.AddNode(nodeA)
	srv.graph.AddNode(nodeB)
	srv.graph.AddNode(nodeC)
	srv.graph.AddNode(nodeTestA)
	srv.graph.AddEdge(&graph.Edge{From: nodeA.ID, To: nodeB.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: nodeB.ID, To: nodeC.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: nodeTestA.ID, To: nodeA.ID, Type: graph.EdgeTypeCalls})

	// Query impact of C: B should be direct caller, A indirect, TestA in tests
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_impact"
	req.Params.Arguments = map[string]interface{}{
		"function_id": "func_C",
	}

	result, err := srv.handleGetImpactMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetImpactMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Impact: %s", textContent.Text)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	directCallers, ok := data["direct_callers"].([]interface{})
	if !ok {
		t.Fatal("expected direct_callers array")
	}
	if len(directCallers) != 1 {
		t.Errorf("expected 1 direct caller (B), got %d", len(directCallers))
	}

	indirectCallers, ok := data["indirect_callers"].([]interface{})
	if !ok {
		t.Fatal("expected indirect_callers array")
	}
	if len(indirectCallers) != 2 {
		t.Errorf("expected 2 indirect callers (A and TestA), got %d", len(indirectCallers))
	}

	tests, ok := data["tests"].([]interface{})
	if !ok {
		t.Fatal("expected tests array")
	}
	if len(tests) != 1 {
		t.Errorf("expected 1 test (TestFuncA), got %d", len(tests))
	}
}

func TestServer_TraceChainTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up A→B→C
	nodeA := &graph.Node{ID: "func_A", Name: "FuncA", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncA()"}
	nodeB := &graph.Node{ID: "func_B", Name: "FuncB", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncB()"}
	nodeC := &graph.Node{ID: "func_C", Name: "FuncC", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func FuncC()"}
	srv.graph.AddNode(nodeA)
	srv.graph.AddNode(nodeB)
	srv.graph.AddNode(nodeC)
	srv.graph.AddEdge(&graph.Edge{From: nodeA.ID, To: nodeB.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: nodeB.ID, To: nodeC.ID, Type: graph.EdgeTypeCalls})

	// Trace from A to C — path should be [A, B, C]
	req := mcp.CallToolRequest{}
	req.Params.Name = "trace_chain"
	req.Params.Arguments = map[string]interface{}{
		"from_id": "func_A",
		"to_id":   "func_C",
	}

	result, err := srv.handleTraceChainMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTraceChainMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("TraceChain: %s", textContent.Text)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	pathLength, ok := data["path_length"].(float64)
	if !ok {
		t.Fatal("expected path_length in result")
	}
	if int(pathLength) != 3 {
		t.Errorf("expected path length 3 [A,B,C], got %d", int(pathLength))
	}

	path, ok := data["path"].([]interface{})
	if !ok {
		t.Fatal("expected path array in result")
	}
	if len(path) != 3 {
		t.Errorf("expected 3 nodes in path, got %d", len(path))
	}

	// Verify path contains all three function names
	pathText := textContent.Text
	if !strings.Contains(pathText, "FuncA") {
		t.Error("path should contain FuncA")
	}
	if !strings.Contains(pathText, "FuncB") {
		t.Error("path should contain FuncB")
	}
	if !strings.Contains(pathText, "FuncC") {
		t.Error("path should contain FuncC")
	}
}

func TestServer_GetContractTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up a function with 2 callers and 1 callee
	target := &graph.Node{ID: "func_target", Name: "Process", Package: "svc", Type: graph.NodeTypeFunction, Signature: "func Process() error"}
	caller1 := &graph.Node{ID: "func_caller1", Name: "Main", Package: "svc", Type: graph.NodeTypeFunction, Signature: "func Main()"}
	caller2 := &graph.Node{ID: "func_caller2", Name: "Run", Package: "svc", Type: graph.NodeTypeFunction, Signature: "func Run()"}
	callee := &graph.Node{ID: "func_callee", Name: "Validate", Package: "svc", Type: graph.NodeTypeFunction, Signature: "func Validate() bool"}
	srv.graph.AddNode(target)
	srv.graph.AddNode(caller1)
	srv.graph.AddNode(caller2)
	srv.graph.AddNode(callee)
	srv.graph.AddEdge(&graph.Edge{From: caller1.ID, To: target.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: caller2.ID, To: target.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: target.ID, To: callee.ID, Type: graph.EdgeTypeCalls})

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_contract"
	req.Params.Arguments = map[string]interface{}{
		"function_id": "func_target",
	}

	result, err := srv.handleGetContractMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetContractMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Contract: %s", textContent.Text)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	callerCount, ok := data["caller_count"].(float64)
	if !ok {
		t.Fatal("expected caller_count in result")
	}
	if int(callerCount) != 2 {
		t.Errorf("expected caller_count=2, got %d", int(callerCount))
	}

	calleeCount, ok := data["callee_count"].(float64)
	if !ok {
		t.Fatal("expected callee_count in result")
	}
	if int(calleeCount) != 1 {
		t.Errorf("expected callee_count=1, got %d", int(calleeCount))
	}
}

func TestServer_DiscoverPatternsTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up a package with NewFoo and NewBar constructors
	srv.graph.AddNode(&graph.Node{ID: "func_NewFoo", Name: "NewFoo", Package: "factory", Type: graph.NodeTypeFunction, Signature: "func NewFoo() *Foo"})
	srv.graph.AddNode(&graph.Node{ID: "func_NewBar", Name: "NewBar", Package: "factory", Type: graph.NodeTypeFunction, Signature: "func NewBar() *Bar"})
	srv.graph.AddNode(&graph.Node{ID: "func_process", Name: "process", Package: "factory", Type: graph.NodeTypeFunction, Signature: "func process()"})

	req := mcp.CallToolRequest{}
	req.Params.Name = "discover_patterns"
	req.Params.Arguments = map[string]interface{}{
		"package":      "factory",
		"pattern_type": "constructors",
	}

	result, err := srv.handleDiscoverPatternsMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiscoverPatternsMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Patterns: %s", textContent.Text)

	if !strings.Contains(textContent.Text, "NewFoo") {
		t.Error("expected NewFoo in constructors result")
	}
	if !strings.Contains(textContent.Text, "NewBar") {
		t.Error("expected NewBar in constructors result")
	}
	if strings.Contains(textContent.Text, `"name": "process"`) {
		t.Error("process should not appear in constructors result")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	count, ok := data["count"].(float64)
	if !ok {
		t.Fatal("expected count in result")
	}
	if int(count) != 2 {
		t.Errorf("expected count=2 (NewFoo and NewBar), got %d", int(count))
	}
}

func TestServer_FindTestsTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up Func→Helper with TestFunc calling Func
	nodeFunc := &graph.Node{ID: "func_Func", Name: "Func", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func Func()"}
	nodeHelper := &graph.Node{ID: "func_Helper", Name: "Helper", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func Helper()"}
	nodeTestFunc := &graph.Node{ID: "func_TestFunc", Name: "TestFunc", Package: "pkg", Type: graph.NodeTypeFunction, Signature: "func TestFunc(t *testing.T)"}
	srv.graph.AddNode(nodeFunc)
	srv.graph.AddNode(nodeHelper)
	srv.graph.AddNode(nodeTestFunc)
	srv.graph.AddEdge(&graph.Edge{From: nodeFunc.ID, To: nodeHelper.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: nodeTestFunc.ID, To: nodeFunc.ID, Type: graph.EdgeTypeCalls})

	// Find tests for Helper — should surface TestFunc via Func→Helper chain
	req := mcp.CallToolRequest{}
	req.Params.Name = "find_tests"
	req.Params.Arguments = map[string]interface{}{
		"function_id": "func_Helper",
	}

	result, err := srv.handleFindTestsMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFindTestsMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("Tests: %s", textContent.Text)

	if !strings.Contains(textContent.Text, "TestFunc") {
		t.Error("expected TestFunc in find_tests result")
	}

	var tests []interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &tests); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(tests) != 1 {
		t.Errorf("expected 1 test, got %d", len(tests))
	}
}

func TestServer_GetFunctionContextTool(t *testing.T) {
	srv, err := NewServer(&Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: ".",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Set up a function with callers and callees
	target := &graph.Node{
		ID:        "func_target",
		Name:      "Process",
		Package:   "svc",
		Type:      graph.NodeTypeFunction,
		Signature: "func Process() error",
		File:      "",
		Line:      0,
	}
	caller := &graph.Node{ID: "func_caller", Name: "Main", Package: "svc", Type: graph.NodeTypeFunction, Signature: "func Main()"}
	callee := &graph.Node{ID: "func_callee", Name: "Validate", Package: "svc", Type: graph.NodeTypeFunction, Signature: "func Validate() bool"}
	srv.graph.AddNode(target)
	srv.graph.AddNode(caller)
	srv.graph.AddNode(callee)
	srv.graph.AddEdge(&graph.Edge{From: caller.ID, To: target.ID, Type: graph.EdgeTypeCalls})
	srv.graph.AddEdge(&graph.Edge{From: target.ID, To: callee.ID, Type: graph.EdgeTypeCalls})

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_function_context"
	req.Params.Arguments = map[string]interface{}{
		"function_id": "func_target",
	}

	result, err := srv.handleGetFunctionContextMCP(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetFunctionContextMCP: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("no result content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	t.Logf("FunctionContext: %s", textContent.Text)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Verify function field is present with the right name
	// Note: graph.Node serializes with uppercase JSON keys (no json tags)
	fn, ok := data["function"].(map[string]interface{})
	if !ok {
		t.Fatal("expected function object in result")
	}
	if fn["Name"] != "Process" {
		t.Errorf("expected function name 'Process', got %v", fn["Name"])
	}

	// Verify callers list contains Main
	callers, ok := data["callers"].([]interface{})
	if !ok {
		t.Fatal("expected callers array in result")
	}
	if len(callers) != 1 {
		t.Errorf("expected 1 caller, got %d", len(callers))
	}
	if !strings.Contains(textContent.Text, "Main") {
		t.Error("expected Main in callers")
	}

	// Verify callees list contains Validate
	callees, ok := data["callees"].([]interface{})
	if !ok {
		t.Fatal("expected callees array in result")
	}
	if len(callees) != 1 {
		t.Errorf("expected 1 callee, got %d", len(callees))
	}
	if !strings.Contains(textContent.Text, "Validate") {
		t.Error("expected Validate in callees")
	}

	// Verify package field
	if data["package"] != "svc" {
		t.Errorf("expected package 'svc', got %v", data["package"])
	}
}
