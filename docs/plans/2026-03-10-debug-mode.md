# Debug Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a debug mode that logs execution details across all major phases (indexing, search, embedding, LLM/summary, persistence) to stderr and optionally a file.

**Architecture:** A new `internal/debug` package exposes a `LevelTrace` slog level constant and a `Setup()` function that configures the global slog logger with a fan-out handler (stderr + optional file). All instrumented packages call `slog.Debug()` / `slog.Log(ctx, debug.LevelTrace, ...)` using the global logger — no injection needed.

**Tech Stack:** Go 1.21+ `log/slog` (standard library), env vars `MCP_CODE_GRAPH_DEBUG` (0/1/2) and `MCP_CODE_GRAPH_DEBUG_FILE`.

---

### Task 1: Create `internal/debug` package

**Files:**
- Create: `internal/debug/debug.go`
- Create: `internal/debug/debug_test.go`

**Step 1: Write the failing test**

```go
// internal/debug/debug_test.go
package debug_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/debug"
)

func TestSetupLevel1EnablesDebug(t *testing.T) {
	var buf bytes.Buffer
	// After Setup(1, ""), the global logger should accept slog.LevelDebug messages.
	if err := debug.Setup(1, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Debug("hello from debug")
	if !strings.Contains(buf.String(), "hello from debug") {
		t.Errorf("expected debug message in output, got: %s", buf.String())
	}
}

func TestSetupLevel2EnablesTrace(t *testing.T) {
	var buf bytes.Buffer
	if err := debug.Setup(2, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Log(nil, debug.LevelTrace, "trace message")
	if !strings.Contains(buf.String(), "trace message") {
		t.Errorf("expected trace message in output, got: %s", buf.String())
	}
}

func TestSetupLevel1SuppressesTrace(t *testing.T) {
	var buf bytes.Buffer
	if err := debug.Setup(1, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Log(nil, debug.LevelTrace, "should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Errorf("trace message should be suppressed at level 1, got: %s", buf.String())
	}
}

func TestSetupLevel0DisablesAll(t *testing.T) {
	var buf bytes.Buffer
	if err := debug.Setup(0, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Debug("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Errorf("debug message should be suppressed at level 0, got: %s", buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /path/to/project && go test ./internal/debug/... -v
```
Expected: compile error — package `debug` does not exist yet.

**Step 3: Write the implementation**

```go
// internal/debug/debug.go
package debug

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// LevelTrace is a custom slog level below LevelDebug, used for level 2 verbosity.
const LevelTrace = slog.Level(-8)

// multiHandler fans out log records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx interface{ Deadline() (interface{}, bool); Done() <-chan struct{}; Err() error; Value(any) any }, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(nil, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx interface{ Deadline() (interface{}, bool); Done() <-chan struct{}; Err() error; Value(any) any }, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
```

Note: the `multiHandler` uses `context.Context` properly. Replace the interface literals above with `context.Context`:

```go
// internal/debug/debug.go
package debug

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

const LevelTrace = slog.Level(-8)

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// Setup configures the global slog logger for debug mode.
// level 0 = off, 1 = debug, 2 = trace.
// filePath is optional; if non-empty, logs are also written to that file (appended).
// w is an optional override writer for testing (pass nil to use os.Stderr).
func Setup(level int, filePath string, w io.Writer) error {
	if level == 0 {
		// Discard all debug output — set logger to warn level.
		h := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})
		slog.SetDefault(slog.New(h))
		return nil
	}

	var minLevel slog.Level
	if level >= 2 {
		minLevel = LevelTrace
	} else {
		minLevel = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: minLevel}

	var writers []io.Writer
	if w != nil {
		writers = append(writers, w)
	} else {
		writers = append(writers, os.Stderr)
	}

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open debug file: %w", err)
		}
		writers = append(writers, f)
	}

	var handlers []slog.Handler
	for _, wr := range writers {
		handlers = append(handlers, slog.NewTextHandler(wr, opts))
	}

	var h slog.Handler
	if len(handlers) == 1 {
		h = handlers[0]
	} else {
		h = &multiHandler{handlers: handlers}
	}

	slog.SetDefault(slog.New(h))
	return nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/debug/... -v
```
Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/debug/debug.go internal/debug/debug_test.go
git commit -m "feat: add debug package with slog-based setup and LevelTrace"
```

---

### Task 2: Wire debug setup into `main.go`

**Files:**
- Modify: `cmd/mcp-code-graph/main.go`

**Step 1: Read existing `main.go` before editing**

File: `cmd/mcp-code-graph/main.go` — already read during planning.

**Step 2: Add debug setup to `main.go`**

Add the following right after `flag.Parse()` (before any other initialization):

```go
// Parse debug config
debugLevel := 0
if v := os.Getenv("MCP_CODE_GRAPH_DEBUG"); v != "" {
    if n, err := strconv.Atoi(v); err == nil {
        debugLevel = n
    }
}
debugFile := os.Getenv("MCP_CODE_GRAPH_DEBUG_FILE")
if err := debug.Setup(debugLevel, debugFile, nil); err != nil {
    log.Printf("warning: failed to setup debug logger: %v", err)
}
```

Also convert the existing `log.Printf` startup lines at the bottom to `slog.Info`:

```go
slog.Info("MCP Code Graph server starting")
slog.Info("project", "path", projectPath)
slog.Info("database", "path", dbPath)
slog.Info("indexed", "functions", server.Graph().NodeCount())
```

Add imports: `"log/slog"`, `"strconv"`, `"github.com/thomassaison/mcp-code-graph/internal/debug"`.

Remove `"log"` import if no longer used (check first).

**Step 3: Build to verify no compile errors**

```bash
go build ./cmd/mcp-code-graph/...
```
Expected: exits 0, no errors.

**Step 4: Smoke test**

```bash
MCP_CODE_GRAPH_DEBUG=1 go run ./cmd/mcp-code-graph/... 2>&1 | head -5
```
Expected: see `level=DEBUG` or `level=INFO` lines on stderr before MCP protocol starts.

**Step 5: Commit**

```bash
git add cmd/mcp-code-graph/main.go
git commit -m "feat: wire debug mode setup in main via MCP_CODE_GRAPH_DEBUG env var"
```

---

### Task 3: Instrument `internal/graph/persist.go`

**Files:**
- Modify: `internal/graph/persist.go`

**Step 1: Add slog calls to `Save` and `Load`**

In `Save`, add at the top of the function body:
```go
slog.Debug("graph save started", "db", p.dbPath)
```
And before `return tx.Commit()`:
```go
slog.Debug("graph save complete", "nodes", len(g.nodes), "db", p.dbPath)
```

In `Load`, add at the top:
```go
slog.Debug("graph load started", "db", p.dbPath)
```
And before `return nil` at the end:
```go
slog.Debug("graph load complete", "db", p.dbPath)
```

Add import: `"log/slog"`.

**Step 2: Run existing persist tests**

```bash
go test ./internal/graph/... -v -run TestPersist
```
Expected: all persist tests PASS (log calls are non-functional changes).

**Step 3: Commit**

```bash
git add internal/graph/persist.go
git commit -m "feat(debug): add slog instrumentation to graph persister"
```

---

### Task 4: Instrument `internal/indexer/indexer.go`

**Files:**
- Modify: `internal/indexer/indexer.go`

**Step 1: Add slog calls to `IndexModule`**

At the top of `IndexModule`:
```go
slog.Debug("indexing module", "root", root)
```

After the loop that adds nodes and edges, before `return nil`:
```go
slog.Debug("indexing module complete", "nodes", len(result.Nodes), "edges", len(result.Edges))
```

**Step 2: Add LevelTrace calls to `resolveEdges`**

At the top of `resolveEdges`:
```go
// (no import needed for debug package — just use slog with the LevelTrace constant)
```

Inside the loop in `resolveEdges` where an edge is resolved:
```go
slog.Log(context.Background(), debug.LevelTrace, "edge resolved", "from", edge.From, "to", edge.To)
```

Add imports: `"context"`, `"log/slog"`, `"github.com/thomassaison/mcp-code-graph/internal/debug"`.

**Step 3: Run existing indexer tests**

```bash
go test ./internal/indexer/... -v
```
Expected: all tests PASS.

**Step 4: Commit**

```bash
git add internal/indexer/indexer.go
git commit -m "feat(debug): add slog instrumentation to indexer"
```

---

### Task 5: Instrument `internal/mcp/tools.go`

**Files:**
- Modify: `internal/mcp/tools.go`

**Step 1: Add slog calls to `handleSearchFunctions`**

At the start of `handleSearchFunctions`:
```go
slog.Debug("search functions", "query", query, "limit", limit)
```

In `handleSearchFunctions`, before `return s.semanticSearch(...)`:
```go
slog.Debug("using semantic search")
```

Before `return s.nameSearch(...)`:
```go
slog.Debug("using name search")
```

**Step 2: Add slog calls to `semanticSearch`**

After embedding the query:
```go
slog.Debug("query embedded", "dim", len(queryEmbedding))
```

After the vector search:
```go
slog.Debug("vector search complete", "results", len(results))
```

**Step 3: Add LevelTrace calls to `ensureFunctionEmbedding`**

At the top:
```go
slog.Log(ctx, debug.LevelTrace, "ensuring function embedding", "function", node.Name)
```

**Step 4: Run existing server tests**

```bash
go test ./internal/mcp/... -v
```
Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat(debug): add slog instrumentation to search and embedding tools"
```

---

### Task 6: Instrument `internal/summary/generator.go`

**Files:**
- Modify: `internal/summary/generator.go`

**Step 1: Add slog calls to `Generate`**

At the top of `Generate` (after the human-summary guard):
```go
slog.Debug("generating summary", "function", node.Name, "package", node.Package)
```

After `g.provider.GenerateSummary(...)` succeeds:
```go
slog.Debug("summary generated", "function", node.Name)
```

**Step 2: Add a LevelTrace call to `GenerateAll`**

At the top of the loop in `GenerateAll`:
```go
slog.Log(ctx, debug.LevelTrace, "processing function for summary", "function", fn.Name)
```

Add imports: `"log/slog"`, `"github.com/thomassaison/mcp-code-graph/internal/debug"`.

**Step 3: Run existing summary tests**

```bash
go test ./internal/summary/... -v
```
Expected: all tests PASS.

**Step 4: Commit**

```bash
git add internal/summary/generator.go
git commit -m "feat(debug): add slog instrumentation to summary generator"
```

---

### Task 7: Instrument `internal/parser/go/parser.go`

**Files:**
- Modify: `internal/parser/go/parser.go`

**Step 1: Add LevelTrace calls to `ParseFile`**

At the top of `ParseFile`:
```go
slog.Log(context.Background(), debug.LevelTrace, "parsing file", "path", path)
```

After building `result.Nodes` (after the loop over `file.Decls`), find where functions are accumulated and add:
```go
slog.Log(context.Background(), debug.LevelTrace, "file parsed", "path", path, "functions", len(result.Nodes))
```

Add imports: `"context"`, `"log/slog"`, `"github.com/thomassaison/mcp-code-graph/internal/debug"`.

**Step 2: Run existing parser tests**

```bash
go test ./internal/parser/... -v
```
Expected: all tests PASS.

**Step 3: Run all tests**

```bash
go test ./...
```
Expected: all tests PASS.

**Step 4: Commit**

```bash
git add internal/parser/go/parser.go
git commit -m "feat(debug): add slog instrumentation to Go parser"
```

---

### Task 8: Update README

**Files:**
- Modify: `README.md`

**Step 1: Add debug mode section**

Find the configuration section in README.md and add:

```markdown
## Debug Mode

Set `MCP_CODE_GRAPH_DEBUG` to enable verbose logging to stderr:

| Value | Effect |
|-------|--------|
| `0` | Off (default) |
| `1` | Basic — indexing, search path, embedding, LLM calls |
| `2` | Verbose — per-file parsing, edge resolution, individual scores |

Optionally write logs to a file (appended) with `MCP_CODE_GRAPH_DEBUG_FILE`:

```json
{
  "env": {
    "MCP_CODE_GRAPH_DEBUG": "1",
    "MCP_CODE_GRAPH_DEBUG_FILE": "/tmp/mcp-code-graph-debug.log"
  }
}
```
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add debug mode documentation"
```
