package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"pinalert/internal/mailer/smtptest"
)

// TestSMTPRefusesAServerThatSpeaksOnlyTLS11 pins IN-04: the client wants TLS
// 1.2 or newer, on STARTTLS and on implicit TLS alike, and sends no password to
// an older server. The control case shows that a TLS 1.2 server, capped the same
// way, is accepted, so the refusal is the version and nothing else.
func TestSMTPRefusesAServerThatSpeaksOnlyTLS11(t *testing.T) {
	for _, implicit := range []bool{false, true} {
		old := smtptest.Start(t, smtptest.Options{MaxTLSVersion: tls.VersionTLS11, ImplicitTLS: implicit})
		m := testSMTPMailer(old, implicit)

		err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
		if !errors.Is(err, errSMTPTLS) {
			t.Fatalf("implicit=%v: err = %v, want errSMTPTLS", implicit, err)
		}
		if len(old.Logins()) != 0 || hasCommandPrefix(old.Commands(), "tls AUTH") || hasCommandPrefix(old.Commands(), "plain AUTH") {
			t.Errorf("implicit=%v: a login reached a TLS 1.1 server: %v %+v", implicit, old.Commands(), old.Logins())
		}
		assertNoSecrets(t, "error", err.Error())

		capped := smtptest.Start(t, smtptest.Options{MaxTLSVersion: tls.VersionTLS12, ImplicitTLS: implicit})
		if err := testSMTPMailer(capped, implicit).SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail()); err != nil {
			t.Errorf("implicit=%v: a TLS 1.2 server should be accepted: %v", implicit, err)
		}
	}
}

// TestSMTPDefaultTimeoutAppliesWithoutADeadline pins IN-04: with no deadline on
// the context (mailcheck passes context.Background()) a silent server is cut at
// the mailer's default, not waited for ever. The test shortens the default
// through the unexported field, and the real value is pinned below.
func TestSMTPDefaultTimeoutAppliesWithoutADeadline(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{Hang: true})
	m := testSMTPMailer(srv, false)
	m.defaultTimeout = 300 * time.Millisecond

	start := time.Now()
	err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed < 250*time.Millisecond || elapsed > 3*time.Second {
		t.Errorf("the send took %v, want it cut near the 300ms default", elapsed)
	}
	assertNoSecrets(t, "error", err.Error())
}

// TestSMTPDefaultTimeoutIsTwentySeconds pins the real value, and that nothing
// outside tests shortens it: a mailer from LoadMailer has no override.
func TestSMTPDefaultTimeoutIsTwentySeconds(t *testing.T) {
	if smtpDefaultTimeout != 20*time.Second {
		t.Errorf("smtpDefaultTimeout = %v, want 20s", smtpDefaultTimeout)
	}
	if got := (&smtpMailer{}).timeout(); got != 20*time.Second {
		t.Errorf("a mailer with no override uses %v, want 20s", got)
	}
	if got := (&smtpMailer{defaultTimeout: time.Second}).timeout(); got != time.Second {
		t.Errorf("the test override is ignored: %v", got)
	}

	setSMTPEnv(t, "smtp.example.com", "", testSMTPUser, testSMTPPassword, "")
	loaded, err := LoadMailer("production")
	if err != nil {
		t.Fatalf("LoadMailer: %v", err)
	}
	if sm := loaded.(*smtpMailer); sm.defaultTimeout != 0 || sm.timeout() != 20*time.Second {
		t.Errorf("a loaded mailer has default timeout override %v, want none (20s)", sm.defaultTimeout)
	}
}
