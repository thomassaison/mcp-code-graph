package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

func (s *Server) GetTools() []Tool {
	return []Tool{
		{
			Name:        "search_functions",
			Description: "Search for functions by name using semantic similarity or name matching",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Maximum number of results to return",
						"default":     10,
					},
				},
				"required": []string{"query"},
			},
			Handler: s.handleSearchFunctions,
		},
		{
			Name:        "get_callers",
			Description: "Get all functions that call this function",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{
						"type":        "string",
						"description": "The ID of the function",
					},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetCallers,
		},
		{
			Name:        "get_callees",
			Description: "Get all functions called by this function",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{
						"type":        "string",
						"description": "The ID of the function",
					},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetCallees,
		},
		{
			Name:        "reindex_project",
			Description: "Trigger full reindex of the project",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			Handler: s.handleReindexProject,
		},
		{
			Name:        "update_summary",
			Description: "Update a function's summary",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{
						"type":        "string",
						"description": "The ID of the function",
					},
					"summary": map[string]any{
						"type":        "string",
						"description": "The new summary text",
					},
				},
				"required": []string{"function_id", "summary"},
			},
			Handler: s.handleUpdateSummary,
		},
	}
}

func (s *Server) handleSearchFunctions(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query must be a string")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	slog.Debug("search functions", "query", query, "limit", limit)
	if s.embeddingProvider != nil {
		slog.Debug("using semantic search")
		return s.semanticSearch(ctx, query, limit)
	}

	slog.Debug("using name search")
	return s.nameSearch(query, limit)
}

func (s *Server) semanticSearch(ctx context.Context, query string, limit int) (string, error) {
	queryEmbedding, err := s.embeddingProvider.Embed(ctx, query)
	if err != nil {
		log.Printf("warning: failed to embed query, falling back to name search: %v", err)
		return s.nameSearch(query, limit)
	}
	slog.Debug("query embedded", "dim", len(queryEmbedding))

	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	for _, fn := range functions {
		if err := s.ensureFunctionEmbedding(ctx, fn); err != nil {
			log.Printf("warning: failed to embed function %s: %v", fn.Name, err)
		}
	}

	results, err := s.vector.Search(queryEmbedding, limit)
	if err != nil {
		log.Printf("warning: vector search failed, falling back to name search: %v", err)
		return s.nameSearch(query, limit)
	}
	slog.Debug("vector search complete", "results", len(results))

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

func (s *Server) ensureFunctionEmbedding(ctx context.Context, node *graph.Node) error {
	slog.Log(ctx, debug.LevelTrace, "ensuring function embedding", "function", node.Name)
	if node.Summary == nil || node.Summary.Text == "" {
		if err := s.summary.Generate(ctx, node); err != nil {
			return fmt.Errorf("generate summary: %w", err)
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

func (s *Server) nameSearch(query string, limit int) (string, error) {
	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	var results []map[string]any

	queryLower := strings.ToLower(query)

	for _, fn := range functions {
		nameLower := strings.ToLower(fn.Name)
		pkgLower := strings.ToLower(fn.Package)

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

func (s *Server) handleGetCallers(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	callers := s.graph.GetCallers(functionID)
	var results []map[string]any
	for _, caller := range callers {
		results = append(results, map[string]any{
			"id":        caller.ID,
			"name":      caller.Name,
			"package":   caller.Package,
			"signature": caller.Signature,
		})
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetCallees(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	callees := s.graph.GetCallees(functionID)
	var results []map[string]any
	for _, callee := range callees {
		results = append(results, map[string]any{
			"id":        callee.ID,
			"name":      callee.Name,
			"package":   callee.Package,
			"signature": callee.Signature,
		})
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleReindexProject(ctx context.Context, args map[string]any) (string, error) {
	if err := s.indexer.IndexModule(s.config.ProjectPath); err != nil {
		return "", fmt.Errorf("reindex failed: %w", err)
	}

	if err := s.persister.Save(s.graph); err != nil {
		return "", fmt.Errorf("save graph: %w", err)
	}

	return fmt.Sprintf("Reindexed project: %d nodes, %d edges", s.graph.NodeCount(), s.graph.EdgeCount()), nil
}

func (s *Server) handleUpdateSummary(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	summaryText, ok := args["summary"].(string)
	if !ok {
		return "", fmt.Errorf("summary must be a string")
	}

	node, err := s.graph.GetNode(functionID)
	if err != nil {
		return "", fmt.Errorf("function not found: %w", err)
	}

	now := time.Now().Unix()
	node.Summary = &graph.Summary{
		Text:        summaryText,
		GeneratedBy: "human",
		Model:       "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.persister.Save(s.graph); err != nil {
		log.Printf("failed to save graph after updating summary: %v", err)
	}

	return fmt.Sprintf("Updated summary for function %s", node.Name), nil
}

// MCP handler methods (mcp-go compatible)

func (s *Server) handleSearchFunctionsMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	limit := 10
	args := req.GetArguments()
	if args != nil {
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
	}

	handlerArgs := map[string]any{
		"query": query,
		"limit": float64(limit),
	}

	result, err := s.handleSearchFunctions(ctx, handlerArgs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGetCallersMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"function_id": functionID,
	}

	result, err := s.handleGetCallers(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGetCalleesMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"function_id": functionID,
	}

	result, err := s.handleGetCallees(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleReindexProjectMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := map[string]any{}

	result, err := s.handleReindexProject(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleUpdateSummaryMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	summaryText, err := req.RequireString("summary")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"function_id": functionID,
		"summary":     summaryText,
	}

	result, err := s.handleUpdateSummary(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}
