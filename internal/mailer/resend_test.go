package mailer

import (
	"strings"
	"testing"
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
	if !strings.Contains(body, "Didn't request this? You can safely ignore this email — no account will be created.") {
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

func TestVerificationSubjectConstant(t *testing.T) {
	if verificationSubject != "Verify your email for Pinalert" {
		t.Errorf("verificationSubject = %q, want %q", verificationSubject, "Verify your email for Pinalert")
	}
}
