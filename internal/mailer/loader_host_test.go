package mailer

import (
	"strings"
	"testing"
)

// TestLoadMailerRefusesABadHost pins IN-05: SMTP_HOST holding a colon (a port
// or a scheme), a slash, a space or a control character is refused at load
// time, in development too. The error names the setting and never repeats the
// value.
func TestLoadMailerRefusesABadHost(t *testing.T) {
	for _, host := range []string{
		"secrethost.example.com:587",
		"smtps://secrethost.example.com",
		"smtp://secrethost.example.com:465",
		"secrethost.example.com/",
		"secrethost.example.com/path",
		"secret host.example.com",
		"secrethost.example.com\tx",
		"secrethost.example.com\a",
		"::1",
		"[::1]:587",
	} {
		for _, env := range []string{"production", "development"} {
			setSMTPEnv(t, host, "", testSMTPUser, testSMTPPassword, "")
			m, err := LoadMailer(env)
			if err == nil || m != nil {
				t.Errorf("host %q, env %s: LoadMailer = %v, %v, want an error and no mailer", host, env, m, err)
				continue
			}
			if !strings.Contains(err.Error(), "SMTP_HOST") {
				t.Errorf("host %q: error %q does not name SMTP_HOST", host, err)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "587") || strings.Contains(err.Error(), testSMTPPassword) {
				t.Errorf("host %q: error %q repeats the value", host, err)
			}
		}
	}
}

// TestLoadMailerAcceptsPlainHostNames pins that the host check lets ordinary
// names and addresses through.
func TestLoadMailerAcceptsPlainHostNames(t *testing.T) {
	for _, host := range []string{"smtp.gmail.com", "smtp-relay.example.com", "mail.example.co.in", "localhost", "127.0.0.1"} {
		setSMTPEnv(t, host, "", testSMTPUser, testSMTPPassword, "")
		m, err := LoadMailer("production")
		if err != nil {
			t.Errorf("host %q: %v", host, err)
			continue
		}
		if sm, ok := m.(*smtpMailer); !ok || sm.cfg.Host != host {
			t.Errorf("host %q: LoadMailer returned %T %v", host, m, m)
		}
	}
}
