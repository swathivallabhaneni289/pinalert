package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestMailcheckStaysSmall pins IN-10: the command that "touches no database"
// must not link the service package (pgx, sqlc, push, moderation, geocode and
// areas with it), nor the fake server or the testing package. The lifetime of a
// code comes from mailer.CodeLifetime instead.
func TestMailcheckStaysSmall(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	deps := strings.Fields(string(out))
	if len(deps) < 20 {
		t.Fatalf("go list -deps listed only %d packages; the check would prove nothing", len(deps))
	}
	have := map[string]bool{}
	for _, d := range deps {
		have[d] = true
	}
	if !have["pinalert/internal/mailer"] {
		t.Error("mailcheck no longer depends on internal/mailer; the check is looking at the wrong package")
	}
	for _, banned := range []string{
		"pinalert/internal/service",
		"pinalert/internal/store",
		"pinalert/internal/mailer/smtptest",
		"github.com/jackc/pgx/v5",
		"testing",
	} {
		if have[banned] {
			t.Errorf("mailcheck depends on %s", banned)
		}
	}
}
