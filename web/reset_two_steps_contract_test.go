// Quick 261113-r5w pins the two steps of Forgot password on the sign in card:
// step one is the code box with the button Confirm code and no visible password
// box, step two (drawn when the server says the code was confirmed) is the new
// password box with Set new password, the confirmed lead, no code box and no
// Resend; sign up's code screen and its Create account button do not change. The
// tests render the real login_gate template and read auth.js out of the embedded
// filesystem.

package web

import (
	"regexp"
	"strings"
	"testing"
)

// r5wReset draws the code screen of Forgot password; verified is the step.
func r5wReset(t *testing.T, verified bool) string {
	t.Helper()
	vm := p711Vm()
	vm["StartPanel"] = "code"
	vm["CodeMode"] = "reset"
	vm["CodeEmail"] = "ravi@example.com"
	vm["CodeSecondsLeft"] = 300
	vm["CodeVerified"] = verified
	return p711RenderVM(t, vm)
}

func r5wHidden(t *testing.T, page, id string) bool {
	t.Helper()
	return regexp.MustCompile(`\shidden[\s>=]`).MatchString(p711Tag(t, page, id))
}

func r5wButtonText(t *testing.T, page, id string) string {
	t.Helper()
	m := regexp.MustCompile(`id="` + regexp.QuoteMeta(id) + `"[^>]*>([^<]*)<`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no button text found for #%s", id)
	}
	return m[1]
}

func TestResetStepOneMarkup(t *testing.T) {
	page := r5wReset(t, false)
	if got := r5wButtonText(t, page, "code-submit"); got != "Confirm code" {
		t.Errorf("step one button = %q, want Confirm code", got)
	}
	if !r5wHidden(t, page, "code-new-wrap") {
		t.Errorf("step one shows the new password box")
	}
	if !r5wHidden(t, page, "code-lead-confirmed") {
		t.Errorf("step one shows the confirmed lead")
	}
	for _, id := range []string{"code-lead", "code-entry-wrap", "code-resend"} {
		if r5wHidden(t, page, id) {
			t.Errorf("step one hides #%s", id)
		}
	}
	if !strings.Contains(p711Tag(t, page, "code-input"), " required") {
		t.Errorf("step one's code box is not required")
	}
	if strings.Contains(p711Tag(t, page, "code-new-password"), " required") {
		t.Errorf("a hidden empty required box blocks the browser's submit on step one")
	}
	if !strings.Contains(page, `data-code-step="code"`) || !strings.Contains(page, `data-code-mode="reset"`) {
		t.Errorf("the card does not name step one of a reset")
	}
	if !strings.Contains(page, `We emailed <span class="login-email" id="code-email">ravi@example.com</span>. Type the 6 digit code from that email.`) {
		t.Errorf("step one lost its lead")
	}
	if !strings.Contains(page, "Reset your password") {
		t.Errorf("the title of a reset is gone")
	}
	// Step two's markup exists on the page, so the script can show it.
	for _, id := range []string{"code-new-password", "code-new-show", "code-lead-confirmed", "code-entry-wrap"} {
		p711Tag(t, page, id)
	}
	if !strings.Contains(page, "Code confirmed. Choose a new password.") {
		t.Errorf("the confirmed lead is not in the page")
	}
	if strings.Contains(page, ">Set new password<") {
		t.Errorf("step one draws the Set new password button")
	}
}

func TestResetStepTwoMarkup(t *testing.T) {
	page := r5wReset(t, true)
	if got := r5wButtonText(t, page, "code-submit"); got != "Set new password" {
		t.Errorf("step two button = %q, want Set new password", got)
	}
	for _, id := range []string{"code-lead", "code-entry-wrap", "code-resend"} {
		if !r5wHidden(t, page, id) {
			t.Errorf("step two does not hide #%s", id)
		}
	}
	for _, id := range []string{"code-new-wrap", "code-lead-confirmed", "code-change-email"} {
		if r5wHidden(t, page, id) {
			t.Errorf("step two hides #%s", id)
		}
	}
	if strings.Contains(p711Tag(t, page, "code-input"), " required") {
		t.Errorf("step two leaves a hidden empty required code box, which blocks the submit")
	}
	if !strings.Contains(p711Tag(t, page, "code-new-password"), " required") {
		t.Errorf("step two's new password box is not required")
	}
	if !strings.Contains(page, `data-code-step="password"`) {
		t.Errorf("the card does not name step two")
	}
	if !strings.Contains(page, "At least 8 characters.") {
		t.Errorf("the rule line of the new password is missing")
	}
	// The address stays page text of the hidden step one lead, for the script.
	if !strings.Contains(page, `id="code-email">ravi@example.com</span>`) {
		t.Errorf("step two lost the address page text the script reads")
	}
	if strings.Contains(page, `value="ravi@example.com"`) || strings.Contains(page, `="ravi@example.com"`) {
		t.Errorf("the address is in an attribute")
	}
}

// TestResetSignUpCodeScreenUnchanged: sign up's code screen is exactly one step.
func TestResetSignUpCodeScreenUnchanged(t *testing.T) {
	vm := p711Vm()
	vm["StartPanel"] = "code"
	vm["CodeMode"] = "create"
	vm["CodeEmail"] = "ravi@example.com"
	page := p711RenderVM(t, vm)
	if got := r5wButtonText(t, page, "code-submit"); got != "Create account" {
		t.Errorf("sign up button = %q, want Create account", got)
	}
	if !r5wHidden(t, page, "code-new-wrap") || !r5wHidden(t, page, "code-lead-confirmed") {
		t.Errorf("sign up's code screen shows a part of step two")
	}
	if r5wHidden(t, page, "code-resend") || r5wHidden(t, page, "code-entry-wrap") {
		t.Errorf("sign up's code screen hides its code box or Resend")
	}
	if !strings.Contains(page, `data-code-step="code"`) || !strings.Contains(page, "Check your email") {
		t.Errorf("sign up's code screen lost its step or its title")
	}
	// A fresh visit of Create account draws no step two part visible either.
	fresh := p711RenderVM(t, p711Vm())
	if !r5wHidden(t, fresh, "code-new-wrap") || !r5wHidden(t, fresh, "code-lead-confirmed") {
		t.Errorf("a fresh page shows a part of step two")
	}
}

// TestResetStepsAuthJS: the script posts the code and the password to their own
// routes, in that order, and keeps only the visible step's field required.
func TestResetStepsAuthJS(t *testing.T) {
	script := readAuthJS(t)
	for _, want := range []string{
		"'/api/auth/reset/code'", "'/api/auth/reset/password'", "'/api/auth/sign-up/code'",
		"function enterPasswordStep", "function backToCodeStep", "data-code-step",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("auth.js lacks %s", want)
		}
	}
	// The old single post carried both fields; no payload names the two together.
	if regexp.MustCompile(`code:\s*codeInput\.value\s*,\s*password:`).MatchString(script) {
		t.Errorf("auth.js still posts the code and the password together")
	}
	// Every id the new steps use is in the template exactly once.
	page := readLoginGateTemplate(t)
	for _, id := range []string{"code-lead", "code-lead-confirmed", "code-entry-wrap"} {
		if n := strings.Count(page, `id="`+id+`"`); n != 1 {
			t.Errorf("#%s appears %d times in the template, want 1", id, n)
		}
		if !strings.Contains(script, "byId('"+id+"')") {
			t.Errorf("auth.js never looks up #%s", id)
		}
	}
	// No dash in the new copy (house rule).
	for _, s := range []string{"Code confirmed. Choose a new password.", "Confirm code", "Set new password"} {
		if strings.ContainsRune(s, rune(0x2013)) || strings.ContainsRune(s, rune(0x2014)) {
			t.Errorf("%q holds a dash", s)
		}
	}
}

// TestResetResendHiddenGuard: the shared .btn sets a display, which would beat
// the hidden attribute on Resend, so auth.css names the hidden state.
func TestResetResendHiddenGuard(t *testing.T) {
	css := p421ReadStatic(t, "css/auth.css")
	if !regexp.MustCompile(`\.login-actions\s+\.btn\[hidden\]\s*\{\s*display:\s*none;`).MatchString(css) {
		t.Errorf("auth.css has no rule hiding a .btn with the hidden attribute inside .login-actions")
	}
}
