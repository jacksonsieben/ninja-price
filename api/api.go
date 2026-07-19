package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jacksonsieben/ninja-price/config"
	"github.com/jacksonsieben/ninja-price/llm"
	"github.com/jacksonsieben/ninja-price/scraper"
	"github.com/jacksonsieben/ninja-price/storage"
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
	http.HandleFunc("/discover.html", a.handleDiscoverPage)
	http.HandleFunc("/history", a.handleHistory)
	http.HandleFunc("/items", a.handleItems)
	http.HandleFunc("/config", a.handleConfig)
	http.HandleFunc("/detect", a.handleDetect)
	http.HandleFunc("/discover", a.handleDiscover)
	http.HandleFunc("/products/{id}/offers", a.handleProductOffers)

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

func (a *API) handleDiscoverPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "discover.html")
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

	var newItems []config.Product
	for _, product := range cfg.Items {
		if product.ID != id {
			newItems = append(newItems, product)
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
		NotifyEmail       bool    `json:"notify_email"`
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
	for i, product := range cfg.Items {
		if product.ID == id {
			cfg.Items[i].TargetPrice = update.TargetPrice
			cfg.Items[i].Name = update.Name
			cfg.Items[i].Active = update.Active
			cfg.Items[i].AlertAnyPriceDrop = update.AlertAnyPriceDrop
			cfg.Items[i].Sticky = update.Sticky
			cfg.Items[i].NotifyEmail = update.NotifyEmail
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	if err := config.SaveConfig(a.configPath, cfg); err != nil {
		http.Error(w, "Error saving config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) handleItemsPost(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		URL               string  `json:"url"`
		Selector          string  `json:"selector"`
		Category          string  `json:"category"`
		TargetPrice       float64 `json:"target_price"`
		Active            bool    `json:"active"`
		AlertAnyPriceDrop bool    `json:"alert_any_price_drop"`
		Sticky            bool    `json:"sticky"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.ID == "" || payload.URL == "" {
		http.Error(w, "Missing required fields (id, url)", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	for _, product := range cfg.Items {
		if product.ID == payload.ID {
			http.Error(w, "Product with this ID already exists", http.StatusConflict)
			return
		}
	}

	product := config.Product{
		ID:                payload.ID,
		Name:              payload.Name,
		Category:          payload.Category,
		TargetPrice:       payload.TargetPrice,
		Active:            payload.Active,
		AlertAnyPriceDrop: payload.AlertAnyPriceDrop,
		Sticky:            payload.Sticky,
		Offers: []config.Offer{
			{
				ID:       payload.ID,
				Store:    storeNameFromURL(payload.URL),
				URL:      payload.URL,
				Selector: payload.Selector,
			},
		},
	}
	cfg.Items = append(cfg.Items, product)

	if err := config.SaveConfig(a.configPath, cfg); err != nil {
		http.Error(w, "Error saving config", http.StatusInternalServerError)
		return
	}

	a.recordInitialPrice(product.Offers[0])

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "Product added successfully"})
}

// handleProductOffers attaches another store's offer to an existing product.
// Used both by the extension's "associate with existing product" action and
// by accepted /discover suggestions.
func (a *API) handleProductOffers(w http.ResponseWriter, r *http.Request) {
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

	productID := r.PathValue("id")
	if productID == "" {
		http.Error(w, "Missing product id", http.StatusBadRequest)
		return
	}

	var body struct {
		Store    string `json:"store"`
		URL      string `json:"url"`
		Selector string `json:"selector"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		http.Error(w, "Missing required field: url", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	found := false
	var newOffer config.Offer
	for i := range cfg.Items {
		if cfg.Items[i].ID != productID {
			continue
		}
		found = true
		store := body.Store
		if store == "" {
			store = storeNameFromURL(body.URL)
		}
		newOffer = config.Offer{
			ID:       fmt.Sprintf("%s-%s-%d", productID, slugify(store), time.Now().UnixNano()%10000),
			Store:    store,
			URL:      body.URL,
			Selector: body.Selector,
		}
		cfg.Items[i].Offers = append(cfg.Items[i].Offers, newOffer)
		break
	}

	if !found {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	if err := config.SaveConfig(a.configPath, cfg); err != nil {
		http.Error(w, "Error saving config", http.StatusInternalServerError)
		return
	}

	a.recordInitialPrice(newOffer)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "Offer added successfully"})
}

// recordInitialPrice scrapes a newly created offer once, immediately, so the
// dashboard shows a price right away instead of waiting for the next
// scheduled check (up to an hour later). A scrape failure here is logged but
// doesn't fail the request — the offer is still saved either way, and the
// price will simply be picked up on the next periodic check.
func (a *API) recordInitialPrice(offer config.Offer) {
	price, source, err := scraper.ScrapePrice(offer.URL, offer.Selector)
	if err != nil {
		log.Printf("Initial price check failed for %s (%s): %v", offer.Store, offer.URL, err)
		return
	}

	hist, err := storage.LoadHistory(a.historyPath)
	if err != nil {
		log.Printf("Error loading history for initial price check: %v", err)
		return
	}
	hist.RecordPrice(offer.ID, price)
	if err := storage.SaveHistory(a.historyPath, hist); err != nil {
		log.Printf("Error saving history for initial price check: %v", err)
		return
	}
	log.Printf("Initial price recorded for %s (%s) via %s: %.2f", offer.Store, offer.URL, source, price)
}

type discoverCandidate struct {
	Store      string  `json:"store"`
	URL        string  `json:"url"`
	Price      float64 `json:"price"`
	Confidence float64 `json:"confidence"`
	Matched    bool    `json:"matched"`
}

// handleDiscover asks the configured LLM provider to find the product on
// other stores, then verifies each candidate through the existing scraper
// pipeline (price + page title) before ever reporting it back. It never
// saves anything itself — the caller reviews and accepts via
// POST /products/{id}/offers.
func (a *API) handleDiscover(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		ProductID   string `json:"product_id"`
		ProductName string `json:"product_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if body.ProductID == "" && body.ProductName == "" {
		http.Error(w, "Missing product_id or product_name", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	if cfg.LLM == nil || cfg.LLM.Provider == "" {
		http.Error(w, "LLM-assisted discovery is disabled", http.StatusNotFound)
		return
	}

	productName := body.ProductName
	existingHosts := map[string]bool{}
	if body.ProductID != "" {
		var product *config.Product
		for i := range cfg.Items {
			if cfg.Items[i].ID == body.ProductID {
				product = &cfg.Items[i]
				break
			}
		}
		if product == nil {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		productName = product.Name
		for _, o := range product.Offers {
			existingHosts[hostOf(o.URL)] = true
		}
	}

	provider, err := llm.NewProvider(*cfg.LLM)
	if err != nil {
		log.Printf("discover: error initializing LLM provider: %v", err)
		http.Error(w, "Error initializing LLM provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	log.Printf("discover: searching for %q via %s across stores %v", productName, provider.Name(), cfg.LLM.DiscoverStores)

	urls, err := provider.Discover(ctx, productName, cfg.LLM.DiscoverStores)
	if err != nil {
		log.Printf("discover: provider.Discover failed for %q: %v", productName, err)
		http.Error(w, "Discovery failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	log.Printf("discover: provider returned %d candidate URL(s) for %q: %v", len(urls), productName, urls)

	var candidates []discoverCandidate
	for _, candidateURL := range urls {
		host := hostOf(candidateURL)
		if existingHosts[host] {
			log.Printf("discover: skipping %s: already tracked for this product", candidateURL)
			continue
		}

		price, source, name, err := scraper.DetectPrice(candidateURL)
		if err != nil {
			log.Printf("discover: skipping %s: could not detect price: %v", candidateURL, err)
			continue
		}
		log.Printf("discover: detected %s (%q) via %s: %.2f", candidateURL, name, source, price)

		match, err := provider.MatchProduct(ctx, productName, name)
		if err != nil {
			log.Printf("discover: skipping %s: MatchProduct failed: %v", candidateURL, err)
			continue
		}
		log.Printf("discover: matched %s against %q: is_match=%v confidence=%.2f", candidateURL, productName, match.IsMatch, match.Confidence)

		candidates = append(candidates, discoverCandidate{
			Store:      host,
			URL:        candidateURL,
			Price:      price,
			Confidence: match.Confidence,
			Matched:    match.IsMatch,
		})
	}

	log.Printf("discover: returning %d candidate(s) for %q", len(candidates), productName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"product_name": productName,
		"candidates":   candidates,
	})
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
}

func storeNameFromURL(rawURL string) string {
	if host := hostOf(rawURL); host != "" {
		return host
	}
	return "Loja"
}
