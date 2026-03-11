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

### Change

In `internal/parser/go/parser.go`, each `ast.FuncDecl` has `Pos()` and `End()` byte offsets into the source. The parser already receives the file bytes as `src []byte` (passed to `goparser.ParseFile`). Extract the slice:

```go
node.Code = string(src[fn.Pos():fn.End()])
```

`Node.Code` is **not** persisted in the graph DB — it is only populated during the parse phase and used during the summary/embedding generation phase of the same process run. On restart, `Code` is empty; the embedding pipeline checks `HasEmbeddings` and skips nodes that are already embedded.

### Consequences

- The LLM summary prompt will include real function source instead of falling back to just the signature.
- `Node.Code` must not be serialized in the graph persistence layer to avoid bloating the DB.

---

## 2. Vector Store Layer

### Schema Migration

The current `embeddings` table has a single `embedding BLOB` column. The new schema adds a second:

```sql
CREATE TABLE embeddings (
    node_id          TEXT PRIMARY KEY,
    summary_text     TEXT,
    code_text        TEXT,
    summary_embedding BLOB,
    code_embedding    BLOB
);
```

Migration strategy: **drop and rebuild**. On startup, if the existing table lacks the new columns, drop it and recreate. Any previously stored embeddings are regenerated during the next `GenerateSummaries` run. This is acceptable because embedding generation is idempotent and the DB is a local cache.

### New API

```go
// Insert stores both embeddings for a node. Either embedding slice may be nil
// (the corresponding column is left NULL).
func (s *Store) Insert(
    nodeID string,
    summaryText string, summaryEmb []float32,
    codeText    string, codeEmb    []float32,
) error

// HasEmbeddings reports whether summary and code embeddings exist for nodeID.
func (s *Store) HasEmbeddings(nodeID string) (hasSummary, hasCode bool)
```

### Search Scoring

At query time, if both embeddings exist:

```
score = 0.6 × cos(query, summaryEmb) + 0.4 × cos(query, codeEmb)
```

Graceful degradation:

| Available embeddings | Score |
|---|---|
| Both | `0.6 × summary + 0.4 × code` |
| Summary only | `summary score` |
| Code only | `code score` |
| Neither | node excluded from results |

### In-Memory Cache

The `cacheEntry` struct gains two embedding fields:

```go
type cacheEntry struct {
    summaryText      string
    codeText         string
    summaryEmbedding []float32
    codeEmbedding    []float32
}
```

`loadCache` reads both columns on startup. `Search` computes the weighted score entirely in memory.

---

## 3. Orchestration Layer — `ensureFunctionEmbedding`

### Current Flow

```
check vector store → generate summary (if missing) → embed summary text → store
```

### New Flow

```
HasEmbeddings(nodeID) → both present? → return early

generate summary (if missing) → SetNodeSummary
embed summary text → summaryEmb
embed code text (node.Code) → codeEmb
store.Insert(nodeID, summaryText, summaryEmb, codeText, codeEmb)
```

Key points:

- **Early exit**: If `HasEmbeddings` returns `(true, true)`, skip everything. This prevents redundant LLM and embedding API calls on warm restarts.
- **Code text**: `node.Code` is populated by the parser. If `node.Code` is empty (e.g., node was loaded from a persisted graph without re-parsing), the code embedding is skipped and `codeEmb` is nil. The summary embedding alone is stored.
- **Summary text**: The text embedded is the JSON summary string returned by the LLM (same as today). This is stored in `summary_text` for inspection and re-use.
- **Two embedding API calls per function**: One for the summary, one for the code. Both use the same `embeddingProvider`.

### `SummaryRequest.Code`

With `Node.Code` now populated at parse time, `ensureFunctionEmbedding` sets `req.Code = node.Code` before calling the LLM. This improves prompt quality — the model sees the actual function body rather than just the signature.

---

## 4. Non-Goals

- **Weighted score tuning**: The 0.6/0.4 split is hardcoded. No config knob.
- **Per-query weight override**: Weights are fixed at search time.
- **Re-embedding on code change**: File watching and incremental re-indexing are out of scope.
- **TypeScript / multi-language code extraction**: Only Go `ast.FuncDecl` byte offsets are implemented now.
