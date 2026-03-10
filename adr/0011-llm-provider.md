# ADR-0011: LLM Provider for Function Summaries

## Status

Accepted

## Context

The MCP code graph server generates function summaries using an LLM, but currently only supports a `MockProvider` that returns placeholder text. The `--model` CLI flag exists but is not connected to any real LLM API.

We need to enable real LLM-based summaries, following the same pattern as the embedding provider (ADR-0010).

## Decision

We will implement an `LLMProvider` interface with an OpenAI-compatible implementation, configured via `LLM_CONFIG` environment variable.

### LLMProvider Interface

```go
type LLMProvider interface {
    GenerateSummary(ctx context.Context, req SummaryRequest) (string, error)
}
```

### Configuration

LLM is configured via `LLM_CONFIG` environment variable with the same structure as `EMBEDDING_CONFIG`:

```json
{
  "provider": "openai",
  "api_key": "sk-...",
  "model": "gpt-4o-mini",
  "base_url": "https://api.openai.com/v1"
}
```

### Ollama Example

```json
{
  "provider": "openai",
  "base_url": "http://localhost:11434/v1",
  "model": "qwen2.5:7b"
}
```

### On-Demand Generation

Summaries are generated on-demand when:
- A function is first accessed during semantic search
- The `update_summary` tool is called without providing a summary

### Graceful Fallback

If LLM provider is not configured:
- Server starts normally with MockProvider
- Summaries return basic placeholder text

## Consequences

### Positive

- Real LLM-generated summaries improve code understanding
- Works with any OpenAI-compatible API (OpenAI, Ollama, vLLM)
- Consistent configuration pattern with embedding provider
- Graceful degradation when not configured

### Negative

- Additional API costs for summary generation
- First access to functions may be slower while generating summaries
- Requires API key or local server for real summaries

### Neutral

- LLM provider interface allows future providers (Anthropic, local models, etc.)
