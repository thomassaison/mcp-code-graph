package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/thomas-saison/mcp-code-graph/internal/graph"
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
			Description: "Search for functions by name (stub - semantic search not yet implemented)",
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

	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	var results []map[string]any
	for i, fn := range functions {
		if i >= limit {
			break
		}
		results = append(results, map[string]any{
			"id":        fn.ID,
			"name":      fn.Name,
			"package":   fn.Package,
			"signature": fn.Signature,
			"score":     1.0 - float32(i)*0.1,
		})
	}

	for _, fn := range functions {
		if fn.Name == query || fn.Package == query {
			found := false
			for _, r := range results {
				if r["id"] == fn.ID {
					found = true
					break
				}
			}
			if !found && len(results) < limit {
				results = append(results, map[string]any{
					"id":        fn.ID,
					"name":      fn.Name,
					"package":   fn.Package,
					"signature": fn.Signature,
					"score":     float32(0.9),
				})
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
