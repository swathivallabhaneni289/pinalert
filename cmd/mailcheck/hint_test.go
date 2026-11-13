package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"pinalert/internal/mailer"
)

const keyHint = "set RESEND_API_KEY (RESEND_FROM is optional) in this shell"

// TestSettingsHintOnlyForTheMissingKey pins that the line "set RESEND_API_KEY"
// follows a load error only when the key is the problem (errors.Is with
// mailer.ErrMissingAPIKey). A load error of any other kind gets the loader's
// own line and no hint, and a send failure gets no hint either.
func TestSettingsHintOnlyForTheMissingKey(t *testing.T) {
	t.Run("the key is missing", func(t *testing.T) {
		clearResendEnv(t)
		var out, errOut bytes.Buffer
		code := run([]string{"-to", checkTo}, &out, &errOut, loadProduction)
		if code != 1 {
			t.Fatalf("exit code %d, want 1; stderr %q", code, errOut.String())
		}
		if !strings.Contains(errOut.String(), "RESEND_API_KEY") {
			t.Errorf("stderr %q does not name RESEND_API_KEY", errOut.String())
		}
		if !strings.Contains(errOut.String(), keyHint) {
			t.Errorf("stderr %q lacks the hint", errOut.String())
		}
	})

	t.Run("a load error of another kind", func(t *testing.T) {
		clearResendEnv(t)
		load := func() (mailer.Mailer, error) { return nil, errors.New("mailer: something else is wrong") }
		var out, errOut bytes.Buffer
		code := run([]string{"-to", checkTo}, &out, &errOut, load)
		if code != 1 {
			t.Fatalf("exit code %d, want 1; stderr %q", code, errOut.String())
		}
		if strings.Contains(errOut.String(), keyHint) {
			t.Errorf("stderr %q shows the key hint for another kind of error", errOut.String())
		}
	})

	t.Run("a send failure", func(t *testing.T) {
		clearResendEnv(t)
		fake := &recordingMailer{err: errors.New("mailer: sending auth email via resend: could not reach resend")}
		var out, errOut bytes.Buffer
		code := run([]string{"-to", checkTo}, &out, &errOut, fakeLoad(fake))
		if code != 1 {
			t.Fatalf("exit code %d, want 1; stderr %q", code, errOut.String())
		}
		if strings.Contains(errOut.String(), keyHint) {
			t.Errorf("stderr %q shows the key hint for a send failure", errOut.String())
		}
		assertNoSecrets(t, "stderr", errOut.String())
	})
}
