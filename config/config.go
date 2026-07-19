package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Offer struct {
	ID       string `json:"id"`
	Store    string `json:"store"`
	URL      string `json:"url"`
	Selector string `json:"selector"` // optional, "" = auto-detect
}

type Product struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	TargetPrice       float64   `json:"target_price"`
	Active            bool      `json:"active"`
	AlertAnyPriceDrop bool      `json:"alert_any_price_drop"`
	Sticky            bool      `json:"sticky"`
	NotifyEmail       bool      `json:"notify_email"`
	LastNotified      time.Time `json:"last_notified,omitempty"`
	Offers            []Offer   `json:"offers"`
}

// SMTPConfig holds optional email delivery settings. When nil (the "smtp"
// key absent from config.json), email notifications are silently skipped —
// desktop notifications keep working regardless.
type SMTPConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordEnv string `json:"password_env"` // env var holding the SMTP password/app-password, kept out of config.json
	From        string `json:"from"`
	To          string `json:"to"`
}

type LLMConfig struct {
	Provider       string   `json:"provider"` // "claude_cli" | "anthropic" | "openai_compatible" | ""
	Model          string   `json:"model"`
	CLIPath        string   `json:"cli_path"`
	APIKeyEnv      string   `json:"api_key_env"`
	BaseURL        string   `json:"base_url"`
	DiscoverStores []string `json:"discover_stores"`
}

type Config struct {
	CooldownPeriod int         `json:"cooldown_period"` // Cooldown period in minutes
	Items          []Product   `json:"items"`
	LLM            *LLMConfig  `json:"llm,omitempty"`
	SMTP           *SMTPConfig `json:"smtp,omitempty"`
}

func defaultLLMConfig() *LLMConfig {
	return &LLMConfig{
		Provider:       "claude_cli",
		Model:          "haiku",
		CLIPath:        "claude",
		APIKeyEnv:      "ANTHROPIC_API_KEY",
		DiscoverStores: []string{"amazon.es", "worten.pt", "fnac.pt", "pcdiga.com"},
	}
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfig := &Config{CooldownPeriod: 5, Items: []Product{}, LLM: defaultLLMConfig()}
			SaveConfig(path, defaultConfig)
			return defaultConfig, nil
		}
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// If the "llm" key was entirely absent, apply full defaults. If present
	// (even with provider:"" to explicitly disable the feature), respect it
	// and only backfill auxiliary fields that were left blank.
	if cfg.LLM == nil {
		cfg.LLM = defaultLLMConfig()
	} else {
		if cfg.LLM.CLIPath == "" {
			cfg.LLM.CLIPath = "claude"
		}
		if cfg.LLM.APIKeyEnv == "" {
			cfg.LLM.APIKeyEnv = "ANTHROPIC_API_KEY"
		}
		if len(cfg.LLM.DiscoverStores) == 0 {
			cfg.LLM.DiscoverStores = defaultLLMConfig().DiscoverStores
		}
	}

	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func UpdateLastNotified(path string, productID string) error {
	cfg, err := LoadConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	updated := false
	for i := range cfg.Items {
		if cfg.Items[i].ID == productID {
			cfg.Items[i].LastNotified = time.Now()
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("product with id %s not found in config", productID)
	}

	return SaveConfig(path, cfg)
}
