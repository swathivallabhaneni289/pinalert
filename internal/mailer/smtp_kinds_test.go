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
