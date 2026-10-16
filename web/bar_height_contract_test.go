package web

import (
	"strings"
	"testing"
)

// TestBarTokenFollowsTheRealBar pins review finding WR-W05 (Phase 4 web
// review). At enlarged text the sticky unofficial bar grows past the CSS
// tier of --disclaimer-bar-height, and every shell that subtracts the token
// then overflows the screen. theme.js, the one script every page with the
// bar loads in its head, writes the bar's measured height into the token:
//
//   - once the document is parsed (readystatechange, before deferred scripts),
//   - again when the bar or its text box changes size (ResizeObserver),
//   - and on window resize (a width tier can lower the floor on its own).
//
// The bar's min-height reads the same token, so each measurement first
// drops the written value; without that the bar could never shrink back.
// The CSS tiers in main.css stay the first paint and no script value; the
// tests in chrome_css_contract_test.go keep pinning them unchanged.
func TestBarTokenFollowsTheRealBar(t *testing.T) {
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading %s: %v", themeJSPath, err)
	}
	code := stripJSComments(string(raw))

	if !strings.Contains(code, "'--disclaimer-bar-height'") {
		t.Errorf("theme.js does not name the bar token")
	}
	measure := jsFunctionBody(t, code, "measureBar()")
	remove := strings.Index(measure, "removeProperty(BAR_PROPERTY)")
	read := strings.Index(measure, "getBoundingClientRect()")
	set := strings.Index(measure, "setProperty(BAR_PROPERTY,")
	if remove < 0 || read < 0 || set < 0 || !(remove < read && read < set) {
		t.Errorf("measureBar must drop the written value, read the bar, then write it, in that order: %q", measure)
	}
	if !strings.Contains(measure, "document.documentElement.style") {
		t.Errorf("measureBar must write the token on the root, where every reader inherits it: %q", measure)
	}

	watch := jsFunctionBody(t, code, "watchBar()")
	for _, want := range []string{
		"querySelector('.disclaimer-bar')",
		"new ResizeObserver(measureBar)",
		"observer.observe(barEl)",
		"observer.observe(barEl.firstElementChild)",
		"window.addEventListener('resize', measureBar)",
	} {
		if !strings.Contains(watch, want) {
			t.Errorf("watchBar lacks %q: %q", want, watch)
		}
	}
	if !strings.Contains(code, "document.addEventListener('readystatechange', onReadyState)") {
		t.Errorf("theme.js must measure the bar on readystatechange, before the deferred scripts lay out")
	}

	// The CSS floor the script overrides is still the bar's min-height, so
	// the first paint and a page without scripts keep the tier value.
	chrome := stripCSSComments(p16ReadStatic(t, "static/css/chrome.css"))
	if !strings.Contains(chrome, "min-height: var(--disclaimer-bar-height);") {
		t.Errorf("chrome.css .disclaimer-bar must keep min-height: var(--disclaimer-bar-height)")
	}
}
