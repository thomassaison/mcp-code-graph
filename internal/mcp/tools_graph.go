// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

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

func (s *Server) handleGetNeighborhood(ctx context.Context, args map[string]any) (string, error) {
	nodeID, ok := args["node_id"].(string)
	if !ok {
		return "", fmt.Errorf("node_id must be a string")
	}

	depth := 2
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	nodes, edges := s.graph.GetNeighborhood(nodeID, depth)

	var nodeResults []map[string]any
	for _, n := range nodes {
		nodeResults = append(nodeResults, map[string]any{
			"id":        n.ID,
			"name":      n.Name,
			"package":   n.Package,
			"type":      n.Type,
			"signature": n.Signature,
			"summary":   n.SummaryText(),
		})
	}

	var edgeResults []map[string]any
	for _, e := range edges {
		edgeResults = append(edgeResults, map[string]any{
			"from": e.From,
			"to":   e.To,
			"type": e.Type,
		})
	}

	result := map[string]any{
		"center": nodeID,
		"depth":  depth,
		"nodes":  nodeResults,
		"edges":  edgeResults,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleTraceChain(ctx context.Context, args map[string]any) (string, error) {
	fromID, ok := args["from_id"].(string)
	if !ok {
		return "", fmt.Errorf("from_id must be a string")
	}
	toID, ok := args["to_id"].(string)
	if !ok {
		return "", fmt.Errorf("to_id must be a string")
	}

	maxDepth := 10
	if d, ok := args["max_depth"].(float64); ok {
		maxDepth = int(d)
	}

	path, err := s.graph.FindPath(fromID, toID, maxDepth)
	if err != nil {
		return "", fmt.Errorf("trace chain: %w", err)
	}

	result := map[string]any{
		"from":        fromID,
		"to":          toID,
		"path_length": len(path),
		"path":        formatNodeList(path),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func (s *Server) handleGetNeighborhoodMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	nodeID, err := req.RequireString("node_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{"node_id": nodeID}
	if d, ok := req.GetArguments()["depth"].(float64); ok {
		args["depth"] = d
	}

	result, err := s.handleGetNeighborhood(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleTraceChainMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	fromID, err := req.RequireString("from_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toID, err := req.RequireString("to_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{"from_id": fromID, "to_id": toID}
	if d, ok := req.GetArguments()["max_depth"].(float64); ok {
		args["max_depth"] = d
	}

	result, err := s.handleTraceChain(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}
