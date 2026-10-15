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

	// map.js ends the link on every close of the linked popup but a resize's
	// (the reader's own close, and a redraw that removed the pin, fixer A2),
	// and redraws after the close has finished.
	mapCode := p31fMapCode(t)
	if n := strings.Count(mapCode, "Pinalert.dropFocus()"); n != 1 {
		t.Errorf("map.js calls Pinalert.dropFocus() %d times, want 1 (the linked popup close)", n)
	}
	closeBody := jsFunctionBody(t, mapCode, "onPopupClose(evt)")
	p13Ordered(t, closeBody, "onPopupClose link end",
		"returnFocusToOpener(",
		"if (id !== null && String(id) === openedFocusId && !resizeClosing) {",
		"endLinkOnClose();")
	p13Ordered(t, jsFunctionBody(t, mapCode, "endLinkOnClose()"), "endLinkOnClose",
		"Pinalert.dropFocus();", "window.setTimeout(redrawAfterLinkEnded, 0);")
	redraw := jsFunctionBody(t, mapCode, "redrawAfterLinkEnded()")
	p13Ordered(t, redraw, "redrawAfterLinkEnded",
		"render(Pinalert.state);", "!before.isConnected", "map.getContainer().focus({ preventScroll: true });")
	resize := jsFunctionBody(t, mapCode, "onMapResize()")
	p13Ordered(t, resize, "onMapResize", "resizeClosing = true;", "map.closePopup();", "resizeClosing = false;")
}

// TestReviewFixAListPaneLoadingMark pins WR-03: a request for another tab,
// chip or centre than the rows on screen marks the list pane, the mark comes
// off when that answer or a failure is applied, the store keeps the rows (so
// the WR-08 literals keep their meaning) and the stylesheet hides every part
// of the old set and shows the skeleton under the mark.
func TestReviewFixAListPaneLoadingMark(t *testing.T) {
	app := p19Code(t, "static/js/app.js")

	mark := jsFunctionBody(t, app, "markListLoading(on)")
	p13MustContain(t, "markListLoading", mark,
		"document.querySelector('.pane--list')", "pane.setAttribute('data-loading', 'true');",
		"pane.removeAttribute('data-loading');")
	p13MustNotContain(t, "markListLoading", mark, "state.reports", "notify(", "fetch(")

	fetchBody := jsFunctionBody(t, app, "fetchReports(options)")
	p13Ordered(t, fetchBody, "fetchReports loading mark",
		"var isBackgroundRefresh = state.status === 'ok' || state.reports.length > 0;",
		"function rowsMatchStore() {",
		"markListLoading(isBackgroundRefresh && !rowsMatchStore());",
		"pending = fetch(url)",
		"lastAppliedSeq = mySeq;", "markListLoading(false);",
		".catch(function (err) {", "if (!rowsMatchStore()) {", "state.status = 'error';")
	if n := strings.Count(fetchBody, "markListLoading(false);\n        notify();"); n != 2 {
		t.Errorf("fetchReports clears the loading mark right before notify in %d places, want 2 (success and failure)", n)
	}
	// A dropped (stale) answer changes nothing, the mark included.
	if n := strings.Count(fetchBody, "if (isStale()) {\n          return whenFresh();"); n != 2 {
		t.Errorf("fetchReports keeps %d stale drops, want 2", n)
	}

	css := stripCSSComments(p13ReadStatic(t, "static/css/feed.css"))
	hideAt := strings.Index(css, ".pane--list[data-loading] #report-list:not([hidden]),")
	if hideAt == -1 {
		t.Fatalf("feed.css lacks the loading rule for the list pane")
	}
	hide := css[hideAt : hideAt+strings.Index(css[hideAt:], "}")]
	for _, id := range []string{"#report-list", "#past-groups", "#feed-count", "#feed-empty",
		"#feed-empty-history", "#disputed-empty", "#past-empty", "#filter-empty", "#feed-error"} {
		if !strings.Contains(hide, ".pane--list[data-loading] "+id+":not([hidden])") {
			t.Errorf("the loading rule does not hide %s", id)
		}
	}
	if !strings.Contains(hide, "display: none;") {
		t.Errorf("the loading rule must hide the old set (display: none), not dim it")
	}
	p13MustContain(t, "feed.css", css, ".pane--list[data-loading] #feed-skeleton[hidden] {\n  display: block;\n}")
	if strings.Contains(hide, "opacity") {
		t.Errorf("the old rows must not stay readable under the new heading")
	}
}

// TestReviewFixAToastPauseMouseOnly pins WR-04: the Undo toast pauses from the
// pointer only for a mouse, tracks the mouse from its own events instead of
// reading :hover, and restarts at 4 seconds only when it was paused, so a tap
// on a phone neither stops the timer for good nor cuts it short.
func TestReviewFixAToastPauseMouseOnly(t *testing.T) {
	app := p19Code(t, "static/js/app.js")

	p13MustContain(t, "app.js", app, "var toastPointerInside = false;")
	p13MustNotContain(t, "app.js", app, ":hover")

	pause := jsFunctionBody(t, app, "pauseToast(evt)")
	p13Ordered(t, pause, "pauseToast",
		"evt.type === 'pointerenter'", "evt.pointerType !== 'mouse'", "return;", "toastPointerInside = true;",
		"window.clearTimeout(toastTimer);")

	resume := jsFunctionBody(t, app, "resumeToast(evt)")
	p13Ordered(t, resume, "resumeToast",
		"evt.type === 'pointerleave'", "toastPointerInside = false;", "toastTimer !== null",
		"toastPointerInside || toastEl.contains(document.activeElement)",
		"startToastTimer(toastEl, TOAST_RESUME_MS);")

	show := jsFunctionBody(t, app, "showToast(message, action)")
	p13Ordered(t, show, "showToast", "toastPending = false;", "toastPointerInside = false;",
		"toastEl.addEventListener('pointerenter', pauseToast);")
}

// TestReviewFixARefusedTakeBackRefetches pins WR-05: a take back the server
// refused (a 409 or any other answered refusal) reads the list again before
// the block is released and the refusal line is written, from the opened
// report's Undo and from the toast Undo, while a transport failure only shows
// the line. Focus stays on the button when it is still shown.
func TestReviewFixARefusedTakeBackRefetches(t *testing.T) {
	votes := p19Code(t, "static/js/votes.js")

	answered := jsFunctionBody(t, votes, "answeredByServer(err)")
	p13MustContain(t, "answeredByServer", answered, "typeof err.field === 'string'")

	click := jsFunctionBody(t, votes, "onTakeBackClick(block, button)")
	failAt := strings.Index(click, "}, function (err) {")
	if failAt == -1 {
		t.Fatalf("onTakeBackClick lost its failure branch")
	}
	p13Ordered(t, click[failAt:], "onTakeBackClick refusal",
		"answeredByServer(err) ? Pinalert.fetchReports({ afterWrite: true }) : Promise.resolve()",
		"return refresh.then(function () {",
		"setVoteButtonLabel(button, restingLabel);", "setBlockBusy(block, false);",
		"showVoteError(block, takeBackErrorMessage(err));",
		"if (canTakeFocus(button)) {", "button.focus();", "focusAfterTakeBack(origin);")

	toast := jsFunctionBody(t, votes, "takeBackFromToast(reportId, wasClosed, link)")
	toastFail := strings.Index(toast, "}, function (err) {")
	if toastFail == -1 {
		t.Fatalf("takeBackFromToast lost its failure branch")
	}
	p13Ordered(t, toast[toastFail:], "takeBackFromToast refusal",
		"var sentence = takeBackErrorMessage(err);", "if (!answeredByServer(err)) {", "return sentence;",
		"return Pinalert.fetchReports({ afterWrite: true }).then(function () {")
	if n := strings.Count(toast[toastFail:], "return sentence;"); n != 2 {
		t.Errorf("takeBackFromToast returns the refusal sentence on %d paths, want 2 (with and without the refetch)", n)
	}
}
