# Web Visualization Design

Date: 2026-03-10
Status: Approved

## Overview

Add a web server to mcp-code-graph that displays the code graph interactively with metadata, enabling visual exploration of packages, nodes, and their relationships.

## Goals

- Visual exploration of indexed code graph
- Browse packages and their nodes (functions, types, interfaces)
- View node neighborhoods (callers, callees, dependencies)
- Search nodes by name
- Display metadata (behaviors, dependencies, signatures)

## Non-Goals

- Authentication/authorization (localhost only)
- Editing the graph (view-only)
- Real-time updates (refresh page to see changes)
- Multiple users (single-user local tool)

## Architecture

### Package Structure

```
internal/web/
├── handler.go     # HTTP handler setup and routing
├── api.go         # API endpoint implementations
├── static/        # Embedded frontend assets
│   ├── index.html
│   ├── style.css
│   └── app.js
└── embed.go       # embed.FS for static assets
```

### Integration

- Embedded in same binary via `embed.FS`
- Enabled via `MCP_CODE_GRAPH_WEB=:8080` env var
- Web handler holds `*graph.Graph`, reads in-memory data directly
- No separate database or persistence layer

## HTTP Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/` | GET | Serve index.html (SPA) |
| `/api/packages` | GET | List all packages |
| `/api/packages/{name}/nodes` | GET | Get nodes in a package |
| `/api/nodes/{id}` | GET | Get single node by ID |
| `/api/nodes/{id}/neighborhood?depth=1-3` | GET | Get node neighborhood |
| `/api/search?q={query}` | GET | Search nodes by name |
| `/api/stats` | GET | Graph statistics |

## Frontend Components

### Layout (CSS Grid)

```
┌──────────────────────────────────────────────────┐
│ Header: Graph Explorer + Search                  │
├─────────────────────┬────────────────────────────┤
│ Package Tree        │ Graph View                 │
│ (native <details>)  │ (Cytoscape.js)            │
│                     │                            │
│                     ├────────────────────────────┤
│                     │ Metadata Panel             │
│                     │ (selected node details)    │
└─────────────────────┴────────────────────────────┘
```

### Components

1. **Tree View** - Native `<details>`/`<summary>` elements for package hierarchy
2. **Graph View** - Cytoscape.js for neighborhood visualization
3. **Metadata Panel** - Node details (type, file, behaviors, dependencies)
4. **Search Bar** - Filter nodes by name

### Interactivity

- Click package → Load nodes, expand tree
- Click node → Show in graph, display metadata
- Adjust depth slider → Change neighborhood depth (1/2/3)
- Search → Filter nodes in tree and graph

## Data Flow

1. UI interaction → User clicks package or searches
2. API call → Frontend fetches `/api/...` endpoint
3. Handler → `handler.go` reads from in-memory `*graph.Graph`
4. Response → JSON returned, frontend updates tree/graph/metadata

### API Response Types

```go
type PackageNode struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Type string `json:"type"` // "function", "type", "interface"
}

type NeighborhoodResponse struct {
    Center  Node   `json:"center"`
    Nodes   []Node `json:"nodes"`
    Edges   []Edge `json:"edges"`
}
```

## Error Handling

| Case | Response |
|------|----------|
| Node not found | 404 `{"error": "node not found"}` |
| Empty search | 200 `[]` |
| Invalid depth | Default to 1, accept 1-3 |
| Missing graph data | Empty state, UI shows "No data indexed" |
| Graph not initialized | 503 Service Unavailable |

**Frontend errors:** Toast notifications for API errors, retry button for network failures.

## Testing

- Unit tests for API handlers using `httptest`
- `NewTestGraph()` helper with sample nodes/edges
- Cover: happy path, 404s, empty results, invalid inputs
- No automated frontend tests (manual browser testing for MVP)

## Implementation Order

1. Create `internal/web/` package structure with `embed.go`
2. Implement API handlers with tests
3. Create basic HTML template with Cytoscape.js
4. Wire up `MCP_CODE_GRAPH_WEB` env var in `main.go`
5. Add frontend interactivity (tree, graph, search)
6. Manual testing, polish UI

## Acceptance Criteria

- `go test ./internal/web/...` passes
- `MCP_CODE_GRAPH_WEB=:8080 mcp-code-graph` serves UI at localhost:8080
- Can browse packages, view nodes, see neighborhoods
- Search returns matching nodes
- Metadata panel shows node details

## Dependencies

- Cytoscape.js (CDN) - Graph visualization
- No additional Go dependencies (standard library `net/http`, `embed`)
