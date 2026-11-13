package mailer

import (
	"bytes"
	"log"
	"net/mail"
	"strings"
	"testing"
)

// setSMTPEnv sets the five SMTP settings for one test; an empty value leaves
// that setting unset.
func setSMTPEnv(t *testing.T, host, port, username, password, from string) {
	t.Helper()
	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", port)
	t.Setenv("SMTP_USERNAME", username)
	t.Setenv("SMTP_PASSWORD", password)
	t.Setenv("MAIL_FROM", from)
}

// TestLoadMailer exercises the SMTP settings / ENV combinations LoadMailer's
// doc comment describes. t.Setenv is used for every variable so no test leaks
// environment state to another, and this suite never touches the network: the
// SMTP cases construct an smtpMailer but never call SendAuthEmail on it.
func TestLoadMailer(t *testing.T) {
	t.Run("all set, env production returns the smtp mailer with defaults", func(t *testing.T) {
		setSMTPEnv(t, "smtp.example.com", "", "pinalert.sender@example.com", "app password", "")

		m, err := LoadMailer("production")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		sm, ok := m.(*smtpMailer)
		if !ok {
			t.Fatalf("LoadMailer returned %T, want *smtpMailer", m)
		}
		if sm.cfg.Host != "smtp.example.com" || sm.cfg.Port != 587 {
			t.Errorf("host/port = %q/%d, want smtp.example.com/587", sm.cfg.Host, sm.cfg.Port)
		}
		if sm.cfg.Username != "pinalert.sender@example.com" {
			t.Errorf("username = %q", sm.cfg.Username)
		}
		if sm.cfg.Password != "app password" {
			t.Errorf("a password for a host other than smtp.gmail.com is used as given, got %q", sm.cfg.Password)
		}
		from, err := mail.ParseAddress(sm.cfg.From)
		if err != nil || from.Name != "Pinalert" || from.Address != "pinalert.sender@example.com" {
			t.Errorf("default From = %q (%v), want Pinalert <username>", sm.cfg.From, err)
		}
	})

	t.Run("custom port and from are used", func(t *testing.T) {
		setSMTPEnv(t, "smtp.example.com", "465", "pinalert.sender@example.com", "pw", "Pinalert Alerts <alerts@example.com>")

		m, err := LoadMailer("production")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		sm := m.(*smtpMailer)
		if sm.cfg.Port != 465 || !sm.implicitTLS() {
			t.Errorf("port = %d, implicit = %v, want 465 and implicit TLS", sm.cfg.Port, sm.implicitTLS())
		}
		from, err := mail.ParseAddress(sm.cfg.From)
		if err != nil || from.Name != "Pinalert Alerts" || from.Address != "alerts@example.com" {
			t.Errorf("From = %q (%v)", sm.cfg.From, err)
		}
	})

	t.Run("host set, env development still returns the smtp mailer", func(t *testing.T) {
		setSMTPEnv(t, "smtp.example.com", "", "pinalert.sender@example.com", "pw", "")

		m, err := LoadMailer("development")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		if _, ok := m.(*smtpMailer); !ok {
			t.Fatalf("LoadMailer returned %T, want *smtpMailer: a set host wins regardless of ENV", m)
		}
	})

	t.Run("nothing set, env development returns the log mailer and warns", func(t *testing.T) {
		setSMTPEnv(t, "", "", "", "", "")
		var logged bytes.Buffer
		previous := log.Writer()
		log.SetOutput(&logged)
		defer log.SetOutput(previous)

		m, err := LoadMailer("development")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		if _, ok := m.(logMailer); !ok {
			t.Fatalf("LoadMailer returned %T, want logMailer", m)
		}
		if !strings.Contains(logged.String(), "SMTP_HOST") {
			t.Errorf("the warning %q does not name SMTP_HOST", logged.String())
		}
	})

	for _, env := range []string{"", "production", "staging"} {
		t.Run("nothing set, env "+env+" returns an error naming the settings", func(t *testing.T) {
			setSMTPEnv(t, "", "", "", "", "")

			m, err := LoadMailer(env)
			if err == nil {
				t.Fatal("LoadMailer returned nil error, want an error naming SMTP_HOST")
			}
			if m != nil {
				t.Fatalf("LoadMailer returned non-nil Mailer %v alongside an error, want nil", m)
			}
			for _, name := range []string{"SMTP_HOST", "SMTP_USERNAME", "SMTP_PASSWORD"} {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error %q does not name %s", err, name)
				}
			}
		})
	}

	t.Run("host set without username names SMTP_USERNAME", func(t *testing.T) {
		setSMTPEnv(t, "smtp.example.com", "", "", "pw", "")
		_, err := LoadMailer("production")
		if err == nil || !strings.Contains(err.Error(), "SMTP_USERNAME") || strings.Contains(err.Error(), "SMTP_PASSWORD") {
			t.Fatalf("err = %v, want an error naming SMTP_USERNAME only", err)
		}
	})

	t.Run("host set without password names SMTP_PASSWORD", func(t *testing.T) {
		setSMTPEnv(t, "smtp.example.com", "", "pinalert.sender@example.com", "", "")
		_, err := LoadMailer("development")
		if err == nil || !strings.Contains(err.Error(), "SMTP_PASSWORD") || strings.Contains(err.Error(), "SMTP_USERNAME") {
			t.Fatalf("err = %v, want an error naming SMTP_PASSWORD only", err)
		}
	})

	t.Run("host set without either names both", func(t *testing.T) {
		setSMTPEnv(t, "smtp.example.com", "", "", "", "")
		_, err := LoadMailer("production")
		if err == nil || !strings.Contains(err.Error(), "SMTP_USERNAME") || !strings.Contains(err.Error(), "SMTP_PASSWORD") {
			t.Fatalf("err = %v, want an error naming both", err)
		}
	})

	t.Run("a half set group is an error outside development", func(t *testing.T) {
		for name, set := range map[string]func(){
			"password only": func() { setSMTPEnv(t, "", "", "", "hunter2hunter2", "") },
			"username only": func() { setSMTPEnv(t, "", "", "pinalert.sender@example.com", "", "") },
			"port only":     func() { setSMTPEnv(t, "", "587", "", "", "") },
			"from only":     func() { setSMTPEnv(t, "", "", "", "", "Pinalert <a@example.com>") },
		} {
			set()
			m, err := LoadMailer("production")
			if err == nil || m != nil {
				t.Errorf("%s: LoadMailer = %v, %v, want an error and no mailer", name, m, err)
				continue
			}
			if !strings.Contains(err.Error(), "SMTP_HOST") {
				t.Errorf("%s: error %q does not name SMTP_HOST", name, err)
			}
		}
	})

	t.Run("a half set group in development warns and uses the log mailer", func(t *testing.T) {
		setSMTPEnv(t, "", "", "", "hunter2hunter2", "")
		var logged bytes.Buffer
		previous := log.Writer()
		log.SetOutput(&logged)
		defer log.SetOutput(previous)

		m, err := LoadMailer("development")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		if _, ok := m.(logMailer); !ok {
			t.Fatalf("LoadMailer returned %T, want logMailer", m)
		}
		if !strings.Contains(logged.String(), "SMTP_HOST") || strings.Contains(logged.String(), "hunter2hunter2") {
			t.Errorf("warning %q should name SMTP_HOST and hold no value", logged.String())
		}
	})

	t.Run("a bad port or from is an error", func(t *testing.T) {
		for _, port := range []string{"abc", "0", "70000", "-1", "587 587"} {
			setSMTPEnv(t, "smtp.example.com", port, "pinalert.sender@example.com", "pw", "")
			if _, err := LoadMailer("production"); err == nil || !strings.Contains(err.Error(), "SMTP_PORT") {
				t.Errorf("port %q: err = %v, want an error naming SMTP_PORT", port, err)
			}
		}
		for _, from := range []string{"not an address", "a@example.com\r\nBcc: b@example.com", "Pinalert <"} {
			setSMTPEnv(t, "smtp.example.com", "", "pinalert.sender@example.com", "pw", from)
			if _, err := LoadMailer("production"); err == nil || !strings.Contains(err.Error(), "MAIL_FROM") {
				t.Errorf("from %q: err = %v, want an error naming MAIL_FROM", from, err)
			}
		}
		setSMTPEnv(t, "smtp.example.com", "", "not-an-email", "pw", "")
		if _, err := LoadMailer("production"); err == nil || !strings.Contains(err.Error(), "MAIL_FROM") {
			t.Errorf("a user name that is no address with MAIL_FROM unset: err = %v, want an error naming MAIL_FROM", err)
		}
	})
}

// TestLoadMailerErrorsNeverHoldAValue pins that no error LoadMailer returns,
// and no log line it writes, repeats the value of any setting.
func TestLoadMailerErrorsNeverHoldAValue(t *testing.T) {
	const (
		host     = "smtp.secret-host.example.com"
		username = "secret.user@example.com"
		password = "SecretPassw0rdValue"
		badPort  = "99secretport"
		badFrom  = "secretfrom-not-an-address"
	)
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	cases := []struct{ env, host, port, user, pw, from string }{
		{"production", "", "", "", password, ""},
		{"production", "", "", username, "", ""},
		{"production", host, "", "", password, ""},
		{"production", host, "", username, "", ""},
		{"production", host, badPort, username, password, ""},
		{"production", host, "", username, password, badFrom},
		{"development", host, "", "", password, ""},
		{"development", "", "", username, password, ""},
	}
	for i, c := range cases {
		setSMTPEnv(t, c.host, c.port, c.user, c.pw, c.from)
		_, err := LoadMailer(c.env)
		text := logged.String()
		if err != nil {
			text += err.Error()
		}
		for _, value := range []string{host, username, password, badPort, badFrom, "secret"} {
			if strings.Contains(text, value) {
				t.Errorf("case %d: error or log holds %q: %s", i, value, text)
			}
		}
	}
}
