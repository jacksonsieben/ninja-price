package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/jacksonsieben/ninja-price/config"
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

func (a *API) handleItems(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for bookmarklet usage
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var newItem config.Item
	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if newItem.ID == "" || newItem.URL == "" || newItem.Selector == "" {
		http.Error(w, "Missing required fields (id, url, selector)", http.StatusBadRequest)
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
