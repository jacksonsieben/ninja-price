package scraper

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// knownSiteSelectors holds hardcoded CSS selectors for sites that don't expose
// price via JSON-LD or meta tags. Keep this small: only add an entry once
// generic detection has been confirmed not to work for that domain.
var knownSiteSelectors = map[string][]string{
	"amazon.": {".a-price .a-offscreen", ".a-price-whole"},
}

// extractSelector reads the text of the first element matching selector and
// parses it as a price.
func extractSelector(doc *goquery.Document, selector string) (float64, bool) {
	if selector == "" {
		return 0, false
	}
	text := strings.TrimSpace(doc.Find(selector).First().Text())
	if text == "" {
		return 0, false
	}
	price, err := SanitizePrice(text)
	if err != nil {
		return 0, false
	}
	return price, true
}

// extractKnownSite tries the hardcoded selector list for the given host.
func extractKnownSite(doc *goquery.Document, host string) (float64, bool) {
	host = strings.TrimPrefix(host, "www.")
	for domain, selectors := range knownSiteSelectors {
		if !strings.Contains(host, domain) {
			continue
		}
		for _, sel := range selectors {
			if price, ok := extractSelector(doc, sel); ok {
				return price, true
			}
		}
	}
	return 0, false
}

// extractMeta checks common e-commerce meta tags used for social/SEO price previews.
func extractMeta(doc *goquery.Document) (float64, bool) {
	metaSelectors := []string{
		`meta[property="product:price:amount"]`,
		`meta[property="og:price:amount"]`,
	}
	for _, sel := range metaSelectors {
		content, exists := doc.Find(sel).First().Attr("content")
		if !exists || content == "" {
			continue
		}
		if price, err := SanitizePrice(content); err == nil {
			return price, true
		}
	}
	return 0, false
}

// jsonLDNode is a loose schema.org node: only the fields we care about are typed,
// everything else is ignored by encoding/json.
type jsonLDNode struct {
	Type   json.RawMessage `json:"@type"`
	Offers json.RawMessage `json:"offers"`
	Graph  []jsonLDNode    `json:"@graph"`
}

type jsonLDOffer struct {
	Price interface{} `json:"price"`
}

// extractJSONLD looks for a schema.org Product's offer price inside any
// application/ld+json script block. Most e-commerce platforms embed this for
// Google Shopping / rich results, which makes it a reliable site-agnostic signal.
func extractJSONLD(doc *goquery.Document) (float64, bool) {
	var found float64
	ok := false
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if price, matched := parseJSONLDBlock(s.Text()); matched {
			found = price
			ok = true
			return false
		}
		return true
	})
	return found, ok
}

func parseJSONLDBlock(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	var nodes []jsonLDNode

	var single jsonLDNode
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		nodes = append(nodes, single)
	} else if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		return 0, false
	}

	for _, n := range nodes {
		if price, matched := priceFromNode(n); matched {
			return price, true
		}
		for _, g := range n.Graph {
			if price, matched := priceFromNode(g); matched {
				return price, true
			}
		}
	}
	return 0, false
}

func priceFromNode(n jsonLDNode) (float64, bool) {
	if !isProductType(n.Type) || len(n.Offers) == 0 {
		return 0, false
	}

	var offer jsonLDOffer
	if err := json.Unmarshal(n.Offers, &offer); err == nil && offer.Price != nil {
		if price, ok := coercePrice(offer.Price); ok {
			return price, true
		}
	}

	var offers []jsonLDOffer
	if err := json.Unmarshal(n.Offers, &offers); err == nil {
		for _, o := range offers {
			if price, ok := coercePrice(o.Price); ok {
				return price, true
			}
		}
	}

	return 0, false
}

func isProductType(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.Contains(s, "Product")
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, v := range arr {
			if strings.Contains(v, "Product") {
				return true
			}
		}
	}
	return false
}

func coercePrice(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case string:
		price, err := SanitizePrice(val)
		if err != nil {
			return 0, false
		}
		return price, true
	default:
		return 0, false
	}
}

// isBotChallenge detects the anti-bot challenge pages seen from Cloudflare and
// DataDome-protected stores, which return HTML but not the actual product page.
func isBotChallenge(statusCode int, doc *goquery.Document) bool {
	if statusCode == 403 || statusCode == 503 || statusCode == 429 {
		return true
	}
	if doc == nil {
		return false
	}
	title := strings.ToLower(doc.Find("title").First().Text())
	body := strings.ToLower(doc.Find("body").First().Text())
	markers := []string{
		"just a moment",
		"captcha-delivery.com",
		"please enable js",
		"attention required",
		"checking your browser",
	}
	for _, m := range markers {
		if strings.Contains(title, m) || strings.Contains(body, m) {
			return true
		}
	}
	return false
}

// pageTitle returns a human-friendly title for a page, preferring the
// og:title meta tag over the raw <title> element.
func pageTitle(doc *goquery.Document) string {
	if content, exists := doc.Find(`meta[property="og:title"]`).First().Attr("content"); exists && content != "" {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}
