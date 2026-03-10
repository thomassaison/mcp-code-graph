# Implicit Interface Resolver - Design Document

Date: 2026-03-10

## Overview

Add implicit interface resolution to mcp-code-graph, enabling AI assistants to query which types implement an interface and which interfaces a type satisfies. This is the #1 pain point in Go code navigation because Go interfaces are satisfied implicitly.

## Goals

- Find all types that implement a given interface (including stdlib/external)
- Find all interfaces a given type satisfies (including stdlib/external)
- Support pointer receiver distinction (`*T` vs `T` implementing an interface)
- Integrate via two MCP tools: `get_implementors` and `get_interfaces`

## Non-Goals

- Incremental type checking on file changes (deferred to future work)
- Method-based lookup (find types by method signature)

## Data Model

### Node Changes

**Existing node types used:**
- `NodeTypeInterface` — for interface definitions
- `NodeTypeType` — for struct types

**New field on Node:**
```go
type Method struct {
    Name      string
    Signature string
}

type Node struct {
    // ... existing fields
    Methods   []Method  // For interfaces: list of required method signatures
}
```

### Edge Direction

```
Struct/Type --[implements]--> Interface
```

Example: `*os.File` implements `io.Reader`

### Graph Indexes

```go
type Graph struct {
    // ... existing indexes
    
    // NEW indexes for O(1) lookup
    byInterface map[string][]*Node  // interface ID -> implementing types
    byTypeImpl  map[string][]*Node  // type ID -> implemented interfaces
}
```

**Edge Metadata:**
```go
type Edge struct {
    From     string
    To       string
    Type     EdgeType
    Metadata map[string]any  // "pointer_receiver": true/false
}
```

## Architecture

### Approach: Type Checker as Separate Pass

```
reindex_project 
    → parser.ParseModule()        # AST parsing (functions, calls)
    → graph.AddNode/AddEdge 
    → types.Check()               # Type checking pass (NEW)
    → graph.AddNode (types, interfaces)
    → graph.AddEdge (implements)
    → persist
```

### New Package Structure

```
internal/types/
├── checker.go      # Type checker wrapper using go/packages
├── interface.go    # Interface extraction logic
├── implements.go   # Implementation detection using types.Implements()
└── checker_test.go
```

### Type Checker Integration

```go
// checker.go
type Checker struct {
    fset *token.FileSet
    pkgs []*packages.Package
}

type CheckResult struct {
    Interfaces []*graph.Node
    Types      []*graph.Node
    Edges      []*graph.Edge
}

func (c *Checker) Check(root string) (*CheckResult, error) {
    cfg := &packages.Config{
        Mode: packages.NeedName | packages.NeedFiles | 
              packages.NeedSyntax | packages.NeedTypes | 
              packages.NeedImports | packages.NeedDeps,
        Dir: root,
    }
    pkgs, err := packages.Load(cfg, "./...")
    // ...
}
```

**Key packages.Config flags:**
- `NeedTypes` — load type information
- `NeedImports` — load imported packages
- `NeedDeps` — load transitive dependencies (required for stdlib/external interfaces)

## MCP Tools

### Tool 1: `get_implementors`

Find all types that implement a given interface.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `interface_id` | string | yes | Interface to query (e.g., "io.Reader") |
| `include_pointer` | boolean | no | Include pointer variants (default: true) |

**Response:**
```json
{
    "interface": {
        "id": "type_io.Reader",
        "name": "Reader",
        "package": "io",
        "methods": [
            {"name": "Read", "signature": "Read(p []byte) (n int, err error)"}
        ]
    },
    "implementors": [
        {
            "id": "type_os.File",
            "name": "File",
            "package": "os",
            "kind": "struct"
        }
    ]
}
```

### Tool 2: `get_interfaces`

Find all interfaces that a type implements.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `type_id` | string | yes | Type to query (e.g., "os.File") |

**Response:**
```json
{
    "type": {
        "id": "type_os.File",
        "name": "File",
        "package": "os",
        "kind": "struct"
    },
    "interfaces": [
        {
            "id": "type_io.Reader",
            "name": "Reader",
            "package": "io",
            "pointer_receiver": false
        },
        {
            "id": "type_io.Writer",
            "name": "Writer",
            "package": "io",
            "pointer_receiver": true
        }
    ]
}
```

**`pointer_receiver` field:** Indicates whether `*T` implements the interface (true) or `T` implements it directly (false).

## File Changes

| File | Changes |
|------|---------|
| `internal/types/checker.go` | NEW — Type checker wrapper |
| `internal/types/interface.go` | NEW — Interface extraction |
| `internal/types/implements.go` | NEW — Implementation detection |
| `internal/indexer/indexer.go` | Add type checking pass after AST parsing |
| `internal/graph/graph.go` | Add indexes, `GetImplementors()`, `GetInterfaces()` |
| `internal/graph/node.go` | Add `Methods []Method` field |
| `internal/mcp/tools.go` | Add tool handlers |
| `internal/mcp/server.go` | Register new tools |
| `README.md` | Document new tools |
| `adr/0014-implicit-interface-resolver.md` | NEW — ADR for this feature |

## Incremental Reindex Behavior

**Current decision:** Type checking runs on full reindex only.

**Rationale:**
- Type relationships span multiple files
- Incremental type checking is complex and error-prone
- File watcher triggers AST reindex, but type checking deferred

**Future consideration:** Add incremental type checking for changed packages only.

## Error Handling

- If type checking fails (missing dependencies, compile errors), log warning and continue with AST-only data
- Return empty results for queries on missing types/interfaces
- Don't block indexing if type checker fails

## Testing Strategy

1. **Unit tests for checker:** Test interface extraction, implementation detection with test fixtures
2. **Unit tests for graph:** Test new indexes and query methods
3. **Integration tests:** Test MCP tools end-to-end
4. **Test fixtures:** Create test packages with various interface patterns:
   - Simple interface implementation
   - Embedded interfaces
   - Pointer receiver methods
   - Empty interface (`interface{}`)
   - Generic interfaces (Go 1.18+)

## Success Criteria

- AI assistants can reliably find implementations of any interface in the project
- AI assistants can find all interfaces a type satisfies, including stdlib
- Pointer receiver distinction is correctly tracked
- Query performance is O(1) via indexes
