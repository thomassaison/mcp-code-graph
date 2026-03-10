# 14. Implicit Interface Resolver

Date: 2026-03-10

## Status

Accepted

## Context

Go interfaces are satisfied implicitly — there is no explicit declaration that a type implements an interface. This makes it difficult for AI assistants to discover which types can be used where an interface is expected.

When an AI assistant sees a function parameter of type `io.Reader`, it cannot easily find all possible types that could be passed to that function without analyzing the entire codebase and its dependencies.

## Decision

Implement implicit interface resolution using Go's type checker (`go/types`). The type checker runs as a separate pass after AST parsing and:

1. Extracts all interface definitions (including stdlib and external packages)
2. Extracts all named types
3. Uses `types.Implements()` to detect which types satisfy which interfaces
4. Creates `implements` edges in the graph

Provide two MCP tools:
- `get_implementors(interface_id)` — find types that implement an interface
- `get_interfaces(type_id)` — find interfaces a type implements

## Consequences

**Positive:**
- AI assistants can discover all implementations of any interface
- Supports stdlib interfaces (`io.Reader`, `http.Handler`, etc.)
- Tracks pointer receiver distinction (`*T` vs `T`)

**Negative:**
- Type checking adds overhead to indexing
- Full reindex required for type information updates
- External dependencies must be available for complete resolution
