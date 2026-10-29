package mailer

import (
	"errors"
	"fmt"
	"html"
	"strings"
)

// AuthEmailKind names one of the four emails the password sign in sends.
type AuthEmailKind string

// The four kinds. The words of each come from the UI-SPEC "Emails (user
// visible copy)" table of phase 07.1.
const (
	// KindCodeSignup carries the 6 digit code of Create account.
	KindCodeSignup AuthEmailKind = "code_signup"
	// KindCodeReset carries the 6 digit code of Forgot password.
	KindCodeReset AuthEmailKind = "code_reset"
	// KindAccountExists tells an address that already has a password that
	// someone asked to create an account with it. It holds no code.
	KindAccountExists AuthEmailKind = "account_exists"
	// KindNoAccount tells an address with no account that someone asked to
	// reset its password. It holds no code.
	KindNoAccount AuthEmailKind = "no_account"
)

// ErrBadAuthEmail is returned by AuthEmailContent for an unknown kind, a code
// that is not exactly 6 digits, or a validity below 1 minute.
var ErrBadAuthEmail = errors.New("mailer: bad auth email")

// AuthEmail is what the caller hands the mailer. Code and MinutesValid are
// used by the two code kinds only. MinutesValid is passed in by the caller so
// the words of the email cannot drift from the lifetime the service enforces.
type AuthEmail struct {
	Kind         AuthEmailKind
	Code         string
	MinutesValid int
}

// authEmailCodeBoxStyle is the bordered box of the code in the HTML part
// (UI-SPEC: system font, the code at 24px and weight 600).
const authEmailCodeBoxStyle = "display:inline-block;border:1px solid #888888;padding:8px 14px;" +
	"font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;font-size:24px;font-weight:600;letter-spacing:2px"

// AuthEmailContent builds the subject, the plain text part and the HTML part
// of an auth email. It is pure: no client, no network, so its words are pinned
// by tests. D-01: no email holds a link, a URL or an address to click, and the
// text part and the HTML part repeat the same sentences.
func AuthEmailContent(msg AuthEmail) (subject, text, htmlBody string, err error) {
	var sentences []string
	showCode := false
	switch msg.Kind {
	case KindCodeSignup, KindCodeReset:
		if !isSixDigits(msg.Code) || msg.MinutesValid < 1 {
			return "", "", "", ErrBadAuthEmail
		}
		showCode = true
		subject = "Your Pinalert code"
		first := "Your Pinalert code is " + msg.Code + "."
		last := "If you did not ask for it, ignore this email. Nothing happens until the code is typed."
		if msg.Kind == KindCodeReset {
			first = "Your Pinalert password reset code is " + msg.Code + "."
			last = "If you did not ask for it, ignore this email. Your password has not changed."
		}
		sentences = []string{
			first,
			fmt.Sprintf("Type it in the browser where you asked for it, within %d minutes.", msg.MinutesValid),
			last,
		}
	case KindAccountExists:
		subject = "Your Pinalert account"
		sentences = []string{
			"Someone asked to create a Pinalert account with this email, but you already have one.",
			"Sign in with your email and password. If you forgot your password, choose Forgot password on the sign in page.",
			"If this was not you, ignore this email.",
		}
	case KindNoAccount:
		subject = "Your Pinalert password reset"
		sentences = []string{
			"Someone asked to reset the password for this email, but there is no Pinalert account with it.",
			"To make one, choose Create account on the Pinalert sign in page.",
			"If this was not you, ignore this email.",
		}
	default:
		return "", "", "", ErrBadAuthEmail
	}

	text = strings.Join(sentences, "\n\n") + "\n"

	var b strings.Builder
	for i, s := range sentences {
		b.WriteString("<p style=\"font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif\">")
		b.WriteString(html.EscapeString(s))
		b.WriteString("</p>")
		if showCode && i == 0 {
			b.WriteString("<p><span style=\"" + authEmailCodeBoxStyle + "\">")
			b.WriteString(html.EscapeString(msg.Code))
			b.WriteString("</span></p>")
		}
	}
	return subject, text, b.String(), nil
}

// isSixDigits reports whether s is exactly six ASCII digits.
func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
