package notifier

import (
	"log"
	"os/exec"
	"strings"
)

// Notify shows a desktop notification through notify-send.
//
// When url is non-empty the notification carries an action, so clicking it
// opens the offer. The action is registered under the key "default", which the
// freedesktop specification reserves for activating the notification body:
// daemons implementing it (quickshell, which serves this session, advertises
// "actions") fire it on a plain click, and daemons that do not simply render it
// as a labelled button. Either way there is exactly one affordance.
//
// Passing an action implies --wait in notify-send: the process stays alive until
// the notification is actioned or closed, and prints the invoked action key to
// stdout. A sticky notification never expires on its own, so this always runs on
// its own goroutine and never blocks the price check that raised it.
func Notify(title, message, url string, sticky bool) {
	args := []string{"-a", "NinjaPrice"}

	if sticky {
		// '-u critical' marks the notification as urgent.
		// '-t 0' explicitly sets the timeout to 0 (never expire).
		// This ensures the notification stays on screen until dismissed.
		args = append(args, "-u", "critical", "-t", "0")
	}

	if url != "" {
		args = append(args, "--action", "default=Abrir a oferta")
	}

	args = append(args, title, message)

	if url == "" {
		if err := exec.Command("notify-send", args...).Run(); err != nil {
			log.Printf("Failed to send notification: %v", err)
		}
		log.Printf("Notification sent: %s - %s", title, message)
		return
	}

	go func() {
		out, err := exec.Command("notify-send", args...).Output()
		if err != nil {
			log.Printf("Failed to send notification: %v", err)
			return
		}
		// Empty output means the notification was dismissed or expired rather
		// than activated; only an invoked action opens anything.
		if strings.TrimSpace(string(out)) == "" {
			return
		}
		if err := exec.Command("xdg-open", url).Start(); err != nil {
			log.Printf("Failed to open %s: %v", url, err)
		}
	}()

	log.Printf("Notification sent: %s - %s (click opens %s)", title, message, url)
}
