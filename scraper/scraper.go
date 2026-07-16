package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/chromedp/chromedp"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ScrapePrice fetches the URL and extracts the price. It tries a lightweight
// HTTP fetch first (fast, near-zero RAM) and only falls back to a full headless
// browser when the site blocks the plain request (e.g. a Cloudflare/DataDome
// challenge). manualSelector may be empty, in which case only the known-site
// table and generic structured-data detection are used.
func ScrapePrice(pageURL string, manualSelector string) (float64, string, error) {
	host := hostFromURL(pageURL)

	if doc, status, err := fetchLight(pageURL); err == nil && !isBotChallenge(status, doc) {
		if price, source, ok := runExtractionChain(doc, host, manualSelector); ok {
			return price, source, nil
		}
	}

	doc, err := fetchHeavy(pageURL)
	if err != nil {
		return 0, "", fmt.Errorf("error scraping with headless browser: %v", err)
	}

	if price, source, ok := runExtractionChain(doc, host, manualSelector); ok {
		return price, source, nil
	}

	return 0, "", fmt.Errorf("could not find a price for %s (tried selector, known-site table, JSON-LD, and meta tags)", pageURL)
}

// DetectPrice runs the same detection chain without a manual selector. It's
// used to auto-suggest a price and item name when adding a new item, so the
// picker only needs manual click-to-select as a last resort.
func DetectPrice(pageURL string) (price float64, source string, name string, err error) {
	host := hostFromURL(pageURL)

	doc, status, ferr := fetchLight(pageURL)
	if ferr != nil || isBotChallenge(status, doc) {
		doc, ferr = fetchHeavy(pageURL)
		if ferr != nil {
			return 0, "", "", fmt.Errorf("error scraping with headless browser: %v", ferr)
		}
	}

	price, source, ok := runExtractionChain(doc, host, "")
	if !ok {
		return 0, "", "", fmt.Errorf("could not auto-detect a price for %s", pageURL)
	}

	return price, source, pageTitle(doc), nil
}

// runExtractionChain tries, in order: an explicit manual selector, the
// hardcoded known-site table, JSON-LD structured data, then meta tags. First
// match wins.
func runExtractionChain(doc *goquery.Document, host, manualSelector string) (float64, string, bool) {
	if price, ok := extractSelector(doc, manualSelector); ok {
		return price, "manual", true
	}
	if price, ok := extractKnownSite(doc, host); ok {
		return price, "known-site", true
	}
	if price, ok := extractJSONLD(doc); ok {
		return price, "json-ld", true
	}
	if price, ok := extractMeta(doc); ok {
		return price, "meta", true
	}
	return 0, "", false
}

// fetchLight performs a plain HTTP GET using a browser-fingerprinted TLS
// client (bogdanfinn/tls-client), which is enough to get past simple
// anti-bot checks without paying for a headless browser launch.
func fetchLight(pageURL string) (*goquery.Document, int, error) {
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithClientProfile(profiles.Chrome_133),
		tlsclient.WithTimeoutSeconds(10),
	)
	if err != nil {
		return nil, 0, err
	}

	req, err := fhttp.NewRequest(fhttp.MethodGet, pageURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return doc, resp.StatusCode, nil
}

// fetchHeavy uses a real headless Chrome instance (chromedp) to render the
// page, which solves JS-based bot challenges (Cloudflare, DataDome) that
// fetchLight can't get past. It closes immediately after grabbing the
// rendered HTML to free RAM.
func fetchHeavy(pageURL string) (*goquery.Document, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent(userAgent),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var renderedHTML string
	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		// Fixed settle time instead of WaitVisible(selector): fetchHeavy runs
		// generic detection (JSON-LD/meta/known-site), not a specific selector,
		// and this also gives Cloudflare/DataDome challenges time to resolve.
		chromedp.Sleep(4*time.Second),
		chromedp.OuterHTML("html", &renderedHTML),
	)
	if err != nil {
		return nil, err
	}

	return goquery.NewDocumentFromReader(strings.NewReader(renderedHTML))
}

func hostFromURL(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
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
