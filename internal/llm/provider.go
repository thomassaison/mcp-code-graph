package llm

import (
	"context"
	"fmt"
)

// SummaryRequest contains the information needed to generate a function summary.
type SummaryRequest struct {
	FunctionName string
	Signature    string
	Package      string
	Docstring    string
	Code         string
}

// LLMProvider generates function summaries using an LLM.
type LLMProvider interface {
	GenerateSummary(ctx context.Context, req SummaryRequest) (string, error)
	Generate(ctx context.Context, prompt string) (string, error)
}

// Config holds configuration for an LLM provider.
type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
}

// MockProvider is a mock LLM provider for testing.
type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) GenerateSummary(ctx context.Context, req SummaryRequest) (string, error) {
	return fmt.Sprintf("Function %s in package %s", req.FunctionName, req.Package), nil
}

func (m *MockProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return `{"behaviors": []}`, nil
}
