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

// maxHistoryPoints bounds how many samples we keep per offer so the JSON file
// can't grow without limit if a price flaps every check. On-change + daily
// recording means this is roughly "N days of history" for a stable price, and
// still comfortably covers years.
const maxHistoryPoints = 3000

// RecordPrice upserts a fresh price reading for the given offer ID: it always
// refreshes LastPrice/LowestPrice/LastChecked, but only appends a new history
// point when that point carries new information — the price changed since the
// last sample, or a day has passed (a "heartbeat" so a long-flat price still
// stretches across the chart). Previously every hourly check appended an
// identical point and the series was capped at 14, so history was really just
// "the last 14 checks" and range filters (1M/3M/1Y) had nothing to work with.
// Shared by the periodic checker and the "record a price the moment an offer is
// created" path, so both stay in sync.
func (h *History) RecordPrice(offerID string, price float64) *HistoryItem {
	now := time.Now()

	item, exists := h.Items[offerID]
	if !exists {
		item = &HistoryItem{LowestPrice: price}
		h.Items[offerID] = item
	}

	item.LastPrice = price
	if item.LowestPrice == 0 || price < item.LowestPrice {
		item.LowestPrice = price
	}
	item.LastChecked = now

	appendPoint := true
	if n := len(item.History); n > 0 {
		last := item.History[n-1]
		sameDay := last.Date.Year() == now.Year() && last.Date.YearDay() == now.YearDay()
		if last.Price == price && sameDay {
			appendPoint = false // same price, already recorded today
		}
	}
	if appendPoint {
		item.History = append(item.History, PricePoint{Price: price, Date: now})
		if len(item.History) > maxHistoryPoints {
			item.History = item.History[len(item.History)-maxHistoryPoints:]
		}
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
