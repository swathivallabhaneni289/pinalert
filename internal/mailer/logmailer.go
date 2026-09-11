package mailer

import (
	"context"
	"log"
)

// logMailer implements Mailer by printing the verification link to the
// process log instead of sending real email — the development/test
// fallback so the flow can be completed end to end without a provider key
// or network access. It deliberately logs the full link (accepted risk
// AR-28, 01.1-01-PLAN.md): this is the only way to complete the flow
// without a Resend key, and plan 01.1-03 makes selecting this
// implementation a fatal error outside ENV=development.
type logMailer struct{}

// NewLogMailer constructs a Mailer that logs instead of sending real email.
func NewLogMailer() Mailer {
	return logMailer{}
}

func (logMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	log.Printf("mailer: [dev] verification link for %s: %s", email, link)
	return nil
}
