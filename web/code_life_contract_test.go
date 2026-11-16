// Quick 261116-i0n pins the line under the code box: "The code works for N
// minutes." It sits inside #code-entry-wrap (so Forgot password step two hides it
// with the code box), takes its number from the server's view model, describes
// #code-input, and appears on neither the Create account nor the Sign in panel.

package web

import (
	"regexp"
	"strings"
	"testing"
)

// i0nPanel returns the markup of one panel section of the card, from its opening
// tag to the next top level panel (or the end of the page).
func i0nPanel(t *testing.T, page, id string) string {
	t.Helper()
	start := strings.Index(page, `<section id="`+id+`"`)
	if start < 0 {
		t.Fatalf("no panel #%s", id)
	}
	rest := page[start+1:]
	if end := strings.Index(rest, `<section id="`); end >= 0 {
		return page[start : start+1+end]
	}
	return page[start:]
}

// i0nWrap returns #code-entry-wrap up to the start of the next block, #code-new-wrap.
func i0nWrap(page string) string {
	return regexp.MustCompile(`(?s)<div id="code-entry-wrap".*?<div id="code-new-wrap"`).FindString(page)
}

func TestCodeLifeLineIsUnderTheCodeBox(t *testing.T) {
	for _, mode := range []string{"create", "reset"} {
		vm := p711Vm()
		vm["StartPanel"] = "code"
		vm["CodeMode"] = mode
		vm["CodeEmail"] = "i0n.tester@example.com"
		vm["CodeMinutes"] = 7
		page := p711RenderVM(t, vm)

		wrap := i0nWrap(page)
		if wrap == "" {
			t.Fatalf("%s: no #code-entry-wrap block found", mode)
		}
		line := `<p class="login-rule" id="code-life">The code works for 7 minutes.</p>`
		if !strings.Contains(wrap, line) {
			t.Errorf("%s: #code-entry-wrap does not hold %q:\n%s", mode, line, wrap)
		}
		if strings.Index(wrap, `login-field--code`) > strings.Index(wrap, `id="code-life"`) {
			t.Errorf("%s: the line is not under the code box", mode)
		}
		if n := strings.Count(page, `id="code-life"`); n != 1 {
			t.Errorf("%s: id=code-life appears %d times, want 1", mode, n)
		}
		if tag := p711Tag(t, page, "code-input"); !strings.Contains(tag, `aria-describedby="code-life"`) {
			t.Errorf("%s: #code-input is not described by #code-life: %s", mode, tag)
		}
		for _, panel := range []string{"auth-panel-create", "auth-panel-signin", "auth-panel-forgot"} {
			if strings.Contains(i0nPanel(t, page, panel), "The code works for") {
				t.Errorf("%s: the line is in #%s", mode, panel)
			}
		}
	}
}

// The line is inside the wrapper Forgot password step two hides, and it is only
// on step one: with CodeVerified the wrapper is hidden and no new text appears
// outside it.
func TestCodeLifeLineHidesWithTheCodeBoxOnStepTwo(t *testing.T) {
	vm := p711Vm()
	vm["StartPanel"] = "code"
	vm["CodeMode"] = "reset"
	vm["CodeVerified"] = true
	page := p711RenderVM(t, vm)
	if !r5wHidden(t, page, "code-entry-wrap") {
		t.Fatal("step two shows #code-entry-wrap")
	}
	if n := strings.Count(page, "The code works for"); n != 1 {
		t.Fatalf("the line appears %d times, want 1 (inside the hidden wrapper)", n)
	}
	wrap := i0nWrap(page)
	if !strings.Contains(wrap, "The code works for") {
		t.Error("the line is outside #code-entry-wrap, so step two would show it")
	}
}

func TestCodeLifeLineHasNoDashes(t *testing.T) {
	vm := p711Vm()
	vm["StartPanel"] = "code"
	page := p711RenderVM(t, vm)
	m := regexp.MustCompile(`id="code-life">([^<]*)<`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no #code-life line")
	}
	if strings.ContainsAny(m[1], "\u2013\u2014") {
		t.Errorf("the line has a dash: %q", m[1])
	}
	if m[1] != "The code works for 5 minutes." {
		t.Errorf("line = %q", m[1])
	}
}
