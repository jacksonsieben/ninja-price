package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// mimics an Amazon page with a buy box, a struck-through list price, and
// unrelated other-seller / related-product prices scattered around.
const amazonMultiOffer = `<html><body>
<div id="corePriceDisplay_desktop_feature_div">
  <span class="a-price a-text-price" data-a-color="secondary"><span class="a-offscreen">€411.00</span></span>
  <span class="a-price priceToPay" data-a-color="base"><span class="a-offscreen">€222.42</span></span>
</div>
<div id="rightCol">
  <span class="a-price"><span class="a-offscreen">€218.80</span></span>
</div>
<div id="similarities_feature_div">
  <span class="a-price"><span class="a-offscreen">€162.63</span></span>
</div>
</body></html>`

// mimics an out-of-stock page: buy box has no price, only other offers exist.
const amazonUnavailable = `<html><body>
<div id="corePriceDisplay_desktop_feature_div"></div>
<div id="aod-offer">
  <span class="a-price"><span class="a-offscreen">€375.09</span></span>
  <span class="a-price"><span class="a-offscreen">€129.09</span></span>
</div>
</body></html>`

func TestExtractKnownSite_AmazonPicksBuyBox(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(amazonMultiOffer))
	price, ok := extractKnownSite(doc, "www.amazon.es")
	if !ok {
		t.Fatal("expected a price")
	}
	if price != 222.42 {
		t.Fatalf("expected buy-box price 222.42, got %.2f", price)
	}
}

func TestExtractKnownSite_AmazonUnavailableReturnsNoPrice(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(amazonUnavailable))
	if price, ok := extractKnownSite(doc, "www.amazon.es"); ok {
		t.Fatalf("expected no price for unavailable buy box, got %.2f", price)
	}
}
