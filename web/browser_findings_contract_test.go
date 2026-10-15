package web

import (
	"strings"
	"testing"
)

// The browser harness (outside the repo) measured four real problems in
// Chromium and WebKit after Phase 3.1. Each test below pins the fix of one of
// them, so the source cannot drift back while the harness is not run.

// TestBrowserFindingF4TrustToggleTakesFocusOnClick pins finding F4: WebKit
// does not focus a button on a click or a tap, so on the Activity page the
// open trust panel did not close on Escape (Escape looks for the row that
// holds focus). The shared toggle's own click handler, which only the
// Activity page mounts, gives the toggle focus after it opens or closes the
// panel. It adds no listener, so activity.js keeps its one listener.
func TestBrowserFindingF4TrustToggleTakesFocusOnClick(t *testing.T) {
	code := p19Code(t, "static/js/trust.js")
	const head = "toggle.addEventListener('click', function () {"
	if n := strings.Count(code, head); n != 1 {
		t.Fatalf("trust.js has %d toggle click handlers, want 1", n)
	}
	body := code[strings.Index(code, head):]
	end := strings.Index(body, "});")
	if end == -1 {
		t.Fatal("trust.js toggle click handler has no end")
	}
	body = body[:end]
	p13Ordered(t, body, "trust toggle click handler",
		"setPanelOpen(instance, toggle.getAttribute('aria-expanded') !== 'true');",
		"if (document.activeElement !== toggle) {",
		"toggle.focus();")

	// The Activity page is the only page that mounts the toggle, and its
	// script still has exactly one listener (A29).
	act := p16ReadStatic(t, "static/js/activity.js")
	if n := strings.Count(act, "addEventListener"); n != 1 {
		t.Errorf("activity.js has %d addEventListener occurrences, want 1", n)
	}
}

// TestBrowserFindingF3SearchStatusSitsUnderTheChips pins finding F3: on a
// laptop the place search refusal or failure line sat at the results' place
// under the box, on top of the chips row (60 to 104px), and took the presses
// meant for the chips until it cleared. The line now starts under the whole
// tools layer (its height token is two rows on a laptop), so it never covers a
// chip; the results list keeps its place under the box, and the phone keeps
// its own fixed place below the one row.
func TestBrowserFindingF3SearchStatusSitsUnderTheChips(t *testing.T) {
	const feed = "static/css/feed.css"
	status := p31mRule(t, feed, "", ".map-search__status:not(.visually-hidden)")
	p16MustDecl(t, "search status line", status, map[string]string{
		"position": "absolute", "z-index": "1005", "pointer-events": "auto",
		"font-size": "var(--font-size-label)",
	})
	if got := p19NoSpace(status.Decls["top"]); got != p19NoSpace("calc(var(--map-tools-height) + var(--space-xs))") {
		t.Errorf("search status line top is %q, want the tools height plus the small space", status.Decls["top"])
	}

	results := p31mRule(t, feed, "", ".map-search__results")
	if got := p19NoSpace(results.Decls["top"]); got != p19NoSpace("calc(var(--space-sm) + var(--touch-target-min) + var(--space-xs))") {
		t.Errorf("search results top is %q, want just under the box", results.Decls["top"])
	}

	// The chips row starts at 8 + 44 + 8 and is one touch target tall, which
	// is the laptop tools height, so a line under the tools clears it.
	chips := p31mRule(t, feed, "", ".map-chips")
	if got := p19NoSpace(chips.Decls["top"]); got != p19NoSpace("calc(var(--space-sm) + var(--touch-target-min) + var(--space-sm))") {
		t.Errorf("chips row top is %q", chips.Decls["top"])
	}
	laptop := p31mRule(t, "static/css/main.css", "@media (min-width: 900px)", ".app-shell > .pane--map")
	if got := p19NoSpace(laptop.Decls["--map-tools-height"]); got !=
		p19NoSpace("calc(var(--space-sm) + var(--touch-target-min) + var(--space-sm) + var(--touch-target-min))") {
		t.Errorf("laptop --map-tools-height is %q", laptop.Decls["--map-tools-height"])
	}
}

// browserFindingsPhoneMedia is the one narrow block of phone.css.
const browserFindingsPhoneMedia = "@media (max-width: 899.98px)"

// TestBrowserFindingF2PhoneCardIsDenser pins finding F2 (and the row half of
// F1): at 320px wide rows were 177 to 220px tall, over the 180px ceiling,
// because line one and the trust line wrapped. On a phone the card takes 8px
// above and below instead of 16px (its 16px sides stay), the parts of line
// one and of the trust line sit 4px apart instead of 8px, and the title
// follows line one by the 4px row gap alone. Nothing the row shows is hidden.
// The laptop row in feed.css is unchanged.
func TestBrowserFindingF2PhoneCardIsDenser(t *testing.T) {
	const phone = "static/css/phone.css"
	const feed = "static/css/feed.css"
	m := browserFindingsPhoneMedia

	p16MustDecl(t, "phone card", p31mRule(t, phone, m, "#report-list .report-row"),
		map[string]string{"padding": "var(--space-sm) var(--space-md)"})
	p16MustDecl(t, "phone white card", p31mRule(t, phone, m, "#report-list .report-row.vis-provisional"),
		map[string]string{"padding": "calc(var(--space-sm) - 1px) calc(var(--space-md) - 1px)"})
	p16MustDecl(t, "phone row body", p31mRule(t, phone, m, "#report-list .report-row .report-row__body"),
		map[string]string{"column-gap": "var(--space-xs)"})
	p16MustDecl(t, "phone row title", p31mRule(t, phone, m, "#report-list .report-row .report-row__title"),
		map[string]string{"margin": "0"})
	p16MustDecl(t, "phone trust line", p31mRule(t, phone, m, "#report-list .report-row .report-row__trustline:not([hidden])"),
		map[string]string{"column-gap": "var(--space-xs)"})
	for _, head := range []string{
		"#report-list .report-row .report-row__trustline-count--second::before",
		"#report-list .report-row .report-row__trustline-word:not([hidden])::before",
	} {
		p16MustDecl(t, "phone trust line dot", p31mRule(t, phone, m, head),
			map[string]string{"margin-right": "var(--space-xs)"})
	}

	// Nothing on a phone hides a part of the row.
	for _, r := range p16ParseCSS(phone, p16ReadStatic(t, phone)) {
		if !strings.Contains(r.Head, "#report-list") {
			continue
		}
		if r.Decls["display"] == "none" || r.Decls["visibility"] == "hidden" {
			t.Errorf("phone.css hides a row part: %q", r.Head)
		}
		if v := r.Decls["-webkit-line-clamp"]; v != "" && v != "2" {
			t.Errorf("phone.css clamps %q to %s lines", r.Head, v)
		}
	}

	// The laptop row keeps its spacing (T13.4a sits at 124.2px of 125px).
	p16MustDecl(t, "laptop row", p31mRule(t, feed, "", "#report-list .report-row"),
		map[string]string{"padding": "var(--space-md)"})
	p16MustDecl(t, "laptop row body", p31mRule(t, feed, "", "#report-list .report-row .report-row__body"),
		map[string]string{"column-gap": "var(--space-sm)", "row-gap": "var(--space-xs)"})
	p16MustDecl(t, "laptop row title", p31mRule(t, feed, "", "#report-list .report-row .report-row__title"),
		map[string]string{"margin": "var(--space-xs) 0 0"})
}

// TestBrowserFindingF1PhoneMapStripLeavesThreeCards pins finding F1: at 390
// by 844 only one or two cards were whole above the bottom bar (UI-SPEC wants
// at least three), because the map took 40 percent of the height and the
// unofficial bar now sits above the page. The map strip is 176px on a phone
// (64 + 64 + 48, tokens only), the grid row and the map box agree, and the
// list pane starts 8px above the tab strip. The tab strip, the heading and
// the 16px sides of the list are unchanged.
func TestBrowserFindingF1PhoneMapStripLeavesThreeCards(t *testing.T) {
	const phone = "static/css/phone.css"
	m := browserFindingsPhoneMedia
	const strip = "calc(var(--space-3xl) * 2 + var(--space-2xl))"

	shell := p31mRule(t, phone, m, "body > .app-shell")
	if got := p19NoSpace(shell.Decls["grid-template-rows"]); got != p19NoSpace(strip+" minmax(0, 1fr)") {
		t.Errorf("phone shell rows are %q, want the 176px strip then the list", shell.Decls["grid-template-rows"])
	}
	mapBox := p31mRule(t, phone, m, ".pane--map #map")
	if got := p19NoSpace(mapBox.Decls["height"]); got != p19NoSpace(strip) {
		t.Errorf("phone map height is %q, want the same strip as the grid row", mapBox.Decls["height"])
	}

	raw := p16ReadStatic(t, phone)
	for _, old := range []string{"40vh", "40dvh"} {
		if strings.Contains(raw, old) {
			t.Errorf("phone.css still holds the old %s map height", old)
		}
	}

	pane := p31mRule(t, phone, m, ".app-shell[data-view] .pane--list")
	if !strings.HasPrefix(pane.Decls["padding"], "var(--space-sm) var(--space-md) ") {
		t.Errorf("phone list pane padding is %q, want 8px on top and 16px at the sides", pane.Decls["padding"])
	}

	// The tab strip and the heading keep their phone spacing.
	p16MustDecl(t, "phone heading", p31mRule(t, phone, m, ".pane--list > .feed-heading"),
		map[string]string{"margin-bottom": "var(--space-sm)"})
	p16MustDecl(t, "tab strip", p31mRule(t, "static/css/feed.css", "", ".view-tabs"),
		map[string]string{"margin": "0 0 var(--space-sm)"})
}
