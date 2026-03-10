# Design: Generate `returns` and `accepts` edges

**Date**: 2026-03-10
**Status**: Approved

## Problem

The type checker creates type/interface nodes and `implements` edges, but no edges connect functions to the types they accept as parameters or return. This leaves ~70% of type nodes isolated in the web graph with zero connections.

## Solution

Extend the type checker (`internal/types/checker.go`) to emit `returns` and `accepts` edges by inspecting function signatures via `go/types`.

## Design

### New pass in `Check()`

After collecting types and interfaces, iterate over all functions in each package scope:

```
for each func in pkg.Types.Scope():
    sig := func.Type().(*types.Signature)
    funcID := func_<pkgPath>_<name>_<file>:<line>

    for each param type in sig.Params():
        if named type with graph node → emit accepts edge

    for each result type in sig.Results():
        if named type with graph node → emit returns edge
```

### Methods

For methods (functions with receivers), iterate over `types.Named.NumMethods()` for each named type. Same signature inspection logic.

### Type unwrapping (`namedTypeID` helper)

Recursively unwrap to find `*types.Named`:
- `*T` → unwrap to `T`
- `[]T` → unwrap to `T`
- `map[K]V` → emit edges for both `K` and `V`
- `chan T` → unwrap to `T`
- Built-ins (`string`, `int`, `error`, `bool`, `byte`, `rune`, `any`, `context.Context`) → skip (no graph nodes)

Returns `type_<pkgPath>_<name>` matching existing `typeToNode` convention.

### Function ID matching (`buildFuncID` helper)

Construct `func_<pkgPath>_<name>_<file>:<line>` matching the parser's `GenerateID()`:
- `pkgPath` from `pkg.PkgPath`
- `name` from `types.Func.Name()`
- `file:line` from `c.fset.Position(func.Pos())`

### Edge semantics

- `accepts`: function → type (one per unique named parameter type)
- `returns`: function → type (one per unique named return type)
- Deduplicate: if a function accepts `(a Config, b Config)`, emit one `accepts` edge to `Config`

### What doesn't change

- **Parser**: untouched
- **Indexer**: untouched — already merges `CheckResult.Edges` into graph
- **Web frontend**: untouched — edge toggles for `returns`/`accepts` already exist
- **Edge type constants**: already defined in `edge.go`

## Testing

- Extend `internal/types/checker_test.go` with a test module containing functions with struct params/returns
- Verify `accepts` and `returns` edges appear with correct type IDs
- Verify methods on structs also get edges
- Verify built-in types are skipped

## Implementation plan

1. Add `namedTypeID` helper to extract type node ID from `types.Type`
2. Add `buildFuncID` helper to construct function node ID
3. Add function/method iteration pass in `Check()` emitting edges
4. Add tests
5. Rebuild, re-index, verify in browser
