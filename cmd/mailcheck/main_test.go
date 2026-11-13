package main

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"pinalert/internal/mailer"
)

const (
	checkKey = "re_made_up_key_for_tests_only"
	checkTo  = "reader.person@example.com"
)

// noLoad fails the test if a run reaches the mailer.
func noLoad(t *testing.T) func() (mailer.Mailer, error) {
	return func() (mailer.Mailer, error) {
		t.Helper()
		t.Fatal("the mailer was loaded, but the arguments should have been refused first")
		return nil, nil
	}
}

func clearResendEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"RESEND_API_KEY", "RESEND_FROM"} {
		t.Setenv(name, "")
	}
	// A run needs no database; make sure none is reachable by accident.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PGDATABASE", "")
}

// sentMail is one call a recordingMailer saw.
type sentMail struct {
	to  string
	msg mailer.AuthEmail
}

// recordingMailer is a fake mailer.Mailer: it records each send and returns
// err. It never touches the network.
type recordingMailer struct {
	sent []sentMail
	err  error
}

func (r *recordingMailer) SendAuthEmail(_ context.Context, to string, msg mailer.AuthEmail) error {
	r.sent = append(r.sent, sentMail{to: to, msg: msg})
	return r.err
}

func fakeLoad(m mailer.Mailer) func() (mailer.Mailer, error) {
	return func() (mailer.Mailer, error) { return m, nil }
}

func assertNoSecrets(t *testing.T, what, text string, extra ...string) {
	t.Helper()
	for _, secret := range append([]string{checkKey, checkTo, "reader.person"}, extra...) {
		if strings.Contains(text, secret) {
			t.Errorf("%s holds %q: %s", what, secret, text)
		}
	}
}

func TestRefusesMissingOrBadArguments(t *testing.T) {
	clearResendEnv(t)
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
		{"an unknown flag", []string{"-key", checkKey}, "not understood"},
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
	clearResendEnv(t)
	// ENV=development must not matter: mailcheck uses the production rule, so
	// the log-only mailer is never used.
	t.Setenv("ENV", "development")

	var out, errOut bytes.Buffer
	code := run([]string{"-to", checkTo}, &out, &errOut, loadProduction)
	if code == 0 {
		t.Fatal("exit code 0 with no RESEND_API_KEY, want non zero")
	}
	if !strings.Contains(errOut.String(), "RESEND_API_KEY") {
		t.Errorf("stderr %q does not name RESEND_API_KEY", errOut.String())
	}
	if strings.Contains(out.String(), "Sent") {
		t.Errorf("stdout claims a send: %q", out.String())
	}
	assertNoSecrets(t, "stderr", errOut.String())
}

func TestSendsOneSampleEmail(t *testing.T) {
	clearResendEnv(t)
	for _, tc := range []struct {
		kind string
		want mailer.AuthEmailKind
	}{
		{"signup", mailer.KindCodeSignup},
		{"reset", mailer.KindCodeReset},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			fake := &recordingMailer{}
			var out, errOut bytes.Buffer
			code := run([]string{"-to", checkTo, "-kind", tc.kind}, &out, &errOut, fakeLoad(fake))
			if code != 0 {
				t.Fatalf("exit code %d, stderr %q", code, errOut.String())
			}
			if out.String() != "Sent. Check the inbox and the spam folder.\n" {
				t.Errorf("stdout = %q", out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("stderr = %q, want empty", errOut.String())
			}

			// One message, to the address typed, of the kind asked for, with a
			// 6 digit code and the real lifetime.
			if len(fake.sent) != 1 {
				t.Fatalf("the fake mailer saw %d sends, want 1", len(fake.sent))
			}
			got := fake.sent[0]
			if got.to != checkTo {
				t.Errorf("sent to %q, want the -to address", got.to)
			}
			if got.msg.Kind != tc.want {
				t.Errorf("kind = %q, want %q", got.msg.Kind, tc.want)
			}
			if !regexp.MustCompile(`^\d{6}$`).MatchString(got.msg.Code) {
				t.Errorf("code %q is not 6 digits", got.msg.Code)
			}
			if got.msg.MinutesValid != 10 {
				t.Errorf("MinutesValid = %d, want 10 (mailer.CodeLifetime)", got.msg.MinutesValid)
			}
			assertNoSecrets(t, "stdout", out.String(), got.msg.Code)
			assertNoSecrets(t, "stderr", errOut.String(), got.msg.Code)
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
	clearResendEnv(t)
	// The fake returns what the real sender returns: the kind of failure and
	// no text from Resend's answer.
	fake := &recordingMailer{err: errors.New("mailer: sending auth email via resend: resend refused the request (status 403)")}

	var out, errOut bytes.Buffer
	code := run([]string{"-to", checkTo}, &out, &errOut, fakeLoad(fake))
	if code == 0 {
		t.Fatal("exit code 0 after a refused send")
	}
	if !strings.Contains(errOut.String(), "resend refused the request") {
		t.Errorf("stderr %q does not say what kind of failure it was", errOut.String())
	}
	if strings.Contains(out.String(), "Sent") {
		t.Errorf("stdout claims a send: %q", out.String())
	}
	assertNoSecrets(t, "stderr", errOut.String())
	if m := regexp.MustCompile(`\d{6}`).FindString(errOut.String()); m != "" {
		t.Errorf("stderr holds a 6 digit number, maybe the code: %q", errOut.String())
	}
	if len(fake.sent) != 1 {
		t.Fatalf("the fake mailer saw %d sends, want 1", len(fake.sent))
	}
	assertNoSecrets(t, "stderr", errOut.String(), fake.sent[0].msg.Code)
}
