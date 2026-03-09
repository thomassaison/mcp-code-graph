# 6. MCP Interface Design

Date: 2026-03-09

## Status

Accepted

## Context

The code graph server exposes functionality via the Model Context Protocol (MCP). MCP provides two main primitives:
- **Tools**: Functions the AI can call
- **Resources**: Data the AI can read

We need to decide what tools and resources to expose.

## Decision

We will expose:

1. **Query tools** — for searching and traversing the graph
2. **Resources** — for navigating entities by URI
3. **Update tools** — for modifying summaries and adding annotations

### Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_functions` | Semantic search on function summaries | `query: string`, `limit?: number` |
| `get_callers` | Find all functions that call this function | `function_id: string` |
| `get_callees` | Find all functions this function calls | `function_id: string` |
| `get_dependencies` | Get type/package dependencies | `node_id: string`, `depth?: number` |
| `find_path` | Find relationship path between two nodes | `from_id: string`, `to_id: string` |
| `reindex_project` | Trigger full reindex | `path?: string` |
| `update_summary` | Edit a function's summary | `function_id: string`, `summary: string` |
| `add_note` | Add a developer note to a node | `node_id: string`, `note: string` |
| `regenerate_summary` | Re-generate LLM summary | `function_id: string` |

### Resources

Resources use a URI scheme for navigation:

| Resource URI Pattern | Description |
|---------------------|-------------|
| `function://{package}/{name}` | Function details + summary |
| `type://{package}/{name}` | Type/struct/interface details |
| `package://{name}` | Package overview + contents |
| `file://{path}` | File overview + functions defined |
| `graph://callers/{function_id}` | Callers of a function |
| `graph://callees/{function_id}` | Functions called by this one |

### Resource Templates

MCP supports resource templates for dynamic URIs:

```json
{
  "uriTemplate": "function://{package}/{name}",
  "name": "Function by package and name",
  "description": "Get function details, signature, summary, and references"
}
```

### Example Resource Response

```
GET function://main/handleRequest

{
  "id": "func_main_handleRequest",
  "name": "handleRequest",
  "package": "main",
  "file": "cmd/server/handler.go:45",
  "signature": "func handleRequest(ctx context.Context, req *Request) (*Response, error)",
  "summary": "Handles incoming HTTP requests, validates input, calls business logic, and formats response.",
  "callers": ["main.main", "middleware.loggingMiddleware"],
  "callees": ["main.validateRequest", "service.ProcessOrder", "main.formatError"],
  "types_used": ["context.Context", "*Request", "*Response", "error"],
  "notes": [
    {"author": "human", "text": "TODO: Add rate limiting", "created": "2026-03-08"}
  ]
}
```

## Consequences

### Positive

- **Rich querying**: Agents can search, traverse, and explore the codebase
- **Navigable**: Resources provide a browsable structure
- **Updatable**: Agents can improve summaries and add context
- **MCP-native**: Follows MCP conventions, works with any MCP client

### Negative

- **Surface area**: Many tools to implement and test
- **Versioning**: Changes to tool signatures may break clients
- **Resource explosion**: Large codebases may have thousands of resources

### Mitigations

- Start with core tools, add advanced features incrementally
- Document tool contracts clearly
- Resource listing returns paginated results

## Implementation Notes

### Tool Handler Structure

```go
func (s *Server) handleSearchFunctions(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
    query, ok := args["query"].(string)
    if !ok {
        return nil, errors.New("query must be a string")
    }
    
    limit := 10
    if l, ok := args["limit"].(float64); ok {
        limit = int(l)
    }
    
    results, err := s.vectorStore.Search(ctx, query, limit)
    if err != nil {
        return nil, err
    }
    
    return mcp.NewToolResult(results), nil
}
```

### Resource Handler Structure

```go
func (s *Server) handleFunctionResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
    // Parse URI: function://{package}/{name}
    parts := strings.Split(strings.TrimPrefix(uri, "function://"), "/")
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid function URI: %s", uri)
    }
    
    pkg, name := parts[0], parts[1]
    fn, err := s.graph.GetFunction(pkg, name)
    if err != nil {
        return nil, err
    }
    
    return mcp.NewReadResourceResult(uri, "application/json", fn), nil
}
```

## References

- [MCP Specification](https://modelcontextprotocol.io/spec)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) (if available, or implement custom)
