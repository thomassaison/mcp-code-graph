# 7. Project Structure

Date: 2026-03-09

## Status

Accepted

## Context

We need to organize the Go module for the mcp-code-graph project. The project has several distinct concerns:
- MCP server (tools and resources)
- Graph engine (in-memory graph + SQLite persistence)
- Vector store (sqlite-vec)
- Code parsing (Go AST)
- Indexing (file watcher + manual reindex)
- Summary generation (LLM integration)

We want a structure that:
- Separates concerns clearly
- Follows Go conventions
- Keeps internal implementation private
- Allows future extensibility (multi-language parsers, alternative LLM providers)

## Decision

We will use a **standard Go project layout** with `cmd/` and `internal/` directories.

### Directory Structure

```
mcp-code-graph/
├── cmd/
│   └── mcp-code-graph/          # Main entrypoint
│       └── main.go
│
├── internal/
│   ├── graph/                   # Graph engine
│   │   ├── graph.go             # In-memory graph implementation
│   │   ├── node.go              # Node types (Function, Type, Package, File)
│   │   ├── edge.go              # Edge types (Calls, Imports, Uses, etc.)
│   │   ├── query.go             # Graph query operations
│   │   └── persist.go           # SQLite persistence layer
│   │
│   ├── vector/                  # Vector store
│   │   ├── store.go             # sqlite-vec wrapper
│   │   └── embeddings.go        # Embedding generation
│   │
│   ├── parser/                  # Code parsing abstraction
│   │   ├── parser.go            # Parser interface
│   │   └── go/                  # Go parser implementation
│   │       ├── parser.go        # Main Go parser
│   │       ├── types.go         # Type information extraction
│   │       └── calls.go         # Call graph extraction
│   │
│   ├── indexer/                 # Indexing logic
│   │   ├── indexer.go           # Main indexer (coordinates parsing + graph building)
│   │   ├── watcher.go           # File watcher (fsnotify)
│   │   └── incremental.go       # Incremental update logic
│   │
│   ├── summary/                 # Summary generation
│   │   ├── generator.go         # LLM summary generation
│   │   └── provider.go          # LLM provider interface
│   │
│   └── mcp/                     # MCP server
│       ├── server.go            # MCP server setup
│       ├── tools.go             # Tool definitions and handlers
│       ├── resources.go         # Resource definitions and handlers
│       └── config.go            # Server configuration
│
├── pkg/                         # Public packages (none initially)
│
├── adr/                         # Architecture Decision Records
│   ├── README.md
│   └── 000X-*.md
│
├── .mcp-code-graph/             # Runtime data (created at runtime)
│   └── db.sqlite                # Graph + vector database
│
├── go.mod
├── go.sum
├── Makefile                     # Build and test commands
└── README.md
```

### Package Responsibilities

| Package | Responsibility |
|---------|---------------|
| `internal/graph` | In-memory graph operations, persistence, queries |
| `internal/vector` | Vector storage, similarity search via sqlite-vec |
| `internal/parser` | Code parsing abstraction, Go implementation |
| `internal/indexer` | Orchestrate parsing → graph building, file watching |
| `internal/summary` | LLM integration for summary generation |
| `internal/mcp` | MCP protocol implementation, tools, resources |

### Dependency Direction

```
cmd/mcp-code-graph
       │
       ▼
   internal/mcp ─────────────┐
       │                     │
       ▼                     ▼
 internal/indexer ──▶ internal/summary
       │                     │
       ▼                     ▼
 internal/parser      internal/vector
       │                     │
       └─────────┬───────────┘
                 ▼
          internal/graph
                 │
                 ▼
            SQLite (via modernc.org/sqlite)
```

## Consequences

### Positive

- **Clear separation**: Each package has a single responsibility
- **Go conventions**: Standard layout familiar to Go developers
- **Encapsulation**: `internal/` prevents accidental external dependencies
- **Extensibility**: `internal/parser` interface allows adding new languages
- **Testability**: Each package can be tested in isolation

### Negative

- **Package count**: Many small packages, more files to navigate
- **Import cycles**: Must be careful about dependency direction

### Mitigations

- Document package boundaries in README
- Use dependency injection to avoid import cycles
- Consider consolidating related packages if boundaries feel artificial

## Alternatives Considered

### Flat Structure (All in root)

```
mcp-code-graph/
├── main.go
├── graph.go
├── parser.go
├── ...
```

- Simpler for small projects
- Becomes unmanageable as project grows
- No clear boundaries

### Domain-Driven (by feature)

```
mcp-code-graph/
├── graph/
│   ├── graph.go
│   ├── parser.go    # Graph-specific parsing
│   └── indexer.go   # Graph-specific indexing
├── mcp/
│   └── ...
```

- Good for domain isolation
- Doesn't match our architecture (shared graph, multiple producers/consumers)
- More complex for this use case

## References

- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Organizing Go Code](https://go.dev/blog/organizing-go-code)
