# MCP Code Graph — "Best MCP Ever" Upgrade

Date: 2026-03-12
Status: Draft

## Context

The project already has a strong foundation: in-memory graph with callers/callees, semantic search via embeddings, LLM summaries, behavioral tagging, and interface resolution. To become the definitive MCP server for LLM-assisted coding on **huge** Go projects (10k-100k+ functions), three blocks of upgrades are needed:

1. **Scale Foundation** — performance and query precision at scale
2. **Impact Analysis & Architecture Tracing** — the "wow" features unique to a graph
3. **Pattern Discovery & LLM-Optimized Context** — polish and output quality

---

## Block 1: Scale Foundation

### Problem

Current architecture: full graph in memory, flatfile (SQLite) persistence with no indexes, global-only queries. On huge projects this means slow startup, excessive memory, and noisy results.

### Changes

#### 1a. SQLite indexes on the Persister

**File**: `internal/graph/persist.go`

Add indexes once in `NewPersister()` after WAL pragma (not on every `Save()`):
```sql
CREATE INDEX IF NOT EXISTS idx_nodes_package ON nodes(package);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type);
```

This enables efficient package-scoped queries and fast edge lookups without loading the full graph.

#### 1b. Package-scoped and filtered queries on Graph

**File**: `internal/graph/graph.go`

Add these new methods:

- `GetNodesByPackageAndType(pkg string, nodeType NodeType) []*Node` — filter by package AND type
- `GetNeighborsByEdgeType(nodeID string, edgeType EdgeType) []*Node` — get connected nodes filtered by edge type (e.g., only `calls` vs `uses` vs `imports`). Returns nodes, not edges — name reflects that.

#### 1c. Neighborhood exploration

**File**: `internal/graph/graph.go` (already has `GetNeighborhood` — it exists but is unused by MCP tools)

Expose via a new MCP tool:

**File**: `internal/mcp/server.go`, `internal/mcp/tools.go`

New tool: `get_neighborhood`
- Parameters: `node_id: string` (required), `depth: number` (default 2)
- Returns: local graph with nodes + edges within `depth` steps of the given node
- Uses existing `Graph.GetNeighborhood()` method

#### 1d. Scoped search

**File**: `internal/mcp/tools.go`

Add optional `package` parameter to `search_functions`:
- When provided, filter node IDs to only functions in that package, then pass to existing `ScoreNodes()` for scoring
- Reduces noise dramatically on huge monorepos

---

## Block 2: Impact Analysis & Architecture Tracing

### Problem

The LLM needs to understand **consequences** of changes. Currently there's no way to ask "what breaks if I change X?" or "how does data flow between these two points?"

### New MCP Tools

#### 2a. `get_impact` — Reverse call graph traversal

**New file**: `internal/graph/impact.go`

```
func (g *Graph) GetImpact(nodeID string) ImpactReport
```

`ImpactReport` struct:
```go
type ImpactReport struct {
    NodeID       string
    DirectCallers  []*Node    // functions that directly call this
    IndirectCallers []*Node   // functions that call this transitively (depth > 1)
    InterfaceContracts []*Node // interfaces that declare this method signature
    Tests        []*Node    // test functions (name contains "Test") that call this or callers
    RiskLevel    string     // "low", "medium", "high" based on blast radius
    TotalReach   int        // total number of functions affected
}
```

Algorithm: BFS reverse traversal from `nodeID` using `inEdges` with a `visited` map for cycle detection. Mark direct (depth 1) vs indirect (depth > 1). Search for test nodes in the affected set. Assign risk level heuristically based on total reach (low < 5, medium < 20, high >= 20) — these thresholds are rough starting points and may need tuning per project size.

**MCP tool registration** in `server.go`:
- Tool: `get_impact`
- Parameters: `function_id: string` (required)
- Returns: JSON `ImpactReport`

#### 2b. `trace_chain` — Shortest call path between two functions

**New file**: `internal/graph/path.go`

```
func (g *Graph) FindPath(fromID, toID string, maxDepth int) ([]*Node, error)
```

BFS from `fromID` following `calls` edges. Returns the shortest path (list of nodes) or error if no path within `maxDepth`.

**MCP tool registration**:
- Tool: `trace_chain`
- Parameters: `from_id: string` (required), `to_id: string` (required), `max_depth: number` (default 10)
- Returns: JSON array of nodes in the path, or error message

#### 2c. `get_contract` — Function contract summary

**New file**: `internal/graph/contract.go`

```
func (g *Graph) GetContract(nodeID string) Contract
```

`Contract` struct:
```go
type Contract struct {
    Node            *Node     // the function node
    CallerCount     int
    CalleeCount     int
    ReceiverType    string    // for methods, the receiver type (empty for free functions)
    TypeInterfaces  []*Node   // interfaces the receiver type implements (empty for free functions)
    ReturnedTypes   []*Node   // types linked via EdgeTypeReturns edges
    AcceptedTypes   []*Node   // types linked via EdgeTypeAccepts edges
    TestFunctions   []*Node   // tests that exercise this function
}
```

For methods, `TypeInterfaces` is populated by navigating from the receiver type (extracted from `Node.Metadata["receiver"]`) to `byTypeImpl`. For free functions, this field is always empty — this is correct Go semantics (types implement interfaces, not functions).

`ReturnedTypes` and `AcceptedTypes` use existing `EdgeTypeReturns` / `EdgeTypeAccepts` edges already created by the parser — no redundant signature parsing needed.

**MCP tool registration**:
- Tool: `get_contract`
- Parameters: `function_id: string` (required)
- Returns: JSON `Contract`

---

## Block 3: Pattern Discovery & LLM-Optimized Context

### Problem

The LLM needs to understand project conventions and patterns. It also needs context in a form it can immediately use, not raw IDs that require follow-up queries.

### New MCP Tools

#### 3a. `discover_patterns` — Find patterns in a package

**New file**: `internal/graph/patterns.go`

```
func (g *Graph) DiscoverPatterns(pkg string, patternType string) PatternReport
```

Supported `patternType` values:
- `"constructors"` — functions named `New*` in the package
- `"error-handling"` — functions with `error` in signature or `error-handle` behavior
- `"tests"` — test functions in the package (name starts with `Test`)
- `"entrypoints"` — exported functions with no callers within the same package
- `"sources"` — functions with callees but no callers (entry points / potential dead code)
- `"sinks"` — functions with callers but no callees (leaf nodes / terminal operations)
- `"hotspots"` — functions with the most callers in the package (top 10)

**MCP tool registration**:
- Tool: `discover_patterns`
- Parameters: `package: string` (required), `pattern_type: string` (required, one of above)
- Returns: JSON with pattern results, counts, and example nodes

#### 3b. `find_tests` — Test discovery for a function

**File**: `internal/graph/graph.go`

```
func (g *Graph) FindTests(nodeID string) []*Node
```

Algorithm: BFS from `nodeID` following **caller direction only** (`inEdges`). Collect all nodes whose `Name` starts with `Test` or `Benchmark` or `Example`, and/or whose `File` ends with `_test.go`. Traversing callees (outEdges) would incorrectly return helpers called by tests rather than tests that exercise the target function.

**MCP tool registration**:
- Tool: `find_tests`
- Parameters: `function_id: string` (required)
- Returns: JSON array of test function nodes with file and line

#### 3c. `get_function_context` — LLM-ready context block

**New file**: `internal/mcp/context.go`

This is the key LLM optimization tool. Instead of returning IDs and requiring the LMA to make 5 follow-up calls, return everything in one shot.

```
func (s *Server) getFunctionContext(nodeID string) FunctionContext
```

`FunctionContext` struct (JSON output):
```go
type FunctionContext struct {
    Function    *Node           // full node with signature, docstring, summary
    Code        string          // lazily read from file using Node.File + Node.Line
    Callers     []ContextEntry  // each caller: name + signature + 1-line summary
    Callees     []ContextEntry  // each callee: name + signature + 1-line summary
    Contract    Contract        // what interfaces, what types flow
    Tests       []*Node         // test functions
    Package     string          // package name
    File        string          // file path
}
```

**Note on `Code`**: `Node.Code` is tagged `json:"-"` and is never persisted — it's only available during the parsing session. For `get_function_context`, the handler reads the source file using `Node.File` and extracts the function body by scanning from `Node.Line`. This means `Code` works correctly even after server restarts.

Each `ContextEntry` includes enough for the LLM to understand the relationship without a follow-up query:
```go
type ContextEntry struct {
    ID        string
    Name      string
    Package   string
    Signature string
    Summary   string  // 1-line summary if available
}
```

**MCP tool registration**:
- Tool: `get_function_context`
- Parameters: `function_id: string` (required)
- Returns: JSON `FunctionContext` — one call gives the LLM everything it needs

---

## Data Model Changes

### New Types in `internal/graph/`

| Type | Location | Purpose |
|------|----------|---------|
| `ImpactReport` | `impact.go` | Impact analysis result |
| `Contract` | `contract.go` | Function contract |
| `PathResult` | `path.go` | Path finding result |
| `PatternReport` | `patterns.go` | Pattern discovery result |
| `FunctionContext` | `internal/mcp/context.go` | LLM-ready context |

### Graph Index Additions

No new persistent storage needed. All new queries operate on the existing in-memory graph with the new SQLite indexes for efficient loading.

---

## MCP Tool Registration Pattern

Each new MCP tool requires **3 functions** (following the existing pattern):
1. `addXxxTool(mcpServer)` in `server.go` — defines tool schema and registers handler
2. `handleXxxMCP(ctx, CallToolRequest)` in `tools.go` — MCP-compatible handler (extracts params from `CallToolRequest`, calls plain handler)
3. `handleXxx(ctx, map[string]any)` in `tools.go` — plain handler with `map[string]any` args (shared logic)

Plus the tool definition entry in `GetTools()` in `tools.go`.

---

## New Files Summary

| File | Purpose |
|------|---------|
| `internal/graph/impact.go` | `GetImpact()` + `ImpactReport` |
| `internal/graph/path.go` | `FindPath()` + BFS pathfinding |
| `internal/graph/contract.go` | `GetContract()` + `Contract` |
| `internal/graph/patterns.go` | `DiscoverPatterns()` + `PatternReport` |
| `internal/mcp/context.go` | `getFunctionContext()` + `FunctionContext` |

## Modified Files Summary

| File | Changes |
|------|---------|
| `internal/graph/graph.go` | Add `GetNodesByPackageAndType`, `GetNeighborsByEdgeType`, `FindTests` |
| `internal/graph/persist.go` | Add SQLite indexes in `NewPersister()` |
| `internal/mcp/server.go` | Register 6 new tools (`get_neighborhood`, `get_impact`, `trace_chain`, `get_contract`, `discover_patterns`, `find_tests`, `get_function_context`) — each with `addXxxTool()` method |
| `internal/mcp/tools.go` | Add `package` param to `search_functions` handler; add MCP + plain handlers for all 6 new tools; add entries in `GetTools()` |
| `internal/mcp/context.go` | New file — `getFunctionContext()` and `FunctionContext` type |

---

## Implementation Order

1. **Graph methods** — `impact.go`, `path.go`, `contract.go`, `patterns.go`, new methods in `graph.go`
2. **Persist indexes** — `persist.go` index additions
3. **MCP tools** — `server.go` registration, `tools.go` handlers, `context.go`
4. **Tests** — unit tests for all new graph methods
5. **Verify** — `go build ./...` and `go test ./...` pass

---

## Verification

- `go build ./...` compiles without errors
- `go test ./...` passes (existing + new tests)
- `go vet ./...` clean
- All 5 new MCP tools callable and return valid JSON
- `get_impact` correctly identifies blast radius
- `trace_chain` finds shortest path
- `get_contract` returns interface relationships
- `discover_patterns` returns correct pattern groups
- `get_function_context` returns complete LLM-ready context in one call
