// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
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
		{
			Name:        "get_function_by_name",
			Description: "Get functions by exact name, optionally filtered by package or file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The exact function name to search for",
					},
					"package": map[string]any{
						"type":        "string",
						"description": "Optional exact package name to filter by",
					},
					"file": map[string]any{
						"type":        "string",
						"description": "Optional file path substring to filter by",
					},
				},
				"required": []string{"name"},
			},
			Handler: s.handleGetFunctionByName,
		},
		{
			Name:        "get_implementors",
			Description: "Find all types that implement a given interface",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"interface_id": map[string]any{
						"type":        "string",
						"description": "The ID of the interface",
					},
				},
				"required": []string{"interface_id"},
			},
			Handler: s.handleGetImplementors,
		},
		{
			Name:        "get_interfaces",
			Description: "Find all interfaces that a given type implements",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type_id": map[string]any{
						"type":        "string",
						"description": "The ID of the type",
					},
				},
				"required": []string{"type_id"},
			},
			Handler: s.handleGetInterfaces,
		},
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
		{
			Name:        "get_neighborhood",
			Description: "Get the local call graph around a node (bidirectional traversal)",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"node_id": map[string]any{"type": "string", "description": "The ID of the node"},
					"depth":   map[string]any{"type": "number", "description": "Steps to traverse (default 2)", "default": 2},
				},
				"required": []string{"node_id"},
			},
			Handler: s.handleGetNeighborhood,
		},
		{
			Name:        "get_impact",
			Description: "Analyze blast radius of changing a function: direct/indirect callers, tests, risk level",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{"type": "string", "description": "The ID of the function"},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetImpact,
		},
		{
			Name:        "trace_chain",
			Description: "Find shortest call path between two functions",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from_id":   map[string]any{"type": "string", "description": "Source function ID"},
					"to_id":     map[string]any{"type": "string", "description": "Target function ID"},
					"max_depth": map[string]any{"type": "number", "description": "Max path length (default 10)", "default": 10},
				},
				"required": []string{"from_id", "to_id"},
			},
			Handler: s.handleTraceChain,
		},
		{
			Name:        "get_contract",
			Description: "Get function contract: types accepted/returned, interfaces, callers, tests",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{"type": "string", "description": "The ID of the function"},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetContract,
		},
		{
			Name:        "discover_patterns",
			Description: "Discover code patterns in a package: constructors, error-handling, tests, entrypoints, sinks, sources, hotspots",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"package":      map[string]any{"type": "string", "description": "The package to analyze"},
					"pattern_type": map[string]any{"type": "string", "description": "Pattern type: constructors, error-handling, tests, entrypoints, sinks, sources, hotspots"},
				},
				"required": []string{"package", "pattern_type"},
			},
			Handler: s.handleDiscoverPatterns,
		},
		{
			Name:        "find_tests",
			Description: "Find test functions that exercise a given function",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{"type": "string", "description": "The ID of the function"},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleFindTests,
		},
		{
			Name:        "get_function_context",
			Description: "Get complete LLM-ready context: code, callers, callees, contract, tests — everything in one call",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_id": map[string]any{"type": "string", "description": "The ID of the function"},
				},
				"required": []string{"function_id"},
			},
			Handler: s.handleGetFunctionContext,
		},
	}
}
