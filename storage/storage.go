package storage

import (
	"encoding/json"
	"os"
	"time"
)

type PricePoint struct {
	Price float64 `json:"price"`
	Date  string  `json:"date"`
}

type HistoryItem struct {
	LastPrice   float64   `json:"last_price"`
	LowestPrice float64   `json:"lowest_price"`
	LastChecked time.Time `json:"last_checked"`
	History     []PricePoint `json:"history"`
}

type History struct {
	Items map[string]*HistoryItem `json:"items"`
}

func LoadHistory(path string) (*History, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &History{Items: make(map[string]*HistoryItem)}, nil
		}
		return nil, err
	}
	defer file.Close()

	var hist History
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&hist); err != nil {
		return nil, err
	}

	if hist.Items == nil {
		hist.Items = make(map[string]*HistoryItem)
	}

	return &hist, nil
}

func SaveHistory(path string, hist *History) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(hist)
}
