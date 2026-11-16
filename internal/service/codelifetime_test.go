package service

import (
	"testing"
	"time"

	"pinalert/internal/mailer"
)

// TestCodeLifetimeIsOneNumber pins IN-10: the lifetime of an emailed code lives
// in mailer.CodeLifetime, which mailcheck reads for its sample email without
// importing this package. The service's CodeTTL (what it enforces, and what the
// email words say) must equal it, and both are the 5 minutes the page and the
// emails promise.
func TestCodeLifetimeIsOneNumber(t *testing.T) {
	if DefaultCredentialLimits.CodeTTL != mailer.CodeLifetime {
		t.Errorf("DefaultCredentialLimits.CodeTTL = %v, mailer.CodeLifetime = %v; they must be the same number", DefaultCredentialLimits.CodeTTL, mailer.CodeLifetime)
	}
	if mailer.CodeLifetime != 5*time.Minute {
		t.Errorf("mailer.CodeLifetime = %v, want 5m: the sign in page and the emails say 5 minutes", mailer.CodeLifetime)
	}
	if got := minutesValid(DefaultCredentialLimits.CodeTTL); got != 5 {
		t.Errorf("the email says %d minutes, want 5", got)
	}
}
