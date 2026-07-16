package main

import (
	"fmt"
	"log"
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
		log.Println("No items configured to track.")
		return
	}

	hist, err := storage.LoadHistory(historyPath)
	if err != nil {
		log.Printf("Error loading history: %v", err)
		return
	}

	for _, item := range cfg.Items {
		if !item.Active {
			log.Printf("Skipping %s as alert_price is disabled.", item.Name)
			continue // Skip items that don't have alert_price enabled
		}
		log.Printf("Checking %s...", item.Name)
		currentPrice, source, err := scraper.ScrapePrice(item.URL, item.Selector)
		if err != nil {
			log.Printf("Failed to scrape %s: %v", item.Name, err)
			continue
		}
		log.Printf("Got price for %s via %s: %.2f", item.Name, source, currentPrice)

		histItem, exists := hist.Items[item.ID]
		if !exists {
			histItem = &storage.HistoryItem{
				LowestPrice: currentPrice,
			}
			hist.Items[item.ID] = histItem
		}

		canNotify := time.Since(item.LastNotified) > time.Duration(cfg.CooldownPeriod) * time.Minute

		// Check conditions for notifications
		if canNotify {
			notified := false
			if item.TargetPrice > 0 && currentPrice <= item.TargetPrice {
				msg := fmt.Sprintf("Target price reached! Currently %.2f", currentPrice)
				// Target price reaching is important, we pass true to make it a sticky notification
				notifier.Notify("Price Alert: "+item.Name, msg, item.Sticky)
				notified = true
			} else if item.AlertAnyPriceDrop  && (exists && histItem.LastPrice > 0 && currentPrice < histItem.LastPrice) {
				diff := histItem.LastPrice - currentPrice
				msg := fmt.Sprintf("Price dropped by %.2f! Now %.2f", diff, currentPrice)
				// Regular price drop can also be sticky, or you can change this to false for transient notifications
				notifier.Notify("Price Drop: "+item.Name, msg, false)
				notified = true
			}
			if notified {
				log.Printf("Notification sent for %s. Current price: %.2f", item.Name, currentPrice)
				if err := config.UpdateLastNotified(configPath, item.ID); err != nil {
					log.Printf("Error updating last_notified for %s: %v", item.Name, err)
				}
			}
		}

		// Update history
		histItem.LastPrice = currentPrice
		if currentPrice < histItem.LowestPrice {
			histItem.LowestPrice = currentPrice
		}
		histItem.LastChecked = time.Now()

		pricePoint := storage.PricePoint{
			Price: currentPrice,
			Date:  time.Now(), 
		}
		histItem.History = append(histItem.History, pricePoint)

		if len(histItem.History) > 14 {
			histItem.History = histItem.History[1:]
		}

		hist.Items[item.ID] = histItem
	}

	if err := storage.SaveHistory(historyPath, hist); err != nil {
		log.Printf("Error saving history: %v", err)
	}

	log.Println("Check finished.")
}