# ADR-0013: get_function_by_name Tool

## Status

Accepted

## Context

The `search_functions` MCP tool uses semantic similarity or substring matching to find functions. This works well for exploratory queries like "find functions that handle authentication" but fails when the AI knows the exact function name it's looking for.

**Problem examples:**
1. Query "main function entry point" returns no results because substring matching looks for "main function entry point" in function names
2. Query "main" should work but may be slow if semantic search is unavailable (iterates all functions)
3. No way to efficiently look up a function when the name is known exactly

## Decision

Add a new `get_function_by_name` MCP tool that provides O(1) lookups by exact function name.

### Tool Definition

```
Name: get_function_by_name
Description: Find functions by exact name match

Parameters:
  - name (required): Function name to search for
  - package (optional): Exact package name to filter by
  - file (optional): File path substring to filter by

Returns: JSON array of matching functions
```

### Graph Index

Add a `byName` index to the `Graph` struct for O(1) name lookups:

```go
type Graph struct {
    // ... existing indexes ...
    byName map[string]map[string]*Node  // name -> id -> Node
}
```

Methods:
- `GetNodesByName(name string) []*Node` — returns all nodes with given name
- `GetNodesByNameAndPackage(name, pkg string) []*Node` — returns nodes matching both

### Return Fields

The tool returns more fields than other tools because the caller knows what they're looking for:
- `id`, `name`, `package`, `signature`
- `file`, `line`
- `docstring`, `summary`

### Type Filtering

Only returns `NodeTypeFunction` and `NodeTypeMethod` nodes (not types, interfaces, packages).

## Consequences

### Positive

- O(1) lookup by name vs O(n) iteration
- Predictable behavior when name is known exactly
- Works without embedding provider configured
- Returns richer information (file, line, docstring, summary)
- Can narrow by package for disambiguation

### Negative

- Additional memory for `byName` index (minimal — stores references)
- Another tool for AI to understand
- Name must be exact (no fuzzy matching)

### Neutral

- Complements `search_functions` rather than replacing it
- AI can choose appropriate tool based on query type

## Usage Examples

```json
// Find all "main" functions
{"name": "main"}

// Find "Handle" in specific package
{"name": "Handle", "package": "handler"}

// Find "Process" in files matching "service"
{"name": "Process", "file": "service"}
```
