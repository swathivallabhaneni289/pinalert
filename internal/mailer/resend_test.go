package mailer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/resend/resend-go/v3"
)

// TestVerificationEmailHTML exercises verificationEmailHTML only — no
// client is constructed and no network is touched, per 01.1-03-PLAN.md's
// requirement that CI never reach Resend.
func TestVerificationEmailHTML(t *testing.T) {
	link := "https://pinalert.example/auth/verify?token=abc123"
	body := verificationEmailHTML(link)

	if got := strings.Count(body, link); got != 1 {
		t.Errorf("verificationEmailHTML(%q) contains the link %d times, want exactly 1 (body: %s)", link, got, body)
	}
	if !strings.Contains(body, "Verify email") {
		t.Errorf("verificationEmailHTML(%q) missing button label %q", link, "Verify email")
	}
	if !strings.Contains(body, "This link expires in 5 minutes and can only be used once.") {
		t.Errorf("verificationEmailHTML(%q) missing expiry copy", link)
	}
	if !strings.Contains(body, "Didn't request this? You can safely ignore this email. No account will be created.") {
		t.Errorf("verificationEmailHTML(%q) missing didn't-request-this line", link)
	}
}

// TestVerificationEmailHTMLEscapesLink guards against an attacker-supplied
// link value (the requested email address is user input) injecting markup
// into the anchor href or visible body.
func TestVerificationEmailHTMLEscapesLink(t *testing.T) {
	link := `https://pinalert.example/auth/verify?token="><script>alert(1)</script>`
	body := verificationEmailHTML(link)

	if strings.Contains(body, "<script>") {
		t.Errorf("verificationEmailHTML(%q) did not escape the link; body contains raw <script>: %s", link, body)
	}
}

// testResendMailer points a real Resend client at a local test server that
// answers every request with status, contentType and body. Nothing leaves the
// machine.
func testResendMailer(t *testing.T, status int, contentType, body string, header map[string]string) *resendMailer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	base, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	client := resend.NewCustomClient(srv.Client(), "re_test_key")
	client.BaseURL = base
	return &resendMailer{client: client, from: "Pinalert <noreply@pinalert.example>"}
}

// assertNoAddress fails when err, or any error it wraps, holds an "@": the
// returned error reaches the server log and must never carry the recipient
// address that Resend's own error text can repeat.
func assertNoAddress(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("SendVerificationLink returned nil, want an error")
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(e.Error(), "@") {
			t.Errorf("error %q holds an @, want no address in the error", e.Error())
		}
	}
}

// TestResendErrorHoldsNoAddress drives the real SDK against a local server
// whose refusals repeat the recipient address, the way Resend's validation
// messages can, and checks that the returned error keeps only the kind.
func TestResendErrorHoldsNoAddress(t *testing.T) {
	const recipient = "someone.private@example.com"
	ctx := context.Background()

	t.Run("validation refusal", func(t *testing.T) {
		m := testResendMailer(t, http.StatusUnprocessableEntity, "application/json",
			`{"statusCode":422,"name":"validation_error","message":"Invalid `+"`to`"+` field: someone.private@example.com"}`, nil)
		err := m.SendVerificationLink(ctx, recipient, "https://pinalert.example/auth/verify?token=x")
		assertNoAddress(t, err)
		if !errors.Is(err, errResendRefused) {
			t.Errorf("error %q is not errResendRefused", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		m := testResendMailer(t, http.StatusInternalServerError, "application/json",
			`{"message":"could not deliver to someone.private@example.com"}`, nil)
		err := m.SendVerificationLink(ctx, recipient, "https://pinalert.example/auth/verify?token=x")
		assertNoAddress(t, err)
		if !errors.Is(err, errResendRefused) {
			t.Errorf("error %q is not errResendRefused", err)
		}
	})

	t.Run("rate limit", func(t *testing.T) {
		m := testResendMailer(t, http.StatusTooManyRequests, "application/json",
			`{"message":"too many emails to someone.private@example.com"}`, map[string]string{"retry-after": "7"})
		err := m.SendVerificationLink(ctx, recipient, "https://pinalert.example/auth/verify?token=x")
		assertNoAddress(t, err)
		if !errors.Is(err, resend.ErrRateLimit) {
			t.Errorf("error %q is not resend.ErrRateLimit", err)
		}
		if !strings.Contains(err.Error(), "retry after 7 seconds") {
			t.Errorf("error %q lost the Retry-After value", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		m := testResendMailer(t, http.StatusOK, "application/json", `{"id":"x"}`, nil)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		err := m.SendVerificationLink(cancelled, recipient, "https://pinalert.example/auth/verify?token=x")
		assertNoAddress(t, err)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error %q is not context.Canceled", err)
		}
	})
}

func TestVerificationSubjectConstant(t *testing.T) {
	if verificationSubject != "Verify your email for Pinalert" {
		t.Errorf("verificationSubject = %q, want %q", verificationSubject, "Verify your email for Pinalert")
	}
}
