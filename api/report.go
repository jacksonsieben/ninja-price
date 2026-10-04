package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonsieben/ninja-price/alert"
	"github.com/jacksonsieben/ninja-price/config"
	"github.com/jacksonsieben/ninja-price/storage"
)

// handleTracked lists the URLs of every offer on an active product.
//
// The extension fetches this so its content script knows, without asking the
// server on each page load, whether the page it is on is worth reporting.
func (a *API) handleTracked(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	type tracked struct {
		ProductID   string `json:"product_id"`
		ProductName string `json:"product_name"`
		OfferID     string `json:"offer_id"`
		Store       string `json:"store"`
		URL         string `json:"url"`
		Selector    string `json:"selector"`
	}
	out := []tracked{}
	for _, p := range cfg.Items {
		if !p.Active {
			continue
		}
		for _, o := range p.Offers {
			out = append(out, tracked{p.ID, p.Name, o.ID, o.Store, o.URL, o.Selector})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

// handleReport records a price read by the browser extension.
//
// It is the way the stores this process cannot fetch get tracked at all:
// pcdiga.com and pccomponentes.pt answer 403 to every server-side client, so
// their prices can only come from a real browser session. The extension reads
// the page it is already on and posts the figure here, and the same rules that
// govern a scraped price then apply — nothing about the alert decision is
// special-cased for this path.
func (a *API) handleReport(w http.ResponseWriter, r *http.Request) {
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
		URL   string  `json:"url"`
		Price float64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" || body.Price <= 0 {
		http.Error(w, "Required fields: url, price (> 0)", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		http.Error(w, "Error loading config", http.StatusInternalServerError)
		return
	}

	product, offer, ok := findOfferByURL(cfg, body.URL)
	if !ok {
		http.Error(w, "No tracked offer matches that URL", http.StatusNotFound)
		return
	}

	hist, err := storage.LoadHistory(a.historyPath)
	if err != nil {
		http.Error(w, "Error loading history", http.StatusInternalServerError)
		return
	}

	// The best price across the product's other offers, read before recording,
	// is what the new figure has to beat to count as a drop. Offers whose last
	// reading has gone stale are left out, exactly as the hourly check does.
	previousBest, havePrevious := bestKnownPrice(hist, product, "")
	hist.RecordPrice(offer.ID, body.Price)
	bestPrice, _ := bestKnownPrice(hist, product, "")
	if bestPrice <= 0 {
		bestPrice = body.Price
	}

	bestOffer := offer
	for _, o := range product.Offers {
		if it, found := hist.Items[o.ID]; found && it.LastPrice == bestPrice {
			bestOffer = o
			break
		}
	}

	if err := storage.SaveHistory(a.historyPath, hist); err != nil {
		log.Printf("Error saving history after a reported price: %v", err)
	}

	log.Printf("Reported price for %s (%s): %.2f", product.Name, offer.Store, body.Price)

	d := alert.Evaluate(product, cfg.CooldownPeriod, bestPrice, previousBest, havePrevious)
	alert.Raise(a.configPath, cfg, product, d, bestOffer, bestPrice, previousBest, havePrevious)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "recorded",
		"product":    product.Name,
		"offer":      offer.ID,
		"price":      body.Price,
		"best_price": bestPrice,
		"notified":   d.Notify,
	})
}

// findOfferByURL resolves a reported URL to the offer it belongs to. The match
// ignores a trailing slash and any query string, because a browser routinely
// adds tracking parameters the stored URL does not have.
func findOfferByURL(cfg *config.Config, reported string) (config.Product, config.Offer, bool) {
	want := canonicalURL(reported)
	for _, p := range cfg.Items {
		for _, o := range p.Offers {
			if canonicalURL(o.URL) == want {
				return p, o, true
			}
		}
	}
	return config.Product{}, config.Offer{}, false
}

func canonicalURL(raw string) string {
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSuffix(strings.TrimSpace(raw), "/")
}

// bestKnownPrice is the lowest price currently recorded across a product's
// offers, ignoring readings older than alert.StaleAfter. skipOfferID leaves one
// offer out, which is how a reported price is compared against its siblings.
func bestKnownPrice(hist *storage.History, p config.Product, skipOfferID string) (float64, bool) {
	best, have := 0.0, false
	for _, o := range p.Offers {
		if o.ID == skipOfferID {
			continue
		}
		it, found := hist.Items[o.ID]
		if !found || it.LastPrice <= 0 || time.Since(it.LastChecked) >= alert.StaleAfter {
			continue
		}
		if !have || it.LastPrice < best {
			best, have = it.LastPrice, true
		}
	}
	return best, have
}
