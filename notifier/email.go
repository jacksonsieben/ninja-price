package notifier

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/jacksonsieben/ninja-price/config"
)

// PriceAlert carries the data needed to render a price-alert email.
type PriceAlert struct {
	ProductName string
	Store       string
	URL         string
	OldPrice    float64 // ignored unless HasOldPrice
	HasOldPrice bool
	NewPrice    float64
	TargetPrice float64 // ignored unless > 0
	TargetHit   bool
}

// SendPriceAlertEmail renders alert as an HTML email and delivers it via the
// configured SMTP server. It silently no-ops (returns nil) when SMTP isn't
// configured, so callers can invoke it unconditionally without special-casing
// "not set up yet".
func SendPriceAlertEmail(cfg *config.SMTPConfig, subject string, alert PriceAlert) error {
	if cfg == nil || cfg.Host == "" || cfg.To == "" {
		return nil
	}

	body, err := renderPriceAlertHTML(alert)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return sendMail(cfg, subject, body)
}

func sendMail(cfg *config.SMTPConfig, subject, htmlBody string) error {
	password := ""
	if cfg.PasswordEnv != "" {
		password = os.Getenv(cfg.PasswordEnv)
	}

	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	msg := buildMessage(from, cfg.To, subject, htmlBody)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, password, cfg.Host)
	}

	var err error
	if cfg.Port == 465 {
		// Port 465 is implicit TLS from the start of the connection, which
		// smtp.SendMail doesn't handle (it only does STARTTLS upgrades).
		err = sendImplicitTLS(addr, cfg.Host, auth, from, []string{cfg.To}, msg)
	} else {
		err = smtp.SendMail(addr, auth, from, []string{cfg.To}, msg)
	}
	if err != nil {
		log.Printf("Failed to send email notification: %v", err)
		return err
	}
	log.Printf("Email notification sent to %s: %s", cfg.To, subject)
	return nil
}

func buildMessage(from, to, subject, htmlBody string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

func sendImplicitTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	defer tlsConn.Close()

	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// priceAlertView holds the fully-formatted strings the template renders;
// html/template escapes every {{.Field}} (including inside the href
// attribute), so scraped product/store text can't break the markup.
type priceAlertView struct {
	ProductName    string
	Store          string
	URL            string
	Badge          string
	AccentColor    string
	HasOldPrice    bool
	OldPriceFmt    string
	NewPriceFmt    string
	HasSavings     bool
	SavingsFmt     string
	SavingsPctFmt  string
	HasTarget      bool
	TargetPriceFmt string
	TargetHit      bool
	Timestamp      string
}

func formatPrice(p float64) string {
	return fmt.Sprintf("€%.2f", p)
}

func renderPriceAlertHTML(a PriceAlert) (string, error) {
	view := priceAlertView{
		ProductName: a.ProductName,
		Store:       a.Store,
		URL:         a.URL,
		HasOldPrice: a.HasOldPrice && a.OldPrice > a.NewPrice,
		NewPriceFmt: formatPrice(a.NewPrice),
		HasTarget:   a.TargetPrice > 0,
		TargetHit:   a.TargetHit,
		Timestamp:   time.Now().Format("Jan 2, 2006 15:04"),
	}

	if a.TargetHit {
		view.Badge = "Target price reached"
		view.AccentColor = "#16a34a"
	} else {
		view.Badge = "Price drop detected"
		view.AccentColor = "#2563eb"
	}

	if view.HasOldPrice {
		view.OldPriceFmt = formatPrice(a.OldPrice)
		savings := a.OldPrice - a.NewPrice
		view.HasSavings = savings > 0
		view.SavingsFmt = formatPrice(savings)
		view.SavingsPctFmt = fmt.Sprintf("%.0f", savings/a.OldPrice*100)
	}

	if view.HasTarget {
		view.TargetPriceFmt = formatPrice(a.TargetPrice)
	}

	tmpl, err := template.New("priceAlert").Parse(priceAlertTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const priceAlertTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>NinjaPrice Alert</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f5f7;padding:32px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="width:480px;max-width:100%;background-color:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.08);">
          <tr>
            <td style="background-color:{{.AccentColor}};padding:20px 28px;">
              <span style="font-size:18px;font-weight:700;color:#ffffff;">🥷 NinjaPrice</span>
            </td>
          </tr>
          <tr>
            <td style="padding:28px;">
              <p style="margin:0 0 6px;font-size:12px;font-weight:700;letter-spacing:0.05em;text-transform:uppercase;color:{{.AccentColor}};">{{.Badge}}</p>
              <h1 style="margin:0 0 4px;font-size:20px;line-height:1.35;color:#1a1a1a;">{{.ProductName}}</h1>
              <p style="margin:0 0 22px;font-size:13px;color:#6b7280;">at <strong>{{.Store}}</strong></p>

              <table role="presentation" cellpadding="0" cellspacing="0" style="margin-bottom:16px;">
                <tr>
                  {{if .HasOldPrice}}
                  <td style="padding-right:20px;vertical-align:bottom;">
                    <p style="margin:0 0 2px;font-size:11px;color:#9ca3af;">Previous price</p>
                    <p style="margin:0;font-size:18px;color:#9ca3af;text-decoration:line-through;">{{.OldPriceFmt}}</p>
                  </td>
                  {{end}}
                  <td style="vertical-align:bottom;">
                    <p style="margin:0 0 2px;font-size:11px;color:#9ca3af;">New price</p>
                    <p style="margin:0;font-size:30px;font-weight:700;color:#16a34a;">{{.NewPriceFmt}}</p>
                  </td>
                </tr>
              </table>

              {{if .HasSavings}}
              <p style="margin:0 0 20px;font-size:13px;font-weight:600;color:#16a34a;background-color:#f0fdf4;padding:10px 14px;border-radius:8px;display:inline-block;">
                💰 You save {{.SavingsFmt}} ({{.SavingsPctFmt}}% off)
              </p>
              {{end}}

              {{if .HasTarget}}
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f9fafb;border-radius:8px;margin-bottom:24px;">
                <tr>
                  <td style="padding:12px 16px;font-size:13px;color:#4b5563;">
                    🎯 Your target price: <strong>{{.TargetPriceFmt}}</strong>{{if .TargetHit}} <span style="color:#16a34a;font-weight:700;">— reached!</span>{{end}}
                  </td>
                </tr>
              </table>
              {{end}}

              <table role="presentation" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="border-radius:8px;background-color:{{.AccentColor}};">
                    <a href="{{.URL}}" style="display:inline-block;padding:12px 26px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;">View Deal →</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:14px 28px;background-color:#f9fafb;border-top:1px solid #eef0f2;">
              <p style="margin:0;font-size:11px;color:#9ca3af;">Sent by NinjaPrice · {{.Timestamp}}</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`
