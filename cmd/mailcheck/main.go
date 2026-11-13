// Command mailcheck sends one sample sign in code email, so the owner can see a
// real email arrive in a real inbox before letting anyone else in.
//
// Usage:
//
//	go run ./cmd/mailcheck -to someone@example.com [-kind signup|reset]
//
// It reads the same settings as the server, RESEND_API_KEY (required) and
// RESEND_FROM (optional), with the production rule, so the log-only mailer is
// never used: with the key missing it exits with an error that names it. It
// touches no database, needs no DATABASE_URL and starts no server.
//
// Without a verified domain the default sender (Resend's own sandbox address)
// only delivers to the email address the Resend account was made with, so -to
// must be that address until the domain is added and RESEND_FROM is set to an
// address at it.
//
// On success it prints "Sent. Check the inbox and the spam folder." On failure
// it prints the kind of failure. It never prints the API key, the address or
// the code.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/mail"
	"os"
	"strings"
	"time"

	"pinalert/internal/mailer"
)

const usage = "usage: go run ./cmd/mailcheck -to someone@example.com [-kind signup|reset]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, loadProduction))
}

// loadProduction loads the mailer with the production rule, so a missing
// setting is an error and the log-only mailer is never returned.
func loadProduction() (mailer.Mailer, error) {
	return mailer.LoadMailer("production")
}

// run is main without the process: it returns the exit code. load is the seam a
// test replaces with a fake mailer, so no test reaches the network.
func run(args []string, stdout, stderr io.Writer, load func() (mailer.Mailer, error)) int {
	fs := flag.NewFlagSet("mailcheck", flag.ContinueOnError)
	// The flag package echoes what it could not parse; keep that away from the
	// output and print our own words instead.
	fs.SetOutput(io.Discard)
	to := fs.String("to", "", "the address to send the sample email to")
	kind := fs.String("kind", "signup", "signup or reset")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "mailcheck: the arguments are not understood")
		fmt.Fprintln(stderr, usage)
		return 2
	}

	var emailKind mailer.AuthEmailKind
	switch *kind {
	case "signup":
		emailKind = mailer.KindCodeSignup
	case "reset":
		emailKind = mailer.KindCodeReset
	default:
		fmt.Fprintln(stderr, "mailcheck: -kind must be signup or reset")
		return 2
	}
	if *to == "" {
		fmt.Fprintln(stderr, "mailcheck: -to is required")
		fmt.Fprintln(stderr, usage)
		return 2
	}
	if !isPlainAddress(*to) {
		fmt.Fprintln(stderr, "mailcheck: -to must be one plain address")
		return 2
	}

	m, err := load()
	if err != nil {
		fmt.Fprintf(stderr, "mailcheck: %v\n", err)
		// The hint is only for the missing key; any other load error is
		// already explained by the loader's own line.
		if errors.Is(err, mailer.ErrMissingAPIKey) {
			fmt.Fprintln(stderr, "mailcheck: set RESEND_API_KEY (RESEND_FROM is optional) in this shell, then run it again.")
		}
		return 1
	}

	code, err := sampleCode()
	if err != nil {
		fmt.Fprintln(stderr, "mailcheck: could not make a sample code")
		return 1
	}
	msg := mailer.AuthEmail{
		Kind: emailKind,
		Code: code,
		// The real lifetime of a code, so the sample says what a real email says.
		MinutesValid: int(mailer.CodeLifetime / time.Minute),
	}
	if err := m.SendAuthEmail(context.Background(), *to, msg); err != nil {
		// The error holds the kind of failure only: no key, address or code.
		fmt.Fprintf(stderr, "mailcheck: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Sent. Check the inbox and the spam folder.")
	return 0
}

// isPlainAddress reports whether s is exactly one bare email address: no
// display name, no angle brackets, no list, no line break.
func isPlainAddress(s string) bool {
	if strings.ContainsAny(s, "\r\n\x00") {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s
}

// sampleCode returns a random 6 digit code from crypto/rand.
func sampleCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
