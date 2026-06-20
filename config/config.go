package config

import (
	"encoding/json"
	"os"
)

type Item struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Selector    string  `json:"selector"`
	Category    string  `json:"category"`
	TargetPrice float64 `json:"target_price"`
}

type Config struct {
	Items []Item `json:"items"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfig := &Config{Items: []Item{}}
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
