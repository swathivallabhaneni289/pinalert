package mailer_test

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"pinalert/internal/mailer"
)

func TestNewLogMailerSatisfiesInterface(t *testing.T) {
	var m mailer.Mailer = mailer.NewLogMailer()

	if err := m.SendAuthEmail(context.Background(), "visitor@example.com", mailer.AuthEmail{Kind: mailer.KindAccountExists}); err != nil {
		t.Fatalf("SendAuthEmail returned %v, want nil", err)
	}
}

// TestLogMailerPrintsCodeOnly checks the dev log line carries the kind and the
// code, and neither the address nor any link.
func TestLogMailerPrintsCodeOnly(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	m := mailer.NewLogMailer()
	err := m.SendAuthEmail(context.Background(), "visitor@example.com",
		mailer.AuthEmail{Kind: mailer.KindCodeSignup, Code: "482913", MinutesValid: 10})
	if err != nil {
		t.Fatalf("SendAuthEmail returned %v, want nil", err)
	}
	out := buf.String()
	if !strings.Contains(out, "kind=code_signup") || !strings.Contains(out, "code=482913") {
		t.Errorf("log line %q misses the kind or the code", out)
	}
	for _, bad := range []string{"visitor@example.com", "http"} {
		if strings.Contains(out, bad) {
			t.Errorf("log line %q holds %q", out, bad)
		}
	}
	if strings.Count(strings.TrimSpace(out), "\n") != 0 {
		t.Errorf("want one log line, got %q", out)
	}
}
