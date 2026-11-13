package mailer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/resend/resend-go/v3"
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
		client: newResendClient(&http.Client{Timeout: resendTimeout}, apiKey),
		from:   from,
	}
}

// resendTimeout is the whole request limit resend.NewClient uses.
const resendTimeout = time.Minute

// newResendClient builds the SDK client the way resend.NewClient does (the key
// trimmed of spaces and quotes), over a copy of httpClient whose transport
// notes the HTTP status of each answer. The SDK drops that status from its
// errors, and it is the one detail that tells a bad key from an unverified
// sending domain or an outage in the server log.
func newResendClient(httpClient *http.Client, apiKey string) *resend.Client {
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	recording := *httpClient
	recording.Transport = statusRecordingTransport{base: base}
	return resend.NewCustomClient(&recording, strings.Trim(strings.TrimSpace(apiKey), "'"))
}

// statusKey carries a *statusRecorder in a request context.
type statusKey struct{}

// statusRecorder holds the HTTP status of the last answer to one send. Each
// send has its own, so sends running at once never mix their statuses.
type statusRecorder struct {
	code atomic.Int32
}

// statusRecordingTransport writes the status of every answer into the
// recorder its request context carries, and changes nothing else.
type statusRecordingTransport struct {
	base http.RoundTripper
}

func (s statusRecordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := s.base.RoundTrip(req)
	if resp != nil {
		if rec, ok := req.Context().Value(statusKey{}).(*statusRecorder); ok {
			rec.code.Store(int32(resp.StatusCode))
		}
	}
	return resp, err
}

// The kinds of Resend failure the caller's log can see. None of them holds any
// text from Resend's answer.
var (
	errResendNetwork = errors.New("could not reach resend")
	errResendRefused = errors.New("resend refused the request")
)

// resendFailureKind maps an SDK error to an error that keeps its kind and
// drops its text. A rate limit keeps the Retry-After header value, a number
// Resend sets. A cancelled or timed out context is returned as the context's
// own error, so errors.Is still finds it. Anything else is a network failure
// when the request never got an answer, and a refusal otherwise, with the
// HTTP status when there was one (status is 0 when none was seen).
func resendFailureKind(err error, status int) error {
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
	if status > 0 {
		return fmt.Errorf("%w (status %d)", errResendRefused, status)
	}
	return errResendRefused
}

// SendAuthEmail sends one auth email through Resend with both a plain text
// part and an HTML part. The returned error names the operation and the kind
// of failure only, never the SDK's text, which can repeat the recipient
// address, and never the code.
func (m *resendMailer) SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error {
	subject, text, htmlBody, err := AuthEmailContent(msg)
	if err != nil {
		return err
	}
	rec := &statusRecorder{}
	_, err = m.client.Emails.SendWithContext(context.WithValue(ctx, statusKey{}, rec), &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
		Text:    text,
	})
	if err != nil {
		return fmt.Errorf("mailer: sending auth email via resend: %w", resendFailureKind(err, int(rec.code.Load())))
	}
	return nil
}
