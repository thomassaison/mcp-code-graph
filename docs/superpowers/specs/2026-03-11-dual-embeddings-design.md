# Design Spec: Dual Embeddings for Semantic Code Search

**Date:** 2026-03-11
**Status:** Approved

---

## Overview

Each indexed function will be represented by two separate embeddings: one for its raw source code and one for its LLM-generated summary. At query time, cosine similarity scores are computed against both and combined with a weighted average (0.6 × summary + 0.4 × code). This improves semantic recall — the code embedding anchors lexical/structural intent, the summary embedding captures higher-level purpose.

---

## 1. Parser Layer — `Node.Code` Extraction

### Goal

Populate `Node.Code` with the raw Go source of each function, so downstream consumers (LLM prompt builder, embedding pipeline) can use it without re-reading the file.

### Add `Code` to `Node`

Add a `Code string` field to `internal/graph/node.go`:

```go
type Node struct {
    // ... existing fields ...
    Code string `json:"-"` // raw source, not persisted
}
```

The `json:"-"` tag ensures it is excluded from graph DB serialization, keeping the DB compact. It is only populated during the parse phase and used during the summary/embedding generation phase of the same process run. On restart, `Code` is empty; the embedding pipeline skips already-embedded nodes via `HasEmbeddings`.

`Node.Clone()` does not need changes — `string` is a value type and is copied correctly by the existing shallow struct copy.

### Extracting the Source Slice

`ast.FuncDecl.Pos()` and `End()` return `token.Pos` values — positions in the `token.FileSet`, not raw byte offsets. The correct extraction uses `fset`:

```go
start := fset.Position(fn.Pos()).Offset
end   := fset.Position(fn.End()).Offset
node.Code = string(src[start:end])
```

where `src []byte` is the file content read with `os.ReadFile` before calling `goparser.ParseFile`. The parser currently passes `nil` as the source argument; this changes to pass the read bytes so AST positions align with `src`:

```go
src, err := os.ReadFile(filePath)
if err != nil {
    return nil, fmt.Errorf("read file: %w", err)
}
f, err := goparser.ParseFile(fset, filePath, src, goparser.ParseComments)
```

### Scope Note

`Node.Code` is only populated for `NodeTypeFunction` nodes. Method nodes (`NodeTypeMethod`) are not processed by the embedding pipeline — this is intentional for this iteration.

---

## 2. Vector Store Layer

### Schema Migration

The current `embeddings` table has a single `embedding BLOB` column. The new schema introduces two:

```sql
CREATE TABLE embeddings (
    node_id           TEXT PRIMARY KEY,
    summary_text      TEXT,
    code_text         TEXT,
    summary_embedding BLOB,
    code_embedding    BLOB
);
CREATE INDEX IF NOT EXISTS idx_embeddings_node ON embeddings(node_id);
```

**Migration strategy — drop and rebuild.** In `NewStore`, call `migrateIfNeeded()` before `initTables()`. `migrateIfNeeded` uses `PRAGMA table_info(embeddings)` to check whether the `summary_embedding` column exists. If not, it drops the table. `initTables` then creates it with the new schema. The full call order in `NewStore` is:

```
open() → migrateIfNeeded() → initTables() → loadCache()
```

Any previously stored embeddings are regenerated during the next `GenerateSummaries` run — idempotent and acceptable for a local cache.

**`NewStore` body change**: the existing call sequence `open() → initTables() → loadCache()` must be updated to `open() → migrateIfNeeded() → initTables() → loadCache()`.

### New API

**`Insert`** — replaces the existing `Insert(nodeID, text string, embedding []float32) error`:

```go
// Insert stores both embeddings for a node.
// Either embedding slice may be nil — the corresponding column is set to NULL.
// Pass a nil slice as a Go nil (not an empty slice) to produce SQL NULL.
// A non-nil empty slice is serialized as an empty BLOB (valid but useless).
// If a row already exists for nodeID, it is fully replaced.
func (s *Store) Insert(
    nodeID      string,
    summaryText string, summaryEmb []float32,
    codeText    string, codeEmb    []float32,
) error
```

The nil-to-NULL mapping requires a nil guard before `float32ToBytes`:

```go
var summaryBytes []byte
if summaryEmb != nil {
    summaryBytes = float32ToBytes(summaryEmb)
}
var codeBytes []byte
if codeEmb != nil {
    codeBytes = float32ToBytes(codeEmb)
}
// pass summaryBytes and codeBytes to db.Exec — Go nil maps to SQL NULL
```

**`HasEmbeddings`** — new method, replaces `GetEmbedding`:

```go
// HasEmbeddings reports whether summary and code embeddings exist for nodeID.
// Reads from the in-memory cache only; no error is returned because a
// cache miss and "not yet embedded" are equivalent for the caller.
func (s *Store) HasEmbeddings(nodeID string) (hasSummary, hasCode bool)
```

Two booleans are returned rather than a single bool so that partial states can be detected by the orchestration layer (Section 3).

**`ScoreNodes`** — new method, replaces `GetEmbedding` in behavior-search:

```go
// ScoreNodes ranks the provided nodeIDs by weighted cosine similarity
// against query, considering only nodes present in the cache.
// The limit applies to the returned slice.
func (s *Store) ScoreNodes(query []float32, nodeIDs []string, limit int) []SearchResult
```

**`GetEmbedding` is removed.** Its consumers are:
- `internal/mcp/tools.go` — `semanticBehaviorSearch` (see Section 3)

**`store_test.go` must be updated.** The test calls `Insert` with the old three-argument signature and must be rewritten to the new five-argument form. Tests should also verify the weighted scoring and degraded-score cases (summary only, code only).

### Updated `SearchResult`

`SearchResult.Text` returns the summary text (existing behavior preserved). Code text is not surfaced.

```go
type SearchResult struct {
    NodeID string
    Text   string  // summary text
    Score  float32
}
```

### Updated `Search` Implementation

The existing `Search` method iterates `s.cache` and references `entry.embedding`. After the `cacheEntry` struct change, this field no longer exists. `Search` must be updated to compute the weighted score using the new fields:

```go
func (s *Store) Search(query []float32, limit int) ([]SearchResult, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var results []SearchResult
    for nodeID, entry := range s.cache {
        score := weightedScore(query, entry)
        if score == 0 { continue } // no usable embeddings
        results = append(results, SearchResult{
            NodeID: nodeID,
            Text:   entry.summaryText,
            Score:  score,
        })
    }
    // sort and limit as before
    return results, nil
}
```

The `weightedScore` helper encapsulates the scoring table from below and is shared between `Search` and `ScoreNodes`.

### Search Scoring

Both `Search` and `ScoreNodes` use the same `weightedScore` helper:

| Available embeddings | Score |
|---|---|
| Both | `0.6 × cos(query, summaryEmb) + 0.4 × cos(query, codeEmb)` |
| Summary only | `cos(query, summaryEmb)` |
| Code only | `cos(query, codeEmb)` |
| Neither | node excluded from results |

### In-Memory Cache

The `cacheEntry` struct gains separate fields for each embedding:

```go
type cacheEntry struct {
    summaryText      string
    codeText         string
    summaryEmbedding []float32 // nil if not present
    codeEmbedding    []float32 // nil if not present
}
```

`loadCache` reads all five columns. Both `Search` and `ScoreNodes` compute the weighted score entirely in memory.

---

## 3. Orchestration Layer — `ensureFunctionEmbedding`

### Scope

Only `NodeTypeFunction` nodes are processed. `NodeTypeMethod` nodes are excluded — intentional for this iteration, consistent with the existing `GenerateSummaries` loop.

### Updated Flow for `ensureFunctionEmbedding`

```
hasSummary, hasCode := HasEmbeddings(nodeID)
if hasSummary && hasCode {
    return nil  // early exit: fully embedded
}
// fall through for all other states: (false,false), (true,false), (false,true)

generate summary if node.Summary == nil → SetNodeSummary
embed summaryText → summaryEmb
if node.Code != "":
    embed node.Code → codeEmb
else:
    codeEmb = nil
store.Insert(nodeID, summaryText, summaryEmb, node.Code, codeEmb)
```

`Insert` uses `INSERT OR REPLACE`, so partial rows (e.g., from the old schema after a migration) are overwritten with full data.

**Empty `node.Code`**: if the node was loaded from a persisted graph (restart without re-parse), `node.Code` is empty. The code embedding is skipped; only the summary embedding is stored. The node remains searchable via summary alone (degraded scoring, Section 2).

**Summary text fallback**: if summary generation fails or returns an empty string, the existing fallback `text = fmt.Sprintf("%s %s", node.Name, node.Signature)` must be preserved — this becomes the `summaryText` passed to `store.Insert` for that node.

### Updated `semanticBehaviorSearch`

This function currently calls `GetEmbedding` after `ensureFunctionEmbedding` and computes cosine similarity manually. Replace with `ScoreNodes`:

```go
// OLD:
for _, node := range nodes {
    if err := s.ensureFunctionEmbedding(ctx, node); err != nil { continue }
    nodeEmbedding, err := s.vector.GetEmbedding(node.ID)
    if err != nil { continue }
    score := math.CosineSimilarity(queryEmbedding, nodeEmbedding)
    scoredNodes = append(scoredNodes, scoredNode{node: node, score: score})
}

// NEW:
nodeIDs := make([]string, 0, len(nodes))
for _, node := range nodes {
    if err := s.ensureFunctionEmbedding(ctx, node); err != nil { continue }
    nodeIDs = append(nodeIDs, node.ID)
}
results := s.vector.ScoreNodes(queryEmbedding, nodeIDs, limit)
// results is already sorted and limited; format directly
```

The `math.CosineSimilarity` import in `tools.go` can be removed once this refactor is done if it has no other consumers there.

### `SummaryRequest.Code`

`Code string` already exists in `llm.SummaryRequest` but is not currently wired up in `ensureFunctionEmbedding`. The change is:

```go
req := llm.SummaryRequest{
    // ... existing fields ...
    Code: node.Code, // wire up existing field
}
```

When `node.Code` is non-empty, the LLM receives the full function body instead of falling back to the signature — improving prompt quality.

---

## 4. Non-Goals

- **Weighted score tuning**: The 0.6/0.4 split is hardcoded. No config knob.
- **Per-query weight override**: Weights are fixed at search time.
- **Re-embedding on code change**: File watching and incremental re-indexing are out of scope.
- **Method node embedding**: Only `NodeTypeFunction` nodes are embedded. Methods require type-checker integration and are deferred.
- **Multi-language code extraction**: Only Go `ast.FuncDecl` byte offsets are implemented now.
