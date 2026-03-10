package summary

import "github.com/thomassaison/mcp-code-graph/internal/llm"

// SummaryRequest contains the information needed to generate a function summary.
// Deprecated: Use llm.SummaryRequest instead.
type SummaryRequest = llm.SummaryRequest

// LLMProvider generates function summaries using an LLM.
// Deprecated: Use llm.LLMProvider instead.
type LLMProvider = llm.LLMProvider

// MockProvider is a mock LLM provider for testing.
// Deprecated: Use llm.MockProvider instead.
type MockProvider = llm.MockProvider
