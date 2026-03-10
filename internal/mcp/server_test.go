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
