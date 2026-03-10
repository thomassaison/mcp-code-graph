# ADR-0016: Web Visualization

## Status

Accepted

## Context

The mcp-code-graph tool currently provides code graph access only through MCP tools, which requires integration with an MCP client. Users need a more direct way to visually explore the indexed code graph to understand code structure, relationships, and behaviors.

## Decision

Add an embedded web server that serves a single-page application for visual graph exploration.

**Key decisions:**

1. **Embedded in same binary** - Use `embed.FS` to include frontend assets, avoiding separate deployment
2. **Direct graph access** - Web handler reads from in-memory `*graph.Graph` directly, no additional abstraction
3. **Opt-in via environment variable** - `MCP_CODE_GRAPH_WEB=:8080` enables the server, defaults to disabled
4. **Local-only focus** - No authentication, single-user, localhost binding
5. **Hybrid visualization** - Tree view for navigation + Cytoscape.js graph for neighborhoods

## Consequences

**Positive:**
- Easy visual exploration without MCP client setup
- Zero additional deployment complexity (single binary)
- Can view graph structure, relationships, and metadata in browser

**Negative:**
- Increases binary size slightly (embedded assets)
- Another component to maintain (frontend code)
- No real-time updates (must refresh for changes)

**Risks:**
- Cytoscape.js performance with large graphs - Mitigated by neighborhood-focused view, not full graph

## Implementation

See `docs/plans/2026-03-10-web-visualization-design.md` for detailed design.
