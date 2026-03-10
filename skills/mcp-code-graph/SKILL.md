---
name: mcp-code-graph
description: Use when exploring code structure, understanding function relationships, searching for functions by purpose or exact name, or navigating call chains in a Go codebase. Triggers on questions like "what calls X", "find functions that do Y", "how does Z work", "show me the call chain for W", "get function by name".
---

# MCP Code Graph

## Overview

This MCP server provides a **code graph database** for understanding Go codebases. It indexes all functions, tracks call relationships, and supports semantic search via embeddings. **Prefer these tools over grep/glob for code understanding tasks.**

## When to Use

**Use MCP tools when:**
- Searching for functions by purpose or behavior ("find functions that handle errors")
- Understanding call relationships ("what calls NewServer?", "what does handleSearch call?")
- Navigating the codebase structure ("show me all functions in the mcp package")
- Answering "how does X work?" questions (combine search + callers/callees)

**Use grep/glob instead when:**
- Looking for exact string matches or regex patterns
- Searching non-Go files (configs, docs, etc.)
- Finding specific variable names or constants

## Quick Reference

| Tool | Use For | Example |
|------|---------|---------|
| `search_functions` | Find functions by name or purpose (semantic/fuzzy) | `{"query": "handle HTTP requests", "limit": 10}` |
| `get_function_by_name` | Find functions by exact name (O(1) lookup) | `{"name": "HandleRequest", "package": "internal/mcp"}` |
| `get_callers` | Who calls this function? | `{"function_id": "pkg.FuncName"}` |
| `get_callees` | What does this function call? | `{"function_id": "pkg.FuncName"}` |
| `reindex_project` | Refresh after code changes | `{}` |
| `update_summary` | Improve function description | `{"function_id": "pkg.FuncName", "summary": "..."}` |

## Resources

| URI Pattern | Purpose |
|-------------|---------|
| `function://{package}/{name}` | Full function details: signature, file, line, callers, callees, summary |
| `package://{name}` | Package overview: all functions with signatures and summaries |

## Workflow: Understanding a Function

1. **Find it:** `search_functions` with a descriptive query
2. **Inspect it:** Read the `function://` resource for full details
3. **Trace callers:** `get_callers` to see who uses it
4. **Trace callees:** `get_callees` to see what it depends on
5. **Widen scope:** Read the `package://` resource for sibling functions

## Workflow: Answering "How does X work?"

1. `search_functions` to find the entry point
2. `get_callees` to map the execution flow
3. Recursively follow callees for deeper understanding
4. Use `function://` resources to read signatures and summaries

## Function ID Format

Function IDs follow the pattern `package.FunctionName` or `package.Type.MethodName`. Use `search_functions` first to discover the exact ID, then pass it to `get_callers`/`get_callees`.

## Tips

- **Exact name lookups:** Use `get_function_by_name` when you know the function name exactly — it's faster and more reliable than `search_functions`
- **Semantic search** works best with natural language: "functions that parse Go source code" beats "parse"
- **After code changes**, call `reindex_project` to refresh the graph
- **Update summaries** when you understand a function better — this improves future semantic search
- **Combine tools:** search -> inspect -> trace callers/callees for full understanding
- **Start broad, narrow down:** search first, then follow relationships
