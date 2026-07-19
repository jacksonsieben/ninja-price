package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jacksonsieben/ninja-price/config"
)

// anthropicProvider calls the Anthropic API directly via the official Go SDK.
// Unlike claude_cli, this is pay-per-token, billed separately from any
// claude.ai subscription, and requires an API key.
type anthropicProvider struct {
	client anthropic.Client
	model  string
}

func NewAnthropicProvider(cfg config.LLMConfig) *anthropicProvider {
	model := cfg.Model
	if model == "" || model == "haiku" {
		model = "claude-haiku-4-5"
	}

	keyEnv := cfg.APIKeyEnv
	if keyEnv == "" {
		keyEnv = "ANTHROPIC_API_KEY"
	}

	return &anthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(os.Getenv(keyEnv))),
		model:  model,
	}
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) MatchProduct(ctx context.Context, ourTitle, candidateTitle string) (MatchResult, error) {
	prompt := fmt.Sprintf(
		"Are these two online store listings the same retail product (same model/edition; ignore store-specific title formatting, language, or differently bundled accessories)? A: %q. B: %q.",
		ourTitle, candidateTitle,
	)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"is_match":   map[string]interface{}{"type": "boolean"},
			"confidence": map[string]interface{}{"type": "number"},
		},
		"required":             []string{"is_match", "confidence"},
		"additionalProperties": false,
	}

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 256,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	})
	if err != nil {
		log.Printf("anthropic: MatchProduct request failed: %v", err)
		return MatchResult{}, fmt.Errorf("anthropic: %w", err)
	}

	var out struct {
		IsMatch    bool    `json:"is_match"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(msg.Content[0].Text), &out); err != nil {
		log.Printf("anthropic: unexpected match response: %v (raw: %s)", err, msg.Content[0].Text)
		return MatchResult{}, fmt.Errorf("anthropic: unexpected match response: %v", err)
	}
	return MatchResult{IsMatch: out.IsMatch, Confidence: out.Confidence}, nil
}

func (p *anthropicProvider) Discover(ctx context.Context, productName string, storeDomains []string) ([]string, error) {
	prompt := fmt.Sprintf(
		"Search the web for the retail product %q, restricted to these store domains: %s. For each domain where you find a matching product page, note the single best-matching product URL. Only include URLs on the listed domains; omit domains where nothing matches.",
		productName, strings.Join(storeDomains, ", "),
	)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"urls": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
		"required":             []string{"urls"},
		"additionalProperties": false,
	}

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Tools: []anthropic.ToolUnionParam{
			{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{
				AllowedDomains: storeDomains,
			}},
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	})
	if err != nil {
		log.Printf("anthropic: Discover request failed for %q: %v", productName, err)
		return nil, fmt.Errorf("anthropic: %w", err)
	}

	var lastText string
	for _, block := range msg.Content {
		if block.Type == "text" {
			lastText = block.Text
		}
	}

	var out struct {
		URLs []string `json:"urls"`
	}
	if err := json.Unmarshal([]byte(lastText), &out); err != nil {
		log.Printf("anthropic: unexpected discover response: %v (raw: %s)", err, lastText)
		return nil, fmt.Errorf("anthropic: unexpected discover response: %v", err)
	}
	log.Printf("anthropic: Discover found %d URL(s) for %q", len(out.URLs), productName)
	return out.URLs, nil
}
