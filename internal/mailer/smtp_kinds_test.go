package mailer

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"pinalert/internal/mailer/smtptest"
)

// TestSMTPRefusedSTARTTLSIsAnEncryptionFailure pins IN-01: a server that
// advertises STARTTLS and then refuses it gets the "did not start encryption"
// kind with the reply code, whatever the code, and nothing follows on the plain
// connection: no AUTH, no MAIL, no login. A server with no STARTTLS at all gets
// the same kind.
func TestSMTPRefusedSTARTTLSIsAnEncryptionFailure(t *testing.T) {
	words := " 4.7.0 TLS is not available for " + testSMTPTo + " SERVERWORDS"
	for _, tc := range []struct {
		name string
		opts smtptest.Options
		code string
	}{
		{"454 try again later", smtptest.Options{STARTTLSReply: "454" + words}, "(reply 454)"},
		{"502 not implemented", smtptest.Options{STARTTLSReply: "502" + words}, "(reply 502)"},
		{"554 refused", smtptest.Options{STARTTLSReply: "554" + words, AdvertiseAuthAlways: true}, "(reply 554)"},
		{"no STARTTLS offered", smtptest.Options{NoSTARTTLS: true, AdvertiseAuthAlways: true}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logged bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logged)
			defer log.SetOutput(previous)
			srv := smtptest.Start(t, tc.opts)
			m := testSMTPMailer(srv, false)

			err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
			if !errors.Is(err, errSMTPNoTLS) {
				t.Fatalf("err = %v, want the did not start encryption kind", err)
			}
			if errors.Is(err, errSMTPTemporary) || errors.Is(err, errSMTPPermanent) || errors.Is(err, errSMTPTLS) {
				t.Errorf("err = %v also matches another kind", err)
			}
			if !strings.Contains(err.Error(), "the mail server did not start encryption") {
				t.Errorf("err = %q, want the wording of the kind", err)
			}
			if tc.code != "" && !strings.Contains(err.Error(), tc.code) {
				t.Errorf("err = %q, want the reply code %s", err, tc.code)
			}
			cmds := srv.Commands()
			for _, bad := range []string{"AUTH", "MAIL", "RCPT", "DATA"} {
				if hasCommandPrefix(cmds, "plain "+bad) || hasCommandPrefix(cmds, "tls "+bad) {
					t.Errorf("a %s command followed the refused STARTTLS: %v", bad, cmds)
				}
			}
			if len(srv.Logins()) != 0 {
				t.Errorf("a login reached the server: %+v", srv.Logins())
			}
			assertNoSecrets(t, "error", err.Error(), "SERVERWORDS", "TLS is not available")
			assertNoSecrets(t, "log", logged.String(), "SERVERWORDS")
		})
	}
}

// TestSMTPSignInFailureKinds pins IN-02: a refused password, a server with no
// PLAIN sign in and a connection lost during the sign in are three different
// kinds, so the owner is not sent to revoke a good app password for a failure
// that was not a refusal.
func TestSMTPSignInFailureKinds(t *testing.T) {
	words := " 5.7.8 Username and Password not accepted for " + testSMTPUser + " SERVERWORDS"
	kinds := []error{errSMTPAuth, errSMTPNoAuth, errSMTPAuthLost}
	for _, tc := range []struct {
		name string
		opts smtptest.Options
		kind error
		say  string
	}{
		{"a refused password is the sign in refused kind", smtptest.Options{AuthReply: "535" + words}, errSMTPAuth, "refused the sign in (reply 535)"},
		{"no PLAIN offered is the no password sign in kind", smtptest.Options{AuthMechanisms: "LOGIN XOAUTH2"}, errSMTPNoAuth, "offers no password sign in"},
		{"a connection lost during sign in is the dropped kind", smtptest.Options{DropAtAuth: true}, errSMTPAuthLost, "dropped the connection during sign in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := smtptest.Start(t, tc.opts)
			m := testSMTPMailer(srv, false)

			err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
			if !errors.Is(err, tc.kind) {
				t.Fatalf("err = %v, want %v", err, tc.kind)
			}
			for _, other := range kinds {
				if other != tc.kind && errors.Is(err, other) {
					t.Errorf("err = %v also matches %v", err, other)
				}
			}
			if !strings.Contains(err.Error(), tc.say) {
				t.Errorf("err = %q, want it to say %q", err, tc.say)
			}
			if len(srv.Logins()) != 0 && tc.kind == errSMTPNoAuth {
				t.Errorf("a login reached a server that offers no PLAIN: %+v", srv.Logins())
			}
			if hasCommandPrefix(srv.Commands(), "tls MAIL") {
				t.Errorf("the message was started after a failed sign in: %v", srv.Commands())
			}
			assertNoSecrets(t, "error", err.Error(), "SERVERWORDS", "not accepted")
		})
	}
}
