# ADR-0012: Debug Mode with Structured Logging

## Status

Accepted

## Context

When running as an MCP server over stdio, the server produces minimal output and it is difficult to understand what is happening during indexing, search, embedding, and summary generation. A debug mode is needed to trace execution through all major phases.

The server already uses `log.Printf` in a few ad-hoc places but there is no systematic approach to debug logging.

Key constraint: stdout is reserved for the MCP protocol. All debug output must go to stderr or a file.

## Decision

We will add a debug mode controlled by two env vars:
- `MCP_CODE_GRAPH_DEBUG` — verbosity level (0=off, 1=basic, 2=verbose)
- `MCP_CODE_GRAPH_DEBUG_FILE` — optional file path to also write logs to (appends)

We will use Go's standard `log/slog` package (available since Go 1.21) as the logging backend. A custom `LevelTrace = slog.Level(-8)` is defined for level 2 verbosity, below `slog.LevelDebug (-4)`.

A `Setup(level int, filePath string) error` function in `internal/debug/debug.go` configures a fan-out handler writing to stderr and optionally a file, then sets it as the global slog default via `slog.SetDefault(...)`.

All instrumented packages call `slog.Debug(...)` or `slog.Log(ctx, debug.LevelTrace, ...)` using the global logger — no logger injection into constructors.

### Log coverage

**Level 1:**
- Startup config, indexing start/end (node/edge counts), search path decision, embedding ops, LLM calls, graph persist events

**Level 2 (in addition):**
- Per-file parsing, per-edge resolution, individual search candidates, vector store operations, embedding vector dimensions

## Consequences

### Positive

- Zero overhead when disabled (slog default handler discards below its minimum level)
- No changes to any component constructors or interfaces
- Structured key-value output is grep-friendly
- File output enables post-mortem inspection when running under a process manager
- Standard library only — no new dependencies

### Negative

- Global logger state means test isolation requires resetting `slog.SetDefault` in tests that care about log output
- Level 2 logging in hot paths (e.g., per-edge resolution) may produce large output for big codebases

### Neutral

- `log.Printf` calls in existing code will be migrated to `slog.Info` / `slog.Warn` as part of this change
