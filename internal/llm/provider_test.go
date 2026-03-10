package llm

import (
	"context"
	"testing"
)

func TestMockProvider(t *testing.T) {
	p := &MockProvider{}
	req := SummaryRequest{
		FunctionName: "TestFunc",
		Package:      "testpkg",
	}

	summary, err := p.GenerateSummary(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Function TestFunc in package testpkg"
	if summary != expected {
		t.Errorf("expected %q, got %q", expected, summary)
	}
}
