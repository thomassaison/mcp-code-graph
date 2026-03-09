# 3. Custom Graph Engine with SQLite Persistence

Date: 2026-03-09

## Status

Accepted

## Context

We need a storage backend for:
1. **Graph data**: Functions, types, packages, and their relationships (calls, imports, uses)
2. **Vector data**: Embeddings for semantic similarity search on code and summaries

The system must run locally with embedded databases (no external services).

Options considered:
1. **SQLite + sqlite-vec**: Single file, SQL + vector search
2. **Memgraph Lite + Chroma**: Native graph DB + vector DB
3. **Custom graph engine + SQLite-vec**: In-memory graph with SQLite persistence

## Decision

We will build a **custom in-memory graph engine with SQLite persistence** and **sqlite-vec for embeddings**.

### Architecture

```
┌─────────────────────────────────────────────┐
│              MCP Server (Go)                │
├─────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────────────┐   │
│  │ Graph Engine│  │   Vector Store      │   │
│  │  (in-memory)│  │  (sqlite-vec)       │   │
│  └──────┬──────┘  └──────────┬──────────┘   │
│         │                    │              │
│         └────────┬───────────┘              │
│                  ▼                          │
│         ┌─────────────────┐                 │
│         │  SQLite (disk)  │                 │
│         └─────────────────┘                 │
└─────────────────────────────────────────────┘
```

### Graph Engine Design

The in-memory graph will be optimized for code graph patterns:

```go
type Graph struct {
    nodes    map[string]*Node       // ID -> Node
    edges    map[string][]*Edge     // NodeID -> outgoing edges
    indexes  map[string]map[string][]*Node  // Type -> Name -> Nodes
}

type Node struct {
    ID       string
    Type     NodeType  // Function, Type, Package, File
    Name     string
    Location Location  // File, Line, Column
    Data     any       // Type-specific data
}

type Edge struct {
    From     string
    To       string
    Type     EdgeType  // Calls, Imports, Uses, Defines, Implements
    Metadata map[string]any
}
```

### Persistence

- SQLite stores nodes and edges as normalized tables
- On load: reconstruct in-memory graph from SQLite
- On change: write-through to SQLite (or batched for file watcher updates)
- Vector embeddings stored via sqlite-vec extension

## Consequences

### Positive

- **Fast graph operations**: In-memory traversals, no query overhead
- **Optimized for code patterns**: Can add specialized queries like "find all callers of this function"
- **Single database file**: `.mcp-code-graph/db.sqlite` — easy to backup, version, share
- **Full control**: Can optimize for specific query patterns
- **Simple deployment**: No external services, pure Go + SQLite

### Negative

- **Memory usage**: Entire graph in memory (acceptable for typical codebases)
- **Development effort**: Need to build graph engine from scratch
- **No query language**: Custom API instead of Cypher/Gremlin

### Mitigations

- **Memory**: For very large codebases, could implement node eviction (LRU) with SQLite as backing store
- **Development**: Graph patterns are well-understood; Go makes this straightforward

## Alternatives Considered

### Neo4j/Memgraph

- Powerful query language (Cypher)
- Requires external service (rejected per local-only requirement)
- Memgraph has no mature embedded Go version

### Pure SQLite with Recursive CTEs

- Simple, uses standard SQL
- Graph traversals are slow (recursive CTEs are not optimized for this)
- No native graph algorithms (shortest path, centrality, etc.)

## References

- [sqlite-vec](https://github.com/asg017/sqlite-vec) — SQLite extension for vector search
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — Pure Go SQLite driver
