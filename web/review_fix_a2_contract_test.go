package web

import (
	"strings"
	"testing"
)

// This file holds the static contract tests of the Phase 03.1 review fixes
// made by fixer A2, the two follow ups fixer A found: the map follows the list
// pane's loading mark, and a redraw that closes the linked report's popup ends
// the link. Like the other contract files nothing here needs a database or a
// server. The behaviour itself was driven with a stubbed DOM outside the
// repository and is described in 03.1-REVIEW-FIX-A2.md.

// TestReviewFixA2MapFollowsLoadingMark pins the map half of WR-03: while the
// list pane carries fixer A's data-loading mark, the report pins and the map
// label are hidden by the stylesheet, keyed off that one mark (no second flag
// in any script), with no dimming and no transition, and the map controls, the
// map tools and the popup are left alone.
func TestReviewFixA2MapFollowsLoadingMark(t *testing.T) {
	css := stripCSSComments(p13ReadStatic(t, "static/css/feed.css"))
	rule := ".app-shell:has(> .pane--list[data-loading]) .pane--map .leaflet-marker-pane,\n" +
		".app-shell:has(> .pane--list[data-loading]) #map-label:not([hidden]) {\n  display: none;\n}"
	p13MustContain(t, "feed.css", css, rule)
	at := strings.Index(css, rule)
	if at == -1 {
		return
	}
	body := css[at : at+len(rule)]
	p13MustNotContain(t, "map loading rule", body,
		"opacity", "transition", "animation", "leaflet-control", "map-tools", "leaflet-popup", "detail-pane")

	// The one mark is fixer A's, set and removed in markListLoading only.
	app := p19Code(t, "static/js/app.js")
	if n := strings.Count(app, "'data-loading'"); n != 2 {
		t.Errorf("app.js names 'data-loading' %d times, want 2 (set and remove in markListLoading)", n)
	}
	for _, path := range []string{"static/js/map.js", "static/js/feed.js", "static/js/map-tools.js"} {
		p13MustNotContain(t, path, p19Code(t, path), "data-loading", "dataset.loading")
	}

	// The selector relies on the map pane and the list pane being children of
	// the shell, in this order.
	tmpl := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, tmpl, "index.html.tmpl panes",
		`<div class="app-shell" id="app-shell"`, `<section class="pane pane--map">`,
		`<p class="map-label" id="map-label" hidden></p>`, `<section class="pane pane--list">`)
}
