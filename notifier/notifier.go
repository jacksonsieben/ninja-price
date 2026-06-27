package notifier

import (
	"log"
	"os/exec"
)

func Notify(title, message string, sticky bool) {
	// Uses notify-send on Linux (Fedora)
	args := []string{"-a", "NinjaPrice"}

	if sticky {
		// '-u critical' marks the notification as urgent.
		// '-t 0' explicitly sets the timeout to 0 (never expire).
		// This ensures the notification stays on screen until dismissed.
		args = append(args, "-u", "critical", "-t", "0")
	}

	args = append(args, title, message)

	cmd := exec.Command("notify-send", args...)
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to send notification: %v", err)
	}
	log.Printf("Notification sent: %s - %s", title, message)
}
