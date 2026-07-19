package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/jacksonsieben/ninja-price/api"
	"github.com/jacksonsieben/ninja-price/config"
	"github.com/jacksonsieben/ninja-price/notifier"
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

	// Start local API in background
	go func() {
		localAPI := api.NewAPI(configPath, historyPath)
		localAPI.Start(apiPort)
	}()

	// Start systray
	systray.Run(onReady, onExit)
}

func onReady() {
	if iconBytes, err := os.ReadFile(systrayIconPath); err != nil {
		log.Printf("Could not load systray icon from %s: %v", systrayIconPath, err)
	} else {
		systray.SetIcon(iconBytes)
	}

	systray.SetTitle("NP")
	systray.SetTooltip("NinjaPrice Tracker")

	mCheckNow := systray.AddMenuItem("Check Now", "Check prices immediately")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit NinjaPrice")

	// Start periodic checker
	ticket := time.NewTicker(checkInterval)

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

		canNotify := time.Since(product.LastNotified) > time.Duration(cfg.CooldownPeriod)*time.Minute

		// Check conditions for notifications, comparing against the best (lowest) price across all offers
		if canNotify {
			title, msg := "", ""
			sticky := false
			targetHit := false
			if product.TargetPrice > 0 && bestPrice <= product.TargetPrice {
				title = "Price Alert: " + product.Name
				msg = fmt.Sprintf("Target price reached! Best price now %.2f", bestPrice)
				// Target price reaching is important, we pass true to make it a sticky notification
				sticky = product.Sticky
				targetHit = true
			} else if product.AlertAnyPriceDrop && havePreviousBest && bestPrice < previousBest {
				diff := previousBest - bestPrice
				title = "Price Drop: " + product.Name
				msg = fmt.Sprintf("Price dropped by %.2f! Now %.2f", diff, bestPrice)
				// Regular price drop can also be sticky, or you can change this to false for transient notifications
			}
			if title != "" {
				notifier.Notify(title, msg, sticky)
				if product.NotifyEmail {
					alert := notifier.PriceAlert{
						ProductName: product.Name,
						Store:       bestOffer.Store,
						URL:         bestOffer.URL,
						OldPrice:    previousBest,
						HasOldPrice: havePreviousBest,
						NewPrice:    bestPrice,
						TargetPrice: product.TargetPrice,
						TargetHit:   targetHit,
					}
					if err := notifier.SendPriceAlertEmail(cfg.SMTP, title, alert); err != nil {
						log.Printf("Error sending email for %s: %v", product.Name, err)
					}
				}
				log.Printf("Notification sent for %s. Best price: %.2f", product.Name, bestPrice)
				if err := config.UpdateLastNotified(configPath, product.ID); err != nil {
					log.Printf("Error updating last_notified for %s: %v", product.Name, err)
				}
			}
		}
	}

	if err := storage.SaveHistory(historyPath, hist); err != nil {
		log.Printf("Error saving history: %v", err)
	}

	log.Println("Check finished.")
}