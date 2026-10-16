package mailer

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"

	"github.com/resend/resend-go/v3"
)

// verificationSubject and verificationBodyText hold the UI-SPEC-prescribed
// copy for the verification email (01.1-UI-SPEC.md "Verification email").
// Both are duplicated verbatim rather than referenced from elsewhere so the
// acceptance-criteria greps in 01.1-03-PLAN.md have a single, stable source
// to match against.
const (
	verificationSubject  = "Verify your email for Pinalert"
	verificationBodyText = "Click the link below to verify your email and start using Pinalert. " +
		"This link expires in 5 minutes and can only be used once."
	verificationButtonLabel = "Verify email"
	verificationDisclaimer  = "Didn't request this? You can safely ignore this email. No account will be created."
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

// The kinds of Resend failure the caller's log can see. None of them holds any
// text from Resend's answer.
var (
	errResendNetwork = errors.New("could not reach resend")
	errResendRefused = errors.New("resend refused the request")
)

// SendVerificationLink sends the verification email through Resend. The
// returned error names the operation and the kind of failure only, never the
// API key, the raw response body or the SDK's own error text: Resend's error
// message can repeat the recipient address, and this error reaches the server
// log. The visitor still sees only the generic 500 established in 01.1-01
// (mailer.go doc comment, threat T-01-55).
func (m *resendMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	_, err := m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{email},
		Subject: verificationSubject,
		Html:    verificationEmailHTML(link),
	})
	if err != nil {
		return fmt.Errorf("mailer: sending verification email via resend: %w", resendFailureKind(err))
	}
	return nil
}

// resendFailureKind maps an SDK error to an error that keeps its kind and
// drops its text. A rate limit keeps the Retry-After header value, a number
// Resend sets. A cancelled or timed out context is returned as the context's
// own error, so errors.Is still finds it. Anything else is a network failure
// when the request never got an answer, and a refusal otherwise.
func resendFailureKind(err error) error {
	var rate *resend.RateLimitError
	switch {
	case errors.As(err, &rate):
		if _, convErr := strconv.Atoi(rate.RetryAfter); convErr == nil {
			return fmt.Errorf("%w (retry after %s seconds)", resend.ErrRateLimit, rate.RetryAfter)
		}
		return resend.ErrRateLimit
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		return context.Canceled
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return errResendNetwork
	}
	return errResendRefused
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
