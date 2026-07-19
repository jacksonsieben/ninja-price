package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacksonsieben/ninja-price/config"
)

type MatchResult struct {
	IsMatch    bool
	Confidence float64
}

// Provider is implemented by each pluggable LLM backend. Exactly one is
// selected at startup via config.LLMConfig.Provider — never switched
// dynamically per request.
type Provider interface {
	Name() string
	MatchProduct(ctx context.Context, ourTitle, candidateTitle string) (MatchResult, error)
	Discover(ctx context.Context, productName string, storeDomains []string) ([]string, error)
}

// ErrUnsupported is returned by providers that can't perform a given
// operation (e.g. a local model with no web access can't Discover).
var ErrUnsupported = errors.New("llm: operation not supported by this provider")

func NewProvider(cfg config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "claude_cli":
		return NewClaudeCLIProvider(cfg), nil
	case "anthropic":
		return NewAnthropicProvider(cfg), nil
	case "openai_compatible":
		return NewOpenAICompatibleProvider(cfg), nil
	case "":
		return nil, errors.New("llm: no provider configured")
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", cfg.Provider)
	}
}
