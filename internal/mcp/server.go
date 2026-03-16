// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/thomassaison/mcp-code-graph/internal/behavior"
	"github.com/thomassaison/mcp-code-graph/internal/embedding"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/indexer"
	"github.com/thomassaison/mcp-code-graph/internal/llm"
	goparser "github.com/thomassaison/mcp-code-graph/internal/parser/go"
	"github.com/thomassaison/mcp-code-graph/internal/summary"
	"github.com/thomassaison/mcp-code-graph/internal/vector"
)

type Config struct {
	DBPath      string
	ProjectPath string
	Embedding   *embedding.Config
	LLM         *llm.Config
}

type Server struct {
	graph     *graph.Graph // shared, owned by Server
	search    *SearchService
	index     *IndexService
	vector    *vector.Store    // still needed for WatchProject and Close
	persister *graph.Persister // still needed for WatchProject and Close
	config    *Config
	// embeddingProvider is kept on Server so tests and GenerateSummaries can access it.
	embeddingProvider embedding.EmbeddingProvider
	llmProvider       llm.LLMProvider
	ready             chan struct{}
	readyOnce         sync.Once
}

func NewServer(cfg *Config) (*Server, error) {
	gr := graph.New()

	vecStore, err := vector.NewStore(cfg.DBPath + ".vec.db")
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	p := goparser.New()
	persister, err := graph.NewPersister(cfg.DBPath + ".graph.db")
	if err != nil {
		return nil, fmt.Errorf("create persister: %w", err)
	}

	// Create LLM provider (falls back to MockProvider if not configured)
	llmProvider, err := llm.NewProviderFromConfig(cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("create LLM provider: %w", err)
	}

	// Create behavior analyzer (only with real LLM provider, not mock)
	var behavAnalyzer behavior.Analyzer
	if _, isMock := llmProvider.(*llm.MockProvider); !isMock && cfg.LLM != nil && cfg.LLM.Provider != "" {
		behavAnalyzer = behavior.NewLLMAnalyzer(llmProvider)
	}
	idx := indexer.NewWithBehaviorAnalyzer(gr, p, behavAnalyzer)

	gen := summary.NewGenerator(llmProvider, "")

	// Create embedding provider (nil if not configured)
	var embProvider embedding.EmbeddingProvider
	if cfg.Embedding != nil {
		embProvider, err = embedding.NewProviderFromConfig(cfg.Embedding)
		if err != nil {
			return nil, fmt.Errorf("create embedding provider: %w", err)
		}
	}

	searchSvc := NewSearchService(gr, vecStore, embProvider, gen)
	indexSvc := NewIndexService(gr, idx, persister, gen, cfg)

	ready := make(chan struct{})
	close(ready) // ready by default; call PrepareAsyncIndex() before background indexing
	return &Server{
		graph:             gr,
		search:            searchSvc,
		index:             indexSvc,
		vector:            vecStore,
		persister:         persister,
		config:            cfg,
		embeddingProvider: embProvider,
		llmProvider:       llmProvider,
		ready:             ready,
	}, nil
}

// LoadGraph loads the previously persisted graph from the database.
// This is fast (<1s) and should be called synchronously at startup so tools
// have data available immediately while a background reindex runs.
func (s *Server) LoadGraph() {
	s.index.LoadGraph()
}

// IndexProject re-parses all source files and saves the updated graph.
// LoadGraph() should be called first to hydrate the graph from the database.
// Safe to call from a background goroutine.
func (s *Server) IndexProject() error {
	return s.index.IndexProject()
}

// GenerateSummaries generates LLM summaries for all functions.
// If an embedding provider is configured, also computes and stores vector embeddings.
// This is optional and can be called after indexing.
func (s *Server) GenerateSummaries(ctx context.Context) error {
	return s.index.GenerateSummaries(ctx, s.search)
}

// RegisterTools registers all MCP tools with the given server.
func (s *Server) RegisterTools(mcpServer *mcpserver.MCPServer) {
	s.addSearchFunctionsTool(mcpServer)
	s.addGetCallersTool(mcpServer)
	s.addGetCalleesTool(mcpServer)
	s.addReindexProjectTool(mcpServer)
	s.addUpdateSummaryTool(mcpServer)
	s.addGetFunctionByNameTool(mcpServer)
	s.addGetImplementorsTool(mcpServer)
	s.addGetInterfacesTool(mcpServer)
	s.addSearchByBehaviorTool(mcpServer)
	s.addGetNeighborhoodTool(mcpServer)
	s.addGetImpactTool(mcpServer)
	s.addTraceChainTool(mcpServer)
	s.addGetContractTool(mcpServer)
	s.addDiscoverPatternsTool(mcpServer)
	s.addFindTestsTool(mcpServer)
	s.addGetFunctionContextTool(mcpServer)
}

// RegisterResources registers all MCP resources with the given server.
func (s *Server) RegisterResources(mcpServer *mcpserver.MCPServer) {
	// Function resource template
	mcpServer.AddResource(
		mcp.NewResource(
			"function://{package}/{name}",
			"Function",
			mcp.WithResourceDescription("Get function details by package and name"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleFunctionResourceMCP,
	)

	// Package resource template
	mcpServer.AddResource(
		mcp.NewResource(
			"package://{name}",
			"Package",
			mcp.WithResourceDescription("Get package overview"),
			mcp.WithMIMEType("application/json"),
		),
		s.handlePackageResourceMCP,
	)
}

// MCP tool registration helpers

func (s *Server) addSearchFunctionsTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("search_functions",
		mcp.WithDescription("Search for functions by name or semantic similarity"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query"),
		),
		mcp.WithString("package",
			mcp.Description("Optional package name to scope the search to"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return"),
		),
	)
	mcpServer.AddTool(tool, s.handleSearchFunctionsMCP)
}

func (s *Server) addGetCallersTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_callers",
		mcp.WithDescription("Get all functions that call this function"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetCallersMCP)
}

func (s *Server) addGetCalleesTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_callees",
		mcp.WithDescription("Get all functions called by this function"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetCalleesMCP)
}

func (s *Server) addReindexProjectTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("reindex_project",
		mcp.WithDescription("Trigger full reindex of the project"),
	)
	mcpServer.AddTool(tool, s.handleReindexProjectMCP)
}

func (s *Server) addUpdateSummaryTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("update_summary",
		mcp.WithDescription("Update a function's summary"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function"),
		),
		mcp.WithString("summary",
			mcp.Required(),
			mcp.Description("The new summary text"),
		),
	)
	mcpServer.AddTool(tool, s.handleUpdateSummaryMCP)
}

func (s *Server) addGetFunctionByNameTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_function_by_name",
		mcp.WithDescription("Get functions by exact name, optionally filtered by package or file"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The exact function name to search for"),
		),
		mcp.WithString("package",
			mcp.Description("Optional package name to filter by"),
		),
		mcp.WithString("file",
			mcp.Description("Optional file path substring to filter by"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetFunctionByNameMCP)
}

func (s *Server) addGetImplementorsTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_implementors",
		mcp.WithDescription("Find all types that implement a given interface"),
		mcp.WithString("interface_id",
			mcp.Required(),
			mcp.Description("Interface to query (e.g., io.Reader)"),
		),
		mcp.WithBoolean("include_pointer",
			mcp.Description("Include pointer variants (default: true)"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetImplementorsMCP)
}

func (s *Server) addGetInterfacesTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_interfaces",
		mcp.WithDescription("Find all interfaces that a given type implements"),
		mcp.WithString("type_id",
			mcp.Required(),
			mcp.Description("Type to query (e.g., os.File)"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetInterfacesMCP)
}

func (s *Server) addSearchByBehaviorTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("search_by_behavior",
		mcp.WithDescription("Search for functions by behavior (logging, error handling, database access, etc.) combined with semantic search"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The semantic search query describing the function purpose"),
		),
		mcp.WithArray("behaviors",
			mcp.Description("Behavior tags to filter by (AND logic): logging, error-handle, database, http-client, file-io, concurrency"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return"),
		),
	)
	mcpServer.AddTool(tool, s.handleSearchByBehaviorMCP)
}

func (s *Server) addGetNeighborhoodTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_neighborhood",
		mcp.WithDescription("Get the local call graph around a node (bidirectional traversal of callers and callees)"),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The ID of the node to explore"),
		),
		mcp.WithNumber("depth",
			mcp.Description("How many steps to traverse (default 2)"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetNeighborhoodMCP)
}

func (s *Server) addGetImpactTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_impact",
		mcp.WithDescription("Analyze the blast radius of changing a function: direct/indirect callers, tests affected, and risk level"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function to analyze"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetImpactMCP)
}

func (s *Server) addTraceChainTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("trace_chain",
		mcp.WithDescription("Find the shortest call path between two functions"),
		mcp.WithString("from_id",
			mcp.Required(),
			mcp.Description("The ID of the source function"),
		),
		mcp.WithString("to_id",
			mcp.Required(),
			mcp.Description("The ID of the target function"),
		),
		mcp.WithNumber("max_depth",
			mcp.Description("Maximum path length to search (default 10)"),
		),
	)
	mcpServer.AddTool(tool, s.handleTraceChainMCP)
}

func (s *Server) addGetContractTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_contract",
		mcp.WithDescription("Get the contract of a function: what it accepts/returns, which interfaces its type implements, who depends on it, and what tests exercise it"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetContractMCP)
}

func (s *Server) addDiscoverPatternsTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("discover_patterns",
		mcp.WithDescription("Discover code patterns in a package: constructors, error-handling, tests, entrypoints, sinks, sources, hotspots"),
		mcp.WithString("package",
			mcp.Required(),
			mcp.Description("The package to analyze"),
		),
		mcp.WithString("pattern_type",
			mcp.Required(),
			mcp.Description("Pattern type: constructors, error-handling, tests, entrypoints, sinks, sources, hotspots"),
		),
	)
	mcpServer.AddTool(tool, s.handleDiscoverPatternsMCP)
}

func (s *Server) addFindTestsTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("find_tests",
		mcp.WithDescription("Find test functions that exercise a given function (traverses callers to find Test/Benchmark/Example functions)"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function"),
		),
	)
	mcpServer.AddTool(tool, s.handleFindTestsMCP)
}

func (s *Server) addGetFunctionContextTool(mcpServer *mcpserver.MCPServer) {
	tool := mcp.NewTool("get_function_context",
		mcp.WithDescription("Get complete LLM-ready context for a function: code, callers, callees, contract, tests — everything in one call"),
		mcp.WithString("function_id",
			mcp.Required(),
			mcp.Description("The ID of the function"),
		),
	)
	mcpServer.AddTool(tool, s.handleGetFunctionContextMCP)
}

// WatchProject starts a filesystem watcher that incrementally re-indexes .go files.
// Returns the watcher, which must be closed via Watcher.Close() during shutdown.
func (s *Server) WatchProject(debounce time.Duration) (*indexer.Watcher, error) {
	return indexer.NewWatcher(s.index.indexer, debounce, s.persister, s.vector)
}

// Close persists the graph and releases all resources.
// The caller should ensure background indexing has completed before calling Close
// to avoid database contention.
func (s *Server) Close() {
	if err := s.persister.Save(s.graph); err != nil {
		slog.Error("failed to save graph on close", "error", err)
	}
	s.vector.Close()
	if err := s.persister.Close(); err != nil {
		slog.Error("failed to close persister", "error", err)
	}
}

func (s *Server) Graph() *graph.Graph {
	return s.graph
}

// PrepareAsyncIndex resets the ready state so tools will return "still indexing"
// until MarkReady() is called. Call this before starting background indexing.
func (s *Server) PrepareAsyncIndex() {
	s.ready = make(chan struct{})
	s.readyOnce = sync.Once{}
}

// IsReady returns true if the initial indexing is complete and the graph is populated.
func (s *Server) IsReady() bool {
	select {
	case <-s.ready:
		return true
	default:
		return false
	}
}

// MarkReady signals that the initial indexing is complete.
// Safe to call multiple times.
func (s *Server) MarkReady() {
	s.readyOnce.Do(func() {
		close(s.ready)
	})
}

// ReadyChan returns the channel that is closed when indexing is complete.
func (s *Server) ReadyChan() <-chan struct{} {
	return s.ready
}
