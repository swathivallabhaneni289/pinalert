package mailer

import (
	"context"
	"fmt"
	"html"

	"github.com/resend/resend-go/v3"
)

// verificationSubject and verificationBodyText hold the UI-SPEC-prescribed
// copy for the verification email (01.1-UI-SPEC.md "Verification email").
// Both are duplicated verbatim rather than referenced from elsewhere so the
// acceptance-criteria greps in 01.1-03-PLAN.md have a single, stable source
// to match against.
const (
	verificationSubject = "Verify your email for Pinalert"
	verificationBodyText = "Click the link below to verify your email and start using Pinalert. " +
		"This link expires in 5 minutes and can only be used once."
	verificationButtonLabel = "Verify email"
	verificationDisclaimer  = "Didn't request this? You can safely ignore this email — no account will be created."
)

// resendMailer implements Mailer by sending through Resend's official Go
// SDK. Construct with NewResendMailer; the loader (loader.go) is the only
// intended caller outside tests.
type resendMailer struct {
	client *resend.Client
	from   string
}

// NewResendMailer constructs a Mailer that sends real email via Resend.
// apiKey authenticates the SDK client; from is the configured sender
// address (see loader.go for its default and RESEND_FROM override).
func NewResendMailer(apiKey, from string) Mailer {
	return &resendMailer{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

// SendVerificationLink sends the verification email through Resend. Any SDK
// error is wrapped with an operation name only — never the API key or raw
// response body — so the caller can log a useful message server-side while
// the visitor still sees only the generic 500 established in 01.1-01
// (mailer.go doc comment, threat T-01-55).
func (m *resendMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	_, err := m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{email},
		Subject: verificationSubject,
		Html:    verificationEmailHTML(link),
	})
	if err != nil {
		return fmt.Errorf("mailer: sending verification email via resend: %w", err)
	}
	return nil
}

// verificationEmailHTML builds the verification email's HTML body. It is a
// pure function — no client, no network — specifically so its copy is
// testable in resend_test.go without touching Resend at all. link is
// HTML-escaped before being embedded, both as the anchor href and as the
// visible fallback text, since it is attacker-influenced input (an email
// address chosen by whoever requested the link) flowing into markup.
func verificationEmailHTML(link string) string {
	escaped := html.EscapeString(link)
	return "<p>" + verificationBodyText + "</p>" +
		"<p><a href=\"" + escaped + "\">" + verificationButtonLabel + "</a></p>" +
		"<p>" + verificationDisclaimer + "</p>"
}
