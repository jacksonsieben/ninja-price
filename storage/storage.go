package storage

import (
	"encoding/json"
	"os"
	"time"
)

type PricePoint struct {
	Price float64 `json:"price"`
	Date  time.Time  `json:"date"`
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

// RecordPrice upserts a fresh price reading for the given offer ID: updates
// LastPrice/LowestPrice/LastChecked and appends to the (capped) history
// series. Shared by the periodic checker and the "record a price the moment
// an offer is created" path, so both stay in sync.
func (h *History) RecordPrice(offerID string, price float64) *HistoryItem {
	item, exists := h.Items[offerID]
	if !exists {
		item = &HistoryItem{LowestPrice: price}
		h.Items[offerID] = item
	}

	item.LastPrice = price
	if item.LowestPrice == 0 || price < item.LowestPrice {
		item.LowestPrice = price
	}
	item.LastChecked = time.Now()
	item.History = append(item.History, PricePoint{Price: price, Date: time.Now()})
	if len(item.History) > 14 {
		item.History = item.History[1:]
	}

	return item
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
