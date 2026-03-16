// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleSearchFunctions(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query must be a string")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	pkg, _ := args["package"].(string)

	slog.Debug("search functions", "query", query, "limit", limit, "package", pkg)

	if pkg != "" {
		return s.search.packageScopedSearch(ctx, query, pkg, limit)
	}

	if s.search.embeddingProvider != nil {
		slog.Debug("using semantic search")
		return s.search.semanticSearch(ctx, query, limit)
	}

	slog.Debug("using name search")
	return s.search.nameSearch(query, limit)
}

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

	if s.search.embeddingProvider != nil {
		return s.search.semanticBehaviorSearch(ctx, query, nodes, limit)
	}

	return s.search.formatBehaviorResults(nodes, limit), nil
}

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

	if pkg, ok := args["package"].(string); ok && pkg != "" {
		handlerArgs["package"] = pkg
	}

	result, err := s.handleSearchFunctions(ctx, handlerArgs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleSearchByBehaviorMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"query": query,
	}

	if behaviorsRaw, ok := req.GetArguments()["behaviors"]; ok {
		args["behaviors"] = behaviorsRaw
	}

	if limitRaw, ok := req.GetArguments()["limit"].(float64); ok {
		args["limit"] = limitRaw
	}

	result, err := s.handleSearchByBehavior(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}
