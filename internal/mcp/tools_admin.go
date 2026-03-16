// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func (s *Server) handleReindexProject(ctx context.Context, args map[string]any) (string, error) {
	if err := s.index.indexer.IndexModule(s.config.ProjectPath); err != nil {
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
