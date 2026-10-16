package web

import (
	"strings"
	"testing"
)

// The Phase 4 browser harness (run outside the repo on 2026-10-16) measured
// these problems in Chromium and WebKit. Each test pins the fix of one finding
// so the source cannot drift back while the harness is not run. F-P4-5 is
// pinned in TestPhoneDetailSheetCSS (phone_layout_contract_test.go).

// TestHarnessFindingP4_6NoThemeToggleOnAnyPhonePage pins F-P4-6: the toggle
// showed on Low data and Moderate under 900px because those pages do not load
// phone.css, and a press there saved a choice over the laptop one. The hide
// rule lives in chrome.css, which every page that includes the toggle links,
// the laptop toggle keeps its look, and the click handler does nothing on a
// phone width.
func TestHarnessFindingP4_6NoThemeToggleOnAnyPhonePage(t *testing.T) {
	const include = `{{template "theme_toggle.html.tmpl" .}}`
	pages := 0
	for path, text := range findFullPageTemplates(t) {
		if !strings.Contains(text, include) {
			continue
		}
		pages++
		if !strings.Contains(text, "/static/css/chrome.css") {
			t.Errorf("%s includes the theme toggle but does not link chrome.css, which hides it under 900px", path)
		}
		// The hide rule's head is body > .theme-toggle, so the include must be
		// a direct child of body (two space indent, outside .top-chrome).
		if !strings.Contains(text, "\n  "+include+"\n") {
			t.Errorf("%s: the theme toggle include is not a direct child of body, so body > .theme-toggle matches nothing", path)
		}
	}
	if pages < 4 {
		t.Errorf("only %d pages include the theme toggle, want the map, Activity, Low data and Moderate pages", pages)
	}

	hide := p31mRule(t, "static/css/chrome.css", "@media (max-width: 899.98px)", "body > .theme-toggle")
	if hide.Decls["display"] != "none" {
		t.Errorf("chrome.css body > .theme-toggle under 900px declares display %q, want none", hide.Decls["display"])
	}
	for _, r := range p16ParseCSS("static/css/chrome.css", p16ReadStatic(t, "static/css/chrome.css")) {
		if strings.Contains(r.Head, ".theme-toggle") && r.Media != "@media (max-width: 899.98px)" {
			t.Errorf("chrome.css touches the theme toggle outside the phone block (%q in %q); the laptop toggle stays as it is", r.Head, r.Media)
		}
	}
	laptop := p31mRule(t, "static/css/auth.css", "", ".theme-toggle")
	p16MustDecl(t, "laptop theme toggle", laptop, map[string]string{
		"position": "fixed", "display": "flex", "right": "var(--space-lg)", "bottom": "var(--space-lg)",
	})

	code := p19Code(t, "static/js/theme.js")
	const head = "control.addEventListener('click', function () {"
	if n := strings.Count(code, head); n != 1 {
		t.Fatalf("theme.js has %d toggle click handlers, want 1", n)
	}
	body := code[strings.Index(code, head):]
	end := strings.Index(body, "});")
	if end == -1 {
		t.Fatal("theme.js toggle click handler has no end")
	}
	body = body[:end]
	p13Ordered(t, body, "theme toggle click handler",
		"if (isPhone()) {", "return;", "currentMode = MODES[nextIndex];", "persistMode(currentMode);")
}
