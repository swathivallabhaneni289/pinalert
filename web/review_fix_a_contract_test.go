package web

import (
	"strings"
	"testing"
)

// This file holds the static contract tests of the Phase 03.1 review fixes
// for the front end state machine (fixer A: CR-01, WR-03, WR-04, WR-05 and
// IN-07). Like the other contract files nothing here needs a database or a
// server: the tests read the embedded scripts and stylesheets as text. They
// pin the shape of each fix; the behaviour itself was driven with a stubbed
// DOM outside the repository and is described in 03.1-REVIEW-FIX-A.md.

// TestReviewFixALinkedReportRefreshes pins CR-01: a report opened from a link
// is redrawn from every answer, the link id is sent until the link ends, a
// final answer (too old, unavailable) ends it, an answer sent before the link
// ended cannot bring it back, and the link ends on a reader's tab, chip or
// place change and on closing the linked popup, never on the link's own tab
// choice.
func TestReviewFixALinkedReportRefreshes(t *testing.T) {
	app := p19Code(t, "static/js/app.js")
	fetchBody := jsFunctionBody(t, app, "fetchReports(options)")

	// Every answer replaces the focus object, gated on the link it answers.
	p13Ordered(t, fetchBody, "fetchReports focus",
		"state.focus = null;",
		"if (body && body.focus) {",
		"if (sentFocusId !== null && sentFocusId === state.focusId) {",
		"state.focus = body.focus;",
		"(state.focus === null || state.focus.status !== 'ok')",
		"state.focusId = null;",
		"rowsFor = { view: sentView, category: sentCategory, lat: sentLat, lon: sentLon };")
	// The link id is no longer cleared after the first answer.
	p13MustNotContain(t, "fetchReports", fetchBody,
		"if (sentFocusId !== null && state.focusId === sentFocusId) {\n          state.focusId = null;")

	drop := jsFunctionBody(t, app, "dropFocus()")
	p13MustContain(t, "dropFocus", drop, "state.focusId = null;", "state.focus = null;")
	p13MustNotContain(t, "dropFocus", drop, "notify(", "fetchReports(", "fetch(")
	p13MustContain(t, "app.js", app, "dropFocus: dropFocus,")

	// The reader's own changes end the link before the list is read again.
	feed := pt12Feed(t)
	p13Ordered(t, jsFunctionBody(t, feed, "switchTab(view)"), "switchTab",
		"Pinalert.dropFocus();", "Pinalert.setView(view);", "Pinalert.fetchReports();")
	p13MustNotContain(t, "selectTabOnScreen", jsFunctionBody(t, feed, "selectTabOnScreen(view)"), "dropFocus")
	clearAt := strings.Index(feed, "filterClearBtn.addEventListener('click', function () {")
	if clearAt == -1 {
		t.Fatalf("feed.js lost the Clear filter listener")
	}
	p13Ordered(t, feed[clearAt:], "filter empty Clear filter",
		"Pinalert.dropFocus();", "Pinalert.setCategory('');", "Pinalert.fetchReports();")
	handle := jsFunctionBody(t, feed, "handleFocus(state)")
	p13MustContain(t, "handleFocus", handle, "if (!focus || handledFocus !== null) {")

	tools := p19Code(t, "static/js/map-tools.js")
	for _, name := range []string{"pickResult(i)", "onChipPress(evt)", "onClearFilter()"} {
		body := jsFunctionBody(t, tools, name)
		p13Ordered(t, body, name, "Pinalert.dropFocus();", "Pinalert.fetchReports()")
	}
	p13Ordered(t, jsFunctionBody(t, tools, "onBackPress()"), "onBackPress",
		"Pinalert.dropFocus();", "PinalertMap.recenterOnReader()")

	// map.js ends the link only from the reader's own close of the linked
	// popup, and redraws after the close has finished.
	mapCode := p31fMapCode(t)
	if n := strings.Count(mapCode, "Pinalert.dropFocus()"); n != 1 {
		t.Errorf("map.js calls Pinalert.dropFocus() %d times, want 1 (the linked popup close)", n)
	}
	closeBody := jsFunctionBody(t, mapCode, "onPopupClose(evt)")
	p13Ordered(t, closeBody, "onPopupClose link end",
		"returnFocusToOpener(",
		"String(id) === openedFocusId && !removingPopup && !resizeClosing",
		"endLinkOnClose();")
	p13Ordered(t, jsFunctionBody(t, mapCode, "endLinkOnClose()"), "endLinkOnClose",
		"Pinalert.dropFocus();", "window.setTimeout(redrawAfterLinkEnded, 0);")
	redraw := jsFunctionBody(t, mapCode, "redrawAfterLinkEnded()")
	p13Ordered(t, redraw, "redrawAfterLinkEnded",
		"render(Pinalert.state);", "!before.isConnected", "map.getContainer().focus({ preventScroll: true });")
	resize := jsFunctionBody(t, mapCode, "onMapResize()")
	p13Ordered(t, resize, "onMapResize", "resizeClosing = true;", "map.closePopup();", "resizeClosing = false;")
}
