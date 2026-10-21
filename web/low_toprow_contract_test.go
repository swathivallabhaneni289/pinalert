package web

import (
	"regexp"
	"strings"
	"testing"
)

// Amended 2026-10-21 by plan 44 (owner). The owner asked for fewer buttons on
// the Low data view: a slim top row with Back (to the full version) at the
// left and one Reload at the right. Removed from the page: the Open full
// version button (now Back), the Retry position button (now Reload), the Retry
// now button of the stale line and the Retry button of the error block. The
// moved assertions are in low_contract_test.go (copy and hidden ids) and
// internal/api/low_routes_test.go; this file pins what the page has now and
// that none of the removed buttons came back.
func TestLowDataTopRow(t *testing.T) {
	html := p418Template(t)

	back := findTagWindow(t, html, `id="low-full"`)
	for _, want := range []string{`<a `, `href="/?full=1"`, `aria-label="Back to the full version"`, `class="low-topbar__btn"`} {
		if !strings.Contains(back, want) {
			t.Errorf("the Back link is missing %q: %s", want, back)
		}
	}
	reload := findTagWindow(t, html, `id="low-reload"`)
	for _, want := range []string{`<button `, `type="button"`, `aria-label="Reload"`, `low-topbar__btn`} {
		if !strings.Contains(reload, want) {
			t.Errorf("the Reload button is missing %q: %s", want, reload)
		}
	}
	if strings.Contains(reload, "hidden") {
		t.Errorf("Reload must always be shown: %s", reload)
	}

	// Back leads the row and Reload ends it, both before the page head.
	backAt, reloadAt, headAt := strings.Index(html, `id="low-full"`), strings.Index(html, `id="low-reload"`), strings.Index(html, `class="low-page__head"`)
	if !(backAt != -1 && backAt < reloadAt && reloadAt < headAt) {
		t.Errorf("the top row must hold Back then Reload before the head (%d, %d, %d)", backAt, reloadAt, headAt)
	}

	// The page head holds no retry button and no second way to the full version.
	pageEnd := strings.Index(html, `</main>`)
	if pageEnd < 0 {
		t.Fatalf("no </main> in the template")
	}
	page := html[:pageEnd]
	for _, gone := range []string{`id="low-retry-position"`, `id="low-stale-retry"`, `id="low-retry"`, "Open full version", "Retry position", "Retry now", ">Retry<"} {
		if strings.Contains(page, gone) {
			t.Errorf("the Low data page still holds %q", gone)
		}
	}
	if n := len(regexp.MustCompile(`<button\b`).FindAllString(page, -1)); n != 2 {
		// Reload and Report an emergency, nothing else.
		t.Errorf("the Low data page has %d buttons outside the report dialog, want 2 (Reload, Report an emergency)", n)
	}

	// Both controls are 44px targets.
	rule := p31mRule(t, "static/css/low.css", "", ".low-topbar__btn:not([hidden])")
	p16MustDecl(t, ".low-topbar__btn", rule, map[string]string{
		"min-width":  "var(--touch-target-min)",
		"min-height": "var(--touch-target-min)",
	})

	// low.js wires Reload and no longer looks up the removed buttons.
	js := p418Static(t, "js/low.js")
	if !strings.Contains(js, `getElementById('low-reload')`) {
		t.Errorf("low.js does not look up #low-reload")
	}
	for _, gone := range []string{"low-retry-position", "low-stale-retry", "'low-retry'"} {
		if strings.Contains(js, gone) {
			t.Errorf("low.js still looks up %s", gone)
		}
	}
}
