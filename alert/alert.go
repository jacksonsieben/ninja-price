// Package alert holds the decision of whether a product's price deserves a
// notification, and the act of raising one.
//
// It exists because prices now arrive from two places. The hourly check scrapes
// them server-side, which works for stores that serve an ordinary HTTP client.
// The stores that do not — pcdiga.com and pccomponentes.pt answer 403 to every
// server-side client, challenge headless Chrome, and 403 even the sitemaps their
// own robots.txt advertises — are reported instead by the browser extension,
// from a real browser session. Both paths have to apply the same rules, so the
// rules live here rather than inline in whichever path was written first.
package alert

import (
	"fmt"
	"log"
	"time"

	"github.com/jacksonsieben/ninja-price/config"
	"github.com/jacksonsieben/ninja-price/notifier"
)

// StaleAfter is how long a recorded price stays usable. Extension-reported
// offers refresh only when their page is visited, so a price has to be allowed
// to outlive the hourly check — but not forever, or a store that quietly
// stopped reporting would keep anchoring the comparison at an old figure.
const StaleAfter = 36 * time.Hour

// Decision is the outcome of evaluating one product. The zero value means no
// notification is due.
type Decision struct {
	Notify    bool
	Title     string
	Message   string
	Sticky    bool
	TargetHit bool
}

// Evaluate decides whether a product whose best price across stores is now
// bestPrice deserves a notification. previousBest is the best price seen before
// this one, and havePrevious says whether there was one at all — a first-ever
// reading cannot be a drop.
//
// It is pure: no I/O, no clock beyond the product's own LastNotified, so the
// two call sites cannot drift apart in behaviour.
func Evaluate(p config.Product, cooldownMinutes int, bestPrice, previousBest float64, havePrevious bool) Decision {
	if bestPrice <= 0 {
		return Decision{}
	}
	if time.Since(p.LastNotified) <= time.Duration(cooldownMinutes)*time.Minute {
		return Decision{}
	}

	if p.TargetPrice > 0 && bestPrice <= p.TargetPrice {
		return Decision{
			Notify:    true,
			Title:     "Price Alert: " + p.Name,
			Message:   fmt.Sprintf("Target price reached! Best price now %.2f", bestPrice),
			Sticky:    p.Sticky,
			TargetHit: true,
		}
	}

	if p.AlertAnyPriceDrop && havePrevious && bestPrice < previousBest {
		return Decision{
			Notify:  true,
			Title:   "Price Drop: " + p.Name,
			Message: fmt.Sprintf("Price dropped by %.2f! Now %.2f", previousBest-bestPrice, bestPrice),
			Sticky:  p.Sticky,
		}
	}

	return Decision{}
}

// Raise sends the desktop notification for a decision, sends the optional
// email, and records that the product was notified so the cooldown applies.
func Raise(configPath string, cfg *config.Config, p config.Product, d Decision, bestOffer config.Offer, bestPrice, previousBest float64, havePrevious bool) {
	if !d.Notify {
		return
	}

	notifier.Notify(d.Title, d.Message, bestOffer.URL, d.Sticky)

	if p.NotifyEmail {
		if err := notifier.SendPriceAlertEmail(cfg.SMTP, d.Title, notifier.PriceAlert{
			ProductName: p.Name,
			Store:       bestOffer.Store,
			URL:         bestOffer.URL,
			OldPrice:    previousBest,
			HasOldPrice: havePrevious,
			NewPrice:    bestPrice,
			TargetPrice: p.TargetPrice,
			TargetHit:   d.TargetHit,
		}); err != nil {
			log.Printf("Error sending email for %s: %v", p.Name, err)
		}
	}

	log.Printf("Notification sent for %s. Best price: %.2f", p.Name, bestPrice)
	if err := config.UpdateLastNotified(configPath, p.ID); err != nil {
		log.Printf("Error updating last_notified for %s: %v", p.Name, err)
	}
}
