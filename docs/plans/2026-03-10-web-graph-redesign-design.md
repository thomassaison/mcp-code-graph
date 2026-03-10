# Web Graph Redesign Design

Date: 2026-03-10
Status: Approved

## Overview

Rework the web visualization to be a full-page, graph-first experience. All nodes and edges visible at once, summaries shown as tooltips on hover, edge type toggles to filter relationships.

## Problem

The current implementation uses a tree sidebar + small neighborhood graph panel. Users want to see the full code graph with all relations visible and be able to read summaries directly on hover.

## Goals

- Full-page graph view showing all nodes and edges
- Tooltips on hover showing summary, signature, type, file
- Edge type toggle checkboxes to filter relationships
- Search that highlights/centers matching nodes
- Color-coded nodes by type, styled edges by relationship

## Non-Goals

- Side panel or metadata panel (replaced by tooltips)
- Package tree navigation (replaced by full graph)
- Neighborhood-only views (full graph replaces this)

## Layout

```
┌──────────────────────────────────────────────────────┐
│ Header: Code Graph Explorer    [Search]   [Filters]  │
├──────────────────────────────────────────────────────┤
│                                                      │
│              Full-page Cytoscape.js Graph             │
│           (all nodes + all edges, cose layout)        │
│                                                      │
│  ┌─────────────┐                                     │
│  │ Edge Toggles │  (floating panel, top-left)        │
│  │ ☑ calls     │                                     │
│  │ ☑ implements│                                     │
│  │ ☑ uses      │                                     │
│  │ ...         │                                     │
│  └─────────────┘                                     │
│                                                      │
└──────────────────────────────────────────────────────┘
```

## New API Endpoint

### `GET /api/graph`

Returns all nodes and all edges in one response.

```go
type GraphResponse struct {
    Nodes []NodeResponse `json:"nodes"`
    Edges []EdgeResponse `json:"edges"`
}
```

Existing endpoints remain unchanged (search, stats, packages, etc.).

## Tooltip Content (on hover)

- Node name (bold)
- Type badge (function/method/type/interface)
- File:line
- Signature (monospace)
- Summary text (if available)
- Behaviors (if available)

Uses `tippy.js` via `cytoscape-popper` extension.

## Node Styling

Color-coded by type:
- Function: blue (#53a8e2)
- Method: light blue (#7bc4f0)
- Type: purple (#b48ede)
- Interface: green (#74c69d)
- Package: yellow (#e9c46a)

## Edge Styling

Different colors and patterns per type:
- `calls`: solid blue
- `implements`: dashed green
- `uses`: dotted gray
- `returns`/`accepts`: thin gray
- Labels hidden by default, visible on hover

## Edge Toggles

Floating overlay panel (top-left):
- Checkbox per edge type (calls, implements, uses, returns, accepts, embeds)
- All checked by default
- Unchecking hides edges of that type (and orphaned nodes with no remaining visible edges)

## Search

- Type to filter — matching nodes highlighted, non-matching fade to 20% opacity
- Enter/click result — pan/zoom to center on node
- Clear search restores all nodes

## Click Interaction

- Click node — highlight it + direct neighbors, dim everything else
- Click background — reset to show all
- Double-click node — copy file:line to clipboard

## Layout Algorithm

`cose` (Compound Spring Embedder) — force-directed, good for full graphs in Cytoscape.js.

## Dependencies

Frontend (CDN, no build step):
- Cytoscape.js (already included)
- cytoscape-popper + tippy.js for tooltips

## Implementation Scope

- Add `handleGraph` handler + route
- Replace `static/index.html`, `static/style.css`, `static/app.js`
- Add test for new endpoint
- Keep all existing endpoints and tests

## Testing

- New: `TestHandleGraph` for the `/api/graph` endpoint
- Existing 15 tests unchanged
- Manual browser testing for frontend
