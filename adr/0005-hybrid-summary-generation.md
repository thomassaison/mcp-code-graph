# 5. Hybrid Summary Generation

Date: 2026-03-09

## Status

Accepted

## Context

Each function in the code graph needs a "surcouche" — a simplified, human-readable summary of its purpose. This summary helps AI agents quickly understand a function's intent without reading the full implementation.

Options:
1. **Manual**: Developers write summaries
2. **Auto-generated**: LLM generates summaries during indexing
3. **Hybrid**: LLM generates initial, developers can edit
4. **Live queries**: Generate on-the-fly when asked

## Decision

We will use a **hybrid approach**: LLM generates initial summaries, developers can edit/refine them.

### Workflow

```
┌──────────────────────────────────────────────────────────┐
│                    Indexing Phase                        │
├──────────────────────────────────────────────────────────┤
│  1. Parse function                                       │
│  2. Extract: signature, docstring, surrounding context   │
│  3. Call LLM (via configurable provider)                 │
│  4. Store summary in graph node                          │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│                    Update Phase                          │
├──────────────────────────────────────────────────────────┤
│  MCP tool: `update_summary(function_id, summary)`        │
│  - Allows manual refinement                              │
│  - Tracks last modified by (LLM or human)                │
│  - Preserves edit history in SQLite                      │
└──────────────────────────────────────────────────────────┘
```

### Summary Schema

```go
type Summary struct {
    Text         string    `json:"text"`
    GeneratedBy  string    `json:"generated_by"`  // "llm" or "human"
    Model        string    `json:"model"`         // LLM model used, if applicable
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
    UpdatedBy    string    `json:"updated_by"`    // "llm" or user identifier
}
```

### LLM Prompt Template

```
Summarize this Go function in 1-2 sentences. Focus on WHAT it does, not HOW.

Function: {{.Name}}
Signature: {{.Signature}}
Package: {{.Package}}
Docstring: {{.Docstring}}

Summary:
```

### LLM Provider Configuration

Configured via MCP server settings:

```json
{
  "summary_provider": {
    "type": "openai",  // or "anthropic", "ollama", "mock"
    "model": "gpt-4o-mini",
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

## Consequences

### Positive

- **Low friction**: Summaries auto-generated, no manual work required
- **High quality**: Developers can fix/improve inaccurate summaries
- **Accountability**: Track whether summary is LLM or human-written
- **Configurable**: Supports multiple LLM providers

### Negative

- **Cost**: LLM API calls during initial indexing (mitigated by using small, cheap models)
- **Latency**: First-time indexing slower due to LLM calls
- **Accuracy**: LLM may hallucinate or misunderstand complex code

### Mitigations

- Use cheaper, faster models (GPT-4o-mini, Claude Haiku) for summaries
- Provide function signature + docstring + context for better accuracy
- Allow re-generation: `regenerate_summary(function_id)`

## Alternatives Considered

### Pure Manual

- High accuracy
- High friction — developers often skip writing summaries
- Defeats purpose of automated code understanding

### Live Queries (No Storage)

- Always fresh, no storage
- Slow — every query requires LLM call
- Expensive — same function queried repeatedly

## Future Enhancements

1. **Summary diff on code change**: Detect if function changed significantly, flag summary for review
2. **Batch regeneration**: Re-generate all summaries with new LLM model
3. **Summary ratings**: Allow users to rate summary quality, use for fine-tuning prompts
