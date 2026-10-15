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
