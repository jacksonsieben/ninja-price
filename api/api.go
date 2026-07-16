package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/jacksonsieben/ninja-price/config"
	"github.com/jacksonsieben/ninja-price/scraper"
)

type API struct {
	configPath  string
	historyPath string
}

func NewAPI(configPath, historyPath string) *API {
	return &API{
		configPath:  configPath,
		historyPath: historyPath,
	}
}

func (a *API) Start(port int) {
	http.HandleFunc("/", a.handleDashboard)
	http.HandleFunc("/history", a.handleHistory)
	http.HandleFunc("/items", a.handleItems)
	http.HandleFunc("/config", a.handleConfig)
	http.HandleFunc("/detect", a.handleDetect)

	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting API server on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("API server failed: %v", err)
	}
}

func (a *API) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    http.ServeFile(w, r, "dashboard.html")
}

func (a *API) handleConfig(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for bookmarklet usage
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, a.configPath)
}

func (a *API) handleHistory(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for bookmarklet usage
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, a.historyPath)
}

// handleDetect tries to auto-detect a product's price and name from its URL,
// so the extension can prefill the "add item" form without requiring the
// user to click-to-pick a CSS selector.
func (a *API) handleDetect(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for extension usage
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageURL := r.URL.Query().Get("url")
	if pageURL == "" {
		http.Error(w, "Missing url", http.StatusBadRequest)
		return
	}

	price, source, name, err := scraper.DetectPrice(pageURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"price":  price,
		"source": source,
		"name":   name,
	})
}

// handleItems atua apenas como um "Router" (Roteador), direcionando para a função correta
func (a *API) handleItems(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for bookmarklet usage
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// Atualizado para permitir PUT e DELETE também
	w.Header().Set("Access-Control-Allow-Methods", "POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	switch r.Method {
	case http.MethodOptions:
		a.handleItemsOptions(w, r)
	case http.MethodDelete:
		a.handleItemsDelete(w, r)
	case http.MethodPut: // Alterado para PUT
		a.handleItemsPut(w, r)
	case http.MethodPost:
		a.handleItemsPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---------------------------------------------------------
// FUNÇÕES DE CADA MÉTODO HTTP
// ---------------------------------------------------------

func (a *API) handleItemsOptions(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (a *API) handleItemsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	var newItems []config.Item
	for _, item := range cfg.Items {
		if item.ID != id {
			newItems = append(newItems, item)
		}
	}
	cfg.Items = newItems

	if err := config.SaveConfig(a.configPath, cfg); err != nil {
		http.Error(w, "Error saving config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) handleItemsPut(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	var update struct {
		TargetPrice       float64 `json:"target_price"`
		Name              string  `json:"name"`
		Active            bool    `json:"active"`
		AlertAnyPriceDrop bool    `json:"alert_any_price_drop"`
		Sticky            bool    `json:"sticky"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	found := false
	for i, item := range cfg.Items {
		if item.ID == id {
			cfg.Items[i].TargetPrice = update.TargetPrice
			cfg.Items[i].Name = update.Name
			cfg.Items[i].Active = update.Active
			cfg.Items[i].AlertAnyPriceDrop = update.AlertAnyPriceDrop
			cfg.Items[i].Sticky = update.Sticky
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	if err := config.SaveConfig(a.configPath, cfg); err != nil {
		http.Error(w, "Error saving config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) handleItemsPost(w http.ResponseWriter, r *http.Request) {
	var newItem config.Item
	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if newItem.ID == "" || newItem.URL == "" {
		http.Error(w, "Missing required fields (id, url)", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	// Check if item already exists
	for _, item := range cfg.Items {
		if item.ID == newItem.ID {
			http.Error(w, "Item with this ID already exists", http.StatusConflict)
			return
		}
	}

	cfg.Items = append(cfg.Items, newItem)

	if err := config.SaveConfig(a.configPath, cfg); err != nil {
		http.Error(w, "Error saving config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "Item added successfully"})
}
