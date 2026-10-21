package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoUnderlineAtRest pins plan 42 (owner, 2026-10-21: "make it either
// buttons or like design it in a way that doesn't look underlined"). No rule
// in any stylesheet may draw an underline at rest: an underline is allowed only
// on a selector that is hovered or keyboard focused (the in-text links).
func TestNoUnderlineAtRest(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("static", "css", "*.css"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no stylesheets found: %v", err)
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range parseCSSRules(string(raw)) {
			decls := declsOf(r.declBody)
			underline := false
			for _, prop := range []string{"text-decoration", "text-decoration-line"} {
				if strings.Contains(decls[prop], "underline") {
					underline = true
				}
			}
			if !underline {
				continue
			}
			for _, sel := range p408Selectors(r.selectorHead) {
				if !strings.Contains(sel, ":hover") && !strings.Contains(sel, ":focus-visible") {
					t.Errorf("%s: %q draws an underline at rest", filepath.Base(path), sel)
				}
			}
		}
	}
}

// TestFormerUnderlinesAreButtons pins the new look of the links and text
// buttons plan 42 turned into the outlined secondary button: page colour, the
// hairline, 8px corners and no underline.
func TestFormerUnderlinesAreButtons(t *testing.T) {
	cases := []struct{ file, head string }{
		{"static/css/feed.css", ".inline-text-button"},
		{"static/css/legal.css", ".legal-parts a:not([hidden])"},
		{"static/css/window.css", ".app-window__quiet"},
		{"static/css/moderate.css", ".mod-row__link:not([hidden])"},
	}
	for _, c := range cases {
		rule, ok := ruleBySelector(parseCSSRules(p408ReadStatic(t, c.file)), c.head)
		if !ok {
			t.Errorf("%s has no %s rule", c.file, c.head)
			continue
		}
		got := declsOf(rule.declBody)
		for prop, want := range map[string]string{
			"border": "1px solid var(--color-border)", "border-radius": "8px",
			"background": "var(--color-bg)", "text-decoration": "none",
			"min-height": "var(--touch-target-min)",
		} {
			if got[prop] != want {
				t.Errorf("%s %s: %s = %q, want %q", c.file, c.head, prop, got[prop], want)
			}
		}
	}

	// The current part of the reader takes the chips' chosen look (D-04): the
	// soft surface fill and no outline, never an underline or a dark outline.
	rule, ok := ruleBySelector(parseCSSRules(p408ReadStatic(t, "static/css/legal.css")), `.legal-parts a[aria-current="page"]`)
	if !ok {
		t.Fatal("legal.css has no current part rule")
	}
	got := declsOf(rule.declBody)
	for prop, want := range map[string]string{
		"background": "var(--color-surface)", "border-color": "transparent",
		"font-weight": "var(--font-weight-semibold)", "text-decoration": "none",
	} {
		if got[prop] != want {
			t.Errorf("current reader part: %s = %q, want %q", prop, got[prop], want)
		}
	}

	// An a.btn is never underlined by the browser.
	btn, ok := ruleBySelector(parseCSSRules(p408ReadStatic(t, "static/css/main.css")), ".btn")
	if !ok || declsOf(btn.declBody)["text-decoration"] != "none" {
		t.Errorf("main.css .btn must set text-decoration: none so an a.btn is not underlined")
	}
}

// TestPhoneVotesOutlinedAtRest pins the owner's 2026-10-21 change (plan 42):
// on the phone Confirm is no longer filled at rest. Confirm and Dispute both
// rest on the page colour with the hairline; only a pressed vote is filled.
func TestPhoneVotesOutlinedAtRest(t *testing.T) {
	rules := parseCSSRules(p408ReadStatic(t, "static/css/phone.css"))
	for _, mod := range []string{"confirm", "dispute"} {
		head := ".leaflet-container .leaflet-detail-pane .vote-btn--" + mod + `:not([hidden]):not([aria-pressed="true"]):not([aria-disabled="true"]):not(:disabled)`
		rule, ok := ruleBySelector(rules, head)
		if !ok {
			t.Errorf("phone.css has no at rest rule for %s", mod)
			continue
		}
		got := declsOf(rule.declBody)
		for prop, want := range map[string]string{
			"background": "var(--color-bg)", "color": "var(--color-text)", "border-color": "var(--color-border)",
		} {
			if got[prop] != want {
				t.Errorf("phone %s at rest: %s = %q, want %q", mod, prop, got[prop], want)
			}
		}
	}
	for _, r := range rules {
		if !strings.Contains(r.selectorHead, `.vote-btn--confirm:not([hidden]):not([aria-pressed="true"])`) {
			continue
		}
		if strings.Contains(declsOf(r.declBody)["background"], "--color-text") {
			t.Errorf("phone.css %q fills an unpressed Confirm with the text colour", r.selectorHead)
		}
	}
}
