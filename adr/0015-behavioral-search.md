# 15. Behavioral Function Search

Date: 2026-03-10

## Status

Accepted

## Context

Users want to find functions by what they DO, not just by name. Queries like "find functions that log errors" or "HTTP handlers that access the database" require understanding function behavior.

Existing semantic search on summaries helps but is imprecise for behavioral filtering.

## Decision

Implement hybrid behavioral search:

1. **Behavior Extraction**: LLM analyzes functions during indexing to identify core behaviors:
   - `logging`: logs messages, errors, debug info
   - `error-handle`: creates, wraps, or handles errors
   - `database`: reads/writes to database
   - `http-client`: makes HTTP requests
   - `file-io`: reads/writes files
   - `concurrency`: uses goroutines, channels, sync

2. **Storage**: Behaviors stored in `Node.Metadata["behaviors"]` as string slice

3. **Search Tool**: `search_by_behavior(query, behaviors[])` combines:
   - Tag filtering (AND logic)
   - Semantic search on summaries
   - Ranked results

## Consequences

**Positive:**
- Precise behavioral queries
- Combined with semantic search for flexibility
- No schema changes (uses metadata)

**Negative:**
- LLM cost during indexing
- Requires LLM configuration
- Accuracy depends on LLM quality
