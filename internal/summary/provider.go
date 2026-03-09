package summary

import (
	"context"
	"fmt"
)

type SummaryRequest struct {
	FunctionName string
	Signature    string
	Package      string
	Docstring    string
	Code         string
}

type LLMProvider interface {
	GenerateSummary(ctx context.Context, req SummaryRequest) (string, error)
}

type MockProvider struct{}

func (m *MockProvider) GenerateSummary(ctx context.Context, req SummaryRequest) (string, error) {
	return fmt.Sprintf("Function %s in package %s", req.FunctionName, req.Package), nil
}
