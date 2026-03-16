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

func (p *OpenAIProvider) doChatCompletion(ctx context.Context, messages []openAIMessage) (string, error) {
	body := openAIRequest{
		Model:    p.model,
		Messages: messages,
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
	defer func() { _ = resp.Body.Close() }()

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

func (p *OpenAIProvider) GenerateSummary(ctx context.Context, req SummaryRequest) (string, error) {
	prompt := buildSummaryPrompt(req)
	messages := []openAIMessage{
		{Role: "system", Content: `You are a senior software engineer performing static code analysis.
Your task is to summarize a single function in a structured and precise way so it can be indexed for semantic code search.
Rules:
- Be factual and concise.
- Do not repeat the code.
- Infer intent from the implementation.
- Identify side effects and external dependencies.
- Prefer short technical wording.
- Do not invent information that cannot be inferred from the code.
Return ONLY valid JSON matching the schema.`},
		{Role: "user", Content: prompt},
	}
	return p.doChatCompletion(ctx, messages)
}

func (p *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	messages := []openAIMessage{{Role: "user", Content: prompt}}
	return p.doChatCompletion(ctx, messages)
}

func buildSummaryPrompt(req SummaryRequest) string {
	var sb strings.Builder

	lang := req.Language
	if lang == "" {
		lang = "Go"
	}

	code := req.Code
	if code == "" {
		var fb strings.Builder
		if req.Docstring != "" {
			fb.WriteString("// ")
			fb.WriteString(req.Docstring)
			fb.WriteString("\n")
		}
		fb.WriteString(req.Signature)
		code = fb.String()
	}

	sb.WriteString("Analyze the following function and produce a structured summary.\n\n")
	fmt.Fprintf(&sb, "Language: %s\n", lang)
	if req.File != "" {
		fmt.Fprintf(&sb, "File: %s\n", req.File)
	}
	fmt.Fprintf(&sb, "\nFunction code:\n```%s\n%s\n```\n", lang, code)
	sb.WriteString(`
Return JSON with this schema:

{
  "name": "function name",
  "purpose": "one sentence describing what the function does",
  "inputs": [
    {
      "name": "parameter name",
      "type": "type if known",
      "description": "short description"
    }
  ],
  "outputs": {
    "type": "return type if known",
    "description": "what the function returns"
  },
  "side_effects": [
    "writes to database",
    "network call",
    "filesystem access",
    "mutates global state"
  ],
  "dependencies": [
    "external function or API used"
  ],
  "error_handling": "how errors are handled if visible",
  "algorithm": "short description of the algorithm or logic used",
  "keywords": [
    "authentication",
    "hashing",
    "caching"
  ]
}`)
	return sb.String()
}
