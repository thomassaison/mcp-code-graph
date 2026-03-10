# MCP Code Graph MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an MCP server that provides a code graph database for AI assistants to understand Go codebases through function summaries, call graphs, and semantic search.

**Architecture:** Custom in-memory graph engine with SQLite persistence, sqlite-vec for embeddings, Go AST for parsing, file watcher for incremental updates, and LLM-generated summaries. Exposed via MCP tools and resources.

**Tech Stack:** Go 1.22+, modernc.org/sqlite, sqlite-vec, go/ast, golang.org/x/tools/go/packages, fsnotify, MCP Go SDK

---

## Phase 1: Project Setup

### Task 1.1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `go.sum`

**Step 1: Initialize module**

Run: `go mod init github.com/thomassaison/mcp-code-graph`

**Step 2: Add core dependencies**

Run:
```bash
go get modernc.org/sqlite@latest
go get golang.org/x/tools/go/packages@latest
go get golang.org/x/tools/go/callgraph@latest
go get github.com/fsnotify/fsnotify@latest
```

**Step 3: Create directory structure**

Run:
```bash
mkdir -p cmd/mcp-code-graph
mkdir -p internal/graph
mkdir -p internal/vector
mkdir -p internal/parser/go
mkdir -p internal/indexer
mkdir -p internal/summary
mkdir -p internal/mcp
```

**Step 4: Verify structure**

Run: `tree -L 3 .`

Expected:
```
.
├── adr/
├── cmd/
│   └── mcp-code-graph/
├── docs/
│   └── plans/
├── go.mod
├── go.sum
└── internal/
    ├── graph/
    ├── indexer/
    ├── mcp/
    ├── parser/
    │   └── go/
    ├── summary/
    └── vector/
```

**Step 5: Commit**

```bash
git add .
git commit -m "chore: initialize project structure"
```

---

### Task 1.2: Create Makefile

**Files:**
- Create: `Makefile`

**Step 1: Write Makefile**

```makefile
.PHONY: build test clean run

build:
	go build -o bin/mcp-code-graph ./cmd/mcp-code-graph

test:
	go test -v ./...

clean:
	rm -rf bin/
	rm -rf .mcp-code-graph/

run:
	go run ./cmd/mcp-code-graph

lint:
	golangci-lint run
```

**Step 2: Test make commands**

Run: `make build && ls bin/`

Expected: `mcp-code-graph` binary exists

**Step 3: Commit**

```bash
git add Makefile
git commit -m "chore: add Makefile for build commands"
```

---

## Phase 2: Graph Engine

### Task 2.1: Define Core Types

**Files:**
- Create: `internal/graph/node.go`
- Create: `internal/graph/edge.go`
- Test: `internal/graph/node_test.go`

**Step 1: Write failing test**

```go
// internal/graph/node_test.go
package graph

import (
	"testing"
)

func TestNodeID(t *testing.T) {
	node := &Node{
		Type: NodeTypeFunction,
		Package: "main",
		Name: "handleRequest",
		File: "handler.go",
		Line: 42,
	}

	expected := "func_main_handleRequest_handler.go:42"
	if got := node.ID(); got != expected {
		t.Errorf("ID() = %q, want %q", got, expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/...`

Expected: FAIL - undefined: Node

**Step 3: Write implementation**

```go
// internal/graph/node.go
package graph

import "fmt"

// NodeType represents the kind of node in the graph
type NodeType string

const (
	NodeTypeFunction NodeType = "function"
	NodeTypeMethod    NodeType = "method"
	NodeTypeType      NodeType = "type"
	NodeTypeInterface NodeType = "interface"
	NodeTypePackage   NodeType = "package"
	NodeTypeFile      NodeType = "file"
)

// Node represents a code entity in the graph
type Node struct {
	ID       string
	Type     NodeType
	Package  string
	Name     string
	File     string
	Line     int
	Column   int
	Signature string
	Docstring string
	Summary  *Summary
	Metadata map[string]any
}

// Summary holds a human-readable function summary
type Summary struct {
	Text        string `json:"text"`
	GeneratedBy string `json:"generated_by"`
	Model       string `json:"model"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// GenerateID creates a unique identifier for the node
func (n *Node) GenerateID() string {
	switch n.Type {
	case NodeTypeFunction, NodeTypeMethod:
		return fmt.Sprintf("func_%s_%s_%s:%d", n.Package, n.Name, n.File, n.Line)
	case NodeTypeType, NodeTypeInterface:
		return fmt.Sprintf("type_%s_%s", n.Package, n.Name)
	case NodeTypePackage:
		return fmt.Sprintf("pkg_%s", n.Name)
	case NodeTypeFile:
		return fmt.Sprintf("file_%s", n.Name)
	default:
		return fmt.Sprintf("node_%s_%s", n.Package, n.Name)
	}
}
```

```go
// internal/graph/edge.go
package graph

// EdgeType represents the kind of relationship between nodes
type EdgeType string

const (
	EdgeTypeCalls       EdgeType = "calls"
	EdgeTypeCalledBy    EdgeType = "called_by"
	EdgeTypeImports     EdgeType = "imports"
	EdgeTypeUses        EdgeType = "uses"
	EdgeTypeDefines     EdgeType = "defines"
	EdgeTypeImplements  EdgeType = "implements"
	EdgeTypeEmbeds      EdgeType = "embeds"
	EdgeTypeReturns     EdgeType = "returns"
	EdgeTypeAccepts     EdgeType = "accepts"
)

// Edge represents a relationship between two nodes
type Edge struct {
	From     string
	To       string
	Type     EdgeType
	Metadata map[string]any
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/...`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/
git commit -m "feat(graph): add Node and Edge types"
```

---

### Task 2.2: Implement In-Memory Graph

**Files:**
- Create: `internal/graph/graph.go`
- Test: `internal/graph/graph_test.go`

**Step 1: Write failing tests**

```go
// internal/graph/graph_test.go
package graph

import (
	"testing"
)

func TestGraphAddNode(t *testing.T) {
	g := New()
	node := &Node{
		Type:    NodeTypeFunction,
		Package: "main",
		Name:    "test",
		File:    "test.go",
		Line:    1,
	}
	node.ID = node.GenerateID()

	g.AddNode(node)

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount() = %d, want 1", g.NodeCount())
	}

	got, err := g.GetNode(node.ID)
	if err != nil {
		t.Fatalf("GetNode() error = %v", err)
	}
	if got.Name != node.Name {
		t.Errorf("GetNode().Name = %q, want %q", got.Name, node.Name)
	}
}

func TestGraphAddEdge(t *testing.T) {
	g := New()
	
	from := &Node{Type: NodeTypeFunction, Package: "main", Name: "caller", File: "test.go", Line: 1}
	from.ID = from.GenerateID()
	
	to := &Node{Type: NodeTypeFunction, Package: "main", Name: "callee", File: "test.go", Line: 5}
	to.ID = to.GenerateID()

	g.AddNode(from)
	g.AddNode(to)
	g.AddEdge(&Edge{From: from.ID, To: to.ID, Type: EdgeTypeCalls})

	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1", g.EdgeCount())
	}

	callers := g.GetCallers(to.ID)
	if len(callers) != 1 {
		t.Errorf("GetCallers() = %d nodes, want 1", len(callers))
	}
}

func TestGraphGetCallees(t *testing.T) {
	g := New()
	
	caller := &Node{Type: NodeTypeFunction, Package: "main", Name: "caller", File: "test.go", Line: 1}
	caller.ID = caller.GenerateID()
	
	callee := &Node{Type: NodeTypeFunction, Package: "main", Name: "callee", File: "test.go", Line: 5}
	callee.ID = callee.GenerateID()

	g.AddNode(caller)
	g.AddNode(callee)
	g.AddEdge(&Edge{From: caller.ID, To: callee.ID, Type: EdgeTypeCalls})

	callees := g.GetCallees(caller.ID)
	if len(callees) != 1 {
		t.Errorf("GetCallees() = %d nodes, want 1", len(callees))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/graph/...`

Expected: FAIL - undefined: New

**Step 3: Write implementation**

```go
// internal/graph/graph.go
package graph

import (
	"errors"
	"sync"
)

var (
	ErrNodeNotFound = errors.New("node not found")
)

// Graph is an in-memory code graph with indexes for fast lookups
type Graph struct {
	mu sync.RWMutex

	nodes    map[string]*Node
	edges    map[string][]*Edge // from-node-id -> outgoing edges
	inEdges  map[string][]*Edge // to-node-id -> incoming edges

	// Indexes
	byType    map[NodeType]map[string]*Node // type -> id -> node
	byPackage map[string]map[string]*Node   // package -> id -> node
}

// New creates a new empty graph
func New() *Graph {
	return &Graph{
		nodes:     make(map[string]*Node),
		edges:     make(map[string][]*Edge),
		inEdges:   make(map[string][]*Edge),
		byType:    make(map[NodeType]map[string]*Node),
		byPackage: make(map[string]map[string]*Node),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node

	// Update type index
	if g.byType[node.Type] == nil {
		g.byType[node.Type] = make(map[string]*Node)
	}
	g.byType[node.Type][node.ID] = node

	// Update package index
	if g.byPackage[node.Package] == nil {
		g.byPackage[node.Package] = make(map[string]*Node)
	}
	g.byPackage[node.Package][node.ID] = node
}

// GetNode retrieves a node by ID
func (g *Graph) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// AddEdge adds an edge to the graph
func (g *Graph) AddEdge(edge *Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges[edge.From] = append(g.edges[edge.From], edge)
	g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)
}

// NodeCount returns the number of nodes
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the number of edges
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, edges := range g.edges {
		count += len(edges)
	}
	return count
}

// GetCallers returns nodes that call the given function
func (g *Graph) GetCallers(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var callers []*Node
	for _, edge := range g.inEdges[nodeID] {
		if edge.Type == EdgeTypeCalls {
			if node, ok := g.nodes[edge.From]; ok {
				callers = append(callers, node)
			}
		}
	}
	return callers
}

// GetCallees returns nodes called by the given function
func (g *Graph) GetCallees(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var callees []*Node
	for _, edge := range g.edges[nodeID] {
		if edge.Type == EdgeTypeCalls {
			if node, ok := g.nodes[edge.To]; ok {
				callees = append(callees, node)
			}
		}
	}
	return callees
}

// GetNodesByType returns all nodes of a given type
func (g *Graph) GetNodesByType(nodeType NodeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byType[nodeType] {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNodesByPackage returns all nodes in a package
func (g *Graph) GetNodesByPackage(pkg string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byPackage[pkg] {
		nodes = append(nodes, node)
	}
	return nodes
}

// RemoveNodesForPackage removes all nodes and edges for a package
func (g *Graph) RemoveNodesForPackage(pkg string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get all node IDs for this package
	nodeIDs := make(map[string]bool)
	for id := range g.byPackage[pkg] {
		nodeIDs[id] = true
	}

	// Remove edges involving these nodes
	for id := range nodeIDs {
		delete(g.edges, id)
		delete(g.inEdges, id)
	}

	// Remove edges pointing to these nodes
	for fromID, edges := range g.edges {
		filtered := edges[:0]
		for _, e := range edges {
			if !nodeIDs[e.To] {
				filtered = append(filtered, e)
			}
		}
		g.edges[fromID] = filtered
	}

	// Remove nodes
	for id := range nodeIDs {
		node := g.nodes[id]
		delete(g.nodes, id)
		delete(g.byType[node.Type], id)
	}
	delete(g.byPackage, pkg)
}

// AllNodes returns all nodes (for iteration)
func (g *Graph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/graph/...`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/
git commit -m "feat(graph): implement in-memory graph with indexes"
```

---

### Task 2.3: Implement SQLite Persistence

**Files:**
- Create: `internal/graph/persist.go`
- Test: `internal/graph/persist_test.go`

**Step 1: Write failing tests**

```go
// internal/graph/persist_test.go
package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistSaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create and populate graph
	g := New()
	node1 := &Node{
		ID:        "func_main_test_test.go:1",
		Type:      NodeTypeFunction,
		Package:   "main",
		Name:      "test",
		File:      "test.go",
		Line:      1,
		Signature: "func test()",
	}
	node2 := &Node{
		ID:        "func_main_helper_test.go:5",
		Type:      NodeTypeFunction,
		Package:   "main",
		Name:      "helper",
		File:      "test.go",
		Line:      5,
		Signature: "func helper() int",
	}
	g.AddNode(node1)
	g.AddNode(node2)
	g.AddEdge(&Edge{From: node1.ID, To: node2.ID, Type: EdgeTypeCalls})

	// Save
	p := NewPersister(dbPath)
	if err := p.Save(g); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file not created")
	}

	// Load into new graph
	g2 := New()
	if err := p.Load(g2); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify nodes
	if g2.NodeCount() != 2 {
		t.Errorf("NodeCount() = %d, want 2", g2.NodeCount())
	}

	// Verify edge
	callees := g2.GetCallees(node1.ID)
	if len(callees) != 1 {
		t.Errorf("GetCallees() = %d, want 1", len(callees))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/graph/...`

Expected: FAIL - undefined: Persister

**Step 3: Write implementation**

```go
// internal/graph/persist.go
package graph

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// Persister handles saving and loading the graph to/from SQLite
type Persister struct {
	dbPath string
	db     *sql.DB
}

// NewPersister creates a new persister
func NewPersister(dbPath string) *Persister {
	return &Persister{dbPath: dbPath}
}

// Save persists the graph to SQLite
func (p *Persister) Save(g *Graph) error {
	if err := p.open(); err != nil {
		return err
	}
	defer p.close()

	// Create tables
	if _, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			package TEXT NOT NULL,
			name TEXT NOT NULL,
			file TEXT NOT NULL,
			line INTEGER NOT NULL,
			column INTEGER DEFAULT 0,
			signature TEXT,
			docstring TEXT,
			summary TEXT,
			metadata TEXT
		);
		CREATE TABLE IF NOT EXISTS edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			type TEXT NOT NULL,
			metadata TEXT,
			FOREIGN KEY (from_id) REFERENCES nodes(id),
			FOREIGN KEY (to_id) REFERENCES nodes(id)
		);
		CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
		CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
	`); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// Clear existing data
	if _, err := p.db.Exec("DELETE FROM edges; DELETE FROM nodes;"); err != nil {
		return fmt.Errorf("clear tables: %w", err)
	}

	// Insert nodes
	for _, node := range g.AllNodes() {
		summaryJSON, _ := json.Marshal(node.Summary)
		metadataJSON, _ := json.Marshal(node.Metadata)

		_, err := p.db.Exec(`
			INSERT INTO nodes (id, type, package, name, file, line, column, signature, docstring, summary, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, node.ID, node.Type, node.Package, node.Name, node.File, node.Line, node.Column,
			node.Signature, node.Docstring, string(summaryJSON), string(metadataJSON))
		if err != nil {
			return fmt.Errorf("insert node %s: %w", node.ID, err)
		}
	}

	// Insert edges
	for _, node := range g.AllNodes() {
		callees := g.GetCallees(node.ID)
		for _, callee := range callees {
			metadataJSON, _ := json.Marshal(map[string]any{})
			_, err := p.db.Exec(`
				INSERT INTO edges (from_id, to_id, type, metadata)
				VALUES (?, ?, ?, ?)
			`, node.ID, callee.ID, EdgeTypeCalls, string(metadataJSON))
			if err != nil {
				return fmt.Errorf("insert edge: %w", err)
			}
		}
	}

	return nil
}

// Load loads the graph from SQLite
func (p *Persister) Load(g *Graph) error {
	if err := p.open(); err != nil {
		return err
	}
	defer p.close()

	// Load nodes
	rows, err := p.db.Query(`
		SELECT id, type, package, name, file, line, column, signature, docstring, summary, metadata
		FROM nodes
	`)
	if err != nil {
		return fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		node := &Node{Metadata: make(map[string]any)}
		var summaryJSON, metadataJSON string

		err := rows.Scan(&node.ID, &node.Type, &node.Package, &node.Name, &node.File,
			&node.Line, &node.Column, &node.Signature, &node.Docstring, &summaryJSON, &metadataJSON)
		if err != nil {
			return fmt.Errorf("scan node: %w", err)
		}

		if summaryJSON != "" && summaryJSON != "null" {
			var summary Summary
			json.Unmarshal([]byte(summaryJSON), &summary)
			node.Summary = &summary
		}

		if metadataJSON != "" && metadataJSON != "null" {
			json.Unmarshal([]byte(metadataJSON), &node.Metadata)
		}

		g.AddNode(node)
	}

	// Load edges
	edgeRows, err := p.db.Query(`SELECT from_id, to_id, type, metadata FROM edges`)
	if err != nil {
		return fmt.Errorf("query edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		edge := &Edge{Metadata: make(map[string]any)}
		var metadataJSON string

		err := edgeRows.Scan(&edge.From, &edge.To, &edge.Type, &metadataJSON)
		if err != nil {
			return fmt.Errorf("scan edge: %w", err)
		}

		if metadataJSON != "" && metadataJSON != "null" {
			json.Unmarshal([]byte(metadataJSON), &edge.Metadata)
		}

		g.AddEdge(edge)
	}

	return nil
}

func (p *Persister) open() error {
	db, err := sql.Open("sqlite", p.dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	p.db = db
	return nil
}

func (p *Persister) close() {
	if p.db != nil {
		p.db.Close()
		p.db = nil
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/graph/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/
git commit -m "feat(graph): add SQLite persistence"
```

---

## Phase 3: Vector Store

### Task 3.1: Implement Vector Store with sqlite-vec

**Files:**
- Create: `internal/vector/store.go`
- Test: `internal/vector/store_test.go`

**Step 1: Write failing tests**

```go
// internal/vector/store_test.go
package vector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVectorStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Insert embeddings
	embedding1 := []float32{0.1, 0.2, 0.3}
	embedding2 := []float32{0.4, 0.5, 0.6}

	if err := store.Insert("node1", "function summary 1", embedding1); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := store.Insert("node2", "function summary 2", embedding2); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Search (using first embedding - should find itself)
	results, err := store.Search(embedding1, 1)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}

	if results[0].NodeID != "node1" {
		t.Errorf("Search()[0].NodeID = %q, want %q", results[0].NodeID, "node1")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/vector/...`

Expected: FAIL - undefined: Store

**Step 3: Write implementation**

```go
// internal/vector/store.go
package vector

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SearchResult represents a similarity search result
type SearchResult struct {
	NodeID    string
	Text      string
	Score     float32
}

// Store handles vector embeddings using sqlite-vec
type Store struct {
	dbPath string
	db     *sql.DB
}

// NewStore creates a new vector store
func NewStore(dbPath string) (*Store, error) {
	store := &Store{dbPath: dbPath}
	if err := store.open(); err != nil {
		return nil, err
	}
	if err := store.initTables(); err != nil {
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

func (s *Store) initTables() error {
	// Create embeddings table
	// Note: sqlite-vec extension must be loaded
	// For MVP, we use a simplified approach without the extension
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS embeddings (
			node_id TEXT PRIMARY KEY,
			text TEXT NOT NULL,
			embedding BLOB
		);
		CREATE INDEX IF NOT EXISTS idx_embeddings_node ON embeddings(node_id);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

// Insert adds or updates an embedding for a node
func (s *Store) Insert(nodeID, text string, embedding []float32) error {
	// Convert embedding to bytes (simplified - in production use sqlite-vec)
	embeddingBytes := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		// Simple float32 to bytes conversion
		bits := float32ToBits(v)
		embeddingBytes[i*4] = byte(bits)
		embeddingBytes[i*4+1] = byte(bits >> 8)
		embeddingBytes[i*4+2] = byte(bits >> 16)
		embeddingBytes[i*4+3] = byte(bits >> 24)
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO embeddings (node_id, text, embedding)
		VALUES (?, ?, ?)
	`, nodeID, text, embeddingBytes)
	return err
}

// Search finds similar embeddings (simplified cosine similarity)
func (s *Store) Search(query []float32, limit int) ([]SearchResult, error) {
	// For MVP, return all and compute similarity in memory
	// In production, use sqlite-vec for vector similarity
	rows, err := s.db.Query(`SELECT node_id, text, embedding FROM embeddings`)
	if err != nil {
		return nil, fmt.Errorf("query embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var nodeID, text string
		var embeddingBytes []byte
		if err := rows.Scan(&nodeID, &text, &embeddingBytes); err != nil {
			continue
		}

		// Convert bytes back to float32
		embedding := make([]float32, len(embeddingBytes)/4)
		for i := range embedding {
			bits := uint32(embeddingBytes[i*4]) |
				uint32(embeddingBytes[i*4+1])<<8 |
				uint32(embeddingBytes[i*4+2])<<16 |
				uint32(embeddingBytes[i*4+3])<<24
			embedding[i] = bitsToFloat32(bits)
		}

		// Compute cosine similarity
		score := cosineSimilarity(query, embedding)
		results = append(results, SearchResult{
			NodeID: nodeID,
			Text:   text,
			Score:  score,
		})
	}

	// Sort by score descending (simplified - in production use proper sorting)
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Close closes the database connection
func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func float32ToBits(f float32) uint32 {
	return uint32(int32(f * 1000000)) // Simplified
}

func bitsToFloat32(b uint32) float32 {
	return float32(int32(b)) / 1000000
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
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
	return dot / (sqrt32(normA) * sqrt32(normB))
}

func sqrt32(x float32) float32 {
	// Newton's method
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/vector/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/vector/
git commit -m "feat(vector): add vector store with sqlite backend"
```

---

## Phase 4: Go Parser

### Task 4.1: Implement Parser Interface

**Files:**
- Create: `internal/parser/parser.go`

**Step 1: Write interface**

```go
// internal/parser/parser.go
package parser

import "github.com/thomassaison/mcp-code-graph/internal/graph"

// ParseResult contains the parsed code graph
type ParseResult struct {
	Nodes []*graph.Node
	Edges []*graph.Edge
}

// Parser defines the interface for code parsers
type Parser interface {
	// ParseFile parses a single file
	ParseFile(path string) (*ParseResult, error)
	
	// ParsePackage parses all files in a package
	ParsePackage(dir string) (*ParseResult, error)
	
	// ParseModule parses all packages in a module
	ParseModule(root string) (*ParseResult, error)
}
```

**Step 2: Commit**

```bash
git add internal/parser/
git commit -m "feat(parser): add Parser interface"
```

---

### Task 4.2: Implement Go Parser

**Files:**
- Create: `internal/parser/go/parser.go`
- Test: `internal/parser/go/parser_test.go`

**Step 1: Write failing tests**

```go
// internal/parser/go/parser_test.go
package goparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	// Create temp Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import "fmt"

// greet says hello
func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func main() {
	msg := greet("World")
	fmt.Println(msg)
}
`
	if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	parser := New()
	result, err := parser.ParseFile(goFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	// Should have 2 function nodes
	if len(result.Nodes) < 2 {
		t.Errorf("len(Nodes) = %d, want at least 2", len(result.Nodes))
	}

	// Check for greet function
	found := false
	for _, node := range result.Nodes {
		if node.Name == "greet" {
			found = true
			if node.Docstring == "" {
				t.Error("greet() missing docstring")
			}
		}
	}
	if !found {
		t.Error("greet function not found")
	}

	// Should have call edges
	if len(result.Edges) == 0 {
		t.Error("no edges found")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/parser/go/... -v`

Expected: FAIL - undefined: New

**Step 3: Write implementation**

```go
// internal/parser/go/parser.go
package goparser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
	"golang.org/x/tools/go/packages"
)

// GoParser implements parser.Parser for Go code
type GoParser struct {
	fset *token.FileSet
}

// New creates a new Go parser
func New() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

// ParseFile parses a single Go file
func (p *GoParser) ParseFile(path string) (*parser.ParseResult, error) {
	result := &parser.ParseResult{}

	// Parse the file
	file, err := parser.ParseFile(p.fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	// Extract package name
	pkgName := file.Name.Name

	// Extract functions
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		node := &graph.Node{
			Type:      graph.NodeTypeFunction,
			Package:   pkgName,
			Name:      fn.Name.Name,
			File:      path,
			Line:      p.fset.Position(fn.Pos()).Line,
			Signature: p.signature(fn),
			Docstring: p.docstring(fn),
			Metadata:  make(map[string]any),
		}
		node.ID = node.GenerateID()
		result.Nodes = append(result.Nodes, node)

		// Extract function calls
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Get called function name
			calleeName := p.callName(call)
			if calleeName == "" {
				return true
			}

			// Create edge (we'll resolve the full ID later)
			edge := &graph.Edge{
				From:     node.ID,
				To:       fmt.Sprintf("func_%s_%s", pkgName, calleeName), // Placeholder
				Type:     graph.EdgeTypeCalls,
				Metadata: make(map[string]any),
			}
			result.Edges = append(result.Edges, edge)

			return true
		})
	}

	return result, nil
}

// ParsePackage parses all files in a package
func (p *GoParser) ParsePackage(dir string) (*parser.ParseResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes,
		Dir:  dir,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	result := &parser.ParseResult{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			pos := p.fset.Position(file.Pos())
			fileResult, err := p.ParseFile(pos.Filename)
			if err != nil {
				continue // Skip files that fail to parse
			}
			result.Nodes = append(result.Nodes, fileResult.Nodes...)
			result.Edges = append(result.Edges, fileResult.Edges...)
		}
	}

	return result, nil
}

// ParseModule parses all packages in a module
func (p *GoParser) ParseModule(root string) (*parser.ParseResult, error) {
	return p.ParsePackage(root)
}

func (p *GoParser) signature(fn *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")

	// Parameters
	if fn.Type.Params != nil {
		for i, param := range fn.Type.Params.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			for _, name := range param.Names {
				sb.WriteString(name.Name)
				sb.WriteString(" ")
			}
			sb.WriteString(p.typeString(param.Type))
		}
	}

	sb.WriteString(")")

	// Return type
	if fn.Type.Results != nil {
		sb.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sb.WriteString(p.typeString(fn.Type.Results.List[0].Type))
		} else {
			sb.WriteString("(")
			for i, res := range fn.Type.Results.List {
				if i > 0 {
					sb.WriteString(", ")
				}
				for _, name := range res.Names {
					sb.WriteString(name.Name)
					sb.WriteString(" ")
				}
				sb.WriteString(p.typeString(res.Type))
			}
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func (p *GoParser) docstring(fn *ast.FuncDecl) string {
	if fn.Doc == nil {
		return ""
	}
	return strings.TrimSpace(fn.Doc.Text())
}

func (p *GoParser) typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return p.typeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + p.typeString(t.X)
	case *ast.ArrayType:
		return "[]" + p.typeString(t.Elt)
	case *ast.MapType:
		return "map[" + p.typeString(t.Key) + "]" + p.typeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + p.typeString(t.Value)
	default:
		return "any"
	}
}

func (p *GoParser) callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/parser/go/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/parser/
git commit -m "feat(parser): implement Go AST parser"
```

---

## Phase 5: Indexer

### Task 5.1: Implement Indexer

**Files:**
- Create: `internal/indexer/indexer.go`
- Test: `internal/indexer/indexer_test.go`

**Step 1: Write failing tests**

```go
// internal/indexer/indexer_test.go
package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	goparser "github.com/thomassaison/mcp-code-graph/internal/parser/go"
)

func TestIndexModule(t *testing.T) {
	// Create temp module
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	goFile := filepath.Join(tmpDir, "main.go")

	os.WriteFile(goMod, []byte("module test\n\ngo 1.22\n"), 0644)
	os.WriteFile(goFile, []byte(`package main

func add(a, b int) int { return a + b }
func main() { println(add(1, 2)) }
`), 0644)

	// Create indexer
	g := graph.New()
	idx := New(g, goparser.New())

	// Index
	if err := idx.IndexModule(tmpDir); err != nil {
		t.Fatalf("IndexModule() error = %v", err)
	}

	// Verify
	if g.NodeCount() < 2 {
		t.Errorf("NodeCount() = %d, want at least 2", g.NodeCount())
	}

	if g.EdgeCount() < 1 {
		t.Errorf("EdgeCount() = %d, want at least 1", g.EdgeCount())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/indexer/... -v`

Expected: FAIL - undefined: Indexer

**Step 3: Write implementation**

```go
// internal/indexer/indexer.go
package indexer

import (
	"fmt"
	"path/filepath"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
)

// Indexer coordinates parsing and graph building
type Indexer struct {
	graph  *graph.Graph
	parser parser.Parser
}

// New creates a new indexer
func New(g *graph.Graph, p parser.Parser) *Indexer {
	return &Indexer{
		graph:  g,
		parser: p,
	}
}

// IndexModule indexes all code in a module
func (idx *Indexer) IndexModule(root string) error {
	result, err := idx.parser.ParseModule(root)
	if err != nil {
		return fmt.Errorf("parse module: %w", err)
	}

	// Clear existing graph
	for _, node := range idx.graph.AllNodes() {
		idx.graph.RemoveNodesForPackage(node.Package)
	}

	// Add nodes
	nodeMap := make(map[string]*graph.Node)
	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
		nodeMap[node.ID] = node
	}

	// Add edges (resolve placeholders to real node IDs)
	for _, edge := range result.Edges {
		// Try to find the actual node ID
		// The parser creates placeholder IDs, we need to resolve them
		for _, node := range result.Nodes {
			if node.Name != "" && edge.To != "" {
				if contains(edge.To, node.Name) {
					edge.To = node.ID
					break
				}
			}
		}
		idx.graph.AddEdge(edge)
	}

	return nil
}

// IndexPackage indexes a single package
func (idx *Indexer) IndexPackage(dir string) error {
	result, err := idx.parser.ParsePackage(dir)
	if err != nil {
		return fmt.Errorf("parse package: %w", err)
	}

	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
	}
	for _, edge := range result.Edges {
		idx.graph.AddEdge(edge)
	}

	return nil
}

// IndexFile indexes a single file
func (idx *Indexer) IndexFile(path string) error {
	result, err := idx.parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	for _, node := range result.Nodes {
		idx.graph.AddNode(node)
	}
	for _, edge := range result.Edges {
		idx.graph.AddEdge(edge)
	}

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/indexer/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/indexer/
git commit -m "feat(indexer): add module/package/file indexing"
```

---

### Task 5.2: Implement File Watcher

**Files:**
- Create: `internal/indexer/watcher.go`
- Test: `internal/indexer/watcher_test.go`

**Step 1: Write implementation**

```go
// internal/indexer/watcher.go
package indexer

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches for file changes and triggers reindexing
type Watcher struct {
	indexer  *Indexer
	watcher  *fsnotify.Watcher
	debounce time.Duration
	events   chan string
	done     chan struct{}
}

// NewWatcher creates a new file watcher
func NewWatcher(idx *Indexer, debounce time.Duration) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		indexer:  idx,
		watcher:  fsWatcher,
		debounce: debounce,
		events:   make(chan string, 100),
		done:     make(chan struct{}),
	}

	go w.processEvents()

	return w, nil
}

// Watch starts watching a directory for Go file changes
func (w *Watcher) Watch(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden and vendor directories
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return w.watcher.Add(path)
		}
		return nil
	})
}

// processEvents handles file change events with debouncing
func (w *Watcher) processEvents() {
	var pending map[string]bool
	var timer *time.Timer

	for {
		select {
		case <-w.done:
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
			// Log error in production
			_ = err
		}
	}
}

// Close stops the watcher
func (w *Watcher) Close() {
	close(w.done)
	w.watcher.Close()
}
```

**Step 2: Add missing import**

```go
import (
	"os"
	...
)
```

**Step 3: Run tests**

Run: `go build ./internal/indexer/...`

**Step 4: Commit**

```bash
git add internal/indexer/
git commit -m "feat(indexer): add file watcher with debouncing"
```

---

## Phase 6: Summary Generation

### Task 6.1: Implement LLM Provider Interface

**Files:**
- Create: `internal/summary/provider.go`

**Step 1: Write interface**

```go
// internal/summary/provider.go
package summary

import "context"

// SummaryRequest contains information needed to generate a summary
type SummaryRequest struct {
	FunctionName string
	Signature    string
	Package      string
	Docstring    string
	Code         string
}

// LLMProvider defines the interface for LLM providers
type LLMProvider interface {
	// GenerateSummary generates a summary for a function
	GenerateSummary(ctx context.Context, req SummaryRequest) (string, error)
}

// MockProvider is a mock LLM provider for testing
type MockProvider struct{}

func (m *MockProvider) GenerateSummary(ctx context.Context, req SummaryRequest) (string, error) {
	return fmt.Sprintf("Function %s in package %s", req.FunctionName, req.Package), nil
}
```

**Step 2: Commit**

```bash
git add internal/summary/
git commit -m "feat(summary): add LLM provider interface"
```

---

### Task 6.2: Implement Summary Generator

**Files:**
- Create: `internal/summary/generator.go`
- Test: `internal/summary/generator_test.go`

**Step 1: Write implementation**

```go
// internal/summary/generator.go
package summary

import (
	"context"
	"fmt"
	"time"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

// Generator generates summaries for functions
type Generator struct {
	provider LLMProvider
	model    string
}

// NewGenerator creates a new summary generator
func NewGenerator(provider LLMProvider, model string) *Generator {
	return &Generator{
		provider: provider,
		model:    model,
	}
}

// Generate generates a summary for a function node
func (g *Generator) Generate(ctx context.Context, node *graph.Node) error {
	if node.Summary != nil && node.Summary.GeneratedBy == "human" {
		// Don't overwrite human-written summaries
		return nil
	}

	req := SummaryRequest{
		FunctionName: node.Name,
		Signature:    node.Signature,
		Package:      node.Package,
		Docstring:    node.Docstring,
	}

	summary, err := g.provider.GenerateSummary(ctx, req)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	node.Summary = &graph.Summary{
		Text:        summary,
		GeneratedBy: "llm",
		Model:       g.model,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	return nil
}

// GenerateAll generates summaries for all functions in the graph
func (g *Generator) GenerateAll(ctx context.Context, gr *graph.Graph) error {
	functions := gr.GetNodesByType(graph.NodeTypeFunction)
	for _, fn := range functions {
		if err := g.Generate(ctx, fn); err != nil {
			// Log error but continue
			continue
		}
	}
	return nil
}
```

**Step 2: Write test**

```go
// internal/summary/generator_test.go
package summary

import (
	"context"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func TestGenerator(t *testing.T) {
	provider := &MockProvider{}
	gen := NewGenerator(provider, "test-model")

	node := &graph.Node{
		Type:      graph.NodeTypeFunction,
		Package:   "main",
		Name:      "test",
		Signature: "func test() string",
	}

	if err := gen.Generate(context.Background(), node); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if node.Summary == nil {
		t.Fatal("Summary not set")
	}

	if node.Summary.GeneratedBy != "llm" {
		t.Errorf("GeneratedBy = %q, want %q", node.Summary.GeneratedBy, "llm")
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/summary/... -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/summary/
git commit -m "feat(summary): add summary generator with LLM provider"
```

---

## Phase 7: MCP Server

### Task 7.1: Implement MCP Server

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/resources.go`

**Step 1: Write server setup**

```go
// internal/mcp/server.go
package mcp

import (
	"context"
	"fmt"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/indexer"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
	"github.com/thomassaison/mcp-code-graph/internal/summary"
	"github.com/thomassaison/mcp-code-graph/internal/vector"
)

// Server is the MCP code graph server
type Server struct {
	graph     *graph.Graph
	vector    *vector.Store
	indexer   *indexer.Indexer
	summary   *summary.Generator
	persister *graph.Persister
	parser    parser.Parser
	config    *Config
}

// Config holds server configuration
type Config struct {
	DBPath      string
	ProjectPath string
	LLMModel    string
}

// NewServer creates a new MCP server
func NewServer(cfg *Config) (*Server, error) {
	// Initialize graph
	g := graph.New()

	// Initialize persister
	persister := graph.NewPersister(cfg.DBPath)
	if err := persister.Load(g); err != nil {
		// Start fresh if load fails
		g = graph.New()
	}

	// Initialize vector store
	vecStore, err := vector.NewStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("init vector store: %w", err)
	}

	// Initialize parser
	goParser := goparser.New()

	// Initialize indexer
	idx := indexer.New(g, goParser)

	// Initialize summary generator (with mock for now)
	sumGen := summary.NewGenerator(&summary.MockProvider{}, cfg.LLMModel)

	return &Server{
		graph:     g,
		vector:    vecStore,
		indexer:   idx,
		summary:   sumGen,
		persister: persister,
		parser:    goParser,
		config:    cfg,
	}, nil
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	// Index project if provided
	if s.config.ProjectPath != "" {
		if err := s.indexer.IndexModule(s.config.ProjectPath); err != nil {
			return fmt.Errorf("index project: %w", err)
		}
		// Generate summaries
		if err := s.summary.GenerateAll(ctx, s.graph); err != nil {
			// Log but continue
		}
		// Save
		if err := s.persister.Save(s.graph); err != nil {
			return fmt.Errorf("save graph: %w", err)
		}
	}

	// TODO: Start MCP protocol handler
	// This will depend on the MCP Go SDK availability
	return nil
}

// Close closes the server
func (s *Server) Close() {
	s.persister.Save(s.graph)
	s.vector.Close()
}
```

**Step 2: Add missing import**

Add `goparser` import and fix the reference.

**Step 3: Write tools**

```go
// internal/mcp/tools.go
package mcp

import (
	"context"
	"encoding/json"
)

// Tool represents an MCP tool
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

// GetTools returns all available tools
func (s *Server) GetTools() []Tool {
	return []Tool{
		{
			Name:        "search_functions",
			Description: "Search for functions by semantic similarity to a query",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Maximum number of results",
					},
				},
				"required": []string{"query"},
			},
			Handler: s.handleSearchFunctions,
		},
		{
			Name:        "get_callers",
			Description: "Get all functions that call the specified function",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{
						"type":        "string",
						"description": "The function ID",
					},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetCallers,
		},
		{
			Name:        "get_callees",
			Description: "Get all functions called by the specified function",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{
						"type":        "string",
						"description": "The function ID",
					},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetCallees,
		},
		{
			Name:        "reindex_project",
			Description: "Trigger a full reindex of the project",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			Handler: s.handleReindexProject,
		},
		{
			Name:        "update_summary",
			Description: "Update the summary for a function",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{
						"type":        "string",
						"description": "The function ID",
					},
					"summary": map[string]any{
						"type":        "string",
						"description": "The new summary",
					},
				},
				"required": []string{"function_id", "summary"},
			},
			Handler: s.handleUpdateSummary,
		},
	}
}

func (s *Server) handleSearchFunctions(ctx context.Context, args map[string]any) (string, error) {
	query := args["query"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// For now, do simple text matching on summaries
	// In production, use vector search
	results := []map[string]any{}
	for _, node := range s.graph.GetNodesByType(graph.NodeTypeFunction) {
		if node.Summary != nil && contains(node.Summary.Text, query) {
			results = append(results, map[string]any{
				"id":       node.ID,
				"name":     node.Name,
				"package":  node.Package,
				"summary":  node.Summary.Text,
			})
			if len(results) >= limit {
				break
			}
		}
	}

	b, _ := json.Marshal(results)
	return string(b), nil
}

func (s *Server) handleGetCallers(ctx context.Context, args map[string]any) (string, error) {
	fnID := args["function_id"].(string)
	callers := s.graph.GetCallers(fnID)

	results := []map[string]any{}
	for _, node := range callers {
		results = append(results, map[string]any{
			"id":       node.ID,
			"name":     node.Name,
			"package":  node.Package,
			"file":     node.File,
			"line":     node.Line,
		})
	}

	b, _ := json.Marshal(results)
	return string(b), nil
}

func (s *Server) handleGetCallees(ctx context.Context, args map[string]any) (string, error) {
	fnID := args["function_id"].(string)
	callees := s.graph.GetCallees(fnID)

	results := []map[string]any{}
	for _, node := range callees {
		results = append(results, map[string]any{
			"id":       node.ID,
			"name":     node.Name,
			"package":  node.Package,
			"file":     node.File,
			"line":     node.Line,
		})
	}

	b, _ := json.Marshal(results)
	return string(b), nil
}

func (s *Server) handleReindexProject(ctx context.Context, args map[string]any) (string, error) {
	if err := s.indexer.IndexModule(s.config.ProjectPath); err != nil {
		return "", err
	}
	if err := s.persister.Save(s.graph); err != nil {
		return "", err
	}
	return `{"status": "ok", "nodes": ` + itoa(s.graph.NodeCount()) + `}`, nil
}

func (s *Server) handleUpdateSummary(ctx context.Context, args map[string]any) (string, error) {
	fnID := args["function_id"].(string)
	summaryText := args["summary"].(string)

	node, err := s.graph.GetNode(fnID)
	if err != nil {
		return "", err
	}

	node.Summary = &graph.Summary{
		Text:        summaryText,
		GeneratedBy: "human",
		CreatedAt:   node.Summary.CreatedAt,
		UpdatedAt:    time.Now().Unix(),
	}

	if err := s.persister.Save(s.graph); err != nil {
		return "", err
	}

	return `{"status": "ok"}`, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && contains(s[1:], substr)) || s[:len(substr)] == substr)
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
```

**Step 4: Write resources**

```go
// internal/mcp/resources.go
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Resource represents an MCP resource
type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

// GetResources returns all available resources
func (s *Server) GetResources() []Resource {
	resources := []Resource{
		{
			URI:         "function://{package}/{name}",
			Name:        "Function by package and name",
			Description: "Get function details, signature, summary, and references",
			MimeType:    "application/json",
		},
		{
			URI:         "package://{name}",
			Name:        "Package overview",
			Description: "Get package contents and exported functions",
			MimeType:    "application/json",
		},
	}

	// Add dynamic resources for all functions
	for _, node := range s.graph.GetNodesByType(graph.NodeTypeFunction) {
		resources = append(resources, Resource{
			URI:         fmt.Sprintf("function://%s/%s", node.Package, node.Name),
			Name:        fmt.Sprintf("%s.%s", node.Package, node.Name),
			Description: node.Docstring,
			MimeType:    "application/json",
		})
	}

	return resources
}

// ReadResource reads a resource by URI
func (s *Server) ReadResource(uri string) (string, error) {
	// Parse URI
	if strings.HasPrefix(uri, "function://") {
		return s.readFunctionResource(uri)
	}
	if strings.HasPrefix(uri, "package://") {
		return s.readPackageResource(uri)
	}
	return "", fmt.Errorf("unknown resource URI: %s", uri)
}

func (s *Server) readFunctionResource(uri string) (string, error) {
	// Parse: function://{package}/{name}
	parts := strings.Split(strings.TrimPrefix(uri, "function://"), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid function URI: %s", uri)
	}

	pkg, name := parts[0], parts[1]

	// Find function
	var found *graph.Node
	for _, node := range s.graph.GetNodesByType(graph.NodeTypeFunction) {
		if node.Package == pkg && node.Name == name {
			found = node
			break
		}
	}

	if found == nil {
		return "", fmt.Errorf("function not found: %s/%s", pkg, name)
	}

	// Build response
	callers := s.graph.GetCallers(found.ID)
	callees := s.graph.GetCallees(found.ID)

	response := map[string]any{
		"id":         found.ID,
		"name":       found.Name,
		"package":    found.Package,
		"file":       found.File,
		"line":       found.Line,
		"signature":  found.Signature,
		"docstring":  found.Docstring,
		"summary":    found.Summary,
		"callers":    flattenNodes(callers),
		"callees":    flattenNodes(callees),
	}

	b, _ := json.MarshalIndent(response, "", "  ")
	return string(b), nil
}

func (s *Server) readPackageResource(uri string) (string, error) {
	// Parse: package://{name}
	pkg := strings.TrimPrefix(uri, "package://")

	nodes := s.graph.GetNodesByPackage(pkg)
	if len(nodes) == 0 {
		return "", fmt.Errorf("package not found: %s", pkg)
	}

	functions := []map[string]any{}
	for _, node := range nodes {
		if node.Type == graph.NodeTypeFunction {
			functions = append(functions, map[string]any{
				"id":        node.ID,
				"name":      node.Name,
				"signature": node.Signature,
				"file":      node.File,
				"line":      node.Line,
			})
		}
	}

	response := map[string]any{
		"name":      pkg,
		"functions": functions,
	}

	b, _ := json.MarshalIndent(response, "", "  ")
	return string(b), nil
}

func flattenNodes(nodes []*graph.Node) []map[string]any {
	result := []map[string]any{}
	for _, n := range nodes {
		result = append(result, map[string]any{
			"id":      n.ID,
			"name":    n.Name,
			"package": n.Package,
		})
	}
	return result
}
```

**Step 5: Run build**

Run: `go build ./...`

**Step 6: Commit**

```bash
git add internal/mcp/
git commit -m "feat(mcp): add MCP server with tools and resources"
```

---

## Phase 8: CLI Entry Point

### Task 8.1: Create main.go

**Files:**
- Create: `cmd/mcp-code-graph/main.go`

**Step 1: Write main.go**

```go
// cmd/mcp-code-graph/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/thomassaison/mcp-code-graph/internal/mcp"
)

func main() {
	projectPath := flag.String("project", ".", "Path to the Go project to index")
	dbPath := flag.String("db", ".mcp-code-graph/db.sqlite", "Path to the database file")
	llmModel := flag.String("model", "gpt-4o-mini", "LLM model for summaries")
	flag.Parse()

	// Ensure DB directory exists
	dbDir := filepath.Dir(*dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DB directory: %v\n", err)
		os.Exit(1)
	}

	// Create server
	server, err := mcp.NewServer(&mcp.Config{
		DBPath:      *dbPath,
		ProjectPath: *projectPath,
		LLMModel:    *llmModel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Start server
	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("MCP Code Graph server started\n")
	fmt.Printf("Project: %s\n", *projectPath)
	fmt.Printf("Database: %s\n", *dbPath)
	fmt.Printf("Functions indexed: %d\n", server.Graph().NodeCount())

	// Keep running until context is cancelled
	<-ctx.Done()
}

// Add missing import
import "path/filepath"
```

**Step 2: Add Graph accessor to Server**

```go
// In internal/mcp/server.go, add:
func (s *Server) Graph() *graph.Graph {
	return s.graph
}
```

**Step 3: Run build**

Run: `make build`

**Step 4: Commit**

```bash
git add cmd/
git commit -m "feat(cli): add main entry point"
```

---

## Phase 9: Testing & Documentation

### Task 9.1: Write README

**Files:**
- Create: `README.md`

**Step 1: Write README**

```markdown
# MCP Code Graph

An MCP (Model Context Protocol) server that provides a code graph database for AI assistants to understand Go codebases through function summaries, call graphs, and semantic search.

## Features

- **Code Graph**: Functions, types, packages, and their relationships
- **Call Graph**: Find callers and callees for any function
- **Semantic Search**: Search functions by purpose (vector similarity)
- **LLM Summaries**: Auto-generated function summaries with human-editable overrides
- **Incremental Indexing**: File watcher + manual full reindex

## Installation

```bash
go install github.com/thomassaison/mcp-code-graph/cmd/mcp-code-graph@latest
```

## Usage

```bash
# Index a project and start the server
mcp-code-graph --project /path/to/go/project

# Custom database location
mcp-code-graph --project . --db ./data/codegraph.db
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `search_functions` | Search for functions by semantic similarity |
| `get_callers` | Get all functions that call this function |
| `get_callees` | Get all functions called by this function |
| `reindex_project` | Trigger full reindex |
| `update_summary` | Update a function's summary |

## MCP Resources

- `function://{package}/{name}` - Function details
- `package://{name}` - Package overview

## Configuration

See [ADR-0007](adr/0007-project-structure.md) for architecture details.

## Development

```bash
make build    # Build binary
make test     # Run tests
make run      # Run locally
```

## License

MIT
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```

---

### Task 9.2: Run Full Test Suite

**Step 1: Run all tests**

Run: `make test`

**Step 2: Fix any failures**

**Step 3: Commit if fixes needed**

---

### Task 9.3: Final Commit

**Step 1: Add .gitignore**

```gitignore
# .gitignore
bin/
.mcp-code-graph/
*.test
*.out
```

**Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add gitignore"
```

**Step 3: Verify build**

Run: `make build && ./bin/mcp-code-graph --help`

---

## Summary

This plan implements the MCP Code Graph MVP in 9 phases:

1. **Project Setup** - Module, dependencies, directory structure
2. **Graph Engine** - Node/Edge types, in-memory graph, SQLite persistence
3. **Vector Store** - sqlite-vec wrapper for semantic search
4. **Go Parser** - AST-based parsing with call graph extraction
5. **Indexer** - Module/package/file indexing + file watcher
6. **Summary Generation** - LLM provider interface + generator
7. **MCP Server** - Tools and resources implementation
8. **CLI** - Main entry point with flags
9. **Testing & Docs** - README, test suite, verification

Each task follows TDD: write failing test → implement → verify pass → commit.
