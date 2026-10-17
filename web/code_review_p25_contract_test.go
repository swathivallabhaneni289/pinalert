package web

import (
	"strings"
	"testing"
)

// This file holds the static contract tests added by Phase 3.4 plan 25 (the
// code review, part 2, browser scripts). The behaviour of each fix is run in
// node (ended_nearby_test.mjs, position_test.mjs, reader_test.mjs,
// settings_window_test.mjs, show_menu_test.mjs, app_bar_back_test.mjs); the
// tests here pin only what a node sandbox cannot see: the order in which the
// page loads its scripts.

// TestEndedNearbyLoadsAfterTheScriptsItCalls pins WR-07. ended-nearby.js starts
// its read when it runs, and the answer can land while the browser is still
// fetching a later deferred script (a real Chromium and WebKit do this: a fast
// answer arrived before a slower deferred script had run, so the draw threw a
// ReferenceError and the part showed "Couldn't load reports"). ended-cards.js
// calls PinalertTrust at draw time and ended-nearby.js calls PinalertEnded, so
// both must run after visibility.js and trust.js, and after the scripts they
// read: position.js and votes.js.
func TestEndedNearbyLoadsAfterTheScriptsItCalls(t *testing.T) {
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")
	p13Ordered(t, tmpl, "profile.html.tmpl scripts",
		`/static/js/app.js?v={{.AssetVersion}}" defer`,
		`/static/js/votes.js?v={{.AssetVersion}}" defer`,
		`/static/js/position.js?v={{.AssetVersion}}" defer`,
		`/static/js/visibility.js?v={{.AssetVersion}}" defer`,
		`/static/js/trust.js?v={{.AssetVersion}}" defer`,
		`/static/js/ended-cards.js?v={{.AssetVersion}}" defer`,
		`/static/js/ended-nearby.js?v={{.AssetVersion}}" defer`)
	for _, name := range []string{"visibility.js", "trust.js", "ended-cards.js", "ended-nearby.js"} {
		if n := strings.Count(tmpl, "/static/js/"+name); n != 1 {
			t.Errorf("profile.html.tmpl loads %s %d times, want 1", name, n)
		}
	}
}
