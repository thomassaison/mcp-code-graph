package behavior

import (
	"context"
	"testing"
)

func TestLLMAnalyzer_Analyze(t *testing.T) {
	provider := &mockLLMProvider{
		response: `{"behaviors": ["logging", "error-handle"]}`,
	}

	analyzer := NewLLMAnalyzer(provider)
	req := AnalysisRequest{
		PackageName:  "service",
		FunctionName: "HandleRequest",
		Signature:    "func HandleRequest(req *Request) error",
		Code:         "func HandleRequest(req *Request) error { log.Printf(\"handling %v\", req); return errors.New(\"failed\") }",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(behaviors) != 2 {
		t.Errorf("Analyze() returned %d behaviors, want 2", len(behaviors))
	}

	found := make(map[string]bool)
	for _, b := range behaviors {
		found[b] = true
	}

	if !found[BehaviorLogging] {
		t.Error("Analyze() missing logging behavior")
	}
	if !found[BehaviorErrorHandle] {
		t.Error("Analyze() missing error-handle behavior")
	}
}

type mockLLMProvider struct {
	response string
	err      error
}

func (m *mockLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}
