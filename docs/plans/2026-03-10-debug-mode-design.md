# Debug Mode Design

**Date:** 2026-03-10

## Overview

Add a debug mode to mcp-code-graph that logs execution details across all major phases (indexing, search, embedding, LLM/summary, persistence) to help understand what is happening at runtime.

## Configuration

Two env vars control debug mode:

| Env var | Values | Effect |
|---------|--------|--------|
| `MCP_CODE_GRAPH_DEBUG` | `0` (default), `1`, `2` | Sets verbosity level |
| `MCP_CODE_GRAPH_DEBUG_FILE` | file path | Also write logs to this file (appends) |

Debug is off by default (`0`). When enabled, logs go to stderr (which is safe — stdout is reserved for the MCP protocol). If `MCP_CODE_GRAPH_DEBUG_FILE` is set, logs are written to both stderr and the file.

## Verbosity Levels

### Level 1 — Basic debug

High-level execution milestones:
- Startup: debug config parsed (level, file path)
- Indexer: module indexing started/completed with node and edge counts
- Search: which path taken (semantic vs name), query, limit
- Embedding: function embedding generated (ID), query embedding generated
- LLM/Summary: summary generation started/completed per function
- Persister: graph loaded/saved

### Level 2 — Verbose

Granular internals (in addition to level 1):
- Parser: each file parsed, function count per file
- Indexer: each edge resolved (from → to)
- Search: each candidate considered, fallback triggers with reason
- Embedding: vector dimension
- Vector store: insert and search operations with result counts

## Architecture

### New: `internal/debug/debug.go`

Exports:
- `LevelTrace = slog.Level(-8)` — custom level below `slog.LevelDebug` (-4), used for level 2 verbosity
- `Setup(level int, filePath string) error` — called once from `main.go`; builds a fan-out handler (stderr + optional file) and sets it as the global `slog` default

### Changed: `main.go`

- Reads `MCP_CODE_GRAPH_DEBUG` (int, 0=off) and `MCP_CODE_GRAPH_DEBUG_FILE`
- Calls `debug.Setup(...)` before any other initialization
- Converts existing `log.Printf` startup lines to `slog.Info(...)`

### Changed: instrumented packages

No constructor changes. All packages use `slog.Default()` implicitly via `slog.Debug(...)` / `slog.Log(ctx, debug.LevelTrace, ...)`.

| Package | What gets logged |
|---------|-----------------|
| `internal/indexer` | Module index start/end, per-edge resolution (L2) |
| `internal/mcp/tools.go` | Search path decision, query, result count, embedding ops |
| `internal/summary/generator.go` | LLM call start/end per function |
| `internal/graph/persist.go` | Load/save events |
| `internal/parser/go/parser.go` | Per-file parse (L2), function count per file (L2) |

## Approach Considered and Rejected

**Interface-based logger injection** — rejected as over-engineered. Requires touching every component's constructor and type signature for no benefit beyond testability of log output, which is not a goal here.

**Thin `log`-based wrapper** — rejected in favor of `slog` which is standard library since Go 1.21, provides structured key-value output, and natively supports multiple log levels and handlers.
