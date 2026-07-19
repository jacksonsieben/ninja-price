package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jacksonsieben/ninja-price/config"
)

// openAICompatibleProvider talks plain HTTP to any endpoint that speaks the
// OpenAI-style /chat/completions wire shape (Ollama, LM Studio, vLLM, most
// self-hosted stacks, OpenRouter, etc.). It's the "bring your own model"
// option: verification only, since a generic local model has no web access.
type openAICompatibleProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAICompatibleProvider(cfg config.LLMConfig) *openAICompatibleProvider {
	apiKey := ""
	if cfg.APIKeyEnv != "" {
		apiKey = os.Getenv(cfg.APIKeyEnv)
	}
	return &openAICompatibleProvider{
		baseURL: cfg.BaseURL,
		apiKey:  apiKey,
		model:   cfg.Model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *openAICompatibleProvider) Name() string { return "openai_compatible" }

type chatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
	Response *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (p *openAICompatibleProvider) chat(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(chatCompletionRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: "Reply with a single JSON object only, no prose, no markdown fences."},
			{Role: "user", Content: prompt},
		},
		Response: &responseFormat{Type: "json_object"},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	log.Printf("openai_compatible: POST %s/chat/completions (model %s)", p.baseURL, p.model)

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("openai_compatible: request failed: %v", err)
		return "", fmt.Errorf("openai_compatible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("openai_compatible: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
		return "", fmt.Errorf("openai_compatible: unexpected status %d", resp.StatusCode)
	}

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("openai_compatible: could not parse response: %v", err)
		return "", fmt.Errorf("openai_compatible: could not parse response: %v", err)
	}
	if len(out.Choices) == 0 {
		log.Printf("openai_compatible: empty response from %s", p.baseURL)
		return "", fmt.Errorf("openai_compatible: empty response from %s", p.baseURL)
	}
	return out.Choices[0].Message.Content, nil
}

func (p *openAICompatibleProvider) MatchProduct(ctx context.Context, ourTitle, candidateTitle string) (MatchResult, error) {
	prompt := fmt.Sprintf(
		"Are these two online store listings the same retail product (same model/edition; ignore store-specific title formatting, language, or differently bundled accessories)? A: %q. B: %q. Respond as JSON: {\"is_match\": bool, \"confidence\": number}.",
		ourTitle, candidateTitle,
	)

	text, err := p.chat(ctx, prompt)
	if err != nil {
		return MatchResult{}, err
	}

	var out struct {
		IsMatch    bool    `json:"is_match"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return MatchResult{}, fmt.Errorf("openai_compatible: unexpected match response: %v", err)
	}
	return MatchResult{IsMatch: out.IsMatch, Confidence: out.Confidence}, nil
}

func (p *openAICompatibleProvider) Discover(ctx context.Context, productName string, storeDomains []string) ([]string, error) {
	// A generic local/self-hosted model has no web access by default.
	return nil, ErrUnsupported
}
