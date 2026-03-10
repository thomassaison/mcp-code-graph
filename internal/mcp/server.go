package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/thomas-saison/mcp-code-graph/internal/embedding"
	"github.com/thomas-saison/mcp-code-graph/internal/graph"
	"github.com/thomas-saison/mcp-code-graph/internal/indexer"
	goparser "github.com/thomas-saison/mcp-code-graph/internal/parser/go"
	"github.com/thomas-saison/mcp-code-graph/internal/summary"
	"github.com/thomas-saison/mcp-code-graph/internal/vector"
)

type Config struct {
	DBPath      string
	ProjectPath string
	LLMModel    string
	Embedding   *embedding.Config
}

type Server struct {
	graph     *graph.Graph
	vector    *vector.Store
	indexer   *indexer.Indexer
	summary   *summary.Generator
	persister *graph.Persister
	parser    *goparser.GoParser
	config    *Config
	embedding embedding.EmbeddingProvider
}

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

	var embProvider embedding.EmbeddingProvider
	if cfg.Embedding != nil {
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

// IndexProject indexes the project and generates summaries.
// Call this before starting the MCP server.
func (s *Server) IndexProject() error {
	// Load existing graph data (ignore errors - file may not exist yet)
	if err := s.persister.Load(s.graph); err != nil {
		// Non-critical: first run or corrupted file, will rebuild
	}

	if err := s.indexer.IndexModule(s.config.ProjectPath); err != nil {
		return fmt.Errorf("index project: %w", err)
	}

	if err := s.persister.Save(s.graph); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}

	return nil
}

// GenerateSummaries generates LLM summaries for all functions.
// This is optional and can be called after indexing.
func (s *Server) GenerateSummaries(ctx context.Context) error {
	return s.summary.GenerateAll(ctx, s.graph)
}

// RegisterTools registers all MCP tools with the given server.
func (s *Server) RegisterTools(mcpServer *mcpserver.MCPServer) {
	s.addSearchFunctionsTool(mcpServer)
	s.addGetCallersTool(mcpServer)
	s.addGetCalleesTool(mcpServer)
	s.addReindexProjectTool(mcpServer)
	s.addUpdateSummaryTool(mcpServer)
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
		mcp.WithDescription("Search for functions by name (stub - semantic search not yet implemented)"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query"),
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

// Start is deprecated. Use IndexProject() followed by MCP server.ServeStdio() instead.
func (s *Server) Start(ctx context.Context) error {
	// Load existing graph data (ignore errors - file may not exist yet)
	if err := s.persister.Load(s.graph); err != nil {
		// Non-critical: first run or corrupted file, will rebuild
	}

	if err := s.indexer.IndexModule(s.config.ProjectPath); err != nil {
		return fmt.Errorf("index project: %w", err)
	}

	// Generate summaries (ignore errors - non-critical for basic operation)
	if err := s.summary.GenerateAll(ctx, s.graph); err != nil {
		// Non-critical: summaries can be regenerated later
	}

	if err := s.persister.Save(s.graph); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}

	return nil
}

func (s *Server) Close() {
	if err := s.persister.Save(s.graph); err != nil {
		log.Printf("failed to save graph on close: %v", err)
	}
	s.vector.Close()
}

func (s *Server) Graph() *graph.Graph {
	return s.graph
}
