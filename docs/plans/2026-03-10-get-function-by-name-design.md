# Design: get_function_by_name MCP Tool

## Problem

The `search_functions` MCP tool fails to find functions when:
1. Query is descriptive (e.g., "main function entry point") - name matching doesn't work
2. Semantic search is unavailable - falls back to simple substring matching

Users need a reliable way to find functions by exact name.

## Solution

Add a new `get_function_by_name` tool with indexed lookups for O(1) performance.

## Design

### Tool Definition

```
Name: get_function_by_name
Description: Find functions by exact name match

Parameters:
  - name (required): Function name to search for
  - package (optional): Filter by package name  
  - file (optional): Filter by file path (substring match)

Returns: Array of matching functions
```

### Graph Index Changes

Add `byName` index to `Graph` struct:

```go
type Graph struct {
    // existing fields
    byName map[string]map[string]*Node  // name -> id -> Node
}
```

### New Methods

```go
// GetNodesByName returns all functions with the given name
func (g *Graph) GetNodesByName(name string) []*Node

// GetNodesByNameAndPackage returns functions matching both name and package
func (g *Graph) GetNodesByNameAndPackage(name, pkg string) []*Node
```

### Handler Logic

1. Validate `name` parameter (required)
2. Use indexed lookup by name
3. Optionally filter by package (uses existing `byPackage` index)
4. Optionally filter by file (substring match)
5. Return JSON array of matching nodes

## Files to Modify

| File | Changes |
|------|---------|
| `internal/graph/graph.go` | Add `byName` index, new lookup methods |
| `internal/mcp/tools.go` | Add tool definition and handler |
| `internal/mcp/server.go` | Register new tool |
| `internal/mcp/server_test.go` | Add tests |

## Implementation Steps

1. Add `byName` index to `Graph` struct
2. Update `AddNode` to maintain the index
3. Implement `GetNodesByName` and `GetNodesByNameAndPackage`
4. Add `handleGetFunctionByName` handler in tools.go
5. Register tool in server.go
6. Add tests
7. Update README with new tool

## Trade-offs

- **Memory:** Small overhead for name index (references only)
- **Performance:** O(1) name lookup vs O(n) iteration
- **Simplicity:** Clean API, predictable behavior
