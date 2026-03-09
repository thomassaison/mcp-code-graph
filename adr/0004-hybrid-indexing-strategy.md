# 4. Hybrid Indexing Strategy

Date: 2026-03-09

## Status

Accepted

## Context

The code graph needs to stay synchronized with the codebase. Changes happen frequently during development, and we need to decide how to detect and process these changes.

Options:
1. **Manual trigger**: User runs a command to reindex
2. **File watcher**: Automatically detect changes and update
3. **Git hooks**: Reindex on commits/pushes
4. **Hybrid**: File watcher + manual trigger

## Decision

We will implement a **hybrid strategy**:

1. **File watcher** for incremental updates
2. **Manual trigger** for full reindex

### File Watcher Behavior

```
┌──────────────┐     ┌─────────────┐     ┌────────────────┐
│ File Change  │────▶│ Debounce    │────▶│ Incremental    │
│ (fsnotify)   │     │ (500ms)     │     │ Re-parse       │
└──────────────┘     └─────────────┘     └────────────────┘
                                               │
                                               ▼
                                        ┌────────────────┐
                                        │ Update Graph   │
                                        │ + Persist      │
                                        └────────────────┘
```

- Use `fsnotify` to watch `.go` files
- Debounce rapid changes (wait 500ms after last change)
- Incremental parse: only re-parse changed files
- Update affected graph nodes and edges
- Detect deleted files and remove their nodes

### Manual Full Reindex

Triggered via MCP tool: `reindex_project`

Use cases:
- After major refactoring
- When graph gets out of sync
- After pulling changes from another branch
- Initial setup

Full reindex:
1. Clear existing graph
2. Parse all `.go` files in module
3. Build complete graph
4. Generate embeddings for all functions
5. Persist to SQLite

## Consequences

### Positive

- **Responsive**: Changes reflected quickly without user action
- **Efficient**: Incremental updates minimize CPU/memory usage
- **Reliable**: Manual reindex available as fallback
- **Control**: User can force full reindex when needed

### Negative

- **Complexity**: Two code paths (incremental vs full)
- **Edge cases**: File renames, moves across packages need careful handling
- **Resource usage**: Background watcher thread

### Trade-offs Accepted

- File watcher may miss changes if too many files change rapidly (mitigated by debouncing)
- Incremental updates may leave stale edges if function signatures change significantly (mitigated by re-running type checker on affected packages)

## Implementation Notes

### Incremental Update Algorithm

```go
func (i *Indexer) handleFileChange(path string) error {
    // 1. Get package containing this file
    pkg := i.getPackage(path)
    
    // 2. Re-parse and type-check entire package
    //    (Go types depend on whole package)
    pkgInfo, err := i.parsePackage(pkg)
    
    // 3. Remove old nodes for this package's files
    i.graph.RemoveNodesForPackage(pkg.ID)
    
    // 4. Add new nodes and edges
    i.buildGraphForPackage(pkgInfo)
    
    // 5. Update affected edges (callers from other packages)
    i.updateCrossPackageEdges(pkg.ID)
    
    // 6. Persist to SQLite
    i.persist()
    
    return nil
}
```

### Watch Scope

- Watch all `.go` files in module
- Ignore `vendor/`, `.git/`, test cache directories
- Watch `go.mod` for dependency changes
