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

### Build and Run

```bash
# Build the binary
go build -o bin/mcp-code-graph ./cmd/mcp-code-graph

# Run in your project directory
cd /path/to/your/go/project
/path/to/bin/mcp-code-graph
```

The server automatically:
- Detects the project from the current working directory
- Creates `.mcp-code-graph/` in the project root for the database

### Command Line Options

- `--model` - LLM model for summaries (empty = mock provider)

### Environment Variables

- `MCP_CODE_GRAPH_DIR` - Override database directory (default: `<project>/.mcp-code-graph`)

## Client Integration

### OpenCode

Add to `~/.config/opencode/opencode.json`:

```json
{
  "mcp": {
    "mcp-code-graph": {
      "type": "local",
      "command": ["/absolute/path/to/bin/mcp-code-graph"],
      "enabled": true
    }
  }
}
```

### Claude Code

Add to your project's `.claude/settings.json` or `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "mcp-code-graph": {
      "command": "/absolute/path/to/bin/mcp-code-graph"
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop config:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`  
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mcp-code-graph": {
      "command": "/absolute/path/to/bin/mcp-code-graph"
    }
  }
}
```

> **Note**: All clients automatically set the working directory to your project root, so no `--project` flag is needed.

### Git Ignore

Add to your project's `.gitignore`:

```
.mcp-code-graph/
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
