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

// TestReviewFixA2LinkEndsOnRedrawClose pins the follow up of CR-01: a redraw
// that removes the linked report's pin (a resolve from Active, a take back off
// Past) ends the link like the reader's own close, so no later poll sends
// report=ID for it. Only a resize close keeps the link, the focus return stays
// gated on the redraw flag and synchronous, and the link end still has one
// dropFocus call and one deferred redraw.
func TestReviewFixA2LinkEndsOnRedrawClose(t *testing.T) {
	mapCode := p31fMapCode(t)
	closeBody := jsFunctionBody(t, mapCode, "onPopupClose(evt)")

	endAt := strings.Index(closeBody, "String(id) === openedFocusId")
	if endAt == -1 {
		t.Fatalf("onPopupClose lost the linked popup check")
	}
	lineEnd := strings.Index(closeBody[endAt:], "\n")
	if lineEnd == -1 {
		t.Fatalf("onPopupClose: the linked popup check has no line end")
	}
	cond := closeBody[endAt : endAt+lineEnd]
	p13MustContain(t, "link end condition", cond, "!resizeClosing")
	p13MustNotContain(t, "link end condition", cond, "removingPopup")

	// The focus return is unchanged: never after a redraw removed the pin,
	// and never deferred.
	p13Ordered(t, closeBody, "onPopupClose focus return",
		"if (hadFocus && lost && !removingPopup) {", "returnFocusToOpener(opener);",
		"String(id) === openedFocusId")
	p13MustNotContain(t, "onPopupClose", closeBody, "setTimeout", "requestAnimationFrame")

	// render still raises the redraw flag around the removal, so the close it
	// makes is told apart for the focus return.
	p13Ordered(t, jsFunctionBody(t, mapCode, "render(state)"), "render removal",
		"removingPopup = true;", "map.removeLayer(markers[id]);", "removingPopup = false;", "delete markers[id];",
		"openFocusOnce(state);")
}
