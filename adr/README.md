# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the mcp-code-graph project.

## What is an ADR?

An ADR is a document that captures an important architectural decision made along with its context and consequences. ADRs help future maintainers understand the "why" behind decisions.

## ADR Index

| Number | Title | Status |
|--------|-------|--------|
| [0001](0001-record-architecture-decisions.md) | Record Architecture Decisions | Accepted |
| [0002](0002-use-go-ast-over-tree-sitter.md) | Use Go AST over Tree-sitter for Initial Go Support | Accepted |
| [0003](0003-custom-graph-engine-with-sqlite-persistence.md) | Custom Graph Engine with SQLite Persistence | Accepted |
| [0004](0004-hybrid-indexing-strategy.md) | Hybrid Indexing Strategy | Accepted |
| [0005](0005-hybrid-summary-generation.md) | Hybrid Summary Generation | Accepted |
| [0006](0006-mcp-interface-design.md) | MCP Interface Design | Accepted |
| [0007](0007-project-structure.md) | Project Structure | Accepted |
| [0008](0008-mcp-protocol-implementation.md) | MCP Protocol Implementation | Accepted |
| [0009](0009-auto-project-detection.md) | Automatic Project Detection and Database Location | Accepted |
| [0010](0010-embedding-provider.md) | Embedding Provider for Semantic Search | Accepted |
| [0011](0011-llm-provider.md) | LLM Provider for Function Summaries | Accepted |
| [0012](0012-debug-mode.md) | Debug Mode with Structured Logging | Accepted |
| [0013](0013-get-function-by-name-tool.md) | get_function_by_name Tool | Accepted |
| [0014](0014-implicit-interface-resolver.md) | Implicit Interface Resolver | Accepted |
| [0015](0015-behavioral-search.md) | Behavioral Function Search | Accepted |

## Creating a New ADR

1. Copy the template below to a new file: `NNNN-short-title.md`
2. Fill in the sections
3. Submit for review

## Template

```markdown
# N. Title

Date: YYYY-MM-DD

## Status

[Proposed | Accepted | Deprecated | Superseded]

## Context

What is the issue that we're seeing that is motivating this decision or change?

## Decision

What is the change that we're proposing and/or doing?

## Consequences

What becomes easier or more difficult to do because of this change?

## Alternatives Considered

(Optional) What other options were considered? Why were they not chosen?
```

## References

- [Documenting Architecture Decisions - Michael Nygard](http://thinkrelevance.com/blog/2011/11/15/documenting-architecture-decisions)
- [Architecture Decision Records - ProductPlan](https://www.productplan.com/glossary/architecture-decision-record/)
