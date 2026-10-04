package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/jacksonsieben/ninja-price/alert"
	"github.com/jacksonsieben/ninja-price/api"
	"github.com/jacksonsieben/ninja-price/config"
	"github.com/jacksonsieben/ninja-price/scraper"
	"github.com/jacksonsieben/ninja-price/storage"
)

const (
	configPath    	= "config.json"
	historyPath   	= "prices_history.json"
	checkInterval 	= 1 * time.Hour // Default background check interval
	apiPort	   		= 65452
	systrayIconPath = "assets/ninja-price-logo-systray.png"
)

func main() {
	log.Println("NinjaPrice starting...")

	// Resolve all relative paths (config.json, prices_history.json,
	// dashboard.html, assets/) against the binary's own directory instead of
	// the process working directory. Launchers like the .desktop autostart
	// entry don't set a reliable CWD, so previously NinjaPrice would read/write
	// a second, empty config.json in $HOME. Products added through the
	// extension then landed in that stray file and never showed up in the
	// dashboard or the extension's product list.
	if exe, err := os.Executable(); err != nil {
		log.Printf("Warning: could not determine executable path, using current working directory: %v", err)
	} else if dir := filepath.Dir(exe); dir != "" {
		if err := os.Chdir(dir); err != nil {
			log.Printf("Warning: could not chdir to executable dir %s: %v", dir, err)
		} else {
			log.Printf("Working directory set to %s", dir)
		}
	}

	// Start local API in background
	go func() {
		localAPI := api.NewAPI(configPath, historyPath)
		localAPI.Start(apiPort)
	}()

	// Start systray
	systray.Run(onReady, onExit)
}

func onReady() {
	iconBytes, err := os.ReadFile(systrayIconPath)
	if err != nil {
		log.Printf("Could not load systray icon from %s: %v", systrayIconPath, err)
	} else {
		// COSMIC's status-area applet needs a themed icon on disk (see
		// writeCosmicThemedIcon). Every other tray host tested — quickshell
		// under Hyprland, GNOME's AppIndicator extension, Waybar — renders
		// the pixmap set below, so the workaround is scoped to COSMIC rather
		// than writing into the user's icon theme on every desktop.
		if runningCosmic() {
			writeCosmicThemedIcon(iconBytes)
		}
		systray.SetIcon(iconBytes)
	}

	systray.SetTitle("NP")
	systray.SetTooltip("NinjaPrice Tracker")

	mCheckNow := systray.AddMenuItem("Check Now", "Check prices immediately")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit NinjaPrice")

	// Start periodic checker
	ticket := time.NewTicker(checkInterval)

	// SIGUSR1 is sent by the resume-watcher (see README) right after the
	// system wakes from suspend, since the ticker above stalls during sleep
	// (its monotonic clock doesn't advance while suspended) and would
	// otherwise leave a long gap before the next check.
	resumeCh := make(chan os.Signal, 1)
	signal.Notify(resumeCh, syscall.SIGUSR1)

	go func() {
		// Run an initial check
		checkPrices()

		for {
			select {
			case <-mCheckNow.ClickedCh:
				log.Println("Manual check triggered.")
				checkPrices()
			case <-ticket.C:
				log.Println("Periodic check triggered.")
				checkPrices()
			case <-resumeCh:
				log.Println("Resume-from-suspend signal received, running check.")
				checkPrices()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	log.Println("NinjaPrice closed.")
}

func checkPrices() {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Printf("Error loading config: %v", err)
		return
	}

	if len(cfg.Items) == 0 {
		log.Println("No products configured to track.")
		return
	}

	hist, err := storage.LoadHistory(historyPath)
	if err != nil {
		log.Printf("Error loading history: %v", err)
		return
	}

	for _, product := range cfg.Items {
		if !product.Active {
			log.Printf("Skipping %s as tracking is disabled.", product.Name)
			continue
		}
		if len(product.Offers) == 0 {
			log.Printf("Skipping %s: no offers configured.", product.Name)
			continue
		}

		var bestPrice, previousBest float64
		var bestOffer config.Offer
		haveBest := false
		havePreviousBest := false

		for _, offer := range product.Offers {
			log.Printf("Checking %s (%s)...", product.Name, offer.Store)
			price, source, err := scraper.ScrapePrice(offer.URL, offer.Selector)
			if err != nil {
				// Out of stock is a normal outcome, not a scrape failure: the
				// price was read fine, it just cannot be bought, so the offer
				// sits out this round rather than feeding the best-price
				// comparison that drives alerts.
				if errors.Is(err, scraper.ErrOutOfStock) {
					log.Printf("Out of stock, ignoring %s (%s)", product.Name, offer.Store)
					continue
				}
				// Stores this process cannot read at all (pcdiga.com and
				// pccomponentes.pt answer 403 to any server-side client) are
				// refreshed by the browser extension instead. A price it
				// reported recently still counts for the comparison, so those
				// offers are not dead weight — it is just not re-recorded here,
				// since nothing new was read.
				if prev, ok := hist.Items[offer.ID]; ok && prev.LastPrice > 0 && time.Since(prev.LastChecked) < alert.StaleAfter {
					log.Printf("Could not read %s (%s); using the %.2f reported %s ago",
						product.Name, offer.Store, prev.LastPrice, time.Since(prev.LastChecked).Round(time.Minute))
					if !haveBest || prev.LastPrice < bestPrice {
						bestPrice, bestOffer, haveBest = prev.LastPrice, offer, true
					}
					continue
				}
				log.Printf("Failed to scrape %s (%s): %v", product.Name, offer.Store, err)
				continue
			}
			log.Printf("Got price for %s (%s) via %s: %.2f", product.Name, offer.Store, source, price)

			if existing, exists := hist.Items[offer.ID]; exists && existing.LastPrice > 0 && (!havePreviousBest || existing.LastPrice < previousBest) {
				previousBest = existing.LastPrice
				havePreviousBest = true
			}

			hist.RecordPrice(offer.ID, price)

			if !haveBest || price < bestPrice {
				bestPrice = price
				bestOffer = offer
				haveBest = true
			}
		}

		if !haveBest {
			continue // every offer failed to scrape this round
		}

		d := alert.Evaluate(product, cfg.CooldownPeriod, bestPrice, previousBest, havePreviousBest)
		alert.Raise(configPath, cfg, product, d, bestOffer, bestPrice, previousBest, havePreviousBest)
	}

	if err := storage.SaveHistory(historyPath, hist); err != nil {
		log.Printf("Error saving history: %v", err)
	}

	log.Println("Check finished.")
}

// runningCosmic reports whether the current session is COSMIC, which is the
// only desktop needing the themed-icon workaround below.
func runningCosmic() bool {
	for _, v := range []string{os.Getenv("XDG_CURRENT_DESKTOP"), os.Getenv("DESKTOP_SESSION")} {
		if strings.Contains(strings.ToUpper(v), "COSMIC") {
			return true
		}
	}
	return false
}

// writeCosmicThemedIcon installs the tray icon into the user's hicolor theme.
//
// COSMIC's status-area applet (v1.0.0) doesn't render icons when IconName is an
// absolute path, and can't find themed icons in ~/.local/share/icons/hicolor/
// unless that directory has an index.theme (required by the freedesktop icon
// spec). Both files are written here; the vendored systray C code calls
// app_indicator_set_icon with the themed name "ninjaprice" so COSMIC resolves
// it through its icon theme chain (Cosmic -> Pop -> hicolor).
func writeCosmicThemedIcon(iconBytes []byte) {
	hicolorDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "icons", "hicolor")
	iconThemeDir := filepath.Join(hicolorDir, "48x48", "apps")
	indexTheme := "[Icon Theme]\nName=Hicolor\nComment=Fallback icon theme\nHidden=true\nDirectories=48x48/apps\n\n[48x48/apps]\nSize=48\nContext=Applications\nType=Threshold\n"

	if err := os.MkdirAll(iconThemeDir, 0755); err != nil {
		log.Printf("Could not create icon theme dir %s: %v", iconThemeDir, err)
	} else if err := os.WriteFile(filepath.Join(hicolorDir, "index.theme"), []byte(indexTheme), 0644); err != nil {
		log.Printf("Could not write index.theme: %v", err)
	} else if err := os.WriteFile(filepath.Join(iconThemeDir, "ninjaprice.png"), iconBytes, 0644); err != nil {
		log.Printf("Could not write themed icon: %v", err)
	} else {
		log.Println("Wrote themed icon for COSMIC panel")
	}
}
