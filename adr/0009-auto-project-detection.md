# 9. Automatic Project Detection and Database Location

Date: 2026-03-09

## Status

Accepted

## Context

Currently, the MCP server requires explicit flags:
- `--project` to specify the project directory
- `--db` to specify the database path

This creates friction when integrating with MCP clients (OpenCode, Claude Desktop):
- User must specify the project path in the config
- Path must be updated when switching projects
- Configuration is project-specific

MCP clients typically spawn the server with the current working directory set to the project root. We can leverage this for automatic detection.

## Decision

### Remove CLI Flags

Remove `--project` and `--db` flags entirely. The server will auto-detect:

1. **Project directory**: Use current working directory (`os.Getwd()`)
2. **Database location**: Use `<project>/.mcp-code-graph/` by default

### Environment Variable Override

Support `MCP_CODE_GRAPH_DIR` environment variable to override the database directory:

```
# Default behavior
./bin/mcp-code-graph
# → Project: $PWD
# → Database: $PWD/.mcp-code-graph/db

# With env var
MCP_CODE_GRAPH_DIR=/custom/path ./bin/mcp-code-graph
# → Project: $PWD
# → Database: /custom/path/db
```

### Directory Structure

```
<project>/
├── .mcp-code-graph/
│   ├── db.graph.db      # Graph database
│   ├── db.vec.db        # Vector store
│   └── db.sqlite        # (legacy, if used)
├── go.mod
├── go.sum
└── ...
```

### Retained Flags

Keep `--model` flag for LLM model specification (optional).

## Consequences

### Positive

- **Zero configuration**: Works automatically with any MCP client
- **Project-local database**: Database lives with the project
- **Simple integration**: No need to specify paths in OpenCode/Claude config
- **Override available**: Env var for special cases

### Negative

- **Less explicit**: User must know database is in `.mcp-code-graph/`
- **Git pollution**: Should add `.mcp-code-graph/` to `.gitignore`
- **No multiple projects per process**: One server instance per project (expected behavior)

### Mitigations

- Document the `.mcp-code-graph/` directory in README
- Provide `.gitignore` snippet
- Env var available for power users

## Implementation Notes

### main.go Changes

```go
func main() {
    llmModel := flag.String("model", "", "LLM model for summaries (empty = mock)")
    flag.Parse()

    // Auto-detect project directory
    projectPath, err := os.Getwd()
    if err != nil {
        log.Fatalf("Failed to get working directory: %v", err)
    }

    // Determine database directory
    dbDir := os.Getenv("MCP_CODE_GRAPH_DIR")
    if dbDir == "" {
        dbDir = filepath.Join(projectPath, ".mcp-code-graph")
    }

    // Ensure directory exists
    if err := os.MkdirAll(dbDir, 0755); err != nil {
        log.Fatalf("Failed to create database directory: %v", err)
    }

    dbPath := filepath.Join(dbDir, "db")

    // Rest of initialization...
}
```

### OpenCode Configuration (Simplified)

```json
{
  "mcp": {
    "mcp-code-graph": {
      "type": "local",
      "command": ["/path/to/bin/mcp-code-graph"],
      "enabled": true
    }
  }
}
```

No `--project` argument needed - OpenCode sets cwd automatically.

## References

- ADR-0008: MCP Protocol Implementation
- ADR-0003: Custom Graph Engine + SQLite
