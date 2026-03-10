# Behavioral Function Search Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add behavioral search capability to find functions by what they DO (logging, error handling, database access, etc.)

**Architecture:** LLM-based behavior extraction during indexing, stored as tags in Node.Metadata, combined with semantic search via new `search_by_behavior` MCP tool.

**Tech Stack:** Go, existing LLM provider infrastructure, existing vector store

---

## Task 1: Create behavior package with constants and interface

**Files:**
- Create: `internal/behavior/behavior.go`
- Create: `internal/behavior/behavior_test.go`

**Step 1: Write the failing test**

```go
package behavior

import "testing"

func TestBehaviorConstants(t *testing.T) {
	tests := []struct {
		name     string
		behavior string
	}{
		{"logging", BehaviorLogging},
		{"error-handle", BehaviorErrorHandle},
		{"database", BehaviorDatabase},
		{"http-client", BehaviorHTTPClient},
		{"file-io", BehaviorFileIO},
		{"concurrency", BehaviorConcurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.behavior == "" {
				t.Errorf("Behavior constant %s is empty", tt.name)
			}
		})
	}
}

func TestAllBehaviors(t *testing.T) {
	expected := []string{
		BehaviorLogging,
		BehaviorErrorHandle,
		BehaviorDatabase,
		BehaviorHTTPClient,
		BehaviorFileIO,
		BehaviorConcurrency,
	}

	all := AllBehaviors()
	if len(all) != len(expected) {
		t.Errorf("AllBehaviors() returned %d behaviors, want %d", len(all), len(expected))
	}

	for _, b := range expected {
		found := false
		for _, a := range all {
			if a == b {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllBehaviors() missing behavior %s", b)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -v`
Expected: FAIL with "package behavior is not in GOROOT"

**Step 3: Create the package directory and implementation**

```bash
mkdir -p internal/behavior
```

```go
package behavior

const (
	BehaviorLogging     = "logging"
	BehaviorErrorHandle = "error-handle"
	BehaviorDatabase    = "database"
	BehaviorHTTPClient  = "http-client"
	BehaviorFileIO      = "file-io"
	BehaviorConcurrency = "concurrency"
)

func AllBehaviors() []string {
	return []string{
		BehaviorLogging,
		BehaviorErrorHandle,
		BehaviorDatabase,
		BehaviorHTTPClient,
		BehaviorFileIO,
		BehaviorConcurrency,
	}
}

func IsValidBehavior(b string) bool {
	for _, behavior := range AllBehaviors() {
		if behavior == b {
			return true
		}
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/behavior/
git commit -m "feat: add behavior package with core behavior constants"
```

---

## Task 2: Create behavior analyzer interface and mock

**Files:**
- Create: `internal/behavior/analyzer.go`
- Create: `internal/behavior/analyzer_test.go`

**Step 1: Write the failing test**

```go
package behavior

import (
	"context"
	"testing"
)

func TestMockAnalyzer(t *testing.T) {
	analyzer := NewMockAnalyzer()

	req := AnalysisRequest{
		PackageName:  "testpkg",
		FunctionName: "TestFunc",
		Signature:    "func TestFunc() error",
		Code:         "func TestFunc() error { log.Println(\"test\"); return nil }",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(behaviors) == 0 {
		t.Error("Analyze() returned no behaviors")
	}
}

func TestMockAnalyzerWithPresetBehaviors(t *testing.T) {
	preset := []string{BehaviorLogging, BehaviorErrorHandle}
	analyzer := NewMockAnalyzer().WithBehaviors(preset)

	req := AnalysisRequest{
		PackageName:  "testpkg",
		FunctionName: "TestFunc",
		Signature:    "func TestFunc() error",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(behaviors) != len(preset) {
		t.Errorf("Analyze() returned %d behaviors, want %d", len(behaviors), len(preset))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -v`
Expected: FAIL with "undefined: AnalysisRequest"

**Step 3: Write the implementation**

```go
package behavior

import (
	"context"
)

type AnalysisRequest struct {
	PackageName  string
	FunctionName string
	Signature    string
	Docstring    string
	Code         string
}

type Analyzer interface {
	Analyze(ctx context.Context, req AnalysisRequest) ([]string, error)
}

type MockAnalyzer struct {
	behaviors []string
}

func NewMockAnalyzer() *MockAnalyzer {
	return &MockAnalyzer{
		behaviors: []string{BehaviorLogging},
	}
}

func (m *MockAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) ([]string, error) {
	if len(m.behaviors) > 0 {
		return m.behaviors, nil
	}
	return []string{BehaviorLogging}, nil
}

func (m *MockAnalyzer) WithBehaviors(behaviors []string) *MockAnalyzer {
	m.behaviors = behaviors
	return m
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/behavior/analyzer.go internal/behavior/analyzer_test.go
git commit -m "feat: add behavior analyzer interface and mock implementation"
```

---

## Task 3: Create LLM-based behavior analyzer

**Files:**
- Create: `internal/behavior/llm_analyzer.go`
- Create: `internal/behavior/llm_analyzer_test.go`

**Step 1: Write the failing test**

```go
package behavior

import (
	"context"
	"testing"
)

func TestLLMAnalyzer_Analyze(t *testing.T) {
	provider := &mockLLMProvider{
		response: `{"behaviors": ["logging", "error-handle"]}`,
	}

	analyzer := NewLLMAnalyzer(provider)
	req := AnalysisRequest{
		PackageName:  "service",
		FunctionName: "HandleRequest",
		Signature:    "func HandleRequest(req *Request) error",
		Code:         "func HandleRequest(req *Request) error { log.Printf(\"handling %v\", req); return errors.New(\"failed\") }",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(behaviors) != 2 {
		t.Errorf("Analyze() returned %d behaviors, want 2", len(behaviors))
	}

	found := make(map[string]bool)
	for _, b := range behaviors {
		found[b] = true
	}

	if !found[BehaviorLogging] {
		t.Error("Analyze() missing logging behavior")
	}
	if !found[BehaviorErrorHandle] {
		t.Error("Analyze() missing error-handle behavior")
	}
}

type mockLLMProvider struct {
	response string
	err      error
}

func (m *mockLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -v`
Expected: FAIL with "undefined: NewLLMAnalyzer"

**Step 3: Write the implementation**

```go
package behavior

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type LLMProvider interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type LLMAnalyzer struct {
	provider LLMProvider
}

func NewLLMAnalyzer(provider LLMProvider) *LLMAnalyzer {
	return &LLMAnalyzer{provider: provider}
}

func (a *LLMAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) ([]string, error) {
	prompt := a.buildPrompt(req)

	response, err := a.provider.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generate: %w", err)
	}

	return a.parseResponse(response)
}

func (a *LLMAnalyzer) buildPrompt(req AnalysisRequest) string {
	var sb strings.Builder

	sb.WriteString("Analyze this Go function and identify which behaviors it exhibits.\n\n")
	sb.WriteString("Available behaviors:\n")
	for _, b := range AllBehaviors() {
		sb.WriteString(fmt.Sprintf("- %s\n", b))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Package: %s\n", req.PackageName))
	sb.WriteString(fmt.Sprintf("Function: %s\n", req.FunctionName))

	if req.Signature != "" {
		sb.WriteString(fmt.Sprintf("Signature: %s\n", req.Signature))
	}
	if req.Docstring != "" {
		sb.WriteString(fmt.Sprintf("Documentation: %s\n", req.Docstring))
	}
	if req.Code != "" {
		sb.WriteString(fmt.Sprintf("Code:\n%s\n", req.Code))
	}

	sb.WriteString("\nRespond with ONLY a JSON object: {\"behaviors\": [\"behavior1\", \"behavior2\"]}\n")
	sb.WriteString("Include only behaviors that are clearly present. If none match, return empty array.\n")

	return sb.String()
}

func (a *LLMAnalyzer) parseResponse(response string) ([]string, error) {
	response = strings.TrimSpace(response)

	var result struct {
		Behaviors []string `json:"behaviors"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var valid []string
	for _, b := range result.Behaviors {
		if IsValidBehavior(b) {
			valid = append(valid, b)
		}
	}

	return valid, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/behavior/llm_analyzer.go internal/behavior/llm_analyzer_test.go
git commit -m "feat: add LLM-based behavior analyzer"
```

---

## Task 4: Integrate behavior analyzer with existing LLM provider

**Files:**
- Modify: `internal/llm/provider.go`
- Create: `internal/behavior/llm_adapter.go`

**Step 1: Write the failing test**

Add to `internal/behavior/llm_analyzer_test.go`:

```go
func TestLLMAnalyzer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := llm.NewMockProvider()
	analyzer := NewLLMAnalyzer(NewLLMProviderAdapter(provider))

	req := AnalysisRequest{
		PackageName:  "test",
		FunctionName: "TestFunc",
		Code:         "func TestFunc() { log.Println(\"test\") }",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	t.Logf("Behaviors: %v", behaviors)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -v`
Expected: FAIL with "undefined: NewLLMProviderAdapter"

**Step 3: Write the adapter**

```go
package behavior

import (
	"context"

	"github.com/thomassaison/mcp-code-graph/internal/llm"
)

type LLMProviderAdapter struct {
	provider llm.Provider
}

func NewLLMProviderAdapter(provider llm.Provider) *LLMProviderAdapter {
	return &LLMProviderAdapter{provider: provider}
}

func (a *LLMProviderAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.provider.Generate(ctx, prompt)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/behavior/llm_adapter.go internal/behavior/llm_analyzer_test.go
git commit -m "feat: add LLM provider adapter for behavior analyzer"
```

---

## Task 5: Add GetNodesByBehaviors method to Graph

**Files:**
- Modify: `internal/graph/graph.go`
- Modify: `internal/graph/graph_test.go`

**Step 1: Write the failing test**

Add to `internal/graph/graph_test.go`:

```go
func TestGraph_GetNodesByBehaviors(t *testing.T) {
	g := NewGraph()

	node1 := &Node{
		ID:      "func_test_Log_test.go:10",
		Type:    NodeTypeFunction,
		Package: "test",
		Name:    "Log",
		Metadata: map[string]any{
			"behaviors": []string{"logging", "error-handle"},
		},
	}

	node2 := &Node{
		ID:      "func_test_Handle_test.go:20",
		Type:    NodeTypeFunction,
		Package: "test",
		Name:    "Handle",
		Metadata: map[string]any{
			"behaviors": []string{"http-client", "error-handle"},
		},
	}

	node3 := &Node{
		ID:      "func_test_Process_test.go:30",
		Type:    NodeTypeFunction,
		Package: "test",
		Name:    "Process",
		Metadata: map[string]any{
			"behaviors": []string{"database"},
		},
	}

	g.AddNode(node1)
	g.AddNode(node2)
	g.AddNode(node3)

	tests := []struct {
		name      string
		behaviors []string
		wantCount int
	}{
		{
			name:      "single behavior",
			behaviors: []string{"logging"},
			wantCount: 1,
		},
		{
			name:      "multiple behaviors (AND)",
			behaviors: []string{"logging", "error-handle"},
			wantCount: 1,
		},
		{
			name:      "behavior not found",
			behaviors: []string{"file-io"},
			wantCount: 0,
		},
		{
			name:      "empty behaviors",
			behaviors: []string{},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := g.GetNodesByBehaviors(tt.behaviors)
			if len(nodes) != tt.wantCount {
				t.Errorf("GetNodesByBehaviors() returned %d nodes, want %d", len(nodes), tt.wantCount)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/... -v -run TestGraph_GetNodesByBehaviors`
Expected: FAIL with "g.GetNodesByBehaviors undefined"

**Step 3: Write the implementation**

Add to `internal/graph/graph.go`:

```go
func (g *Graph) GetNodesByBehaviors(behaviors []string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(behaviors) == 0 {
		result := make([]*Node, 0, len(g.nodes))
		for _, node := range g.nodes {
			if node.Type == NodeTypeFunction || node.Type == NodeTypeMethod {
				result = append(result, node)
			}
		}
		return result
	}

	var result []*Node
	for _, node := range g.nodes {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}

		nodeBehaviors := getBehaviorsFromMetadata(node)
		if hasAllBehaviors(nodeBehaviors, behaviors) {
			result = append(result, node)
		}
	}

	return result
}

func getBehaviorsFromMetadata(node *Node) []string {
	if node.Metadata == nil {
		return nil
	}

	behaviorsRaw, ok := node.Metadata["behaviors"]
	if !ok {
		return nil
	}

	behaviors, ok := behaviorsRaw.([]string)
	if !ok {
		return nil
	}

	return behaviors
}

func hasAllBehaviors(nodeBehaviors, requiredBehaviors []string) bool {
	for _, required := range requiredBehaviors {
		found := false
		for _, b := range nodeBehaviors {
			if b == required {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/... -v -run TestGraph_GetNodesByBehaviors`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/graph/graph.go internal/graph/graph_test.go
git commit -m "feat: add GetNodesByBehaviors method to Graph"
```

---

## Task 6: Integrate behavior analyzer into indexer

**Files:**
- Modify: `internal/indexer/indexer.go`

**Step 1: Write the failing test**

Add to `internal/indexer/indexer_test.go`:

```go
func TestIndexer_WithBehaviorAnalysis(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "service.go")
	err := os.WriteFile(testFile, []byte(`
package service

import "log"

func LogError(msg string) {
	log.Printf("error: %s", msg)
}
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := behavior.NewMockAnalyzer().WithBehaviors([]string{behavior.BehaviorLogging})
	ix := NewWithBehaviorAnalyzer(dir, analyzer)

	if err := ix.IndexModule(); err != nil {
		t.Fatalf("IndexModule() error = %v", err)
	}

	nodes := ix.Graph().AllNodes()
	var logFunc *graph.Node
	for _, n := range nodes {
		if n.Name == "LogError" {
			logFunc = n
			break
		}
	}

	if logFunc == nil {
		t.Fatal("LogError function not found in graph")
	}

	behaviors := logFunc.Metadata["behaviors"]
	if behaviors == nil {
		t.Fatal("behaviors not set in metadata")
	}

	behaviorList, ok := behaviors.([]string)
	if !ok {
		t.Fatal("behaviors is not []string")
	}

	found := false
	for _, b := range behaviorList {
		if b == behavior.BehaviorLogging {
			found = true
			break
		}
	}

	if !found {
		t.Error("logging behavior not found in function metadata")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/indexer/... -v -run TestIndexer_WithBehaviorAnalysis`
Expected: FAIL with "ix.NewWithBehaviorAnalyzer undefined"

**Step 3: Write the implementation**

Add to `internal/indexer/indexer.go`:

```go
type Indexer struct {
	projectDir       string
	graph            *graph.Graph
	persist          *graph.Persistor
	embedding        embedding.Provider
	summarizer       *summary.Generator
	behaviorAnalyzer behavior.Analyzer
}

func NewWithBehaviorAnalyzer(projectDir string, analyzer behavior.Analyzer) *Indexer {
	return &Indexer{
		projectDir:       projectDir,
		graph:            graph.NewGraph(),
		behaviorAnalyzer: analyzer,
	}
}

func (ix *Indexer) analyzeBehaviors(ctx context.Context, nodes []*graph.Node) {
	if ix.behaviorAnalyzer == nil {
		return
	}

	for _, node := range nodes {
		if node.Type != graph.NodeTypeFunction && node.Type != graph.NodeTypeMethod {
			continue
		}

		req := behavior.AnalysisRequest{
			PackageName:  node.Package,
			FunctionName: node.Name,
			Signature:    node.Signature,
			Docstring:    node.Docstring,
			Code:         node.Metadata["code"].(string),
		}

		behaviors, err := ix.behaviorAnalyzer.Analyze(ctx, req)
		if err != nil {
			slog.Debug("behavior analysis failed", "function", node.Name, "error", err)
			continue
		}

		if node.Metadata == nil {
			node.Metadata = make(map[string]any)
		}
		node.Metadata["behaviors"] = behaviors
	}
}
```

Then call `analyzeBehaviors` after parsing in `IndexModule`:

```go
func (ix *Indexer) IndexModule() error {
	// ... existing parsing code ...

	// Analyze behaviors
	ix.analyzeBehaviors(context.Background(), nodes)

	// ... rest of existing code ...
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/indexer/... -v -run TestIndexer_WithBehaviorAnalysis`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/indexer/indexer.go internal/indexer/indexer_test.go
git commit -m "feat: integrate behavior analyzer into indexer"
```

---

## Task 7: Create search_by_behavior MCP tool

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

Add to `internal/mcp/server_test.go`:

```go
func TestServer_SearchByBehaviorTool(t *testing.T) {
	s := NewTestServer(t)

	node := &graph.Node{
		ID:        "func_service_LogError_test.go:10",
		Type:      graph.NodeTypeFunction,
		Package:   "service",
		Name:      "LogError",
		Signature: "func LogError(msg string)",
		Summary:   &graph.Summary{Text: "[Logging] Logs error messages to stdout"},
		Metadata: map[string]any{
			"behaviors": []string{"logging", "error-handle"},
		},
	}
	s.graph.AddNode(node)

	result, err := s.handleSearchByBehavior(context.Background(), map[string]any{
		"query":     "log errors",
		"behaviors": []any{"logging", "error-handle"},
		"limit":     float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearchByBehavior() error = %v", err)
	}

	t.Logf("Result: %s", result)

	if !strings.Contains(result, "LogError") {
		t.Error("Result should contain LogError function")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/... -v -run TestServer_SearchByBehaviorTool`
Expected: FAIL with "s.handleSearchByBehavior undefined"

**Step 3: Write the implementation**

Add to `internal/mcp/tools.go` in `GetTools()`:

```go
{
	Name:        "search_by_behavior",
	Description: "Search for functions by behavior (logging, error handling, database access, etc.) combined with semantic search",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The semantic search query describing the function purpose",
			},
			"behaviors": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Behavior tags to filter by (AND logic): logging, error-handle, database, http-client, file-io, concurrency",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Maximum number of results to return",
				"default":     10,
			},
		},
		"required": []string{"query"},
	},
	Handler: s.handleSearchByBehavior,
},
```

Add the handler:

```go
func (s *Server) handleSearchByBehavior(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)

	var behaviors []string
	if behaviorsRaw, ok := args["behaviors"].([]any); ok {
		for _, b := range behaviorsRaw {
			if bs, ok := b.(string); ok {
				behaviors = append(behaviors, bs)
			}
		}
	}

	limit := 10
	if limitRaw, ok := args["limit"].(float64); ok {
		limit = int(limitRaw)
	}

	nodes := s.graph.GetNodesByBehaviors(behaviors)

	if s.vector == nil {
		return s.formatBehaviorResults(nodes, limit), nil
	}

	queryEmbedding, err := s.embedding.Embed(ctx, query)
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

		nodeEmbedding, err := s.vector.GetEmbedding(node.ID)
		if err != nil {
			continue
		}

		score := cosineSimilarity(queryEmbedding, nodeEmbedding)
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

func (s *Server) formatBehaviorResults(nodes []*graph.Node, limit int) string {
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}

	var results []map[string]any
	for _, node := range nodes {
		results = append(results, map[string]any{
			"id":        node.ID,
			"name":      node.Name,
			"package":   node.Package,
			"signature": node.Signature,
			"behaviors": node.Metadata["behaviors"],
			"summary":   node.SummaryText(),
		})
	}

	resultJSON, _ := json.MarshalIndent(results, "", "  ")
	return string(resultJSON)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/... -v -run TestServer_SearchByBehaviorTool`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/server_test.go
git commit -m "feat: add search_by_behavior MCP tool"
```

---

## Task 8: Update README and create ADR

**Files:**
- Modify: `README.md`
- Create: `adr/0015-behavioral-search.md`
- Modify: `adr/README.md`

**Step 1: Update README MCP Tools table**

Add row to the table:

```markdown
| `search_by_behavior` | Search for functions by behavior tags combined with semantic search |
```

**Step 2: Create ADR-0015**

```markdown
# 15. Behavioral Function Search

Date: 2026-03-10

## Status

Accepted

## Context

Users want to find functions by what they DO, not just by name. Queries like "find functions that log errors" or "HTTP handlers that access the database" require understanding function behavior.

Existing semantic search on summaries helps but is imprecise for behavioral filtering.

## Decision

Implement hybrid behavioral search:

1. **Behavior Extraction**: LLM analyzes functions during indexing to identify core behaviors:
   - `logging`: logs messages, errors, debug info
   - `error-handle`: creates, wraps, or handles errors
   - `database`: reads/writes to database
   - `http-client`: makes HTTP requests
   - `file-io`: reads/writes files
   - `concurrency`: uses goroutines, channels, sync

2. **Storage**: Behaviors stored in `Node.Metadata["behaviors"]` as string slice

3. **Search Tool**: `search_by_behavior(query, behaviors[])` combines:
   - Tag filtering (AND logic)
   - Semantic search on summaries
   - Ranked results

## Consequences

**Positive:**
- Precise behavioral queries
- Combined with semantic search for flexibility
- No schema changes (uses metadata)

**Negative:**
- LLM cost during indexing
- Requires LLM configuration
- Accuracy depends on LLM quality
```

**Step 3: Update ADR index**

Add to `adr/README.md` table:

```markdown
| [0015](0015-behavioral-search.md) | Behavioral Function Search | Accepted |
```

**Step 4: Run tests to verify nothing broke**

Run: `go test ./... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add README.md adr/0015-behavioral-search.md adr/README.md
git commit -m "docs: add behavioral search documentation (ADR-0015)"
```

---

## Task 9: Run full test suite and create release

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Run linter**

Run: `golangci-lint run`
Expected: No errors

**Step 3: Build**

Run: `go build ./...`
Expected: Success

**Step 4: Commit any remaining changes**

```bash
git status
git add -A
git commit -m "chore: final cleanup for behavioral search feature"
```

**Step 5: Create tag and push**

```bash
git push
git tag -a v0.3.0 -m "feat: add behavioral function search

- Add behavior extraction during indexing (logging, error-handle, database, http-client, file-io, concurrency)
- Create search_by_behavior MCP tool with tag filtering + semantic search
- Store behaviors in Node.Metadata
- Document in ADR-0015"
git push origin v0.3.0
```
