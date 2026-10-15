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
