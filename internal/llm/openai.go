package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIProvider implements LLMProvider using OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI-compatible LLM provider.
func NewOpenAIProvider(cfg *Config) (*OpenAIProvider, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required for OpenAI provider")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &OpenAIProvider{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *OpenAIProvider) GenerateSummary(ctx context.Context, req SummaryRequest) (string, error) {
	prompt := buildSummaryPrompt(req)

	body := openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "system", Content: `You are a Go code documentation expert. Generate a single-line summary that captures:
1. The function's primary purpose (start with a strong action verb)
2. Key behavior (error handling, side effects, concurrency)
3. Pattern tag if applicable: [Constructor], [Middleware], [HTTP Handler], [Factory], [Interface impl]

Format: [Tag] <action verb> <purpose>. <behavior notes>.

Examples:
- [Constructor] Creates a new Server instance with configured timeouts. Thread-safe.
- [HTTP Handler] Dispatches incoming requests to appropriate handlers. Returns 400 on invalid input.
- Parses JSON configuration into a Config struct. Returns error on malformed input.

Respond with ONLY the summary, no additional text.`},
			{Role: "user", Content: prompt},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func buildSummaryPrompt(req SummaryRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate a one-line summary for this Go function:\n\n")
	sb.WriteString(fmt.Sprintf("Package: %s\n", req.Package))
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

	sb.WriteString("\nIdentify: pattern (constructor/middleware/handler/factory), error behavior, side effects, concurrency usage, and any interfaces implemented.\n")

	return sb.String()
}
