# Code Quality Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix critical bugs, improve code quality, and address architectural issues identified in the code review.

**Architecture:** The fixes span 5 packages: `graph`, `indexer`, `vector`, `mcp`, and `parser`. Each fix is independent and can be committed separately.

**Tech Stack:** Go 1.21+, SQLite (modernc.org/sqlite), fsnotify, mcp-go

---

## Task 1: Fix RemoveNodesForPackage inEdges Leak

**Problem:** When removing nodes for a package, `RemoveNodesForPackage` deletes `g.inEdges[id]` for removed nodes but doesn't clean up edges pointing TO those nodes from surviving nodes. This leaves stale "ghost edges" in `g.inEdges`.

**Files:**
- Modify: `internal/graph/graph.go:197-253`
- Test: `internal/graph/graph_test.go`

**Step 1: Write the failing test**

Add to `graph_test.go`:

```go
func TestRemoveNodesForPackage_CleansInEdges(t *testing.T) {
    g := New()
    
    g.AddNode(&Node{ID: "func_pkg1_Foo", Type: NodeTypeFunction, Package: "pkg1", Name: "Foo"})
    g.AddNode(&Node{ID: "func_pkg2_Bar", Type: NodeTypeFunction, Package: "pkg2", Name: "Bar"})
    
    g.AddEdge(&Edge{From: "func_pkg2_Bar", To: "func_pkg1_Foo", Type: EdgeTypeCalls})
    
    g.RemoveNodesForPackage("pkg1")
    
    _, err := g.GetNode("func_pkg2_Bar")
    if err != nil {
        t.Fatalf("Bar should still exist: %v", err)
    }
    
    g.mu.RLock()
    inEdges := g.inEdges["func_pkg2_Bar"]
    g.mu.RUnlock()
    
    if len(inEdges) != 0 {
        t.Errorf("inEdges for Bar should be empty, got %d edges", len(inEdges))
        for _, e := range inEdges {
            t.Errorf("  ghost edge: %s -> %s", e.From, e.To)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/graph -v -run TestRemoveNodesForPackage_CleansInEdges`
Expected: FAIL - ghost edges remain

**Step 3: Implement the fix**

In `graph.go`, modify `RemoveNodesForPackage` - after the loop that filters `g.edges`, add:

```go
for toID, edges := range g.inEdges {
    filtered := edges[:0]
    for _, e := range edges {
        if !nodeIDs[e.From] {
            filtered = append(filtered, e)
        }
    }
    g.inEdges[toID] = filtered
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/graph -v -run TestRemoveNodesForPackage_CleansInEdges`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/graph.go internal/graph/graph_test.go
git commit -m "fix: clean up inEdges for surviving nodes when removing package"
```

---

## Task 2: Fix Watcher Debounce Race Condition

**Problem:** In `watcher.go:75-81`, the code sets `pending = nil` immediately after creating the timer. If new events arrive before the timer fires, they go into a new `pending` map, but the timer callback uses the old `snapshot` which may miss events.

**Files:**
- Modify: `internal/indexer/watcher.go:52-89`
- Test: `internal/indexer/watcher_test.go`

**Step 1: Write the failing test**

```go
func TestWatcher_DebounceAccumulates(t *testing.T) {
    dir := t.TempDir()
    goFile := filepath.Join(dir, "test.go")
    os.WriteFile(goFile, []byte("package test"), 0644)
    
    var indexed []string
    var mu sync.Mutex
    idx := &Indexer{
        indexFile: func(path string) {
            mu.Lock()
            indexed = append(indexed, path)
            mu.Unlock()
        },
    }
    
    w, _ := NewWatcher(idx, 50*time.Millisecond)
    defer w.Close()
    w.Watch(dir)
    
    for i := 0; i < 5; i++ {
        os.WriteFile(goFile, []byte(fmt.Sprintf("package test // v%d", i)), 0644)
        time.Sleep(10 * time.Millisecond)
    }
    
    time.Sleep(100 * time.Millisecond)
    
    mu.Lock()
    count := len(indexed)
    mu.Unlock()
    
    if count == 0 {
        t.Error("file should have been indexed at least once")
    }
}
```

**Step 2: Implement the fix**

```go
func (w *Watcher) processEvents() {
    var pending map[string]bool
    var timer *time.Timer
    
    for {
        select {
        case <-w.done:
            if timer != nil {
                timer.Stop()
            }
            return
        case event, ok := <-w.watcher.Events:
            if !ok {
                return
            }
            if !strings.HasSuffix(event.Name, ".go") {
                continue
            }
            if pending == nil {
                pending = make(map[string]bool)
            }
            pending[event.Name] = true
            
            if timer != nil {
                timer.Stop()
            }
            timer = time.AfterFunc(w.debounce, func() {
                for path := range pending {
                    w.indexer.IndexFile(path)
                }
                pending = nil
            })
            
        case err, ok := <-w.watcher.Errors:
            if !ok {
                return
            }
            _ = err
        }
    }
}
```

Key changes:
- Move `pending = nil` into the timer callback
- Don't snapshot pending before the timer fires

**Step 3: Run tests**

Run: `go test ./internal/indexer -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/indexer/watcher.go internal/indexer/watcher_test.go
git commit -m "fix: watcher debounce race - keep accumulating until timer fires"
```

---

## Task 3: Unify cosineSimilarity Implementations

**Problem:** Two different `cosineSimilarity` functions exist:
- `internal/vector/store.go:143-157` - has length check, uses Newton's method sqrt
- `internal/mcp/tools.go:647-658` - no length check, uses math.Sqrt

**Files:**
- Modify: `internal/vector/store.go`, `internal/mcp/tools.go`
- Create: `internal/math/similarity.go`
- Test: `internal/math/similarity_test.go`

**Step 1: Create shared math package**

```go
// internal/math/similarity.go
package math

import "math"

func CosineSimilarity(a, b []float32) float32 {
    if len(a) != len(b) || len(a) == 0 {
        return 0
    }
    
    var dot, normA, normB float32
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    
    if normA == 0 || normB == 0 {
        return 0
    }
    
    return dot / float32(math.Sqrt(float64(normA*normB)))
}
```

**Step 2: Update store.go**

Replace local `cosineSimilarity` and `sqrt32` with import of `internal/math`.

**Step 3: Update tools.go**

Delete local `cosineSimilarity` and `sqrt32` functions (lines 647-662).

**Step 4: Run tests**

Run: `go test ./...`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/math/ internal/vector/store.go internal/mcp/tools.go
git commit -m "refactor: unify cosineSimilarity in shared math package"
```

---

## Task 4: Fix sqrt32 Division-by-Zero Risk

**Note:** This is already addressed in Task 3 by using `math.Sqrt` instead of Newton's method. No separate task needed.

---

## Task 5: Eliminate Dual Handler Pattern

**Problem:** `tools.go` has ~400 lines of boilerplate MCP handler wrappers that just extract args and call internal handlers. Some handlers (`handleGetImplementorsMCP`, `handleGetInterfacesMCP`) re-implement logic instead of delegating.

**Files:**
- Modify: `internal/mcp/tools.go`, `internal/mcp/server.go`

**Step 1: Create generic adapter**

```go
func (s *Server) adaptHandler(
    handler func(context.Context, map[string]any) (string, error),
    requiredArgs ...string,
) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        args := make(map[string]any)
        
        for _, arg := range requiredArgs {
            val, err := req.RequireString(arg)
            if err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }
            args[arg] = val
        }
        
        if reqArgs := req.GetArguments(); reqArgs != nil {
            for k, v := range reqArgs {
                args[k] = v
            }
        }
        
        result, err := handler(ctx, args)
        if err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        return mcp.NewToolResultText(result), nil
    }
}
```

**Step 2: Replace MCP handlers**

Replace ~400 lines of boilerplate with adapter calls.

**Step 3: Fix non-delegating handlers**

`handleGetImplementorsMCP` and `handleGetInterfacesMCP` should delegate to `handleGetImplementors` and `handleGetInterfaces`.

**Step 4: Run tests**

Run: `go test ./internal/mcp -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/server.go
git commit -m "refactor: eliminate 400 lines of MCP handler boilerplate"
```

---

## Task 6: Persistent DB Connection in Persister

**Problem:** `persist.go` opens and closes SQLite on every Save/Load. No connection pooling, no WAL mode.

**Files:**
- Modify: `internal/graph/persist.go`

**Step 1: Refactor Persister to hold connection**

```go
func NewPersister(dbPath string) (*Persister, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, err
    }
    
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
        db.Close()
        return nil, err
    }
    
    return &Persister{dbPath: dbPath, db: db}, nil
}

func (p *Persister) Close() error {
    if p.db != nil {
        return p.db.Close()
    }
    return nil
}
```

**Step 2: Update Save/Load**

Remove `sql.Open` and `defer p.db.Close()` - use `p.db` directly.

**Step 3: Run tests**

Run: `go test ./internal/graph -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/graph/persist.go
git commit -m "perf: persistent DB connection with WAL mode"
```

---

## Task 7: Add Method Receiver Detection in Parser

**Problem:** Parser never sets `NodeTypeMethod`. Functions with receivers are stored as `NodeTypeFunction`, losing receiver type info.

**Files:**
- Modify: `internal/parser/go/parser.go:39-56`
- Test: `internal/parser/go/parser_test.go`

**Step 1: Write the test**

```go
func TestParseFile_MethodReceiver(t *testing.T) {
    src := tempfile(t, `
package test
type Server struct{}
func (s *Server) Handle() {}
func main() {}
`)
    p := New()
    result, err := p.ParseFile(src)
    if err != nil {
        t.Fatal(err)
    }
    
    handleMethod := findNode(result.Nodes, "Handle")
    if handleMethod.Type != NodeTypeMethod {
        t.Errorf("Handle should be NodeTypeMethod, got %v", handleMethod.Type)
    }
    if handleMethod.Metadata["receiver"] != "*Server" {
        t.Errorf("Handle receiver should be *Server, got %v", handleMethod.Metadata["receiver"])
    }
}
```

**Step 2: Implement the fix**

In `ParseFile`, after creating node:

```go
if fn.Recv != nil && len(fn.Recv.List) > 0 {
    node.Type = graph.NodeTypeMethod
    node.Metadata["receiver"] = p.typeString(fn.Recv.List[0].Type)
}
```

**Step 3: Run tests**

Run: `go test ./internal/parser/go -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/parser/go/parser.go internal/parser/go/parser_test.go
git commit -m "feat: detect method receivers and set NodeTypeMethod"
```

---

## Task 8: Defensive Copies for Graph Nodes

**Problem:** `GetNode` returns `*Node` pointer directly into internal map. Callers can mutate without lock.

**Files:**
- Modify: `internal/graph/graph.go:77-86`, `internal/graph/node.go`

**Step 1: Add Clone method to Node**

```go
func (n *Node) Clone() *Node {
    clone := *n
    if n.Metadata != nil {
        clone.Metadata = make(map[string]any, len(n.Metadata))
        for k, v := range n.Metadata {
            clone.Metadata[k] = v
        }
    }
    if n.Summary != nil {
        clone.Summary = &Summary{
            Text:     n.Summary.Text,
            Modified: n.Summary.Modified,
        }
    }
    return &clone
}
```

**Step 2: Update GetNode**

```go
func (g *Graph) GetNode(id string) (*Node, error) {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    node, ok := g.nodes[id]
    if !ok {
        return nil, ErrNodeNotFound
    }
    return node.Clone(), nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/graph -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/graph/graph.go internal/graph/node.go
git commit -m "fix: return defensive copies from graph getters"
```

---

## Task 9: Cache Embeddings in Memory

**Problem:** `Store.Search` loads ALL embeddings from SQLite on every query. O(N) memory and CPU.

**Files:**
- Modify: `internal/vector/store.go`

**Step 1: Add in-memory cache**

```go
type Store struct {
    dbPath string
    db     *sql.DB
    
    mu    sync.RWMutex
    cache map[string]cacheEntry
}

type cacheEntry struct {
    text      string
    embedding []float32
}
```

**Step 2: Load cache on startup**

Add `loadCache()` method called from `NewStore`.

**Step 3: Update Search to use cache**

Read from `s.cache` instead of querying DB.

**Step 4: Update Insert to update cache**

Update cache when inserting new embedding.

**Step 5: Run tests**

Run: `go test ./internal/vector -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/vector/store.go
git commit -m "perf: cache embeddings in memory for O(1) queries"
```

---

## Task 10: Graceful Shutdown

**Problem:** No graceful shutdown. Web server goroutine leaks. No context cancellation.

**Files:**
- Modify: `cmd/mcp-code-graph/main.go`

**Step 1: Add signal handling**

```go
import (
    "context"
    "os/signal"
    "syscall"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // ... existing setup ...
    
    // Web server with shutdown
    if webAddr := os.Getenv("MCP_CODE_GRAPH_WEB"); webAddr != "" {
        go func() {
            srv := &http.Server{Addr: webAddr, Handler: webHandler}
            go func() {
                <-ctx.Done()
                srv.Shutdown(context.Background())
            }()
            if err := srv.ListenAndServe(); err != http.ErrServerClosed {
                slog.Error("Web server error", "error", err)
            }
        }()
    }
    
    select {
    case <-sigChan:
        slog.Info("Shutting down...")
        cancel()
    case <-ctx.Done():
    }
}
```

**Step 2: Commit**

```bash
git add cmd/mcp-code-graph/main.go
git commit -m "feat: graceful shutdown with signal handling"
```

---

## Summary

| Task | Priority | Effort | Risk |
|------|----------|--------|------|
| 1. Fix inEdges leak | P0 High | 15min | Low |
| 2. Fix watcher race | P0 High | 20min | Medium |
| 3. Unify cosineSimilarity | P0 High | 30min | Low |
| 5. Eliminate dual handlers | P1 Medium | 2h | Medium |
| 6. Persistent DB connection | P1 Medium | 30min | Low |
| 7. Method receiver detection | P1 Medium | 30min | Low |
| 8. Defensive copies | P2 Low | 45min | Low |
| 9. Cache embeddings | P2 Low | 1h | Medium |
| 10. Graceful shutdown | P2 Low | 30min | Low |

**Total estimated effort:** ~6 hours
