package notifier

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"

	"github.com/jacksonsieben/ninja-price/config"
)

// SendEmail delivers a plain-text email via the configured SMTP server. It
// silently no-ops (returns nil) when SMTP isn't configured, so callers can
// invoke it unconditionally without special-casing "not set up yet".
func SendEmail(cfg *config.SMTPConfig, subject, body string) error {
	if cfg == nil || cfg.Host == "" || cfg.To == "" {
		return nil
	}

	password := ""
	if cfg.PasswordEnv != "" {
		password = os.Getenv(cfg.PasswordEnv)
	}

	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	msg := buildMessage(from, cfg.To, subject, body)
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

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
	b.WriteString(body)
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
