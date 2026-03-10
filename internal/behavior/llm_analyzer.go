package behavior

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type LLMProvider interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type LLMAnalyzer struct {
	provider LLMProvider
}

func NewLLMAnalyzer(provider LLMProvider) *LLMAnalyzer {
	return &LLMAnalyzer{provider: provider}
}

func (a *LLMAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) ([]string, error) {
	prompt := a.buildPrompt(req)

	response, err := a.provider.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generate: %w", err)
	}

	return a.parseResponse(response)
}

func (a *LLMAnalyzer) buildPrompt(req AnalysisRequest) string {
	var sb strings.Builder

	sb.WriteString("Analyze this Go function and identify which behaviors it exhibits.\n\n")
	sb.WriteString("Available behaviors:\n")
	for _, b := range AllBehaviors() {
		sb.WriteString(fmt.Sprintf("- %s\n", b))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Package: %s\n", req.PackageName))
	sb.WriteString(fmt.Sprintf("Function: %s\n", req.FunctionName))

	if req.Signature != "" {
		sb.WriteString(fmt.Sprintf("Signature: %s\n", req.Signature))
	}
	if req.Docstring != "" {
		sb.WriteString(fmt.Sprintf("Documentation: %s\n", req.Docstring))
	}
	if req.Code != "" {
		sb.WriteString(fmt.Sprintf("Code:\n%s\n", req.Code))
	}

	sb.WriteString("\nRespond with ONLY a JSON object: {\"behaviors\": [\"behavior1\", \"behavior2\"]}\n")
	sb.WriteString("Include only behaviors that are clearly present. If none match, return empty array.\n")

	return sb.String()
}

func (a *LLMAnalyzer) parseResponse(response string) ([]string, error) {
	response = strings.TrimSpace(response)

	var result struct {
		Behaviors []string `json:"behaviors"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var valid []string
	for _, b := range result.Behaviors {
		if IsValidBehavior(b) {
			valid = append(valid, b)
		}
	}

	return valid, nil
}
