package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestSettingsHintOnlyForTheThreeRequiredSettings pins IN-07: the line "set
// SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD" follows a load error only when
// one of those three is the problem. After a bad SMTP_PORT or MAIL_FROM the
// loader's own line is the whole answer, because the three are set.
func TestSettingsHintOnlyForTheThreeRequiredSettings(t *testing.T) {
	const hint = "set SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD"
	for _, tc := range []struct {
		name     string
		set      map[string]string
		wantHint bool
		names    string // the setting the loader's own line must name
	}{
		{"nothing set", nil, true, "SMTP_HOST"},
		{"only the password set", map[string]string{"SMTP_PASSWORD": checkPassword}, true, "SMTP_HOST"},
		{"host and user name, no password", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": checkUser}, true, "SMTP_PASSWORD"},
		{"host and password, no user name", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PASSWORD": checkPassword}, true, "SMTP_USERNAME"},
		{"a host with a port", map[string]string{"SMTP_HOST": "smtp.example.com:587", "SMTP_USERNAME": checkUser, "SMTP_PASSWORD": checkPassword}, true, "SMTP_HOST"},
		{"a bad port", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": checkUser, "SMTP_PASSWORD": checkPassword, "SMTP_PORT": "abc"}, false, "SMTP_PORT"},
		{"a bad sender", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": checkUser, "SMTP_PASSWORD": checkPassword, "MAIL_FROM": "not an address"}, false, "MAIL_FROM"},
		{"a user name that is no address, no sender", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": "not-an-email", "SMTP_PASSWORD": checkPassword}, false, "MAIL_FROM"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearSMTPEnv(t)
			for k, v := range tc.set {
				t.Setenv(k, v)
			}
			var out, errOut bytes.Buffer
			code := run([]string{"-to", checkTo}, &out, &errOut, loadProduction)
			if code != 1 {
				t.Fatalf("exit code %d, want 1; stderr %q", code, errOut.String())
			}
			if !strings.Contains(errOut.String(), tc.names) {
				t.Errorf("stderr %q does not name %s", errOut.String(), tc.names)
			}
			if got := strings.Contains(errOut.String(), hint); got != tc.wantHint {
				t.Errorf("hint shown = %v, want %v; stderr %q", got, tc.wantHint, errOut.String())
			}
			assertNoSecrets(t, "stderr", errOut.String(), "smtp.example.com", "not-an-email")
		})
	}
}
