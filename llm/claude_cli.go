package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/jacksonsieben/ninja-price/config"
)

// claudeCLIProvider shells out to the user's local `claude` CLI in
// non-interactive print mode. It rides whatever auth the CLI already has
// (typically a Pro/Max subscription login), so there's no separate API key
// or per-token billing — usage just counts against the subscription's
// included Claude Code usage.
type claudeCLIProvider struct {
	cliPath string
	model   string
}

func NewClaudeCLIProvider(cfg config.LLMConfig) *claudeCLIProvider {
	model := cfg.Model
	if model == "" {
		model = "haiku"
	}
	cliPath := cfg.CLIPath
	if cliPath == "" {
		cliPath = "claude"
	}
	return &claudeCLIProvider{cliPath: cliPath, model: model}
}

func (p *claudeCLIProvider) Name() string { return "claude_cli" }

// claudeCLIResult mirrors the subset of the `--output-format json` envelope
// we care about (verified live: `claude -p --output-format json ...`).
type claudeCLIResult struct {
	IsError          bool            `json:"is_error"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
}

// run invokes the CLI with a JSON-schema-constrained response, always from a
// neutral working directory so it doesn't pick up this project's own
// CLAUDE.md/context (which would add unrelated cost and behavior).
func (p *claudeCLIProvider) run(ctx context.Context, prompt, jsonSchema string, tools []string) (json.RawMessage, error) {
	// --allowedTools/--tools are variadic flags: they greedily consume every
	// following argument that doesn't start with "--". They must come before
	// other "--flag value" pairs (which bound the consumption) and never
	// immediately before the trailing prompt, or the prompt itself gets
	// swallowed as an extra tool name and the CLI sees no prompt at all.
	args := []string{"-p", "--model", p.model}
	if len(tools) > 0 {
		toolList := strings.Join(tools, ",")
		args = append(args, "--allowedTools", toolList, "--tools", toolList)
	} else {
		args = append(args, "--tools", "")
	}
	args = append(args,
		"--output-format", "json",
		"--json-schema", jsonSchema,
		"--no-session-persistence",
		"--max-budget-usd", "0.20",
		prompt,
	)

	log.Printf("claude_cli: running %s %s", p.cliPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = os.TempDir()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run() // errors are also reported via the JSON envelope's is_error field below
	if runErr != nil {
		log.Printf("claude_cli: command exited with error: %v (stderr: %s)", runErr, stderr.String())
	}
	if stderr.Len() > 0 {
		log.Printf("claude_cli: stderr: %s", stderr.String())
	}

	var result claudeCLIResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		log.Printf("claude_cli: could not parse CLI stdout as JSON: %v (raw stdout: %s)", err, stdout.String())
		return nil, fmt.Errorf("claude_cli: could not parse CLI output: %v (stderr: %s)", err, stderr.String())
	}
	if result.IsError {
		log.Printf("claude_cli: CLI reported an error: %s", result.Result)
		return nil, fmt.Errorf("claude_cli: %s", result.Result)
	}
	log.Printf("claude_cli: got structured_output (%d bytes)", len(result.StructuredOutput))
	return result.StructuredOutput, nil
}

func (p *claudeCLIProvider) MatchProduct(ctx context.Context, ourTitle, candidateTitle string) (MatchResult, error) {
	schema := `{"type":"object","properties":{"is_match":{"type":"boolean"},"confidence":{"type":"number"}},"required":["is_match","confidence"]}`
	prompt := fmt.Sprintf(
		"Are these two online store listings the same retail product (same model/edition; ignore store-specific title formatting, language, or differently bundled accessories)? A: %q. B: %q. Respond as JSON.",
		ourTitle, candidateTitle,
	)

	raw, err := p.run(ctx, prompt, schema, nil)
	if err != nil {
		return MatchResult{}, err
	}

	var out struct {
		IsMatch    bool    `json:"is_match"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return MatchResult{}, fmt.Errorf("claude_cli: unexpected match response: %v", err)
	}
	return MatchResult{IsMatch: out.IsMatch, Confidence: out.Confidence}, nil
}

func (p *claudeCLIProvider) Discover(ctx context.Context, productName string, storeDomains []string) ([]string, error) {
	schema := `{"type":"object","properties":{"urls":{"type":"array","items":{"type":"string"}}},"required":["urls"]}`
	prompt := fmt.Sprintf(
		"Search the web for the retail product %q, restricted to these store domains: %s. For each domain where you find a matching product page, include the single best-matching product URL. Only include URLs on the listed domains; omit domains where nothing matches. Respond as JSON.",
		productName, strings.Join(storeDomains, ", "),
	)

	raw, err := p.run(ctx, prompt, schema, []string{"WebSearch"})
	if err != nil {
		return nil, err
	}

	var out struct {
		URLs []string `json:"urls"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("claude_cli: unexpected discover response: %v", err)
	}
	return out.URLs, nil
}
