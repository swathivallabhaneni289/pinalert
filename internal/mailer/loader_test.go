package mailer

import (
	"strings"
	"testing"
)

// TestLoadMailer exercises the four RESEND_API_KEY / ENV combinations
// LoadMailer's doc comment describes. t.Setenv is used for both variables
// so no test leaks environment state to another — and, critically, so this
// suite never touches the network: the RESEND_API_KEY-set case constructs a
// resendMailer but never calls SendVerificationLink on it.
func TestLoadMailer(t *testing.T) {
	t.Run("key set, env production returns resend mailer", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "test-key")
		t.Setenv("ENV", "production")

		m, err := LoadMailer("production")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		if _, ok := m.(*resendMailer); !ok {
			t.Fatalf("LoadMailer returned %T, want *resendMailer", m)
		}
	})

	t.Run("key set, env development still returns resend mailer", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "test-key")
		t.Setenv("ENV", "development")

		m, err := LoadMailer("development")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		if _, ok := m.(*resendMailer); !ok {
			t.Fatalf("LoadMailer returned %T, want *resendMailer — a set key wins regardless of ENV", m)
		}
	})

	t.Run("key unset, env development returns log mailer", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "")
		t.Setenv("ENV", "development")

		m, err := LoadMailer("development")
		if err != nil {
			t.Fatalf("LoadMailer returned error %v, want nil", err)
		}
		if _, ok := m.(logMailer); !ok {
			t.Fatalf("LoadMailer returned %T, want logMailer", m)
		}
	})

	t.Run("key unset, env empty returns error naming RESEND_API_KEY", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "")
		t.Setenv("ENV", "")

		m, err := LoadMailer("")
		if err == nil {
			t.Fatal("LoadMailer returned nil error, want an error naming RESEND_API_KEY")
		}
		if m != nil {
			t.Fatalf("LoadMailer returned non-nil Mailer %v alongside an error, want nil", m)
		}
		if got := err.Error(); !strings.Contains(got, "RESEND_API_KEY") {
			t.Fatalf("LoadMailer error %q does not name RESEND_API_KEY", got)
		}
	})

	t.Run("key unset, env production returns error naming RESEND_API_KEY", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "")
		t.Setenv("ENV", "production")

		m, err := LoadMailer("production")
		if err == nil {
			t.Fatal("LoadMailer returned nil error, want an error naming RESEND_API_KEY")
		}
		if m != nil {
			t.Fatalf("LoadMailer returned non-nil Mailer %v alongside an error, want nil", m)
		}
		if got := err.Error(); !strings.Contains(got, "RESEND_API_KEY") {
			t.Fatalf("LoadMailer error %q does not name RESEND_API_KEY", got)
		}
	})
}
