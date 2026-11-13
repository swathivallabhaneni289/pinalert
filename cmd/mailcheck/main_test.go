package main

import (
	"bytes"
	"io"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"testing"

	"pinalert/internal/mailer"
	"pinalert/internal/mailer/smtptest"
)

const (
	checkUser     = "pinalert.sender@example.com"
	checkPassword = "s3cretAppPassw0rd"
	checkTo       = "reader.person@example.com"
)

// noLoad fails the test if a run reaches the mailer.
func noLoad(t *testing.T) func() (mailer.Mailer, error) {
	return func() (mailer.Mailer, error) {
		t.Helper()
		t.Fatal("the mailer was loaded, but the arguments should have been refused first")
		return nil, nil
	}
}

func clearSMTPEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "MAIL_FROM"} {
		t.Setenv(name, "")
	}
	// A run needs no database; make sure none is reachable by accident.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PGDATABASE", "")
}

func fakeLoad(srv *smtptest.Server) func() (mailer.Mailer, error) {
	return func() (mailer.Mailer, error) {
		return mailer.NewSMTPMailerWithRootCAs(mailer.SMTPConfig{
			Host:     srv.Host,
			Port:     srv.Port,
			Username: checkUser,
			Password: checkPassword,
			From:     "Pinalert <" + checkUser + ">",
		}, srv.Roots), nil
	}
}

func assertNoSecrets(t *testing.T, what, text string, extra ...string) {
	t.Helper()
	for _, secret := range append([]string{checkPassword, checkUser, checkTo, "reader.person"}, extra...) {
		if strings.Contains(text, secret) {
			t.Errorf("%s holds %q: %s", what, secret, text)
		}
	}
}

func TestRefusesMissingOrBadArguments(t *testing.T) {
	clearSMTPEnv(t)
	cases := []struct {
		name string
		args []string
		say  string
	}{
		{"no -to", nil, "-to is required"},
		{"empty -to", []string{"-to", ""}, "-to is required"},
		{"a name and brackets", []string{"-to", "Reader <" + checkTo + ">"}, "-to must be one plain address"},
		{"angle brackets", []string{"-to", "<" + checkTo + ">"}, "-to must be one plain address"},
		{"not an address", []string{"-to", "nope"}, "-to must be one plain address"},
		{"two addresses", []string{"-to", checkTo + ",other@example.com"}, "-to must be one plain address"},
		{"a header injection", []string{"-to", checkTo + "\r\nBcc: victim@example.com"}, "-to must be one plain address"},
		{"a bad kind", []string{"-to", checkTo, "-kind", "welcome"}, "-kind must be signup or reset"},
		{"a stray argument", []string{"-to", checkTo, "extra"}, "not understood"},
		{"an unknown flag", []string{"-password", checkPassword}, "not understood"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tc.args, &out, &errOut, noLoad(t))
			if code == 0 {
				t.Fatalf("exit code 0, want non zero")
			}
			if !strings.Contains(errOut.String(), tc.say) {
				t.Errorf("stderr %q does not say %q", errOut.String(), tc.say)
			}
			if out.Len() != 0 {
				t.Errorf("stdout %q should be empty", out.String())
			}
			// Nothing the person typed is echoed back.
			assertNoSecrets(t, "stderr", errOut.String(), "victim@example.com", "other@example.com")
		})
	}
}

func TestMissingSettingsExitNonZeroAndNameThem(t *testing.T) {
	clearSMTPEnv(t)
	// ENV=development must not matter: mailcheck uses the production rule, so
	// the log-only mailer is never used.
	t.Setenv("ENV", "development")

	var out, errOut bytes.Buffer
	code := run([]string{"-to", checkTo}, &out, &errOut, loadProduction)
	if code == 0 {
		t.Fatal("exit code 0 with no SMTP settings, want non zero")
	}
	for _, name := range []string{"SMTP_HOST", "SMTP_USERNAME", "SMTP_PASSWORD"} {
		if !strings.Contains(errOut.String(), name) {
			t.Errorf("stderr %q does not name %s", errOut.String(), name)
		}
	}
	if strings.Contains(out.String(), "Sent") {
		t.Errorf("stdout claims a send: %q", out.String())
	}
	assertNoSecrets(t, "stderr", errOut.String())
}

func TestHalfSetSettingsExitNonZero(t *testing.T) {
	clearSMTPEnv(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_USERNAME", checkUser)
	// no SMTP_PASSWORD

	var out, errOut bytes.Buffer
	code := run([]string{"-to", checkTo}, &out, &errOut, loadProduction)
	if code == 0 || !strings.Contains(errOut.String(), "SMTP_PASSWORD") {
		t.Fatalf("code %d, stderr %q: want non zero and a message naming SMTP_PASSWORD", code, errOut.String())
	}
	assertNoSecrets(t, "stderr", errOut.String(), "smtp.example.com")
}

func TestSendsOneSampleEmail(t *testing.T) {
	clearSMTPEnv(t)
	for _, tc := range []struct {
		kind string
		say  string
	}{
		{"signup", "Your Pinalert code is "},
		{"reset", "Your Pinalert password reset code is "},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			srv := smtptest.Start(t, smtptest.Options{})
			var out, errOut bytes.Buffer
			code := run([]string{"-to", checkTo, "-kind", tc.kind}, &out, &errOut, fakeLoad(srv))
			if code != 0 {
				t.Fatalf("exit code %d, stderr %q", code, errOut.String())
			}
			if out.String() != "Sent. Check the inbox and the spam folder.\n" {
				t.Errorf("stdout = %q", out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("stderr = %q, want empty", errOut.String())
			}

			// One message, to the address typed, with a 6 digit code and the
			// real lifetime.
			raw := srv.Message()
			if !strings.Contains(raw, "To: "+checkTo+"\r\n") {
				t.Errorf("the message is not addressed to the -to address:\n%s", raw)
			}
			decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(raw)))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			text := string(decoded)
			match := regexp.MustCompile(regexp.QuoteMeta(tc.say) + `(\d{6})\.`).FindStringSubmatch(text)
			if match == nil {
				t.Fatalf("the message lacks %q and a 6 digit code:\n%s", tc.say, text)
			}
			if !strings.Contains(text, "within 10 minutes") {
				t.Errorf("the message does not carry the real code lifetime:\n%s", text)
			}
			assertNoSecrets(t, "stdout", out.String(), match[1])
			assertNoSecrets(t, "stderr", errOut.String(), match[1])
			logins := srv.Logins()
			if len(logins) != 1 || !logins[0].OverTLS {
				t.Errorf("logins = %+v, want one, over TLS", logins)
			}
		})
	}
}

func TestSampleCodesAreSixDigitsAndVary(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		c, err := sampleCode()
		if err != nil || !regexp.MustCompile(`^\d{6}$`).MatchString(c) {
			t.Fatalf("code %q, err %v", c, err)
		}
		seen[c] = true
	}
	if len(seen) < 20 {
		t.Errorf("40 codes gave only %d different values", len(seen))
	}
}

func TestFailureShowsOnlyTheKind(t *testing.T) {
	clearSMTPEnv(t)
	words := " 5.7.8 Username and Password not accepted for " + checkUser + " and " + checkTo + " SERVERWORDS"
	srv := smtptest.Start(t, smtptest.Options{AuthReply: "535" + words})

	var out, errOut bytes.Buffer
	code := run([]string{"-to", checkTo}, &out, &errOut, fakeLoad(srv))
	if code == 0 {
		t.Fatal("exit code 0 after a refused sign in")
	}
	if !strings.Contains(errOut.String(), "refused the sign in") {
		t.Errorf("stderr %q does not say what kind of failure it was", errOut.String())
	}
	if strings.Contains(out.String(), "Sent") {
		t.Errorf("stdout claims a send: %q", out.String())
	}
	assertNoSecrets(t, "stderr", errOut.String(), "SERVERWORDS", "Username and Password")
	if m := regexp.MustCompile(`\d{6}`).FindString(errOut.String()); m != "" {
		t.Errorf("stderr holds a 6 digit number, maybe the code: %q", errOut.String())
	}
}
