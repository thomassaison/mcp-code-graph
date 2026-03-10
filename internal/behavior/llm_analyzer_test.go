package behavior

import (
	"context"
	"errors"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/llm"
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

func TestLLMAnalyzer_Analyze_ProviderError(t *testing.T) {
	provider := &mockLLMProvider{err: errors.New("connection failed")}
	analyzer := NewLLMAnalyzer(provider)

	_, err := analyzer.Analyze(context.Background(), AnalysisRequest{})
	if err == nil {
		t.Error("Analyze() should return error when provider fails")
	}
}

func TestLLMAnalyzer_Analyze_InvalidJSON(t *testing.T) {
	provider := &mockLLMProvider{response: "not json"}
	analyzer := NewLLMAnalyzer(provider)

	_, err := analyzer.Analyze(context.Background(), AnalysisRequest{})
	if err == nil {
		t.Error("Analyze() should return error for invalid JSON")
	}
}

func TestLLMAnalyzer_Analyze_EmptyBehaviors(t *testing.T) {
	provider := &mockLLMProvider{response: `{"behaviors": []}`}
	analyzer := NewLLMAnalyzer(provider)

	behaviors, err := analyzer.Analyze(context.Background(), AnalysisRequest{})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(behaviors) != 0 {
		t.Errorf("Analyze() returned %d behaviors, want 0", len(behaviors))
	}
}

func TestLLMAnalyzer_Analyze_MarkdownResponse(t *testing.T) {
	provider := &mockLLMProvider{
		response: "```json\n{\"behaviors\": [\"logging\"]}\n```",
	}
	analyzer := NewLLMAnalyzer(provider)

	behaviors, err := analyzer.Analyze(context.Background(), AnalysisRequest{})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(behaviors) != 1 || behaviors[0] != BehaviorLogging {
		t.Errorf("Analyze() behaviors = %v, want [logging]", behaviors)
	}
}

func TestLLMAnalyzer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	provider := llm.NewMockProvider()
	analyzer := NewLLMAnalyzer(NewLLMProviderAdapter(provider))

	req := AnalysisRequest{
		PackageName:  "test",
		FunctionName: "TestFunc",
		Code:         "func TestFunc() { log.Println(\"test\") }",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	t.Logf("Behaviors: %v", behaviors)
}
