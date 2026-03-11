# Dual Embeddings Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store two embeddings per function (raw source + LLM summary) and combine them with weighted cosine similarity scoring at query time.

**Architecture:** Three independent layers are changed in sequence: (1) the Go parser extracts raw function source into `Node.Code`, (2) the vector store schema and API gain a second embedding column, and (3) the orchestration layer generates both embeddings and uses them for search. No new files are created — all changes modify existing files in-place.

**Tech Stack:** Go 1.26, `database/sql` + `modernc.org/sqlite`, standard `go/ast` + `go/token` for source extraction.

---

## File Structure

| File | Change |
|---|---|
| `internal/graph/node.go` | Add `Code string \`json:"-"\`` field to `Node` |
| `internal/parser/go/parser.go` | `os.ReadFile` before parse; extract `node.Code` via byte offsets |
| `internal/parser/go/parser_test.go` | Add `TestParseFile_ExtractsCode` |
| `internal/summary/generator.go` | Add `Code: node.Code` to `SummaryRequest` |
| `internal/vector/store.go` | New schema, `cacheEntry`, `migrateIfNeeded`, `Insert`, `Search`, `HasEmbeddings`, `ScoreNodes`; remove `GetEmbedding` |
| `internal/vector/store_test.go` | Full rewrite for new API; add weighted-score and degraded-score cases |
| `internal/mcp/tools.go` | `ensureFunctionEmbedding`: early exit + dual embed; `semanticBehaviorSearch`: use `ScoreNodes`; remove `math` import |

---

## Chunk 1: Node.Code and Parser

### Task 1: Add `Code` field to `Node` and write failing test

**Files:**
- Modify: `internal/graph/node.go`
- Modify: `internal/parser/go/parser_test.go`

- [ ] **Step 1: Add `Code` field to `Node`**

  In `internal/graph/node.go`, add after the `Docstring` field:

  ```go
  Code      string         `json:"-"` // raw source, not persisted
  ```

  The full struct after the change (lines 16–29):

  ```go
  type Node struct {
      ID        string
      Type      NodeType
      Package   string
      Name      string
      File      string
      Line      int
      Column    int
      Signature string
      Docstring string
      Code      string         `json:"-"` // raw source, not persisted
      Summary   *Summary
      Methods   []Method `json:"methods,omitempty"`
      Metadata  map[string]any
  }
  ```

- [ ] **Step 2: Write failing test for Code extraction**

  Append `TestParseFile_ExtractsCode` to `internal/parser/go/parser_test.go`:

  ```go
  func TestParseFile_ExtractsCode(t *testing.T) {
      tmpDir := t.TempDir()
      goFile := filepath.Join(tmpDir, "test.go")

      code := `package main

  func add(a, b int) int {
      return a + b
  }

  func main() {}
  `
      if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
          t.Fatal(err)
      }

      p := New()
      result, err := p.ParseFile(goFile)
      if err != nil {
          t.Fatalf("ParseFile() error = %v", err)
      }

      var addNode *graph.Node
      for _, n := range result.Nodes {
          if n.Name == "add" {
              addNode = n
              break
          }
      }
      if addNode == nil {
          t.Fatal("add function not found")
      }

      if addNode.Code == "" {
          t.Error("add.Code is empty, want non-empty source")
      }
      if !strings.Contains(addNode.Code, "return a + b") {
          t.Errorf("add.Code = %q, want it to contain the function body", addNode.Code)
      }

      // Methods should NOT have Code populated (only functions)
      // main is a function, not a method — it should also have Code
      var mainNode *graph.Node
      for _, n := range result.Nodes {
          if n.Name == "main" {
              mainNode = n
              break
          }
      }
      if mainNode == nil {
          t.Fatal("main function not found")
      }
      if mainNode.Code == "" {
          t.Error("main.Code is empty, want non-empty source")
      }
  }
  ```

  Also add `"strings"` to the import block in `parser_test.go`.

- [ ] **Step 3: Run test — expect FAIL**

  ```bash
  go test ./internal/parser/go/... -run TestParseFile_ExtractsCode -v
  ```

  Expected: `FAIL` — `add.Code is empty, want non-empty source`

- [ ] **Step 4: Implement Code extraction in parser**

  In `internal/parser/go/parser.go`, make these changes:

  **Add `"os"` to imports:**

  ```go
  import (
      "context"
      "fmt"
      "go/ast"
      goparser "go/parser"
      "go/token"
      "log/slog"
      "os"
      "strings"

      "github.com/thomassaison/mcp-code-graph/internal/debug"
      "github.com/thomassaison/mcp-code-graph/internal/graph"
      "github.com/thomassaison/mcp-code-graph/internal/parser"
      "golang.org/x/tools/go/packages"
  )
  ```

  **Replace the first two lines of `ParseFile` (lines 32–35):**

  Old:
  ```go
  file, err := goparser.ParseFile(p.fset, path, nil, goparser.ParseComments)
  if err != nil {
      return nil, fmt.Errorf("parse file: %w", err)
  }
  ```

  New:
  ```go
  src, err := os.ReadFile(path)
  if err != nil {
      return nil, fmt.Errorf("read file: %w", err)
  }
  file, err := goparser.ParseFile(p.fset, path, src, goparser.ParseComments)
  if err != nil {
      return nil, fmt.Errorf("parse file: %w", err)
  }
  ```

  **Add Code extraction after `node.ID = node.GenerateID()` (currently line 76), before `result.Nodes = append(...)`:**

  ```go
  node.ID = node.GenerateID()

  if node.Type == graph.NodeTypeFunction {
      start := p.fset.Position(fn.Pos()).Offset
      end := p.fset.Position(fn.End()).Offset
      if start >= 0 && end <= len(src) && start < end {
          node.Code = string(src[start:end])
      }
  }

  result.Nodes = append(result.Nodes, node)
  ```

- [ ] **Step 5: Run test — expect PASS**

  ```bash
  go test ./internal/parser/go/... -v
  ```

  Expected: all tests PASS including `TestParseFile_ExtractsCode`

- [ ] **Step 6: Commit**

  ```bash
  git add internal/graph/node.go internal/parser/go/parser.go internal/parser/go/parser_test.go
  git commit -m "feat: extract raw function source into Node.Code at parse time"
  ```

---

### Task 2: Wire `Node.Code` into the summary prompt

**Files:**
- Modify: `internal/summary/generator.go`
- Modify: `internal/summary/generator_test.go`

- [ ] **Step 1: Write the failing test**

  Append to `internal/summary/generator_test.go`:

  ```go
  func TestGenerator_PassesCodeToProvider(t *testing.T) {
      var capturedReq SummaryRequest
      capture := &captureProvider{fn: func(req SummaryRequest) {
          capturedReq = req
      }}
      gen := NewGenerator(capture, "test-model")

      node := &graph.Node{
          Type:      graph.NodeTypeFunction,
          Package:   "main",
          Name:      "add",
          Signature: "func add(a, b int) int",
          Code:      "func add(a, b int) int { return a + b }",
      }

      if err := gen.Generate(context.Background(), node); err != nil {
          t.Fatal(err)
      }
      if capturedReq.Code != node.Code {
          t.Errorf("SummaryRequest.Code = %q, want %q", capturedReq.Code, node.Code)
      }
  }

  type captureProvider struct {
      fn func(SummaryRequest)
  }

  func (c *captureProvider) GenerateSummary(_ context.Context, req SummaryRequest) (string, error) {
      c.fn(req)
      return `{"text":"summary"}`, nil
  }

  func (c *captureProvider) Generate(_ context.Context, _ string) (string, error) {
      return "", nil
  }
  ```

- [ ] **Step 2: Run test — expect FAIL**

  ```bash
  go test ./internal/summary/... -run TestGenerator_PassesCodeToProvider -v
  ```

  Expected: `FAIL` — `SummaryRequest.Code = "", want "func add(a, b int) int { return a + b }"`

- [ ] **Step 3: Add `Code: node.Code` to `SummaryRequest`**

  In `generator.go`, the `req` struct literal (line 32). Add the `Code` field:

  ```go
  req := SummaryRequest{
      FunctionName: node.Name,
      Signature:    node.Signature,
      Package:      node.Package,
      Docstring:    node.Docstring,
      File:         node.File,
      Language:     "Go",
      Code:         node.Code, // wire up existing field
  }
  ```

  (`Code` already exists in `llm.SummaryRequest` — this just passes the value through.)

- [ ] **Step 4: Run tests — expect PASS**

  ```bash
  go test ./internal/summary/... -v
  ```

  Expected: all tests PASS

- [ ] **Step 5: Commit**

  ```bash
  git add internal/summary/generator.go internal/summary/generator_test.go
  git commit -m "feat: pass Node.Code to LLM summary prompt"
  ```

---

## Chunk 2: Vector Store Dual Embeddings

### Task 3: Rewrite vector store internals

**Files:**
- Modify: `internal/vector/store.go`

- [ ] **Step 1: Write the new store_test.go first (TDD red state)**

  Replace the entire contents of `internal/vector/store_test.go` with:

  ```go
  package vector

  import (
      "path/filepath"
      "testing"
  )

  func TestVectorStore_InsertAndSearch(t *testing.T) {
      store := newTestStore(t)

      summaryEmb := []float32{1, 0, 0}
      codeEmb := []float32{0, 1, 0}

      if err := store.Insert("fn1", "does authentication", summaryEmb, "func authenticate()", codeEmb); err != nil {
          t.Fatalf("Insert() error = %v", err)
      }
      if err := store.Insert("fn2", "handles requests", []float32{0, 0, 1}, "func handle()", []float32{1, 0, 0}); err != nil {
          t.Fatalf("Insert() error = %v", err)
      }

      results, err := store.Search(summaryEmb, 1)
      if err != nil {
          t.Fatalf("Search() error = %v", err)
      }
      if len(results) == 0 {
          t.Fatal("Search() returned 0 results")
      }
      if results[0].NodeID != "fn1" {
          t.Errorf("Search()[0].NodeID = %q, want fn1", results[0].NodeID)
      }
      if results[0].Text != "does authentication" {
          t.Errorf("Search()[0].Text = %q, want summary text", results[0].Text)
      }
  }

  func TestVectorStore_WeightedScore(t *testing.T) {
      store := newTestStore(t)

      // query = {1, 0}
      // fn1: summaryEmb={1,0} (perfect match), codeEmb={0,1} (no match)
      //      score = 0.6*1.0 + 0.4*0.0 = 0.6
      // fn2: summaryEmb={0,1} (no match), codeEmb={1,0} (perfect match)
      //      score = 0.6*0.0 + 0.4*1.0 = 0.4
      // fn1 must rank higher than fn2
      query := []float32{1, 0}

      if err := store.Insert("fn1", "summary1", []float32{1, 0}, "code1", []float32{0, 1}); err != nil {
          t.Fatal(err)
      }
      if err := store.Insert("fn2", "summary2", []float32{0, 1}, "code2", []float32{1, 0}); err != nil {
          t.Fatal(err)
      }

      results, err := store.Search(query, 2)
      if err != nil {
          t.Fatal(err)
      }
      if len(results) < 2 {
          t.Fatalf("want 2 results, got %d", len(results))
      }
      if results[0].NodeID != "fn1" {
          t.Errorf("results[0].NodeID = %q, want fn1 (score 0.6)", results[0].NodeID)
      }
      if results[1].NodeID != "fn2" {
          t.Errorf("results[1].NodeID = %q, want fn2 (score 0.4)", results[1].NodeID)
      }
  }

  func TestVectorStore_DegradedScore_SummaryOnly(t *testing.T) {
      store := newTestStore(t)

      summaryEmb := []float32{1, 0}
      if err := store.Insert("fn1", "summary only", summaryEmb, "", nil); err != nil {
          t.Fatal(err)
      }

      results, err := store.Search(summaryEmb, 1)
      if err != nil {
          t.Fatal(err)
      }
      if len(results) == 0 {
          t.Fatal("node with summary-only embedding should appear in results")
      }
      if results[0].NodeID != "fn1" {
          t.Errorf("results[0].NodeID = %q, want fn1", results[0].NodeID)
      }
  }

  func TestVectorStore_DegradedScore_CodeOnly(t *testing.T) {
      store := newTestStore(t)

      codeEmb := []float32{1, 0}
      if err := store.Insert("fn1", "", nil, "func foo()", codeEmb); err != nil {
          t.Fatal(err)
      }

      results, err := store.Search(codeEmb, 1)
      if err != nil {
          t.Fatal(err)
      }
      if len(results) == 0 {
          t.Fatal("node with code-only embedding should appear in results")
      }
  }

  func TestVectorStore_NoEmbeddings_ExcludedFromSearch(t *testing.T) {
      store := newTestStore(t)

      if err := store.Insert("fn1", "", nil, "", nil); err != nil {
          t.Fatal(err)
      }

      results, err := store.Search([]float32{1, 0}, 10)
      if err != nil {
          t.Fatal(err)
      }
      for _, r := range results {
          if r.NodeID == "fn1" {
              t.Error("node with no embeddings should be excluded from search results")
          }
      }
  }

  func TestVectorStore_HasEmbeddings(t *testing.T) {
      store := newTestStore(t)

      store.Insert("both", "s", []float32{1}, "c", []float32{1})   //nolint:errcheck
      store.Insert("summary-only", "s", []float32{1}, "", nil)      //nolint:errcheck
      store.Insert("code-only", "", nil, "c", []float32{1})         //nolint:errcheck
      store.Insert("neither", "", nil, "", nil)                     //nolint:errcheck

      tests := []struct {
          nodeID      string
          wantSummary bool
          wantCode    bool
      }{
          {"both", true, true},
          {"summary-only", true, false},
          {"code-only", false, true},
          {"neither", false, false},
          {"missing", false, false},
      }
      for _, tt := range tests {
          hasSummary, hasCode := store.HasEmbeddings(tt.nodeID)
          if hasSummary != tt.wantSummary || hasCode != tt.wantCode {
              t.Errorf("HasEmbeddings(%q) = (%v, %v), want (%v, %v)",
                  tt.nodeID, hasSummary, hasCode, tt.wantSummary, tt.wantCode)
          }
      }
  }

  func TestVectorStore_ScoreNodes(t *testing.T) {
      store := newTestStore(t)

      store.Insert("fn1", "auth", []float32{1, 0}, "code1", []float32{0, 1}) //nolint:errcheck
      store.Insert("fn2", "http", []float32{0, 1}, "code2", []float32{1, 0}) //nolint:errcheck
      store.Insert("fn3", "db",   []float32{0, 1}, "code3", []float32{0, 1}) //nolint:errcheck

      // Only score fn1 and fn2 (not fn3)
      results := store.ScoreNodes([]float32{1, 0}, []string{"fn1", "fn2"}, 10)
      for _, r := range results {
          if r.NodeID == "fn3" {
              t.Error("fn3 should not appear in ScoreNodes results (not in nodeIDs list)")
          }
      }
      if len(results) == 0 {
          t.Error("expected results for fn1 and fn2")
      }
  }

  func TestVectorStore_Persistence(t *testing.T) {
      tmpDir := t.TempDir()
      dbPath := filepath.Join(tmpDir, "test.db")

      // Insert with first store instance
      s1, err := NewStore(dbPath)
      if err != nil {
          t.Fatal(err)
      }
      s1.Insert("fn1", "summary", []float32{1, 0}, "code", []float32{0, 1}) //nolint:errcheck
      s1.Close()

      // Reload and verify
      s2, err := NewStore(dbPath)
      if err != nil {
          t.Fatal(err)
      }
      defer s2.Close()

      hasSummary, hasCode := s2.HasEmbeddings("fn1")
      if !hasSummary || !hasCode {
          t.Errorf("HasEmbeddings after reload = (%v, %v), want (true, true)", hasSummary, hasCode)
      }
  }

  func newTestStore(t *testing.T) *Store {
      t.Helper()
      tmpDir := t.TempDir()
      store, err := NewStore(filepath.Join(tmpDir, "test.db"))
      if err != nil {
          t.Fatalf("NewStore() error = %v", err)
      }
      t.Cleanup(store.Close)
      return store
  }
  ```

- [ ] **Step 2: Run tests — expect compile errors**

  ```bash
  go test ./internal/vector/... -v
  ```

  Expected: compile errors — wrong number of arguments to `Insert`, undefined `HasEmbeddings`, `ScoreNodes`

- [ ] **Step 3: Rewrite `store.go`**

  Replace the entire contents of `internal/vector/store.go` with:

  ```go
  package vector

  import (
      "database/sql"
      "fmt"
      stdmath "math"
      "sort"
      "sync"

      "github.com/thomassaison/mcp-code-graph/internal/math"
      _ "modernc.org/sqlite"
  )

  type SearchResult struct {
      NodeID string
      Text   string
      Score  float32
  }

  type cacheEntry struct {
      summaryText      string
      codeText         string
      summaryEmbedding []float32
      codeEmbedding    []float32
  }

  type Store struct {
      dbPath string
      db     *sql.DB

      mu    sync.RWMutex
      cache map[string]cacheEntry
  }

  func NewStore(dbPath string) (*Store, error) {
      store := &Store{
          dbPath: dbPath,
          cache:  make(map[string]cacheEntry),
      }
      if err := store.open(); err != nil {
          return nil, err
      }
      if err := store.migrateIfNeeded(); err != nil {
          return nil, err
      }
      if err := store.initTables(); err != nil {
          return nil, err
      }
      if err := store.loadCache(); err != nil {
          return nil, err
      }
      return store, nil
  }

  func (s *Store) open() error {
      db, err := sql.Open("sqlite", s.dbPath)
      if err != nil {
          return fmt.Errorf("open database: %w", err)
      }
      s.db = db
      return nil
  }

  func (s *Store) migrateIfNeeded() error {
      rows, err := s.db.Query(`PRAGMA table_info(embeddings)`)
      if err != nil {
          return fmt.Errorf("check schema: %w", err)
      }
      defer rows.Close()

      for rows.Next() {
          var cid, notNull, pk int
          var name, typ string
          var dfltValue any
          if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
              continue
          }
          if name == "summary_embedding" {
              return nil // already on new schema
          }
      }
      rows.Close()

      // Old schema or no table — drop and let initTables recreate
      _, err = s.db.Exec(`DROP TABLE IF EXISTS embeddings`)
      return err
  }

  func (s *Store) initTables() error {
      _, err := s.db.Exec(`
          CREATE TABLE IF NOT EXISTS embeddings (
              node_id           TEXT PRIMARY KEY,
              summary_text      TEXT,
              code_text         TEXT,
              summary_embedding BLOB,
              code_embedding    BLOB
          );
          CREATE INDEX IF NOT EXISTS idx_embeddings_node ON embeddings(node_id);
      `)
      if err != nil {
          return fmt.Errorf("create tables: %w", err)
      }
      return nil
  }

  func (s *Store) loadCache() error {
      rows, err := s.db.Query(`SELECT node_id, summary_text, code_text, summary_embedding, code_embedding FROM embeddings`)
      if err != nil {
          return fmt.Errorf("load cache: %w", err)
      }
      defer rows.Close()

      for rows.Next() {
          var nodeID, summaryText, codeText string
          var summaryBytes, codeBytes []byte
          if err := rows.Scan(&nodeID, &summaryText, &codeText, &summaryBytes, &codeBytes); err != nil {
              continue
          }
          s.cache[nodeID] = cacheEntry{
              summaryText:      summaryText,
              codeText:         codeText,
              summaryEmbedding: nullableBytesToFloat32(summaryBytes),
              codeEmbedding:    nullableBytesToFloat32(codeBytes),
          }
      }
      return nil
  }

  func nullableBytesToFloat32(b []byte) []float32 {
      if b == nil {
          return nil
      }
      return bytesToFloat32(b)
  }

  func bytesToFloat32(b []byte) []float32 {
      embedding := make([]float32, len(b)/4)
      for i := range embedding {
          bits := uint32(b[i*4]) |
              uint32(b[i*4+1])<<8 |
              uint32(b[i*4+2])<<16 |
              uint32(b[i*4+3])<<24
          embedding[i] = stdmath.Float32frombits(bits)
      }
      return embedding
  }

  func float32ToBytes(f []float32) []byte {
      b := make([]byte, len(f)*4)
      for i, v := range f {
          bits := stdmath.Float32bits(v)
          b[i*4] = byte(bits)
          b[i*4+1] = byte(bits >> 8)
          b[i*4+2] = byte(bits >> 16)
          b[i*4+3] = byte(bits >> 24)
      }
      return b
  }

  // Insert stores both embeddings for a node. Either embedding slice may be nil
  // (the corresponding column is set to NULL). If a row already exists for
  // nodeID, it is fully replaced.
  func (s *Store) Insert(
      nodeID string,
      summaryText string, summaryEmb []float32,
      codeText string, codeEmb []float32,
  ) error {
      var summaryBytes []byte
      if summaryEmb != nil {
          summaryBytes = float32ToBytes(summaryEmb)
      }
      var codeBytes []byte
      if codeEmb != nil {
          codeBytes = float32ToBytes(codeEmb)
      }

      _, err := s.db.Exec(`
          INSERT OR REPLACE INTO embeddings (node_id, summary_text, code_text, summary_embedding, code_embedding)
          VALUES (?, ?, ?, ?, ?)
      `, nodeID, summaryText, codeText, summaryBytes, codeBytes)
      if err != nil {
          return err
      }

      s.mu.Lock()
      s.cache[nodeID] = cacheEntry{
          summaryText:      summaryText,
          codeText:         codeText,
          summaryEmbedding: summaryEmb,
          codeEmbedding:    codeEmb,
      }
      s.mu.Unlock()

      return nil
  }

  // weightedScore computes the combined similarity score for a cache entry.
  // Returns 0 if the entry has no usable embeddings.
  func weightedScore(query []float32, entry cacheEntry) float32 {
      hasSummary := entry.summaryEmbedding != nil
      hasCode := entry.codeEmbedding != nil
      switch {
      case hasSummary && hasCode:
          return 0.6*math.CosineSimilarity(query, entry.summaryEmbedding) +
              0.4*math.CosineSimilarity(query, entry.codeEmbedding)
      case hasSummary:
          return math.CosineSimilarity(query, entry.summaryEmbedding)
      case hasCode:
          return math.CosineSimilarity(query, entry.codeEmbedding)
      default:
          return 0
      }
  }

  func (s *Store) Search(query []float32, limit int) ([]SearchResult, error) {
      s.mu.RLock()
      defer s.mu.RUnlock()

      var results []SearchResult
      for nodeID, entry := range s.cache {
          score := weightedScore(query, entry)
          if score == 0 {
              continue
          }
          results = append(results, SearchResult{
              NodeID: nodeID,
              Text:   entry.summaryText,
              Score:  score,
          })
      }

      sort.Slice(results, func(i, j int) bool {
          return results[i].Score > results[j].Score
      })
      if len(results) > limit {
          results = results[:limit]
      }
      return results, nil
  }

  // HasEmbeddings reports whether summary and code embeddings exist for nodeID.
  // Reads from the in-memory cache only.
  func (s *Store) HasEmbeddings(nodeID string) (hasSummary, hasCode bool) {
      s.mu.RLock()
      defer s.mu.RUnlock()
      entry, ok := s.cache[nodeID]
      if !ok {
          return false, false
      }
      return entry.summaryEmbedding != nil, entry.codeEmbedding != nil
  }

  // ScoreNodes ranks the provided nodeIDs by weighted cosine similarity
  // against query. Only nodes present in the cache are scored.
  func (s *Store) ScoreNodes(query []float32, nodeIDs []string, limit int) []SearchResult {
      s.mu.RLock()
      defer s.mu.RUnlock()

      nodeSet := make(map[string]struct{}, len(nodeIDs))
      for _, id := range nodeIDs {
          nodeSet[id] = struct{}{}
      }

      var results []SearchResult
      for nodeID, entry := range s.cache {
          if _, ok := nodeSet[nodeID]; !ok {
              continue
          }
          score := weightedScore(query, entry)
          if score == 0 {
              continue
          }
          results = append(results, SearchResult{
              NodeID: nodeID,
              Text:   entry.summaryText,
              Score:  score,
          })
      }

      sort.Slice(results, func(i, j int) bool {
          return results[i].Score > results[j].Score
      })
      if len(results) > limit {
          results = results[:limit]
      }
      return results
  }

  func (s *Store) Close() {
      if s.db != nil {
          s.db.Close()
      }
  }
  ```

- [ ] **Step 4: Run tests — expect PASS**

  ```bash
  go test ./internal/vector/... -v
  ```

  Expected: all tests PASS

- [ ] **Step 5: Confirm vector tests pass, note build is not yet clean**

  ```bash
  go test ./internal/vector/... -v
  ```

  Expected: all vector store tests PASS.

  `go build ./...` will fail at this point with errors in `internal/mcp/tools.go` (`Insert` wrong argument count, `GetEmbedding` undefined) — this is expected and will be fixed in Chunk 3 before committing.

---

## Chunk 3: Orchestration

### Task 4: Update `ensureFunctionEmbedding` and `semanticBehaviorSearch`

**Files:**
- Modify: `internal/mcp/tools.go`

- [ ] **Step 1: Replace `ensureFunctionEmbedding` (lines 253–278)**

  Old function:
  ```go
  func (s *Server) ensureFunctionEmbedding(ctx context.Context, node *graph.Node) error {
      slog.Log(ctx, debug.LevelTrace, "ensuring function embedding", "function", node.Name)
      if node.Summary == nil || node.Summary.Text == "" {
          if err := s.summary.Generate(ctx, node); err != nil {
              return fmt.Errorf("generate summary: %w", err)
          }
          if node.Summary != nil {
              s.graph.SetNodeSummary(node.ID, node.Summary) //nolint:errcheck
          }
      }

      text := node.SummaryText()
      if text == "" {
          text = fmt.Sprintf("%s %s", node.Name, node.Signature)
      }

      embedding, err := s.embeddingProvider.Embed(ctx, text)
      if err != nil {
          return fmt.Errorf("embed: %w", err)
      }

      if err := s.vector.Insert(node.ID, text, embedding); err != nil {
          return fmt.Errorf("store embedding: %w", err)
      }

      return nil
  }
  ```

  New function (replace in-place):
  ```go
  func (s *Server) ensureFunctionEmbedding(ctx context.Context, node *graph.Node) error {
      slog.Log(ctx, debug.LevelTrace, "ensuring function embedding", "function", node.Name)

      hasSummary, hasCode := s.vector.HasEmbeddings(node.ID)
      if hasSummary && hasCode {
          return nil // already fully embedded
      }

      if node.Summary == nil || node.Summary.Text == "" {
          if err := s.summary.Generate(ctx, node); err != nil {
              return fmt.Errorf("generate summary: %w", err)
          }
          if node.Summary != nil {
              s.graph.SetNodeSummary(node.ID, node.Summary) //nolint:errcheck
          }
      }

      summaryText := node.SummaryText()
      if summaryText == "" {
          summaryText = fmt.Sprintf("%s %s", node.Name, node.Signature)
      }

      summaryEmb, err := s.embeddingProvider.Embed(ctx, summaryText)
      if err != nil {
          return fmt.Errorf("embed summary: %w", err)
      }

      var codeEmb []float32
      if node.Code != "" {
          codeEmb, err = s.embeddingProvider.Embed(ctx, node.Code)
          if err != nil {
              slog.Warn("failed to embed code, proceeding with summary only", "function", node.Name, "error", err)
              codeEmb = nil
          }
      }

      if err := s.vector.Insert(node.ID, summaryText, summaryEmb, node.Code, codeEmb); err != nil {
          return fmt.Errorf("store embedding: %w", err)
      }

      return nil
  }
  ```

- [ ] **Step 2: Replace `semanticBehaviorSearch` (lines 572–625)**

  Old function:
  ```go
  func (s *Server) semanticBehaviorSearch(ctx context.Context, query string, nodes []*graph.Node, limit int) (string, error) {
      queryEmbedding, err := s.embeddingProvider.Embed(ctx, query)
      if err != nil {
          slog.Debug("embedding failed, returning filtered results", "error", err)
          return s.formatBehaviorResults(nodes, limit), nil
      }

      type scoredNode struct {
          node  *graph.Node
          score float32
      }

      var scoredNodes []scoredNode
      for _, node := range nodes {
          summary := node.SummaryText()
          if summary == "" {
              continue
          }

          if err := s.ensureFunctionEmbedding(ctx, node); err != nil {
              continue
          }

          nodeEmbedding, err := s.vector.GetEmbedding(node.ID)
          if err != nil {
              continue
          }

          score := math.CosineSimilarity(queryEmbedding, nodeEmbedding)
          scoredNodes = append(scoredNodes, scoredNode{node: node, score: score})
      }

      sort.Slice(scoredNodes, func(i, j int) bool {
          return scoredNodes[i].score > scoredNodes[j].score
      })

      if len(scoredNodes) > limit {
          scoredNodes = scoredNodes[:limit]
      }

      var results []map[string]any
      for _, sn := range scoredNodes {
          results = append(results, map[string]any{
              "id":        sn.node.ID,
              "name":      sn.node.Name,
              "package":   sn.node.Package,
              "signature": sn.node.Signature,
              "behaviors": sn.node.Metadata["behaviors"],
              "summary":   sn.node.SummaryText(),
              "score":     sn.score,
          })
      }

      resultJSON, _ := json.MarshalIndent(results, "", "  ")
      return string(resultJSON), nil
  }
  ```

  New function (replace in-place):
  ```go
  func (s *Server) semanticBehaviorSearch(ctx context.Context, query string, nodes []*graph.Node, limit int) (string, error) {
      queryEmbedding, err := s.embeddingProvider.Embed(ctx, query)
      if err != nil {
          slog.Debug("embedding failed, returning filtered results", "error", err)
          return s.formatBehaviorResults(nodes, limit), nil
      }

      nodeIDs := make([]string, 0, len(nodes))
      nodeByID := make(map[string]*graph.Node, len(nodes))
      for _, node := range nodes {
          if node.SummaryText() == "" {
              continue
          }
          if err := s.ensureFunctionEmbedding(ctx, node); err != nil {
              continue
          }
          nodeIDs = append(nodeIDs, node.ID)
          nodeByID[node.ID] = node
      }

      scored := s.vector.ScoreNodes(queryEmbedding, nodeIDs, limit)

      var results []map[string]any
      for _, r := range scored {
          node, ok := nodeByID[r.NodeID]
          if !ok {
              continue
          }
          results = append(results, map[string]any{
              "id":        node.ID,
              "name":      node.Name,
              "package":   node.Package,
              "signature": node.Signature,
              "behaviors": node.Metadata["behaviors"],
              "summary":   node.SummaryText(),
              "score":     r.Score,
          })
      }

      resultJSON, _ := json.MarshalIndent(results, "", "  ")
      return string(resultJSON), nil
  }
  ```

- [ ] **Step 3: Remove the `math` import from `tools.go`**

  In the import block of `internal/mcp/tools.go`, remove:
  ```go
  "github.com/thomassaison/mcp-code-graph/internal/math"
  ```

  Also remove the `sort` import if `sort.Slice` is no longer called in `semanticBehaviorSearch`. Verify by running `go build` — the compiler will flag unused imports.

- [ ] **Step 4: Build — expect clean compile**

  ```bash
  go build ./...
  ```

  Expected: no errors

- [ ] **Step 5: Run all tests**

  ```bash
  go test ./...
  ```

  Expected: all tests PASS

- [ ] **Step 6: Commit vector store + orchestration together**

  The vector store changes (Chunk 2) were not committed to avoid shipping a broken build. Commit everything now that the build is clean:

  ```bash
  git add internal/vector/store.go internal/vector/store_test.go internal/mcp/tools.go
  git commit -m "feat: dual embeddings in vector store and orchestration layer"
  ```

---

## Final verification

- [ ] **Run full test suite**

  ```bash
  go test ./... -v 2>&1 | tail -20
  ```

  Expected: all packages PASS, no failures

- [ ] **Build binary**

  ```bash
  go build ./cmd/mcp-code-graph/...
  ```

  Expected: clean build
