# 2. Use Go AST over Tree-sitter for Initial Go Support

Date: 2026-03-09

## Status

Accepted

## Context

We need to parse and analyze Go source code to build a code graph. Two primary options exist:

1. **Go standard library (`go/ast` + `go/types` + `golang.org/x/tools/go/packages`)**
   - Native Go tooling, understands Go semantics
   - Provides full type information and whole-program analysis
   - Go-specific, not portable to other languages

2. **Tree-sitter**
   - Multi-language parser generator with 40+ language support
   - Extremely fast incremental parsing
   - Syntax-only, no semantic understanding (types, interfaces, etc.)

Future requirement: The project may eventually support multiple programming languages.

## Decision

We will use **Go AST + `golang.org/x/tools/go/packages`** for initial Go support.

We will design the parser layer behind an interface that can later be implemented with tree-sitter for multi-language support.

## Consequences

### Positive

- **Rich semantic information**: Full type checking, interface satisfaction, method sets, type inference
- **Whole-program analysis**: Can trace calls across packages, resolve imports correctly
- **Mature tooling**: Well-documented, stable, part of Go standard library
- **Accurate call graphs**: `golang.org/x/tools/go/callgraph` provides precise static call graphs

### Negative

- **Go-only initially**: Cannot analyze other languages until tree-sitter layer is added
- **No incremental parsing**: File changes require re-analysis (mitigated by file watcher batching)
- **More development for multi-language**: Will need to add tree-sitter integration later

### Neutral

- Parser abstraction adds some complexity, but enables future extensibility

## Alternatives Considered

### Tree-sitter from Day One

Would provide multi-language support immediately, but would require building:
- Type inference engine
- Semantic analyzer for each language
- Cross-reference resolver

This represents significant additional complexity for unclear benefit given the initial Go-only scope.
