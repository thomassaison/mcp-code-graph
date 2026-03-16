---
name: mcp-code-graph
description: Use when exploring code structure, understanding function relationships, searching for functions by purpose or exact name, or navigating call chains in a Go codebase. Triggers on questions like "what calls X", "find functions that do Y", "how does Z work", "show me the call chain for W", "get function by name".
---

# MCP Code Graph

## CRITICAL: Use ONLY MCP Tools

**DO NOT use Read, Grep, Glob, or cat/head/tail to explore Go code.** The MCP code graph tools provide all the information you need — function source code, signatures, callers, callees, contracts, tests, and more. Using file-reading tools defeats the purpose of the code graph and wastes context window.

The ONLY exception is for non-Go files (configs, docs, markdown, etc.) which are not indexed by the graph.

## Overview

This MCP server provides a **code graph database** for understanding Go codebases. It indexes all functions, tracks call relationships, resolves interfaces, analyzes impact, discovers patterns, and supports semantic search via embeddings. **16 tools + 2 resources.**

## When to Use

- Searching for functions by purpose or behavior ("find functions that handle errors")
- Understanding call relationships ("what calls NewServer?", "what does handleSearch call?")
- Navigating the codebase structure ("show me all functions in the mcp package")
- Answering "how does X work?" questions (combine search + callers/callees)
- Assessing change risk ("what breaks if I change this function?")
- Tracing execution paths ("how does main reach this handler?")
- Finding tests for a function ("what tests exercise this code?")
- Understanding function contracts ("what does this accept/return, who depends on it?")
- Discovering code patterns ("find all constructors in this package")
- Getting full context for an LLM prompt ("give me everything about this function")

## Tool Reference

### Search & Discovery

| Tool | Use For | Key Params |
|------|---------|------------|
| `search_functions` | Find functions by name or purpose (semantic/fuzzy) | `query` (required), `package` (optional scope), `limit` |
| `get_function_by_name` | Exact name lookup (fast O(1)) | `name` (required), `package`, `file` |
| `search_by_behavior` | Find functions by behavior tags + semantic search | `query` (required), `behaviors[]` (logging, error-handle, database, http-client, file-io, concurrency), `limit` |
| `discover_patterns` | Find code patterns in a package | `package` (required), `pattern_type` (required): constructors, error-handling, tests, entrypoints, sinks, sources, hotspots |

### Call Graph Navigation

| Tool | Use For | Key Params |
|------|---------|------------|
| `get_callers` | Who calls this function? | `function_id` (required) |
| `get_callees` | What does this function call? | `function_id` (required) |
| `get_neighborhood` | Local call graph around a node (bidirectional) | `node_id` (required), `depth` (default 2) |
| `trace_chain` | Shortest call path between two functions | `from_id` (required), `to_id` (required), `max_depth` (default 10) |

### Impact & Architecture Analysis

| Tool | Use For | Key Params |
|------|---------|------------|
| `get_impact` | Blast radius: direct/indirect callers, tests affected, risk level | `function_id` (required) |
| `get_contract` | Function contract: accepts/returns, interfaces, dependents, tests | `function_id` (required) |
| `find_tests` | Find Test/Benchmark/Example functions that exercise a function | `function_id` (required) |

### Type System

| Tool | Use For | Key Params |
|------|---------|------------|
| `get_implementors` | Find all types that implement an interface | `interface_id` (required, e.g. `io.Reader`), `include_pointer` (default true) |
| `get_interfaces` | Find all interfaces a type implements | `type_id` (required, e.g. `os.File`) |

### LLM-Optimized Context

| Tool | Use For | Key Params |
|------|---------|------------|
| `get_function_context` | Complete LLM-ready context: code, callers, callees, contract, tests | `function_id` (required) |

### Maintenance

| Tool | Use For | Key Params |
|------|---------|------------|
| `reindex_project` | Refresh graph after code changes | (none) |
| `update_summary` | Improve a function's description for better semantic search | `function_id` (required), `summary` (required) |

## Resources

| URI Pattern | Purpose |
|-------------|---------|
| `function://{package}/{name}` | Full function details: signature, file, line, callers, callees, summary |
| `package://{name}` | Package overview: all functions with signatures and summaries |

## Workflows

### Understanding a Function

1. **Find it:** `search_functions` with a descriptive query, or `get_function_by_name` if you know the name
2. **Get full context:** `get_function_context` for everything in one call (code, callers, callees, contract, tests)
3. **Or step by step:**
   - `get_callers` to see who uses it
   - `get_callees` to see what it depends on
   - `get_contract` for its interface contracts and test coverage
4. **Widen scope:** `get_neighborhood` for the surrounding call graph, or read `package://` resource

### Answering "How does X work?"

1. `search_functions` to find the entry point
2. `get_callees` to map the execution flow
3. `trace_chain` if you need to find the path between two specific functions
4. `get_function_context` for deep dives on specific functions

### Assessing Change Risk

1. `get_impact` to see blast radius (direct/indirect callers, affected tests, risk level)
2. `find_tests` to identify what test coverage exists
3. `get_contract` to understand interface obligations and dependents
4. `get_neighborhood` to visualize the surrounding graph

### Exploring a Package

1. `discover_patterns` with `constructors` to find entry points
2. `discover_patterns` with `entrypoints` for exported functions
3. `discover_patterns` with `hotspots` for the most-connected functions
4. `discover_patterns` with `tests` to see test coverage
5. Read `package://` resource for an overview of all functions

### Tracing Execution Paths

1. `trace_chain` with `from_id` and `to_id` to find the shortest call path
2. `get_neighborhood` with a higher `depth` to explore the graph around a node
3. `get_callers`/`get_callees` for one-hop exploration

## Pattern Types for `discover_patterns`

| Pattern | Finds |
|---------|-------|
| `constructors` | Functions starting with `New` that create instances |
| `error-handling` | Functions whose signature returns `error` |
| `tests` | Functions starting with `Test`, `Benchmark`, or `Example` |
| `entrypoints` | Exported functions (capitalized) with no callers (API surface) |
| `sinks` | Functions that don't call any other project functions (leaves) |
| `sources` | Functions that aren't called by anything (roots/entrypoints) |
| `hotspots` | Functions with the most connections (callers + callees), sorted by count |

## Function ID Format

Function IDs are full paths like:
```
func_github.com/org/repo/internal/mcp_NewServer_/path/to/file.go:44
```

Use `search_functions` or `get_function_by_name` first to discover the exact ID, then pass it to other tools.

## Tips

- **Start with `get_function_context`** for deep dives — it gives you everything in one call (code, callers, callees, contract, tests)
- **Use `get_function_by_name`** when you know the exact function name — faster and more precise than search
- **Package-scoped search:** Pass `package` param to `search_functions` to narrow results
- **Semantic search** works best with natural language: "functions that parse Go source code" beats "parse"
- **`get_impact` before refactoring** — always check blast radius before changing a function
- **`find_tests` before modifying** — know what tests you'll need to update
- **`trace_chain` for debugging** — find how execution reaches a specific function
- **`discover_patterns` for onboarding** — quickly understand a package's structure and conventions
- **After code changes**, call `reindex_project` to refresh the graph
- **Update summaries** when you understand a function better — improves future semantic search
