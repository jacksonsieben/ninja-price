package config

import (
	"encoding/json"
	"os"
	"fmt"
	"time"
)

type Item struct {
	ID          		string  	`json:"id"`
	Name        		string  	`json:"name"`
	URL         		string  	`json:"url"`
	Selector    		string  	`json:"selector"`
	Category    		string  	`json:"category"`
	TargetPrice 		float64 	`json:"target_price"`
	Active  			bool    	`json:"active"`
	AlertAnyPriceDrop 	bool    	`json:"alert_any_price_drop"`
	Sticky      		bool    	`json:"sticky"`
	LastNotified 		time.Time 	`json:"last_notified,omitempty"`
}

type Config struct {
	CooldownPeriod 		int    		`json:"cooldown_period"` // Cooldown period in minutes
	Items 				[]Item 		`json:"items"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfig := &Config{CooldownPeriod: 5, Items: []Item{}}
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

func UpdateLastNotified(path string, itemID string) error {
	cfg, err := LoadConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	updated := false
	for i := range cfg.Items {
		if cfg.Items[i].ID == itemID {
			cfg.Items[i].LastNotified = time.Now()
			updated = true
			break 
		}
	}

	if !updated {
		return fmt.Errorf("item with id %s not found in config", itemID)
	}

	return SaveConfig(path, cfg)
}