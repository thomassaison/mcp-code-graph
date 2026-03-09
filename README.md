# MCP Code Graph

An MCP (Model Context Protocol) server that provides a code graph database for AI assistants to understand Go codebases through function summaries, call graphs, and semantic search.

## Features

- **Code Graph**: Functions, types, packages, and their relationships
- **Call Graph**: Find callers and callees for any function
- **Semantic Search**: Search functions by purpose (stub - semantic search not yet implemented)
- **LLM Summaries**: Auto-generated function summaries with human-editable overrides
- **Incremental Indexing**: File watcher + manual full reindex

## Installation

```bash
go install github.com/thomas-saison/mcp-code-graph/cmd/mcp-code-graph@latest
```

## Usage

```bash
# Index a project and start the server
mcp-code-graph --project /path/to/go/project

# Custom database location
mcp-code-graph --project . --db ./data/codegraph.db
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `search_functions` | Search for functions by name (stub - semantic search not yet implemented) |
| `get_callers` | Get all functions that call this function |
| `get_callees` | Get all functions called by this function |
| `reindex_project` | Trigger full reindex |
| `update_summary` | Update a function's summary |

## MCP Resources

- `function://{package}/{name}` - Function details
- `package://{name}` - Package overview

## Configuration

See [ADR-0007](adr/0007-project-structure.md) for architecture details.

## Development

```bash
make build    # Build binary
make test     # Run tests
make run      # Run locally
```

## License

MIT
