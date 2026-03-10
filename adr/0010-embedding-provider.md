# ADR-0010: Embedding Provider for Semantic Search

## Status

Accepted

## Context

The MCP code graph server has a vector store implementation (`internal/vector/store.go`) but it is not connected to the search function. The current `handleSearchFunctions` in `internal/mcp/tools.go` returns fake scores (1.0, 0.9, 0.8...) and only does exact name/package matching.

To enable semantic search over function summaries, we need:
1. An embedding provider to convert text to vectors
2. Integration with the search function
3. Configuration via MCP config file

## Decision

We will implement an `EmbeddingProvider` interface with an OpenAI-compatible implementation that supports any OpenAI-compatible API (OpenAI, Ollama, vLLM, local servers, etc.).

### EmbeddingProvider Interface

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}
```

### Configuration

Embedding is configured via MCP configuration file with the following structure:

```json
{
  "mcpServers": {
    "vector-db": {
      "env": {
        "EMBEDDING_CONFIG": "{\"provider\":\"openai\",\"api_key\":\"sk-...\",\"model\":\"text-embedding-3-small\",\"base_url\":\"https://api.openai.com/v1\"}"
      }
    }
  }
}
```

Configuration fields:
- `provider`: Provider type (currently only "openai" supported, extensible for future)
- `api_key`: API key for authentication (optional for local servers like Ollama)
- `model`: Embedding model to use (e.g., "text-embedding-3-small", "nomic-embed-text" for Ollama)
- `base_url`: Server address (default: "https://api.openai.com/v1"). Enables Ollama, vLLM, or any OpenAI-compatible server.

### Ollama Example

```json
{
  "provider": "openai",
  "base_url": "http://localhost:11434/v1",
  "model": "nomic-embed-text",
  "api_key": "ollama"
}
```

### On-Demand Embedding Strategy

We chose **fully lazy embedding**:
- Query is embedded on every search
- Function summaries are embedded lazily when first searched
- No upfront batch embedding step required

### Graceful Fallback

If embedding provider is not configured:
- Server starts normally with a warning log
- Search falls back to name/package substring matching
- No semantic search capability

## Consequences

### Positive

- Semantic search enabled with real similarity scores
- Works with any OpenAI-compatible API (OpenAI, Ollama, vLLM, local models)
- Flexible configuration for different deployment scenarios
- No required upfront processing - works on-demand
- Graceful degradation when not configured

### Negative

- First search may be slow while generating embeddings
- Requires API key or local server for semantic search
- Additional API costs for embedding generation

### Neutral

- Embedding provider interface allows future providers (Cohere, local models, etc.)
