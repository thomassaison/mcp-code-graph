# 8. MCP Protocol Implementation

Date: 2026-03-09

## Status

Accepted

## Context

The MCP server (ADR-0006) defines tools and resources, but does not implement the MCP protocol itself. The current `Start()` method just indexes the project and exits - it cannot be used by MCP clients like OpenCode.

We need to implement the MCP protocol (JSON-RPC over stdio) so that:
1. AI assistants can discover and call tools
2. AI assistants can read resources
3. The server integrates with any MCP-compatible client

## Decision

We will use the `github.com/mark3labs/mcp-go` library to implement the MCP protocol.

### Why mcp-go

| Criteria | mcp-go | modelcontextprotocol/go-sdk |
|----------|--------|----------------------------|
| Benchmark Score | 87.53 | 57.2 |
| Code Snippets | 580 | 6026 |
| API Simplicity | High | Medium |
| stdio transport | Native | Native |
| Tool registration | `AddTool()` | Similar |
| Resource registration | `AddResource()` | Similar |

`mcp-go` has a simpler API and higher benchmark score. Both are production-ready.

### Architecture

```
cmd/mcp-code-graph/main.go
    └── creates MCPServer with mcp-go
    └── registers tools via s.AddTool()
    └── registers resources via s.AddResource()
    └── calls server.ServeStdio(s)
    
internal/mcp/
    └── server.go     — wraps mcp-go MCPServer, holds graph/indexer
    └── tools.go      — tool definitions and handlers (business logic)
    └── resources.go  — resource definitions and handlers (business logic)
```

### Tool Registration Pattern

```go
func (s *Server) RegisterTools(mcpServer *server.MCPServer) {
    s.addSearchFunctionsTool(mcpServer)
    s.addGetCallersTool(mcpServer)
    s.addGetCalleesTool(mcpServer)
    s.addReindexProjectTool(mcpServer)
    s.addUpdateSummaryTool(mcpServer)
}

func (s *Server) addSearchFunctionsTool(mcpServer *server.MCPServer) {
    tool := mcp.NewTool("search_functions",
        mcp.WithDescription("Search for functions by name"),
        mcp.WithString("query",
            mcp.Required(),
            mcp.Description("The search query"),
        ),
        mcp.WithNumber("limit",
            mcp.Description("Maximum number of results to return"),
        ),
    )
    
    mcpServer.AddTool(tool, s.handleSearchFunctions)
}
```

### Resource Registration Pattern

```go
func (s *Server) RegisterResources(mcpServer *server.MCPServer) {
    // Function resource template
    mcpServer.AddResource(
        mcp.NewResource(
            "function://{package}/{name}",
            "Function",
            mcp.WithResourceDescription("Get function details by package and name"),
            mcp.WithMIMEType("application/json"),
        ),
        s.handleFunctionResource,
    )
    
    // Package resource template
    mcpServer.AddResource(
        mcp.NewResource(
            "package://{name}",
            "Package",
            mcp.WithResourceDescription("Get package overview"),
            mcp.WithMIMEType("application/json"),
        ),
        s.handlePackageResource,
    )
}
```

## Consequences

### Positive

- **Real MCP integration**: Works with OpenCode and other MCP clients
- **Simple API**: mcp-go requires minimal boilerplate
- **Type-safe**: Handlers use mcp.CallToolRequest with type helpers
- **Testable**: Can test tools and resources without MCP protocol

### Negative

- **External dependency**: Adds mcp-go to go.mod
- **Lock-in**: Would need refactoring to switch libraries
- **Limited control**: Protocol details handled by library

### Mitigations

- mcp-go is lightweight and well-maintained
- Handler logic is decoupled from MCP protocol (business logic in tools.go/resources.go)
- Can switch libraries by changing only RegisterTools/RegisterResources

## Implementation Notes

### main.go Changes

```go
func main() {
    // ... flag parsing ...
    
    srv, err := mcp.NewServer(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer srv.Close()
    
    // Index project first (synchronous)
    if err := srv.IndexProject(); err != nil {
        log.Fatal(err)
    }
    
    // Create MCP server
    mcpServer := server.NewMCPServer(
        "mcp-code-graph",
        "1.0.0",
        server.WithToolCapabilities(true),
        server.WithResourceCapabilities(true, true),
    )
    
    // Register tools and resources
    srv.RegisterTools(mcpServer)
    srv.RegisterResources(mcpServer)
    
    // Start stdio transport
    if err := server.ServeStdio(mcpServer); err != nil {
        log.Fatal(err)
    }
}
```

### Handler Signature Change

Current handlers return `(string, error)`. MCP handlers return `(*mcp.CallToolResult, error)`:

```go
// Before
func (s *Server) handleSearchFunctions(ctx context.Context, args map[string]any) (string, error)

// After
func (s *Server) handleSearchFunctions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
```

## References

- [mcp-go on GitHub](https://github.com/mark3labs/mcp-go)
- [mcp-go documentation](https://mcp-go.dev)
- [MCP Specification](https://modelcontextprotocol.io/spec)
