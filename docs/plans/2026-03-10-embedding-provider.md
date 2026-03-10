# Embedding Provider Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Connect the vector store to the search function, enabling semantic search over function summaries using OpenAI-compatible embedding API.

**Architecture:** Create EmbeddingProvider interface with OpenAI-compatible implementation. Search function embeds queries lazily, ensures function summaries have embeddings, then uses vector similarity search. Falls back to name matching if not configured.

**Tech Stack:** Go 1.22+, net/http for API calls, existing vector store

---

## Task 1: Create EmbeddingProvider Interface

**Files:**
- Create: `internal/embedding/provider.go`

**Step 1: Write the interface**

```go
// internal/embedding/provider.go
package embedding

import "context"

// Provider defines the interface for embedding providers.
type Provider interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	// Implementations should batch API calls for efficiency.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Config holds embedding provider configuration.
type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
}
```

**Step 2: Run build to verify**

Run: `go build ./internal/embedding/...`

Expected: Success (no errors)

**Step 3: Commit**

```bash
git add internal/embedding/
git commit -m "feat(embedding): add Provider interface and Config"
```

---

## Task 2: Implement OpenAI-Compatible Provider

**Files:**
- Create: `internal/embedding/openai.go`
- Create: `internal/embedding/openai_test.go`

**Step 1: Write failing test**

```go
// internal/embedding/openai_test.go
package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_Embed(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [{
				"embedding": [0.1, 0.2, 0.3],
				"index": 0
			}]
		}`))
	}))
	defer server.Close()

	cfg := &Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		BaseURL:  server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	embedding, err := provider.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 3 {
		t.Errorf("len(embedding) = %d, want 3", len(embedding))
	}
}

func TestOpenAIProvider_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"embedding": [0.1, 0.2, 0.3], "index": 0},
				{"embedding": [0.4, 0.5, 0.6], "index": 1}
			]
		}`))
	}))
	defer server.Close()

	cfg := &Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		BaseURL:  server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	embeddings, err := provider.EmbedBatch(context.Background(), []string{"text1", "text2"})
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("len(embeddings) = %d, want 2", len(embeddings))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/embedding/... -v`

Expected: FAIL - undefined: NewOpenAIProvider

**Step 3: Write implementation**

```go
// internal/embedding/openai.go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIProvider implements Provider for OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI-compatible embedding provider.
func NewOpenAIProvider(cfg *Config) (*OpenAIProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type embeddingRequest struct {
	Input string   `json:"input"`
	Model string   `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed generates an embedding for a single text.
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := embeddingRequest{
		Input: texts[0], // For simplicity, handle one at a time for now
		Model: p.model,
	}

	// If multiple texts, use array input
	if len(texts) > 1 {
		// Create request with array input
		reqMap := map[string]interface{}{
			"input": texts,
			"model": p.model,
		}
		body, err := json.Marshal(reqMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		defer resp.Body.Close()

		return p.parseResponse(resp.Body, len(texts))
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	return p.parseResponse(resp.Body, len(texts))
}

func (p *OpenAIProvider) parseResponse(body io.Reader, expectedCount int) ([][]float32, error) {
	var resp embeddingResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	// Sort by index to maintain order
	embeddings := make([][]float32, len(resp.Data))
	for _, item := range resp.Data {
		if item.Index >= len(embeddings) {
			return nil, fmt.Errorf("invalid index %d", item.Index)
		}
		embeddings[item.Index] = item.Embedding
	}

	return embeddings, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/embedding/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/embedding/
git commit -m "feat(embedding): implement OpenAI-compatible provider"
```

---

## Task 3: Add Config Parsing

**Files:**
- Create: `internal/embedding/config.go`
- Create: `internal/embedding/config_test.go`

**Step 1: Write failing test**

```go
// internal/embedding/config_test.go
package embedding

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	jsonConfig := `{"provider":"openai","api_key":"sk-test","model":"text-embedding-3-small","base_url":"http://localhost:11434/v1"}`

	cfg, err := ParseConfig(jsonConfig)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
	if cfg.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-test")
	}
	if cfg.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q, want %q", cfg.Model, "text-embedding-3-small")
	}
	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:11434/v1")
	}
}

func TestParseConfig_Empty(t *testing.T) {
	cfg, err := ParseConfig("")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if cfg != nil {
		t.Errorf("ParseConfig('') = %v, want nil", cfg)
	}
}

func TestNewProviderFromConfig(t *testing.T) {
	cfg := &Config{
		Provider: "openai",
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		BaseURL:  "http://localhost:11434/v1",
	}

	provider, err := NewProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewProviderFromConfig() error = %v", err)
	}

	if provider == nil {
		t.Error("NewProviderFromConfig() returned nil")
	}
}

func TestNewProviderFromConfig_Nil(t *testing.T) {
	provider, err := NewProviderFromConfig(nil)
	if err != nil {
		t.Fatalf("NewProviderFromConfig(nil) error = %v", err)
	}
	if provider != nil {
		t.Error("NewProviderFromConfig(nil) should return nil provider")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/embedding/... -v`

Expected: FAIL - undefined: ParseConfig

**Step 3: Write implementation**

```go
// internal/embedding/config.go
package embedding

import "encoding/json"

// ParseConfig parses embedding configuration from JSON string.
// Returns nil if the string is empty or invalid.
func ParseConfig(s string) (*Config, error) {
	if s == "" {
		return nil, nil
	}

	var cfg Config
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return nil, err
	}

	if cfg.Provider == "" {
		return nil, nil
	}

	return &cfg, nil
}

// NewProviderFromConfig creates a provider from configuration.
// Returns nil if config is nil or provider is not configured.
func NewProviderFromConfig(cfg *Config) (Provider, error) {
	if cfg == nil || cfg.Provider == "" {
		return nil, nil
	}

	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg)
	default:
		return nil, nil
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/embedding/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/embedding/
git commit -m "feat(embedding): add config parsing"
```

---

## Task 4: Update Server to Use Embedding Provider

**Files:**
- Modify: `internal/mcp/server.go`
- Create: `internal/mcp/server_embedding_test.go`

**Step 1: Write failing test**

```go
// internal/mcp/server_embedding_test.go
package mcp

import (
	"context"
	"testing"

	"github.com/thomas-saison/mcp-code-graph/internal/embedding"
)

func TestServer_WithEmbedding(t *testing.T) {
	cfg := &Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: t.TempDir(),
		Embedding: &embedding.Config{
			Provider: "openai",
			APIKey:   "test-key",
			Model:    "text-embedding-3-small",
		},
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if server.embedding == nil {
		t.Error("Server.embedding is nil, expected provider")
	}
}

func TestServer_WithoutEmbedding(t *testing.T) {
	cfg := &Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: t.TempDir(),
		Embedding:   nil,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if server.embedding != nil {
		t.Error("Server.embedding should be nil when not configured")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/... -v`

Expected: FAIL - Config has no Embedding field

**Step 3: Update Config struct in server.go**

```go
// internal/mcp/server.go
// Add import:
import (
	...
	"github.com/thomas-saison/mcp-code-graph/internal/embedding"
)

// Update Config struct:
type Config struct {
	DBPath      string
	ProjectPath string
	LLMModel    string
	Embedding   *embedding.Config // Add this line
}

// Update Server struct:
type Server struct {
	graph     *graph.Graph
	vector    *vector.Store
	indexer   *indexer.Indexer
	summary   *summary.Generator
	persister *graph.Persister
	parser    *goparser.GoParser
	config    *Config
	embedding embedding.Provider // Add this line
}

// Update NewServer function:
func NewServer(cfg *Config) (*Server, error) {
	gr := graph.New()

	vecStore, err := vector.NewStore(cfg.DBPath + ".vec.db")
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	p := goparser.New()
	idx := indexer.New(gr, p)
	persister := graph.NewPersister(cfg.DBPath + ".graph.db")

	var gen *summary.Generator
	if cfg.LLMModel != "" {
		gen = summary.NewGenerator(&summary.MockProvider{}, cfg.LLMModel)
	} else {
		gen = summary.NewGenerator(&summary.MockProvider{}, "mock")
	}

	// Create embedding provider
	var embProvider embedding.Provider
	if cfg.Embedding != nil {
		var err error
		embProvider, err = embedding.NewProviderFromConfig(cfg.Embedding)
		if err != nil {
			return nil, fmt.Errorf("create embedding provider: %w", err)
		}
	}

	return &Server{
		graph:     gr,
		vector:    vecStore,
		indexer:   idx,
		summary:   gen,
		persister: persister,
		parser:    p,
		config:    cfg,
		embedding: embProvider,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/
git commit -m "feat(mcp): add embedding provider to Server"
```

---

## Task 5: Update main.go to Read Embedding Config

**Files:**
- Modify: `cmd/mcp-code-graph/main.go`

**Step 1: Update main.go**

```go
// cmd/mcp-code-graph/main.go
// Add import:
import (
	...
	"github.com/thomas-saison/mcp-code-graph/internal/embedding"
)

// Update main function to read EMBEDDING_CONFIG env var:
func main() {
	projectPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("get working directory: %v", err)
	}

	dbPath := os.Getenv("MCP_CODE_GRAPH_DIR")
	if dbPath == "" {
		homeDir, _ := os.UserHomeDir()
		dbPath = filepath.Join(homeDir, ".mcp-code-graph", "data")
	}

	// Parse embedding config
	embeddingCfg, err := embedding.ParseConfig(os.Getenv("EMBEDDING_CONFIG"))
	if err != nil {
		log.Printf("warning: failed to parse embedding config: %v", err)
	}

	cfg := &mcp.Config{
		DBPath:      dbPath,
		ProjectPath: projectPath,
		Embedding:   embeddingCfg,
	}

	server, err := mcp.NewServer(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}
	...
}
```

**Step 2: Run build to verify**

Run: `go build ./cmd/mcp-code-graph/...`

Expected: Success

**Step 3: Commit**

```bash
git add cmd/mcp-code-graph/
git commit -m "feat(cli): read EMBEDDING_CONFIG from environment"
```

---

## Task 6: Implement Semantic Search in handleSearchFunctions

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/tools_test.go`

**Step 1: Write failing test for semantic search**

```go
// internal/mcp/tools_test.go
// Add to existing file

func TestHandleSearchFunctions_WithEmbedding(t *testing.T) {
	// Create server with mock embedding provider
	cfg := &Config{
		DBPath:      t.TempDir() + "/test.db",
		ProjectPath: t.TempDir(),
		Embedding: &embedding.Config{
			Provider: "openai",
			APIKey:   "test",
			Model:    "text-embedding-3-small",
		},
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Add a test function
	node := &graph.Node{
		ID:        "func_main_test_test.go:1",
		Type:      graph.NodeTypeFunction,
		Package:   "main",
		Name:      "connectDatabase",
		File:      "test.go",
		Line:      1,
		Signature: "func connectDatabase() *DB",
	}
	node.Summary = &graph.Summary{Text: "Connects to the PostgreSQL database"}
	server.graph.AddNode(node)

	// Search
	result, err := server.handleSearchFunctions(context.Background(), map[string]any{
		"query": "database connection",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearchFunctions() error = %v", err)
	}

	// Result should be valid JSON with the function
	if !strings.Contains(result, "connectDatabase") {
		t.Errorf("result should contain 'connectDatabase', got: %s", result)
	}
}
```

**Step 2: Run test to verify it fails or passes with stub**

Run: `go test ./internal/mcp/... -v -run TestHandleSearchFunctions_WithEmbedding`

Expected: Test runs (may pass with stub or fail with network error)

**Step 3: Implement semantic search**

```go
// internal/mcp/tools.go
// Add import:
import (
	...
	"strings"
)

// Replace handleSearchFunctions with:
func (s *Server) handleSearchFunctions(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query must be a string")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Use semantic search if embedding provider is configured
	if s.embedding != nil {
		return s.semanticSearch(ctx, query, limit)
	}

	// Fall back to name matching
	return s.nameSearch(query, limit)
}

// semanticSearch performs vector similarity search
func (s *Server) semanticSearch(ctx context.Context, query string, limit int) (string, error) {
	// 1. Embed the query
	queryEmbedding, err := s.embedding.Embed(ctx, query)
	if err != nil {
		// Log error and fall back to name search
		log.Printf("warning: failed to embed query, falling back to name search: %v", err)
		return s.nameSearch(query, limit)
	}

	// 2. Ensure all functions have embeddings
	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	for _, fn := range functions {
		if err := s.ensureFunctionEmbedding(ctx, fn); err != nil {
			log.Printf("warning: failed to embed function %s: %v", fn.Name, err)
		}
	}

	// 3. Search vector store
	results, err := s.vector.Search(queryEmbedding, limit)
	if err != nil {
		log.Printf("warning: vector search failed, falling back to name search: %v", err)
		return s.nameSearch(query, limit)
	}

	// 4. Enrich results with graph data
	var output []map[string]any
	for _, r := range results {
		node, err := s.graph.GetNode(r.NodeID)
		if err != nil {
			continue
		}
		output = append(output, map[string]any{
			"id":        node.ID,
			"name":      node.Name,
			"package":   node.Package,
			"signature": node.Signature,
			"summary":   node.SummaryText(),
			"score":     r.Score,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ensureFunctionEmbedding generates and stores embedding for a function if not exists
func (s *Server) ensureFunctionEmbedding(ctx context.Context, node *graph.Node) error {
	// Check if already embedded by trying to search for it
	// (This is a simple check - in production, track embedding status)

	// Generate summary if not exists
	if node.Summary == nil || node.Summary.Text == "" {
		if err := s.summary.Generate(ctx, node); err != nil {
			return fmt.Errorf("generate summary: %w", err)
		}
	}

	// Get text to embed (summary or name + signature)
	text := node.Summary.Text
	if text == "" {
		text = fmt.Sprintf("%s %s", node.Name, node.Signature)
	}

	// Embed
	embedding, err := s.embedding.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	// Store in vector store
	if err := s.vector.Insert(node.ID, text, embedding); err != nil {
		return fmt.Errorf("store embedding: %w", err)
	}

	return nil
}

// nameSearch performs simple name/package matching (fallback)
func (s *Server) nameSearch(query string, limit int) (string, error) {
	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	var results []map[string]any

	queryLower := strings.ToLower(query)

	for _, fn := range functions {
		nameLower := strings.ToLower(fn.Name)
		pkgLower := strings.ToLower(fn.Package)

		// Substring match on name or package
		if strings.Contains(nameLower, queryLower) || strings.Contains(pkgLower, queryLower) {
			results = append(results, map[string]any{
				"id":        fn.ID,
				"name":      fn.Name,
				"package":   fn.Package,
				"signature": fn.Signature,
			})

			if len(results) >= limit {
				break
			}
		}
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
```

**Step 4: Add helper method to Node**

```go
// internal/graph/node.go
// Add method:
func (n *Node) SummaryText() string {
	if n.Summary != nil {
		return n.Summary.Text
	}
	return ""
}
```

**Step 5: Run tests**

Run: `go test ./internal/mcp/... -v`

Expected: PASS

**Step 6: Commit**

```bash
git add internal/mcp/ internal/graph/
git commit -m "feat(search): implement semantic search with embedding provider"
```

---

## Task 7: Update Tool Description

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/server.go`

**Step 1: Update tool description**

```go
// internal/mcp/tools.go
// In GetTools(), update description:
{
	Name:        "search_functions",
	Description: "Search for functions by name or semantic similarity. Returns matching functions with similarity scores if embedding provider is configured.",
	...
}

// In server.go, update addSearchFunctionsTool:
func (s *Server) addSearchFunctionsTool(mcpServer *mcpserver.MCPServer) {
	description := "Search for functions by name or semantic similarity"
	if s.embedding != nil {
		description = "Search for functions by name or semantic similarity. Returns matching functions with similarity scores."
	}
	
	tool := mcp.NewTool("search_functions",
		mcp.WithDescription(description),
		...
	)
	mcpServer.AddTool(tool, s.handleSearchFunctionsMCP)
}
```

**Step 2: Run tests**

Run: `go test ./...`

Expected: PASS

**Step 3: Commit**

```bash
git add internal/mcp/
git commit -m "docs(tools): update search_functions description"
```

---

## Task 8: Run All Tests and Verify

**Step 1: Run all tests**

Run: `go test ./... -v`

Expected: All tests pass

**Step 2: Build binary**

Run: `go build -o bin/mcp-code-graph ./cmd/mcp-code-graph`

Expected: Binary created successfully

**Step 3: Final commit**

```bash
git add .
git commit -m "feat: complete embedding provider implementation"
```

---

## Summary

After completing these tasks:
- Embedding provider interface created
- OpenAI-compatible provider implemented (works with OpenAI, Ollama, vLLM, etc.)
- Configuration via `EMBEDDING_CONFIG` environment variable
- Semantic search connected to vector store
- Graceful fallback to name matching if not configured
