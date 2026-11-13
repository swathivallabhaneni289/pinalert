package mailer

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsRequiredSettingError pins IN-07 at the loader: the error is of the
// required setting kind for a missing or unusable SMTP_HOST, SMTP_USERNAME or
// SMTP_PASSWORD, and not for a bad SMTP_PORT or MAIL_FROM, whose text may name
// SMTP_USERNAME all the same.
func TestIsRequiredSettingError(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		set  func()
		want bool
	}{
		{"nothing set", "production", func() { setSMTPEnv(t, "", "", "", "", "") }, true},
		{"half set group", "production", func() { setSMTPEnv(t, "", "", "", "pw", "") }, true},
		{"no password", "production", func() { setSMTPEnv(t, "smtp.example.com", "", testSMTPUser, "", "") }, true},
		{"no user name", "production", func() { setSMTPEnv(t, "smtp.example.com", "", "", "pw", "") }, true},
		{"bad host", "production", func() { setSMTPEnv(t, "smtp.example.com:587", "", testSMTPUser, "pw", "") }, true},
		{"bad port", "production", func() { setSMTPEnv(t, "smtp.example.com", "abc", testSMTPUser, "pw", "") }, false},
		{"bad sender", "production", func() { setSMTPEnv(t, "smtp.example.com", "", testSMTPUser, "pw", "nope") }, false},
		{"user name that is no address", "production", func() { setSMTPEnv(t, "smtp.example.com", "", "nope", "pw", "") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.set()
			_, err := LoadMailer(tc.env)
			if err == nil {
				t.Fatal("LoadMailer returned no error")
			}
			if got := IsRequiredSettingError(err); got != tc.want {
				t.Errorf("IsRequiredSettingError(%q) = %v, want %v", err, got, tc.want)
			}
			if got := IsRequiredSettingError(fmt.Errorf("wrapped: %w", err)); got != tc.want {
				t.Errorf("a wrapped error: got %v, want %v", got, tc.want)
			}
		})
	}
	if IsRequiredSettingError(errors.New("some other error")) || IsRequiredSettingError(nil) {
		t.Error("an unrelated error or nil is not a required setting error")
	}
}
