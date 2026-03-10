package behavior

import (
	"context"

	"github.com/thomassaison/mcp-code-graph/internal/llm"
)

type LLMProviderAdapter struct {
	provider llm.LLMProvider
}

func NewLLMProviderAdapter(provider llm.LLMProvider) *LLMProviderAdapter {
	return &LLMProviderAdapter{provider: provider}
}

func (a *LLMProviderAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.provider.Generate(ctx, prompt)
}
