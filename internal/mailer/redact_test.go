package mailer

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestSMTPConfigAndMailerNeverPrintThePassword pins WR-01: no fmt verb prints
// the app password, the user name or the sender, whether the value is the
// config, a pointer to it, the mailer LoadMailer returns, or that mailer inside
// the send ceiling.
func TestSMTPConfigAndMailerNeverPrintThePassword(t *testing.T) {
	setSMTPEnv(t, "smtp.example.com", "587", testSMTPUser, testSMTPPassword, "Pinalert <"+testSMTPUser+">")
	loaded, err := LoadMailer("production")
	if err != nil {
		t.Fatalf("LoadMailer: %v", err)
	}
	cfg := SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: testSMTPUser,
		Password: testSMTPPassword,
		From:     "Pinalert <" + testSMTPUser + ">",
	}
	ceiling := NewSendCeiling(loaded, DefaultCeilingLimits, time.Now)
	type holder struct {
		Mail Mailer
		Cfg  SMTPConfig
	}

	values := map[string]any{
		"config value":      cfg,
		"config pointer":    &cfg,
		"loaded mailer":     loaded,
		"mailer value":      *(loaded.(*smtpMailer)),
		"ceiling":           ceiling,
		"struct holding it": holder{Mail: loaded, Cfg: cfg},
		"slice of configs":  []SMTPConfig{cfg},
		"map of configs":    map[string]SMTPConfig{"a": cfg},
	}
	// Every verb: the string verbs, the Go syntax verb, and the number verbs,
	// which a String method alone does not cover.
	for name, v := range values {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%d", "%T"} {
			text := fmt.Sprintf(verb, v)
			for _, secret := range []string{testSMTPPassword, testSMTPUser, "pinalert.sender"} {
				if strings.Contains(text, secret) {
					t.Errorf("%s with %s holds %q: %s", name, verb, secret, text)
				}
			}
		}
		for what, text := range map[string]string{"Sprint": fmt.Sprint(v), "Sprintln": fmt.Sprintln(v)} {
			if strings.Contains(text, testSMTPPassword) || strings.Contains(text, testSMTPUser) {
				t.Errorf("%s with %s holds a secret: %s", name, what, text)
			}
		}
	}
}

// TestSMTPConfigStringKeepsTheHost pins that the redacted text is still useful:
// it names the host and port, and says the secrets are hidden.
func TestSMTPConfigStringKeepsTheHost(t *testing.T) {
	cfg := SMTPConfig{Host: "smtp.example.com", Port: 587, Username: testSMTPUser, Password: testSMTPPassword, From: testSMTPUser}
	for what, text := range map[string]string{
		"String":   cfg.String(),
		"GoString": cfg.GoString(),
		"%v":       fmt.Sprintf("%v", cfg),
		"%#v":      fmt.Sprintf("%#v", cfg),
	} {
		if !strings.Contains(text, "smtp.example.com") || !strings.Contains(text, "587") || !strings.Contains(text, "redacted") {
			t.Errorf("%s = %q, want the host, the port and the word redacted", what, text)
		}
	}
	m := &smtpMailer{cfg: &cfg}
	if text := m.String(); !strings.Contains(text, "smtp.example.com") || strings.Contains(text, testSMTPPassword) {
		t.Errorf("mailer String = %q", text)
	}
	if m.GoString() == "" {
		t.Error("mailer GoString is empty")
	}
}
