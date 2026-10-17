package web

import (
	"strings"
	"testing"
)

// TestBarTokenFollowsTheRealBar pins review finding WR-W05 (Phase 4 web
// review) as Phase 3.4 reworked it. At enlarged text, or on a narrower laptop,
// the notice strip wraps and grows, and every shell that subtracts
// --disclaimer-bar-height would then overflow the screen. theme.js, the one
// script every page loads in its head, writes the strip's measured height into
// the token:
//
//   - once the document is parsed (readystatechange, before deferred scripts),
//   - again when the strip changes size (ResizeObserver),
//   - and on window resize.
//
// and writes 0px when there is no strip on the page or it is not displayed.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
// The strip's own floor is the 48px space token and never reads the bar
// token, so a measurement no longer drops the written value first (the old
// grow-but-never-shrink hazard has no subject), and the strip itself is
// observed because writing the token resizes the shells, never the strip.
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
	zero := strings.Index(measure, "setProperty(BAR_PROPERTY, '0px')")
	read := strings.Index(measure, "getBoundingClientRect()")
	set := strings.LastIndex(measure, "setProperty(BAR_PROPERTY,")
	// The last write takes the strip's measured rectangle as its value, so it
	// starts before the read in the text and reads inside its own argument.
	if zero < 0 || read < 0 || set < 0 || !(zero < set && set < read) {
		t.Errorf("measureBar must write 0px when there is no displayed strip, then write the strip's measured height, in that order: %q", measure)
	}
	if !strings.Contains(measure, "offsetHeight === 0") {
		t.Errorf("measureBar must treat a strip that is not displayed (offsetHeight 0) as no strip: %q", measure)
	}
	if !strings.Contains(measure, "document.documentElement.style") {
		t.Errorf("measureBar must write the token on the root, where every reader inherits it: %q", measure)
	}
	if strings.Contains(measure, "removeProperty") {
		t.Errorf("measureBar must not drop the written value; the strip does not read the token: %q", measure)
	}

	// Amended 2026-10-17 by Phase 3.4 plan 21 (plan 17 finding 4): the strip and
	// the shell are both direct children of the body, so they sit at the same
	// depth. Writing the token inside the strip's own ResizeObserver callback
	// resized the shell (which sheet.js observes) in the same delivery; the
	// browser cannot deliver an observation at the same depth in that pass and
	// raised "ResizeObserver loop completed with undelivered notifications" at
	// 150 and 200 percent text. The observer now only schedules the write for the
	// next animation frame (the old pin was
	// "new ResizeObserver(measureBar).observe(barEl)"); the first measurement and
	// the window resize listener still write at once.
	watch := jsFunctionBody(t, code, "watchBar()")
	for _, want := range []string{
		"querySelector('.notice-strip')",
		"new ResizeObserver(scheduleMeasure).observe(barEl)",
		"window.addEventListener('resize', measureBar)",
	} {
		if !strings.Contains(watch, want) {
			t.Errorf("watchBar lacks %q: %q", want, watch)
		}
	}
	if strings.Contains(watch, "new ResizeObserver(measureBar)") {
		t.Errorf("watchBar hands measureBar straight to the observer; the write must wait for the next frame (plan 21): %q", watch)
	}
	schedule := jsFunctionBody(t, code, "scheduleMeasure()")
	if !strings.Contains(schedule, "requestAnimationFrame(") || !strings.Contains(schedule, "measureBar()") {
		t.Errorf("scheduleMeasure must call measureBar from a requestAnimationFrame callback: %q", schedule)
	}
	if strings.Contains(schedule, "setProperty") || strings.Contains(schedule, "getBoundingClientRect") {
		t.Errorf("scheduleMeasure runs inside the observer callback and must not read or write layout itself: %q", schedule)
	}
	if strings.Contains(code, "querySelector('.disclaimer-bar')") {
		t.Errorf("theme.js still looks for the Phase 4 bar")
	}
	if !strings.Contains(code, "document.addEventListener('readystatechange', onReadyState)") {
		t.Errorf("theme.js must measure the strip on readystatechange, before the deferred scripts lay out")
	}

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
	// D-31): see the note above. The strip's own floor is the 48px space token,
	// so no rule of chrome.css uses the bar token as its own min-height.
	chrome := stripCSSComments(p16ReadStatic(t, "static/css/chrome.css"))
	if strings.Contains(chrome, "min-height: var(--disclaimer-bar-height);") {
		t.Errorf("chrome.css: nothing may use the bar token as its own floor")
	}
}
