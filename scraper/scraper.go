package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// ScrapePrice fetches the URL and extracts the price based on a given CSS selector
func ScrapePrice(url string, selector string) (float64, error) {
	// Use headless browser (chromedp) to bypass JavaScript challenges from Cloudflare.
	// The browser executes the challenge, extracts the price, and closes immediately to free RAM.

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout to prevent hanging if the site blocks or takes too long
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var priceStr string

	// Execute browser commands: navigate, wait for selector to appear, extract text
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Text(selector, &priceStr, chromedp.ByQuery),
	)

	if err != nil {
		return 0, fmt.Errorf("error scraping with headless browser: %v", err)
	}

	if priceStr == "" {
		return 0, fmt.Errorf("price not found with selector: %s", selector)
	}

	return SanitizePrice(priceStr)
}

// SanitizePrice converts price strings (e.g., "€ 476,00" or "$1,234.56") to float64
func SanitizePrice(raw string) (float64, error) {
	clean := strings.TrimSpace(raw)

	re := regexp.MustCompile(`[^\d\.,]`)
	clean = re.ReplaceAllString(clean, "")

	if clean == "" {
		return 0, fmt.Errorf("no numeric value found in '%s'", raw)
	}

	lastDot := strings.LastIndex(clean, ".")
	lastComma := strings.LastIndex(clean, ",")

	var finalStr string
	if lastComma > lastDot {
		clean = strings.ReplaceAll(clean, ".", "")
		finalStr = strings.Replace(clean, ",", ".", 1)
	} else if lastDot > lastComma {
		finalStr = strings.ReplaceAll(clean, ",", "")
	} else {
		if lastComma != -1 {
			finalStr = strings.ReplaceAll(clean, ",", ".")
		} else {
			finalStr = clean
		}
	}

	price, err := strconv.ParseFloat(finalStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price '%s': %v", finalStr, err)
	}

	return price, nil
}
