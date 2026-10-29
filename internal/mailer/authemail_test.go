package mailer

import (
	"errors"
	"strings"
	"testing"
)

func allAuthEmails() []AuthEmail {
	return []AuthEmail{
		{Kind: KindCodeSignup, Code: "482913", MinutesValid: 10},
		{Kind: KindCodeReset, Code: "482913", MinutesValid: 10},
		{Kind: KindAccountExists},
		{Kind: KindNoAccount},
	}
}

func TestAuthEmailContentKinds(t *testing.T) {
	cases := []struct {
		msg     AuthEmail
		subject string
		text    string
	}{
		{
			AuthEmail{Kind: KindCodeSignup, Code: "482913", MinutesValid: 10},
			"Your Pinalert code",
			"Your Pinalert code is 482913.\n\n" +
				"Type it in the browser where you asked for it, within 10 minutes.\n\n" +
				"If you did not ask for it, ignore this email. Nothing happens until the code is typed.\n",
		},
		{
			AuthEmail{Kind: KindCodeReset, Code: "482913", MinutesValid: 10},
			"Your Pinalert code",
			"Your Pinalert password reset code is 482913.\n\n" +
				"Type it in the browser where you asked for it, within 10 minutes.\n\n" +
				"If you did not ask for it, ignore this email. Your password has not changed.\n",
		},
		{
			AuthEmail{Kind: KindAccountExists},
			"Your Pinalert account",
			"Someone asked to create a Pinalert account with this email, but you already have one.\n\n" +
				"Sign in with your email and password. If you forgot your password, choose Forgot password on the sign in page.\n\n" +
				"If this was not you, ignore this email.\n",
		},
		{
			AuthEmail{Kind: KindNoAccount},
			"Your Pinalert password reset",
			"Someone asked to reset the password for this email, but there is no Pinalert account with it.\n\n" +
				"To make one, choose Create account on the Pinalert sign in page.\n\n" +
				"If this was not you, ignore this email.\n",
		},
	}
	for _, c := range cases {
		t.Run(string(c.msg.Kind), func(t *testing.T) {
			subject, text, htmlBody, err := AuthEmailContent(c.msg)
			if err != nil {
				t.Fatalf("AuthEmailContent: %v", err)
			}
			if subject != c.subject {
				t.Errorf("subject = %q, want %q", subject, c.subject)
			}
			if text != c.text {
				t.Errorf("text = %q, want %q", text, c.text)
			}
			// The HTML part repeats every sentence of the text part.
			for _, sentence := range strings.Split(strings.TrimSpace(c.text), "\n\n") {
				if !strings.Contains(htmlBody, sentence) {
					t.Errorf("html part misses the sentence %q", sentence)
				}
			}
			hasCode := c.msg.Kind == KindCodeSignup || c.msg.Kind == KindCodeReset
			if got := strings.Contains(text, "482913") || strings.Contains(htmlBody, "482913"); got != hasCode {
				t.Errorf("code present = %v, want %v", got, hasCode)
			}
			if hasCode {
				if n := strings.Count(text, "482913"); n != 1 {
					t.Errorf("text holds the code %d times, want once", n)
				}
				if !strings.Contains(htmlBody, "border:1px solid") || !strings.Contains(htmlBody, "font-size:24px") ||
					!strings.Contains(htmlBody, "font-weight:600") {
					t.Errorf("html code box misses the border, 24px or weight 600 style: %s", htmlBody)
				}
			}
		})
	}
}

func TestAuthEmailContentMinutesComeFromTheCaller(t *testing.T) {
	_, text, htmlBody, err := AuthEmailContent(AuthEmail{Kind: KindCodeSignup, Code: "000123", MinutesValid: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "within 7 minutes.") || !strings.Contains(htmlBody, "within 7 minutes.") {
		t.Errorf("minutes not taken from the caller: %q", text)
	}
}

func TestAuthEmailContentHasNoLink(t *testing.T) {
	for _, msg := range allAuthEmails() {
		subject, text, htmlBody, err := AuthEmailContent(msg)
		if err != nil {
			t.Fatalf("%s: %v", msg.Kind, err)
		}
		for _, part := range []struct{ name, body string }{{"subject", subject}, {"text", text}, {"html", htmlBody}} {
			low := strings.ToLower(part.body)
			for _, bad := range []string{"http", "www."} {
				if strings.Contains(low, bad) {
					t.Errorf("%s %s holds %q", msg.Kind, part.name, bad)
				}
			}
		}
		low := strings.ToLower(htmlBody)
		for _, bad := range []string{"<a", "href", "@"} {
			if strings.Contains(low, bad) {
				t.Errorf("%s html holds %q: %s", msg.Kind, bad, htmlBody)
			}
		}
	}
}

func TestAuthEmailContentPlainPunctuation(t *testing.T) {
	// Written as escapes so this file holds neither character.
	for _, msg := range allAuthEmails() {
		subject, text, htmlBody, err := AuthEmailContent(msg)
		if err != nil {
			t.Fatalf("%s: %v", msg.Kind, err)
		}
		for _, body := range []string{subject, text, htmlBody} {
			if strings.ContainsAny(body, string(rune(0x2014))+string(rune(0x2013))) {
				t.Errorf("%s holds an em or en dash: %q", msg.Kind, body)
			}
		}
	}
}

func TestAuthEmailContentRefusesBadInput(t *testing.T) {
	bad := []AuthEmail{
		{Kind: KindCodeSignup, Code: "12345", MinutesValid: 10},
		{Kind: KindCodeSignup, Code: "1234567", MinutesValid: 10},
		{Kind: KindCodeReset, Code: "12345a", MinutesValid: 10},
		{Kind: KindCodeReset, Code: "<b>12<", MinutesValid: 10},
		{Kind: KindCodeSignup, Code: "", MinutesValid: 10},
		{Kind: KindCodeSignup, Code: "482913", MinutesValid: 0},
		{Kind: KindCodeReset, Code: "482913", MinutesValid: -3},
		{Kind: AuthEmailKind("verification"), Code: "482913", MinutesValid: 10},
		{Kind: AuthEmailKind("")},
	}
	for _, msg := range bad {
		if _, _, _, err := AuthEmailContent(msg); !errors.Is(err, ErrBadAuthEmail) {
			t.Errorf("AuthEmailContent(%+v) error = %v, want ErrBadAuthEmail", msg, err)
		}
	}
}
