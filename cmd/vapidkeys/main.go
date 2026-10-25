// Command vapidkeys prints a fresh VAPID key pair for Web Push.
//
// Usage:
//
//	go run ./cmd/vapidkeys
//
// It prints two lines to standard output, VAPID_PUBLIC_KEY=... and
// VAPID_PRIVATE_KEY=..., and one reminder line to standard error. It writes no
// file. Store both values in the host's secret settings, never in the repo or
// an example env file. Changing the pair later breaks every stored
// subscription, so run this once.
package main

import (
	"fmt"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vapidkeys: could not generate a key pair:", err)
		os.Exit(1)
	}
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", public)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", private)
	fmt.Fprintln(os.Stderr, "Store both values as secrets outside the repo. Changing them later breaks every stored subscription.")
}
