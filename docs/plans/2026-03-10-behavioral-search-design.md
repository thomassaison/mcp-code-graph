# Behavioral Function Search - Design Document

Date: 2026-03-10

## Overview

Add behavior extraction during indexing and a new MCP tool `search_by_behavior` that combines structured tag filtering with semantic search, enabling queries like "find functions that log errors."

## Problem Statement

Currently, `search_functions` relies on semantic search over summaries. While summaries now mention behaviors (like "logs errors"), they're unstructured text. Users want to:

- Find functions by specific behaviors: "functions that log errors"
- Filter by multiple behaviors: "HTTP handlers that access the database"
- Combine behavioral filters with semantic search

## Solution

Hybrid approach: structured behavior tags + semantic search on summaries.

### Core Behaviors (6 categories)

```go
const (
    BehaviorLogging      = "logging"       // logs messages, errors, debug info
    BehaviorErrorHandle  = "error-handle"  // creates, wraps, or handles errors
    BehaviorDatabase     = "database"      // reads/writes to database
    BehaviorHTTPClient   = "http-client"   // makes HTTP requests
    BehaviorFileIO       = "file-io"       // reads/writes files
    BehaviorConcurrency  = "concurrency"   // uses goroutines, channels, sync
)
```

## Architecture

```
┌─────────────┐    ┌──────────────────┐    ┌─────────────┐
│   Indexer   │───▶│ BehaviorAnalyzer │───▶│   Graph     │
└─────────────┘    └──────────────────┘    └─────────────┘
                          │                      │
                          ▼                      ▼
                    ┌─────────────┐        ┌─────────────┐
                    │ LLM Provider│        │  Metadata   │
                    └─────────────┘        │ behaviors[] │
                                           └─────────────┘

┌────────────────────────────────────────────────────────┐
│              search_by_behavior(query, behaviors[])     │
│  1. Filter nodes by behavior tags                       │
│  2. Semantic search on summaries within filtered set    │
│  3. Return ranked results                               │
└────────────────────────────────────────────────────────┘
```

## Components

### 1. Behavior Analyzer (`internal/behavior/analyzer.go`)

LLM-based behavior extraction:
- Input: function code, signature, docstring
- Output: list of detected behaviors from core categories
- Batches functions for efficiency (10-20 per LLM call)

### 2. Storage

Store behaviors in `Node.Metadata` as a string slice:
```go
node.Metadata["behaviors"] = []string{"logging", "error-handle"}
```

This requires no schema changes and persists with the graph.

### 3. MCP Tool: `search_by_behavior`

Parameters:
- `query` (string): semantic search query
- `behaviors` ([]string): behavior tags to filter by (AND logic)
- `limit` (int): max results (default: 10)

Flow:
1. Filter graph nodes by behavior tags (must have ALL specified behaviors)
2. Apply semantic search on summaries within filtered set
3. Rank by similarity score
4. Return top N results

### 4. Indexing Integration

After AST parsing and type checking, call behavior analyzer:
```go
behaviors := b.analyzer.Analyze(ctx, functions)
for _, fn := range functions {
    fn.Metadata["behaviors"] = behaviors[fn.ID]
}
```

## Data Flow

```
Parse Go files → AST nodes → Type check → Behavior extraction → Graph storage
                                                                          ↓
User query: "functions that log errors" ──▶ search_by_behavior(query="log errors", behaviors=["logging", "error-handle"])
                                                                          ↓
                                              Filter: nodes with BOTH tags
                                                                          ↓
                                              Semantic: rank by similarity to "log errors"
                                                                          ↓
                                              Return: top N functions
```

## API Example

### Request
```json
{
  "query": "log errors when validation fails",
  "behaviors": ["logging", "error-handle"],
  "limit": 10
}
```

### Response
```json
[
  {
    "id": "func_service_Validate_test.go:45",
    "name": "ValidateUser",
    "package": "service",
    "behaviors": ["logging", "error-handle", "validation"],
    "summary": "[Validator] Validates user input fields. Logs errors on failure."
  }
]
```

## Trade-offs

### Chosen Approach
- **Structured tags + semantic search**: Best of both worlds
- **Core behaviors only**: Focused, high-value, manageable
- **Metadata storage**: Simple, no schema changes

### Alternatives Considered
- **Semantic only**: Simpler but less precise filtering
- **Extended behaviors**: More comprehensive but higher LLM cost
- **Separate SQLite table**: Better query performance but more complexity

## Success Criteria

1. `search_by_behavior` returns accurate results for behavioral queries
2. Performance: < 100ms for filtering + semantic search on 10k functions
3. Behavior extraction accuracy: > 90% precision on test cases
4. No breaking changes to existing tools
