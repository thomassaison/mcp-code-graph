# ADR: Structural Changes — 2026-03-11

## Status

Accepted

---

## 1. Cross-Package Call Resolution in the Go Parser

### Context

The AST parser built call graph edges using the **caller's package name** as the target package for all call expressions. For a call like `mcp.NewServer(...)`, the edge target was `func_main_NewServer` instead of `func_github.com/.../internal/mcp_NewServer`. Only same-package calls (unqualified identifiers like `readModulePath`) resolved correctly, making the call graph nearly useless for cross-package analysis.

### Decision

At parse time, build an **import alias map** from the file's `import` block (local alias → full import path). When resolving a `SelectorExpr` call (`pkg.Func`), look up the qualifier in that map. If found, use the full import path as the target package; otherwise fall back to the current package (covers method calls on variables, which require type info to resolve fully).

```
ast.SelectorExpr { X: Ident("mcp"), Sel: Ident("NewServer") }
→ importMap["mcp"] = "github.com/.../internal/mcp"
→ edge.To = "func_github.com/.../internal/mcp_NewServer"
```

### Consequences

- Cross-package function calls now resolve to correct node IDs.
- Method calls on variables (`server.Close()`) still fall back to the caller's package — correct resolution requires type-checker integration, which is a future concern.
- The existing `ParsePackage` ID-rewriting logic remains correct: it only rewrites edges whose `To` still starts with the short package prefix, which no longer applies to cross-package calls.

---

## 2. `Graph.SetNodeSummary` — Write-Back for Defensive Copies

### Context

The `Graph` type returns **clones** from all read methods (`GetNodesByType`, `GetNode`, etc.) to protect internal state from external mutation. This is correct for most uses, but it created a silent bug: `GenerateAll` and `ensureFunctionEmbedding` received clones, set `node.Summary` on them, and the mutation was discarded. The web API (`AllNodes`, which returns raw pointers) served nodes with `Summary == nil`.

### Decision

Add `Graph.SetNodeSummary(id string, summary *Summary) error` as the single authorized write-back path for summaries. It acquires the write lock and mutates the stored node directly.

```go
func (g *Graph) SetNodeSummary(id string, summary *Summary) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    node, ok := g.nodes[id]
    if !ok {
        return ErrNodeNotFound
    }
    node.Summary = summary
    return nil
}
```

All callers that generate a summary (`GenerateAll`, `ensureFunctionEmbedding`) now call `SetNodeSummary` after generation to persist the result into the graph.

### Consequences

- Summaries generated at startup are immediately visible to the web API.
- The defensive-copy contract of `Graph` is preserved: mutation of returned values still has no effect; the only mutation path goes through explicit setter methods.
- `AllNodes` continues to return raw pointers (acceptable since it is internal to the web handler and does not escape to external callers that could corrupt state).

---

## 3. Combined Summary + Embedding Pipeline in `GenerateSummaries`

### Context

`GenerateSummaries` only generated text summaries. Vector embeddings were computed lazily by `ensureFunctionEmbedding` during search. This meant:

- A cold start with no prior searches produced no embeddings.
- The first semantic search was slower (embedding all functions on demand).
- If the process restarted between startup and the first search, no embeddings were cached in the vector store.

### Decision

When an `embeddingProvider` is configured, `Server.GenerateSummaries` delegates entirely to `ensureFunctionEmbedding` for each function, which already implements the full pipeline:

```
generate summary (if missing) → write to graph → compute embedding → store in vector DB
```

When no embedding provider is configured, it falls back to `summary.GenerateAll` (text-only).

### Consequences

- Startup produces a fully populated vector store, making the first semantic search fast.
- No duplicate work: `ensureFunctionEmbedding` skips summary generation for nodes that already have a summary (e.g., loaded from the persisted graph DB).
- `summary.GenerateAll` is now only used in the no-embedding path; its write-back logic (`SetNodeSummary`) is still exercised there.

---

## 4. Asynchronous Summary/Embedding Generation at Startup

### Context

`GenerateSummaries` calls the LLM once per indexed function. For a project with hundreds of functions, this blocked the main goroutine for minutes. The MCP client (Claude Desktop) timed out with `-32001: Request timed out` before the server ever served a request.

### Decision

Move `GenerateSummaries` out of the synchronous startup sequence into a **background goroutine**, launched after the MCP server goroutine is already serving stdio:

```
IndexProject()          ← synchronous, must complete before serving
ServeStdio(mcpSrv)      ← goroutine, server is live
GenerateSummaries()     ← goroutine, runs in background
```

### Consequences

- The MCP server is available immediately after indexing (~seconds), regardless of how long LLM/embedding calls take.
- Semantic search on a cold start (before `GenerateSummaries` finishes) falls back to name-based search, which is the existing behavior.
- If the process is killed before `GenerateSummaries` completes, partial results are saved to the graph DB via `Close()` (which calls `persister.Save`). Completed summaries survive; incomplete ones are regenerated on next startup.
- There is no coordination mechanism to signal when background generation is done. This is acceptable for now; a future improvement could expose a readiness endpoint.

---

## 5. `SummaryRequest` Language and File Fields

### Context

The LLM prompt only had access to function name, signature, package, docstring, and code. The new structured prompt template requires `language` and `file_path` as top-level context fields to improve model accuracy (especially for multi-language support in the future).

### Decision

Add `Language string` and `File string` to `SummaryRequest`. The Go summary generator sets `Language: "Go"` and `File: node.File`. The prompt builder uses these to populate the template; both are optional (fall back to `"Go"` and omit the file line respectively).

### Consequences

- The `LLMProvider` interface is unchanged; only the data carried in `SummaryRequest` is extended.
- `MockProvider` is unaffected (it ignores request fields).
- Future parsers for other languages (TypeScript, Python, etc.) can pass their own language identifier and file path without changing the provider interface.
